// Package backup implements cluster state export and import for hived.
// Backup data is a JSON-encoded ClusterBackup struct containing all
// store buckets. Secrets are base64-encoded (they are already encrypted
// at rest by the secrets.Vault layer).
package backup

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/jalsarraf0/hive/daemon/internal/store"
)

// ClusterBackup is the serializable representation of all cluster state.
type ClusterBackup struct {
	Version    string            `json:"version"`
	ExportedAt time.Time        `json:"exported_at"`
	Services   map[string]string `json:"services"`   // name -> JSON service definition
	Secrets    map[string]string `json:"secrets"`    // name -> base64 encoded (encrypted) bytes
	Placements map[string]string `json:"placements"` // service name -> node name
	History    map[string]string `json:"history"`    // service name -> JSON previous definition
	Meta       map[string]string `json:"meta"`       // key -> value
}

// Export reads all persistent state from the store and returns a ClusterBackup.
// Exported buckets: services, secrets, service_placements, service_history, meta.
// Runtime-only buckets (containers, health_state) are excluded.
func Export(s *store.Store) (*ClusterBackup, error) {
	b := &ClusterBackup{
		Version:    "1",
		ExportedAt: time.Now().UTC(),
		Services:   make(map[string]string),
		Secrets:    make(map[string]string),
		Placements: make(map[string]string),
		History:    make(map[string]string),
		Meta:       make(map[string]string),
	}

	var errs []error

	// Export services
	if keys, err := s.List("services"); err != nil {
		errs = append(errs, fmt.Errorf("list services: %w", err))
	} else {
		for _, k := range keys {
			if v, err := s.Get("services", k); err != nil {
				errs = append(errs, fmt.Errorf("get service %q: %w", k, err))
			} else if v != nil {
				b.Services[k] = string(v)
			}
		}
	}

	// Export secrets (base64-encode because values are encrypted binary)
	if keys, err := s.List("secrets"); err != nil {
		errs = append(errs, fmt.Errorf("list secrets: %w", err))
	} else {
		for _, k := range keys {
			if v, err := s.Get("secrets", k); err != nil {
				errs = append(errs, fmt.Errorf("get secret %q: %w", k, err))
			} else if v != nil {
				b.Secrets[k] = base64.StdEncoding.EncodeToString(v)
			}
		}
	}

	// Export placements
	if keys, err := s.List("service_placements"); err != nil {
		errs = append(errs, fmt.Errorf("list placements: %w", err))
	} else {
		for _, k := range keys {
			if v, err := s.Get("service_placements", k); err != nil {
				errs = append(errs, fmt.Errorf("get placement %q: %w", k, err))
			} else if v != nil {
				b.Placements[k] = string(v)
			}
		}
	}

	// Export service history
	if keys, err := s.List("service_history"); err != nil {
		errs = append(errs, fmt.Errorf("list history: %w", err))
	} else {
		for _, k := range keys {
			if v, err := s.Get("service_history", k); err != nil {
				errs = append(errs, fmt.Errorf("get history %q: %w", k, err))
			} else if v != nil {
				b.History[k] = string(v)
			}
		}
	}

	// Export meta
	if keys, err := s.List("meta"); err != nil {
		errs = append(errs, fmt.Errorf("list meta: %w", err))
	} else {
		for _, k := range keys {
			if v, err := s.Get("meta", k); err != nil {
				errs = append(errs, fmt.Errorf("get meta %q: %w", k, err))
			} else if v != nil {
				b.Meta[k] = string(v)
			}
		}
	}

	if len(errs) > 0 {
		return b, fmt.Errorf("export had %d errors (first: %w)", len(errs), errs[0])
	}
	return b, nil
}

// Marshal serializes a ClusterBackup to JSON bytes.
func Marshal(b *ClusterBackup) ([]byte, error) {
	data, err := json.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("marshal backup: %w", err)
	}
	return data, nil
}

// Unmarshal deserializes JSON bytes into a ClusterBackup.
func Unmarshal(data []byte) (*ClusterBackup, error) {
	var b ClusterBackup
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("unmarshal backup: %w", err)
	}
	if b.Version == "" {
		return nil, fmt.Errorf("invalid backup: missing version field")
	}
	return &b, nil
}

// Import restores cluster state from a ClusterBackup into the store.
// If overwrite is false, existing keys are skipped.
// Returns (services imported, secrets imported, error).
func Import(s *store.Store, b *ClusterBackup, overwrite bool) (int, int, error) {
	svcCount := 0
	secCount := 0

	// Import services
	for k, v := range b.Services {
		if !overwrite {
			if existing, _ := s.Get("services", k); existing != nil {
				continue
			}
		}
		if err := s.Put("services", k, []byte(v)); err != nil {
			return svcCount, secCount, fmt.Errorf("import service %q: %w", k, err)
		}
		svcCount++
	}

	// Import secrets (base64-decode back to encrypted binary)
	for k, v := range b.Secrets {
		if !overwrite {
			if existing, _ := s.Get("secrets", k); existing != nil {
				continue
			}
		}
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return svcCount, secCount, fmt.Errorf("decode secret %q: %w", k, err)
		}
		if err := s.Put("secrets", k, decoded); err != nil {
			return svcCount, secCount, fmt.Errorf("import secret %q: %w", k, err)
		}
		secCount++
	}

	// Import placements
	for k, v := range b.Placements {
		if !overwrite {
			if existing, _ := s.Get("service_placements", k); existing != nil {
				continue
			}
		}
		if err := s.Put("service_placements", k, []byte(v)); err != nil {
			return svcCount, secCount, fmt.Errorf("import placement %q: %w", k, err)
		}
	}

	// Import service history
	for k, v := range b.History {
		if !overwrite {
			if existing, _ := s.Get("service_history", k); existing != nil {
				continue
			}
		}
		if err := s.Put("service_history", k, []byte(v)); err != nil {
			return svcCount, secCount, fmt.Errorf("import history %q: %w", k, err)
		}
	}

	// Import meta
	for k, v := range b.Meta {
		if !overwrite {
			if existing, _ := s.Get("meta", k); existing != nil {
				continue
			}
		}
		if err := s.Put("meta", k, []byte(v)); err != nil {
			return svcCount, secCount, fmt.Errorf("import meta %q: %w", k, err)
		}
	}

	return svcCount, secCount, nil
}

// RunScheduledBackup exports cluster state to a timestamped file in the given directory.
func RunScheduledBackup(s *store.Store, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}

	b, err := Export(s)
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}

	data, err := Marshal(b)
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("hive-backup-%s.json", time.Now().UTC().Format("20060102-150405"))
	path := filepath.Join(outputDir, filename)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write backup: %w", err)
	}

	slog.Info("scheduled backup completed", "path", path, "size", len(data))
	return nil
}
