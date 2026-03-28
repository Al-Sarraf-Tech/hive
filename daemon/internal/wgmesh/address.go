package wgmesh

import (
	"crypto/sha256"
	"fmt"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// MeshIPFromKey computes a deterministic 10.47.X.X address from a WireGuard public key.
// Uses SHA-256 of the public key bytes, taking the first 2 bytes for the last 2 octets.
// This provides ~65K unique addresses which is more than sufficient for a Hive cluster.
func MeshIPFromKey(pubKey wgtypes.Key) string {
	hash := sha256.Sum256(pubKey[:])
	return fmt.Sprintf("10.47.%d.%d", hash[0], hash[1])
}
