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

func TestMatchesConstraintsMemory(t *testing.T) {
	s := &Scheduler{localName: "test"}
	node := mesh.NodeInfo{
		Name:      "test",
		Platforms: []string{"linux/amd64"},
		MemAvail:  256 * 1024 * 1024, // 256MB available
	}

	// Service requires 128MB — node has 256MB available, should match
	svc := hivefile.ServiceDef{
		Resources: hivefile.ResourceDef{Memory: "128M"},
	}
	if !s.matchesConstraints(node, svc) {
		t.Error("node with 256MB avail should satisfy 128MB requirement")
	}

	// Service requires 512MB — node has 256MB available, should not match
	svc.Resources.Memory = "512M"
	if s.matchesConstraints(node, svc) {
		t.Error("node with 256MB avail should not satisfy 512MB requirement")
	}

	// Node with no memory info (MemAvail=0) should not be rejected
	nodeNoInfo := mesh.NodeInfo{Name: "test", Platforms: []string{"linux/amd64"}}
	svc.Resources.Memory = "512M"
	if !s.matchesConstraints(nodeNoInfo, svc) {
		t.Error("node with no memory info should not be rejected")
	}
}

func TestScoreMemoryHeadroom(t *testing.T) {
	s := &Scheduler{localName: "test"}

	highMem := mesh.NodeInfo{
		Containers: 2,
		MemTotal:   8 * 1024 * 1024 * 1024,
		MemAvail:   6 * 1024 * 1024 * 1024, // 75% free
	}
	lowMem := mesh.NodeInfo{
		Containers: 2,
		MemTotal:   8 * 1024 * 1024 * 1024,
		MemAvail:   1 * 1024 * 1024 * 1024, // 12.5% free
	}

	highScore := s.score(highMem, false)
	lowScore := s.score(lowMem, false)

	if highScore <= lowScore {
		t.Errorf("node with more free memory (%f) should score higher than low memory (%f)", highScore, lowScore)
	}
}

func TestScoreCPUCores(t *testing.T) {
	s := &Scheduler{localName: "test"}

	manyCores := mesh.NodeInfo{Containers: 2, CPUCores: 16}
	fewCores := mesh.NodeInfo{Containers: 2, CPUCores: 2}

	manyScore := s.score(manyCores, false)
	fewScore := s.score(fewCores, false)

	if manyScore <= fewScore {
		t.Errorf("node with more cores (%f) should score higher than fewer cores (%f)", manyScore, fewScore)
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
