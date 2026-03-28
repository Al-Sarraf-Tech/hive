package wgmesh

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// WGMesh manages a WireGuard mesh overlay network.
// Each Hive node gets a deterministic 10.47.X.X address derived from its public key.
// Peers are added/removed dynamically as nodes join/leave the cluster via gossip.
//
// The userspace WireGuard device is created via netstack (gvisor TCP/IP stack),
// so no root privileges or kernel WireGuard module are required. Traffic between
// Hive nodes is encrypted end-to-end through the WireGuard tunnel.
type WGMesh struct {
	privateKey wgtypes.Key
	publicKey  wgtypes.Key
	listenPort int
	meshIP     string // 10.47.X.X
	dataDir    string

	// WireGuard device and userspace network stack
	device   *device.Device  // userspace WireGuard device
	tnet     *netstack.Net   // userspace TCP/IP stack for dialing through tunnel
	deviceUp atomic.Bool     // true after device.Up() succeeds (atomic for lock-free reads)

	mu    sync.RWMutex
	peers map[string]peerInfo // nodeName -> peer config
}

type peerInfo struct {
	pubKey   wgtypes.Key
	endpoint string // host:port
	meshIP   string // 10.47.X.X
}

// New creates a new WGMesh instance. It loads or generates a WireGuard private key
// from <dataDir>/wg/private.key, derives the public key, and computes a deterministic
// mesh IP address.
func New(dataDir string, listenPort int) (*WGMesh, error) {
	wgDir := filepath.Join(dataDir, "wg")
	if err := os.MkdirAll(wgDir, 0700); err != nil {
		return nil, fmt.Errorf("create wireguard data dir: %w", err)
	}

	keyPath := filepath.Join(wgDir, "private.key")
	var privateKey wgtypes.Key

	data, err := os.ReadFile(keyPath)
	if err == nil {
		// Validate file permissions (security: private key must not be world-readable)
		if fi, statErr := os.Stat(keyPath); statErr == nil {
			if runtime.GOOS != "windows" && fi.Mode().Perm()&0o077 != 0 {
				return nil, fmt.Errorf("wireguard private key %s has insecure permissions %04o (must be 0600)", keyPath, fi.Mode().Perm())
			}
		}
		// Parse existing key (base64-encoded, may have trailing newline)
		decoded, decErr := base64.StdEncoding.DecodeString(string(trimNewline(data)))
		if decErr != nil {
			return nil, fmt.Errorf("decode wireguard private key from %s: %w", keyPath, decErr)
		}
		if len(decoded) != wgtypes.KeyLen {
			return nil, fmt.Errorf("wireguard private key has wrong length: got %d, want %d", len(decoded), wgtypes.KeyLen)
		}
		privateKey = wgtypes.Key(decoded)
	} else if os.IsNotExist(err) {
		// Generate new key
		privateKey, err = wgtypes.GeneratePrivateKey()
		if err != nil {
			return nil, fmt.Errorf("generate wireguard private key: %w", err)
		}
		encoded := base64.StdEncoding.EncodeToString(privateKey[:])
		if err := os.WriteFile(keyPath, []byte(encoded+"\n"), 0600); err != nil {
			return nil, fmt.Errorf("save wireguard private key to %s: %w", keyPath, err)
		}
	} else {
		return nil, fmt.Errorf("read wireguard private key from %s: %w", keyPath, err)
	}

	publicKey := privateKey.PublicKey()
	meshIP := MeshIPFromKey(publicKey)

	// Create userspace TUN device backed by a gvisor TCP/IP stack.
	// This avoids requiring root or a kernel WireGuard module.
	localAddr := netip.MustParseAddr(meshIP)
	tunDev, tnet, err := netstack.CreateNetTUN(
		[]netip.Addr{localAddr},
		nil,  // no DNS servers needed for mesh traffic
		1420, // standard WireGuard MTU
	)
	if err != nil {
		return nil, fmt.Errorf("create wireguard tun: %w", err)
	}

	// Create the userspace WireGuard device
	logger := device.NewLogger(device.LogLevelError, "wg: ")
	dev := device.NewDevice(tunDev, conn.NewDefaultBind(), logger)

	// Configure the device with our private key and listen port via the
	// WireGuard IPC protocol. Keys must be hex-encoded (not base64).
	ipcConfig := fmt.Sprintf("private_key=%s\nlisten_port=%d\n",
		hex.EncodeToString(privateKey[:]),
		listenPort,
	)
	if err := dev.IpcSet(ipcConfig); err != nil {
		dev.Close()
		return nil, fmt.Errorf("configure wireguard device: %w", err)
	}

	// Bring the device up so it starts processing packets
	if err := dev.Up(); err != nil {
		dev.Close()
		return nil, fmt.Errorf("bring up wireguard device: %w", err)
	}

	slog.Info("wireguard mesh initialized",
		"public_key", base64.StdEncoding.EncodeToString(publicKey[:]),
		"mesh_ip", meshIP,
		"listen_port", listenPort,
	)

	wg := &WGMesh{
		privateKey: privateKey,
		publicKey:  publicKey,
		listenPort: listenPort,
		meshIP:     meshIP,
		dataDir:    dataDir,
		device:     dev,
		tnet:       tnet,
		peers:      make(map[string]peerInfo),
	}
	wg.deviceUp.Store(true)
	return wg, nil
}

// PublicKey returns the base64-encoded WireGuard public key for this node.
func (w *WGMesh) PublicKey() string {
	return base64.StdEncoding.EncodeToString(w.publicKey[:])
}

// MeshIP returns the deterministic 10.47.X.X mesh address for this node.
func (w *WGMesh) MeshIP() string {
	return w.meshIP
}

// ListenPort returns the configured WireGuard UDP listen port.
func (w *WGMesh) ListenPort() int {
	return w.listenPort
}

// AddPeer registers a remote node's WireGuard public key and endpoint.
// The peer's mesh IP is computed deterministically from its public key.
// If the WireGuard device is active, the peer is also configured on the device
// so tunnel traffic can flow immediately.
func (w *WGMesh) AddPeer(name string, pubKeyBase64 string, endpoint string) error {
	decoded, err := base64.StdEncoding.DecodeString(pubKeyBase64)
	if err != nil {
		return fmt.Errorf("decode peer %q public key: %w", name, err)
	}
	if len(decoded) != wgtypes.KeyLen {
		return fmt.Errorf("peer %q public key has wrong length: got %d, want %d", name, len(decoded), wgtypes.KeyLen)
	}
	pubKey := wgtypes.Key(decoded)
	meshIP := MeshIPFromKey(pubKey)

	w.mu.Lock()
	// Check for IP collision with local node
	if meshIP == w.meshIP {
		w.mu.Unlock()
		slog.Warn("wireguard mesh IP collision with local node", "peer", name, "ip", meshIP)
		return fmt.Errorf("mesh IP collision: peer %q maps to %s (same as this node)", name, meshIP)
	}
	// Check for IP collision with existing peers
	for existingName, existing := range w.peers {
		if existing.meshIP == meshIP && existingName != name {
			w.mu.Unlock()
			slog.Warn("wireguard mesh IP collision detected",
				"new_peer", name, "existing_peer", existingName, "ip", meshIP)
			return fmt.Errorf("mesh IP collision: %s and %s both map to %s", name, existingName, meshIP)
		}
	}
	w.peers[name] = peerInfo{
		pubKey:   pubKey,
		endpoint: endpoint,
		meshIP:   meshIP,
	}

	// Configure the peer on the WireGuard device under the same lock to prevent
	// race conditions with RemovePeer. Rollback if IpcSet fails.
	if w.device != nil && w.deviceUp.Load() {
		peerConfig := fmt.Sprintf("public_key=%s\nendpoint=%s\nallowed_ip=%s/32\npersistent_keepalive_interval=25\n",
			hex.EncodeToString(pubKey[:]),
			endpoint,
			meshIP,
		)
		if err := w.device.IpcSet(peerConfig); err != nil {
			delete(w.peers, name) // rollback
			w.mu.Unlock()
			return fmt.Errorf("configure wireguard peer %q on device: %w", name, err)
		}
	}
	w.mu.Unlock()

	slog.Info("wireguard peer added", "name", name, "mesh_ip", meshIP, "endpoint", endpoint)
	return nil
}

// RemovePeer removes a remote node from the WireGuard peer table and device.
func (w *WGMesh) RemovePeer(name string) error {
	w.mu.Lock()
	peer, existed := w.peers[name]
	delete(w.peers, name)
	// Remove from device under the same lock to prevent race with AddPeer
	if existed && w.device != nil && w.deviceUp.Load() {
		removeConfig := fmt.Sprintf("public_key=%s\nremove=true\n",
			hex.EncodeToString(peer.pubKey[:]),
		)
		if err := w.device.IpcSet(removeConfig); err != nil {
			slog.Warn("failed to remove wireguard peer from device",
				"name", name, "error", err)
		}
	}
	w.mu.Unlock()

	if existed {
		slog.Info("wireguard peer removed", "name", name)
	}
	return nil
}

// PeerCount returns the number of tracked WireGuard peers.
func (w *WGMesh) PeerCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.peers)
}

// IsUp returns whether the WireGuard device is operational and can route traffic.
func (w *WGMesh) IsUp() bool {
	return w.device != nil && w.deviceUp.Load()
}

// DialTCP dials a TCP address through the WireGuard tunnel using the userspace
// network stack. This allows Hive daemon-to-daemon communication over encrypted
// WireGuard links without touching the host network stack.
func (w *WGMesh) DialTCP(ip string, port int) (net.Conn, error) {
	w.mu.RLock()
	tnet := w.tnet
	w.mu.RUnlock()
	if tnet == nil {
		return nil, fmt.Errorf("wireguard tunnel not active")
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return nil, fmt.Errorf("parse wireguard peer address %q: %w", ip, err)
	}
	return tnet.DialTCPAddrPort(netip.AddrPortFrom(addr, uint16(port)))
}

// DialContext dials a TCP connection through the WireGuard tunnel with context support.
// The network parameter should be "tcp", "tcp4", or "tcp6".
func (w *WGMesh) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	w.mu.RLock()
	tnet := w.tnet
	w.mu.RUnlock()
	if tnet == nil {
		return nil, fmt.Errorf("wireguard tunnel not active")
	}
	return tnet.DialContext(ctx, network, address)
}

// Close shuts down the WireGuard device and releases all resources.
func (w *WGMesh) Close() error {
	w.deviceUp.Store(false)

	w.mu.Lock()
	dev := w.device
	w.device = nil
	w.tnet = nil
	w.peers = make(map[string]peerInfo) // clear stale state
	w.mu.Unlock()

	if dev != nil {
		dev.Close()
		slog.Info("wireguard device closed")
	}
	slog.Info("wireguard mesh closed")
	return nil
}

// trimNewline strips trailing newlines/carriage returns from key file content.
func trimNewline(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}
