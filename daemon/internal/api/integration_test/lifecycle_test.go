// Package integration_test provides end-to-end tests for the Hive API server
// using a mock container provider (no real Docker required).
package integration_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/jalsarraf0/hive/daemon/internal/api"
	"github.com/jalsarraf0/hive/daemon/internal/container"
	"github.com/jalsarraf0/hive/daemon/internal/health"
	"github.com/jalsarraf0/hive/daemon/internal/secrets"
	"github.com/jalsarraf0/hive/daemon/internal/store"

	hivev1 "github.com/jalsarraf0/hive/daemon/internal/api/gen/hive/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

// mockProvider implements container.Provider for testing without Docker.
type mockProvider struct {
	mu         sync.Mutex
	containers map[string]*mockContainer
	nextID     int
}

type mockContainer struct {
	info    container.ContainerInfo
	running bool
}

func newMockProvider() *mockProvider {
	return &mockProvider{containers: make(map[string]*mockContainer)}
}

func (m *mockProvider) RuntimeName() string             { return "mock" }
func (m *mockProvider) Ping(_ context.Context) error    { return nil }
func (m *mockProvider) DetectCapabilities() []string     { return []string{"linux/amd64"} }
func (m *mockProvider) Close() error                     { return nil }
func (m *mockProvider) PullImage(_ context.Context, _ string) error { return nil }
func (m *mockProvider) CreateNetwork(_ context.Context, _ string) (string, error) { return "net-1", nil }
func (m *mockProvider) RemoveNetwork(_ context.Context, _ string) error { return nil }

func (m *mockProvider) CreateAndStart(_ context.Context, spec container.ContainerSpec) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	id := fmt.Sprintf("mock-%d", m.nextID)
	m.containers[id] = &mockContainer{
		info: container.ContainerInfo{
			ID:     id,
			Name:   spec.Name,
			Image:  spec.Image,
			Status: "running",
			Labels: spec.Labels,
		},
		running: true,
	}
	return id, nil
}

func (m *mockProvider) Stop(_ context.Context, id string, _ int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.containers[id]; ok {
		c.running = false
		c.info.Status = "exited"
	}
	return nil
}

func (m *mockProvider) Remove(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.containers, id)
	return nil
}

func (m *mockProvider) ListContainers(_ context.Context, filters map[string]string) ([]container.ContainerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []container.ContainerInfo
	for _, c := range m.containers {
		match := true
		for k, v := range filters {
			if c.info.Labels[k] != v {
				match = false
				break
			}
		}
		if match {
			out = append(out, c.info)
		}
	}
	return out, nil
}

func (m *mockProvider) Logs(_ context.Context, _ string, _ container.LogOpts) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *mockProvider) Exec(_ context.Context, _ string, _ []string) (container.ExecResult, error) {
	return container.ExecResult{ExitCode: 0, Stdout: "ok\n"}, nil
}

func (m *mockProvider) Stats(_ context.Context, _ string) (*container.ContainerStats, error) {
	return &container.ContainerStats{CPUPercent: 1.5, MemoryBytes: 1024 * 1024}, nil
}

func (m *mockProvider) Inspect(_ context.Context, id string) (*container.ContainerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.containers[id]; ok {
		return &c.info, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockProvider) runningCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, c := range m.containers {
		if c.running {
			n++
		}
	}
	return n
}

// setupServer creates a test API server with a mock provider and temp store.
func setupServer(t *testing.T) (*api.Server, *mockProvider) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	mock := newMockProvider()
	hc := health.NewChecker(mock)
	vault, err := secrets.NewVault(dir)
	if err != nil {
		t.Fatalf("new vault: %v", err)
	}

	srv := api.NewServer(s, mock, hc, "test-node", nil, nil, vault, dir)
	return srv, mock
}

const testHivefile = `
[service.web]
image = "nginx:latest"
replicas = 3
restart_policy = "on-failure"

[service.web.ports]
"8080" = "80"
`

func TestDeployLifecycle(t *testing.T) {
	srv, mock := setupServer(t)
	ctx := context.Background()

	// Deploy
	resp, err := srv.DeployService(ctx, &hivev1.DeployServiceRequest{
		HivefileToml: testHivefile,
	})
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if len(resp.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(resp.Services))
	}
	svc := resp.Services[0]
	if svc.Name != "web" {
		t.Errorf("service name: %s, want web", svc.Name)
	}
	if svc.ReplicasDesired != 3 {
		t.Errorf("replicas desired: %d, want 3", svc.ReplicasDesired)
	}
	if svc.ReplicasRunning != 3 {
		t.Errorf("replicas running: %d, want 3", svc.ReplicasRunning)
	}
	if mock.runningCount() != 3 {
		t.Errorf("mock running containers: %d, want 3", mock.runningCount())
	}

	// List services
	listResp, err := srv.ListServices(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(listResp.Services) != 1 {
		t.Errorf("listed services: %d, want 1", len(listResp.Services))
	}

	// Scale up to 5
	_, err = srv.ScaleService(ctx, &hivev1.ScaleServiceRequest{Name: "web", Replicas: 5})
	if err != nil {
		t.Fatalf("scale: %v", err)
	}
	if mock.runningCount() != 5 {
		t.Errorf("after scale up: %d running, want 5", mock.runningCount())
	}

	// Scale down to 2
	_, err = srv.ScaleService(ctx, &hivev1.ScaleServiceRequest{Name: "web", Replicas: 2})
	if err != nil {
		t.Fatalf("scale down: %v", err)
	}
	if mock.runningCount() != 2 {
		t.Errorf("after scale down: %d running, want 2", mock.runningCount())
	}

	// Exec
	execResp, err := srv.ExecContainer(ctx, &hivev1.ExecContainerRequest{
		ServiceName: "web",
		Command:     []string{"echo", "hello"},
	})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if execResp.ExitCode != 0 {
		t.Errorf("exec exit code: %d, want 0", execResp.ExitCode)
	}

	// Stop
	_, err = srv.StopService(ctx, &hivev1.StopServiceRequest{Name: "web"})
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if mock.runningCount() != 0 {
		t.Errorf("after stop: %d running, want 0", mock.runningCount())
	}
}

func TestDeployWithDependencies(t *testing.T) {
	srv, mock := setupServer(t)
	ctx := context.Background()

	hivefile := `
[service.db]
image = "postgres:16"

[service.api]
image = "api:latest"

[service.api.depends_on]
services = ["db"]

[service.web]
image = "nginx:latest"

[service.web.depends_on]
services = ["api"]
`
	resp, err := srv.DeployService(ctx, &hivev1.DeployServiceRequest{
		HivefileToml: hivefile,
	})
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if len(resp.Services) != 3 {
		t.Fatalf("expected 3 services, got %d", len(resp.Services))
	}
	if mock.runningCount() != 3 {
		t.Errorf("expected 3 running containers, got %d", mock.runningCount())
	}

	// Verify deploy order: db first, then api, then web
	if resp.Services[0].Name != "db" {
		t.Errorf("first service should be db, got %s", resp.Services[0].Name)
	}
	if resp.Services[1].Name != "api" {
		t.Errorf("second service should be api, got %s", resp.Services[1].Name)
	}
	if resp.Services[2].Name != "web" {
		t.Errorf("third service should be web, got %s", resp.Services[2].Name)
	}
}

func TestDeployCycleDetection(t *testing.T) {
	srv, _ := setupServer(t)
	ctx := context.Background()

	hivefile := `
[service.a]
image = "a:latest"

[service.a.depends_on]
services = ["b"]

[service.b]
image = "b:latest"

[service.b.depends_on]
services = ["a"]
`
	_, err := srv.DeployService(ctx, &hivev1.DeployServiceRequest{
		HivefileToml: hivefile,
	})
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected cycle error, got: %v", err)
	}
}

func TestRollback(t *testing.T) {
	srv, mock := setupServer(t)
	ctx := context.Background()

	// Deploy v1
	_, err := srv.DeployService(ctx, &hivev1.DeployServiceRequest{
		HivefileToml: `
[service.app]
image = "app:v1"
`,
	})
	if err != nil {
		t.Fatalf("deploy v1: %v", err)
	}

	// Deploy v2 (overwrites v1)
	_, err = srv.DeployService(ctx, &hivev1.DeployServiceRequest{
		HivefileToml: `
[service.app]
image = "app:v2"
`,
	})
	if err != nil {
		t.Fatalf("deploy v2: %v", err)
	}

	// Rollback to v1
	_, err = srv.RollbackService(ctx, &hivev1.RollbackServiceRequest{Name: "app"})
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}

	// Check: should have 1 running container with v1 image
	if mock.runningCount() != 1 {
		t.Errorf("after rollback: %d running, want 1", mock.runningCount())
	}
}

func TestScaleValidation(t *testing.T) {
	srv, _ := setupServer(t)
	ctx := context.Background()

	// Scale non-existent service
	_, err := srv.ScaleService(ctx, &hivev1.ScaleServiceRequest{Name: "nope", Replicas: 3})
	if err == nil {
		t.Error("expected error scaling non-existent service")
	}

	// Scale with 0 replicas
	_, err = srv.ScaleService(ctx, &hivev1.ScaleServiceRequest{Name: "web", Replicas: 0})
	if err == nil {
		t.Error("expected error scaling to 0 replicas")
	}
}

func TestSecretLifecycle(t *testing.T) {
	srv, _ := setupServer(t)
	ctx := context.Background()

	// Set a secret
	_, err := srv.SetSecret(ctx, &hivev1.SetSecretRequest{Key: "DB_PASS", Value: []byte("s3cret")})
	if err != nil {
		t.Fatalf("set secret: %v", err)
	}

	// List secrets
	listResp, err := srv.ListSecrets(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("list secrets: %v", err)
	}
	if len(listResp.Secrets) != 1 || listResp.Secrets[0].Key != "DB_PASS" {
		t.Errorf("expected 1 secret named DB_PASS, got %v", listResp.Secrets)
	}

	// Delete secret
	_, err = srv.DeleteSecret(ctx, &hivev1.DeleteSecretRequest{Key: "DB_PASS"})
	if err != nil {
		t.Fatalf("delete secret: %v", err)
	}

	// Verify deleted
	listResp, err = srv.ListSecrets(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(listResp.Secrets) != 0 {
		t.Errorf("expected 0 secrets after delete, got %d", len(listResp.Secrets))
	}
}
