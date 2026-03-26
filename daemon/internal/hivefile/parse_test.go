package hivefile

import (
	"testing"
)

func TestParseBasicHivefile(t *testing.T) {
	input := `
[service.web]
image = "nginx:alpine"
replicas = 2

  [service.web.health]
  type = "http"
  path = "/"
  port = 80

  [service.web.ports]
  "8080" = "80"
`
	hf, err := ParseString(input)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(hf.Service) != 1 {
		t.Fatalf("expected 1 service, got %d", len(hf.Service))
	}

	web, ok := hf.Service["web"]
	if !ok {
		t.Fatal("service 'web' not found")
	}

	if web.Image != "nginx:alpine" {
		t.Errorf("expected image nginx:alpine, got %s", web.Image)
	}
	if web.Replicas != 2 {
		t.Errorf("expected 2 replicas, got %d", web.Replicas)
	}
	if web.Health.Type != "http" {
		t.Errorf("expected health type http, got %s", web.Health.Type)
	}
	if web.Health.Port != 80 {
		t.Errorf("expected health port 80, got %d", web.Health.Port)
	}
	if web.Ports["8080"] != "80" {
		t.Errorf("expected port mapping 8080->80")
	}
}

func TestParseMultipleServices(t *testing.T) {
	input := `
[service.api]
image = "myapp:latest"

[service.db]
image = "postgres:16"
`
	hf, err := ParseString(input)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(hf.Service) != 2 {
		t.Fatalf("expected 2 services, got %d", len(hf.Service))
	}

	if hf.Service["api"].Image != "myapp:latest" {
		t.Error("api image mismatch")
	}
	if hf.Service["db"].Image != "postgres:16" {
		t.Error("db image mismatch")
	}
}

func TestParseDefaults(t *testing.T) {
	input := `
[service.app]
image = "myapp:v1"
`
	hf, err := ParseString(input)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	svc := hf.Service["app"]
	if svc.Replicas != 1 {
		t.Errorf("expected default replicas=1, got %d", svc.Replicas)
	}
	if svc.RestartPolicy != "on-failure" {
		t.Errorf("expected default restart_policy=on-failure, got %s", svc.RestartPolicy)
	}
	if svc.Deploy.Strategy != "rolling" {
		t.Errorf("expected default deploy strategy=rolling, got %s", svc.Deploy.Strategy)
	}
	if svc.Health.Interval != "30s" {
		t.Errorf("expected default health interval=30s, got %s", svc.Health.Interval)
	}
}

func TestParseNoImage(t *testing.T) {
	input := `
[service.broken]
replicas = 1
`
	_, err := ParseString(input)
	if err == nil {
		t.Fatal("expected error for missing image")
	}
}

func TestParseEmpty(t *testing.T) {
	_, err := ParseString("")
	if err == nil {
		t.Fatal("expected error for empty hivefile")
	}
}

func TestExtractSecretRefs(t *testing.T) {
	svc := ServiceDef{
		Env: map[string]string{
			"DB_HOST": "localhost",
			"DB_PASS": "{{ secret:db-password }}",
			"API_KEY": "{{secret:api-key}}",
		},
	}

	refs := ExtractSecretRefs(svc)
	if len(refs) != 2 {
		t.Fatalf("expected 2 secret refs, got %d", len(refs))
	}

	found := make(map[string]bool)
	for _, r := range refs {
		found[r] = true
	}
	if !found["db-password"] {
		t.Error("missing db-password ref")
	}
	if !found["api-key"] {
		t.Error("missing api-key ref")
	}
}

func TestResolveEnv(t *testing.T) {
	env := map[string]string{
		"HOST":    "localhost",
		"DB_PASS": "{{ secret:db-pass }}",
	}
	secrets := map[string]string{
		"db-pass": "s3cret!",
	}

	resolved, err := ResolveEnv(env, secrets)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	if resolved["HOST"] != "localhost" {
		t.Error("plain value should be preserved")
	}
	if resolved["DB_PASS"] != "s3cret!" {
		t.Errorf("expected resolved secret, got %s", resolved["DB_PASS"])
	}
}

func TestResolveEnvUnresolved(t *testing.T) {
	env := map[string]string{
		"DB_PASS": "{{ secret:missing-key }}",
	}
	resolved, err := ResolveEnv(env, map[string]string{})
	if err == nil {
		t.Fatal("expected error for unresolved secret ref")
	}
	// Unresolved refs should stay as placeholders
	if resolved["DB_PASS"] != "{{ secret:missing-key }}" {
		t.Errorf("expected placeholder preserved, got %q", resolved["DB_PASS"])
	}
}

func TestResolveEnvNilEnv(t *testing.T) {
	resolved, err := ResolveEnv(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved) != 0 {
		t.Errorf("expected empty map, got %v", resolved)
	}
}

func TestResolveEnvMultipleSecretsInOneValue(t *testing.T) {
	env := map[string]string{
		"DSN": "postgres://{{ secret:user }}:{{ secret:pass }}@localhost/db",
	}
	secrets := map[string]string{
		"user": "admin",
		"pass": "s3cret",
	}
	resolved, err := ResolveEnv(env, secrets)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	expected := "postgres://admin:s3cret@localhost/db"
	if resolved["DSN"] != expected {
		t.Errorf("got %q, want %q", resolved["DSN"], expected)
	}
}

func TestParseMemory(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"512M", 512 * 1024 * 1024},
		{"1G", 1024 * 1024 * 1024},
		{"256MB", 256 * 1024 * 1024},
		{"2GB", 2 * 1024 * 1024 * 1024},
		{"1024K", 1024 * 1024},
		{"128KB", 128 * 1024},
		{"", 0},
		{"1024", 1024}, // plain bytes
	}

	for _, tc := range tests {
		result, err := ParseMemory(tc.input)
		if err != nil {
			t.Errorf("ParseMemory(%q) failed: %v", tc.input, err)
			continue
		}
		if result != tc.expected {
			t.Errorf("ParseMemory(%q) = %d, want %d", tc.input, result, tc.expected)
		}
	}
}

func TestParseMemoryInvalid(t *testing.T) {
	invalids := []string{"abc", "1.5G", "-512M"}
	for _, s := range invalids {
		_, err := ParseMemory(s)
		if err == nil {
			t.Errorf("ParseMemory(%q) should have failed", s)
		}
	}
}
