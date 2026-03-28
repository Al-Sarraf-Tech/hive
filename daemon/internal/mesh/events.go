package mesh

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/hashicorp/memberlist"
)

// meshEventDelegate implements memberlist.EventDelegate for node join/leave events.
type meshEventDelegate struct {
	mesh *Mesh
}

func (d *meshEventDelegate) NotifyJoin(node *memberlist.Node) {
	d.mesh.peersMu.RLock()
	localName := d.mesh.local.Name
	d.mesh.peersMu.RUnlock()
	if node.Name == localName {
		return // ignore self
	}

	info, err := DecodeNodeMeta(node.Meta)
	if err != nil {
		slog.Warn("failed to decode node meta on join", "node", node.Name, "error", err)
		info = NodeInfo{
			Name:          node.Name,
			AdvertiseAddr: node.Addr.String(),
			Status:        int(NodeStatusReady),
		}
	}

	d.mesh.updatePeer(info)

	// Add WireGuard peer if the joining node advertises a WG public key
	d.mesh.peersMu.RLock()
	wg := d.mesh.wgMesh
	d.mesh.peersMu.RUnlock()
	if wg != nil && info.WGPubKey != "" && info.WGPort > 0 {
		endpoint := fmt.Sprintf("%s:%d", info.AdvertiseAddr, info.WGPort)
		if err := wg.AddPeer(info.Name, info.WGPubKey, endpoint); err != nil {
			slog.Warn("failed to add wireguard peer on join", "node", info.Name, "error", err)
		}
	}

	slog.Info("node joined cluster", "node", node.Name, "addr", node.Addr)
	if !d.mesh.stopped.Load() {
		select {
		case d.mesh.eventCh <- MeshEvent{Type: EventNodeJoined, Node: node.Name, Info: info}:
		default:
			slog.Warn("event channel full, dropped node join event", "node", node.Name)
		}
	}
}

func (d *meshEventDelegate) NotifyLeave(node *memberlist.Node) {
	d.mesh.peersMu.RLock()
	localName := d.mesh.local.Name
	d.mesh.peersMu.RUnlock()
	if node.Name == localName {
		return
	}

	d.mesh.removePeer(node.Name)

	// Remove WireGuard peer when node leaves
	d.mesh.peersMu.RLock()
	wg := d.mesh.wgMesh
	d.mesh.peersMu.RUnlock()
	if wg != nil {
		if err := wg.RemovePeer(node.Name); err != nil {
			slog.Warn("failed to remove wireguard peer on leave", "node", node.Name, "error", err)
		}
	}

	slog.Info("node left cluster", "node", node.Name)
	if !d.mesh.stopped.Load() {
		select {
		case d.mesh.eventCh <- MeshEvent{Type: EventNodeLeft, Node: node.Name}:
		default:
			slog.Warn("event channel full, dropped node leave event", "node", node.Name)
		}
	}
}

func (d *meshEventDelegate) NotifyUpdate(node *memberlist.Node) {
	d.mesh.peersMu.RLock()
	localName := d.mesh.local.Name
	d.mesh.peersMu.RUnlock()
	if node.Name == localName {
		return
	}

	info, err := DecodeNodeMeta(node.Meta)
	if err != nil {
		slog.Debug("failed to decode node meta on update", "node", node.Name, "error", err)
		return
	}

	// Close stale gRPC connection and update peer atomically to avoid TOCTOU race.
	// Capture old info BEFORE overwriting for WireGuard change detection.
	d.mesh.peersMu.Lock()
	existing, ok := d.mesh.peers[node.Name]
	var oldInfo NodeInfo
	if ok {
		oldInfo = existing.Info
	}
	if ok && (oldInfo.AdvertiseAddr != info.AdvertiseAddr || oldInfo.GRPCPort != info.GRPCPort || oldInfo.MeshPort != info.MeshPort) {
		slog.Info("peer endpoint changed, resetting connection",
			"node", node.Name,
			"old", fmt.Sprintf("%s:%d", oldInfo.AdvertiseAddr, oldInfo.GRPCPort),
			"new", fmt.Sprintf("%s:%d", info.AdvertiseAddr, info.GRPCPort),
		)
		if existing.grpcConn != nil {
			existing.grpcConn.Close()
		}
		existing.grpcConn = nil
		existing.client = nil
	}
	if existing != nil {
		existing.Info = info
		existing.LastSeen = time.Now()
	} else {
		d.mesh.peers[node.Name] = &Peer{Info: info, LastSeen: time.Now()}
	}
	wg := d.mesh.wgMesh
	d.mesh.peersMu.Unlock()

	// Sync WireGuard peer if WG fields changed (compare against OLD info)
	if wg != nil && info.WGPubKey != "" && info.WGPort > 0 {
		if ok && (oldInfo.WGPubKey != info.WGPubKey || oldInfo.WGPort != info.WGPort || oldInfo.AdvertiseAddr != info.AdvertiseAddr) {
			_ = wg.RemovePeer(info.Name)
			endpoint := fmt.Sprintf("%s:%d", info.AdvertiseAddr, info.WGPort)
			if err := wg.AddPeer(info.Name, info.WGPubKey, endpoint); err != nil {
				slog.Warn("failed to update wireguard peer", "node", info.Name, "error", err)
			}
		}
	}

	if !d.mesh.stopped.Load() {
		select {
		case d.mesh.eventCh <- MeshEvent{Type: EventNodeUpdated, Node: node.Name, Info: info}:
		default:
			slog.Warn("event channel full, dropped node update event", "node", node.Name)
		}
	}
}

// updatePeer creates or updates a peer entry.
func (m *Mesh) updatePeer(info NodeInfo) {
	m.peersMu.Lock()
	defer m.peersMu.Unlock()

	if p, ok := m.peers[info.Name]; ok {
		p.Info = info
		p.LastSeen = time.Now()
	} else {
		m.peers[info.Name] = &Peer{
			Info:     info,
			LastSeen: time.Now(),
		}
	}
}

// removePeer removes a peer and closes its gRPC connection.
func (m *Mesh) removePeer(name string) {
	m.peersMu.Lock()
	defer m.peersMu.Unlock()

	if p, ok := m.peers[name]; ok {
		if p.grpcConn != nil {
			p.grpcConn.Close()
		}
		delete(m.peers, name)
	}
}
