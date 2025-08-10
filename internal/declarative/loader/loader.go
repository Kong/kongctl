package loader

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"sigs.k8s.io/yaml"
)

// temporaryParseResult holds the raw parsed YAML including defaults
// This is used internally during parsing to capture both resources and file-level defaults
type temporaryParseResult struct {
	Defaults *resources.FileDefaults `json:"_defaults,omitempty" yaml:"_defaults,omitempty"`
	resources.ResourceSet         `yaml:",inline"`
}

// Loader handles loading declarative configuration from files
type Loader struct {
	// baseDir is the base directory for resolving relative file paths in tags
	baseDir string
	// tagRegistry is the registry of tag resolvers (created on demand)
	tagRegistry *tags.ResolverRegistry
}

// New creates a new configuration loader
func New() *Loader {
	return &Loader{
		baseDir: ".", // Default to current directory
	}
}

// NewWithPath creates a new configuration loader with a root path (deprecated, for backward compatibility)
func NewWithPath(_ string) *Loader {
	return &Loader{
		baseDir: ".",
	}
}

// getTagRegistry returns the tag registry, creating it if needed
func (l *Loader) getTagRegistry() *tags.ResolverRegistry {
	if l.tagRegistry == nil {
		l.tagRegistry = tags.NewResolverRegistry()
	}
	return l.tagRegistry
}

// LoadFromSources loads configuration from multiple sources
func (l *Loader) LoadFromSources(sources []Source, recursive bool) (*resources.ResourceSet, error) {
	var allResources resources.ResourceSet
	
	// Use global ref registry to enforce uniqueness across ALL resource types
	globalRefRegistry := resources.NewGlobalRefRegistry()
	refSourceMap := make(map[string]string) // ref -> source path for error messages
	
	// Still track names separately per type (names are only unique within type)
	portalNames := make(map[string]string)     // name -> source path
	authStratNames := make(map[string]string)  // name -> source path
	cpNames := make(map[string]string)         // name -> source path
	apiNames := make(map[string]string)        // name -> source path
	
	for _, source := range sources {
		var rs *resources.ResourceSet
		var err error
		
		switch source.Type {
		case SourceTypeFile:
			rs, err = l.loadSingleFile(source.Path)
		case SourceTypeDirectory:
			rs, err = l.loadDirectorySource(source.Path, recursive, 
				globalRefRegistry, refSourceMap,
				portalNames, authStratNames, cpNames, apiNames)
		case SourceTypeSTDIN:
			rs, err = l.loadSTDIN()
		default:
			return nil, errors.FormatConfigurationError(source.Path, 0, fmt.Sprintf("unknown source type: %v", source.Type))
		}
		
		if err != nil {
			return nil, err
		}
		
		// Merge resources with duplicate detection
		// For directories, duplicates are already checked during loading
		if source.Type != SourceTypeDirectory {
			if err := l.mergeResourceSet(&allResources, rs, source.Path,
				globalRefRegistry, refSourceMap,
				portalNames, authStratNames, cpNames, apiNames); err != nil {
				return nil, err
			}
		} else {
			// For directories, just append since duplicates were already checked
			allResources.Portals = append(allResources.Portals, rs.Portals...)
			allResources.ApplicationAuthStrategies = append(allResources.ApplicationAuthStrategies,
				rs.ApplicationAuthStrategies...)
			allResources.ControlPlanes = append(allResources.ControlPlanes, rs.ControlPlanes...)
			allResources.APIs = append(allResources.APIs, rs.APIs...)
			allResources.APIVersions = append(allResources.APIVersions, rs.APIVersions...)
			allResources.APIPublications = append(allResources.APIPublications, rs.APIPublications...)
			allResources.APIImplementations = append(allResources.APIImplementations, rs.APIImplementations...)
			allResources.APIDocuments = append(allResources.APIDocuments, rs.APIDocuments...)
			allResources.PortalCustomizations = append(allResources.PortalCustomizations, rs.PortalCustomizations...)
			allResources.PortalCustomDomains = append(allResources.PortalCustomDomains, rs.PortalCustomDomains...)
			allResources.PortalPages = append(allResources.PortalPages, rs.PortalPages...)
			allResources.PortalSnippets = append(allResources.PortalSnippets, rs.PortalSnippets...)
		}
	}
	
	// Apply SDK defaults to merged resources
	// Note: Namespace defaults were already applied per-file in parseYAML
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
	rs, err := l.loadSingleFile(path)
	if err != nil {
		return nil, err
	}
	
	// Validate for backward compatibility
	if err := l.validateResourceSet(rs); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}
	
	return rs, nil
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
	var temp temporaryParseResult

	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read content from %s: %w", sourcePath, err)
	}

	// Process custom tags if needed
	registry := l.getTagRegistry()
	
	// Update base directory based on source file location
	baseDir := l.baseDir
	if sourcePath != "stdin" && sourcePath != "" {
		baseDir = filepath.Dir(sourcePath)
	}
	
	// Always register/update file resolver with correct base directory
	// This ensures each file gets the correct base directory for relative paths
	registry.Register(tags.NewFileTagResolver(baseDir))
	
	if registry.HasResolvers() {
		processedContent, err := registry.Process(content)
		if err != nil {
			return nil, fmt.Errorf("failed to process tags in %s: %w", sourcePath, err)
		}
		content = processedContent
	}

	if err := yaml.UnmarshalStrict(content, &temp); err != nil {
		// Try to provide a more helpful error message for unknown fields
		errMsg := err.Error()
		if strings.Contains(errMsg, "unknown field") {
			// Extract field name from error
			// Error format: "error unmarshaling JSON: while decoding JSON: json: unknown field \"fieldname\""
			if match := regexp.MustCompile(`unknown field "(\w+)"`).FindStringSubmatch(errMsg); len(match) > 1 {
				fieldName := match[1]
				suggestion := l.suggestFieldName(fieldName)
				if suggestion != "" {
					return nil, fmt.Errorf("unknown field '%s' in %s. Did you mean '%s'?", 
						fieldName, sourcePath, suggestion)
				}
				return nil, fmt.Errorf("unknown field '%s' in %s. Please check the field name against the schema", 
					fieldName, sourcePath)
			}
		}
		return nil, fmt.Errorf("failed to parse YAML in %s: %w", sourcePath, err)
	}

	// Extract the clean ResourceSet
	rs := temp.ResourceSet
	
	// Apply file-level namespace and protected defaults
	if err := l.applyNamespaceDefaults(&rs, temp.Defaults); err != nil {
		return nil, fmt.Errorf("failed to apply namespace defaults: %w", err)
	}

	// Validate API version constraints before extraction
	// This ensures we catch multiple versions as configured by the user
	for i := range rs.APIs {
		api := &rs.APIs[i]
		if len(api.Versions) > 1 {
			return nil, fmt.Errorf("api %q defines %d versions, but Konnect currently supports only one version per API. "+
				"Ensure each API versions key has only 1 version defined", api.GetRef(), len(api.Versions))
		}
	}

	// Extract nested child resources to root level first
	l.extractNestedResources(&rs)

	// Apply SDK defaults to all resources (including extracted child resources)
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
	globalRefRegistry *resources.GlobalRefRegistry, refSourceMap map[string]string,
	portalNames, authStratNames, cpNames, apiNames map[string]string,
) (*resources.ResourceSet, error) {
	
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
					globalRefRegistry, refSourceMap,
					portalNames, authStratNames, cpNames, apiNames)
				if err != nil {
					return nil, err
				}
				
				// Merge is handled by parent since we pass the maps
				allResources.Portals = append(allResources.Portals, subRS.Portals...)
				allResources.ApplicationAuthStrategies = append(allResources.ApplicationAuthStrategies,
					subRS.ApplicationAuthStrategies...)
				allResources.ControlPlanes = append(allResources.ControlPlanes, subRS.ControlPlanes...)
				allResources.APIs = append(allResources.APIs, subRS.APIs...)
				allResources.APIVersions = append(allResources.APIVersions, subRS.APIVersions...)
				allResources.APIPublications = append(allResources.APIPublications, subRS.APIPublications...)
				allResources.APIImplementations = append(allResources.APIImplementations, subRS.APIImplementations...)
				allResources.APIDocuments = append(allResources.APIDocuments, subRS.APIDocuments...)
				allResources.PortalCustomizations = append(allResources.PortalCustomizations, subRS.PortalCustomizations...)
				allResources.PortalCustomDomains = append(allResources.PortalCustomDomains, subRS.PortalCustomDomains...)
				allResources.PortalPages = append(allResources.PortalPages, subRS.PortalPages...)
				allResources.PortalSnippets = append(allResources.PortalSnippets, subRS.PortalSnippets...)
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
			// Check global ref uniqueness
			if err := globalRefRegistry.AddRef(portal.Ref, "portal"); err != nil {
				if existingPath, exists := refSourceMap[portal.Ref]; exists {
					return nil, fmt.Errorf("%w (found in %s, already defined in %s)", err, path, existingPath)
				}
				return nil, fmt.Errorf("%w (found in %s)", err, path)
			}
			refSourceMap[portal.Ref] = path
			
			// Check name uniqueness (within type)
			if existingPath, exists := portalNames[portal.Name]; exists {
				return nil, fmt.Errorf("duplicate portal name '%s' found in %s (already defined in %s with ref '%s')", 
					portal.Name, path, existingPath, l.findRefByName(allResources.Portals, portal.Name))
			}
			portalNames[portal.Name] = path
			allResources.Portals = append(allResources.Portals, portal)
		}
		
		for _, authStrat := range rs.ApplicationAuthStrategies {
			// Check global ref uniqueness
			if err := globalRefRegistry.AddRef(authStrat.Ref, "application_auth_strategy"); err != nil {
				if existingPath, exists := refSourceMap[authStrat.Ref]; exists {
					return nil, fmt.Errorf("%w (found in %s, already defined in %s)", err, path, existingPath)
				}
				return nil, fmt.Errorf("%w (found in %s)", err, path)
			}
			refSourceMap[authStrat.Ref] = path
			
			// Check name uniqueness (within type)
			authName := authStrat.GetMoniker()
			if existingPath, exists := authStratNames[authName]; exists {
				existingRef := l.findRefByName(allResources.ApplicationAuthStrategies, authName)
				return nil, fmt.Errorf(
					"duplicate application_auth_strategy name '%s' found in %s (already defined in %s with ref '%s')", 
					authName, path, existingPath, existingRef)
			}
			authStratNames[authName] = path
			allResources.ApplicationAuthStrategies = append(allResources.ApplicationAuthStrategies, authStrat)
		}
		
		for _, cp := range rs.ControlPlanes {
			// Check global ref uniqueness
			if err := globalRefRegistry.AddRef(cp.Ref, "control_plane"); err != nil {
				if existingPath, exists := refSourceMap[cp.Ref]; exists {
					return nil, fmt.Errorf("%w (found in %s, already defined in %s)", err, path, existingPath)
				}
				return nil, fmt.Errorf("%w (found in %s)", err, path)
			}
			refSourceMap[cp.Ref] = path
			
			// Check name uniqueness (within type)
			if existingPath, exists := cpNames[cp.Name]; exists {
				return nil, fmt.Errorf("duplicate control_plane name '%s' found in %s (already defined in %s with ref '%s')", 
					cp.Name, path, existingPath, l.findRefByName(allResources.ControlPlanes, cp.Name))
			}
			cpNames[cp.Name] = path
			allResources.ControlPlanes = append(allResources.ControlPlanes, cp)
		}
		
		for _, api := range rs.APIs {
			// Check global ref uniqueness
			if err := globalRefRegistry.AddRef(api.Ref, "api"); err != nil {
				if existingPath, exists := refSourceMap[api.Ref]; exists {
					return nil, fmt.Errorf("%w (found in %s, already defined in %s)", err, path, existingPath)
				}
				return nil, fmt.Errorf("%w (found in %s)", err, path)
			}
			refSourceMap[api.Ref] = path
			
			// Check name uniqueness (within type)
			if existingPath, exists := apiNames[api.Name]; exists {
				return nil, fmt.Errorf("duplicate api name '%s' found in %s (already defined in %s with ref '%s')", 
					api.Name, path, existingPath, l.findRefByName(allResources.APIs, api.Name))
			}
			apiNames[api.Name] = path
			allResources.APIs = append(allResources.APIs, api)
		}
		
		// Check duplicates for API child resources
		for _, version := range rs.APIVersions {
			if err := globalRefRegistry.AddRef(version.Ref, "api_version"); err != nil {
				if existingPath, exists := refSourceMap[version.Ref]; exists {
					return nil, fmt.Errorf("%w (found in %s, already defined in %s)", err, path, existingPath)
				}
				return nil, fmt.Errorf("%w (found in %s)", err, path)
			}
			refSourceMap[version.Ref] = path
			allResources.APIVersions = append(allResources.APIVersions, version)
		}
		
		for _, pub := range rs.APIPublications {
			if err := globalRefRegistry.AddRef(pub.Ref, "api_publication"); err != nil {
				if existingPath, exists := refSourceMap[pub.Ref]; exists {
					return nil, fmt.Errorf("%w (found in %s, already defined in %s)", err, path, existingPath)
				}
				return nil, fmt.Errorf("%w (found in %s)", err, path)
			}
			refSourceMap[pub.Ref] = path
			allResources.APIPublications = append(allResources.APIPublications, pub)
		}
		
		for _, impl := range rs.APIImplementations {
			if err := globalRefRegistry.AddRef(impl.Ref, "api_implementation"); err != nil {
				if existingPath, exists := refSourceMap[impl.Ref]; exists {
					return nil, fmt.Errorf("%w (found in %s, already defined in %s)", err, path, existingPath)
				}
				return nil, fmt.Errorf("%w (found in %s)", err, path)
			}
			refSourceMap[impl.Ref] = path
			allResources.APIImplementations = append(allResources.APIImplementations, impl)
		}
		
		for _, doc := range rs.APIDocuments {
			if err := globalRefRegistry.AddRef(doc.Ref, "api_document"); err != nil {
				if existingPath, exists := refSourceMap[doc.Ref]; exists {
					return nil, fmt.Errorf("%w (found in %s, already defined in %s)", err, path, existingPath)
				}
				return nil, fmt.Errorf("%w (found in %s)", err, path)
			}
			refSourceMap[doc.Ref] = path
			allResources.APIDocuments = append(allResources.APIDocuments, doc)
		}
		
		// Check duplicates for Portal child resources
		for _, customization := range rs.PortalCustomizations {
			if err := globalRefRegistry.AddRef(customization.Ref, "portal_customization"); err != nil {
				if existingPath, exists := refSourceMap[customization.Ref]; exists {
					return nil, fmt.Errorf("%w (found in %s, already defined in %s)", err, path, existingPath)
				}
				return nil, fmt.Errorf("%w (found in %s)", err, path)
			}
			refSourceMap[customization.Ref] = path
			allResources.PortalCustomizations = append(allResources.PortalCustomizations, customization)
		}
		
		for _, domain := range rs.PortalCustomDomains {
			if err := globalRefRegistry.AddRef(domain.Ref, "portal_custom_domain"); err != nil {
				if existingPath, exists := refSourceMap[domain.Ref]; exists {
					return nil, fmt.Errorf("%w (found in %s, already defined in %s)", err, path, existingPath)
				}
				return nil, fmt.Errorf("%w (found in %s)", err, path)
			}
			refSourceMap[domain.Ref] = path
			allResources.PortalCustomDomains = append(allResources.PortalCustomDomains, domain)
		}
		
		for _, page := range rs.PortalPages {
			if err := globalRefRegistry.AddRef(page.Ref, "portal_page"); err != nil {
				if existingPath, exists := refSourceMap[page.Ref]; exists {
					return nil, fmt.Errorf("%w (found in %s, already defined in %s)", err, path, existingPath)
				}
				return nil, fmt.Errorf("%w (found in %s)", err, path)
			}
			refSourceMap[page.Ref] = path
			allResources.PortalPages = append(allResources.PortalPages, page)
		}
		
		for _, snippet := range rs.PortalSnippets {
			if err := globalRefRegistry.AddRef(snippet.Ref, "portal_snippet"); err != nil {
				if existingPath, exists := refSourceMap[snippet.Ref]; exists {
					return nil, fmt.Errorf("%w (found in %s, already defined in %s)", err, path, existingPath)
				}
				return nil, fmt.Errorf("%w (found in %s)", err, path)
			}
			refSourceMap[snippet.Ref] = path
			allResources.PortalSnippets = append(allResources.PortalSnippets, snippet)
		}
	}
	
	// Provide helpful error if no YAML files found
	if yamlCount == 0 && subdirCount > 0 && !recursive {
		return nil, fmt.Errorf("no YAML files found in directory '%s'. Found %d subdirectories. "+
			"Use -R to search subdirectories", dirPath, subdirCount)
	} else if yamlCount == 0 && len(allResources.Portals) == 0 && 
		len(allResources.ApplicationAuthStrategies) == 0 &&
		len(allResources.ControlPlanes) == 0 && len(allResources.APIs) == 0 &&
		len(allResources.APIVersions) == 0 && len(allResources.APIPublications) == 0 &&
		len(allResources.APIImplementations) == 0 && len(allResources.APIDocuments) == 0 &&
		len(allResources.PortalCustomizations) == 0 && len(allResources.PortalCustomDomains) == 0 &&
		len(allResources.PortalPages) == 0 && len(allResources.PortalSnippets) == 0 {
		// Only error if no files were found at all (not just empty files)
		return nil, fmt.Errorf("no YAML files found in directory '%s'", dirPath)
	}
	
	return &allResources, nil
}

// mergeResourceSet merges source resources into target with duplicate detection
func (l *Loader) mergeResourceSet(target, source *resources.ResourceSet, sourcePath string,
	globalRefRegistry *resources.GlobalRefRegistry, refSourceMap map[string]string,
	portalNames, authStratNames, cpNames, apiNames map[string]string) error {
	
	// For single files and STDIN, we need to check duplicates
	// For directories, duplicates are already checked during loading
	
	// Merge portals
	for _, portal := range source.Portals {
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(portal.Ref, "portal"); err != nil {
			if existingPath, exists := refSourceMap[portal.Ref]; exists {
				return fmt.Errorf("%w (found in %s, already defined in %s)", err, sourcePath, existingPath)
			}
			return fmt.Errorf("%w (found in %s)", err, sourcePath)
		}
		refSourceMap[portal.Ref] = sourcePath
		
		// Check name uniqueness (within type)
		if existingPath, exists := portalNames[portal.Name]; exists {
			return fmt.Errorf("duplicate portal name '%s' found in %s (already defined in %s with ref '%s')", 
				portal.Name, sourcePath, existingPath, l.findRefByName(target.Portals, portal.Name))
		}
		portalNames[portal.Name] = sourcePath
		target.Portals = append(target.Portals, portal)
	}
	
	// Merge auth strategies
	for _, authStrat := range source.ApplicationAuthStrategies {
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(authStrat.Ref, "application_auth_strategy"); err != nil {
			if existingPath, exists := refSourceMap[authStrat.Ref]; exists {
				return fmt.Errorf("%w (found in %s, already defined in %s)", err, sourcePath, existingPath)
			}
			return fmt.Errorf("%w (found in %s)", err, sourcePath)
		}
		refSourceMap[authStrat.Ref] = sourcePath
		
		// Check name uniqueness (within type)
		authName := authStrat.GetMoniker()
		if existingPath, exists := authStratNames[authName]; exists {
			existingRef := l.findRefByName(target.ApplicationAuthStrategies, authName)
			return fmt.Errorf(
				"duplicate application_auth_strategy name '%s' found in %s (already defined in %s with ref '%s')", 
				authName, sourcePath, existingPath, existingRef)
		}
		authStratNames[authName] = sourcePath
		target.ApplicationAuthStrategies = append(target.ApplicationAuthStrategies, authStrat)
	}
	
	// Merge control planes
	for _, cp := range source.ControlPlanes {
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(cp.Ref, "control_plane"); err != nil {
			if existingPath, exists := refSourceMap[cp.Ref]; exists {
				return fmt.Errorf("%w (found in %s, already defined in %s)", err, sourcePath, existingPath)
			}
			return fmt.Errorf("%w (found in %s)", err, sourcePath)
		}
		refSourceMap[cp.Ref] = sourcePath
		
		// Check name uniqueness (within type)
		if existingPath, exists := cpNames[cp.Name]; exists {
			return fmt.Errorf("duplicate control_plane name '%s' found in %s (already defined in %s with ref '%s')", 
				cp.Name, sourcePath, existingPath, l.findRefByName(target.ControlPlanes, cp.Name))
		}
		cpNames[cp.Name] = sourcePath
		target.ControlPlanes = append(target.ControlPlanes, cp)
	}
	
	// Merge APIs
	for _, api := range source.APIs {
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(api.Ref, "api"); err != nil {
			if existingPath, exists := refSourceMap[api.Ref]; exists {
				return fmt.Errorf("%w (found in %s, already defined in %s)", err, sourcePath, existingPath)
			}
			return fmt.Errorf("%w (found in %s)", err, sourcePath)
		}
		refSourceMap[api.Ref] = sourcePath
		
		// Check name uniqueness (within type)
		if existingPath, exists := apiNames[api.Name]; exists {
			return fmt.Errorf("duplicate api name '%s' found in %s (already defined in %s with ref '%s')", 
				api.Name, sourcePath, existingPath, l.findRefByName(target.APIs, api.Name))
		}
		apiNames[api.Name] = sourcePath
		target.APIs = append(target.APIs, api)
	}
	
	// Merge API child resources
	for _, version := range source.APIVersions {
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(version.Ref, "api_version"); err != nil {
			if existingPath, exists := refSourceMap[version.Ref]; exists {
				return fmt.Errorf("%w (found in %s, already defined in %s)", err, sourcePath, existingPath)
			}
			return fmt.Errorf("%w (found in %s)", err, sourcePath)
		}
		refSourceMap[version.Ref] = sourcePath
		target.APIVersions = append(target.APIVersions, version)
	}
	
	for _, pub := range source.APIPublications {
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(pub.Ref, "api_publication"); err != nil {
			if existingPath, exists := refSourceMap[pub.Ref]; exists {
				return fmt.Errorf("%w (found in %s, already defined in %s)", err, sourcePath, existingPath)
			}
			return fmt.Errorf("%w (found in %s)", err, sourcePath)
		}
		refSourceMap[pub.Ref] = sourcePath
		target.APIPublications = append(target.APIPublications, pub)
	}
	
	for _, impl := range source.APIImplementations {
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(impl.Ref, "api_implementation"); err != nil {
			if existingPath, exists := refSourceMap[impl.Ref]; exists {
				return fmt.Errorf("%w (found in %s, already defined in %s)", err, sourcePath, existingPath)
			}
			return fmt.Errorf("%w (found in %s)", err, sourcePath)
		}
		refSourceMap[impl.Ref] = sourcePath
		target.APIImplementations = append(target.APIImplementations, impl)
	}
	
	for _, doc := range source.APIDocuments {
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(doc.Ref, "api_document"); err != nil {
			if existingPath, exists := refSourceMap[doc.Ref]; exists {
				return fmt.Errorf("%w (found in %s, already defined in %s)", err, sourcePath, existingPath)
			}
			return fmt.Errorf("%w (found in %s)", err, sourcePath)
		}
		refSourceMap[doc.Ref] = sourcePath
		target.APIDocuments = append(target.APIDocuments, doc)
	}
	
	// Merge Portal child resources
	for _, customization := range source.PortalCustomizations {
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(customization.Ref, "portal_customization"); err != nil {
			if existingPath, exists := refSourceMap[customization.Ref]; exists {
				return fmt.Errorf("%w (found in %s, already defined in %s)", err, sourcePath, existingPath)
			}
			return fmt.Errorf("%w (found in %s)", err, sourcePath)
		}
		refSourceMap[customization.Ref] = sourcePath
		target.PortalCustomizations = append(target.PortalCustomizations, customization)
	}
	
	for _, domain := range source.PortalCustomDomains {
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(domain.Ref, "portal_custom_domain"); err != nil {
			if existingPath, exists := refSourceMap[domain.Ref]; exists {
				return fmt.Errorf("%w (found in %s, already defined in %s)", err, sourcePath, existingPath)
			}
			return fmt.Errorf("%w (found in %s)", err, sourcePath)
		}
		refSourceMap[domain.Ref] = sourcePath
		target.PortalCustomDomains = append(target.PortalCustomDomains, domain)
	}
	
	for _, page := range source.PortalPages {
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(page.Ref, "portal_page"); err != nil {
			if existingPath, exists := refSourceMap[page.Ref]; exists {
				return fmt.Errorf("%w (found in %s, already defined in %s)", err, sourcePath, existingPath)
			}
			return fmt.Errorf("%w (found in %s)", err, sourcePath)
		}
		refSourceMap[page.Ref] = sourcePath
		target.PortalPages = append(target.PortalPages, page)
	}
	
	for _, snippet := range source.PortalSnippets {
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(snippet.Ref, "portal_snippet"); err != nil {
			if existingPath, exists := refSourceMap[snippet.Ref]; exists {
				return fmt.Errorf("%w (found in %s, already defined in %s)", err, sourcePath, existingPath)
			}
			return fmt.Errorf("%w (found in %s)", err, sourcePath)
		}
		refSourceMap[snippet.Ref] = sourcePath
		target.PortalSnippets = append(target.PortalSnippets, snippet)
	}
	
	// Preserve DefaultNamespace if set in source
	// When merging multiple files, the last one with a default namespace wins
	if source.DefaultNamespace != "" {
		target.DefaultNamespace = source.DefaultNamespace
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
			if r.GetMoniker() == name {
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

// applyNamespaceDefaults applies file-level namespace and protected defaults to parent resources
func (l *Loader) applyNamespaceDefaults(rs *resources.ResourceSet, fileDefaults *resources.FileDefaults) error {
	// Determine the effective namespace default
	defaultNamespace := "default"
	namespaceDefault := &defaultNamespace
	var protectedDefault *bool
	
	if fileDefaults != nil && fileDefaults.Kongctl != nil {
		// Validate that namespace default is not empty
		if fileDefaults.Kongctl.Namespace != nil && *fileDefaults.Kongctl.Namespace == "" {
			return fmt.Errorf("namespace in _defaults.kongctl cannot be empty")
		}
		if fileDefaults.Kongctl.Namespace != nil {
			namespaceDefault = fileDefaults.Kongctl.Namespace
			// Preserve the namespace from defaults in ResourceSet for planner to use
			// when no resources are present
			rs.DefaultNamespace = *namespaceDefault
		}
		protectedDefault = fileDefaults.Kongctl.Protected
	}
	
	// Apply defaults to portals (parent resources)
	for i := range rs.Portals {
		if rs.Portals[i].Kongctl == nil {
			rs.Portals[i].Kongctl = &resources.KongctlMeta{}
		}
		// Validate that explicit namespace is not empty
		if rs.Portals[i].Kongctl.Namespace != nil && *rs.Portals[i].Kongctl.Namespace == "" {
			return fmt.Errorf("portal '%s' cannot have an empty namespace", rs.Portals[i].Ref)
		}
		// Apply namespace default if not set
		if rs.Portals[i].Kongctl.Namespace == nil {
			rs.Portals[i].Kongctl.Namespace = namespaceDefault
		}
		// Apply protected default if not set
		if rs.Portals[i].Kongctl.Protected == nil && protectedDefault != nil {
			rs.Portals[i].Kongctl.Protected = protectedDefault
		}
		// Ensure protected has a value (false if still nil)
		if rs.Portals[i].Kongctl.Protected == nil {
			falseVal := false
			rs.Portals[i].Kongctl.Protected = &falseVal
		}
	}
	
	// Apply defaults to APIs (parent resources)
	for i := range rs.APIs {
		if rs.APIs[i].Kongctl == nil {
			rs.APIs[i].Kongctl = &resources.KongctlMeta{}
		}
		// Validate that explicit namespace is not empty
		if rs.APIs[i].Kongctl.Namespace != nil && *rs.APIs[i].Kongctl.Namespace == "" {
			return fmt.Errorf("api '%s' cannot have an empty namespace", rs.APIs[i].Ref)
		}
		// Apply namespace default if not set
		if rs.APIs[i].Kongctl.Namespace == nil {
			rs.APIs[i].Kongctl.Namespace = namespaceDefault
		}
		// Apply protected default if not set
		if rs.APIs[i].Kongctl.Protected == nil && protectedDefault != nil {
			rs.APIs[i].Kongctl.Protected = protectedDefault
		}
		// Ensure protected has a value (false if still nil)
		if rs.APIs[i].Kongctl.Protected == nil {
			falseVal := false
			rs.APIs[i].Kongctl.Protected = &falseVal
		}
	}
	
	// Apply defaults to ApplicationAuthStrategies (parent resources)
	for i := range rs.ApplicationAuthStrategies {
		if rs.ApplicationAuthStrategies[i].Kongctl == nil {
			rs.ApplicationAuthStrategies[i].Kongctl = &resources.KongctlMeta{}
		}
		// Validate that explicit namespace is not empty
		if rs.ApplicationAuthStrategies[i].Kongctl.Namespace != nil && 
			*rs.ApplicationAuthStrategies[i].Kongctl.Namespace == "" {
			return fmt.Errorf("application_auth_strategy '%s' cannot have an empty namespace", 
				rs.ApplicationAuthStrategies[i].Ref)
		}
		// Apply namespace default if not set
		if rs.ApplicationAuthStrategies[i].Kongctl.Namespace == nil {
			rs.ApplicationAuthStrategies[i].Kongctl.Namespace = namespaceDefault
		}
		// Apply protected default if not set
		if rs.ApplicationAuthStrategies[i].Kongctl.Protected == nil && protectedDefault != nil {
			rs.ApplicationAuthStrategies[i].Kongctl.Protected = protectedDefault
		}
		// Ensure protected has a value (false if still nil)
		if rs.ApplicationAuthStrategies[i].Kongctl.Protected == nil {
			falseVal := false
			rs.ApplicationAuthStrategies[i].Kongctl.Protected = &falseVal
		}
	}
	
	// Apply defaults to ControlPlanes (parent resources)
	for i := range rs.ControlPlanes {
		if rs.ControlPlanes[i].Kongctl == nil {
			rs.ControlPlanes[i].Kongctl = &resources.KongctlMeta{}
		}
		// Validate that explicit namespace is not empty
		if rs.ControlPlanes[i].Kongctl.Namespace != nil && *rs.ControlPlanes[i].Kongctl.Namespace == "" {
			return fmt.Errorf("control_plane '%s' cannot have an empty namespace", rs.ControlPlanes[i].Ref)
		}
		// Apply namespace default if not set
		if rs.ControlPlanes[i].Kongctl.Namespace == nil {
			rs.ControlPlanes[i].Kongctl.Namespace = namespaceDefault
		}
		// Apply protected default if not set
		if rs.ControlPlanes[i].Kongctl.Protected == nil && protectedDefault != nil {
			rs.ControlPlanes[i].Kongctl.Protected = protectedDefault
		}
		// Ensure protected has a value (false if still nil)
		if rs.ControlPlanes[i].Kongctl.Protected == nil {
			falseVal := false
			rs.ControlPlanes[i].Kongctl.Protected = &falseVal
		}
	}
	
	// Note: Child resources (API versions, publications, etc.) do not get kongctl metadata
	// as Konnect doesn't support labels on child resources
	return nil
}

// applyDefaults applies SDK default values to all resources in the set
func (l *Loader) applyDefaults(rs *resources.ResourceSet) {
	// Apply SDK defaults to portals
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
		for j := range api.Documents {
			api.Documents[j].SetDefaults()
		}
	}

	// Apply defaults to root-level API child resources
	for i := range rs.APIVersions {
		rs.APIVersions[i].SetDefaults()
	}
	for i := range rs.APIPublications {
		rs.APIPublications[i].SetDefaults()
	}
	for i := range rs.APIImplementations {
		rs.APIImplementations[i].SetDefaults()
	}
	for i := range rs.APIDocuments {
		rs.APIDocuments[i].SetDefaults()
	}
	
	// Apply defaults to portal child resources
	for i := range rs.PortalCustomizations {
		rs.PortalCustomizations[i].SetDefaults()
	}
	for i := range rs.PortalCustomDomains {
		rs.PortalCustomDomains[i].SetDefaults()
	}
	for i := range rs.PortalPages {
		rs.PortalPages[i].SetDefaults()
	}
	for i := range rs.PortalSnippets {
		rs.PortalSnippets[i].SetDefaults()
	}
}

// extractPortalPages recursively extracts and flattens nested portal pages
func (l *Loader) extractPortalPages(
	allPages *[]resources.PortalPageResource, 
	page resources.PortalPageResource, 
	portalRef string, 
	parentPageRef string,
) {
	// Set portal and parent references
	page.Portal = portalRef
	page.ParentPageRef = parentPageRef
	
	// Process children before clearing them
	children := page.Children
	page.Children = nil // Clear children from the page before appending
	
	// Append current page
	*allPages = append(*allPages, page)
	
	// Recursively process children
	for _, child := range children {
		l.extractPortalPages(allPages, child, portalRef, page.Ref)
	}
}

// extractNestedResources extracts nested child resources to root level with parent references
func (l *Loader) extractNestedResources(rs *resources.ResourceSet) {
	// Extract nested API child resources
	for i := range rs.APIs {
		api := &rs.APIs[i]
		
		// Extract versions
		for j := range api.Versions {
			version := api.Versions[j]
			version.API = api.Ref // Set parent reference
			rs.APIVersions = append(rs.APIVersions, version)
		}
		
		// Extract publications
		for j := range api.Publications {
			publication := api.Publications[j]
			publication.API = api.Ref // Set parent reference
			rs.APIPublications = append(rs.APIPublications, publication)
		}
		
		// Extract implementations
		for j := range api.Implementations {
			implementation := api.Implementations[j]
			implementation.API = api.Ref // Set parent reference
			rs.APIImplementations = append(rs.APIImplementations, implementation)
		}
		
		// Clear nested resources from API
		api.Versions = nil
		api.Publications = nil
		api.Implementations = nil
	}

	// Extract nested Portal child resources
	for i := range rs.Portals {
		portal := &rs.Portals[i]
		
		// Extract customization (single resource)
		if portal.Customization != nil {
			customization := *portal.Customization
			customization.Portal = portal.Ref // Set parent reference
			rs.PortalCustomizations = append(rs.PortalCustomizations, customization)
		}
		
		// Extract custom domain (single resource)
		if portal.CustomDomain != nil {
			customDomain := *portal.CustomDomain
			customDomain.Portal = portal.Ref // Set parent reference
			
			rs.PortalCustomDomains = append(rs.PortalCustomDomains, customDomain)
		}
		
		// Extract pages (with recursive flattening)
		for j := range portal.Pages {
			page := portal.Pages[j]
			page.Portal = portal.Ref // Set parent reference
			l.extractPortalPages(&rs.PortalPages, page, portal.Ref, "")
		}
		
		// Extract snippets
		for j := range portal.Snippets {
			snippet := portal.Snippets[j]
			snippet.Portal = portal.Ref // Set parent reference
			rs.PortalSnippets = append(rs.PortalSnippets, snippet)
		}
		
		// Clear nested resources from Portal
		portal.Customization = nil
		portal.CustomDomain = nil
		portal.Pages = nil
		portal.Snippets = nil
	}
}

// suggestFieldName suggests a correct field name for a misspelled field
func (l *Loader) suggestFieldName(fieldName string) string {
	// Common field names that users might misspell
	knownFields := map[string][]string{
		// Portal fields
		"labels":       {"lables", "label", "labeles", "lablels"},
		"name":         {"nam", "nme", "name"},
		"description":  {"desc", "description", "descriptin", "descrption"},
		"ref":          {"reference", "id", "key"},
		"kongctl":      {"kong_ctl", "kong-ctl", "kongcontrol"},
		"namespace":    {"namspace", "namesapce", "ns"},
		"protected":    {"protect", "potected", "proteced"},
		"is_public":    {"public", "ispublic", "is-public"},
		"custom_domain": {"domain", "customdomain", "custom-domain"},
		"customization": {"customize", "custom", "theme"},
		"pages":        {"page", "content"},
		"snippets":     {"snippet", "code"},
		
		// API fields
		"versions":     {"version", "api_versions", "api-versions"},
		"publications": {"publication", "publish", "published"},
		"implementations": {"implementation", "impl", "service"},
		
		// Auth strategy fields
		"strategy_type": {"type", "auth_type", "strategy-type", "strategytype"},
		"configs":      {"config", "configuration", "settings"},
		"display_name": {"displayname", "display-name", "title"},
		
		// Common across resources
		"created_at":   {"created", "createdat", "created-at"},
		"updated_at":   {"updated", "updatedat", "updated-at"},
	}
	
	fieldLower := strings.ToLower(fieldName)
	
	// Check if the misspelled field matches any known misspellings
	for correct, misspellings := range knownFields {
		for _, misspelling := range misspellings {
			if fieldLower == misspelling {
				return correct
			}
		}
	}
	
	// Simple Levenshtein distance check for close matches
	// This is a simplified version - just check if it's very close
	for correct := range knownFields {
		if levenshteinClose(fieldLower, correct) {
			return correct
		}
	}
	
	return ""
}

// levenshteinClose checks if two strings are close enough (simple heuristic)
func levenshteinClose(s1, s2 string) bool {
	// Very simple heuristic: if lengths differ by more than 2, not close
	if abs(len(s1)-len(s2)) > 2 {
		return false
	}
	
	// Check if one is substring of the other
	if strings.Contains(s1, s2) || strings.Contains(s2, s1) {
		return true
	}
	
	// Check if they share most characters
	matches := 0
	for i := 0; i < len(s1) && i < len(s2); i++ {
		if s1[i] == s2[i] {
			matches++
		}
	}
	
	// If more than 70% characters match in order, consider it close
	minLen := len(s1)
	if len(s2) < minLen {
		minLen = len(s2)
	}
	return float64(matches)/float64(minLen) > 0.7
}

// abs returns absolute value of an integer
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}