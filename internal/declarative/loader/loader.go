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
}

// New creates a new configuration loader
func New() *Loader {
	return &Loader{}
}

// NewWithPath creates a new configuration loader with a root path (deprecated, for backward compatibility)
func NewWithPath(_ string) *Loader {
	return &Loader{}
}

// LoadFromSources loads configuration from multiple sources
func (l *Loader) LoadFromSources(sources []Source, recursive bool) (*resources.ResourceSet, error) {
	var allResources resources.ResourceSet
	
	// Track refs and names for duplicate detection across all sources
	portalRefs := make(map[string]string)      // ref -> source path
	portalNames := make(map[string]string)     // name -> source path
	authStratRefs := make(map[string]string)   // ref -> source path
	authStratNames := make(map[string]string)  // name -> source path
	cpRefs := make(map[string]string)          // ref -> source path
	cpNames := make(map[string]string)         // name -> source path
	apiRefs := make(map[string]string)         // ref -> source path
	apiNames := make(map[string]string)        // name -> source path
	
	for _, source := range sources {
		var rs *resources.ResourceSet
		var err error
		
		switch source.Type {
		case SourceTypeFile:
			rs, err = l.loadSingleFile(source.Path)
		case SourceTypeDirectory:
			rs, err = l.loadDirectorySource(source.Path, recursive, 
				portalRefs, portalNames, authStratRefs, authStratNames,
				cpRefs, cpNames, apiRefs, apiNames)
		case SourceTypeSTDIN:
			rs, err = l.loadSTDIN()
		default:
			return nil, fmt.Errorf("unknown source type for %s", source.Path)
		}
		
		if err != nil {
			return nil, err
		}
		
		// Merge resources with duplicate detection
		// For directories, duplicates are already checked during loading
		if source.Type != SourceTypeDirectory {
			if err := l.mergeResourceSet(&allResources, rs, source.Path,
				portalRefs, portalNames, authStratRefs, authStratNames,
				cpRefs, cpNames, apiRefs, apiNames); err != nil {
				return nil, err
			}
		} else {
			// For directories, just append since duplicates were already checked
			allResources.Portals = append(allResources.Portals, rs.Portals...)
			allResources.ApplicationAuthStrategies = append(allResources.ApplicationAuthStrategies,
				rs.ApplicationAuthStrategies...)
			allResources.ControlPlanes = append(allResources.ControlPlanes, rs.ControlPlanes...)
			allResources.APIs = append(allResources.APIs, rs.APIs...)
		}
	}
	
	// Apply defaults to merged resources
	l.applyDefaults(&allResources)
	
	// Validate merged resources
	if err := l.validateResourceSet(&allResources); err != nil {
		return nil, err
	}
	
	return &allResources, nil
}

// Load loads configuration from the default current directory (deprecated)
func (l *Loader) Load() (*resources.ResourceSet, error) {
	sources := []Source{{Path: ".", Type: SourceTypeDirectory}}
	return l.LoadFromSources(sources, true) // Keep recursive for backward compatibility
}

// LoadFile loads configuration from a single YAML file (deprecated, for backward compatibility)
func (l *Loader) LoadFile(path string) (*resources.ResourceSet, error) {
	return l.loadSingleFile(path)
}

// loadSingleFile loads configuration from a single YAML file
func (l *Loader) loadSingleFile(path string) (*resources.ResourceSet, error) {
	// Validate YAML extension
	if !ValidateYAMLFile(path) {
		return nil, fmt.Errorf("file %s does not have .yaml or .yml extension", path)
	}
	
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer file.Close()

	rs, err := l.parseYAML(file, path)
	if err != nil {
		return nil, err
	}
	
	// Don't validate here - will be validated after merging all sources
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

// loadSTDIN loads configuration from stdin
func (l *Loader) loadSTDIN() (*resources.ResourceSet, error) {
	// Check if stdin has data
	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat stdin: %w", err)
	}
	
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return nil, fmt.Errorf("no data provided on stdin")
	}
	
	rs, err := l.parseYAML(os.Stdin, "stdin")
	if err != nil {
		return nil, err
	}
	
	return rs, nil
}

// loadDirectorySource loads YAML files from a directory
func (l *Loader) loadDirectorySource(dirPath string, recursive bool,
	portalRefs, portalNames, authStratRefs, authStratNames,
	cpRefs, cpNames, apiRefs, apiNames map[string]string) (*resources.ResourceSet, error) {
	
	var allResources resources.ResourceSet
	yamlCount := 0
	subdirCount := 0
	
	// First, check direct YAML files in the directory
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}
	
	for _, entry := range entries {
		path := filepath.Join(dirPath, entry.Name())
		
		if entry.IsDir() {
			subdirCount++
			if recursive {
				// Recursively load subdirectory
				subRS, err := l.loadDirectorySource(path, recursive,
					portalRefs, portalNames, authStratRefs, authStratNames,
					cpRefs, cpNames, apiRefs, apiNames)
				if err != nil {
					return nil, err
				}
				
				// Merge is handled by parent since we pass the maps
				allResources.Portals = append(allResources.Portals, subRS.Portals...)
				allResources.ApplicationAuthStrategies = append(allResources.ApplicationAuthStrategies,
					subRS.ApplicationAuthStrategies...)
				allResources.ControlPlanes = append(allResources.ControlPlanes, subRS.ControlPlanes...)
				allResources.APIs = append(allResources.APIs, subRS.APIs...)
			}
			continue
		}
		
		// Skip non-YAML files
		if !ValidateYAMLFile(path) {
			continue
		}
		
		yamlCount++
		
		// Load file without validation (will validate merged result later)
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open %s: %w", path, err)
		}
		
		rs, err := l.parseYAML(file, path)
		file.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", path, err)
		}
		
		// Check duplicates during merge (inline to reuse the maps)
		// This is the fail-fast duplicate detection logic
		for _, portal := range rs.Portals {
			if existingPath, exists := portalRefs[portal.Ref]; exists {
				return nil, fmt.Errorf("duplicate portal ref '%s' found in %s (already defined in %s)", 
					portal.Ref, path, existingPath)
			}
			if existingPath, exists := portalNames[portal.Name]; exists {
				return nil, fmt.Errorf("duplicate portal name '%s' found in %s (already defined in %s with ref '%s')", 
					portal.Name, path, existingPath, l.findRefByName(allResources.Portals, portal.Name))
			}
			portalRefs[portal.Ref] = path
			portalNames[portal.Name] = path
			allResources.Portals = append(allResources.Portals, portal)
		}
		
		for _, authStrat := range rs.ApplicationAuthStrategies {
			if existingPath, exists := authStratRefs[authStrat.Ref]; exists {
				return nil, fmt.Errorf("duplicate application_auth_strategy ref '%s' found in %s (already defined in %s)", 
					authStrat.Ref, path, existingPath)
			}
			authName := authStrat.GetName()
			if existingPath, exists := authStratNames[authName]; exists {
				existingRef := l.findRefByName(allResources.ApplicationAuthStrategies, authName)
				return nil, fmt.Errorf(
					"duplicate application_auth_strategy name '%s' found in %s (already defined in %s with ref '%s')", 
					authName, path, existingPath, existingRef)
			}
			authStratRefs[authStrat.Ref] = path
			authStratNames[authName] = path
			allResources.ApplicationAuthStrategies = append(allResources.ApplicationAuthStrategies, authStrat)
		}
		
		for _, cp := range rs.ControlPlanes {
			if existingPath, exists := cpRefs[cp.Ref]; exists {
				return nil, fmt.Errorf("duplicate control_plane ref '%s' found in %s (already defined in %s)", 
					cp.Ref, path, existingPath)
			}
			if existingPath, exists := cpNames[cp.Name]; exists {
				return nil, fmt.Errorf("duplicate control_plane name '%s' found in %s (already defined in %s with ref '%s')", 
					cp.Name, path, existingPath, l.findRefByName(allResources.ControlPlanes, cp.Name))
			}
			cpRefs[cp.Ref] = path
			cpNames[cp.Name] = path
			allResources.ControlPlanes = append(allResources.ControlPlanes, cp)
		}
		
		for _, api := range rs.APIs {
			if existingPath, exists := apiRefs[api.Ref]; exists {
				return nil, fmt.Errorf("duplicate api ref '%s' found in %s (already defined in %s)", 
					api.Ref, path, existingPath)
			}
			if existingPath, exists := apiNames[api.Name]; exists {
				return nil, fmt.Errorf("duplicate api name '%s' found in %s (already defined in %s with ref '%s')", 
					api.Name, path, existingPath, l.findRefByName(allResources.APIs, api.Name))
			}
			apiRefs[api.Ref] = path
			apiNames[api.Name] = path
			allResources.APIs = append(allResources.APIs, api)
		}
	}
	
	// Provide helpful error if no YAML files found
	if yamlCount == 0 && subdirCount > 0 && !recursive {
		return nil, fmt.Errorf("no YAML files found in directory '%s'. Found %d subdirectories. "+
			"Use -R to search subdirectories", dirPath, subdirCount)
	} else if yamlCount == 0 && len(allResources.Portals) == 0 && 
		len(allResources.ApplicationAuthStrategies) == 0 &&
		len(allResources.ControlPlanes) == 0 && len(allResources.APIs) == 0 {
		// Only error if no files were found at all (not just empty files)
		return nil, fmt.Errorf("no YAML files found in directory '%s'", dirPath)
	}
	
	return &allResources, nil
}

// mergeResourceSet merges source resources into target with duplicate detection
func (l *Loader) mergeResourceSet(target, source *resources.ResourceSet, sourcePath string,
	portalRefs, portalNames, authStratRefs, authStratNames,
	cpRefs, cpNames, apiRefs, apiNames map[string]string) error {
	
	// For single files and STDIN, we need to check duplicates
	// For directories, duplicates are already checked during loading
	
	// Merge portals
	for _, portal := range source.Portals {
		if existingPath, exists := portalRefs[portal.Ref]; exists {
			return fmt.Errorf("duplicate portal ref '%s' found in %s (already defined in %s)", 
				portal.Ref, sourcePath, existingPath)
		}
		if existingPath, exists := portalNames[portal.Name]; exists {
			return fmt.Errorf("duplicate portal name '%s' found in %s (already defined in %s with ref '%s')", 
				portal.Name, sourcePath, existingPath, l.findRefByName(target.Portals, portal.Name))
		}
		portalRefs[portal.Ref] = sourcePath
		portalNames[portal.Name] = sourcePath
		target.Portals = append(target.Portals, portal)
	}
	
	// Merge auth strategies
	for _, authStrat := range source.ApplicationAuthStrategies {
		if existingPath, exists := authStratRefs[authStrat.Ref]; exists {
			return fmt.Errorf("duplicate application_auth_strategy ref '%s' found in %s (already defined in %s)", 
				authStrat.Ref, sourcePath, existingPath)
		}
		authName := authStrat.GetName()
		if existingPath, exists := authStratNames[authName]; exists {
			existingRef := l.findRefByName(target.ApplicationAuthStrategies, authName)
			return fmt.Errorf(
				"duplicate application_auth_strategy name '%s' found in %s (already defined in %s with ref '%s')", 
				authName, sourcePath, existingPath, existingRef)
		}
		authStratRefs[authStrat.Ref] = sourcePath
		authStratNames[authName] = sourcePath
		target.ApplicationAuthStrategies = append(target.ApplicationAuthStrategies, authStrat)
	}
	
	// Merge control planes
	for _, cp := range source.ControlPlanes {
		if existingPath, exists := cpRefs[cp.Ref]; exists {
			return fmt.Errorf("duplicate control_plane ref '%s' found in %s (already defined in %s)", 
				cp.Ref, sourcePath, existingPath)
		}
		if existingPath, exists := cpNames[cp.Name]; exists {
			return fmt.Errorf("duplicate control_plane name '%s' found in %s (already defined in %s with ref '%s')", 
				cp.Name, sourcePath, existingPath, l.findRefByName(target.ControlPlanes, cp.Name))
		}
		cpRefs[cp.Ref] = sourcePath
		cpNames[cp.Name] = sourcePath
		target.ControlPlanes = append(target.ControlPlanes, cp)
	}
	
	// Merge APIs
	for _, api := range source.APIs {
		if existingPath, exists := apiRefs[api.Ref]; exists {
			return fmt.Errorf("duplicate api ref '%s' found in %s (already defined in %s)", 
				api.Ref, sourcePath, existingPath)
		}
		if existingPath, exists := apiNames[api.Name]; exists {
			return fmt.Errorf("duplicate api name '%s' found in %s (already defined in %s with ref '%s')", 
				api.Name, sourcePath, existingPath, l.findRefByName(target.APIs, api.Name))
		}
		apiRefs[api.Ref] = sourcePath
		apiNames[api.Name] = sourcePath
		target.APIs = append(target.APIs, api)
	}
	
	return nil
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