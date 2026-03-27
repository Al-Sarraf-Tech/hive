package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/hived.toml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	d := Default()
	if cfg.Ports.GRPC != d.Ports.GRPC {
		t.Errorf("expected default GRPC port %d, got %d", d.Ports.GRPC, cfg.Ports.GRPC)
	}
	if cfg.Logging.Level != d.Logging.Level {
		t.Errorf("expected default log level %q, got %q", d.Logging.Level, cfg.Logging.Level)
	}
}

func TestLoadValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hived.toml")

	content := `
[node]
name = "worker-1"
advertise_addr = "10.0.0.5"
data_dir = "/data/hive"
join = "10.0.0.1:7946"

[ports]
grpc = 9000
gossip = 9001
mesh = 9002

[security]
tls = true
gossip_key = "deadbeef"

[logging]
level = "debug"

[http]
port = 8080
token = "s3cret"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Node.Name != "worker-1" {
		t.Errorf("Node.Name = %q, want %q", cfg.Node.Name, "worker-1")
	}
	if cfg.Node.AdvertiseAddr != "10.0.0.5" {
		t.Errorf("Node.AdvertiseAddr = %q, want %q", cfg.Node.AdvertiseAddr, "10.0.0.5")
	}
	if cfg.Node.DataDir != "/data/hive" {
		t.Errorf("Node.DataDir = %q, want %q", cfg.Node.DataDir, "/data/hive")
	}
	if cfg.Node.Join != "10.0.0.1:7946" {
		t.Errorf("Node.Join = %q, want %q", cfg.Node.Join, "10.0.0.1:7946")
	}
	if cfg.Ports.GRPC != 9000 {
		t.Errorf("Ports.GRPC = %d, want %d", cfg.Ports.GRPC, 9000)
	}
	if cfg.Ports.Gossip != 9001 {
		t.Errorf("Ports.Gossip = %d, want %d", cfg.Ports.Gossip, 9001)
	}
	if cfg.Ports.Mesh != 9002 {
		t.Errorf("Ports.Mesh = %d, want %d", cfg.Ports.Mesh, 9002)
	}
	if !cfg.Security.TLS {
		t.Error("Security.TLS = false, want true")
	}
	if cfg.Security.GossipKey != "deadbeef" {
		t.Errorf("Security.GossipKey = %q, want %q", cfg.Security.GossipKey, "deadbeef")
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "debug")
	}
	if cfg.HTTP.Port != 8080 {
		t.Errorf("HTTP.Port = %d, want %d", cfg.HTTP.Port, 8080)
	}
	if cfg.HTTP.Token != "s3cret" {
		t.Errorf("HTTP.Token = %q, want %q", cfg.HTTP.Token, "s3cret")
	}
}

func TestLoadPartialFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hived.toml")

	content := `
[ports]
grpc = 5000
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Ports.GRPC != 5000 {
		t.Errorf("Ports.GRPC = %d, want %d", cfg.Ports.GRPC, 5000)
	}
	// Unset sections should retain defaults
	d := Default()
	if cfg.Ports.Gossip != d.Ports.Gossip {
		t.Errorf("Ports.Gossip = %d, want default %d", cfg.Ports.Gossip, d.Ports.Gossip)
	}
	if cfg.Logging.Level != d.Logging.Level {
		t.Errorf("Logging.Level = %q, want default %q", cfg.Logging.Level, d.Logging.Level)
	}
}

func TestMergeFlagsOverrideConfig(t *testing.T) {
	cfg := Config{
		Node:  NodeConfig{Name: "from-config"},
		Ports: PortsConfig{GRPC: 9000, Gossip: 9001, Mesh: 9002},
	}

	name := "from-flag"
	port := 7777
	overrides := FlagOverrides{
		Name:     &name,
		GRPCPort: &port,
	}

	merged := cfg.Merge(overrides)
	if merged.Node.Name != "from-flag" {
		t.Errorf("Name = %q, want %q", merged.Node.Name, "from-flag")
	}
	if merged.Ports.GRPC != 7777 {
		t.Errorf("GRPC = %d, want %d", merged.Ports.GRPC, 7777)
	}
	// Non-overridden values preserved
	if merged.Ports.Gossip != 9001 {
		t.Errorf("Gossip = %d, want %d", merged.Ports.Gossip, 9001)
	}
}

func TestMergeFlagsNilPreserveConfig(t *testing.T) {
	cfg := Config{
		Node:    NodeConfig{Name: "keep-me", DataDir: "/data"},
		Ports:   PortsConfig{GRPC: 9000},
		Logging: LoggingConfig{Level: "warn"},
	}

	// All nil overrides — config should be unchanged
	merged := cfg.Merge(FlagOverrides{})

	if merged.Node.Name != "keep-me" {
		t.Errorf("Name = %q, want %q", merged.Node.Name, "keep-me")
	}
	if merged.Node.DataDir != "/data" {
		t.Errorf("DataDir = %q, want %q", merged.Node.DataDir, "/data")
	}
	if merged.Ports.GRPC != 9000 {
		t.Errorf("GRPC = %d, want %d", merged.Ports.GRPC, 9000)
	}
	if merged.Logging.Level != "warn" {
		t.Errorf("Level = %q, want %q", merged.Logging.Level, "warn")
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hived.toml")

	if err := os.WriteFile(path, []byte("not valid [[ toml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML, got nil")
	}
}
