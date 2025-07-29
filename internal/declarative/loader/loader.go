package loader

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
	
	// Track refs and names for duplicate detection across all sources
	portalRefs := make(map[string]string)      // ref -> source path
	portalNames := make(map[string]string)     // name -> source path
	authStratRefs := make(map[string]string)   // ref -> source path
	authStratNames := make(map[string]string)  // name -> source path
	cpRefs := make(map[string]string)          // ref -> source path
	cpNames := make(map[string]string)         // name -> source path
	apiRefs := make(map[string]string)         // ref -> source path
	apiNames := make(map[string]string)        // name -> source path
	apiVersionRefs := make(map[string]string)  // ref -> source path
	apiPubRefs := make(map[string]string)      // ref -> source path
	apiImplRefs := make(map[string]string)     // ref -> source path
	apiDocRefs := make(map[string]string)      // ref -> source path
	portalCustomizationRefs := make(map[string]string) // ref -> source path
	portalCustomDomainRefs := make(map[string]string)  // ref -> source path
	portalPageRefs := make(map[string]string)          // ref -> source path
	portalSnippetRefs := make(map[string]string)       // ref -> source path
	
	for _, source := range sources {
		var rs *resources.ResourceSet
		var err error
		
		switch source.Type {
		case SourceTypeFile:
			rs, err = l.loadSingleFile(source.Path)
		case SourceTypeDirectory:
			rs, err = l.loadDirectorySource(source.Path, recursive, 
				portalRefs, portalNames, authStratRefs, authStratNames,
				cpRefs, cpNames, apiRefs, apiNames,
				apiVersionRefs, apiPubRefs, apiImplRefs, apiDocRefs,
				portalCustomizationRefs, portalCustomDomainRefs, portalPageRefs, portalSnippetRefs)
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
				cpRefs, cpNames, apiRefs, apiNames,
				apiVersionRefs, apiPubRefs, apiImplRefs, apiDocRefs,
				portalCustomizationRefs, portalCustomDomainRefs, portalPageRefs, portalSnippetRefs); err != nil {
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

	// Apply SDK defaults to all resources
	l.applyDefaults(&rs)

	// Extract nested child resources to root level
	l.extractNestedResources(&rs)

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
	cpRefs, cpNames, apiRefs, apiNames,
	apiVersionRefs, apiPubRefs, apiImplRefs, apiDocRefs,
	portalCustomizationRefs, portalCustomDomainRefs, 
	portalPageRefs, portalSnippetRefs map[string]string,
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
					portalRefs, portalNames, authStratRefs, authStratNames,
					cpRefs, cpNames, apiRefs, apiNames,
					apiVersionRefs, apiPubRefs, apiImplRefs, apiDocRefs,
					portalCustomizationRefs, portalCustomDomainRefs, portalPageRefs, portalSnippetRefs)
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
			authName := authStrat.GetMoniker()
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
		
		// Check duplicates for API child resources
		for _, version := range rs.APIVersions {
			if existingPath, exists := apiVersionRefs[version.Ref]; exists {
				return nil, fmt.Errorf("duplicate api_version ref '%s' found in %s (already defined in %s)", 
					version.Ref, path, existingPath)
			}
			apiVersionRefs[version.Ref] = path
			allResources.APIVersions = append(allResources.APIVersions, version)
		}
		
		for _, pub := range rs.APIPublications {
			if existingPath, exists := apiPubRefs[pub.Ref]; exists {
				return nil, fmt.Errorf("duplicate api_publication ref '%s' found in %s (already defined in %s)", 
					pub.Ref, path, existingPath)
			}
			apiPubRefs[pub.Ref] = path
			allResources.APIPublications = append(allResources.APIPublications, pub)
		}
		
		for _, impl := range rs.APIImplementations {
			if existingPath, exists := apiImplRefs[impl.Ref]; exists {
				return nil, fmt.Errorf("duplicate api_implementation ref '%s' found in %s (already defined in %s)", 
					impl.Ref, path, existingPath)
			}
			apiImplRefs[impl.Ref] = path
			allResources.APIImplementations = append(allResources.APIImplementations, impl)
		}
		
		for _, doc := range rs.APIDocuments {
			if existingPath, exists := apiDocRefs[doc.Ref]; exists {
				return nil, fmt.Errorf("duplicate api_document ref '%s' found in %s (already defined in %s)", 
					doc.Ref, path, existingPath)
			}
			apiDocRefs[doc.Ref] = path
			allResources.APIDocuments = append(allResources.APIDocuments, doc)
		}
		
		// Check duplicates for Portal child resources
		for _, customization := range rs.PortalCustomizations {
			if existingPath, exists := portalCustomizationRefs[customization.Ref]; exists {
				return nil, fmt.Errorf("duplicate portal_customization ref '%s' found in %s (already defined in %s)", 
					customization.Ref, path, existingPath)
			}
			portalCustomizationRefs[customization.Ref] = path
			allResources.PortalCustomizations = append(allResources.PortalCustomizations, customization)
		}
		
		for _, domain := range rs.PortalCustomDomains {
			if existingPath, exists := portalCustomDomainRefs[domain.Ref]; exists {
				return nil, fmt.Errorf("duplicate portal_custom_domain ref '%s' found in %s (already defined in %s)", 
					domain.Ref, path, existingPath)
			}
			portalCustomDomainRefs[domain.Ref] = path
			allResources.PortalCustomDomains = append(allResources.PortalCustomDomains, domain)
		}
		
		for _, page := range rs.PortalPages {
			if existingPath, exists := portalPageRefs[page.Ref]; exists {
				return nil, fmt.Errorf("duplicate portal_page ref '%s' found in %s (already defined in %s)", 
					page.Ref, path, existingPath)
			}
			portalPageRefs[page.Ref] = path
			allResources.PortalPages = append(allResources.PortalPages, page)
		}
		
		for _, snippet := range rs.PortalSnippets {
			if existingPath, exists := portalSnippetRefs[snippet.Ref]; exists {
				return nil, fmt.Errorf("duplicate portal_snippet ref '%s' found in %s (already defined in %s)", 
					snippet.Ref, path, existingPath)
			}
			portalSnippetRefs[snippet.Ref] = path
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
	portalRefs, portalNames, authStratRefs, authStratNames,
	cpRefs, cpNames, apiRefs, apiNames,
	apiVersionRefs, apiPubRefs, apiImplRefs, apiDocRefs,
	portalCustomizationRefs, portalCustomDomainRefs, portalPageRefs, portalSnippetRefs map[string]string) error {
	
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
		authName := authStrat.GetMoniker()
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
	
	// Merge API child resources
	for _, version := range source.APIVersions {
		if existingPath, exists := apiVersionRefs[version.Ref]; exists {
			return fmt.Errorf("duplicate api_version ref '%s' found in %s (already defined in %s)", 
				version.Ref, sourcePath, existingPath)
		}
		apiVersionRefs[version.Ref] = sourcePath
		target.APIVersions = append(target.APIVersions, version)
	}
	
	for _, pub := range source.APIPublications {
		if existingPath, exists := apiPubRefs[pub.Ref]; exists {
			return fmt.Errorf("duplicate api_publication ref '%s' found in %s (already defined in %s)", 
				pub.Ref, sourcePath, existingPath)
		}
		apiPubRefs[pub.Ref] = sourcePath
		target.APIPublications = append(target.APIPublications, pub)
	}
	
	for _, impl := range source.APIImplementations {
		if existingPath, exists := apiImplRefs[impl.Ref]; exists {
			return fmt.Errorf("duplicate api_implementation ref '%s' found in %s (already defined in %s)", 
				impl.Ref, sourcePath, existingPath)
		}
		apiImplRefs[impl.Ref] = sourcePath
		target.APIImplementations = append(target.APIImplementations, impl)
	}
	
	for _, doc := range source.APIDocuments {
		if existingPath, exists := apiDocRefs[doc.Ref]; exists {
			return fmt.Errorf("duplicate api_document ref '%s' found in %s (already defined in %s)", 
				doc.Ref, sourcePath, existingPath)
		}
		apiDocRefs[doc.Ref] = sourcePath
		target.APIDocuments = append(target.APIDocuments, doc)
	}
	
	// Merge Portal child resources
	for _, customization := range source.PortalCustomizations {
		if existingPath, exists := portalCustomizationRefs[customization.Ref]; exists {
			return fmt.Errorf("duplicate portal_customization ref '%s' found in %s (already defined in %s)", 
				customization.Ref, sourcePath, existingPath)
		}
		portalCustomizationRefs[customization.Ref] = sourcePath
		target.PortalCustomizations = append(target.PortalCustomizations, customization)
	}
	
	for _, domain := range source.PortalCustomDomains {
		if existingPath, exists := portalCustomDomainRefs[domain.Ref]; exists {
			return fmt.Errorf("duplicate portal_custom_domain ref '%s' found in %s (already defined in %s)", 
				domain.Ref, sourcePath, existingPath)
		}
		portalCustomDomainRefs[domain.Ref] = sourcePath
		target.PortalCustomDomains = append(target.PortalCustomDomains, domain)
	}
	
	for _, page := range source.PortalPages {
		if existingPath, exists := portalPageRefs[page.Ref]; exists {
			return fmt.Errorf("duplicate portal_page ref '%s' found in %s (already defined in %s)", 
				page.Ref, sourcePath, existingPath)
		}
		portalPageRefs[page.Ref] = sourcePath
		target.PortalPages = append(target.PortalPages, page)
	}
	
	for _, snippet := range source.PortalSnippets {
		if existingPath, exists := portalSnippetRefs[snippet.Ref]; exists {
			return fmt.Errorf("duplicate portal_snippet ref '%s' found in %s (already defined in %s)", 
				snippet.Ref, sourcePath, existingPath)
		}
		portalSnippetRefs[snippet.Ref] = sourcePath
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