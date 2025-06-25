package loader

import (
	"fmt"
	"io"
	"os"

	"github.com/kong/kongctl/internal/declarative/resources"
	"sigs.k8s.io/yaml"
)

// Loader handles loading declarative configuration from files
type Loader struct {
	rootPath string
}

// New creates a new configuration loader
func New(rootPath string) *Loader {
	return &Loader{
		rootPath: rootPath,
	}
}

// Load loads all YAML files from the root path
func (l *Loader) Load() (*resources.ResourceSet, error) {
	info, err := os.Stat(l.rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path %s: %w", l.rootPath, err)
	}

	if info.IsDir() {
		return l.loadDirectory()
	}

	return l.LoadFile(l.rootPath)
}

// LoadFile loads configuration from a single YAML file
func (l *Loader) LoadFile(path string) (*resources.ResourceSet, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer file.Close()

	return l.parseYAML(file, path)
}

// parseYAML parses YAML content into ResourceSet
func (l *Loader) parseYAML(r io.Reader, sourcePath string) (*resources.ResourceSet, error) {
	var rs resources.ResourceSet

	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read content from %s: %w", sourcePath, err)
	}

	if err := yaml.Unmarshal(content, &rs); err != nil {
		return nil, fmt.Errorf("failed to parse YAML in %s: %w", sourcePath, err)
	}

	// Apply defaults to all resources
	l.applyDefaults(&rs)

	// Validate all resources
	if err := l.validateResourceSet(&rs); err != nil {
		return nil, fmt.Errorf("validation failed for %s: %w", sourcePath, err)
	}

	return &rs, nil
}

// loadDirectory loads all YAML files from a directory (preparation for Step 6)
func (l *Loader) loadDirectory() (*resources.ResourceSet, error) {
	// For now, just return an error - will be implemented in Step 6
	return nil, fmt.Errorf("directory loading not yet implemented (coming in Step 6)")
}

// applyDefaults applies default values to all resources in the set
func (l *Loader) applyDefaults(rs *resources.ResourceSet) {
	// Apply defaults to portals
	for i := range rs.Portals {
		rs.Portals[i].SetDefaults()
	}

	// Apply defaults to auth strategies
	for i := range rs.ApplicationAuthStrategies {
		rs.ApplicationAuthStrategies[i].SetDefaults()
	}

	// Apply defaults to control planes
	for i := range rs.ControlPlanes {
		rs.ControlPlanes[i].SetDefaults()
	}

	// Apply defaults to APIs and their children
	for i := range rs.APIs {
		api := &rs.APIs[i]
		api.SetDefaults()

		// Apply defaults to nested resources
		for j := range api.Versions {
			api.Versions[j].SetDefaults()
		}
		for j := range api.Publications {
			api.Publications[j].SetDefaults()
		}
		for j := range api.Implementations {
			api.Implementations[j].SetDefaults()
		}
	}
}