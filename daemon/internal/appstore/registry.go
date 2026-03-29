package appstore

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jalsarraf0/hive/daemon/internal/container"
	"github.com/jalsarraf0/hive/daemon/internal/secrets"
	hivestore "github.com/jalsarraf0/hive/daemon/internal/store"
)

// RegistryRecord is stored encrypted in bbolt.
type RegistryRecord struct {
	URL       string    `json:"url"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	CreatedAt time.Time `json:"created_at"`
}

// RegistryManager handles Docker registry credential storage and lookup.
type RegistryManager struct {
	vault *secrets.Vault
	store *hivestore.Store
}

// NewRegistryManager creates a registry manager.
func NewRegistryManager(v *secrets.Vault, s *hivestore.Store) *RegistryManager {
	return &RegistryManager{vault: v, store: s}
}

// Login stores encrypted credentials for a registry.
func (rm *RegistryManager) Login(url, username, password string) error {
	record := RegistryRecord{
		URL:       normalizeRegistryURL(url),
		Username:  username,
		Password:  password,
		CreatedAt: time.Now().UTC(),
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal registry record: %w", err)
	}

	// Encrypt if vault is available
	var stored []byte
	if rm.vault != nil {
		enc, err := rm.vault.Encrypt(data)
		if err != nil {
			return fmt.Errorf("encrypt registry credentials: %w", err)
		}
		stored = enc
	} else {
		stored = data
	}

	if err := rm.store.Put("registries", record.URL, stored); err != nil {
		return fmt.Errorf("store registry credentials: %w", err)
	}

	slog.Info("registry login stored", "url", record.URL, "username", username)
	return nil
}

// List returns all configured registries (without passwords).
func (rm *RegistryManager) List() []RegistryRecord {
	keys, _ := rm.store.List("registries")
	var records []RegistryRecord
	for _, key := range keys {
		record, err := rm.loadRecord(key)
		if err != nil {
			continue
		}
		record.Password = "" // never expose passwords
		records = append(records, *record)
	}
	return records
}

// Remove deletes credentials for a registry.
func (rm *RegistryManager) Remove(url string) error {
	return rm.store.Delete("registries", normalizeRegistryURL(url))
}

// GetAuth returns credentials for an image reference.
// Matches the registry portion of the image (e.g., "ghcr.io/foo/bar" → "ghcr.io").
func (rm *RegistryManager) GetAuth(imageRef string) *container.RegistryAuth {
	registryURL := extractRegistry(imageRef)
	record, err := rm.loadRecord(registryURL)
	if err != nil || record == nil {
		return nil
	}
	return &container.RegistryAuth{
		Username: record.Username,
		Password: record.Password,
	}
}

func (rm *RegistryManager) loadRecord(key string) (*RegistryRecord, error) {
	data, err := rm.store.Get("registries", key)
	if err != nil || data == nil {
		return nil, err
	}

	// Decrypt if vault available
	if rm.vault != nil {
		dec, err := rm.vault.Decrypt(data)
		if err != nil {
			return nil, fmt.Errorf("decrypt registry record: %w", err)
		}
		data = dec
	}

	var record RegistryRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

// normalizeRegistryURL ensures consistent key format.
func normalizeRegistryURL(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimSuffix(url, "/")
	return url
}

// extractRegistry extracts the registry host from an image reference.
// "nginx:alpine" → "docker.io"
// "ghcr.io/foo/bar:latest" → "ghcr.io"
// "registry.example.com/app:v1" → "registry.example.com"
func extractRegistry(imageRef string) string {
	// Remove tag/digest
	if idx := strings.IndexAny(imageRef, ":@"); idx > 0 {
		// Check if the : is part of a port or tag
		before := imageRef[:idx]
		if !strings.Contains(before, "/") || strings.Contains(before, ".") {
			imageRef = before
		}
	}

	parts := strings.SplitN(imageRef, "/", 2)
	if len(parts) == 1 {
		return "docker.io" // official image
	}
	// If first part has a dot or colon, it's a registry
	if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") {
		return parts[0]
	}
	return "docker.io" // Docker Hub user image
}
