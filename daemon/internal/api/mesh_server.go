package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	hivev1 "github.com/jalsarraf0/hive/daemon/internal/api/gen/hive/v1"
	"github.com/jalsarraf0/hive/daemon/internal/container"
	"github.com/jalsarraf0/hive/daemon/internal/hivefile"
	"github.com/jalsarraf0/hive/daemon/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// MeshServer implements the HiveMesh gRPC service for daemon-to-daemon communication.
type MeshServer struct {
	hivev1.UnimplementedHiveMeshServer
	store     *store.Store
	container container.Provider
	nodeName  string
}

// NewMeshServer creates a new MeshServer.
func NewMeshServer(s *store.Store, c container.Provider, nodeName string) *MeshServer {
	return &MeshServer{
		store:     s,
		container: c,
		nodeName:  nodeName,
	}
}

// RegisterMesh registers the HiveMesh gRPC service.
func RegisterMesh(s *grpc.Server, srv *MeshServer) {
	hivev1.RegisterHiveMeshServer(s, srv)
	slog.Info("mesh server registered", "node", srv.nodeName)
}

// Ping returns lightweight status info for this node.
func (s *MeshServer) Ping(ctx context.Context, _ *emptypb.Empty) (*hivev1.PingResponse, error) {
	containers, err := s.container.ListContainers(ctx, map[string]string{
		"hive.managed": "true",
	})
	if err != nil {
		slog.Warn("container runtime degraded during ping", "error", err)
	}
	running := uint32(0)
	for _, c := range containers {
		if c.Status == "running" {
			running++
		}
	}

	return &hivev1.PingResponse{
		NodeId:            s.nodeName,
		NodeName:          s.nodeName,
		RunningContainers: running,
	}, nil
}

// SyncState returns the full local cluster state (services + containers on this node).
func (s *MeshServer) SyncState(ctx context.Context, _ *emptypb.Empty) (*hivev1.ClusterState, error) {
	// Load local services
	serviceNames, err := s.store.List("services")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list services: %v", err)
	}

	var services []*hivev1.Service
	for _, name := range serviceNames {
		data, err := s.store.Get("services", name)
		if err != nil || data == nil {
			continue
		}
		var def hivefile.ServiceDef
		if err := json.Unmarshal(data, &def); err != nil {
			continue
		}
		services = append(services, &hivev1.Service{
			Name:            name,
			Image:           def.Image,
			ReplicasDesired: uint32(def.Replicas),
			NodeConstraint:  s.nodeName,
		})
	}

	// List local containers
	localContainers, err := s.container.ListContainers(ctx, map[string]string{
		"hive.managed": "true",
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list containers: %v", err)
	}

	var containers []*hivev1.Container
	for _, c := range localContainers {
		cStatus := hivev1.ContainerStatus_CONTAINER_STATUS_STOPPED
		if c.Status == "running" {
			cStatus = hivev1.ContainerStatus_CONTAINER_STATUS_RUNNING
		}
		containers = append(containers, &hivev1.Container{
			Id:          c.ID,
			ServiceName: c.Labels["hive.service"],
			NodeId:      s.nodeName,
			Image:       c.Image,
			Status:      cStatus,
		})
	}

	// Build local node entry
	localNode := &hivev1.Node{
		Id:   s.nodeName,
		Name: s.nodeName,
		Capabilities: &hivev1.NodeCapabilities{
			ContainerRuntime: s.container.RuntimeName(),
			Platforms:        s.container.DetectCapabilities(),
		},
		Status: hivev1.NodeStatus_NODE_STATUS_READY,
	}

	return &hivev1.ClusterState{
		Nodes:      []*hivev1.Node{localNode},
		Services:   services,
		Containers: containers,
	}, nil
}

// StartContainer creates and starts a container on this node.
// Called by remote nodes when the scheduler places a service here.
func (s *MeshServer) StartContainer(ctx context.Context, req *hivev1.StartContainerRequest) (*hivev1.StartContainerResponse, error) {
	if req.Service == nil {
		return nil, status.Error(codes.InvalidArgument, "service is required")
	}
	svc := req.Service
	slog.Info("remote deploy request", "service", svc.Name, "image", svc.Image, "from_node", "remote")

	// Build env from the service proto + injected secrets
	env := make(map[string]string)
	for k, v := range svc.Env {
		env[k] = v
	}
	for k, v := range req.Secrets {
		env[k] = string(v) // secrets injected as env vars by the requesting node
	}

	containerName := fmt.Sprintf("hive-%s", svc.Name)

	// Parse resource limits
	memMB := int64(0)
	cpus := float64(0)
	if svc.ResourceSpec != nil {
		memBytes, err := hivefile.ParseMemory(svc.ResourceSpec.MemoryLimit)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid memory limit %q: %v", svc.ResourceSpec.MemoryLimit, err)
		}
		memMB = memBytes / (1024 * 1024)
		cpus = svc.ResourceSpec.CpuLimit
	}

	spec := container.ContainerSpec{
		Name:  containerName,
		Image: svc.Image,
		Env:   env,
		Ports: svc.Ports,
		Labels: map[string]string{
			"hive.managed": "true",
			"hive.service": svc.Name,
		},
		MemoryMB:      memMB,
		CPUs:          cpus,
		RestartPolicy: "on-failure",
	}

	// Pull image
	if err := s.container.PullImage(ctx, svc.Image); err != nil {
		slog.Warn("image pull failed (may be local)", "image", svc.Image, "error", err)
	}

	// Stop and remove existing container if any (graceful shutdown)
	existing, _ := s.container.ListContainers(ctx, map[string]string{
		"hive.service": svc.Name,
	})
	for _, c := range existing {
		_ = s.container.Stop(ctx, c.ID, 10)
		_ = s.container.Remove(ctx, c.ID)
	}

	id, err := s.container.CreateAndStart(ctx, spec)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "start container: %v", err)
	}

	// Persist service definition locally so health checks and SyncState work
	svcDef := hivefile.ServiceDef{
		Image:         svc.Image,
		Replicas:      1,
		RestartPolicy: "on-failure",
	}
	if svcJSON, err := json.Marshal(svcDef); err == nil {
		_ = s.store.Put("services", svc.Name, svcJSON)
	}

	slog.Info("container started via remote deploy", "service", svc.Name, "id", id)

	return &hivev1.StartContainerResponse{
		Container: &hivev1.Container{
			Id:          id,
			ServiceName: svc.Name,
			NodeId:      s.nodeName,
			Image:       svc.Image,
			Status:      hivev1.ContainerStatus_CONTAINER_STATUS_RUNNING,
			StartedAt:   timestamppb.Now(),
		},
	}, nil
}

// StopContainer stops and removes a container on this node.
func (s *MeshServer) StopContainer(ctx context.Context, req *hivev1.StopContainerRequest) (*emptypb.Empty, error) {
	if req.ContainerId == "" {
		return nil, status.Error(codes.InvalidArgument, "container_id is required")
	}
	timeout := int(req.TimeoutSeconds)
	if timeout == 0 {
		timeout = 10
	}
	if err := s.container.Stop(ctx, req.ContainerId, timeout); err != nil {
		slog.Warn("stop container failed", "id", req.ContainerId, "error", err)
	}
	if err := s.container.Remove(ctx, req.ContainerId); err != nil {
		return nil, status.Errorf(codes.Internal, "remove container %s: %v", req.ContainerId, err)
	}
	return &emptypb.Empty{}, nil
}

// PullLogs streams container logs from this node.
func (s *MeshServer) PullLogs(req *hivev1.PullLogsRequest, stream hivev1.HiveMesh_PullLogsServer) error {
	if req.ContainerId == "" {
		return status.Error(codes.InvalidArgument, "container_id is required")
	}

	reader, err := s.container.Logs(stream.Context(), req.ContainerId, container.LogOpts{
		Follow:    req.Follow,
		TailLines: int(req.TailLines),
	})
	if err != nil {
		return status.Errorf(codes.Internal, "get logs: %v", err)
	}
	defer reader.Close()

	return container.StreamDockerLogs(reader, func(line string, streamType string) error {
		return stream.Send(&hivev1.LogEntry{
			ContainerId:   req.ContainerId,
			NodeName:      s.nodeName,
			Line:          line,
			Stream:        streamType,
			TimestampUnix: time.Now().Unix(),
		})
	})
}

// ReplicateSecret stores a secret replicated from another node.
func (s *MeshServer) ReplicateSecret(_ context.Context, req *hivev1.ReplicateSecretRequest) (*emptypb.Empty, error) {
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}
	if err := s.store.Put("secrets", req.Key, req.EncryptedValue); err != nil {
		return nil, status.Errorf(codes.Internal, "store secret: %v", err)
	}
	slog.Debug("secret replicated from peer", "key", req.Key)
	return &emptypb.Empty{}, nil
}
