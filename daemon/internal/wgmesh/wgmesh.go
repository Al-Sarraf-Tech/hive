package wgmesh

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// WGMesh manages a WireGuard mesh overlay network.
// Each Hive node gets a deterministic 10.47.X.X address derived from its public key.
// Peers are added/removed dynamically as nodes join/leave the cluster via gossip.
//
// This initial implementation handles key management and peer tracking.
// Actual WireGuard device creation (TUN interface, netstack) is deferred to a
// follow-up phase once gossip-based key exchange is verified.
type WGMesh struct {
	privateKey wgtypes.Key
	publicKey  wgtypes.Key
	listenPort int
	meshIP     string // 10.47.X.X/16
	dataDir    string

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

	slog.Info("wireguard mesh initialized",
		"public_key", base64.StdEncoding.EncodeToString(publicKey[:]),
		"mesh_ip", meshIP,
		"listen_port", listenPort,
	)

	return &WGMesh{
		privateKey: privateKey,
		publicKey:  publicKey,
		listenPort: listenPort,
		meshIP:     meshIP,
		dataDir:    dataDir,
		peers:      make(map[string]peerInfo),
	}, nil
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
//
// Note: actual WireGuard device configuration is deferred to a later phase.
// For now, this tracks peer info so the gossip layer can exchange keys.
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
	w.mu.Unlock()

	slog.Info("wireguard peer added", "name", name, "mesh_ip", meshIP, "endpoint", endpoint)
	return nil
}

// RemovePeer removes a remote node from the WireGuard peer table.
func (w *WGMesh) RemovePeer(name string) error {
	w.mu.Lock()
	_, existed := w.peers[name]
	delete(w.peers, name)
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

// Close cleans up WGMesh resources. Currently a no-op since the actual
// WireGuard device is not yet created; exists for forward compatibility.
func (w *WGMesh) Close() error {
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
