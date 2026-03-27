package hivefile

import (
	"strings"
	"testing"
)

func TestTopoSort_NoDeps(t *testing.T) {
	services := map[string]ServiceDef{
		"web":   {Image: "nginx"},
		"api":   {Image: "api:latest"},
		"redis": {Image: "redis:7"},
	}
	order, err := TopoSort(services)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 services, got %d", len(order))
	}
	// With no deps, should be alphabetical (deterministic)
	if order[0] != "api" || order[1] != "redis" || order[2] != "web" {
		t.Errorf("expected alphabetical order, got %v", order)
	}
}

func TestTopoSort_LinearChain(t *testing.T) {
	// db -> api -> web
	services := map[string]ServiceDef{
		"web": {Image: "nginx", DependsOn: DependsDef{Services: []string{"api"}}},
		"api": {Image: "api:latest", DependsOn: DependsDef{Services: []string{"db"}}},
		"db":  {Image: "postgres:16"},
	}
	order, err := TopoSort(services)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// db must come before api, api must come before web
	dbIdx := indexOf(order, "db")
	apiIdx := indexOf(order, "api")
	webIdx := indexOf(order, "web")
	if dbIdx >= apiIdx {
		t.Errorf("db (%d) should come before api (%d)", dbIdx, apiIdx)
	}
	if apiIdx >= webIdx {
		t.Errorf("api (%d) should come before web (%d)", apiIdx, webIdx)
	}
}

func TestTopoSort_DiamondDeps(t *testing.T) {
	// db -> api, db -> worker, api -> web, worker -> web
	services := map[string]ServiceDef{
		"web":    {Image: "nginx", DependsOn: DependsDef{Services: []string{"api", "worker"}}},
		"api":    {Image: "api:latest", DependsOn: DependsDef{Services: []string{"db"}}},
		"worker": {Image: "worker:latest", DependsOn: DependsDef{Services: []string{"db"}}},
		"db":     {Image: "postgres:16"},
	}
	order, err := TopoSort(services)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 4 {
		t.Fatalf("expected 4 services, got %d", len(order))
	}
	// db must be first, web must be last
	if order[0] != "db" {
		t.Errorf("db should be first, got %s", order[0])
	}
	if order[3] != "web" {
		t.Errorf("web should be last, got %s", order[3])
	}
}

func TestTopoSort_Cycle(t *testing.T) {
	services := map[string]ServiceDef{
		"a": {Image: "a", DependsOn: DependsDef{Services: []string{"b"}}},
		"b": {Image: "b", DependsOn: DependsDef{Services: []string{"c"}}},
		"c": {Image: "c", DependsOn: DependsDef{Services: []string{"a"}}},
	}
	_, err := TopoSort(services)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected cycle error, got: %v", err)
	}
}

func TestTopoSort_MissingDep(t *testing.T) {
	services := map[string]ServiceDef{
		"web": {Image: "nginx", DependsOn: DependsDef{Services: []string{"redis"}}},
	}
	_, err := TopoSort(services)
	if err == nil {
		t.Fatal("expected missing dep error, got nil")
	}
	if !strings.Contains(err.Error(), "not defined") {
		t.Errorf("expected 'not defined' error, got: %v", err)
	}
}

func TestTopoSort_SingleService(t *testing.T) {
	services := map[string]ServiceDef{
		"solo": {Image: "solo:1"},
	}
	order, err := TopoSort(services)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 1 || order[0] != "solo" {
		t.Errorf("expected [solo], got %v", order)
	}
}

func TestTopoSort_SelfDep(t *testing.T) {
	services := map[string]ServiceDef{
		"loop": {Image: "loop", DependsOn: DependsDef{Services: []string{"loop"}}},
	}
	_, err := TopoSort(services)
	if err == nil {
		t.Fatal("expected error for self-dependency, got nil")
	}
	if !strings.Contains(err.Error(), "depends on itself") {
		t.Errorf("expected 'depends on itself' error, got: %v", err)
	}
}

func TestTopoSort_Empty(t *testing.T) {
	order, err := TopoSort(map[string]ServiceDef{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 0 {
		t.Errorf("expected empty order, got %v", order)
	}
}

func TestTopoSort_DuplicateDeps(t *testing.T) {
	// Duplicate dependencies should be silently deduplicated
	services := map[string]ServiceDef{
		"db":  {Image: "postgres"},
		"api": {Image: "api", DependsOn: DependsDef{Services: []string{"db", "db", "db"}}},
	}
	order, err := TopoSort(services)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 2 {
		t.Fatalf("expected 2 services, got %d", len(order))
	}
	if order[0] != "db" || order[1] != "api" {
		t.Errorf("expected [db, api], got %v", order)
	}
}

func indexOf(s []string, target string) int {
	for i, v := range s {
		if v == target {
			return i
		}
	}
	return -1
}
