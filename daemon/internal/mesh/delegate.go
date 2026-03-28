package mesh

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

// meshDelegate implements memberlist.Delegate for metadata exchange.
type meshDelegate struct {
	mesh *Mesh
}

// NodeMeta returns the local node's metadata, serialized to bytes.
// Must be < 512 bytes (memberlist MetaMaxSize).
// Thread-safe: acquires RLock to read m.local.
func (d *meshDelegate) NodeMeta(limit int) []byte {
	d.mesh.peersMu.RLock()
	localCopy := d.mesh.local
	d.mesh.peersMu.RUnlock()

	data, err := json.Marshal(localCopy)
	if err != nil {
		slog.Error("failed to marshal node meta", "error", err)
		return nil
	}
	if len(data) > limit {
		slog.Error("node meta exceeds limit", "size", len(data), "limit", limit)
		return nil
	}
	return data
}

// NotifyMsg is called when a user-data message is received.
func (d *meshDelegate) NotifyMsg(msg []byte) {
	_ = msg
}

// GetBroadcasts returns any queued broadcasts to piggyback on gossip.
func (d *meshDelegate) GetBroadcasts(overhead, limit int) [][]byte {
	return nil
}

// LocalState is called during push-pull anti-entropy exchange.
func (d *meshDelegate) LocalState(join bool) []byte {
	d.mesh.peersMu.RLock()
	localCopy := d.mesh.local
	d.mesh.peersMu.RUnlock()

	data, err := json.Marshal(localCopy)
	if err != nil {
		slog.Error("failed to marshal local state for push-pull", "error", err)
		return nil
	}
	return data
}

// MergeRemoteState is called during push-pull to merge a remote node's state.
// Thread-safe: acquires RLock to read local name for self-check.
func (d *meshDelegate) MergeRemoteState(buf []byte, join bool) {
	var info NodeInfo
	if err := json.Unmarshal(buf, &info); err != nil {
		slog.Debug("failed to unmarshal remote state", "error", err)
		return
	}

	d.mesh.peersMu.RLock()
	localName := d.mesh.local.Name
	d.mesh.peersMu.RUnlock()

	if info.Name == "" || info.Name == localName {
		return
	}
	d.mesh.updatePeer(info)

	// Add WireGuard peer if applicable (anti-entropy may discover peers
	// that were missed by NotifyJoin during network partitions)
	d.mesh.peersMu.RLock()
	wg := d.mesh.wgMesh
	d.mesh.peersMu.RUnlock()
	if wg != nil && info.WGPubKey != "" && info.WGPort > 0 {
		endpoint := fmt.Sprintf("%s:%d", info.AdvertiseAddr, info.WGPort)
		_ = wg.AddPeer(info.Name, info.WGPubKey, endpoint) // idempotent
	}
}

// DecodeNodeMeta deserializes NodeInfo from memberlist node metadata.
func DecodeNodeMeta(data []byte) (NodeInfo, error) {
	var info NodeInfo
	err := json.Unmarshal(data, &info)
	return info, err
}
