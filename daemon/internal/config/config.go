package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/jalsarraf0/hive/daemon/internal/platform"
)

// Config holds all daemon configuration. Every field maps to a CLI flag.
type Config struct {
	Node     NodeConfig     `toml:"node"`
	Ports    PortsConfig    `toml:"ports"`
	Security SecurityConfig `toml:"security"`
	Logging  LoggingConfig  `toml:"logging"`
	HTTP     HTTPConfig     `toml:"http"`
}

// NodeConfig holds node identity and cluster membership settings.
type NodeConfig struct {
	Name          string `toml:"name"`
	AdvertiseAddr string `toml:"advertise_addr"`
	DataDir       string `toml:"data_dir"`
	Join          string `toml:"join"`
}

// PortsConfig holds all listening port settings.
type PortsConfig struct {
	GRPC   int `toml:"grpc"`
	Gossip int `toml:"gossip"`
	Mesh   int `toml:"mesh"`
}

// SecurityConfig holds TLS and encryption settings.
type SecurityConfig struct {
	TLS       bool   `toml:"tls"`
	GossipKey string `toml:"gossip_key"`
}

// LoggingConfig holds log settings.
type LoggingConfig struct {
	Level string `toml:"level"`
}

// HTTPConfig holds web console HTTP server settings.
type HTTPConfig struct {
	Port  int    `toml:"port"`
	Token string `toml:"token"`
}

// Default returns a Config with the same defaults as the CLI flags.
func Default() Config {
	return Config{
		Ports: PortsConfig{
			GRPC:   7947,
			Gossip: 7946,
			Mesh:   7948,
		},
		Logging: LoggingConfig{
			Level: "info",
		},
		HTTP: HTTPConfig{
			Port: 7949,
		},
	}
}

// DefaultPath returns the platform-specific default config file path.
func DefaultPath() string {
	return filepath.Join(platform.DefaultConfigDir(), "hived.toml")
}

// Load reads and parses a TOML config file. If the file does not exist,
// it returns Default() with no error. Other I/O or parse errors are returned.
func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}

	return cfg, nil
}

// FlagOverrides holds pointers to flag values. A nil pointer means the flag
// was not set on the command line and should not override the config value.
type FlagOverrides struct {
	Name          *string
	AdvertiseAddr *string
	DataDir       *string
	Join          *string
	GRPCPort      *int
	GossipPort    *int
	MeshPort      *int
	TLS           *bool
	GossipKey     *string
	LogLevel      *string
	HTTPPort      *int
}

// Merge applies explicit flag overrides on top of the config. Only non-nil
// flag pointers overwrite config values.
func (c Config) Merge(o FlagOverrides) Config {
	if o.Name != nil {
		c.Node.Name = *o.Name
	}
	if o.AdvertiseAddr != nil {
		c.Node.AdvertiseAddr = *o.AdvertiseAddr
	}
	if o.DataDir != nil {
		c.Node.DataDir = *o.DataDir
	}
	if o.Join != nil {
		c.Node.Join = *o.Join
	}
	if o.GRPCPort != nil {
		c.Ports.GRPC = *o.GRPCPort
	}
	if o.GossipPort != nil {
		c.Ports.Gossip = *o.GossipPort
	}
	if o.MeshPort != nil {
		c.Ports.Mesh = *o.MeshPort
	}
	if o.TLS != nil {
		c.Security.TLS = *o.TLS
	}
	if o.GossipKey != nil {
		c.Security.GossipKey = *o.GossipKey
	}
	if o.LogLevel != nil {
		c.Logging.Level = *o.LogLevel
	}
	if o.HTTPPort != nil {
		c.HTTP.Port = *o.HTTPPort
	}
	return c
}
