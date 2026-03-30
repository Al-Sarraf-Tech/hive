// Package api implements the gRPC API server for hived.
package api

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	hivev1 "github.com/jalsarraf0/hive/daemon/internal/api/gen/hive/v1"
	"github.com/jalsarraf0/hive/daemon/internal/appstore"
	"github.com/jalsarraf0/hive/daemon/internal/backup"
	"github.com/jalsarraf0/hive/daemon/internal/container"
	"github.com/jalsarraf0/hive/daemon/internal/cron"
	"github.com/jalsarraf0/hive/daemon/internal/health"
	"github.com/jalsarraf0/hive/daemon/internal/hivefile"
	"github.com/jalsarraf0/hive/daemon/internal/hooks"
	"github.com/jalsarraf0/hive/daemon/internal/joincode"
	"github.com/jalsarraf0/hive/daemon/internal/mesh"
	"github.com/jalsarraf0/hive/daemon/internal/metrics"
	"github.com/jalsarraf0/hive/daemon/internal/pki"
	"github.com/jalsarraf0/hive/daemon/internal/scheduler"
	"github.com/jalsarraf0/hive/daemon/internal/secrets"
	"github.com/jalsarraf0/hive/daemon/internal/store"
	"github.com/jalsarraf0/hive/daemon/internal/sysinfo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements the HiveAPI gRPC service.
// validServiceName restricts service names to safe characters for labels, store keys, and container names.
var validServiceName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,62}$`)

// warnErr logs a warning if err is non-nil. Used for best-effort operations
// (container cleanup, store metadata) where failure should be visible but not fatal.
func warnErr(err error, msg string, args ...any) {
	if err != nil {
		slog.Warn(msg, append(args, "error", err)...)
	}
}

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
	cronSched       *cron.Scheduler // nil if no cron jobs
	healthHistory   *health.History // nil if health timeline disabled
	hookMgr         *hooks.Manager  // nil if no hooks configured
	proxyMgr        proxyManager    // nil if ingress disabled
	appStore        *appstore.Store          // nil if app store disabled
	registryMgr     *appstore.RegistryManager // nil if registry manager disabled
	deployMu        sync.Mutex      // serializes DeployService to prevent concurrent races
	certBootstrapMu sync.Mutex      // serializes bootstrapNodeCert to prevent concurrent CSR signing
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

// SetCronScheduler sets the cron scheduler for ListCronJobs.
func (s *Server) SetCronScheduler(cs *cron.Scheduler) {
	s.cronSched = cs
}

// SetHealthHistory sets the health event history for the timeline API.
func (s *Server) SetHealthHistory(h *health.History) {
	s.healthHistory = h
}

// proxyManager is the interface for the ingress proxy manager.
// Using an interface avoids a direct import of the proxy package.
type proxyManager interface {
	EnsureProxy(ctx context.Context, serviceName string, svcDef hivefile.ServiceDef, networkName string) error
	RefreshUpstreams(ctx context.Context, serviceName string) error
	RemoveProxy(ctx context.Context, serviceName string) error
}

// SetProxyManager sets the ingress proxy manager.
func (s *Server) SetProxyManager(pm proxyManager) {
	s.proxyMgr = pm
}

// SetAppStore sets the app catalog store.
func (s *Server) SetAppStore(as *appstore.Store) {
	s.appStore = as
}

// SetRegistryManager sets the registry credential manager.
func (s *Server) SetRegistryManager(rm *appstore.RegistryManager) {
	s.registryMgr = rm
}

// SetHookManager sets the webhook hook manager for lifecycle event delivery.
func (s *Server) SetHookManager(m *hooks.Manager) {
	s.hookMgr = m
}

// HookManager returns the hook manager (used by health loop for health-fail events).
func (s *Server) HookManager() *hooks.Manager {
	return s.hookMgr
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
	var wgPubKey, wgAddr string
	if s.mesh != nil {
		local := s.mesh.LocalNode()
		nodeStatus = hivev1.NodeStatus(local.Status)
		advertiseAddr = local.AdvertiseAddr
		grpcPort = uint32(local.GRPCPort)
		wgPubKey = local.WGPubKey
		wgAddr = local.WGAddr
	}

	memTotal, memAvail := sysinfo.MemInfo()
	diskTotal, diskAvail := sysinfo.DiskInfo(s.dataDir)

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
		Resources: &hivev1.NodeResources{
			CpuCores:             sysinfo.CPUCount(),
			MemoryTotalBytes:     memTotal,
			MemoryAvailableBytes: memAvail,
			DiskTotalBytes:       diskTotal,
			DiskAvailableBytes:   diskAvail,
		},
		JoinedAt: timestamppb.New(s.startedAt),
		WgPubKey: wgPubKey,
		WgAddr:   wgAddr,
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
	var encryptFn func([]byte) ([]byte, error)
	if s.vault != nil {
		encryptFn = s.vault.Encrypt
	}
	if err := pki.SaveCA(s.dataDir, caCertPEM, caKeyPEM, encryptFn); err != nil {
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

	// Generate short human-readable join code and persist it alongside the gossip address.
	gossipAddr := fmt.Sprintf("%s:%d", local.AdvertiseAddr, s.mesh.GossipPort())
	jc, err := joincode.Encode(joinToken)
	if err != nil {
		slog.Warn("failed to generate join code", "error", err)
		jc = "" // non-fatal — cluster still works without a join code
	} else {
		if err := s.store.Put("meta", "join_code", []byte(jc)); err != nil {
			slog.Error("failed to persist join code", "error", err)
			jc = ""
		}
		if err := s.store.Put("meta", "join_code_addr", []byte(gossipAddr)); err != nil {
			slog.Error("failed to persist join code address", "error", err)
		}
		slog.Info("join code generated")
	}

	return &hivev1.InitClusterResponse{
		ClusterId:     clusterName,
		NodeName:      local.Name,
		GossipAddr:    gossipAddr,
		CaFingerprint: pki.CACertFingerprint(caCert),
		JoinToken:     joinToken,
		JoinCode:      jc,
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

	// Bootstrap node certificate via CSR signing if not already provisioned.
	// Lock is scoped to just the bootstrap block to avoid holding it during
	// the remaining work (building node list, querying peers, etc.).
	if !pki.HasNodeCert(s.dataDir) {
		func() {
			s.certBootstrapMu.Lock()
			defer s.certBootstrapMu.Unlock()
			// Re-check under lock to avoid TOCTOU
			if !pki.HasNodeCert(s.dataDir) {
				if err := s.bootstrapNodeCert(req.JoinToken); err != nil {
					slog.Warn("node certificate bootstrap failed — mTLS will not be active until resolved", "error", err)
				}
			}
		}()
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

	localRunning := 0
	for _, c := range containers {
		if c.Status == "running" {
			localRunning++
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
	// Peer container counts come from gossip metadata, which is updated by each
	// node's health loop counting only running containers. Safe to sum directly.
	totalRunning := localRunning
	if s.mesh != nil {
		for _, peer := range s.mesh.Peers() {
			nodes = append(nodes, peerToNode(peer.Info))
			totalNodes++
			if peer.Info.Status == int(mesh.NodeStatusReady) {
				healthyNodes++
			}
			totalRunning += peer.Info.Containers
		}
	}

	metrics.NodeCount.Set(float64(totalNodes))
	metrics.ServiceCount.Set(float64(len(serviceNames)))

	// Populate containers per node from gossip metadata
	containersPerNode := make(map[string]uint32)
	containersPerNode[s.nodeName] = uint32(localRunning)
	if s.mesh != nil {
		for _, peer := range s.mesh.Peers() {
			containersPerNode[peer.Info.Name] = uint32(peer.Info.Containers)
		}
	}

	return &hivev1.ClusterStatusResponse{
		TotalNodes:        totalNodes,
		HealthyNodes:      healthyNodes,
		TotalServices:     uint32(len(serviceNames)),
		RunningContainers: uint32(totalRunning),
		Nodes:             nodes,
		ContainersPerNode: containersPerNode,
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
		Id:            info.Name,
		Name:          info.Name,
		AdvertiseAddr: info.AdvertiseAddr,
		GrpcPort:      uint32(info.GRPCPort),
		Status:        hivev1.NodeStatus(info.Status),
		Resources: &hivev1.NodeResources{
			CpuCores:             info.CPUCores,
			MemoryTotalBytes:     info.MemTotal,
			MemoryAvailableBytes: info.MemAvail,
			DiskTotalBytes:       info.DiskTotal,
			DiskAvailableBytes:   info.DiskAvail,
		},
		Capabilities: &hivev1.NodeCapabilities{
			Os:               info.OS,
			Arch:             info.Arch,
			Platforms:        info.Platforms,
			ContainerRuntime: info.Runtime,
		},
		WgPubKey: info.WGPubKey,
		WgAddr:   info.WGAddr,
	}
}

// DrainNode drains a node: marks it as draining (stops new scheduling),
// then migrates all running containers to other available nodes.
func (s *Server) DrainNode(ctx context.Context, req *hivev1.DrainNodeRequest) (*emptypb.Empty, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "node name is required")
	}
	if req.Name != s.nodeName {
		return nil, status.Error(codes.Unimplemented, "remote drain not supported — run from the node being drained")
	}

	s.deployMu.Lock()
	defer s.deployMu.Unlock()

	// Mark node as draining — scheduler will skip this node
	if s.mesh != nil {
		s.mesh.SetStatus(int(mesh.NodeStatusDraining))
	}
	slog.Info("node drain started", "node", req.Name)

	// List all local managed containers
	containers, err := s.container.ListContainers(ctx, map[string]string{"hive.managed": "true"})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list containers: %v", err)
	}
	if len(containers) == 0 {
		slog.Info("drain complete — no containers to migrate")
		return &emptypb.Empty{}, nil
	}

	migrated, failed := 0, 0
	for _, c := range containers {
		svcName := c.Labels["hive.service"]
		replicaLabel := c.Labels["hive.replica"]
		if svcName == "" {
			continue
		}

		var svcDef hivefile.ServiceDef
		if data, _ := s.store.Get("services", svcName); data != nil {
			if err := json.Unmarshal(data, &svcDef); err != nil {
				slog.Warn("corrupt service definition, skipping migration", "service", svcName, "error", err)
				failed++
				continue
			}
		}

		if s.scheduler == nil || s.mesh == nil {
			slog.Warn("cannot migrate — no scheduler/mesh", "service", svcName)
			failed++
			continue
		}

		candidate, pickErr := s.scheduler.Pick(svcDef)
		if pickErr != nil || candidate.Local {
			slog.Warn("no remote node for migration", "service", svcName, "error", pickErr)
			failed++
			continue
		}

		// Resolve env
		secretKeys, _ := s.store.List("secrets")
		secrets := make(map[string]string, len(secretKeys))
		for _, key := range secretKeys {
			val, _ := s.store.Get("secrets", key)
			if val != nil {
				if s.vault != nil {
					if dec, decErr := s.vault.Decrypt(val); decErr == nil {
						secrets[key] = string(dec)
					} else {
						slog.Warn("drain: failed to decrypt secret for migrated service, secret will be missing", "service", svcName, "key", key, "error", decErr)
					}
				} else {
					secrets[key] = string(val)
				}
			}
		}
		env, _ := hivefile.ResolveEnv(svcDef.Env, secrets)

		replicaIdx := 0
		if replicaLabel != "" {
			fmt.Sscanf(replicaLabel, "%d", &replicaIdx)
		}

		slog.Info("migrating", "service", svcName, "replica", replicaIdx, "to", candidate.NodeName)

		if _, deployErr := s.deployRemoteReplica(ctx, svcName, replicaIdx, svcDef, env, candidate.NodeName); deployErr != nil {
			slog.Error("migration failed", "service", svcName, "error", deployErr)
			failed++
			continue
		}

		if stopErr := s.container.Stop(ctx, c.ID, 10); stopErr != nil {
			slog.Warn("failed to stop old container during drain", "id", container.ShortID(c.ID), "error", stopErr)
		}
		if rmErr := s.container.Remove(ctx, c.ID); rmErr != nil {
			slog.Warn("failed to remove old container during drain", "id", container.ShortID(c.ID), "error", rmErr)
		}
		if err := s.store.SetPlacement(svcName, candidate.NodeName); err != nil {
			slog.Warn("failed to update placement after drain", "service", svcName, "error", err)
		}
		migrated++
	}

	slog.Info("drain complete", "migrated", migrated, "failed", failed)

	if migrated == 0 && failed > 0 {
		// All migrations failed — restore node to Ready so it remains functional
		if s.mesh != nil {
			s.mesh.SetStatus(int(mesh.NodeStatusReady))
		}
		return nil, status.Errorf(codes.Internal, "drain failed: all %d container migrations failed", failed)
	}

	// Drain complete — mark node as Down.
	// Even with partial failures, the node should be marked Down so it does not
	// remain stuck in Draining indefinitely. Containers that failed to migrate
	// remain running locally; the operator is informed via the log above.
	if s.mesh != nil {
		s.mesh.SetStatus(int(mesh.NodeStatusDown))
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
					return nil, status.Errorf(codes.Internal, "failed to decrypt secret %q: %v", key, err)
				}
				secrets[key] = string(decrypted)
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
	if len(deployOrder) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "hivefile contains no services")
	}
	slog.Debug("deploy order resolved", "order", deployOrder)

	// Create an isolated Docker network for this deployment.
	// All services in the same Hivefile share a network; separate Hivefiles are isolated.
	networkName := "hive-" + deployOrder[0]
	if len(deployOrder) > 1 {
		// SHA-256 hash of sorted service names for a short, deterministic, collision-resistant name
		h := sha256.New()
		for _, n := range deployOrder {
			h.Write([]byte(n))
			h.Write([]byte{0}) // null separator to avoid "ab"+"c" == "a"+"bc"
		}
		networkName = "hive-" + hex.EncodeToString(h.Sum(nil))[:12]
	}
	if _, netErr := s.container.CreateNetwork(ctx, networkName); netErr != nil {
		slog.Warn("failed to create deployment network", "network", networkName, "error", netErr)
		networkName = "" // fall back to default bridge
	} else {
		slog.Info("deployment network created", "network", networkName)
		// Persist network name for each service so StopService can clean it up
		for _, svcName := range deployOrder {
			if err := s.store.Put("meta", "network:"+svcName, []byte(networkName)); err != nil {
				slog.Warn("failed to persist network mapping", "service", svcName, "error", err)
			}
		}
	}

	var deployed []*hivev1.Service
	for _, name := range deployOrder {
		svcDef := hf.Service[name]

		// Resolve env with secrets — fail if any secret references are unresolved.
		env, err := hivefile.ResolveEnv(svcDef.Env, secrets)
		if err != nil {
			return nil, status.Errorf(codes.FailedPrecondition, "service %q: %v — set missing secrets with 'hive secret set'", name, err)
		}

		// Inject service discovery env vars for depends_on services.
		// Topo sort guarantees dependencies are already deployed, so placements exist.
		for _, depName := range svcDef.DependsOn.Services {
			upperName := strings.ToUpper(strings.ReplaceAll(depName, "-", "_"))
			hostKey := "HIVE_SERVICE_" + upperName + "_HOST"
			portKey := "HIVE_SERVICE_" + upperName + "_PORT"

			// Only set if user has not already defined these env vars
			if _, exists := env[hostKey]; !exists {
				// When services share a Docker network, use the service name as host —
				// Docker's built-in DNS resolves network aliases automatically.
				// Fall back to IP-based discovery for cross-node or no-network cases.
				if networkName != "" {
					env[hostKey] = depName
				} else {
					host := "127.0.0.1"
					if placement := s.store.GetPlacement(depName); placement != "" && placement != s.nodeName && s.mesh != nil {
						for _, peer := range s.mesh.Peers() {
							if peer.Info.Name == placement {
								host = peer.Info.AdvertiseAddr
								break
							}
						}
					} else if s.mesh != nil {
						local := s.mesh.LocalNode()
						if local.AdvertiseAddr != "" {
							host = local.AdvertiseAddr
						}
					}
					env[hostKey] = host
				}
			}

			if _, exists := env[portKey]; !exists {
				if depDef, ok := hf.Service[depName]; ok && len(depDef.Ports) > 0 {
					// When using Docker network DNS, inject the CONTAINER port
					// (traffic goes directly container-to-container on the network).
					// When using IP-based discovery, inject the HOST port
					// (traffic hits the host's port mapping).
					hostPorts := make([]string, 0, len(depDef.Ports))
					for k := range depDef.Ports {
						hostPorts = append(hostPorts, k)
					}
					sort.Strings(hostPorts)
					if networkName != "" {
						env[portKey] = depDef.Ports[hostPorts[0]] // container port
					} else {
						env[portKey] = hostPorts[0] // host port
					}
				}
			}
		}

		// Inject ingress service discovery env vars for services with ingress configured
		for otherName, otherDef := range hf.Service {
			if otherDef.Ingress.Port == 0 || otherName == name {
				continue
			}
			upperName := strings.ToUpper(strings.ReplaceAll(otherName, "-", "_"))
			hostKey := "HIVE_INGRESS_" + upperName + "_HOST"
			portKey := "HIVE_INGRESS_" + upperName + "_PORT"
			if _, exists := env[hostKey]; !exists {
				if networkName != "" {
					env[hostKey] = otherName + "-ingress" // Docker DNS alias set by proxy manager
				} else if s.mesh != nil {
					env[hostKey] = s.mesh.LocalNode().AdvertiseAddr
				} else {
					env[hostKey] = "127.0.0.1"
				}
			}
			if _, exists := env[portKey]; !exists {
				env[portKey] = strconv.Itoa(otherDef.Ingress.Port)
			}
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
		if err := s.container.PullImage(ctx, svcDef.Image, nil); err != nil {
			slog.Warn("image pull failed (may be local)", "image", svcDef.Image, "error", err)
		}

		// Deploy N replicas, distributing across nodes via scheduler
		replicas := svcDef.Replicas
		if replicas <= 0 {
			replicas = 1
		}

		// Archive previous version for rollback BEFORE deploying (ensures rollback is possible if deploy crashes)
		if prev, err := s.store.Get("services", name); err != nil {
			slog.Warn("failed to read previous service definition for rollback history", "service", name, "error", err)
		} else if prev != nil {
			if err := s.store.Put("service_history", name, prev); err != nil {
				slog.Warn("failed to archive service version for rollback", "service", name, "error", err)
			}
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
		primaryNode := s.nodeName // track the node of the first successful replica for placement

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

				// If the replica moved to a remote node, stop the old local
				// container for this replica index first. deployLocalReplica
				// handles this internally, but deployRemoteReplica does not.
				if targetNode != s.nodeName {
					oldLocal, _ := s.container.ListContainers(ctx, map[string]string{
						"hive.managed": "true",
						"hive.service": name,
						"hive.replica": fmt.Sprintf("%d", i),
					})
					for _, old := range oldLocal {
						slog.Info("rolling update: stopping old local container (replica moved remote)", "service", name, "replica", i, "id", container.ShortID(old.ID))
						warnErr(s.container.Stop(ctx, old.ID, 10), "failed to stop container")
						warnErr(s.container.Remove(ctx, old.ID), "failed to remove container")
					}
				}

				var id string
				replicaEnv := cloneEnv(env)
				if targetNode == s.nodeName {
					id, err = s.deployLocalReplica(ctx, name, i, svcDef, replicaEnv, memBytes, networkName)
				} else {
					id, err = s.deployRemoteReplica(ctx, name, i, svcDef, replicaEnv, targetNode)
				}
				if err != nil {
					slog.Error("rolling update: replica failed", "service", name, "replica", i, "error", err)
					continue
				}

				containerIDs = append(containerIDs, id)
				replicasRunning++
				if replicasRunning == 1 {
					primaryNode = targetNode
				}

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
				// Re-query to get current containers (some may already have been replaced by the rolling update)
				currentContainers, _ := s.container.ListContainers(ctx, map[string]string{
					"hive.managed": "true",
					"hive.service": name,
				})
				if len(currentContainers) > replicas {
					// Sort by replica index and remove highest
					sort.Slice(currentContainers, func(a, b int) bool {
						ra, rb := currentContainers[a].Labels["hive.replica"], currentContainers[b].Labels["hive.replica"]
						var ia, ib int
						fmt.Sscanf(ra, "%d", &ia)
						fmt.Sscanf(rb, "%d", &ib)
						return ia < ib
					})
					for i := replicas; i < len(currentContainers); i++ {
						warnErr(s.container.Stop(ctx, currentContainers[i].ID, 10), "failed to stop container")
						warnErr(s.container.Remove(ctx, currentContainers[i].ID), "failed to remove container")
					}
				}
			}
		} else if isUpdate && strategy == "blue-green" {
			// Blue-green: deploy new (green) replicas first, health check, THEN stop old (blue).
			// This avoids the downtime window that recreate has.
			slog.Info("blue-green deployment", "service", name, "existing", len(existingContainers), "desired", replicas)

			// Phase 1: Deploy green replicas with offset indices to avoid name/port conflicts with blue set.
			greenOffset := replicas
			var greenIDs []string
			for i := 0; i < replicas; i++ {
				greenIdx := greenOffset + i
				targetNode := s.nodeName
				if s.scheduler != nil {
					if candidate, pickErr := s.scheduler.Pick(svcDef); pickErr == nil {
						targetNode = candidate.NodeName
					}
				}
				var id string
				var deployErr error
				replicaEnv := cloneEnv(env)
				if targetNode == s.nodeName {
					id, deployErr = s.deployLocalReplica(ctx, name, greenIdx, svcDef, replicaEnv, memBytes, networkName)
				} else {
					id, deployErr = s.deployRemoteReplica(ctx, name, greenIdx, svcDef, replicaEnv, targetNode)
				}
				if deployErr != nil {
					slog.Error("blue-green: failed to deploy green replica", "service", name, "replica", i, "error", deployErr)
					continue
				}
				greenIDs = append(greenIDs, id)
			}

			// Phase 2: Health check the green set.
			greenHealthy := len(greenIDs) == replicas
			if greenHealthy && svcDef.Health.Type != "" && svcDef.Health.Port > 0 {
				checkTimeout := 5 * time.Second
				if d, parseErr := time.ParseDuration(svcDef.Health.Timeout); parseErr == nil && d > 0 {
					checkTimeout = d
				}
				maxChecks := 10
				for gi, gid := range greenIDs {
					healthy := false
					for check := 0; check < maxChecks; check++ {
						select {
						case <-ctx.Done():
							greenHealthy = false
							slog.Error("blue-green: context cancelled during health check", "service", name, "replica", gi)
							break
						case <-time.After(checkTimeout):
						}
						if !greenHealthy {
							break
						}
						// Offset health port for green replicas (they use offset host ports)
						greenHealthPort := svcDef.Health.Port
						for hostPort := range svcDef.Ports {
							if hp, pErr := strconv.Atoi(hostPort); pErr == nil && hp == svcDef.Health.Port {
								greenHealthPort = hp + greenOffset + gi
								break
							}
						}
						result := s.health.Check(ctx, health.Config{
							Type:    health.CheckType(svcDef.Health.Type),
							Host:    "127.0.0.1",
							Port:    greenHealthPort,
							Path:    svcDef.Health.Path,
							Timeout: checkTimeout,
						})
						if result.Healthy {
							healthy = true
							slog.Info("blue-green: green replica healthy", "service", name, "replica", gi, "check", check+1)
							break
						}
						slog.Debug("blue-green: health check pending", "service", name, "replica", gi, "check", check+1, "message", result.Message)
					}
					if !healthy {
						slog.Error("blue-green: green replica failed health check", "service", name, "replica", gi, "id", container.ShortID(gid))
						greenHealthy = false
						break
					}
				}
			}

			// Phase 3: Swap or rollback.
			if greenHealthy {
				// Green set is healthy — remove blue (old) containers.
				slog.Info("blue-green: green set healthy, swapping to final replicas", "service", name)
				// Order: stop blue (green still serves) → deploy final → stop green
				// This ensures at least one set is running at all times.
				for _, old := range existingContainers {
					warnErr(s.container.Stop(ctx, old.ID, 10), "failed to stop container")
					warnErr(s.container.Remove(ctx, old.ID), "failed to remove container")
				}
				// Deploy final replicas with correct indices 0..N-1
				for i := 0; i < replicas; i++ {
					targetNode := s.nodeName
					if s.scheduler != nil {
						if candidate, pickErr := s.scheduler.Pick(svcDef); pickErr == nil {
							targetNode = candidate.NodeName
						}
					}
					var id string
					replicaEnv := cloneEnv(env)
					if targetNode == s.nodeName {
						id, err = s.deployLocalReplica(ctx, name, i, svcDef, replicaEnv, memBytes, networkName)
					} else {
						id, err = s.deployRemoteReplica(ctx, name, i, svcDef, replicaEnv, targetNode)
					}
					if err != nil {
						slog.Error("blue-green: final replica deploy failed", "service", name, "replica", i, "error", err)
						continue
					}
					containerIDs = append(containerIDs, id)
					replicasRunning++
					if replicasRunning == 1 {
						primaryNode = s.nodeName
					}
				}
				// Now stop green offset containers (final replicas are serving)
				for _, gid := range greenIDs {
					warnErr(s.container.Stop(ctx, gid, 10), "failed to stop container")
					warnErr(s.container.Remove(ctx, gid), "failed to remove container")
				}
			} else {
				// Rollback: remove all green containers, keep blue (old) running.
				slog.Warn("blue-green: health check failed, rolling back", "service", name)
				for _, gid := range greenIDs {
					warnErr(s.container.Stop(ctx, gid, 10), "failed to stop container")
					warnErr(s.container.Remove(ctx, gid), "failed to remove container")
				}
				for _, old := range existingContainers {
					containerIDs = append(containerIDs, old.ID)
				}
				replicasRunning = uint32(len(existingContainers))
			}
		} else if isUpdate && strategy == "canary" {
			// Canary strategy: deploy 1 canary replica with low traffic weight,
			// health check it, then promote by replacing remaining replicas.
			// Requires ingress to be configured for weighted routing.
			if svcDef.Ingress.Port == 0 {
				return nil, status.Errorf(codes.InvalidArgument, "canary strategy requires [ingress] to be configured for service %q", name)
			}
			canaryWeight := svcDef.Deploy.CanaryWeight
			if canaryWeight <= 0 {
				canaryWeight = 10 // default 10% to canary
			}

			slog.Info("canary deploy: deploying canary replica", "service", name, "weight", canaryWeight)

			// Deploy 1 canary replica at offset index
			canaryIdx := replicas // offset to avoid conflicts
			canaryTarget := s.nodeName
			if s.scheduler != nil {
				if c, err := s.scheduler.Pick(svcDef); err == nil {
					canaryTarget = c.NodeName
				}
			}
			// Deploy canary replica (local or remote)
			var canaryID string
			if canaryTarget == s.nodeName {
				id, err := s.deployLocalReplica(ctx, name, canaryIdx, svcDef, cloneEnv(env), memBytes, networkName)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "canary deploy failed: %v", err)
				}
				canaryID = id
			} else if s.mesh != nil {
				// Deploy canary on remote node
				id, err := s.deployRemoteReplica(ctx, name, canaryIdx, svcDef, env, canaryTarget)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "canary deploy on %s failed: %v", canaryTarget, err)
				}
				canaryID = id
			}

			// Configure proxy with canary weight
			if s.proxyMgr != nil {
				if err := s.proxyMgr.EnsureProxy(ctx, name, svcDef, networkName); err != nil {
					slog.Warn("canary: failed to update proxy", "error", err)
				}
			}

			// Health check canary — use configured health check type, not hardcoded HTTP
			canaryHealthy := true
			if svcDef.Health.Port > 0 || len(svcDef.Health.ExecCommand) > 0 {
				checkTimeout := 5 * time.Second
				if d, parseErr := time.ParseDuration(svcDef.Health.Timeout); parseErr == nil && d > 0 {
					checkTimeout = d
				}
				canaryHealthy = false
				for attempt := 0; attempt < 10; attempt++ {
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(checkTimeout):
					}
					cfg := health.Config{
						Host: "127.0.0.1",
						Port: svcDef.Health.Port,
						Path: svcDef.Health.Path,
					}
					switch svcDef.Health.Type {
					case "http":
						cfg.Type = health.CheckHTTP
					case "tcp":
						cfg.Type = health.CheckTCP
					case "exec":
						cfg.Type = health.CheckExec
					default:
						cfg.Type = health.CheckHTTP
					}
					result := s.health.Check(ctx, cfg)
					if result.Healthy {
						canaryHealthy = true
						break
					}
				}
			}

			if canaryHealthy {
				slog.Info("canary deploy: canary healthy, promoting", "service", name)
				// Remove canary offset replica
				if canaryID != "" {
					warnErr(s.container.Stop(ctx, canaryID, 10), "failed to stop container")
					warnErr(s.container.Remove(ctx, canaryID), "failed to remove container")
				}
				// Rolling replace all existing replicas with new version
				for _, old := range existingContainers {
					replicaIdx := 0
					if label := old.Labels["hive.replica"]; label != "" {
						fmt.Sscanf(label, "%d", &replicaIdx)
					}
					warnErr(s.container.Stop(ctx, old.ID, 10), "failed to stop container")
					warnErr(s.container.Remove(ctx, old.ID), "failed to remove container")
					id, err := s.deployLocalReplica(ctx, name, replicaIdx, svcDef, cloneEnv(env), memBytes, networkName)
					if err != nil {
						slog.Error("canary promotion failed", "service", name, "replica", replicaIdx, "error", err)
						continue
					}
					containerIDs = append(containerIDs, id)
					replicasRunning++
					if replicasRunning == 1 {
						primaryNode = s.nodeName
					}
				}
			} else {
				// Rollback canary — remove canary, keep existing
				slog.Warn("canary deploy: canary failed health check, rolling back", "service", name)
				if canaryID != "" {
					warnErr(s.container.Stop(ctx, canaryID, 10), "failed to stop container")
					warnErr(s.container.Remove(ctx, canaryID), "failed to remove container")
				}
				for _, old := range existingContainers {
					containerIDs = append(containerIDs, old.ID)
				}
				replicasRunning = uint32(len(existingContainers))
			}
		} else {
			// Recreate strategy (or fresh deploy): stop all existing containers first, then deploy
			for _, c := range existingContainers {
				warnErr(s.container.Stop(ctx, c.ID, 10), "failed to stop container")
				warnErr(s.container.Remove(ctx, c.ID), "failed to remove container")
			}

			for i := 0; i < replicas; i++ {
				targetNode := s.nodeName
				if s.scheduler != nil {
					if candidate, pickErr := s.scheduler.Pick(svcDef); pickErr == nil {
						targetNode = candidate.NodeName
					}
				}

				slog.Info("deploying replica", "service", name, "replica", i, "target", targetNode)

				var id string
				replicaEnv := cloneEnv(env)
				if targetNode == s.nodeName {
					id, err = s.deployLocalReplica(ctx, name, i, svcDef, replicaEnv, memBytes, networkName)
				} else {
					id, err = s.deployRemoteReplica(ctx, name, i, svcDef, replicaEnv, targetNode)
				}
				if err != nil {
					slog.Error("failed to deploy replica", "service", name, "replica", i, "error", err)
					continue
				}

				containerIDs = append(containerIDs, id)
				replicasRunning++
				if replicasRunning == 1 {
					primaryNode = targetNode
				}
			}
		}

		if replicasRunning == 0 {
			metrics.DeployTotal.WithLabelValues("failure").Inc()
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
		if err := s.store.SetPlacement(name, primaryNode); err != nil {
			slog.Error("failed to record service placement", "service", name, "error", err)
		}

		// Persist service definition
		svcJSON, err := json.Marshal(svcDef)
		if err != nil {
			slog.Error("failed to marshal service definition", "service", name, "error", err)
		} else if err := s.store.Put("services", name, svcJSON); err != nil {
			slog.Error("failed to persist service definition", "service", name, "error", err)
		}

		// Register cron jobs from the service definition
		if s.cronSched != nil {
			for i, cj := range svcDef.Cron {
				jobName := fmt.Sprintf("%s-cron-%d", name, i)
				if err := s.cronSched.Add(jobName, cj.Schedule, name, cj.Command); err != nil {
					slog.Warn("failed to register cron job", "service", name, "schedule", cj.Schedule, "error", err)
				}
			}
		}

		slog.Info("service deployed", "name", name, "replicas", fmt.Sprintf("%d/%d", replicasRunning, replicas))
	}

	metrics.DeployTotal.WithLabelValues("success").Inc()

	// Fire post-deploy webhooks for each deployed service
	if s.hookMgr != nil {
		for _, svc := range deployed {
			s.hookMgr.Fire("post-deploy", hooks.Event{
				Service: svc.Name,
				Node:    s.nodeName,
				Message: fmt.Sprintf("deployed %s (%d/%d replicas)", svc.Image, svc.ReplicasRunning, svc.ReplicasDesired),
			})
		}
	}

	// Set up / update ingress proxies for services with ingress config
	if s.proxyMgr != nil {
		for _, name := range deployOrder {
			svcDef := hf.Service[name]
			if svcDef.Ingress.Port != 0 {
				if err := s.proxyMgr.EnsureProxy(ctx, name, svcDef, networkName); err != nil {
					slog.Error("failed to set up ingress proxy", "service", name, "error", err)
				}
			}
		}
	}

	return &hivev1.DeployServiceResponse{Services: deployed}, nil
}

// deployLocalReplica creates a single replica container on this node.
func (s *Server) deployLocalReplica(ctx context.Context, name string, replicaIndex int, svcDef hivefile.ServiceDef, env map[string]string, memBytes int64, networkName ...string) (string, error) {
	memMB := memBytes / (1024 * 1024)
	containerName := fmt.Sprintf("hive-%s-%d", name, replicaIndex)
	// Offset host ports for replicas > 0 to avoid "port already in use" conflicts.
	ports := make(map[string]string, len(svcDef.Ports))
	for hostPort, containerPort := range svcDef.Ports {
		if replicaIndex > 0 {
			hp, err := strconv.Atoi(hostPort)
			if err == nil {
				hostPort = strconv.Itoa(hp + replicaIndex)
			}
		}
		ports[hostPort] = containerPort
	}

	spec := container.ContainerSpec{
		Name:  containerName,
		Image: svcDef.Image,
		Env:   env,
		Ports: ports,
		Labels: map[string]string{
			"hive.managed": "true",
			"hive.service": name,
			"hive.replica": fmt.Sprintf("%d", replicaIndex),
		},
		MemoryMB:      memMB,
		CPUs:          svcDef.Resources.CPUs,
		RestartPolicy: svcDef.RestartPolicy,
	}
	if len(networkName) > 0 && networkName[0] != "" {
		spec.NetworkName = networkName[0]
		spec.NetworkAliases = []string{
			name,                                          // "web" — any replica resolves by service name
			fmt.Sprintf("%s-%d", name, replicaIndex),      // "web-0" — specific replica
		}
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
		"hive.managed": "true",
		"hive.service": name,
		"hive.replica":  fmt.Sprintf("%d", replicaIndex),
	})
	for _, c := range existing {
		warnErr(s.container.Stop(ctx, c.ID, 10), "failed to stop container")
		warnErr(s.container.Remove(ctx, c.ID), "failed to remove container")
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

	// Fire pre-stop webhook before doing any work
	if s.hookMgr != nil {
		s.hookMgr.Fire("pre-stop", hooks.Event{
			Service: req.Name,
			Node:    s.nodeName,
			Message: "stopping service",
		})
	}

	s.deployMu.Lock()
	defer s.deployMu.Unlock()

	// Check placement to find which node owns this service
	placement := s.store.GetPlacement(req.Name)

	if placement != "" && placement != s.nodeName && s.mesh != nil {
		// Service is on a remote node — forward the stop
		slog.Info("forwarding stop to remote node", "service", req.Name, "node", placement)
		peer, err := s.mesh.PeerByName(placement)
		if err != nil {
			// Remote node unreachable — clean up stale placement
			slog.Warn("remote node unreachable, cleaning up stale placement", "service", req.Name, "node", placement, "error", err)
			warnErr(s.store.Delete("services", req.Name), "failed to delete from store", "bucket", "services")
			warnErr(s.store.DeletePlacement(req.Name), "failed to delete placement", "service", req.Name)
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
		warnErr(s.store.Delete("services", req.Name), "failed to delete from store", "bucket", "services")
		warnErr(s.store.DeletePlacement(req.Name), "failed to delete placement", "service", req.Name)

		// Clean up cron jobs for this service
		if s.cronSched != nil {
			for _, j := range s.cronSched.List() {
				if strings.HasPrefix(j.Name, req.Name+"-cron-") {
					s.cronSched.Remove(j.Name)
				}
			}
		}

		slog.Info("service stopped on remote node", "name", req.Name, "node", placement)
		return &emptypb.Empty{}, nil
	}

	// Local stop
	containers, err := s.container.ListContainers(ctx, map[string]string{
		"hive.managed": "true",
		"hive.service": req.Name,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list containers: %v", err)
	}
	allRemoved := true
	for _, c := range containers {
		slog.Info("stopping container", "service", req.Name, "id", c.ID)
		warnErr(s.container.Stop(ctx, c.ID, 10), "failed to stop container") // graceful stop first
		if err := s.container.Remove(ctx, c.ID); err != nil {
			slog.Error("failed to remove container", "id", c.ID, "error", err)
			allRemoved = false
		}
	}
	if allRemoved {
		warnErr(s.store.Delete("services", req.Name), "failed to delete from store", "bucket", "services")
		warnErr(s.store.DeletePlacement(req.Name), "failed to delete placement", "service", req.Name)

		// Clean up Docker network if no other services use it
		if netName, _ := s.store.Get("meta", "network:"+req.Name); netName != nil {
			warnErr(s.store.Delete("meta", "network:"+req.Name), "failed to delete from store", "bucket", "meta")
			// Only remove the network if no other service references it
			networkInUse := false
			if keys, err := s.store.List("meta"); err == nil {
				for _, k := range keys {
					if strings.HasPrefix(k, "network:") && k != "network:"+req.Name {
						if val, _ := s.store.Get("meta", k); val != nil && string(val) == string(netName) {
							networkInUse = true
							break
						}
					}
				}
			}
			if !networkInUse {
				if err := s.container.RemoveNetwork(ctx, string(netName)); err != nil {
					slog.Warn("failed to remove network", "network", string(netName), "error", err)
				} else {
					slog.Info("removed deployment network", "network", string(netName))
				}
			}
		}

		// Clean up cron jobs for this service
		if s.cronSched != nil {
			for _, j := range s.cronSched.List() {
				if strings.HasPrefix(j.Name, req.Name+"-cron-") {
					s.cronSched.Remove(j.Name)
				}
			}
		}

		slog.Info("service stopped", "name", req.Name)

		// Remove ingress proxy if configured
		if s.proxyMgr != nil {
			if err := s.proxyMgr.RemoveProxy(ctx, req.Name); err != nil {
				slog.Warn("failed to remove ingress proxy", "service", req.Name, "error", err)
			}
		}
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
			if getErr == nil && val != nil {
				if s.vault != nil {
					if decrypted, decErr := s.vault.Decrypt(val); decErr == nil {
						secrets[key] = string(decrypted)
					}
				} else {
					secrets[key] = string(val)
				}
			}
		}
		env, err := hivefile.ResolveEnv(svcDef.Env, secrets)
		if err != nil {
			return nil, status.Errorf(codes.FailedPrecondition, "service %q: %v — set missing secrets with 'hive secret set'", req.Name, err)
		}
		memBytes, err := hivefile.ParseMemory(svcDef.Resources.Memory)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "service %q: invalid memory %q: %v", req.Name, svcDef.Resources.Memory, err)
		}

		if pullErr := s.container.PullImage(ctx, svcDef.Image, nil); pullErr != nil {
			slog.Warn("image pull failed (may be local)", "image", svcDef.Image, "error", pullErr)
		}

		var networkName string
		if netBytes, netErr := s.store.Get("meta", "network:"+req.Name); netErr == nil && netBytes != nil {
			networkName = string(netBytes)
		}

		for i := currentCount; i < desired; i++ {
			targetNode := s.nodeName
			if s.scheduler != nil {
				if candidate, pickErr := s.scheduler.Pick(svcDef); pickErr == nil {
					targetNode = candidate.NodeName
				}
			}
			if targetNode == s.nodeName {
				if _, deployErr := s.deployLocalReplica(ctx, req.Name, i, svcDef, cloneEnv(env), memBytes, networkName); deployErr != nil {
					slog.Error("failed to scale up replica", "service", req.Name, "replica", i, "error", deployErr)
				}
			} else {
				if _, deployErr := s.deployRemoteReplica(ctx, req.Name, i, svcDef, cloneEnv(env), targetNode); deployErr != nil {
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
			warnErr(s.container.Stop(ctx, c.ID, 10), "failed to stop container")
			if removeErr := s.container.Remove(ctx, c.ID); removeErr != nil {
				slog.Error("failed to remove container during scale-down", "id", c.ID, "error", removeErr)
			}
		}
	}

	// Update stored service definition with new replica count
	svcDef.Replicas = desired
	if svcJSON, marshalErr := json.Marshal(svcDef); marshalErr == nil {
		warnErr(s.store.Put("services", req.Name, svcJSON), "failed to persist to store", "bucket", "services")
	}

	slog.Info("service scaled", "name", req.Name, "replicas", desired)

	// Update ingress proxy upstreams after scale
	if s.proxyMgr != nil {
		if err := s.proxyMgr.RefreshUpstreams(ctx, req.Name); err != nil {
			slog.Warn("failed to refresh ingress proxy after scale", "service", req.Name, "error", err)
		}
	}

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
					if _, stopErr := peerConn.MeshClient().StopContainer(ctx, &hivev1.StopContainerRequest{
						ContainerId: c.Id, TimeoutSeconds: 10,
					}); stopErr != nil {
						slog.Warn("failed to stop remote container during rollback", "container", c.Id, "error", stopErr)
					}
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
		warnErr(s.container.Stop(ctx, c.ID, 10), "failed to stop container")
		warnErr(s.container.Remove(ctx, c.ID), "failed to remove container")
	}

	// Resolve secrets for the previous definition
	secretKeys, _ := s.store.List("secrets")
	secrets := make(map[string]string, len(secretKeys))
	for _, key := range secretKeys {
		val, getErr := s.store.Get("secrets", key)
		if getErr == nil && val != nil {
			if s.vault != nil {
				if decrypted, decErr := s.vault.Decrypt(val); decErr == nil {
					secrets[key] = string(decrypted)
				}
			} else {
				secrets[key] = string(val)
			}
		}
	}
	env, err := hivefile.ResolveEnv(prevDef.Env, secrets)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "service %q: %v — set missing secrets with 'hive secret set'", req.Name, err)
	}
	memBytes, err := hivefile.ParseMemory(prevDef.Resources.Memory)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "service %q: invalid memory %q: %v", req.Name, prevDef.Resources.Memory, err)
	}

	if pullErr := s.container.PullImage(ctx, prevDef.Image, nil); pullErr != nil {
		slog.Warn("image pull failed (may be local)", "image", prevDef.Image, "error", pullErr)
	}

	// Redeploy all replicas with the previous definition
	replicas := prevDef.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	// Archive current version as history (swap)
	if current, _ := s.store.Get("services", req.Name); current != nil {
		warnErr(s.store.Put("service_history", req.Name, current), "failed to persist to store", "bucket", "service_history")
	}

	var networkName string
	if netBytes, netErr := s.store.Get("meta", "network:"+req.Name); netErr == nil && netBytes != nil {
		networkName = string(netBytes)
	}

	replicasStarted := 0
	for i := 0; i < replicas; i++ {
		targetNode := s.nodeName
		if s.scheduler != nil {
			if candidate, pickErr := s.scheduler.Pick(prevDef); pickErr == nil {
				targetNode = candidate.NodeName
			}
		}
		if targetNode == s.nodeName {
			if _, deployErr := s.deployLocalReplica(ctx, req.Name, i, prevDef, cloneEnv(env), memBytes, networkName); deployErr != nil {
				slog.Error("failed to deploy rollback replica", "service", req.Name, "replica", i, "target", targetNode, "error", deployErr)
			} else {
				replicasStarted++
			}
		} else {
			if _, deployErr := s.deployRemoteReplica(ctx, req.Name, i, prevDef, cloneEnv(env), targetNode); deployErr != nil {
				slog.Error("failed to deploy rollback replica", "service", req.Name, "replica", i, "target", targetNode, "error", deployErr)
			} else {
				replicasStarted++
			}
		}
	}

	if replicasStarted == 0 {
		return nil, status.Errorf(codes.Internal, "rollback failed: no replicas started for %q", req.Name)
	}

	// Persist the rolled-back definition as current
	if svcJSON, marshalErr := json.Marshal(prevDef); marshalErr == nil {
		warnErr(s.store.Put("services", req.Name, svcJSON), "failed to persist to store", "bucket", "services")
	}

	slog.Info("service rolled back", "name", req.Name, "image", prevDef.Image, "replicas", replicas)
	return &emptypb.Empty{}, nil
}

// RestartService performs a rolling restart of all replicas without changing the service definition.
func (s *Server) RestartService(ctx context.Context, req *hivev1.RestartServiceRequest) (*emptypb.Empty, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "service name is required")
	}

	s.deployMu.Lock()
	defer s.deployMu.Unlock()

	// Load service definition
	svcData, err := s.store.Get("services", req.Name)
	if err != nil || svcData == nil {
		return nil, status.Errorf(codes.NotFound, "service %q not found", req.Name)
	}
	var svcDef hivefile.ServiceDef
	if err := json.Unmarshal(svcData, &svcDef); err != nil {
		return nil, status.Errorf(codes.Internal, "corrupt service definition for %q: %v", req.Name, err)
	}

	// Resolve secrets
	secretKeys, _ := s.store.List("secrets")
	secrets := make(map[string]string, len(secretKeys))
	for _, key := range secretKeys {
		val, getErr := s.store.Get("secrets", key)
		if getErr == nil && val != nil {
			if s.vault != nil {
				if decrypted, decErr := s.vault.Decrypt(val); decErr == nil {
					secrets[key] = string(decrypted)
				}
			} else {
				secrets[key] = string(val)
			}
		}
	}
	env, err := hivefile.ResolveEnv(svcDef.Env, secrets)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "service %q: %v — set missing secrets with 'hive secret set'", req.Name, err)
	}
	memBytes, err := hivefile.ParseMemory(svcDef.Resources.Memory)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "service %q: invalid memory %q: %v", req.Name, svcDef.Resources.Memory, err)
	}

	if pullErr := s.container.PullImage(ctx, svcDef.Image, nil); pullErr != nil {
		slog.Warn("image pull failed (may be local)", "image", svcDef.Image, "error", pullErr)
	}

	replicas := svcDef.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	var networkName string
	if netBytes, netErr := s.store.Get("meta", "network:"+req.Name); netErr == nil && netBytes != nil {
		networkName = string(netBytes)
	}

	slog.Info("restarting service", "name", req.Name, "replicas", replicas)

	// Rolling restart: replace each replica one at a time, using scheduler for placement
	restarted := 0
	name := req.Name
	for i := 0; i < replicas; i++ {
		// Clean up existing container for this replica before deploying replacement
		existing, _ := s.container.ListContainers(ctx, map[string]string{
			"hive.managed": "true",
			"hive.service": name,
			"hive.replica": fmt.Sprintf("%d", i),
		})
		for _, old := range existing {
			warnErr(s.container.Stop(ctx, old.ID, 10), "failed to stop container")
			warnErr(s.container.Remove(ctx, old.ID), "failed to remove container")
		}

		targetNode := s.nodeName
		if s.scheduler != nil {
			if candidate, pickErr := s.scheduler.Pick(svcDef); pickErr == nil {
				targetNode = candidate.NodeName
			}
		}
		if targetNode == s.nodeName {
			if _, deployErr := s.deployLocalReplica(ctx, req.Name, i, svcDef, cloneEnv(env), memBytes, networkName); deployErr != nil {
				slog.Error("failed to restart replica", "service", req.Name, "replica", i, "target", targetNode, "error", deployErr)
			} else {
				restarted++
			}
		} else {
			if _, deployErr := s.deployRemoteReplica(ctx, req.Name, i, svcDef, cloneEnv(env), targetNode); deployErr != nil {
				slog.Error("failed to restart replica", "service", req.Name, "replica", i, "target", targetNode, "error", deployErr)
			} else {
				restarted++
			}
		}
	}

	if restarted == 0 {
		return nil, status.Errorf(codes.Internal, "restart failed: no replicas could be started for %q", req.Name)
	}

	slog.Info("service restarted", "name", req.Name, "restarted", restarted, "total", replicas)

	// Update ingress proxy upstreams after restart
	if s.proxyMgr != nil {
		if err := s.proxyMgr.RefreshUpstreams(ctx, req.Name); err != nil {
			slog.Warn("failed to refresh ingress proxy after restart", "service", req.Name, "error", err)
		}
	}

	return &emptypb.Empty{}, nil
}

// UpdateService applies partial updates (image, replicas) to an existing service without a full Hivefile redeploy.
func (s *Server) UpdateService(ctx context.Context, req *hivev1.UpdateServiceRequest) (*emptypb.Empty, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "service name is required")
	}

	s.deployMu.Lock()
	defer s.deployMu.Unlock()

	// Load existing service definition from store
	svcJSON, err := s.store.Get("services", req.Name)
	if err != nil || svcJSON == nil {
		return nil, status.Errorf(codes.NotFound, "service %q not found", req.Name)
	}
	var svcDef hivefile.ServiceDef
	if err := json.Unmarshal(svcJSON, &svcDef); err != nil {
		return nil, status.Errorf(codes.Internal, "corrupt service definition for %q: %v", req.Name, err)
	}

	// Apply requested updates
	changed := false
	if req.Image != "" && req.Image != svcDef.Image {
		svcDef.Image = req.Image
		changed = true
	}
	if req.Replicas > 0 && int(req.Replicas) != svcDef.Replicas {
		svcDef.Replicas = int(req.Replicas)
		changed = true
	}
	if len(req.Env) > 0 {
		if svcDef.Env == nil {
			svcDef.Env = make(map[string]string)
		}
		for k, v := range req.Env {
			svcDef.Env[k] = v
		}
		changed = true
	}
	if !changed {
		return &emptypb.Empty{}, nil
	}

	// Persist updated definition
	updatedJSON, err := json.Marshal(svcDef)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal updated service definition: %v", err)
	}
	if err := s.store.Put("services", req.Name, updatedJSON); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to persist updated service definition: %v", err)
	}

	slog.Info("service definition updated, triggering rolling restart",
		"service", req.Name,
		"image", svcDef.Image,
		"replicas", svcDef.Replicas,
	)

	// Do a rolling restart with health checks between replicas
	if err := s.rollingRestart(ctx, req.Name, svcDef); err != nil {
		return nil, status.Errorf(codes.Internal, "update succeeded but rolling restart failed for %q: %v", req.Name, err)
	}

	return &emptypb.Empty{}, nil
}

// rollingRestart restarts each replica of a service one at a time, waiting
// for each to pass health checks before proceeding to the next.
func (s *Server) rollingRestart(ctx context.Context, name string, svcDef hivefile.ServiceDef) error {
	containers, _ := s.container.ListContainers(ctx, map[string]string{
		"hive.managed": "true",
		"hive.service": name,
	})
	// Filter out ingress proxy containers
	var replicas []container.ContainerInfo
	for _, c := range containers {
		if c.Labels["hive.ingress"] != "true" && c.Status == "running" {
			replicas = append(replicas, c)
		}
	}

	if len(replicas) == 0 {
		return nil
	}

	// Resolve secrets for env injection
	secretKeys, _ := s.store.List("secrets")
	secrets := make(map[string]string, len(secretKeys))
	for _, key := range secretKeys {
		val, err := s.store.Get("secrets", key)
		if err != nil || val == nil {
			continue
		}
		if s.vault != nil {
			if dec, decErr := s.vault.Decrypt(val); decErr == nil {
				secrets[key] = string(dec)
			}
		} else {
			secrets[key] = string(val)
		}
	}
	env, _ := hivefile.ResolveEnv(svcDef.Env, secrets)
	memBytes, _ := hivefile.ParseMemory(svcDef.Resources.Memory)

	// Get the network name
	networkName := ""
	if netData, err := s.store.Get("meta", "network:"+name); err == nil && netData != nil {
		networkName = string(netData)
	}

	for i, c := range replicas {
		replicaIdx := 0
		if label := c.Labels["hive.replica"]; label != "" {
			fmt.Sscanf(label, "%d", &replicaIdx)
		}

		slog.Info("rolling restart: stopping replica", "service", name, "replica", replicaIdx, "index", i+1, "total", len(replicas))

		// Stop and remove old container
		warnErr(s.container.Stop(ctx, c.ID, 10), "failed to stop container")
		warnErr(s.container.Remove(ctx, c.ID), "failed to remove container")

		// Create new container (clone env to prevent cross-replica mutation)
		if _, err := s.deployLocalReplica(ctx, name, replicaIdx, svcDef, cloneEnv(env), memBytes, networkName); err != nil {
			return fmt.Errorf("rolling restart replica %d: %w", replicaIdx, err)
		}

		// Wait for health check to pass (if configured)
		if svcDef.Health.Port > 0 || len(svcDef.Health.ExecCommand) > 0 {
			checkTimeout := 5 * time.Second
			if d, parseErr := time.ParseDuration(svcDef.Health.Timeout); parseErr == nil && d > 0 {
				checkTimeout = d
			}
			healthy := false
			for attempt := 0; attempt < 10; attempt++ {
				time.Sleep(checkTimeout)
				cfg := health.Config{
					Host: "127.0.0.1",
					Port: svcDef.Health.Port,
					Path: svcDef.Health.Path,
				}
				switch svcDef.Health.Type {
				case "http":
					cfg.Type = health.CheckHTTP
				case "tcp":
					cfg.Type = health.CheckTCP
				case "exec":
					cfg.Type = health.CheckExec
				}
				result := s.health.Check(ctx, cfg)
				if result.Healthy {
					healthy = true
					break
				}
			}
			if !healthy {
				slog.Warn("rolling restart: replica did not pass health check, continuing", "service", name, "replica", replicaIdx)
			}
		}
	}

	slog.Info("rolling restart complete", "service", name, "replicas", len(replicas))
	return nil
}

// RotateSecret updates a secret value and rolling-restarts all services that reference it.
func (s *Server) RotateSecret(ctx context.Context, req *hivev1.RotateSecretRequest) (*hivev1.RotateSecretResponse, error) {
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "secret key is required")
	}
	if len(req.NewValue) == 0 {
		return nil, status.Error(codes.InvalidArgument, "secret value is required")
	}

	// Encrypt and store the new value
	var encrypted []byte
	if s.vault != nil {
		enc, err := s.vault.Encrypt(req.NewValue)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "encrypt secret: %v", err)
		}
		encrypted = enc
	} else {
		encrypted = req.NewValue
	}
	if err := s.store.Put("secrets", req.Key, encrypted); err != nil {
		return nil, status.Errorf(codes.Internal, "store secret: %v", err)
	}

	slog.Info("secret rotated", "key", req.Key)

	// Find all services referencing this secret
	var restarted []string
	svcKeys, _ := s.store.List("services")
	for _, svcName := range svcKeys {
		data, err := s.store.Get("services", svcName)
		if err != nil || data == nil {
			continue
		}
		var svcDef hivefile.ServiceDef
		if err := json.Unmarshal(data, &svcDef); err != nil {
			continue
		}
		// Check if any env var references the secret via template placeholder
		found := false
		for _, v := range svcDef.Env {
			if hivefile.ReferencesSecret(v, req.Key) {
				found = true
				break
			}
		}
		if !found {
			continue
		}

		slog.Info("secret rotation: restarting service", "service", svcName, "secret", req.Key)
		s.deployMu.Lock()
		err = s.rollingRestart(ctx, svcName, svcDef)
		s.deployMu.Unlock()
		if err != nil {
			slog.Error("secret rotation: restart failed", "service", svcName, "error", err)
		} else {
			restarted = append(restarted, svcName)
		}
	}

	return &hivev1.RotateSecretResponse{RestartedServices: restarted}, nil
}

// SetNodeLabel sets a label on a node (persisted in store).
func (s *Server) SetNodeLabel(ctx context.Context, req *hivev1.SetNodeLabelRequest) (*emptypb.Empty, error) {
	if req.Node == "" || req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "node and key are required")
	}
	labels := s.getNodeLabels(req.Node)
	labels[req.Key] = req.Value
	data, err := json.Marshal(labels)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal labels: %v", err)
	}
	if err := s.store.Put("meta", "node_labels:"+req.Node, data); err != nil {
		return nil, status.Errorf(codes.Internal, "persist label: %v", err)
	}
	slog.Info("node label set", "node", req.Node, "key", req.Key, "value", req.Value)
	return &emptypb.Empty{}, nil
}

// RemoveNodeLabel removes a label from a node.
func (s *Server) RemoveNodeLabel(ctx context.Context, req *hivev1.RemoveNodeLabelRequest) (*emptypb.Empty, error) {
	if req.Node == "" || req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "node and key are required")
	}
	labels := s.getNodeLabels(req.Node)
	delete(labels, req.Key)
	data, err := json.Marshal(labels)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal labels: %v", err)
	}
	if err := s.store.Put("meta", "node_labels:"+req.Node, data); err != nil {
		return nil, status.Errorf(codes.Internal, "persist label: %v", err)
	}
	slog.Info("node label removed", "node", req.Node, "key", req.Key)
	return &emptypb.Empty{}, nil
}

// getNodeLabels loads labels for a node from the store.
func (s *Server) getNodeLabels(nodeName string) map[string]string {
	data, _ := s.store.Get("meta", "node_labels:"+nodeName)
	labels := make(map[string]string)
	if data != nil {
		json.Unmarshal(data, &labels)
	}
	return labels
}

// DeployStack deploys a stack of multiple Hivefiles as a single unit.
func (s *Server) DeployStack(ctx context.Context, req *hivev1.DeployStackRequest) (*hivev1.DeployServiceResponse, error) {
	if len(req.HivefileTomls) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one Hivefile is required")
	}

	// Parse and merge all Hivefiles
	var hivefiles []*hivefile.Hivefile
	for i, toml := range req.HivefileTomls {
		hf, err := hivefile.Parse([]byte(toml))
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "hivefile %d: %v", i+1, err)
		}
		hivefiles = append(hivefiles, hf)
	}

	merged, err := hivefile.MergeHivefiles(hivefiles)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "merge stack: %v", err)
	}

	slog.Info("deploying stack", "name", req.StackName, "services", len(merged.Service))

	// Store stack membership
	var svcNames []string
	for name := range merged.Service {
		svcNames = append(svcNames, name)
	}
	if stackJSON, err := json.Marshal(svcNames); err == nil {
		stackName := req.StackName
		if stackName == "" {
			stackName = "default"
		}
		warnErr(s.store.Put("meta", "stack:"+stackName, stackJSON), "failed to persist stack membership")
	}

	// Serialize the merged Hivefile back to TOML for DeployService
	mergedTOML, err := hivefile.MarshalHivefile(merged)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal merged hivefile: %v", err)
	}

	return s.DeployService(ctx, &hivev1.DeployServiceRequest{HivefileToml: string(mergedTOML)})
}

// ListContainers lists containers, optionally filtered.
// ─── App Store RPCs ──────────────────────────────────────────────

// ListApps returns available apps, optionally filtered by category.
func (s *Server) ListApps(_ context.Context, req *hivev1.ListAppsRequest) (*hivev1.ListAppsResponse, error) {
	if s.appStore == nil {
		return &hivev1.ListAppsResponse{}, nil
	}
	apps := s.appStore.List(req.Category)
	var protoApps []*hivev1.AppDefinition
	for _, a := range apps {
		protoApps = append(protoApps, appDefToProto(a))
	}
	return &hivev1.ListAppsResponse{Apps: protoApps}, nil
}

// GetApp returns a single app by ID.
func (s *Server) GetApp(_ context.Context, req *hivev1.GetAppRequest) (*hivev1.AppDefinition, error) {
	if s.appStore == nil {
		return nil, status.Error(codes.NotFound, "app store not initialized")
	}
	app, ok := s.appStore.Get(req.Id)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "app %q not found", req.Id)
	}
	return appDefToProto(app), nil
}

// SearchApps searches the app catalog.
func (s *Server) SearchApps(_ context.Context, req *hivev1.SearchAppsRequest) (*hivev1.ListAppsResponse, error) {
	if s.appStore == nil {
		return &hivev1.ListAppsResponse{}, nil
	}
	apps := s.appStore.Search(req.Query)
	var protoApps []*hivev1.AppDefinition
	for _, a := range apps {
		protoApps = append(protoApps, appDefToProto(a))
	}
	return &hivev1.ListAppsResponse{Apps: protoApps}, nil
}

// InstallApp generates a Hivefile from an app template and deploys it.
func (s *Server) InstallApp(ctx context.Context, req *hivev1.InstallAppRequest) (*hivev1.DeployServiceResponse, error) {
	if s.appStore == nil {
		return nil, status.Error(codes.FailedPrecondition, "app store not initialized")
	}
	serviceName := req.ServiceName
	if serviceName == "" {
		serviceName = req.AppId
	}

	// Generate Hivefile from template
	hivefileToml, err := s.appStore.GenerateHivefile(req.AppId, serviceName, req.Config)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "generate hivefile: %v", err)
	}

	slog.Info("installing app", "app", req.AppId, "service", serviceName)

	// Deploy via existing pipeline
	resp, err := s.DeployService(ctx, &hivev1.DeployServiceRequest{HivefileToml: hivefileToml})
	if err != nil {
		return nil, err
	}

	// Record installation
	if recordErr := s.appStore.RecordInstall(req.AppId, serviceName, req.Config); recordErr != nil {
		slog.Warn("failed to record app installation", "app", req.AppId, "error", recordErr)
	}

	return resp, nil
}

// ListInstalledApps returns apps installed from the catalog.
func (s *Server) ListInstalledApps(_ context.Context, _ *emptypb.Empty) (*hivev1.ListInstalledAppsResponse, error) {
	if s.appStore == nil {
		return &hivev1.ListInstalledAppsResponse{}, nil
	}
	records := s.appStore.ListInstalled()
	var protoApps []*hivev1.InstalledApp
	for _, r := range records {
		protoApps = append(protoApps, &hivev1.InstalledApp{
			AppId:       r.AppID,
			ServiceName: r.ServiceName,
		})
	}
	return &hivev1.ListInstalledAppsResponse{Apps: protoApps}, nil
}

// AddCustomApp adds a user-defined app to the catalog.
func (s *Server) AddCustomApp(_ context.Context, req *hivev1.AddCustomAppRequest) (*hivev1.AppDefinition, error) {
	if s.appStore == nil {
		return nil, status.Error(codes.FailedPrecondition, "app store not initialized")
	}
	app, err := s.appStore.AddCustom(req.RecipeToml)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "add custom app: %v", err)
	}
	return appDefToProto(app), nil
}

// RemoveCustomApp removes a user-defined app.
func (s *Server) RemoveCustomApp(_ context.Context, req *hivev1.RemoveCustomAppRequest) (*emptypb.Empty, error) {
	if s.appStore == nil {
		return nil, status.Error(codes.FailedPrecondition, "app store not initialized")
	}
	if err := s.appStore.RemoveCustom(req.Id); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	return &emptypb.Empty{}, nil
}

// ─── Registry RPCs ──────────────────────────────────────────────

// RegistryLogin stores encrypted credentials for a container registry.
func (s *Server) RegistryLogin(_ context.Context, req *hivev1.RegistryLoginRequest) (*emptypb.Empty, error) {
	if s.registryMgr == nil {
		return nil, status.Error(codes.FailedPrecondition, "registry manager not initialized")
	}
	if err := s.registryMgr.Login(req.Url, req.Username, req.Password); err != nil {
		return nil, status.Errorf(codes.Internal, "registry login: %v", err)
	}
	return &emptypb.Empty{}, nil
}

// ListRegistries returns configured registries (without passwords).
func (s *Server) ListRegistries(_ context.Context, _ *emptypb.Empty) (*hivev1.ListRegistriesResponse, error) {
	if s.registryMgr == nil {
		return &hivev1.ListRegistriesResponse{}, nil
	}
	records := s.registryMgr.List()
	var protos []*hivev1.RegistryCredential
	for _, r := range records {
		protos = append(protos, &hivev1.RegistryCredential{
			Url:      r.URL,
			Username: r.Username,
		})
	}
	return &hivev1.ListRegistriesResponse{Registries: protos}, nil
}

// RemoveRegistry deletes credentials for a container registry.
func (s *Server) RemoveRegistry(_ context.Context, req *hivev1.RemoveRegistryRequest) (*emptypb.Empty, error) {
	if s.registryMgr == nil {
		return nil, status.Error(codes.FailedPrecondition, "registry manager not initialized")
	}
	if err := s.registryMgr.Remove(req.Url); err != nil {
		return nil, status.Errorf(codes.Internal, "remove registry: %v", err)
	}
	return &emptypb.Empty{}, nil
}

// appDefToProto converts an AppDef to a protobuf AppDefinition.
func appDefToProto(a *appstore.AppDef) *hivev1.AppDefinition {
	var fields []*hivev1.AppConfigField
	for _, f := range a.ConfigFields {
		fields = append(fields, &hivev1.AppConfigField{
			Key:          f.Key,
			Label:        f.Label,
			Type:         f.Type,
			Required:     f.Required,
			DefaultValue: f.Default,
			Description:  f.Description,
		})
	}
	return &hivev1.AppDefinition{
		Id:           a.ID,
		Name:         a.Name,
		Description:  a.Description,
		Icon:         a.Icon,
		Category:     a.Category,
		Tags:         a.Tags,
		Image:        a.Image,
		Version:      a.Version,
		ConfigFields: fields,
		MinMemory:    a.MinMemory,
		Platforms:    a.Platforms,
		Builtin:      a.Builtin,
	}
}

// ─── Containers ──────────────────────────────────────────────

// ─── Discovery RPCs ─────────────────────────────────────────────

// DiscoverContainers lists Docker containers NOT managed by Hive.
func (s *Server) DiscoverContainers(ctx context.Context, _ *emptypb.Empty) (*hivev1.DiscoverContainersResponse, error) {
	all, err := s.container.ListAllContainers(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list containers: %v", err)
	}
	var discovered []*hivev1.DiscoveredContainer
	for _, c := range all {
		if c.Labels["hive.managed"] == "true" {
			continue // skip Hive-managed containers
		}
		discovered = append(discovered, &hivev1.DiscoveredContainer{
			Id:      c.ID,
			Name:    c.Name,
			Image:   c.Image,
			Status:  c.Status,
			Ports:   c.Ports,
			Env:     c.Env,
			Volumes: c.Volumes,
			Command: c.Command,
		})
	}
	return &hivev1.DiscoverContainersResponse{Containers: discovered}, nil
}

// AdoptContainer imports an unmanaged container into Hive by generating a Hivefile from its config.
func (s *Server) AdoptContainer(ctx context.Context, req *hivev1.AdoptContainerRequest) (*hivev1.DeployServiceResponse, error) {
	if req.ContainerId == "" {
		return nil, status.Error(codes.InvalidArgument, "container_id is required")
	}
	serviceName := req.ServiceName
	if serviceName == "" {
		return nil, status.Error(codes.InvalidArgument, "service_name is required")
	}

	// Inspect the container to get its full config
	info, err := s.container.Inspect(ctx, req.ContainerId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "inspect container: %v", err)
	}

	// Get detailed info including env/volumes
	allContainers, _ := s.container.ListAllContainers(ctx)
	var env []string
	var volumes []string
	for _, c := range allContainers {
		if c.ID == req.ContainerId {
			env = c.Env
			volumes = c.Volumes
			break
		}
	}

	// Build Hivefile TOML from container config
	var tomlLines []string
	tomlLines = append(tomlLines, fmt.Sprintf("[service.%s]", serviceName))
	tomlLines = append(tomlLines, fmt.Sprintf("image = %q", info.Image))
	tomlLines = append(tomlLines, "replicas = 1")
	tomlLines = append(tomlLines, "")

	// Ports
	if len(info.Ports) > 0 {
		tomlLines = append(tomlLines, fmt.Sprintf("[service.%s.ports]", serviceName))
		for host, container := range info.Ports {
			tomlLines = append(tomlLines, fmt.Sprintf("%q = %q", host, container))
		}
		tomlLines = append(tomlLines, "")
	}

	// Env vars (filter out common Docker-internal ones)
	if len(env) > 0 {
		tomlLines = append(tomlLines, fmt.Sprintf("[service.%s.env]", serviceName))
		for _, kv := range env {
			if idx := strings.IndexByte(kv, '='); idx > 0 {
				key := kv[:idx]
				val := kv[idx+1:]
				// Skip PATH and common Docker internals
				if key == "PATH" || key == "HOME" || key == "HOSTNAME" {
					continue
				}
				tomlLines = append(tomlLines, fmt.Sprintf("%s = %q", key, val))
			}
		}
		tomlLines = append(tomlLines, "")
	}

	// Volumes
	if len(volumes) > 0 {
		for _, v := range volumes {
			tomlLines = append(tomlLines, fmt.Sprintf("[[service.%s.volumes]]", serviceName))
			tomlLines = append(tomlLines, fmt.Sprintf("linux = %q", v))
			tomlLines = append(tomlLines, "")
		}
	}

	hivefileToml := strings.Join(tomlLines, "\n")
	slog.Info("adopting container", "id", container.ShortID(req.ContainerId), "service", serviceName, "image", info.Image)

	// Stop original if requested
	if req.StopOriginal {
		warnErr(s.container.Stop(ctx, req.ContainerId, 10), "failed to stop original container")
		warnErr(s.container.Remove(ctx, req.ContainerId), "failed to remove original container")
	}

	// Deploy via standard pipeline
	return s.DeployService(ctx, &hivev1.DeployServiceRequest{HivefileToml: hivefileToml})
}

// ─── Disk RPCs ──────────────────────────────────────────────

// ListDisks returns available filesystems on this node.
func (s *Server) ListDisks(_ context.Context, _ *emptypb.Empty) (*hivev1.ListDisksResponse, error) {
	disks := sysinfo.ListDisks()
	var entries []*hivev1.DiskEntry
	for _, d := range disks {
		entries = append(entries, &hivev1.DiskEntry{
			Path:           d.Path,
			TotalBytes:     d.Total,
			AvailableBytes: d.Available,
			Fstype:         d.FSType,
			Device:         d.Device,
		})
	}
	return &hivev1.ListDisksResponse{Disks: entries}, nil
}

// ─── Containers ──────────────────────────────────────────────

func (s *Server) ListContainers(ctx context.Context, req *hivev1.ListContainersRequest) (*hivev1.ListContainersResponse, error) {
	// If a specific remote node was requested, fan out only to that peer.
	if req.NodeName != "" && req.NodeName != s.nodeName {
		if s.mesh == nil {
			return &hivev1.ListContainersResponse{}, nil
		}
		peerConn, err := s.mesh.PeerByName(req.NodeName)
		if err != nil {
			return &hivev1.ListContainersResponse{}, nil
		}
		peerCtx, peerCancel := context.WithTimeout(ctx, 5*time.Second)
		defer peerCancel()
		state, err := peerConn.MeshClient().SyncState(peerCtx, &emptypb.Empty{})
		if err != nil {
			slog.Debug("failed to sync state from peer for ListContainers", "peer", req.NodeName, "error", err)
			return &hivev1.ListContainersResponse{}, nil
		}
		var protos []*hivev1.Container
		for _, c := range state.Containers {
			if req.ServiceName != "" && c.ServiceName != req.ServiceName {
				continue
			}
			protos = append(protos, c)
		}
		return &hivev1.ListContainersResponse{Containers: protos}, nil
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
	seenIDs := make(map[string]bool)
	for _, c := range containers {
		cStatus := hivev1.ContainerStatus_CONTAINER_STATUS_STOPPED
		switch c.Status {
		case "running":
			cStatus = hivev1.ContainerStatus_CONTAINER_STATUS_RUNNING
		case "restarting":
			cStatus = hivev1.ContainerStatus_CONTAINER_STATUS_RESTARTING
		}
		seenIDs[c.ID] = true
		protos = append(protos, &hivev1.Container{
			Id:          c.ID,
			ServiceName: c.Labels["hive.service"],
			NodeId:      s.nodeName,
			Image:       c.Image,
			Status:      cStatus,
		})
	}

	// Fan out to peers for cluster-wide container view
	if s.mesh != nil && req.NodeName == "" {
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
					resultCh <- peerResult{}
					return
				}
				peerCtx, peerCancel := context.WithTimeout(fanoutCtx, 5*time.Second)
				state, err := peerConn.MeshClient().SyncState(peerCtx, &emptypb.Empty{})
				peerCancel()
				if err != nil {
					slog.Debug("failed to sync state from peer for ListContainers", "peer", peerName, "error", err)
					resultCh <- peerResult{}
					return
				}
				var filtered []*hivev1.Container
				for _, c := range state.Containers {
					if req.ServiceName != "" && c.ServiceName != req.ServiceName {
						continue
					}
					filtered = append(filtered, c)
				}
				resultCh <- peerResult{peerName: peerName, containers: filtered}
			}(peer.Info.Name)
		}

		for range peers {
			var pr peerResult
			select {
			case pr = <-resultCh:
			case <-ctx.Done():
				return nil, status.Errorf(codes.Canceled, "client disconnected during container list fan-out")
			}
			for _, c := range pr.containers {
				if !seenIDs[c.Id] {
					seenIDs[c.Id] = true
					if c.NodeId == "" {
						c.NodeId = pr.peerName
					}
					protos = append(protos, c)
				}
			}
		}
	}

	return &hivev1.ListContainersResponse{Containers: protos}, nil
}

// ContainerLogs streams logs from a container (local or remote).
// When service_name is provided instead of container_id, logs are streamed
// from ALL replicas of the service concurrently.
func (s *Server) ContainerLogs(req *hivev1.ContainerLogsRequest, stream hivev1.HiveAPI_ContainerLogsServer) error {
	if req.ContainerId == "" && req.ServiceName == "" {
		return status.Error(codes.InvalidArgument, "container_id or service_name is required")
	}

	// If a specific container_id is given, stream from that single container.
	if req.ContainerId != "" {
		return s.streamSingleContainerLogs(req.ContainerId, req, stream)
	}

	// service_name provided — find ALL local replicas and stream from all of them.
	containers, err := s.container.ListContainers(stream.Context(), map[string]string{
		"hive.managed": "true",
		"hive.service": req.ServiceName,
	})
	if err != nil {
		slog.Debug("failed to list containers for service", "service", req.ServiceName, "error", err)
	}

	if len(containers) > 0 {
		return s.streamMultiContainerLogs(containers, req, stream)
	}

	// No local containers — try remote peers via SyncState
	if s.mesh != nil {
		// Collect all remote container IDs across all peers for this service.
		type remoteTarget struct {
			containerID string
			peerConn    *mesh.Peer
		}
		var remoteTargets []remoteTarget

		for _, peer := range s.mesh.Peers() {
			peerConn, err := s.mesh.PeerByName(peer.Info.Name)
			if err != nil {
				continue
			}
			state, err := peerConn.MeshClient().SyncState(stream.Context(), &emptypb.Empty{})
			if err != nil {
				continue
			}
			for _, c := range state.Containers {
				if c.ServiceName == req.ServiceName {
					remoteTargets = append(remoteTargets, remoteTarget{
						containerID: c.Id,
						peerConn:    peerConn,
					})
				}
			}
		}

		if len(remoteTargets) > 0 {
			// Stream from all remote replicas concurrently using a merged channel.
			entryCh := make(chan *hivev1.LogEntry, 64)
			var wg sync.WaitGroup
			for _, rt := range remoteTargets {
				wg.Add(1)
				go func(rt remoteTarget) {
					defer wg.Done()
					remoteStream, err := rt.peerConn.MeshClient().PullLogs(stream.Context(), &hivev1.PullLogsRequest{
						ContainerId: rt.containerID,
						Follow:      req.Follow,
						TailLines:   req.TailLines,
					})
					if err != nil {
						return
					}
					for {
						entry, err := remoteStream.Recv()
						if err != nil {
							return
						}
						select {
						case entryCh <- entry:
						case <-stream.Context().Done():
							return
						}
					}
				}(rt)
			}

			// Close channel when all goroutines finish.
			go func() {
				wg.Wait()
				close(entryCh)
			}()

			for entry := range entryCh {
				if err := stream.Send(entry); err != nil {
					return err
				}
			}
			return nil
		}
	}

	return status.Errorf(codes.NotFound, "no containers found for service %q on any node", req.ServiceName)
}

// streamSingleContainerLogs streams logs from one specific container (local or remote).
func (s *Server) streamSingleContainerLogs(containerID string, req *hivev1.ContainerLogsRequest, stream hivev1.HiveAPI_ContainerLogsServer) error {
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

	// Try remote peers
	if s.mesh != nil {
		for _, peer := range s.mesh.Peers() {
			peerConn, err := s.mesh.PeerByName(peer.Info.Name)
			if err != nil {
				continue
			}
			remoteStream, err := peerConn.MeshClient().PullLogs(stream.Context(), &hivev1.PullLogsRequest{
				ContainerId: containerID,
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

	return status.Errorf(codes.NotFound, "container %s not found on any node", containerID)
}

// streamMultiContainerLogs streams logs from multiple local containers concurrently.
// All container log entries are merged into the single gRPC response stream.
func (s *Server) streamMultiContainerLogs(containers []container.ContainerInfo, req *hivev1.ContainerLogsRequest, stream hivev1.HiveAPI_ContainerLogsServer) error {
	if len(containers) == 1 {
		// Optimization: single replica, no goroutine overhead needed.
		return s.streamSingleContainerLogs(containers[0].ID, req, stream)
	}

	// Stream from all replicas concurrently, merging into one channel.
	// Use a cancellable context so workers exit promptly when stream.Send fails.
	cancelCtx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	entryCh := make(chan *hivev1.LogEntry, 64)
	var wg sync.WaitGroup
	var readers []io.ReadCloser

	for _, c := range containers {
		cID := c.ID
		reader, err := s.container.Logs(cancelCtx, cID, container.LogOpts{
			Follow:    req.Follow,
			TailLines: int(req.TailLines),
		})
		if err != nil {
			slog.Debug("failed to get logs for replica", "container", cID, "error", err)
			continue
		}
		readers = append(readers, reader)

		wg.Add(1)
		go func(cID string, reader io.ReadCloser) {
			defer wg.Done()
			if logErr := container.StreamDockerLogs(reader, func(line, streamType string) error {
				entry := &hivev1.LogEntry{
					ContainerId:   cID,
					NodeName:      s.nodeName,
					ServiceName:   req.ServiceName,
					Line:          line,
					Stream:        streamType,
					TimestampUnix: time.Now().Unix(),
				}
				select {
				case entryCh <- entry:
					return nil
				case <-cancelCtx.Done():
					return cancelCtx.Err()
				}
			}); logErr != nil {
				slog.Warn("log stream error", "container", cID, "error", logErr)
			}
		}(cID, reader)
	}

	if len(readers) == 0 {
		return status.Errorf(codes.NotFound, "no containers found for service %q", req.ServiceName)
	}

	// Close channel and readers when all goroutines finish.
	go func() {
		wg.Wait()
		close(entryCh)
		for _, r := range readers {
			r.Close()
		}
	}()

	for entry := range entryCh {
		if err := stream.Send(entry); err != nil {
			return err
		}
	}

	return nil
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

	// Verify the target container is Hive-managed
	{
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

// ─── Volumes ────────────────────────────────────────────────

// ListVolumes returns all named volumes from the container runtime.
func (s *Server) ListVolumes(ctx context.Context, _ *emptypb.Empty) (*hivev1.ListVolumesResponse, error) {
	vols, err := s.container.ListVolumes(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list volumes: %v", err)
	}
	var out []*hivev1.VolumeInfo
	for _, v := range vols {
		out = append(out, &hivev1.VolumeInfo{
			Name:       v.Name,
			Driver:     v.Driver,
			Mountpoint: v.Mountpoint,
			CreatedAt:  v.CreatedAt,
		})
	}
	return &hivev1.ListVolumesResponse{Volumes: out}, nil
}

// CreateVolume creates a named volume on the container runtime.
func (s *Server) CreateVolume(ctx context.Context, req *hivev1.CreateVolumeRequest) (*hivev1.CreateVolumeResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "volume name is required")
	}
	mountpoint, err := s.container.CreateVolume(ctx, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create volume: %v", err)
	}
	slog.Info("volume created", "name", req.Name, "mountpoint", mountpoint)
	return &hivev1.CreateVolumeResponse{Name: req.Name, Mountpoint: mountpoint}, nil
}

// DeleteVolume removes a named volume from the container runtime.
func (s *Server) DeleteVolume(ctx context.Context, req *hivev1.DeleteVolumeRequest) (*emptypb.Empty, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "volume name is required")
	}
	if err := s.container.DeleteVolume(ctx, req.Name); err != nil {
		return nil, status.Errorf(codes.Internal, "delete volume: %v", err)
	}
	slog.Info("volume deleted", "name", req.Name)
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
		subID, eventCh := s.mesh.Subscribe(64)
		defer s.mesh.Unsubscribe(subID)
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
				case mesh.EventNodeUpdated:
					evType = hivev1.EventType_EVENT_TYPE_NODE_UPDATED
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

// ListCronJobs returns all registered cron jobs.
func (s *Server) ListCronJobs(_ context.Context, _ *emptypb.Empty) (*hivev1.ListCronJobsResponse, error) {
	if s.cronSched == nil {
		return &hivev1.ListCronJobsResponse{}, nil
	}
	jobs := s.cronSched.List()
	var protos []*hivev1.CronJob
	for _, j := range jobs {
		cj := &hivev1.CronJob{
			Name:    j.Name,
			Service: j.Service,
			Command: j.Command,
			NextRun: j.NextRun.Format(time.RFC3339),
		}
		if !j.LastRun.IsZero() {
			cj.LastRun = j.LastRun.Format(time.RFC3339)
		}
		protos = append(protos, cj)
	}
	return &hivev1.ListCronJobsResponse{Jobs: protos}, nil
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
		csrCtx, csrCancel := context.WithTimeout(context.Background(), 10*time.Second)
		resp, err := peerConn.MeshClient().SignNodeCSR(csrCtx, &hivev1.SignCSRRequest{
			CsrPem:    csrPEM,
			NodeName:  local.Name,
			JoinToken: joinToken,
		})
		csrCancel()
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

// cloneEnv returns a shallow copy of the env map to prevent cross-replica mutation.
func cloneEnv(m map[string]string) map[string]string {
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
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

// ValidateHivefile validates a hivefile and optionally runs server-side checks.
func (s *Server) ValidateHivefile(ctx context.Context, req *hivev1.ValidateHivefileRequest) (*hivev1.ValidateHivefileResponse, error) {
	if req.HivefileToml == "" {
		return nil, status.Error(codes.InvalidArgument, "hivefile_toml is required")
	}

	// Run client-side (pure) validation
	clientIssues := hivefile.Validate(req.HivefileToml)

	// Convert to proto issues
	var protoIssues []*hivev1.ValidationIssue
	hasError := false
	for _, ci := range clientIssues {
		pi := &hivev1.ValidationIssue{
			Severity: toProtoSeverity(ci.Severity),
			Field:    ci.Field,
			Message:  ci.Message,
			Service:  ci.Service,
		}
		protoIssues = append(protoIssues, pi)
		if ci.Severity == "error" {
			hasError = true
		}
	}

	// Server-side checks (secrets, nodes, port conflicts)
	if req.ServerChecks {
		hf, err := hivefile.Parse([]byte(req.HivefileToml))
		if err == nil {
			// Check secrets exist
			secretKeys, _ := s.store.List("secrets")
			secretSet := make(map[string]bool, len(secretKeys))
			for _, k := range secretKeys {
				secretSet[k] = true
			}

			// Build set of known node names
			knownNodes := map[string]bool{s.nodeName: true}
			if s.mesh != nil {
				for _, peer := range s.mesh.Peers() {
					knownNodes[peer.Info.Name] = true
				}
			}

			// Get running containers for port conflict detection
			runningContainers, _ := s.container.ListContainers(ctx, map[string]string{
				"hive.managed": "true",
			})
			usedPorts := make(map[string]string) // hostPort -> serviceName
			for _, c := range runningContainers {
				svcName := c.Labels["hive.service"]
				for hostPort := range c.Ports {
					usedPorts[hostPort] = svcName
				}
			}

			for svcName, svc := range hf.Service {
				// Check secret references
				refs := hivefile.ExtractSecretRefs(svc)
				for _, ref := range refs {
					if !secretSet[ref] {
						protoIssues = append(protoIssues, &hivev1.ValidationIssue{
							Severity: hivev1.ValidationSeverity_VALIDATION_SEVERITY_ERROR,
							Field:    fmt.Sprintf("service.%s.env", svcName),
							Message:  fmt.Sprintf("secret %q not found in vault", ref),
							Service:  svcName,
						})
						hasError = true
					}
				}

				// Check node constraints
				if svc.Node != "" && !knownNodes[svc.Node] {
					protoIssues = append(protoIssues, &hivev1.ValidationIssue{
						Severity: hivev1.ValidationSeverity_VALIDATION_SEVERITY_ERROR,
						Field:    fmt.Sprintf("service.%s.node", svcName),
						Message:  fmt.Sprintf("node %q is not a known cluster member", svc.Node),
						Service:  svcName,
					})
					hasError = true
				}

				// Check port conflicts with running containers
				for hostPort := range svc.Ports {
					if existingSvc, conflict := usedPorts[hostPort]; conflict {
						protoIssues = append(protoIssues, &hivev1.ValidationIssue{
							Severity: hivev1.ValidationSeverity_VALIDATION_SEVERITY_WARNING,
							Field:    fmt.Sprintf("service.%s.ports", svcName),
							Message:  fmt.Sprintf("host port %s is already in use by running service %q", hostPort, existingSvc),
							Service:  svcName,
						})
					}
				}
			}
		}
	}

	return &hivev1.ValidateHivefileResponse{
		Valid:  !hasError,
		Issues: protoIssues,
	}, nil
}

// toProtoSeverity converts a string severity to the proto enum.
func toProtoSeverity(sev string) hivev1.ValidationSeverity {
	switch sev {
	case "error":
		return hivev1.ValidationSeverity_VALIDATION_SEVERITY_ERROR
	case "warning":
		return hivev1.ValidationSeverity_VALIDATION_SEVERITY_WARNING
	case "info":
		return hivev1.ValidationSeverity_VALIDATION_SEVERITY_INFO
	default:
		return hivev1.ValidationSeverity_VALIDATION_SEVERITY_UNSPECIFIED
	}
}

// GetServiceHealth returns the health check timeline for a service.
func (s *Server) GetServiceHealth(_ context.Context, req *hivev1.GetServiceHealthRequest) (*hivev1.GetServiceHealthResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "service name is required")
	}

	if s.healthHistory == nil {
		return &hivev1.GetServiceHealthResponse{
			ServiceName: req.Name,
		}, nil
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	events := s.healthHistory.Get(req.Name, limit)
	healthy, consecutiveFailures := s.healthHistory.CurrentState(req.Name)

	var protoEvents []*hivev1.HealthEvent
	for _, e := range events {
		protoEvents = append(protoEvents, &hivev1.HealthEvent{
			Timestamp:           timestamppb.New(e.Timestamp),
			Healthy:             e.Healthy,
			Message:             e.Message,
			DurationMs:          e.DurationMs,
			CheckType:           e.CheckType,
			ConsecutiveFailures: e.ConsecutiveFailures,
		})
	}

	return &hivev1.GetServiceHealthResponse{
		ServiceName:         req.Name,
		Events:              protoEvents,
		CurrentlyHealthy:    healthy,
		ConsecutiveFailures: int32(consecutiveFailures),
	}, nil
}

// ExportCluster exports all persistent cluster state as a JSON backup.
func (s *Server) ExportCluster(_ context.Context, _ *emptypb.Empty) (*hivev1.ExportClusterResponse, error) {
	b, err := backup.Export(s.store)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "export failed: %v", err)
	}
	data, err := backup.Marshal(b)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal failed: %v", err)
	}
	return &hivev1.ExportClusterResponse{Data: data, Version: b.Version}, nil
}

// ImportCluster restores cluster state from a JSON backup.
// DiffDeploy compares a Hivefile against the currently deployed state and returns
// a diff showing what would change if the Hivefile were deployed.
func (s *Server) DiffDeploy(_ context.Context, req *hivev1.DiffDeployRequest) (*hivev1.DiffDeployResponse, error) {
	if req.HivefileToml == "" {
		return nil, status.Error(codes.InvalidArgument, "hivefile_toml is required")
	}

	hf, err := hivefile.ParseString(req.HivefileToml)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "parse hivefile: %v", err)
	}

	var diffs []*hivev1.ServiceDiff
	for name, svcDef := range hf.Service {
		diff := &hivev1.ServiceDiff{Name: name}

		// Load existing service definition from store
		existingJSON, err := s.store.Get("services", name)
		if err != nil || existingJSON == nil {
			// New service
			diff.Action = hivev1.DiffAction_DIFF_ACTION_CREATE
			diff.NewImage = svcDef.Image
			diff.NewReplicas = uint32(svcDef.Replicas)
			diff.Changes = append(diff.Changes, "new service")
		} else {
			// Existing service — compare
			var oldDef hivefile.ServiceDef
			if jsonErr := json.Unmarshal(existingJSON, &oldDef); jsonErr != nil {
				diff.Action = hivev1.DiffAction_DIFF_ACTION_UPDATE
				diff.NewImage = svcDef.Image
				diff.NewReplicas = uint32(svcDef.Replicas)
				diff.Changes = append(diff.Changes, "stored definition unreadable, treating as update")
				diffs = append(diffs, diff)
				continue
			}

			diff.OldImage = oldDef.Image
			diff.NewImage = svcDef.Image
			diff.OldReplicas = uint32(oldDef.Replicas)
			diff.NewReplicas = uint32(svcDef.Replicas)

			if oldDef.Image != svcDef.Image {
				diff.Changes = append(diff.Changes, fmt.Sprintf("image: %s → %s", oldDef.Image, svcDef.Image))
			}
			if oldDef.Replicas != svcDef.Replicas {
				diff.Changes = append(diff.Changes, fmt.Sprintf("replicas: %d → %d", oldDef.Replicas, svcDef.Replicas))
			}
			// Compare ports
			for hp, cp := range svcDef.Ports {
				if oldCp, ok := oldDef.Ports[hp]; !ok {
					diff.Changes = append(diff.Changes, fmt.Sprintf("+ port %s → %s", hp, cp))
				} else if oldCp != cp {
					diff.Changes = append(diff.Changes, fmt.Sprintf("port %s: %s → %s", hp, oldCp, cp))
				}
			}
			for hp := range oldDef.Ports {
				if _, ok := svcDef.Ports[hp]; !ok {
					diff.Changes = append(diff.Changes, fmt.Sprintf("- port %s", hp))
				}
			}
			// Compare deploy strategy
			if oldDef.Deploy.Strategy != svcDef.Deploy.Strategy {
				diff.Changes = append(diff.Changes, fmt.Sprintf("strategy: %s → %s", oldDef.Deploy.Strategy, svcDef.Deploy.Strategy))
			}

			if len(diff.Changes) > 0 {
				diff.Action = hivev1.DiffAction_DIFF_ACTION_UPDATE
			} else {
				diff.Action = hivev1.DiffAction_DIFF_ACTION_UNCHANGED
			}
		}
		diffs = append(diffs, diff)
	}

	// Sort for deterministic output
	sort.Slice(diffs, func(i, j int) bool { return diffs[i].Name < diffs[j].Name })
	return &hivev1.DiffDeployResponse{Diffs: diffs}, nil
}

func (s *Server) ImportCluster(_ context.Context, req *hivev1.ImportClusterRequest) (*hivev1.ImportClusterResponse, error) {
	b, err := backup.Unmarshal(req.Data)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid backup data: %v", err)
	}
	svc, sec, err := backup.Import(s.store, b, req.Overwrite)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "import failed: %v", err)
	}
	slog.Info("cluster state imported", "services", svc, "secrets", sec, "overwrite", req.Overwrite)
	return &hivev1.ImportClusterResponse{
		ServicesImported: uint32(svc),
		SecretsImported:  uint32(sec),
	}, nil
}
