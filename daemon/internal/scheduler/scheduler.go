// Package scheduler determines which node should run a given service.
package scheduler

import (
	"fmt"
	"sort"

	"github.com/jalsarraf0/hive/daemon/internal/hivefile"
	"github.com/jalsarraf0/hive/daemon/internal/mesh"
)

// Candidate is a node that can run a service, with a fitness score.
type Candidate struct {
	NodeName string
	Score    float64
	Local    bool // true if this is the local node
}

// Scheduler picks the best node for a service based on constraints and scoring.
type Scheduler struct {
	mesh      *mesh.Mesh
	localName string
}

// New creates a scheduler that uses the mesh for peer awareness.
func New(m *mesh.Mesh, localName string) *Scheduler {
	return &Scheduler{mesh: m, localName: localName}
}

// Pick selects the best node to run the given service.
// Returns an error if no node satisfies the constraints.
func (s *Scheduler) Pick(svc hivefile.ServiceDef) (Candidate, error) {
	candidates := s.buildCandidates(svc)

	if len(candidates) == 0 {
		return Candidate{}, fmt.Errorf("no available node satisfies constraints for service (node=%q, platform=%q)", svc.Node, svc.Platform)
	}

	// Sort by score descending
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].NodeName < candidates[j].NodeName
	})

	return candidates[0], nil
}

func (s *Scheduler) buildCandidates(svc hivefile.ServiceDef) []Candidate {
	if s.mesh == nil {
		return nil
	}

	var candidates []Candidate

	// Check local node (skip if draining or down)
	local := s.mesh.LocalNode()
	if local.Status == int(mesh.NodeStatusReady) && s.matchesConstraints(local, svc) {
		candidates = append(candidates, Candidate{
			NodeName: local.Name,
			Score:    s.score(local, true),
			Local:    true,
		})
	}

	// Check remote peers
	for _, peer := range s.mesh.Peers() {
		if peer.Info.Status != int(mesh.NodeStatusReady) {
			continue // skip draining/down nodes
		}
		if s.matchesConstraints(peer.Info, svc) {
			candidates = append(candidates, Candidate{
				NodeName: peer.Info.Name,
				Score:    s.score(peer.Info, false),
				Local:    false,
			})
		}
	}

	return candidates
}

// matchesConstraints checks hard constraints: node pin and platform.
func (s *Scheduler) matchesConstraints(node mesh.NodeInfo, svc hivefile.ServiceDef) bool {
	// Node pin constraint
	if svc.Node != "" && svc.Node != node.Name {
		return false
	}

	// Platform constraint
	if svc.Platform != "" {
		found := false
		for _, p := range node.Platforms {
			if p == svc.Platform {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Memory constraint: reject nodes that don't have enough available memory
	if svc.Resources.Memory != "" {
		required, err := hivefile.ParseMemory(svc.Resources.Memory)
		if err == nil && required > 0 {
			// Only reject if we have resource info for this node
			if node.MemAvail > 0 && node.MemAvail < uint64(required) {
				return false
			}
		}
	}

	return true
}

// score computes a fitness score for a node. Higher is better.
func (s *Scheduler) score(node mesh.NodeInfo, isLocal bool) float64 {
	score := 100.0

	// Spread: fewer containers = higher score (max 50 points)
	containerPenalty := float64(node.Containers) * 5.0
	if containerPenalty > 50 {
		containerPenalty = 50
	}
	score -= containerPenalty

	// Memory headroom bonus (0-10 points)
	if node.MemTotal > 0 {
		pct := float64(node.MemAvail) / float64(node.MemTotal) * 100
		score += pct / 10 // up to +10 for nodes with more free memory
	}

	// CPU core bonus: prefer nodes with more cores (+1 per 4 cores, capped at +5)
	if node.CPUCores > 0 {
		cpuBonus := float64(node.CPUCores) / 4.0
		if cpuBonus > 5 {
			cpuBonus = 5
		}
		score += cpuBonus
	}

	// Local bonus: small preference for local to avoid unnecessary network hops
	if isLocal {
		score += 5.0
	}

	return score
}
