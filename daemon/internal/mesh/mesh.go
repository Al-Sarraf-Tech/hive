package mesh

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	hivev1 "github.com/jalsarraf0/hive/daemon/internal/api/gen/hive/v1"
)

// NodeInfo is the metadata broadcast via gossip for each node.
// Must serialize to < 512 bytes (memberlist MetaMaxSize).
// JSON tags use short names to keep the serialized size small.
type NodeInfo struct {
	Name          string   `json:"n" msg:"n"`
	AdvertiseAddr string   `json:"a" msg:"a"`
	GRPCPort      int      `json:"g" msg:"g"`
	OS            string   `json:"o" msg:"o"`
	Arch          string   `json:"r" msg:"r"`
	Runtime       string   `json:"t" msg:"t"` // container runtime name
	Platforms     []string `json:"p" msg:"p"` // e.g. ["linux/amd64"]
	Status        int      `json:"s" msg:"s"` // NodeStatus
	Containers    int      `json:"c" msg:"c"` // running container count
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
	config    Config
	delegate  *meshDelegate
}

// New creates and starts a new gossip mesh.
func New(cfg Config, containerRuntime string, platforms []string) (*Mesh, error) {
	if cfg.GossipPort == 0 {
		cfg.GossipPort = 7946
	}
	if cfg.GRPCPort == 0 {
		cfg.GRPCPort = 7947
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
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		Runtime:       containerRuntime,
		Platforms:     platforms,
		Status:        int(NodeStatusReady),
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

// Leave gracefully leaves the cluster.
func (m *Mesh) Leave(timeout time.Duration) error {
	return m.mlist.Leave(timeout)
}

// Shutdown stops the mesh and closes all peer connections.
func (m *Mesh) Shutdown() error {
	m.closePeerConns()
	close(m.eventCh)
	return m.mlist.Shutdown()
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
// Returns a copy so that concurrent NotifyUpdate calls that reset the connection
// do not race with the caller using the returned Peer.
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

	// Return a snapshot copy to avoid data races with concurrent connection resets
	return &Peer{
		Info:     peer.Info,
		grpcConn: peer.grpcConn,
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

// dialPeer establishes a gRPC connection to a peer.
func (m *Mesh) dialPeer(info NodeInfo) (*grpc.ClientConn, error) {
	if info.GRPCPort == 0 {
		return nil, fmt.Errorf("peer %s has no gRPC port", info.Name)
	}
	addr := fmt.Sprintf("%s:%d", info.AdvertiseAddr, info.GRPCPort)
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
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
