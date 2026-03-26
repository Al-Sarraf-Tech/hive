package scheduler

import (
	"testing"

	"github.com/jalsarraf0/hive/daemon/internal/hivefile"
	"github.com/jalsarraf0/hive/daemon/internal/mesh"
)

func TestMatchesConstraintsNodePin(t *testing.T) {
	s := &Scheduler{localName: "nodeA"}
	node := mesh.NodeInfo{Name: "nodeA", Platforms: []string{"linux/amd64"}}
	svc := hivefile.ServiceDef{Node: "nodeA"}

	if !s.matchesConstraints(node, svc) {
		t.Error("pinned node should match")
	}

	svc.Node = "nodeB"
	if s.matchesConstraints(node, svc) {
		t.Error("wrong node should not match")
	}
}

func TestMatchesConstraintsPlatform(t *testing.T) {
	s := &Scheduler{localName: "test"}
	node := mesh.NodeInfo{Name: "test", Platforms: []string{"linux/amd64"}}

	svc := hivefile.ServiceDef{Platform: "linux/amd64"}
	if !s.matchesConstraints(node, svc) {
		t.Error("matching platform should match")
	}

	svc.Platform = "windows/amd64"
	if s.matchesConstraints(node, svc) {
		t.Error("non-matching platform should not match")
	}

	svc.Platform = ""
	if !s.matchesConstraints(node, svc) {
		t.Error("empty platform should match any node")
	}
}

func TestMatchesConstraintsNoConstraints(t *testing.T) {
	s := &Scheduler{localName: "test"}
	node := mesh.NodeInfo{Name: "test", Platforms: []string{"linux/amd64"}}
	svc := hivefile.ServiceDef{} // no constraints

	if !s.matchesConstraints(node, svc) {
		t.Error("no constraints should match any node")
	}
}

func TestScoreSpread(t *testing.T) {
	s := &Scheduler{localName: "test"}

	lightly := mesh.NodeInfo{Containers: 1}
	heavily := mesh.NodeInfo{Containers: 8}

	lightScore := s.score(lightly, false)
	heavyScore := s.score(heavily, false)

	if lightScore <= heavyScore {
		t.Errorf("lightly loaded node (%f) should score higher than heavy (%f)", lightScore, heavyScore)
	}
}

func TestScoreLocalBonus(t *testing.T) {
	s := &Scheduler{localName: "test"}
	node := mesh.NodeInfo{Containers: 2}

	localScore := s.score(node, true)
	remoteScore := s.score(node, false)

	if localScore <= remoteScore {
		t.Errorf("local node (%f) should have higher score than remote (%f)", localScore, remoteScore)
	}
}
