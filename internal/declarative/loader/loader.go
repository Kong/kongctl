package loader

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

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

	rs, err := l.parseYAML(file, path)
	if err != nil {
		return nil, err
	}
	
	// Validate when loading a single file
	if err := l.validateResourceSet(rs); err != nil {
		return nil, fmt.Errorf("validation failed for %s: %w", path, err)
	}
	
	return rs, nil
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

	// Note: We don't validate here when called from loadDirectory
	// because cross-references might be in other files.
	// loadDirectory will validate the merged result.

	return &rs, nil
}

// loadDirectory loads all YAML files from a directory
func (l *Loader) loadDirectory() (*resources.ResourceSet, error) {
	var allResources resources.ResourceSet
	
	// Track refs and names for duplicate detection
	portalRefs := make(map[string]string)      // ref -> file path
	portalNames := make(map[string]string)     // name -> file path
	authStratRefs := make(map[string]string)   // ref -> file path
	authStratNames := make(map[string]string)  // name -> file path
	cpRefs := make(map[string]string)          // ref -> file path
	cpNames := make(map[string]string)         // name -> file path
	apiRefs := make(map[string]string)         // ref -> file path
	apiNames := make(map[string]string)        // name -> file path
	
	err := filepath.Walk(l.rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories
		if info.IsDir() {
			return nil
		}
		
		// Only process .yaml and .yml files
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		
		// Load file without validation (will validate merged result later)
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", path, err)
		}
		defer file.Close()
		
		rs, err := l.parseYAML(file, path)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}
		
		// Merge resources with duplicate detection
		// Portals
		for _, portal := range rs.Portals {
			// Check ref uniqueness
			if existingPath, exists := portalRefs[portal.Ref]; exists {
				return fmt.Errorf("duplicate portal ref '%s' found in %s (already defined in %s)", 
					portal.Ref, path, existingPath)
			}
			// Check name uniqueness
			if existingPath, exists := portalNames[portal.Name]; exists {
				return fmt.Errorf("duplicate portal name '%s' found in %s (already defined in %s with ref '%s')", 
					portal.Name, path, existingPath, l.findRefByName(allResources.Portals, portal.Name))
			}
			portalRefs[portal.Ref] = path
			portalNames[portal.Name] = path
			allResources.Portals = append(allResources.Portals, portal)
		}
		
		// Auth strategies
		for _, authStrat := range rs.ApplicationAuthStrategies {
			// Check ref uniqueness
			if existingPath, exists := authStratRefs[authStrat.Ref]; exists {
				return fmt.Errorf("duplicate application_auth_strategy ref '%s' found in %s (already defined in %s)", 
					authStrat.Ref, path, existingPath)
			}
			// Check name uniqueness
			authName := authStrat.GetName()
			if existingPath, exists := authStratNames[authName]; exists {
				existingRef := l.findRefByName(allResources.ApplicationAuthStrategies, authName)
				return fmt.Errorf(
					"duplicate application_auth_strategy name '%s' found in %s (already defined in %s with ref '%s')", 
					authName, path, existingPath, existingRef)
			}
			authStratRefs[authStrat.Ref] = path
			authStratNames[authName] = path
			allResources.ApplicationAuthStrategies = append(allResources.ApplicationAuthStrategies, authStrat)
		}
		
		// Control planes
		for _, cp := range rs.ControlPlanes {
			// Check ref uniqueness
			if existingPath, exists := cpRefs[cp.Ref]; exists {
				return fmt.Errorf("duplicate control_plane ref '%s' found in %s (already defined in %s)", 
					cp.Ref, path, existingPath)
			}
			// Check name uniqueness
			if existingPath, exists := cpNames[cp.Name]; exists {
				return fmt.Errorf("duplicate control_plane name '%s' found in %s (already defined in %s with ref '%s')", 
					cp.Name, path, existingPath, l.findRefByName(allResources.ControlPlanes, cp.Name))
			}
			cpRefs[cp.Ref] = path
			cpNames[cp.Name] = path
			allResources.ControlPlanes = append(allResources.ControlPlanes, cp)
		}
		
		// APIs
		for _, api := range rs.APIs {
			// Check ref uniqueness
			if existingPath, exists := apiRefs[api.Ref]; exists {
				return fmt.Errorf("duplicate api ref '%s' found in %s (already defined in %s)", 
					api.Ref, path, existingPath)
			}
			// Check name uniqueness
			if existingPath, exists := apiNames[api.Name]; exists {
				return fmt.Errorf("duplicate api name '%s' found in %s (already defined in %s with ref '%s')", 
					api.Name, path, existingPath, l.findRefByName(allResources.APIs, api.Name))
			}
			apiRefs[api.Ref] = path
			apiNames[api.Name] = path
			allResources.APIs = append(allResources.APIs, api)
		}
		
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	// Apply defaults to merged resources
	l.applyDefaults(&allResources)
	
	// Validate merged resources
	if err := l.validateResourceSet(&allResources); err != nil {
		return nil, err
	}
	
	return &allResources, nil
}

// findRefByName is a generic helper to find ref by name for any resource type
func (l *Loader) findRefByName(resourceList interface{}, name string) string {
	switch res := resourceList.(type) {
	case []resources.PortalResource:
		for _, r := range res {
			if r.Name == name {
				return r.Ref
			}
		}
	case []resources.ApplicationAuthStrategyResource:
		for _, r := range res {
			if r.GetName() == name {
				return r.Ref
			}
		}
	case []resources.ControlPlaneResource:
		for _, r := range res {
			if r.Name == name {
				return r.Ref
			}
		}
	case []resources.APIResource:
		for _, r := range res {
			if r.Name == name {
				return r.Ref
			}
		}
	}
	return ""
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