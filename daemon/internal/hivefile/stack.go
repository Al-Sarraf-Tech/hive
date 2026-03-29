package hivefile

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// StackFile defines a multi-Hivefile deployment unit.
// Stack files reference multiple Hivefiles that deploy together
// with shared networking and dependency ordering.
type StackFile struct {
	Name  string       `toml:"name"`
	Files []StackEntry `toml:"stack"`
}

// StackEntry references a single Hivefile within a stack.
type StackEntry struct {
	File string `toml:"file"`
}

// ParseStackFile reads a stack definition from a TOML file.
func ParseStackFile(path string) (*StackFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read stack file %q: %w", path, err)
	}
	return ParseStack(data)
}

// ParseStack parses stack definition from TOML content.
func ParseStack(data []byte) (*StackFile, error) {
	var sf StackFile
	if err := toml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parse stack: %w", err)
	}
	if sf.Name == "" {
		sf.Name = "default"
	}
	if len(sf.Files) == 0 {
		return nil, fmt.Errorf("stack contains no files")
	}
	return &sf, nil
}

// MarshalHivefile serializes a Hivefile back to TOML format.
func MarshalHivefile(hf *Hivefile) ([]byte, error) {
	data, err := toml.Marshal(hf)
	if err != nil {
		return nil, fmt.Errorf("marshal hivefile: %w", err)
	}
	return data, nil
}

// MergeHivefiles merges multiple Hivefiles into a single Hivefile.
// Returns an error on service name collisions.
func MergeHivefiles(hivefiles []*Hivefile) (*Hivefile, error) {
	merged := &Hivefile{
		Service: make(map[string]ServiceDef),
	}
	for _, hf := range hivefiles {
		for name, svc := range hf.Service {
			if _, exists := merged.Service[name]; exists {
				return nil, fmt.Errorf("service name collision: %q defined in multiple Hivefiles", name)
			}
			merged.Service[name] = svc
		}
	}
	if len(merged.Service) == 0 {
		return nil, fmt.Errorf("merged stack contains no services")
	}
	return merged, nil
}
