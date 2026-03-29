package appstore

import (
	"fmt"

	toml "github.com/pelletier/go-toml/v2"
)

// AppDef is the parsed representation of a catalog recipe.
type AppDef struct {
	ID           string
	Name         string
	Description  string
	Icon         string
	Category     string
	Tags         []string
	Image        string
	Version      string
	MinMemory    string
	Platforms    []string
	ConfigFields []ConfigField
	Builtin      bool
	RawTOML      string // original TOML content for template rendering
}

// ConfigField describes a user-configurable setting for an app.
type ConfigField struct {
	Key         string
	Label       string
	Type        string // "string", "secret", "int", "bool"
	Required    bool
	Default     string
	Description string
}

// recipeFile is the TOML structure of a catalog recipe file.
type recipeFile struct {
	Recipe recipeHeader `toml:"recipe"`
}

type recipeHeader struct {
	ID          string                   `toml:"id"`
	Name        string                   `toml:"name"`
	Description string                   `toml:"description"`
	Icon        string                   `toml:"icon"`
	Category    string                   `toml:"category"`
	Tags        []string                 `toml:"tags"`
	Image       string                   `toml:"image"`
	Version     string                   `toml:"version"`
	MinMemory   string                   `toml:"min_memory"`
	Platforms   []string                 `toml:"platforms"`
	Config      map[string]configFieldDef `toml:"config"`
}

type configFieldDef struct {
	Label       string `toml:"label"`
	Type        string `toml:"type"`
	Required    bool   `toml:"required"`
	Default     string `toml:"default"`
	Description string `toml:"description"`
}

// parseRecipe parses a TOML recipe into an AppDef.
func parseRecipe(data []byte) (*AppDef, error) {
	var rf recipeFile
	if err := toml.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("parse recipe: %w", err)
	}
	if rf.Recipe.ID == "" {
		return nil, fmt.Errorf("recipe missing id")
	}

	app := &AppDef{
		ID:          rf.Recipe.ID,
		Name:        rf.Recipe.Name,
		Description: rf.Recipe.Description,
		Icon:        rf.Recipe.Icon,
		Category:    rf.Recipe.Category,
		Tags:        rf.Recipe.Tags,
		Image:       rf.Recipe.Image,
		Version:     rf.Recipe.Version,
		MinMemory:   rf.Recipe.MinMemory,
		Platforms:   rf.Recipe.Platforms,
		RawTOML:     string(data),
	}

	for key, cfg := range rf.Recipe.Config {
		app.ConfigFields = append(app.ConfigFields, ConfigField{
			Key:         key,
			Label:       cfg.Label,
			Type:        cfg.Type,
			Required:    cfg.Required,
			Default:     cfg.Default,
			Description: cfg.Description,
		})
	}

	return app, nil
}
