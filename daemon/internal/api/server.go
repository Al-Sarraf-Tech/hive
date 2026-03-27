// Package api implements the gRPC API server for hived.
package api

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	hivev1 "github.com/jalsarraf0/hive/daemon/internal/api/gen/hive/v1"
	"github.com/jalsarraf0/hive/daemon/internal/container"
	"github.com/jalsarraf0/hive/daemon/internal/health"
	"github.com/jalsarraf0/hive/daemon/internal/hivefile"
	"github.com/jalsarraf0/hive/daemon/internal/mesh"
	"github.com/jalsarraf0/hive/daemon/internal/pki"
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
// validServiceName restricts service names to safe characters for labels, store keys, and container names.
var validServiceName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,62}$`)

type Server struct {
	hivev1.UnimplementedHiveAPIServer
	store     *store.Store
	container container.Provider
	health    *health.Checker
	mesh      *mesh.Mesh           // nil in single-node mode
	scheduler *scheduler.Scheduler // nil in single-node mode
	vault     *secrets.Vault       // nil if encryption disabled
	nodeName  string
	dataDir   string
	startedAt time.Time
	deployMu        sync.Mutex // serializes DeployService to prevent concurrent races
	certBootstrapMu sync.Mutex // serializes bootstrapNodeCert to prevent concurrent CSR signing
}

// NewServer creates a new API server.
// mesh, sched, and vault may be nil for single-node or unencrypted mode.
func NewServer(s *store.Store, c container.Provider, h *health.Checker, nodeName string, m *mesh.Mesh, sched *scheduler.Scheduler, v *secrets.Vault, dataDir string) *Server {
	return &Server{
		store:     s,
		container: c,
		health:    h,
		mesh:      m,
		scheduler: sched,
		vault:     v,
		nodeName:  nodeName,
		dataDir:   dataDir,
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

	// Prevent accidental CA regeneration — InitCluster must be called only once
	if pki.HasCACert(s.dataDir) {
		return nil, status.Error(codes.AlreadyExists, "cluster already initialized — CA material exists. Use a fresh data directory to re-initialize.")
	}

	clusterName := req.ClusterName
	if clusterName == "" {
		clusterName = "hive"
	}
	// Store cluster info
	if err := s.store.Put("meta", "cluster_name", []byte(clusterName)); err != nil {
		return nil, status.Errorf(codes.Internal, "persist cluster name: %v", err)
	}

	// Generate cluster CA and node certificate
	caKey, caCert, caCertPEM, caKeyPEM, err := pki.GenerateCA()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate cluster CA: %v", err)
	}
	if err := pki.SaveCA(s.dataDir, caCertPEM, caKeyPEM); err != nil {
		return nil, status.Errorf(codes.Internal, "save cluster CA: %v", err)
	}

	local := s.mesh.LocalNode()
	nodeCertPEM, nodeKeyPEM, err := pki.GenerateNodeCert(caKey, caCert, local.Name, local.AdvertiseAddr)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate node certificate: %v", err)
	}
	if err := pki.SaveNodeCert(s.dataDir, nodeCertPEM, nodeKeyPEM); err != nil {
		return nil, status.Errorf(codes.Internal, "save node certificate: %v", err)
	}

	slog.Info("cluster PKI initialized",
		"ca_fingerprint", pki.CACertFingerprint(caCert),
		"node_cert_cn", local.Name,
	)

	// Generate a cryptographically random join token for CSR authentication
	tokenBytes := make([]byte, 32)
	if _, err := cryptorand.Read(tokenBytes); err != nil {
		return nil, status.Errorf(codes.Internal, "generate join token: %v", err)
	}
	joinToken := hex.EncodeToString(tokenBytes)
	if err := s.store.Put("meta", "join_token", []byte(joinToken)); err != nil {
		return nil, status.Errorf(codes.Internal, "persist join token: %v", err)
	}

	return &hivev1.InitClusterResponse{
		ClusterId:     clusterName,
		NodeName:      local.Name,
		GossipAddr:    fmt.Sprintf("%s:%d", local.AdvertiseAddr, s.mesh.GossipPort()),
		CaFingerprint: pki.CACertFingerprint(caCert),
		JoinToken:     joinToken,
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

	// Bootstrap node certificate via CSR signing if not already provisioned
	if !pki.HasNodeCert(s.dataDir) {
		s.certBootstrapMu.Lock()
		defer s.certBootstrapMu.Unlock()
		// Re-check under lock to avoid TOCTOU
		if !pki.HasNodeCert(s.dataDir) {
			if err := s.bootstrapNodeCert(req.JoinToken); err != nil {
				slog.Warn("node certificate bootstrap failed — mTLS will not be active until resolved", "error", err)
			}
		}
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

	s.deployMu.Lock()
	defer s.deployMu.Unlock()

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

	// Validate all service names before deploying any
	for name := range hf.Service {
		if !validServiceName.MatchString(name) {
			return nil, status.Errorf(codes.InvalidArgument, "invalid service name %q: must match [a-zA-Z0-9][a-zA-Z0-9._-]{0,62}", name)
		}
	}

	// Topologically sort services by depends_on (dependencies deploy first).
	// Detects cycles and missing dependency references.
	deployOrder, err := hivefile.TopoSort(hf.Service)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "dependency error: %v", err)
	}
	slog.Debug("deploy order resolved", "order", deployOrder)

	var deployed []*hivev1.Service
	for _, name := range deployOrder {
		svcDef := hf.Service[name]

		// Resolve env with secrets — fail if any secret references are unresolved
		env, err := hivefile.ResolveEnv(svcDef.Env, secrets)
		if err != nil {
			return nil, status.Errorf(codes.FailedPrecondition, "service %q: %v — set missing secrets with 'hive secret set'", name, err)
		}

		// Parse memory limit
		memBytes, err := hivefile.ParseMemory(svcDef.Resources.Memory)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "service %q: invalid memory %q: %v", name, svcDef.Resources.Memory, err)
		}
		if memBytes > 0 && memBytes < 1024*1024 {
			return nil, status.Errorf(codes.InvalidArgument, "service %q: memory %q is below 1MB minimum", name, svcDef.Resources.Memory)
		}

		// Pull image once before deploying replicas
		if err := s.container.PullImage(ctx, svcDef.Image); err != nil {
			slog.Warn("image pull failed (may be local)", "image", svcDef.Image, "error", err)
		}

		// Deploy N replicas, distributing across nodes via scheduler
		replicas := svcDef.Replicas
		if replicas <= 0 {
			replicas = 1
		}

		// Archive previous version for rollback BEFORE deploying (ensures rollback is possible if deploy crashes)
		if prev, _ := s.store.Get("services", name); prev != nil {
			_ = s.store.Put("service_history", name, prev)
		}

		// Check if this is an update to an existing service (for rolling strategy)
		existingContainers, _ := s.container.ListContainers(ctx, map[string]string{
			"hive.managed": "true",
			"hive.service": name,
		})
		isUpdate := len(existingContainers) > 0
		strategy := svcDef.Deploy.Strategy
		if strategy == "" {
			strategy = "rolling"
		}

		var containerIDs []string
		replicasRunning := uint32(0)

		if isUpdate && strategy == "rolling" {
			// Rolling update: replace replicas one at a time
			slog.Info("rolling update", "service", name, "existing", len(existingContainers), "desired", replicas)

			healthPause := 5 * time.Second
			if svcDef.Health.Type != "" && svcDef.Health.Port > 0 {
				if d, parseErr := time.ParseDuration(svcDef.Health.Interval); parseErr == nil && d > 0 {
					healthPause = d
				}
			}

			for i := 0; i < replicas; i++ {
				targetNode := s.nodeName
				if s.scheduler != nil {
					if candidate, pickErr := s.scheduler.Pick(svcDef); pickErr == nil {
						targetNode = candidate.NodeName
					}
				}

				slog.Info("rolling update replica", "service", name, "replica", i, "target", targetNode)

				var id string
				if targetNode == s.nodeName {
					id, err = s.deployLocalReplica(ctx, name, i, svcDef, env, memBytes)
				} else {
					id, err = s.deployRemoteReplica(ctx, name, i, svcDef, env, targetNode)
				}
				if err != nil {
					slog.Error("rolling update: replica failed", "service", name, "replica", i, "error", err)
					continue
				}

				containerIDs = append(containerIDs, id)
				replicasRunning++

				// Verify the new replica is healthy before proceeding
				if svcDef.Health.Type != "" && svcDef.Health.Port > 0 {
					healthy := false
					checkTimeout := 5 * time.Second
					if d, parseErr := time.ParseDuration(svcDef.Health.Timeout); parseErr == nil && d > 0 {
						checkTimeout = d
					}
					maxChecks := 10
					for check := 0; check < maxChecks; check++ {
						select {
						case <-ctx.Done():
							return nil, status.Errorf(codes.Canceled, "deploy cancelled during rolling update health check")
						case <-time.After(checkTimeout):
						}
						result := s.health.Check(ctx, health.Config{
							Type:    health.CheckType(svcDef.Health.Type),
							Host:    "127.0.0.1",
							Port:    svcDef.Health.Port,
							Path:    svcDef.Health.Path,
							Timeout: checkTimeout,
						})
						if result.Healthy {
							healthy = true
							slog.Info("rolling update: replica healthy", "service", name, "replica", i, "check", check+1)
							break
						}
						slog.Debug("rolling update: health check pending", "service", name, "replica", i, "check", check+1, "message", result.Message)
					}
					if !healthy {
						slog.Error("rolling update: replica failed health check after all retries", "service", name, "replica", i)
						// Continue deploying remaining replicas despite health failure
					}
				} else if i < replicas-1 {
					// No health check configured — just wait a fixed pause
					select {
					case <-ctx.Done():
						return nil, status.Errorf(codes.Canceled, "deploy cancelled during rolling update")
					case <-time.After(healthPause):
					}
				}
			}

			// Clean up excess old containers if scaling down during update
			if len(existingContainers) > replicas {
				for i := replicas; i < len(existingContainers); i++ {
					c := existingContainers[i]
					_ = s.container.Stop(ctx, c.ID, 10)
					_ = s.container.Remove(ctx, c.ID)
				}
			}
		} else {
			// Recreate strategy (or fresh deploy): deploy all replicas at once
			for i := 0; i < replicas; i++ {
				targetNode := s.nodeName
				if s.scheduler != nil {
					if candidate, pickErr := s.scheduler.Pick(svcDef); pickErr == nil {
						targetNode = candidate.NodeName
					}
				}

				slog.Info("deploying replica", "service", name, "replica", i, "target", targetNode)

				var id string
				if targetNode == s.nodeName {
					id, err = s.deployLocalReplica(ctx, name, i, svcDef, env, memBytes)
				} else {
					id, err = s.deployRemoteReplica(ctx, name, i, svcDef, env, targetNode)
				}
				if err != nil {
					slog.Error("failed to deploy replica", "service", name, "replica", i, "error", err)
					continue
				}

				containerIDs = append(containerIDs, id)
				replicasRunning++
			}
		}

		if replicasRunning == 0 {
			return nil, status.Errorf(codes.Internal, "all replicas of %q failed to deploy", name)
		}

		svcStatus := hivev1.ServiceStatus_SERVICE_STATUS_RUNNING
		if replicasRunning < uint32(replicas) {
			svcStatus = hivev1.ServiceStatus_SERVICE_STATUS_DEGRADED
		}

		svcProto := &hivev1.Service{
			Id:              containerIDs[0], // primary container ID
			Name:            name,
			Image:           svcDef.Image,
			ReplicasDesired: uint32(replicas),
			ReplicasRunning: replicasRunning,
			Status:          svcStatus,
			NodeConstraint:  svcDef.Node,
			CreatedAt:       timestamppb.Now(),
			UpdatedAt:       timestamppb.Now(),
		}
		deployed = append(deployed, svcProto)

		// Record placement (primary node)
		if err := s.store.SetPlacement(name, s.nodeName); err != nil {
			slog.Error("failed to record service placement", "service", name, "error", err)
		}

		// Persist service definition
		svcJSON, err := json.Marshal(svcDef)
		if err != nil {
			slog.Error("failed to marshal service definition", "service", name, "error", err)
		} else if err := s.store.Put("services", name, svcJSON); err != nil {
			slog.Error("failed to persist service definition", "service", name, "error", err)
		}

		slog.Info("service deployed", "name", name, "replicas", fmt.Sprintf("%d/%d", replicasRunning, replicas))
	}

	return &hivev1.DeployServiceResponse{Services: deployed}, nil
}

// deployLocalReplica creates a single replica container on this node.
func (s *Server) deployLocalReplica(ctx context.Context, name string, replicaIndex int, svcDef hivefile.ServiceDef, env map[string]string, memBytes int64) (string, error) {
	memMB := memBytes / (1024 * 1024)
	containerName := fmt.Sprintf("hive-%s-%d", name, replicaIndex)
	spec := container.ContainerSpec{
		Name:  containerName,
		Image: svcDef.Image,
		Env:   env,
		Ports: svcDef.Ports,
		Labels: map[string]string{
			"hive.managed": "true",
			"hive.service": name,
			"hive.replica": fmt.Sprintf("%d", replicaIndex),
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

	// Remove existing container with this name (redeploy)
	existing, _ := s.container.ListContainers(ctx, map[string]string{
		"hive.service": name,
		"hive.replica":  fmt.Sprintf("%d", replicaIndex),
	})
	for _, c := range existing {
		_ = s.container.Stop(ctx, c.ID, 10)
		_ = s.container.Remove(ctx, c.ID)
	}

	id, err := s.container.CreateAndStart(ctx, spec)
	if err != nil {
		return "", status.Errorf(codes.Internal, "deploy %q replica %d locally: %v", name, replicaIndex, err)
	}

	slog.Info("replica started", "service", name, "replica", replicaIndex, "id", container.ShortID(id))
	return id, nil
}

// deployRemoteReplica sends a StartContainer RPC for a single replica to a remote node.
func (s *Server) deployRemoteReplica(ctx context.Context, name string, replicaIndex int, svcDef hivefile.ServiceDef, env map[string]string, targetNode string) (string, error) {
	if s.mesh == nil {
		return "", status.Error(codes.FailedPrecondition, "mesh not initialized for remote deploy")
	}

	peer, err := s.mesh.PeerByName(targetNode)
	if err != nil {
		return "", status.Errorf(codes.NotFound, "target node %q not reachable: %v", targetNode, err)
	}

	// Use indexed name so the remote node creates a uniquely named container
	replicaName := fmt.Sprintf("%s-%d", name, replicaIndex)

	svcProto := &hivev1.Service{
		Name:  replicaName,
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

	// Send secrets separately
	secretRefs := hivefile.ExtractSecretRefs(svcDef)

	// Refuse to transit secrets over unencrypted mesh connections
	if !pki.HasNodeCert(s.dataDir) && len(secretRefs) > 0 {
		return "", status.Errorf(codes.FailedPrecondition, "cannot send secrets to remote node: mTLS not active")
	}

	secretBytes := make(map[string][]byte, len(secretRefs))
	for _, ref := range secretRefs {
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
		return "", status.Errorf(codes.Internal, "deploy %q replica %d on %s: %v", name, replicaIndex, targetNode, err)
	}

	slog.Info("remote replica started", "service", name, "replica", replicaIndex, "node", targetNode, "id", container.ShortID(resp.Container.Id))
	return resp.Container.Id, nil
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

	// Fan-out to remote peers concurrently — skip containers already seen locally
	if s.mesh != nil {
		type peerResult struct {
			peerName   string
			containers []*hivev1.Container
		}

		peers := s.mesh.Peers()
		resultCh := make(chan peerResult, len(peers))

		fanoutCtx, fanoutCancel := context.WithTimeout(ctx, 10*time.Second)
		defer fanoutCancel()

		for _, peer := range peers {
			go func(peerName string) {
				peerConn, err := s.mesh.PeerByName(peerName)
				if err != nil {
					resultCh <- peerResult{peerName: peerName}
					return
				}
				peerCtx, peerCancel := context.WithTimeout(fanoutCtx, 5*time.Second)
				state, err := peerConn.MeshClient().SyncState(peerCtx, &emptypb.Empty{})
				peerCancel()
				if err != nil {
					slog.Debug("failed to sync state from peer", "peer", peerName, "error", err)
					resultCh <- peerResult{peerName: peerName}
					return
				}
				resultCh <- peerResult{peerName: peerName, containers: state.Containers}
			}(peer.Info.Name)
		}

		for range peers {
			var pr peerResult
			select {
			case pr = <-resultCh:
			case <-ctx.Done():
				return nil, status.Errorf(codes.Canceled, "client disconnected during service list fan-out")
			}
			for _, c := range pr.containers {
				if seenContainers[c.Id] {
					continue
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
						NodeConstraint:  pr.peerName,
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
		var stopErrors []string
		for _, c := range state.Containers {
			if c.ServiceName == req.Name {
				_, err := peer.MeshClient().StopContainer(ctx, &hivev1.StopContainerRequest{
					ContainerId:    c.Id,
					TimeoutSeconds: 10,
				})
				if err != nil {
					stopErrors = append(stopErrors, fmt.Sprintf("container %s: %v", c.Id, err))
				}
			}
		}

		if len(stopErrors) > 0 {
			return nil, status.Errorf(codes.Internal, "failed to stop all containers on %q: %s", placement, strings.Join(stopErrors, "; "))
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

// ScaleService changes the replica count for a running service.
// Scale up: creates additional replicas. Scale down: stops excess replicas.
func (s *Server) ScaleService(ctx context.Context, req *hivev1.ScaleServiceRequest) (*emptypb.Empty, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "service name is required")
	}
	if req.Replicas == 0 {
		return nil, status.Error(codes.InvalidArgument, "replica count must be at least 1 — use StopService to remove a service")
	}

	s.deployMu.Lock()
	defer s.deployMu.Unlock()

	// Load service definition from store
	svcData, err := s.store.Get("services", req.Name)
	if err != nil || svcData == nil {
		return nil, status.Errorf(codes.NotFound, "service %q not found", req.Name)
	}
	var svcDef hivefile.ServiceDef
	if err := json.Unmarshal(svcData, &svcDef); err != nil {
		return nil, status.Errorf(codes.Internal, "corrupt service definition for %q: %v", req.Name, err)
	}

	// Count current local replicas
	containers, err := s.container.ListContainers(ctx, map[string]string{
		"hive.managed": "true",
		"hive.service": req.Name,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list containers: %v", err)
	}
	currentCount := len(containers)
	desired := int(req.Replicas)

	slog.Info("scaling service", "name", req.Name, "current", currentCount, "desired", desired)

	if desired > currentCount {
		// Scale up — load secrets and create additional replicas
		secretKeys, _ := s.store.List("secrets")
		secrets := make(map[string]string, len(secretKeys))
		for _, key := range secretKeys {
			val, getErr := s.store.Get("secrets", key)
			if getErr == nil && val != nil && s.vault != nil {
				if decrypted, decErr := s.vault.Decrypt(val); decErr == nil {
					secrets[key] = string(decrypted)
				}
			}
		}
		env, _ := hivefile.ResolveEnv(svcDef.Env, secrets)
		memBytes, _ := hivefile.ParseMemory(svcDef.Resources.Memory)

		if pullErr := s.container.PullImage(ctx, svcDef.Image); pullErr != nil {
			slog.Warn("image pull failed (may be local)", "image", svcDef.Image, "error", pullErr)
		}

		for i := currentCount; i < desired; i++ {
			targetNode := s.nodeName
			if s.scheduler != nil {
				if candidate, pickErr := s.scheduler.Pick(svcDef); pickErr == nil {
					targetNode = candidate.NodeName
				}
			}
			if targetNode == s.nodeName {
				if _, deployErr := s.deployLocalReplica(ctx, req.Name, i, svcDef, env, memBytes); deployErr != nil {
					slog.Error("failed to scale up replica", "service", req.Name, "replica", i, "error", deployErr)
				}
			} else {
				if _, deployErr := s.deployRemoteReplica(ctx, req.Name, i, svcDef, env, targetNode); deployErr != nil {
					slog.Error("failed to scale up remote replica", "service", req.Name, "replica", i, "error", deployErr)
				}
			}
		}
	} else if desired < currentCount {
		// Sort containers by replica index so we remove highest indices first
		sort.Slice(containers, func(a, b int) bool {
			ra := containers[a].Labels["hive.replica"]
			rb := containers[b].Labels["hive.replica"]
			// Parse as integers for proper numeric sorting
			var ia, ib int
			fmt.Sscanf(ra, "%d", &ia)
			fmt.Sscanf(rb, "%d", &ib)
			return ia < ib
		})

		// Scale down — stop excess replicas (highest indices first)
		for i := currentCount - 1; i >= desired; i-- {
			c := containers[i]
			slog.Info("scaling down, stopping replica", "service", req.Name, "container", container.ShortID(c.ID))
			_ = s.container.Stop(ctx, c.ID, 10)
			if removeErr := s.container.Remove(ctx, c.ID); removeErr != nil {
				slog.Error("failed to remove container during scale-down", "id", c.ID, "error", removeErr)
			}
		}
	}

	// Update stored service definition with new replica count
	svcDef.Replicas = desired
	if svcJSON, marshalErr := json.Marshal(svcDef); marshalErr == nil {
		_ = s.store.Put("services", req.Name, svcJSON)
	}

	slog.Info("service scaled", "name", req.Name, "replicas", desired)
	return &emptypb.Empty{}, nil
}

// RollbackService rolls back a service to its previous version by redeploying
// the archived service definition from service_history.
// Note: only one previous version is retained per service. Rolling back twice
// ping-pongs between the two most recent versions (current <-> previous).
func (s *Server) RollbackService(ctx context.Context, req *hivev1.RollbackServiceRequest) (*emptypb.Empty, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "service name is required")
	}

	s.deployMu.Lock()
	defer s.deployMu.Unlock()

	// Load previous version from history
	prevData, err := s.store.Get("service_history", req.Name)
	if err != nil || prevData == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "no previous version of %q to roll back to", req.Name)
	}
	var prevDef hivefile.ServiceDef
	if err := json.Unmarshal(prevData, &prevDef); err != nil {
		return nil, status.Errorf(codes.Internal, "corrupt service history for %q: %v", req.Name, err)
	}

	slog.Info("rolling back service", "name", req.Name, "image", prevDef.Image)

	// Stop remote containers for this service
	if s.mesh != nil {
		for _, peer := range s.mesh.Peers() {
			peerConn, err := s.mesh.PeerByName(peer.Info.Name)
			if err != nil {
				continue
			}
			state, err := peerConn.MeshClient().SyncState(ctx, &emptypb.Empty{})
			if err != nil {
				continue
			}
			for _, c := range state.Containers {
				if c.ServiceName == req.Name {
					_, _ = peerConn.MeshClient().StopContainer(ctx, &hivev1.StopContainerRequest{
						ContainerId: c.Id, TimeoutSeconds: 10,
					})
				}
			}
		}
	}

	// Stop all current local containers for this service
	containers, err := s.container.ListContainers(ctx, map[string]string{
		"hive.managed": "true",
		"hive.service": req.Name,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list containers: %v", err)
	}
	for _, c := range containers {
		_ = s.container.Stop(ctx, c.ID, 10)
		_ = s.container.Remove(ctx, c.ID)
	}

	// Resolve secrets for the previous definition
	secretKeys, _ := s.store.List("secrets")
	secrets := make(map[string]string, len(secretKeys))
	for _, key := range secretKeys {
		val, getErr := s.store.Get("secrets", key)
		if getErr == nil && val != nil && s.vault != nil {
			if decrypted, decErr := s.vault.Decrypt(val); decErr == nil {
				secrets[key] = string(decrypted)
			}
		}
	}
	env, _ := hivefile.ResolveEnv(prevDef.Env, secrets)
	memBytes, _ := hivefile.ParseMemory(prevDef.Resources.Memory)

	if pullErr := s.container.PullImage(ctx, prevDef.Image); pullErr != nil {
		slog.Warn("image pull failed (may be local)", "image", prevDef.Image, "error", pullErr)
	}

	// Redeploy all replicas with the previous definition
	replicas := prevDef.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	// Archive current version as history (swap)
	if current, _ := s.store.Get("services", req.Name); current != nil {
		_ = s.store.Put("service_history", req.Name, current)
	}

	replicasStarted := 0
	for i := 0; i < replicas; i++ {
		// TODO: Use scheduler to distribute across nodes (currently local-only)
		if _, deployErr := s.deployLocalReplica(ctx, req.Name, i, prevDef, env, memBytes); deployErr != nil {
			slog.Error("failed to deploy rollback replica", "service", req.Name, "replica", i, "error", deployErr)
		} else {
			replicasStarted++
		}
	}

	if replicasStarted == 0 {
		return nil, status.Errorf(codes.Internal, "rollback failed: no replicas started for %q", req.Name)
	}

	// Persist the rolled-back definition as current
	if svcJSON, marshalErr := json.Marshal(prevDef); marshalErr == nil {
		_ = s.store.Put("services", req.Name, svcJSON)
	}

	slog.Info("service rolled back", "name", req.Name, "image", prevDef.Image, "replicas", replicas)
	return &emptypb.Empty{}, nil
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
			return container.StreamDockerLogs(reader, func(line, streamType string) error {
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
		// Only fall through to remote if the container wasn't found locally.
		// For other errors (permission denied, runtime failure), return immediately.
		if !strings.Contains(err.Error(), "No such container") && !strings.Contains(err.Error(), "not found") {
			return status.Errorf(codes.Internal, "get logs for %s: %v", containerID, err)
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

// ExecContainer runs a command in a running container.
func (s *Server) ExecContainer(ctx context.Context, req *hivev1.ExecContainerRequest) (*hivev1.ExecContainerResponse, error) {
	if len(req.Command) == 0 {
		return nil, status.Error(codes.InvalidArgument, "command is required")
	}

	// Resolve container ID from service name if needed
	containerID := req.ContainerId
	if containerID == "" && req.ServiceName != "" {
		containers, err := s.container.ListContainers(ctx, map[string]string{
			"hive.managed": "true",
			"hive.service": req.ServiceName,
		})
		if err == nil && len(containers) > 0 {
			containerID = containers[0].ID
		}
	}
	if containerID == "" {
		return nil, status.Error(codes.InvalidArgument, "container_id or service_name is required")
	}

	// Verify the target container is Hive-managed when specified by ID
	if containerID != "" {
		info, err := s.container.Inspect(ctx, containerID)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "container %q not found: %v", containerID, err)
		}
		if info.Labels["hive.managed"] != "true" {
			return nil, status.Errorf(codes.PermissionDenied, "container %q is not managed by Hive", containerID)
		}
	}

	result, err := s.container.Exec(ctx, containerID, req.Command)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "exec: %v", err)
	}

	return &hivev1.ExecContainerResponse{
		ExitCode: int32(result.ExitCode),
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
	}, nil
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
		Id:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Type:      hivev1.EventType_EVENT_TYPE_NODE_JOINED,
		Source:    s.nodeName,
		Message:   fmt.Sprintf("Connected to %s", s.nodeName),
		Timestamp: timestamppb.Now(),
	})
	if err != nil {
		return err
	}

	// Forward mesh events to the stream until the client disconnects
	if s.mesh != nil {
		eventCh := s.mesh.Events()
		for {
			select {
			case <-stream.Context().Done():
				return nil
			case ev, ok := <-eventCh:
				if !ok {
					return nil
				}
				evType := hivev1.EventType_EVENT_TYPE_UNSPECIFIED
				switch ev.Type {
				case mesh.EventNodeJoined:
					evType = hivev1.EventType_EVENT_TYPE_NODE_JOINED
				case mesh.EventNodeLeft:
					evType = hivev1.EventType_EVENT_TYPE_NODE_LEFT
				case mesh.EventNodeFailed:
					evType = hivev1.EventType_EVENT_TYPE_NODE_FAILED
				}
				if err := stream.Send(&hivev1.Event{
					Id:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
					Type:      evType,
					Source:    ev.Node,
					Message:   fmt.Sprintf("Node %s: %v", ev.Node, ev.Type),
					Timestamp: timestamppb.Now(),
				}); err != nil {
					return err
				}
			}
		}
	}

	<-stream.Context().Done()
	return nil
}

// bootstrapNodeCert generates a CSR and gets it signed by a peer that holds the CA key.
func (s *Server) bootstrapNodeCert(joinToken string) error {
	if s.mesh == nil {
		return fmt.Errorf("mesh not initialized")
	}
	local := s.mesh.LocalNode()
	csrPEM, keyPEM, err := pki.GenerateCSR(local.Name, local.AdvertiseAddr)
	if err != nil {
		return fmt.Errorf("generate CSR: %w", err)
	}

	// Try each known peer to find one that can sign (holds the CA key)
	for _, peer := range s.mesh.Peers() {
		peerConn, err := s.mesh.PeerByName(peer.Info.Name)
		if err != nil {
			continue
		}
		resp, err := peerConn.MeshClient().SignNodeCSR(context.Background(), &hivev1.SignCSRRequest{
			CsrPem:    csrPEM,
			NodeName:  local.Name,
			JoinToken: joinToken,
		})
		if err != nil {
			slog.Debug("peer cannot sign CSR", "peer", peer.Info.Name, "error", err)
			continue
		}
		if err := pki.SaveCACert(s.dataDir, resp.CaCertPem); err != nil {
			return fmt.Errorf("save CA cert: %w", err)
		}
		if err := pki.SaveNodeCert(s.dataDir, resp.NodeCertPem, keyPEM); err != nil {
			return fmt.Errorf("save node cert: %w", err)
		}
		slog.Info("node certificate bootstrapped via CSR", "signed_by", peer.Info.Name)
		return nil
	}
	return fmt.Errorf("no peer could sign the CSR — the init node may be unreachable")
}

// splitVolume splits "source:target" volume strings.
func splitVolume(s string) []string {
	if len(s) >= 3 && s[1] == ':' && ((s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= 'a' && s[0] <= 'z')) {
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
