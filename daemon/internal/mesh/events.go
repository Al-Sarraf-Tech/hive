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

	slog.Info("node joined cluster", "node", node.Name, "addr", node.Addr)
	select {
	case d.mesh.eventCh <- MeshEvent{Type: EventNodeJoined, Node: node.Name, Info: info}:
	default:
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

	slog.Info("node left cluster", "node", node.Name)
	select {
	case d.mesh.eventCh <- MeshEvent{Type: EventNodeLeft, Node: node.Name}:
	default:
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

	// Close stale gRPC connection if address or gRPC port changed (write lock for safe mutation)
	d.mesh.peersMu.Lock()
	existing, ok := d.mesh.peers[node.Name]
	if ok && (existing.Info.AdvertiseAddr != info.AdvertiseAddr || existing.Info.GRPCPort != info.GRPCPort) {
		slog.Info("peer endpoint changed, resetting connection",
			"node", node.Name,
			"old", fmt.Sprintf("%s:%d", existing.Info.AdvertiseAddr, existing.Info.GRPCPort),
			"new", fmt.Sprintf("%s:%d", info.AdvertiseAddr, info.GRPCPort),
		)
		if existing.grpcConn != nil {
			existing.grpcConn.Close()
		}
		existing.grpcConn = nil
		existing.client = nil
	}
	d.mesh.peersMu.Unlock()

	d.mesh.updatePeer(info)

	select {
	case d.mesh.eventCh <- MeshEvent{Type: EventNodeUpdated, Node: node.Name, Info: info}:
	default:
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
