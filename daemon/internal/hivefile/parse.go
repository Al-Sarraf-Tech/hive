// Package hivefile parses Hive service definition files (TOML format).
package hivefile

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// secretRefPattern matches {{ secret:key-name }} placeholders.
var secretRefPattern = regexp.MustCompile(`\{\{\s*secret:([a-zA-Z0-9_-]+)\s*\}\}`)

// Hivefile is the top-level structure of a hive.toml file.
type Hivefile struct {
	Service map[string]ServiceDef `toml:"service"`
}

// ServiceDef defines a single service in the Hivefile.
type ServiceDef struct {
	Image         string            `toml:"image"`
	Replicas      int               `toml:"replicas"`
	Platform      string            `toml:"platform"`
	Node          string            `toml:"node"`
	Isolation     string            `toml:"isolation"`
	Env           map[string]string `toml:"env"`
	Ports         map[string]string `toml:"ports"`
	Volumes       []VolumeDef       `toml:"volumes"`
	Labels        map[string]string `toml:"labels"`
	Health        HealthDef         `toml:"health"`
	Resources     ResourceDef       `toml:"resources"`
	Deploy        DeployDef         `toml:"deploy"`
	DependsOn     DependsDef        `toml:"depends_on"`
	RestartPolicy string            `toml:"restart_policy"`
}

// VolumeDef supports both simple and cross-platform volume syntax.
type VolumeDef struct {
	Name     string `toml:"name"`
	Target   string `toml:"target"`
	Linux    string `toml:"linux"`
	Windows  string `toml:"windows"`
	ReadOnly bool   `toml:"read_only"`
}

// HealthDef configures health checking.
type HealthDef struct {
	Type     string `toml:"type"`
	Path     string `toml:"path"`
	Port     int    `toml:"port"`
	Interval string `toml:"interval"`
	Timeout  string `toml:"timeout"`
	Retries  int    `toml:"retries"`
}

// ResourceDef sets resource limits.
type ResourceDef struct {
	Memory string  `toml:"memory"`
	CPUs   float64 `toml:"cpus"`
}

// DeployDef configures deployment strategy.
type DeployDef struct {
	Strategy string `toml:"strategy"`
	MaxSurge int    `toml:"max_surge"`
}

// DependsDef defines service dependencies.
type DependsDef struct {
	Services []string `toml:"services"`
}

// Parse reads a Hivefile from TOML content.
func Parse(data []byte) (*Hivefile, error) {
	var hf Hivefile
	if err := toml.Unmarshal(data, &hf); err != nil {
		return nil, fmt.Errorf("parse hivefile: %w", err)
	}

	if len(hf.Service) == 0 {
		return nil, fmt.Errorf("hivefile contains no services")
	}

	// Validate and set defaults
	for name, svc := range hf.Service {
		if svc.Image == "" {
			return nil, fmt.Errorf("service %q: image is required", name)
		}
		if svc.Replicas <= 0 {
			svc.Replicas = 1
		}
		if svc.RestartPolicy == "" {
			svc.RestartPolicy = "on-failure"
		}
		if svc.Health.Retries <= 0 {
			svc.Health.Retries = 3
		}
		if svc.Health.Interval == "" {
			svc.Health.Interval = "30s"
		}
		if svc.Health.Timeout == "" {
			svc.Health.Timeout = "5s"
		}
		if svc.Deploy.Strategy == "" {
			svc.Deploy.Strategy = "rolling"
		}
		hf.Service[name] = svc
	}

	return &hf, nil
}

// ParseString is a convenience wrapper around Parse.
func ParseString(data string) (*Hivefile, error) {
	return Parse([]byte(data))
}

// ExtractSecretRefs returns all secret key references found in environment values.
func ExtractSecretRefs(svc ServiceDef) []string {
	var refs []string
	seen := make(map[string]bool)
	for _, v := range svc.Env {
		matches := secretRefPattern.FindAllStringSubmatch(v, -1)
		for _, m := range matches {
			key := m[1]
			if !seen[key] {
				refs = append(refs, key)
				seen[key] = true
			}
		}
	}
	return refs
}

// ResolveEnv replaces {{ secret:key }} placeholders with actual values from the secrets map.
// Returns an error listing any unresolved secret references.
func ResolveEnv(env map[string]string, secrets map[string]string) (map[string]string, error) {
	if secrets == nil {
		secrets = make(map[string]string)
	}
	resolved := make(map[string]string, len(env))
	var unresolved []string
	for k, v := range env {
		resolved[k] = secretRefPattern.ReplaceAllStringFunc(v, func(match string) string {
			sub := secretRefPattern.FindStringSubmatch(match)
			if len(sub) >= 2 {
				if val, ok := secrets[sub[1]]; ok {
					return val
				}
				unresolved = append(unresolved, sub[1])
				return match
			}
			return match
		})
	}
	if len(unresolved) > 0 {
		return resolved, fmt.Errorf("unresolved secret references: %s", strings.Join(unresolved, ", "))
	}
	return resolved, nil
}

// ParseMemory converts memory strings like "512M", "1G", "256MB", "2GB" to bytes.
func ParseMemory(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	s = strings.TrimSpace(strings.ToUpper(s))
	multiplier := int64(1)
	// Check longer suffixes first to avoid "G" matching "GB"
	switch {
	case strings.HasSuffix(s, "GB"):
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "GB")
	case strings.HasSuffix(s, "G"):
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "G")
	case strings.HasSuffix(s, "MB"):
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "MB")
	case strings.HasSuffix(s, "M"):
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "M")
	case strings.HasSuffix(s, "KB"):
		multiplier = 1024
		s = strings.TrimSuffix(s, "KB")
	case strings.HasSuffix(s, "K"):
		multiplier = 1024
		s = strings.TrimSuffix(s, "K")
	default:
		// assume bytes
	}
	// Validate the remaining string is a pure positive integer
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid memory value: %q (expected positive integer with optional suffix)", s)
		}
	}
	var val int64
	_, err := fmt.Sscanf(s, "%d", &val)
	if err != nil {
		return 0, fmt.Errorf("invalid memory value: %q", s)
	}
	if val < 0 {
		return 0, fmt.Errorf("invalid memory value: negative numbers not allowed")
	}
	if val == 0 && multiplier > 1 {
		return 0, fmt.Errorf("invalid memory value: zero with unit suffix is not allowed")
	}
	// Guard against integer overflow
	if multiplier > 1 && val > (1<<63-1)/multiplier {
		return 0, fmt.Errorf("invalid memory value: %q overflows int64", s)
	}
	return val * multiplier, nil
}
