// Package mesh implements the SWIM gossip mesh for Hive node discovery and membership.
package mesh

// Config holds mesh configuration.
type Config struct {
	NodeName      string // Required: unique name for this node
	AdvertiseAddr string // IP address to advertise (auto-detect if empty)
	GRPCPort      int    // gRPC port for HiveMesh service (default 7947)
	GossipPort    int    // SWIM gossip port (default 7946)
	GossipKey     []byte // Optional shared encryption key for gossip traffic
}

// NodeStatus represents a node's operational state.
type NodeStatus int

const (
	NodeStatusReady    NodeStatus = 1
	NodeStatusDraining NodeStatus = 2
	NodeStatusDown     NodeStatus = 3
)
