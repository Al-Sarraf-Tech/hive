package api

import (
	"context"
	"crypto/subtle"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"strings"
	"time"

	hivev1 "github.com/jalsarraf0/hive/daemon/internal/api/gen/hive/v1"
	"github.com/jalsarraf0/hive/daemon/internal/container"
	"github.com/jalsarraf0/hive/daemon/internal/hivefile"
	"github.com/jalsarraf0/hive/daemon/internal/pki"
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
	dataDir   string                         // path to data dir for PKI operations
	decryptFn func([]byte) ([]byte, error) // decrypts CA key (nil = plaintext)
}

// NewMeshServer creates a new MeshServer.
func NewMeshServer(s *store.Store, c container.Provider, nodeName, dataDir string, decryptFn func([]byte) ([]byte, error)) *MeshServer {
	return &MeshServer{
		store:     s,
		container: c,
		nodeName:  nodeName,
		dataDir:   dataDir,
		decryptFn: decryptFn,
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

	// Parse base service name and replica index from the name (format: "name-N")
	baseName := svc.Name
	replicaLabel := "0"
	if idx := strings.LastIndex(svc.Name, "-"); idx > 0 {
		suffix := svc.Name[idx+1:]
		if _, err := fmt.Sscanf(suffix, "%d", new(int)); err == nil {
			baseName = svc.Name[:idx]
			replicaLabel = suffix
		}
	}

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
			"hive.service": baseName,
			"hive.replica": replicaLabel,
		},
		MemoryMB:      memMB,
		CPUs:          cpus,
		RestartPolicy: "on-failure",
	}

	// Pull image
	if err := s.container.PullImage(ctx, svc.Image); err != nil {
		slog.Warn("image pull failed (may be local)", "image", svc.Image, "error", err)
	}

	// Stop and remove existing container for this specific replica only
	existing, _ := s.container.ListContainers(ctx, map[string]string{
		"hive.managed": "true",
		"hive.service": baseName,
		"hive.replica": replicaLabel,
	})
	for _, c := range existing {
		if stopErr := s.container.Stop(ctx, c.ID, 10); stopErr != nil {
			slog.Warn("failed to stop existing container", "id", c.ID, "error", stopErr)
		}
		if rmErr := s.container.Remove(ctx, c.ID); rmErr != nil {
			slog.Warn("failed to remove existing container", "id", c.ID, "error", rmErr)
		}
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
		if putErr := s.store.Put("services", baseName, svcJSON); putErr != nil {
			slog.Warn("failed to persist service definition", "service", baseName, "error", putErr)
		}
	}

	slog.Info("container started via remote deploy", "service", baseName, "replica", replicaLabel, "id", id)

	return &hivev1.StartContainerResponse{
		Container: &hivev1.Container{
			Id:          id,
			ServiceName: baseName,
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

// SignNodeCSR signs a joining node's Certificate Signing Request using the cluster CA.
// Only the init node (which holds ca.key) can service this request.
func (s *MeshServer) SignNodeCSR(_ context.Context, req *hivev1.SignCSRRequest) (*hivev1.SignCSRResponse, error) {
	if len(req.CsrPem) == 0 {
		return nil, status.Error(codes.InvalidArgument, "csr_pem is required")
	}

	// Validate join token — required for CSR signing authentication
	storedToken, err := s.store.Get("meta", "join_token")
	if err != nil || storedToken == nil {
		return nil, status.Error(codes.FailedPrecondition, "no join token configured — run 'hive init' first")
	}
	if req.JoinToken == "" || subtle.ConstantTimeCompare([]byte(req.JoinToken), storedToken) != 1 {
		return nil, status.Error(codes.PermissionDenied, "invalid join token")
	}

	// Validate CSR common name matches the declared node name
	block, _ := pem.Decode(req.CsrPem)
	if block == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid CSR PEM")
	}
	csr, csrErr := x509.ParseCertificateRequest(block.Bytes)
	if csrErr != nil {
		return nil, status.Errorf(codes.InvalidArgument, "parse CSR: %v", csrErr)
	}
	if csr.Subject.CommonName != req.NodeName {
		return nil, status.Errorf(codes.InvalidArgument, "CSR CommonName %q does not match declared node_name %q", csr.Subject.CommonName, req.NodeName)
	}

	caKey, caCert, err := pki.LoadCA(s.dataDir, s.decryptFn)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "this node cannot sign certificates (no CA key): %v", err)
	}

	signedCertPEM, err := pki.SignCSR(caKey, caCert, req.CsrPem)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "sign CSR: %v", err)
	}

	caCertPEM, err := pki.LoadCACertPEM(s.dataDir)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load CA cert: %v", err)
	}

	slog.Info("signed node CSR", "node", req.NodeName)
	return &hivev1.SignCSRResponse{
		NodeCertPem: signedCertPEM,
		CaCertPem:   caCertPEM,
	}, nil
}
