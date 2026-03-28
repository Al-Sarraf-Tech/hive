package mesh

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/memberlist"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	hivev1 "github.com/jalsarraf0/hive/daemon/internal/api/gen/hive/v1"
	"github.com/jalsarraf0/hive/daemon/internal/pki"
)

// NodeInfo is the metadata broadcast via gossip for each node.
// Must serialize to < 512 bytes (memberlist MetaMaxSize).
// JSON tags use short names to keep the serialized size small.
type NodeInfo struct {
	Name          string   `json:"n" msg:"n"`
	AdvertiseAddr string   `json:"a" msg:"a"`
	GRPCPort      int      `json:"g" msg:"g"`
	MeshPort      int      `json:"m" msg:"m"` // HiveMesh gRPC port (mTLS)
	OS            string   `json:"o" msg:"o"`
	Arch          string   `json:"r" msg:"r"`
	Runtime       string   `json:"t" msg:"t"` // container runtime name
	Platforms     []string `json:"p" msg:"p"` // e.g. ["linux/amd64"]
	Status        int      `json:"s" msg:"s"` // NodeStatus
	Containers    int      `json:"c" msg:"c"` // running container count
	CPUCores      uint32   `json:"C" msg:"C"` // CPU core count
	MemTotal      uint64   `json:"M" msg:"M"` // total memory bytes
	MemAvail      uint64   `json:"A" msg:"A"` // available memory bytes
	DiskTotal     uint64   `json:"D" msg:"D"` // total disk bytes
	DiskAvail     uint64   `json:"d" msg:"d"` // available disk bytes
	WGPubKey      string   `json:"w" msg:"w"` // WireGuard public key (base64)
	WGAddr        string   `json:"W" msg:"W"` // WireGuard mesh IP (10.47.X.X)
	WGPort        int      `json:"U" msg:"U"` // WireGuard UDP listen port
}

// Peer represents a remote node in the cluster.
type Peer struct {
	Info     NodeInfo
	grpcConn *grpc.ClientConn
	client   hivev1.HiveMeshClient
	LastSeen time.Time
}

// MeshClient returns the HiveMesh gRPC client for this peer.
func (p *Peer) MeshClient() hivev1.HiveMeshClient {
	return p.client
}

// MeshEventType is the type of mesh event.
type MeshEventType int

const (
	EventNodeJoined MeshEventType = iota
	EventNodeLeft
	EventNodeFailed
	EventNodeUpdated
)

// MeshEvent is emitted when a node joins, leaves, or fails.
type MeshEvent struct {
	Type MeshEventType
	Node string
	Info NodeInfo
}

// Mesh manages the gossip membership layer and peer gRPC connections.
type Mesh struct {
	mlist     *memberlist.Memberlist
	local     NodeInfo
	peers     map[string]*Peer // keyed by node name
	peersMu   sync.RWMutex
	eventCh   chan MeshEvent
	stopped   atomic.Bool // set during shutdown to prevent sends on closed eventCh
	config    Config
	delegate  *meshDelegate
	wgMesh    wgMeshInterface // optional WireGuard mesh (nil when disabled)
}

// wgMeshInterface is the subset of wgmesh.WGMesh methods used by the mesh layer.
// Defined as an interface to avoid a circular import between mesh and wgmesh.
type wgMeshInterface interface {
	AddPeer(name string, pubKeyBase64 string, endpoint string) error
	RemovePeer(name string) error
}

// SetWGMesh sets the WireGuard mesh instance for automatic peer management.
// When set, peers with WireGuard keys are automatically added/removed as they
// join/leave the gossip cluster.
func (m *Mesh) SetWGMesh(wg wgMeshInterface) {
	m.peersMu.Lock()
	m.wgMesh = wg
	// Add any peers that joined before WG was set (startup race window)
	if wg != nil {
		for name, peer := range m.peers {
			if peer.Info.WGPubKey != "" && peer.Info.WGPort > 0 {
				endpoint := fmt.Sprintf("%s:%d", peer.Info.AdvertiseAddr, peer.Info.WGPort)
				if err := wg.AddPeer(name, peer.Info.WGPubKey, endpoint); err != nil {
					slog.Warn("failed to add existing peer to wireguard", "node", name, "error", err)
				}
			}
		}
	}
	m.peersMu.Unlock()
}

// New creates and starts a new gossip mesh.
func New(cfg Config, containerRuntime string, platforms []string) (*Mesh, error) {
	if cfg.GossipPort == 0 {
		cfg.GossipPort = 7946
	}
	if cfg.GRPCPort == 0 {
		cfg.GRPCPort = 7947
	}
	if cfg.MeshPort == 0 {
		cfg.MeshPort = 7948
	}

	// Auto-detect advertise address if not set
	if cfg.AdvertiseAddr == "" {
		addr, err := detectAdvertiseAddr()
		if err != nil {
			return nil, fmt.Errorf("cannot detect advertise address (use --advertise-addr): %w", err)
		}
		cfg.AdvertiseAddr = addr
		slog.Info("auto-detected advertise address", "addr", addr)
	}

	local := NodeInfo{
		Name:          cfg.NodeName,
		AdvertiseAddr: cfg.AdvertiseAddr,
		GRPCPort:      cfg.GRPCPort,
		MeshPort:      cfg.MeshPort,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		Runtime:       containerRuntime,
		Platforms:     platforms,
		Status:        int(NodeStatusReady),
	}
	if cfg.WireGuardEnabled {
		local.WGPubKey = cfg.WireGuardPubKey
		local.WGAddr = cfg.WireGuardAddr
		local.WGPort = cfg.WireGuardPort
	}

	m := &Mesh{
		local:   local,
		peers:   make(map[string]*Peer),
		eventCh: make(chan MeshEvent, 64),
		config:  cfg,
	}

	m.delegate = &meshDelegate{mesh: m}

	// Configure memberlist
	mlCfg := memberlist.DefaultLANConfig()
	mlCfg.Name = cfg.NodeName
	mlCfg.BindPort = cfg.GossipPort
	mlCfg.AdvertiseAddr = cfg.AdvertiseAddr
	mlCfg.AdvertisePort = cfg.GossipPort
	mlCfg.Delegate = m.delegate
	mlCfg.Events = &meshEventDelegate{mesh: m}
	mlCfg.LogOutput = &slogWriter{level: slog.LevelDebug}

	// Encryption key
	if len(cfg.GossipKey) > 0 {
		keyring, err := memberlist.NewKeyring(nil, cfg.GossipKey)
		if err != nil {
			return nil, fmt.Errorf("invalid gossip key: %w", err)
		}
		mlCfg.Keyring = keyring
	}

	mlist, err := memberlist.Create(mlCfg)
	if err != nil {
		return nil, fmt.Errorf("create memberlist: %w", err)
	}
	m.mlist = mlist

	slog.Info("mesh started",
		"node", cfg.NodeName,
		"gossip", fmt.Sprintf("%s:%d", cfg.AdvertiseAddr, cfg.GossipPort),
		"grpc", fmt.Sprintf("%s:%d", cfg.AdvertiseAddr, cfg.GRPCPort),
	)

	return m, nil
}

// Join connects to existing cluster nodes via their gossip addresses.
func (m *Mesh) Join(addrs []string) (int, error) {
	n, err := m.mlist.Join(addrs)
	if err != nil {
		return 0, fmt.Errorf("join cluster: %w", err)
	}
	return n, nil
}

// Leave gracefully leaves the cluster and closes peer connections.
func (m *Mesh) Leave(timeout time.Duration) error {
	m.stopped.Store(true)
	err := m.mlist.Leave(timeout)
	m.closePeerConns()
	return err
}

// Shutdown stops the mesh and closes all peer connections.
// Order: (1) set stopped flag so late callbacks skip eventCh sends,
// (2) shut down memberlist so no new callbacks fire after return,
// (3) close peer gRPC connections, (4) close eventCh.
// The stopped flag covers the narrow window where memberlist is
// shutting down but callbacks can still fire.
func (m *Mesh) Shutdown() error {
	m.stopped.Store(true)
	err := m.mlist.Shutdown()
	m.closePeerConns()
	close(m.eventCh)
	return err
}

// GossipPort returns the gossip port this mesh is listening on.
func (m *Mesh) GossipPort() int {
	return m.config.GossipPort
}

// LocalNode returns this node's info (thread-safe copy).
func (m *Mesh) LocalNode() NodeInfo {
	m.peersMu.RLock()
	defer m.peersMu.RUnlock()
	return m.local
}

// UpdateContainerCount updates the local node's running container count (thread-safe).
func (m *Mesh) UpdateContainerCount(count int) {
	m.peersMu.Lock()
	m.local.Containers = count
	m.peersMu.Unlock()
}

// SetStatus updates the local node's status (thread-safe).
func (m *Mesh) SetStatus(status int) {
	m.peersMu.Lock()
	m.local.Status = status
	m.peersMu.Unlock()
}

// UpdateResources updates the local node's resource metrics (thread-safe).
func (m *Mesh) UpdateResources(cpuCores uint32, memTotal, memAvail, diskTotal, diskAvail uint64) {
	m.peersMu.Lock()
	m.local.CPUCores = cpuCores
	m.local.MemTotal = memTotal
	m.local.MemAvail = memAvail
	m.local.DiskTotal = diskTotal
	m.local.DiskAvail = diskAvail
	m.peersMu.Unlock()
}

// Peers returns a snapshot of all known remote peers.
// Returns copies of the peer info to avoid data races with concurrent updates.
func (m *Mesh) Peers() []Peer {
	m.peersMu.RLock()
	defer m.peersMu.RUnlock()
	peers := make([]Peer, 0, len(m.peers))
	for _, p := range m.peers {
		peers = append(peers, Peer{
			Info:     p.Info,
			LastSeen: p.LastSeen,
		})
	}
	return peers
}

// PeerByName returns a snapshot of a peer by name, establishing a gRPC connection if needed.
// Thread-safe: uses write lock for the lazy connection establishment.
// The returned snapshot does NOT include the grpcConn to prevent use-after-close:
// if the peer disconnects or its endpoint changes, the internal connection is closed
// by removePeer/NotifyUpdate; callers holding a snapshot with the old conn would
// hit a closed transport. The gRPC client stub is safe to use — RPCs on a closed
// conn return a transport error rather than causing undefined behavior.
func (m *Mesh) PeerByName(name string) (*Peer, error) {
	m.peersMu.Lock()
	defer m.peersMu.Unlock()
	peer, ok := m.peers[name]
	if !ok {
		return nil, fmt.Errorf("peer %q not found", name)
	}

	// Lazy gRPC connection
	if peer.grpcConn == nil {
		conn, err := m.dialPeer(peer.Info)
		if err != nil {
			return nil, fmt.Errorf("connect to peer %q: %w", name, err)
		}
		peer.grpcConn = conn
		peer.client = hivev1.NewHiveMeshClient(conn)
	}

	// Return a snapshot copy WITHOUT grpcConn — caller gets the client stub only.
	// This prevents use-after-close if the internal conn is reset concurrently.
	return &Peer{
		Info:     peer.Info,
		client:   peer.client,
		LastSeen: peer.LastSeen,
	}, nil
}

// PeerCount returns the number of known peers (excluding self).
func (m *Mesh) PeerCount() int {
	m.peersMu.RLock()
	defer m.peersMu.RUnlock()
	return len(m.peers)
}

// Events returns the channel for mesh events.
func (m *Mesh) Events() <-chan MeshEvent {
	return m.eventCh
}

// Members returns the total member count including self.
func (m *Mesh) Members() int {
	return m.mlist.NumMembers()
}

// dialPeer establishes a gRPC connection to a peer's mesh port.
// Uses mTLS when TLS is enabled, insecure transport otherwise.
func (m *Mesh) dialPeer(info NodeInfo) (*grpc.ClientConn, error) {
	// Use mesh port for daemon-to-daemon communication
	port := info.MeshPort
	if port == 0 {
		port = info.GRPCPort // fallback for peers that haven't been updated
	}
	if port == 0 {
		return nil, fmt.Errorf("peer %s has no gRPC/mesh port", info.Name)
	}

	// NOTE: WireGuard mesh address routing is deferred until the WG device
	// is fully operational (Phase 2). Until then, always use AdvertiseAddr.
	// Future: if m.config.WireGuardEnabled && info.WGAddr != "" && m.wgDeviceUp { use WGAddr }
	addr := fmt.Sprintf("%s:%d", info.AdvertiseAddr, port)

	var creds grpc.DialOption
	if m.config.TLSEnabled && m.config.DataDir != "" {
		tlsCfg, err := pki.MeshClientTLSConfig(m.config.DataDir)
		if err != nil {
			return nil, fmt.Errorf("load mesh TLS config: %w", err)
		}
		// Set ServerName to the peer's node name (matches cert CN)
		tlsCfg.ServerName = info.Name
		creds = grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg))
	} else {
		creds = grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	conn, err := grpc.NewClient(addr, creds)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	return conn, nil
}

// closePeerConns closes all cached gRPC connections.
func (m *Mesh) closePeerConns() {
	m.peersMu.Lock()
	defer m.peersMu.Unlock()
	for _, p := range m.peers {
		if p.grpcConn != nil {
			p.grpcConn.Close()
			p.grpcConn = nil
			p.client = nil
		}
	}
}

// detectAdvertiseAddr finds the first non-loopback IPv4 address.
func detectAdvertiseAddr() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	// Sort interfaces by name for deterministic selection
	sort.Slice(ifaces, func(i, j int) bool {
		return ifaces[i].Name < ifaces[j].Name
	})
	for _, iface := range ifaces {
		// Skip loopback, down interfaces, and common virtual prefixes
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil && !ipnet.IP.IsLoopback() {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no suitable non-loopback IPv4 address found")
}

// slogWriter adapts slog for memberlist's log output.
type slogWriter struct {
	level slog.Level
}

func (w *slogWriter) Write(p []byte) (n int, err error) {
	slog.Log(context.Background(), w.level, string(p))
	return len(p), nil
}
