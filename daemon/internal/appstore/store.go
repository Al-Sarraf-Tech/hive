// Package appstore provides the built-in app catalog and installation flow for Hive.
// Apps are TOML recipe files embedded in the binary. Users can also add custom apps.
// Installation generates a Hivefile from the template and feeds it through DeployService.
package appstore

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	hivestore "github.com/jalsarraf0/hive/daemon/internal/store"
)

var configPlaceholder = regexp.MustCompile(`\{\{\s*config:([a-zA-Z0-9_-]+)\s*\}\}`)

// InstalledAppRecord tracks a deployed app from the catalog.
type InstalledAppRecord struct {
	AppID       string            `json:"app_id"`
	ServiceName string            `json:"service_name"`
	Config      map[string]string `json:"config"`
	InstalledAt time.Time         `json:"installed_at"`
}

// Store manages the app catalog (built-in + custom) and installation records.
type Store struct {
	mu       sync.RWMutex
	builtin  map[string]*AppDef
	custom   map[string]*AppDef
	dbStore  *hivestore.Store
}

// New creates an app store, loading built-in catalog from embedded FS and custom apps from bbolt.
func New(s *hivestore.Store) (*Store, error) {
	as := &Store{
		builtin: make(map[string]*AppDef),
		custom:  make(map[string]*AppDef),
		dbStore: s,
	}

	// Load built-in catalog from embedded FS
	entries, err := fs.ReadDir(catalogFS, "catalog")
	if err != nil {
		return nil, fmt.Errorf("read embedded catalog: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		data, err := fs.ReadFile(catalogFS, "catalog/"+entry.Name())
		if err != nil {
			slog.Warn("failed to read catalog entry", "file", entry.Name(), "error", err)
			continue
		}
		app, err := parseRecipe(data)
		if err != nil {
			slog.Warn("failed to parse catalog entry", "file", entry.Name(), "error", err)
			continue
		}
		app.Builtin = true
		as.builtin[app.ID] = app
	}
	slog.Info("app catalog loaded", "builtin", len(as.builtin))

	// Load custom apps from store
	keys, _ := s.List("custom_apps")
	for _, key := range keys {
		data, err := s.Get("custom_apps", key)
		if err != nil || data == nil {
			continue
		}
		app, err := parseRecipe(data)
		if err != nil {
			slog.Warn("corrupt custom app in store", "key", key, "error", err)
			continue
		}
		as.custom[app.ID] = app
	}

	return as, nil
}

// List returns all apps, optionally filtered by category.
func (as *Store) List(category string) []*AppDef {
	as.mu.RLock()
	defer as.mu.RUnlock()

	var apps []*AppDef
	for _, a := range as.builtin {
		if category == "" || a.Category == category {
			apps = append(apps, a)
		}
	}
	for _, a := range as.custom {
		if category == "" || a.Category == category {
			apps = append(apps, a)
		}
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].Name < apps[j].Name })
	return apps
}

// Get returns an app by ID.
func (as *Store) Get(id string) (*AppDef, bool) {
	as.mu.RLock()
	defer as.mu.RUnlock()
	if a, ok := as.builtin[id]; ok {
		return a, true
	}
	if a, ok := as.custom[id]; ok {
		return a, true
	}
	return nil, false
}

// Search finds apps matching the query in name, description, or tags.
func (as *Store) Search(query string) []*AppDef {
	q := strings.ToLower(query)
	as.mu.RLock()
	defer as.mu.RUnlock()

	var results []*AppDef
	for _, a := range as.builtin {
		if matchesSearch(a, q) {
			results = append(results, a)
		}
	}
	for _, a := range as.custom {
		if matchesSearch(a, q) {
			results = append(results, a)
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results
}

func matchesSearch(a *AppDef, q string) bool {
	if strings.Contains(strings.ToLower(a.Name), q) {
		return true
	}
	if strings.Contains(strings.ToLower(a.Description), q) {
		return true
	}
	for _, tag := range a.Tags {
		if strings.Contains(strings.ToLower(tag), q) {
			return true
		}
	}
	return false
}

// GenerateHivefile renders a Hivefile TOML from an app template with user config values.
// The service key in the template is renamed to serviceName.
func (as *Store) GenerateHivefile(appID, serviceName string, config map[string]string) (string, error) {
	app, ok := as.Get(appID)
	if !ok {
		return "", fmt.Errorf("app %q not found", appID)
	}

	// Validate required config fields
	for _, field := range app.ConfigFields {
		if field.Required {
			if val, exists := config[field.Key]; !exists || val == "" {
				if field.Default != "" {
					config[field.Key] = field.Default
				} else {
					return "", fmt.Errorf("required config field %q (%s) not provided", field.Key, field.Label)
				}
			}
		} else if _, exists := config[field.Key]; !exists && field.Default != "" {
			config[field.Key] = field.Default
		}
	}

	// Start with the raw TOML and substitute {{ config:key }} placeholders
	result := app.RawTOML

	// Remove the [recipe] section — only keep [service.*] sections
	lines := strings.Split(result, "\n")
	var serviceLines []string
	inService := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[recipe") {
			inService = false
			continue
		}
		if strings.HasPrefix(trimmed, "[service.") || strings.HasPrefix(trimmed, "[[service.") {
			inService = true
		}
		if inService {
			serviceLines = append(serviceLines, line)
		}
	}
	result = strings.Join(serviceLines, "\n")

	// Substitute config placeholders
	result = configPlaceholder.ReplaceAllStringFunc(result, func(match string) string {
		submatch := configPlaceholder.FindStringSubmatch(match)
		if len(submatch) > 1 {
			if val, ok := config[submatch[1]]; ok {
				return val
			}
		}
		return match
	})

	// Rename service key if custom name provided
	if serviceName != "" && serviceName != appID {
		for _, a := range as.builtin {
			if a.ID == appID {
				for svcKey := range extractServiceKeys(result) {
					result = strings.ReplaceAll(result, "[service."+svcKey+"]", "[service."+serviceName+"]")
					result = strings.ReplaceAll(result, "[service."+svcKey+".", "[service."+serviceName+".")
					result = strings.ReplaceAll(result, "[[service."+svcKey+".", "[[service."+serviceName+".")
				}
				break
			}
		}
		// Also try custom apps
		for _, a := range as.custom {
			if a.ID == appID {
				for svcKey := range extractServiceKeys(result) {
					result = strings.ReplaceAll(result, "[service."+svcKey+"]", "[service."+serviceName+"]")
					result = strings.ReplaceAll(result, "[service."+svcKey+".", "[service."+serviceName+".")
					result = strings.ReplaceAll(result, "[[service."+svcKey+".", "[[service."+serviceName+".")
				}
				break
			}
		}
	}

	return result, nil
}

// extractServiceKeys finds all [service.X] keys in a TOML string.
func extractServiceKeys(tomlStr string) map[string]bool {
	keys := make(map[string]bool)
	for _, line := range strings.Split(tomlStr, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[service.") {
			// [service.postgres] → "postgres"
			inner := strings.TrimPrefix(trimmed, "[service.")
			inner = strings.TrimSuffix(inner, "]")
			if idx := strings.IndexByte(inner, '.'); idx >= 0 {
				inner = inner[:idx]
			}
			keys[inner] = true
		}
	}
	return keys
}

// RecordInstall records that an app was installed.
func (as *Store) RecordInstall(appID, serviceName string, config map[string]string) error {
	record := InstalledAppRecord{
		AppID:       appID,
		ServiceName: serviceName,
		Config:      config,
		InstalledAt: time.Now().UTC(),
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return as.dbStore.Put("installed_apps", serviceName, data)
}

// ListInstalled returns all installed app records.
func (as *Store) ListInstalled() []InstalledAppRecord {
	keys, _ := as.dbStore.List("installed_apps")
	var records []InstalledAppRecord
	for _, key := range keys {
		data, err := as.dbStore.Get("installed_apps", key)
		if err != nil || data == nil {
			continue
		}
		var record InstalledAppRecord
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}
		records = append(records, record)
	}
	return records
}

// AddCustom adds a user-defined app to the catalog.
func (as *Store) AddCustom(recipeToml string) (*AppDef, error) {
	app, err := parseRecipe([]byte(recipeToml))
	if err != nil {
		return nil, err
	}
	as.mu.Lock()
	defer as.mu.Unlock()
	if _, exists := as.builtin[app.ID]; exists {
		return nil, fmt.Errorf("cannot override built-in app %q", app.ID)
	}
	if err := as.dbStore.Put("custom_apps", app.ID, []byte(recipeToml)); err != nil {
		return nil, err
	}
	as.custom[app.ID] = app
	return app, nil
}

// RemoveCustom removes a user-defined app from the catalog.
func (as *Store) RemoveCustom(id string) error {
	as.mu.Lock()
	defer as.mu.Unlock()
	if _, exists := as.builtin[id]; exists {
		return fmt.Errorf("cannot remove built-in app %q", id)
	}
	delete(as.custom, id)
	return as.dbStore.Delete("custom_apps", id)
}
