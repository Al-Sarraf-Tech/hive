// Package api implements the gRPC API server for hived.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"time"

	hivev1 "github.com/jalsarraf0/hive/daemon/internal/api/gen/hive/v1"
	"github.com/jalsarraf0/hive/daemon/internal/container"
	"github.com/jalsarraf0/hive/daemon/internal/health"
	"github.com/jalsarraf0/hive/daemon/internal/hivefile"
	"github.com/jalsarraf0/hive/daemon/internal/mesh"
	"github.com/jalsarraf0/hive/daemon/internal/scheduler"
	"github.com/jalsarraf0/hive/daemon/internal/secrets"
	"github.com/jalsarraf0/hive/daemon/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements the HiveAPI gRPC service.
type Server struct {
	hivev1.UnimplementedHiveAPIServer
	store     *store.Store
	container container.Provider
	health    *health.Checker
	mesh      *mesh.Mesh           // nil in single-node mode
	scheduler *scheduler.Scheduler // nil in single-node mode
	vault     *secrets.Vault       // nil if encryption disabled
	nodeName  string
	startedAt time.Time
}

// NewServer creates a new API server.
// mesh, sched, and vault may be nil for single-node or unencrypted mode.
func NewServer(s *store.Store, c container.Provider, h *health.Checker, nodeName string, m *mesh.Mesh, sched *scheduler.Scheduler, v *secrets.Vault) *Server {
	return &Server{
		store:     s,
		container: c,
		health:    h,
		mesh:      m,
		scheduler: sched,
		vault:     v,
		nodeName:  nodeName,
		startedAt: time.Now(),
	}
}

// Register registers the gRPC services on the given server.
func Register(s *grpc.Server, srv *Server) {
	hivev1.RegisterHiveAPIServer(s, srv)
	slog.Info("api server registered", "node", srv.nodeName)
}

func (s *Server) makeNode() *hivev1.Node {
	nodeStatus := hivev1.NodeStatus_NODE_STATUS_READY
	var advertiseAddr string
	var grpcPort uint32
	if s.mesh != nil {
		local := s.mesh.LocalNode()
		nodeStatus = hivev1.NodeStatus(local.Status)
		advertiseAddr = local.AdvertiseAddr
		grpcPort = uint32(local.GRPCPort)
	}
	return &hivev1.Node{
		Id:            s.nodeName,
		Name:          s.nodeName,
		AdvertiseAddr: advertiseAddr,
		GrpcPort:      grpcPort,
		Status:        nodeStatus,
		Capabilities: &hivev1.NodeCapabilities{
			Os:               runtime.GOOS,
			Arch:             runtime.GOARCH,
			Platforms:        s.container.DetectCapabilities(),
			ContainerRuntime: s.container.RuntimeName(),
		},
		JoinedAt: timestamppb.New(s.startedAt),
	}
}

// InitCluster initializes this node as a new cluster.
func (s *Server) InitCluster(_ context.Context, req *hivev1.InitClusterRequest) (*hivev1.InitClusterResponse, error) {
	if s.mesh == nil {
		return nil, status.Error(codes.FailedPrecondition, "mesh not initialized")
	}
	clusterName := req.ClusterName
	if clusterName == "" {
		clusterName = "hive"
	}
	// Store cluster info
	if err := s.store.Put("meta", "cluster_name", []byte(clusterName)); err != nil {
		slog.Error("failed to store cluster name", "error", err)
	}

	local := s.mesh.LocalNode()
	return &hivev1.InitClusterResponse{
		ClusterId:  clusterName,
		NodeName:   local.Name,
		GossipAddr: fmt.Sprintf("%s:%d", local.AdvertiseAddr, s.mesh.GossipPort()),
	}, nil
}

// JoinCluster joins this node to an existing cluster.
func (s *Server) JoinCluster(_ context.Context, req *hivev1.JoinClusterRequest) (*hivev1.JoinClusterResponse, error) {
	if s.mesh == nil {
		return nil, status.Error(codes.FailedPrecondition, "mesh not initialized")
	}
	if len(req.SeedAddrs) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one seed address is required")
	}

	n, err := s.mesh.Join(req.SeedAddrs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "join cluster: %v", err)
	}

	// Build node list from mesh
	nodes := []*hivev1.Node{s.makeNode()}
	for _, peer := range s.mesh.Peers() {
		nodes = append(nodes, peerToNode(peer.Info))
	}

	return &hivev1.JoinClusterResponse{
		NodesJoined: uint32(n),
		Nodes:       nodes,
	}, nil
}

// GetClusterStatus returns the current cluster status.
func (s *Server) GetClusterStatus(ctx context.Context, _ *emptypb.Empty) (*hivev1.ClusterStatusResponse, error) {
	containers, err := s.container.ListContainers(ctx, map[string]string{
		"hive.managed": "true",
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list containers: %v", err)
	}

	running := 0
	for _, c := range containers {
		if c.Status == "running" {
			running++
		}
	}

	serviceNames, err := s.store.List("services")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list services: %v", err)
	}

	// Build node list from mesh
	localNode := s.makeNode()
	nodes := []*hivev1.Node{localNode}
	totalNodes := uint32(1)
	healthyNodes := uint32(0)
	if localNode.Status == hivev1.NodeStatus_NODE_STATUS_READY {
		healthyNodes = 1
	}
	if s.mesh != nil {
		for _, peer := range s.mesh.Peers() {
			nodes = append(nodes, peerToNode(peer.Info))
			totalNodes++
			if peer.Info.Status == int(mesh.NodeStatusReady) {
				healthyNodes++
			}
			// Aggregate container count from peer's gossip metadata
			running += peer.Info.Containers
		}
	}

	return &hivev1.ClusterStatusResponse{
		TotalNodes:        totalNodes,
		HealthyNodes:      healthyNodes,
		TotalServices:     uint32(len(serviceNames)),
		RunningContainers: uint32(running),
		Nodes:             nodes,
	}, nil
}

// ListNodes returns all nodes in the cluster.
func (s *Server) ListNodes(_ context.Context, _ *emptypb.Empty) (*hivev1.ListNodesResponse, error) {
	nodes := []*hivev1.Node{s.makeNode()}
	if s.mesh != nil {
		for _, peer := range s.mesh.Peers() {
			nodes = append(nodes, peerToNode(peer.Info))
		}
	}
	return &hivev1.ListNodesResponse{Nodes: nodes}, nil
}

// GetNode returns a specific node.
func (s *Server) GetNode(_ context.Context, req *hivev1.GetNodeRequest) (*hivev1.Node, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "node name is required")
	}
	if req.Name == s.nodeName {
		return s.makeNode(), nil
	}
	// Check mesh peers
	if s.mesh != nil {
		for _, peer := range s.mesh.Peers() {
			if peer.Info.Name == req.Name {
				return peerToNode(peer.Info), nil
			}
		}
	}
	return nil, status.Errorf(codes.NotFound, "node %q not found", req.Name)
}

// peerToNode converts mesh NodeInfo to a proto Node.
func peerToNode(info mesh.NodeInfo) *hivev1.Node {
	return &hivev1.Node{
		Name:          info.Name,
		AdvertiseAddr: info.AdvertiseAddr,
		GrpcPort:      uint32(info.GRPCPort),
		Status:        hivev1.NodeStatus(info.Status),
		Capabilities: &hivev1.NodeCapabilities{
			Os:               info.OS,
			Arch:             info.Arch,
			Platforms:        info.Platforms,
			ContainerRuntime: info.Runtime,
		},
	}
}

// DrainNode drains a node (stops scheduling new containers).
func (s *Server) DrainNode(_ context.Context, req *hivev1.DrainNodeRequest) (*emptypb.Empty, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "node name is required")
	}

	if req.Name == s.nodeName {
		if s.mesh != nil {
			s.mesh.SetStatus(int(mesh.NodeStatusDraining))
		}
		slog.Info("local node draining", "node", req.Name)
	} else {
		slog.Info("drain requested for remote node (not yet forwarded)", "node", req.Name)
		return nil, status.Error(codes.Unimplemented, "remote node drain not yet implemented")
	}

	return &emptypb.Empty{}, nil
}

// DeployService deploys services from a Hivefile.
// Uses the scheduler to select target nodes — may deploy locally or remotely.
func (s *Server) DeployService(ctx context.Context, req *hivev1.DeployServiceRequest) (*hivev1.DeployServiceResponse, error) {
	if req.HivefileToml == "" {
		return nil, status.Error(codes.InvalidArgument, "hivefile_toml is required")
	}

	hf, err := hivefile.ParseString(req.HivefileToml)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "parse hivefile: %v", err)
	}

	// Load and decrypt secrets from store for env resolution
	secretKeys, _ := s.store.List("secrets")
	secrets := make(map[string]string, len(secretKeys))
	for _, key := range secretKeys {
		val, err := s.store.Get("secrets", key)
		if err == nil && val != nil {
			// Decrypt if vault is available
			if s.vault != nil {
				decrypted, err := s.vault.Decrypt(val)
				if err != nil {
					slog.Warn("failed to decrypt secret, using raw value", "key", key, "error", err)
					secrets[key] = string(val)
				} else {
					secrets[key] = string(decrypted)
				}
			} else {
				secrets[key] = string(val)
			}
		}
	}

	// Warn about unimplemented depends_on (deploy order is non-deterministic)
	for name, svcDef := range hf.Service {
		if len(svcDef.DependsOn.Services) > 0 {
			slog.Warn("depends_on is not yet enforced — services may start in any order",
				"service", name, "depends_on", svcDef.DependsOn.Services)
		}
	}

	var deployed []*hivev1.Service
	for name, svcDef := range hf.Service {
		// Use scheduler to pick target node
		targetNode := s.nodeName
		if s.scheduler != nil {
			candidate, err := s.scheduler.Pick(svcDef)
			if err != nil {
				return nil, status.Errorf(codes.FailedPrecondition, "no node available for %q: %v", name, err)
			}
			targetNode = candidate.NodeName
		}

		slog.Info("deploying service", "name", name, "image", svcDef.Image, "target", targetNode)

		// Resolve env with secrets
		env, err := hivefile.ResolveEnv(svcDef.Env, secrets)
		if err != nil {
			slog.Warn("unresolved secrets in service env", "service", name, "error", err)
		}

		// Parse memory limit
		memBytes, err := hivefile.ParseMemory(svcDef.Resources.Memory)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "service %q: invalid memory %q: %v", name, svcDef.Resources.Memory, err)
		}
		if memBytes > 0 && memBytes < 1024*1024 {
			return nil, status.Errorf(codes.InvalidArgument, "service %q: memory %q is below 1MB minimum", name, svcDef.Resources.Memory)
		}

		var svcProto *hivev1.Service

		if targetNode == s.nodeName {
			// Deploy locally
			svcProto, err = s.deployLocal(ctx, name, svcDef, env, memBytes)
		} else {
			// Deploy remotely via MeshServer.StartContainer
			svcProto, err = s.deployRemote(ctx, name, svcDef, env, targetNode)
		}
		if err != nil {
			return nil, err
		}

		deployed = append(deployed, svcProto)

		// Record placement
		if err := s.store.SetPlacement(name, targetNode); err != nil {
			slog.Error("failed to record service placement", "service", name, "node", targetNode, "error", err)
		}

		// Persist service definition
		svcJSON, err := json.Marshal(svcDef)
		if err != nil {
			slog.Error("failed to marshal service definition", "service", name, "error", err)
		} else if err := s.store.Put("services", name, svcJSON); err != nil {
			slog.Error("failed to persist service definition", "service", name, "error", err)
		}

		slog.Info("service deployed", "name", name, "node", targetNode, "id", svcProto.Id)
	}

	return &hivev1.DeployServiceResponse{Services: deployed}, nil
}

// deployLocal creates a container on this node.
func (s *Server) deployLocal(ctx context.Context, name string, svcDef hivefile.ServiceDef, env map[string]string, memBytes int64) (*hivev1.Service, error) {
	memMB := memBytes / (1024 * 1024)
	containerName := fmt.Sprintf("hive-%s", name)
	spec := container.ContainerSpec{
		Name:  containerName,
		Image: svcDef.Image,
		Env:   env,
		Ports: svcDef.Ports,
		Labels: map[string]string{
			"hive.managed": "true",
			"hive.service": name,
		},
		MemoryMB:      memMB,
		CPUs:          svcDef.Resources.CPUs,
		RestartPolicy: svcDef.RestartPolicy,
	}

	// Add volumes
	for _, v := range svcDef.Volumes {
		if v.Name == "" && v.Linux == "" && v.Windows == "" && v.Target == "" {
			continue
		}
		vs := container.VolumeSpec{Name: v.Name, Target: v.Target, ReadOnly: v.ReadOnly}
		if runtime.GOOS == "windows" && v.Windows != "" {
			parts := splitVolume(v.Windows)
			vs.Source = parts[0]
			if len(parts) > 1 {
				vs.Target = parts[1]
			}
		} else if v.Linux != "" {
			parts := splitVolume(v.Linux)
			vs.Source = parts[0]
			if len(parts) > 1 {
				vs.Target = parts[1]
			}
		}
		spec.Volumes = append(spec.Volumes, vs)
	}

	// Pull image
	if err := s.container.PullImage(ctx, svcDef.Image); err != nil {
		slog.Warn("image pull failed (may be local)", "image", svcDef.Image, "error", err)
	}

	// Remove existing container
	existing, _ := s.container.ListContainers(ctx, map[string]string{"hive.service": name})
	for _, c := range existing {
		_ = s.container.Stop(ctx, c.ID, 10)
		_ = s.container.Remove(ctx, c.ID)
	}

	id, err := s.container.CreateAndStart(ctx, spec)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "deploy %q locally: %v", name, err)
	}

	return &hivev1.Service{
		Id:              id,
		Name:            name,
		Image:           svcDef.Image,
		ReplicasDesired: uint32(svcDef.Replicas),
		ReplicasRunning: 1,
		Status:          hivev1.ServiceStatus_SERVICE_STATUS_RUNNING,
		NodeConstraint:  s.nodeName,
		CreatedAt:       timestamppb.Now(),
		UpdatedAt:       timestamppb.Now(),
	}, nil
}

// deployRemote sends a StartContainer RPC to a remote node via the mesh.
func (s *Server) deployRemote(ctx context.Context, name string, svcDef hivefile.ServiceDef, env map[string]string, targetNode string) (*hivev1.Service, error) {
	if s.mesh == nil {
		return nil, status.Error(codes.FailedPrecondition, "mesh not initialized for remote deploy")
	}

	peer, err := s.mesh.PeerByName(targetNode)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "target node %q not reachable: %v", targetNode, err)
	}

	// Build the Service proto for the remote call
	svcProto := &hivev1.Service{
		Name:  name,
		Image: svcDef.Image,
		Env:   env,
		Ports: svcDef.Ports,
	}
	if svcDef.Resources.Memory != "" || svcDef.Resources.CPUs > 0 {
		svcProto.ResourceSpec = &hivev1.ResourceSpec{
			MemoryLimit: svcDef.Resources.Memory,
			CpuLimit:    svcDef.Resources.CPUs,
		}
	}

	// Send secrets separately — only env vars that originated from secret references.
	// The resolved env is already in svcProto.Env; secrets duplicates were causing
	// every plain env var to be sent twice (once in Env, once in Secrets).
	secretRefs := hivefile.ExtractSecretRefs(svcDef)
	secretBytes := make(map[string][]byte, len(secretRefs))
	for _, ref := range secretRefs {
		// Find env keys whose original values contained this secret ref
		for k, origVal := range svcDef.Env {
			if strings.Contains(origVal, "secret:"+ref) {
				secretBytes[k] = []byte(env[k])
			}
		}
	}

	resp, err := peer.MeshClient().StartContainer(ctx, &hivev1.StartContainerRequest{
		Service: svcProto,
		Secrets: secretBytes,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "deploy %q on %s: %v", name, targetNode, err)
	}

	return &hivev1.Service{
		Id:              resp.Container.Id,
		Name:            name,
		Image:           svcDef.Image,
		ReplicasDesired: uint32(svcDef.Replicas),
		ReplicasRunning: 1,
		Status:          hivev1.ServiceStatus_SERVICE_STATUS_RUNNING,
		NodeConstraint:  targetNode,
		CreatedAt:       timestamppb.Now(),
		UpdatedAt:       timestamppb.Now(),
	}, nil
}

// ListServices returns all deployed services.
func (s *Server) ListServices(ctx context.Context, _ *emptypb.Empty) (*hivev1.ListServicesResponse, error) {
	containers, err := s.container.ListContainers(ctx, map[string]string{
		"hive.managed": "true",
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list containers: %v", err)
	}

	// Track seen container IDs to prevent double-counting from fan-out
	seenContainers := make(map[string]bool)

	// Group by service name — local containers first
	serviceMap := make(map[string]*hivev1.Service)
	for _, c := range containers {
		svcName := c.Labels["hive.service"]
		if svcName == "" {
			continue
		}
		seenContainers[c.ID] = true
		svc, ok := serviceMap[svcName]
		if !ok {
			desired := uint32(1)
			if stored, err := s.store.Get("services", svcName); err == nil && stored != nil {
				var def hivefile.ServiceDef
				if json.Unmarshal(stored, &def) == nil && def.Replicas > 0 {
					desired = uint32(def.Replicas)
				}
			}
			svc = &hivev1.Service{
				Id:              c.ID,
				Name:            svcName,
				Image:           c.Image,
				ReplicasDesired: desired,
				NodeConstraint:  s.nodeName,
			}
			serviceMap[svcName] = svc
		}
		if c.Status == "running" {
			svc.ReplicasRunning++
		}
	}

	// Fan-out to remote peers — skip containers already seen locally
	if s.mesh != nil {
		for _, peer := range s.mesh.Peers() {
			peerConn, err := s.mesh.PeerByName(peer.Info.Name)
			if err != nil {
				continue
			}
			peerCtx, peerCancel := context.WithTimeout(ctx, 5*time.Second)
			state, err := peerConn.MeshClient().SyncState(peerCtx, &emptypb.Empty{})
			peerCancel()
			if err != nil {
				slog.Debug("failed to sync state from peer", "peer", peer.Info.Name, "error", err)
				continue
			}
			for _, c := range state.Containers {
				if seenContainers[c.Id] {
					continue // already counted from local Docker
				}
				seenContainers[c.Id] = true
				svcName := c.ServiceName
				if svcName == "" {
					continue
				}
				svc, ok := serviceMap[svcName]
				if !ok {
					svc = &hivev1.Service{
						Id:              c.Id,
						Name:            svcName,
						Image:           c.Image,
						ReplicasDesired: 1,
						NodeConstraint:  peer.Info.Name,
					}
					serviceMap[svcName] = svc
				}
				if c.Status == hivev1.ContainerStatus_CONTAINER_STATUS_RUNNING {
					svc.ReplicasRunning++
				}
			}
		}
	}

	// Compute aggregate status from final replica counts
	var services []*hivev1.Service
	for _, svc := range serviceMap {
		if svc.ReplicasRunning == 0 {
			svc.Status = hivev1.ServiceStatus_SERVICE_STATUS_STOPPED
		} else if svc.ReplicasRunning < svc.ReplicasDesired {
			svc.Status = hivev1.ServiceStatus_SERVICE_STATUS_DEGRADED
		} else {
			svc.Status = hivev1.ServiceStatus_SERVICE_STATUS_RUNNING
		}
		services = append(services, svc)
	}

	return &hivev1.ListServicesResponse{Services: services}, nil
}

// GetService returns a specific service.
func (s *Server) GetService(ctx context.Context, req *hivev1.GetServiceRequest) (*hivev1.Service, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "service name is required")
	}
	resp, err := s.ListServices(ctx, nil)
	if err != nil {
		return nil, err
	}
	for _, svc := range resp.Services {
		if svc.Name == req.Name {
			return svc, nil
		}
	}
	return nil, status.Errorf(codes.NotFound, "service %q not found", req.Name)
}

// StopService stops all containers for a service, locally or on remote nodes.
func (s *Server) StopService(ctx context.Context, req *hivev1.StopServiceRequest) (*emptypb.Empty, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "service name is required")
	}

	// Check placement to find which node owns this service
	placement := s.store.GetPlacement(req.Name)

	if placement != "" && placement != s.nodeName && s.mesh != nil {
		// Service is on a remote node — forward the stop
		slog.Info("forwarding stop to remote node", "service", req.Name, "node", placement)
		peer, err := s.mesh.PeerByName(placement)
		if err != nil {
			// Remote node unreachable — clean up stale placement
			slog.Warn("remote node unreachable, cleaning up stale placement", "service", req.Name, "node", placement, "error", err)
			_ = s.store.Delete("services", req.Name)
			_ = s.store.DeletePlacement(req.Name)
			return &emptypb.Empty{}, nil
		}

		// Find the container on the remote node via SyncState
		state, err := peer.MeshClient().SyncState(ctx, &emptypb.Empty{})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "sync state from %q: %v", placement, err)
		}
		for _, c := range state.Containers {
			if c.ServiceName == req.Name {
				_, err := peer.MeshClient().StopContainer(ctx, &hivev1.StopContainerRequest{
					ContainerId:    c.Id,
					TimeoutSeconds: 10,
				})
				if err != nil {
					return nil, status.Errorf(codes.Internal, "stop container on %q: %v", placement, err)
				}
			}
		}

		_ = s.store.Delete("services", req.Name)
		_ = s.store.DeletePlacement(req.Name)
		slog.Info("service stopped on remote node", "name", req.Name, "node", placement)
		return &emptypb.Empty{}, nil
	}

	// Local stop
	containers, err := s.container.ListContainers(ctx, map[string]string{
		"hive.service": req.Name,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list containers: %v", err)
	}
	allRemoved := true
	for _, c := range containers {
		slog.Info("stopping container", "service", req.Name, "id", c.ID)
		_ = s.container.Stop(ctx, c.ID, 10) // graceful stop first
		if err := s.container.Remove(ctx, c.ID); err != nil {
			slog.Error("failed to remove container", "id", c.ID, "error", err)
			allRemoved = false
		}
	}
	if allRemoved {
		_ = s.store.Delete("services", req.Name)
		_ = s.store.DeletePlacement(req.Name)
		slog.Info("service stopped", "name", req.Name)
	} else {
		return nil, status.Errorf(codes.Internal, "some containers for %q could not be removed", req.Name)
	}
	return &emptypb.Empty{}, nil
}

// ScaleService changes the replica count.
func (s *Server) ScaleService(_ context.Context, req *hivev1.ScaleServiceRequest) (*emptypb.Empty, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "service name is required")
	}
	slog.Info("scale requested", "service", req.Name, "replicas", req.Replicas)
	return nil, status.Error(codes.Unimplemented, "scaling is not yet implemented")
}

// RollbackService rolls back to the previous version.
func (s *Server) RollbackService(_ context.Context, req *hivev1.RollbackServiceRequest) (*emptypb.Empty, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "service name is required")
	}
	slog.Info("rollback requested", "service", req.Name)
	return nil, status.Error(codes.Unimplemented, "rollback is not yet implemented")
}

// ListContainers lists containers, optionally filtered.
func (s *Server) ListContainers(ctx context.Context, req *hivev1.ListContainersRequest) (*hivev1.ListContainersResponse, error) {
	// If a specific node was requested and it's not this node, return empty.
	// In multi-node mode the request should be forwarded to the correct node.
	if req.NodeName != "" && req.NodeName != s.nodeName {
		return &hivev1.ListContainersResponse{}, nil
	}

	filters := map[string]string{"hive.managed": "true"}
	if req.ServiceName != "" {
		filters["hive.service"] = req.ServiceName
	}

	containers, err := s.container.ListContainers(ctx, filters)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list containers: %v", err)
	}

	var protos []*hivev1.Container
	for _, c := range containers {
		cStatus := hivev1.ContainerStatus_CONTAINER_STATUS_STOPPED
		switch c.Status {
		case "running":
			cStatus = hivev1.ContainerStatus_CONTAINER_STATUS_RUNNING
		case "restarting":
			cStatus = hivev1.ContainerStatus_CONTAINER_STATUS_RESTARTING
		}
		protos = append(protos, &hivev1.Container{
			Id:          c.ID,
			ServiceName: c.Labels["hive.service"],
			NodeId:      s.nodeName,
			Image:       c.Image,
			Status:      cStatus,
		})
	}

	return &hivev1.ListContainersResponse{Containers: protos}, nil
}

// ContainerLogs streams logs from a container (local or remote).
func (s *Server) ContainerLogs(req *hivev1.ContainerLogsRequest, stream hivev1.HiveAPI_ContainerLogsServer) error {
	if req.ContainerId == "" && req.ServiceName == "" {
		return status.Error(codes.InvalidArgument, "container_id or service_name is required")
	}

	// If service_name is provided, look up the container
	containerID := req.ContainerId
	if containerID == "" && req.ServiceName != "" {
		containers, err := s.container.ListContainers(stream.Context(), map[string]string{
			"hive.service": req.ServiceName,
		})
		if err == nil && len(containers) > 0 {
			containerID = containers[0].ID
		}
	}

	// Try local first
	if containerID != "" {
		reader, err := s.container.Logs(stream.Context(), containerID, container.LogOpts{
			Follow:    req.Follow,
			TailLines: int(req.TailLines),
		})
		if err == nil {
			defer reader.Close()
			return container.StreamDockerLogs(reader, func(line string, streamType string) error {
				return stream.Send(&hivev1.LogEntry{
					ContainerId:   containerID,
					NodeName:      s.nodeName,
					ServiceName:   req.ServiceName,
					Line:          line,
					Stream:        streamType,
					TimestampUnix: time.Now().Unix(),
				})
			})
		}
		slog.Debug("container not found locally, trying remote peers", "container", containerID, "error", err)
	}

	// Not local — try remote peers via SyncState to find the container
	if s.mesh != nil {
		for _, peer := range s.mesh.Peers() {
			peerConn, err := s.mesh.PeerByName(peer.Info.Name)
			if err != nil {
				continue
			}

			// If we don't have a container ID, look it up on the remote node
			remoteContainerID := containerID
			if remoteContainerID == "" && req.ServiceName != "" {
				state, err := peerConn.MeshClient().SyncState(stream.Context(), &emptypb.Empty{})
				if err != nil {
					continue
				}
				for _, c := range state.Containers {
					if c.ServiceName == req.ServiceName {
						remoteContainerID = c.Id
						break
					}
				}
				if remoteContainerID == "" {
					continue // this peer doesn't have the service
				}
			}
			if remoteContainerID == "" {
				continue
			}

			remoteStream, err := peerConn.MeshClient().PullLogs(stream.Context(), &hivev1.PullLogsRequest{
				ContainerId: remoteContainerID,
				Follow:      req.Follow,
				TailLines:   req.TailLines,
			})
			if err != nil {
				continue
			}
			for {
				entry, err := remoteStream.Recv()
				if err != nil {
					break
				}
				if err := stream.Send(entry); err != nil {
					return err
				}
			}
			return nil
		}
	}

	return status.Errorf(codes.NotFound, "container not found on any node")
}

// SetSecret stores a secret.
func (s *Server) SetSecret(_ context.Context, req *hivev1.SetSecretRequest) (*emptypb.Empty, error) {
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "secret key is required")
	}
	// Encrypt with age if vault is available
	valueToStore := req.Value
	if s.vault != nil {
		encrypted, err := s.vault.Encrypt(req.Value)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "encrypt secret: %v", err)
		}
		valueToStore = encrypted
	}
	if err := s.store.Put("secrets", req.Key, valueToStore); err != nil {
		return nil, status.Errorf(codes.Internal, "store secret: %v", err)
	}
	slog.Debug("secret set", "key", req.Key)
	return &emptypb.Empty{}, nil
}

// ListSecrets returns metadata about stored secrets.
func (s *Server) ListSecrets(_ context.Context, _ *emptypb.Empty) (*hivev1.ListSecretsResponse, error) {
	keys, err := s.store.List("secrets")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list secrets: %v", err)
	}

	var metas []*hivev1.SecretMeta
	for _, key := range keys {
		metas = append(metas, &hivev1.SecretMeta{
			Key: key,
		})
	}

	return &hivev1.ListSecretsResponse{Secrets: metas}, nil
}

// DeleteSecret removes a secret.
func (s *Server) DeleteSecret(_ context.Context, req *hivev1.DeleteSecretRequest) (*emptypb.Empty, error) {
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "secret key is required")
	}
	if err := s.store.Delete("secrets", req.Key); err != nil {
		return nil, status.Errorf(codes.Internal, "delete secret: %v", err)
	}
	slog.Debug("secret deleted", "key", req.Key)
	return &emptypb.Empty{}, nil
}

// StreamEvents streams cluster events.
func (s *Server) StreamEvents(_ *emptypb.Empty, stream hivev1.HiveAPI_StreamEventsServer) error {
	err := stream.Send(&hivev1.Event{
		Type:      hivev1.EventType_EVENT_TYPE_NODE_JOINED,
		Source:    s.nodeName,
		Message:   fmt.Sprintf("Connected to %s", s.nodeName),
		Timestamp: timestamppb.Now(),
	})
	if err != nil {
		return err
	}
	<-stream.Context().Done()
	return nil
}

// splitVolume splits "source:target" volume strings.
func splitVolume(s string) []string {
	if len(s) >= 3 && s[1] == ':' && (s[0] >= 'A' && s[0] <= 'Z' || s[0] >= 'a' && s[0] <= 'z') {
		rest := s[2:]
		idx := findColonSplit(rest)
		if idx >= 0 {
			return []string{s[:2+idx], rest[idx+1:]}
		}
		return []string{s}
	}
	idx := findColonSplit(s)
	if idx >= 0 {
		return []string{s[:idx], s[idx+1:]}
	}
	return []string{s}
}

func findColonSplit(s string) int {
	for i, c := range s {
		if c == ':' {
			return i
		}
	}
	return -1
}
