package loader

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	decerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/util"
	"sigs.k8s.io/yaml"
)

// temporaryParseResult holds the raw parsed YAML including defaults
// This is used internally during parsing to capture both resources and file-level defaults
type temporaryParseResult struct {
	Defaults              *resources.FileDefaults `json:"_defaults,omitempty" yaml:"_defaults,omitempty"`
	resources.ResourceSet ` yaml:",inline"`
}

// Loader handles loading declarative configuration from files
type Loader struct {
	// baseDir is the base directory for resolving relative file paths in tags
	baseDir string
	// tagRootDir is the boundary directory for !file tag resolution
	tagRootDir string
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
func NewWithPath(path string) *Loader {
	return NewWithBaseDir(path)
}

// NewWithBaseDir creates a new configuration loader with a base directory for !file boundaries
func NewWithBaseDir(baseDir string) *Loader {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = "."
	}
	return &Loader{
		baseDir:    baseDir,
		tagRootDir: baseDir,
	}
}

// getTagRegistry returns the tag registry, creating it if needed
func (l *Loader) getTagRegistry() *tags.ResolverRegistry {
	if l.tagRegistry == nil {
		l.tagRegistry = tags.NewResolverRegistry()
	}
	return l.tagRegistry
}

func (l *Loader) resolveSourceRoot(source Source) string {
	if strings.TrimSpace(l.tagRootDir) != "" {
		return l.tagRootDir
	}

	switch source.Type {
	case SourceTypeFile:
		return filepath.Dir(source.Path)
	case SourceTypeDirectory:
		return source.Path
	case SourceTypeSTDIN:
		return l.baseDir
	default:
		return ""
	}
}

// LoadFromSources loads configuration from multiple sources
func (l *Loader) LoadFromSources(sources []Source, recursive bool) (*resources.ResourceSet, error) {
	return l.LoadFromSourcesWithContext(context.Background(), sources, recursive)
}

// LoadFromSourcesWithContext loads configuration from multiple sources with context support
func (l *Loader) LoadFromSourcesWithContext(ctx context.Context, sources []Source,
	recursive bool,
) (*resources.ResourceSet, error) {
	var allResources resources.ResourceSet
	// Running index of refs for O(1) duplicate checking across files
	refIndex := make(map[string]resources.ResourceType)

	for _, source := range sources {
		var err error
		rootDir := l.resolveSourceRoot(source)

		switch source.Type {
		case SourceTypeFile:
			err = l.loadSingleFileWithContext(ctx, source.Path, rootDir, &allResources, refIndex)
		case SourceTypeDirectory:
			err = l.loadDirectorySourceWithContext(ctx, source.Path, rootDir, recursive, &allResources, refIndex)
		case SourceTypeSTDIN:
			err = l.loadSTDINWithContext(ctx, rootDir, &allResources, refIndex)
		default:
			return nil, decerrors.FormatConfigurationError(
				source.Path,
				0,
				fmt.Sprintf("unknown source type: %v", source.Type),
			)
		}

		if err != nil {
			return nil, err
		}
	}

	// Apply SDK defaults to merged resources
	// Note: Only namespace defaults are applied per-file in parseYAML
	l.applyDefaults(&allResources)

	// Reference resolution must happen after all files are loaded but before validation.
	// This order is critical for cross-file references to work correctly.
	if err := ResolveReferences(ctx, &allResources); err != nil {
		return nil, fmt.Errorf("resolving references: %w", err)
	}

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
	var rs resources.ResourceSet
	refIndex := make(map[string]resources.ResourceType)
	if err := l.loadSingleFile(path, filepath.Dir(path), &rs, refIndex); err != nil {
		return nil, err
	}

	// Apply defaults
	l.applyDefaults(&rs)

	// Validate for backward compatibility
	if err := l.validateResourceSet(&rs); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	return &rs, nil
}

// loadSingleFile loads configuration from a single YAML file
func (l *Loader) loadSingleFile(
	path string,
	rootDir string,
	accumulated *resources.ResourceSet,
	refIndex map[string]resources.ResourceType,
) error {
	// Validate YAML extension
	if !ValidateYAMLFile(path) {
		return fmt.Errorf("file %s does not have .yaml or .yml extension", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer file.Close()

	rs, err := l.parseYAML(file, path, rootDir)
	if err != nil {
		return err
	}

	// Append resources with duplicate checking
	return l.appendResourcesWithDuplicateCheck(accumulated, rs, path, refIndex)
}

// parseYAML parses YAML content into ResourceSet
func (l *Loader) parseYAML(r io.Reader, sourcePath string, rootDir string) (*resources.ResourceSet, error) {
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

	tagRootDir := strings.TrimSpace(l.tagRootDir)
	if tagRootDir == "" {
		tagRootDir = strings.TrimSpace(rootDir)
	}

	// Always register/update resolvers with correct base directory
	// This ensures each file gets the correct base directory for relative paths
	registry.Register(tags.NewFileTagResolver(baseDir, tagRootDir))
	registry.Register(tags.NewRefTagResolver(baseDir))

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

	// Extract nested child resources to root level first
	l.extractNestedResources(&rs)
	// Resolve deck config paths relative to the source file.
	if err := l.resolveDeckConfigPaths(&rs, baseDir, tagRootDir); err != nil {
		return nil, fmt.Errorf("failed to resolve deck config paths in %s: %w", sourcePath, err)
	}

	// Note: We don't validate here when called from loadDirectory
	// because cross-references might be in other files.
	// loadDirectory will validate the merged result.

	return &rs, nil
}

// loadSTDIN loads configuration from stdin
func (l *Loader) loadSTDIN(
	rootDir string,
	accumulated *resources.ResourceSet,
	refIndex map[string]resources.ResourceType,
) error {
	// Check if stdin has data
	stat, err := os.Stdin.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat stdin: %w", err)
	}

	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return fmt.Errorf("no data provided on stdin")
	}

	rs, err := l.parseYAML(os.Stdin, "stdin", rootDir)
	if err != nil {
		return err
	}

	// Append resources with duplicate checking
	return l.appendResourcesWithDuplicateCheck(accumulated, rs, "stdin", refIndex)
}

// loadDirectorySource loads YAML files from a directory
func (l *Loader) loadDirectorySource(
	dirPath string,
	rootDir string,
	recursive bool,
	accumulated *resources.ResourceSet,
	refIndex map[string]resources.ResourceType,
) error {
	yamlCount := 0
	subdirCount := 0

	// First, check direct YAML files in the directory
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	for _, entry := range entries {
		path := filepath.Join(dirPath, entry.Name())

		if entry.IsDir() {
			subdirCount++
			if recursive {
				// Recursively load subdirectory
				if err := l.loadDirectorySource(path, rootDir, recursive, accumulated, refIndex); err != nil {
					return err
				}
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
			return fmt.Errorf("failed to open %s: %w", path, err)
		}

		rs, err := l.parseYAML(file, path, rootDir)
		file.Close()
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}

		// Append resources with duplicate checking
		if err := l.appendResourcesWithDuplicateCheck(accumulated, rs, path, refIndex); err != nil {
			return err
		}

	}

	// Provide helpful error if no YAML files found
	if yamlCount == 0 && subdirCount > 0 && !recursive {
		return fmt.Errorf("no YAML files found in directory '%s'. Found %d subdirectories. "+
			"Use -R to search subdirectories", dirPath, subdirCount)
	} else if yamlCount == 0 {
		// Check if accumulated has any resources using the registry
		if accumulated.IsEmpty() {
			// Only error if no files were found at all (not just empty files)
			return fmt.Errorf("no YAML files found in directory '%s'", dirPath)
		}
	}

	return nil
}

// appendResourcesWithDuplicateCheck appends resources from source to accumulated with global duplicate checking.
// The refIndex is a running index maintained across all files for O(1) duplicate lookups.
func (l *Loader) appendResourcesWithDuplicateCheck(
	accumulated, source *resources.ResourceSet,
	sourcePath string,
	refIndex map[string]resources.ResourceType,
) error {
	// Check for duplicate refs
	// We need to check both:
	// 1. Duplicates within the source file itself
	// 2. Duplicates between source and accumulated (using refIndex)
	seenRefs := make(map[string]resources.ResourceType, source.ResourceCount())
	var duplicateErr error

	source.ForEachResource(func(r resources.Resource) bool {
		ref := r.GetRef()
		resourceType := r.GetType()

		// Check for duplicate within the same source file
		if existingType, exists := seenRefs[ref]; exists {
			duplicateErr = fmt.Errorf("duplicate ref '%s' found in %s (already defined as %s)",
				ref, sourcePath, existingType)
			return false
		}
		seenRefs[ref] = resourceType

		// Check for duplicate against accumulated resources - O(1) lookup using running index
		if existingType, exists := refIndex[ref]; exists {
			duplicateErr = fmt.Errorf("duplicate ref '%s' found in %s (already defined as %s)",
				ref, sourcePath, existingType)
			return false
		}
		return true
	})

	if duplicateErr != nil {
		return duplicateErr
	}

	// Append all resources from source to accumulated using the registry
	accumulated.AppendAll(source)

	// Update the running index with newly added refs
	for ref, resourceType := range seenRefs {
		refIndex[ref] = resourceType
	}

	// If this source defines a namespace default without parent resources,
	// propagate it so sync mode can inspect the correct namespace.
	if source.DefaultNamespace != "" {
		parentCount := len(source.Portals) +
			len(source.ApplicationAuthStrategies) +
			len(source.ControlPlanes) +
			len(source.APIs) +
			len(source.OrganizationTeams)

		if parentCount == 0 {
			accumulated.AddDefaultNamespace(source.DefaultNamespace)
		}
	}

	return nil
}

// applyNamespaceDefaults applies file-level namespace and protected defaults to parent resources
func (l *Loader) applyNamespaceDefaults(rs *resources.ResourceSet, fileDefaults *resources.FileDefaults) error {
	// Determine the effective namespace default
	defaultNamespace := "default"
	namespaceDefault := &defaultNamespace
	var protectedDefault *bool
	hasFileNamespaceDefault := false

	if fileDefaults != nil && fileDefaults.Kongctl != nil {
		// Validate that namespace default is not empty
		if fileDefaults.Kongctl.Namespace != nil && *fileDefaults.Kongctl.Namespace == "" {
			return fmt.Errorf("namespace in _defaults.kongctl cannot be empty")
		}
		if fileDefaults.Kongctl.Namespace != nil {
			namespaceDefault = fileDefaults.Kongctl.Namespace
			rs.AddDefaultNamespace(*namespaceDefault)
			hasFileNamespaceDefault = true
		}
		protectedDefault = fileDefaults.Kongctl.Protected
	}

	assignNamespace := func(meta **resources.KongctlMeta, resourceType, resourceRef string) error {
		if *meta == nil {
			*meta = &resources.KongctlMeta{}
		}
		m := *meta

		// Validate that explicit namespace is not empty
		if m.Namespace != nil && *m.Namespace == "" {
			return fmt.Errorf("%s '%s' cannot have an empty namespace", resourceType, resourceRef)
		}

		if m.Namespace != nil {
			m.NamespaceOrigin = resources.NamespaceOriginExplicit
			return nil
		}

		m.Namespace = namespaceDefault
		if hasFileNamespaceDefault {
			m.NamespaceOrigin = resources.NamespaceOriginFileDefault
		} else {
			m.NamespaceOrigin = resources.NamespaceOriginImplicitDefault
		}

		return nil
	}

	// Apply defaults to portals (parent resources)
	for i := range rs.Portals {
		if rs.Portals[i].IsExternal() {
			if rs.Portals[i].Kongctl != nil {
				return fmt.Errorf(
					"portal '%s' is marked as external and cannot use kongctl metadata",
					rs.Portals[i].Ref,
				)
			}
			continue
		}
		if err := assignNamespace(&rs.Portals[i].Kongctl, "portal", rs.Portals[i].Ref); err != nil {
			return err
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
		if err := assignNamespace(&rs.APIs[i].Kongctl, "api", rs.APIs[i].Ref); err != nil {
			return err
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

	// Apply defaults to CatalogServices (parent resources)
	for i := range rs.CatalogServices {
		if err := assignNamespace(&rs.CatalogServices[i].Kongctl, "catalog_service", rs.CatalogServices[i].Ref); err != nil {
			return err
		}
		if rs.CatalogServices[i].Kongctl.Protected == nil && protectedDefault != nil {
			rs.CatalogServices[i].Kongctl.Protected = protectedDefault
		}
		if rs.CatalogServices[i].Kongctl.Protected == nil {
			falseVal := false
			rs.CatalogServices[i].Kongctl.Protected = &falseVal
		}
	}

	// Apply defaults to ApplicationAuthStrategies (parent resources)
	for i := range rs.ApplicationAuthStrategies {
		if err := assignNamespace(&rs.ApplicationAuthStrategies[i].Kongctl,
			"application_auth_strategy", rs.ApplicationAuthStrategies[i].Ref); err != nil {
			return err
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
		if rs.ControlPlanes[i].IsExternal() {
			if rs.ControlPlanes[i].Kongctl != nil {
				return fmt.Errorf("control_plane '%s' is marked as external and cannot use kongctl metadata",
					rs.ControlPlanes[i].Ref)
			}
			continue
		}
		if err := assignNamespace(&rs.ControlPlanes[i].Kongctl, "control_plane", rs.ControlPlanes[i].Ref); err != nil {
			return err
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

	// Apply defaults to ControlPlanes (parent resources)
	if util.IsEventGatewayEnabled() {
		for i := range rs.EventGatewayControlPlanes {
			if err := assignNamespace(
				&rs.EventGatewayControlPlanes[i].Kongctl,
				"control_plane",
				rs.EventGatewayControlPlanes[i].Ref,
			); err != nil {
				return err
			}
			// Apply protected default if not set
			if rs.EventGatewayControlPlanes[i].Kongctl.Protected == nil && protectedDefault != nil {
				rs.EventGatewayControlPlanes[i].Kongctl.Protected = protectedDefault
			}
			// Ensure protected has a value (false if still nil)
			if rs.EventGatewayControlPlanes[i].Kongctl.Protected == nil {
				falseVal := false
				rs.EventGatewayControlPlanes[i].Kongctl.Protected = &falseVal
			}
		}
	}

	// Apply namespace defaults to teams
	for i := range rs.OrganizationTeams {
		if rs.OrganizationTeams[i].IsExternal() {
			if rs.OrganizationTeams[i].Kongctl != nil {
				return fmt.Errorf(
					"team '%s' is marked as external and cannot use kongctl metadata",
					rs.OrganizationTeams[i].Ref,
				)
			}
			continue
		}
		if err := assignNamespace(&rs.OrganizationTeams[i].Kongctl, "team", rs.OrganizationTeams[i].Ref); err != nil {
			return err
		}
		// Apply protected default if not set
		if rs.OrganizationTeams[i].Kongctl.Protected == nil && protectedDefault != nil {
			rs.OrganizationTeams[i].Kongctl.Protected = protectedDefault
		}
		// Ensure protected has a value (false if still nil)
		if rs.OrganizationTeams[i].Kongctl.Protected == nil {
			falseVal := false
			rs.OrganizationTeams[i].Kongctl.Protected = &falseVal
		}
	}

	// Note: Child resources (API versions, publications, etc.) do not get kongctl metadata
	// as Konnect doesn't support labels on child resources
	return nil
}

// applyDefaults applies SDK default values to all resources in the set.
// Uses the registry to iterate all registered resource types.
func (l *Loader) applyDefaults(rs *resources.ResourceSet) {
	// Apply defaults to all registered resources via registry (zero allocation)
	rs.ForEachResource(func(r resources.Resource) bool {
		r.SetDefaults()
		return true
	})

	// Special case: API Documents are flattened but kept nested under APIs
	// (extractNestedResources reassigns them rather than moving to rs.APIDocuments)
	for i := range rs.APIs {
		for j := range rs.APIs[i].Documents {
			rs.APIs[i].Documents[j].SetDefaults()
		}
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
	// Extract organization nested resources
	if rs.Organization != nil {
		org := rs.Organization
		// Extract organization teams from organization
		rs.OrganizationTeams = append(rs.OrganizationTeams, org.Teams...)

		org.Teams = nil
	}

	for i := range rs.ControlPlanes {
		cp := &rs.ControlPlanes[i]

		for j := range cp.GatewayServices {
			service := cp.GatewayServices[j]
			service.ControlPlane = cp.Ref
			rs.GatewayServices = append(rs.GatewayServices, service)
		}

		cp.GatewayServices = nil
	}

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

		// Extract documents (with recursive flattening) and reassign to API
		docs := make([]resources.APIDocumentResource, 0)
		for j := range api.Documents {
			document := api.Documents[j]
			l.extractAPIDocuments(&docs, document, api.Ref, "")
		}
		api.Documents = docs

		// Clear other nested resources from API
		api.Versions = nil
		api.Publications = nil
		api.Implementations = nil
	}

	// Extract root-level API documents (with recursive flattening)
	flattenedDocs := make([]resources.APIDocumentResource, 0)
	for _, document := range rs.APIDocuments {
		l.extractAPIDocuments(&flattenedDocs, document, document.API, "")
	}
	rs.APIDocuments = flattenedDocs

	// Extract nested Portal child resources
	for i := range rs.Portals {
		portal := &rs.Portals[i]

		// Extract customization (single resource)
		if portal.Customization != nil {
			customization := *portal.Customization
			customization.Portal = portal.Ref // Set parent reference
			rs.PortalCustomizations = append(rs.PortalCustomizations, customization)
		}

		// Extract auth settings (single resource)
		if portal.AuthSettings != nil {
			authSettings := *portal.AuthSettings
			authSettings.Portal = portal.Ref
			rs.PortalAuthSettings = append(rs.PortalAuthSettings, authSettings)
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

		// Extract teams
		for j := range portal.Teams {
			team := portal.Teams[j]
			team.Portal = portal.Ref // Set parent reference
			for k := range team.Roles {
				role := team.Roles[k]
				role.Portal = portal.Ref
				role.Team = team.Ref
				rs.PortalTeamRoles = append(rs.PortalTeamRoles, role)
			}
			team.Roles = nil
			rs.PortalTeams = append(rs.PortalTeams, team)
		}

		// Extract email config (singleton)
		if portal.EmailConfig != nil {
			cfg := *portal.EmailConfig
			cfg.Portal = portal.Ref
			rs.PortalEmailConfigs = append(rs.PortalEmailConfigs, cfg)
		}

		// Extract email templates (map keyed by template name)
		for key, tpl := range portal.EmailTemplates {
			if tpl.Name == "" {
				tpl.Name = kkComps.EmailTemplateName(key)
			}
			if tpl.Ref == "" {
				tpl.Ref = key
			}
			tpl.Portal = portal.Ref
			rs.PortalEmailTemplates = append(rs.PortalEmailTemplates, tpl)
		}

		// Clear nested resources from Portal
		portal.Customization = nil
		portal.AuthSettings = nil
		portal.CustomDomain = nil
		portal.Pages = nil
		portal.Snippets = nil
		portal.Teams = nil
		portal.EmailConfig = nil
		portal.EmailTemplates = nil
	}
}

// extractAPIDocuments recursively extracts and flattens nested API documents
func (l *Loader) extractAPIDocuments(
	allDocs *[]resources.APIDocumentResource,
	doc resources.APIDocumentResource,
	apiRef string,
	parentDocRef string,
) {
	if apiRef != "" {
		doc.API = apiRef
	}
	if parentDocRef != "" {
		doc.ParentDocumentRef = parentDocRef
	}

	children := doc.Children
	doc.Children = nil

	*allDocs = append(*allDocs, doc)

	for _, child := range children {
		childAPIRef := apiRef
		if child.API != "" {
			childAPIRef = child.API
		}
		l.extractAPIDocuments(allDocs, child, childAPIRef, doc.Ref)
	}
}

// suggestFieldName suggests a correct field name for a misspelled field
func (l *Loader) suggestFieldName(fieldName string) string {
	// Common field names that users might misspell
	knownFields := map[string][]string{
		// Portal fields
		"labels":        {"lables", "label", "labeles", "lablels"},
		"name":          {"nam", "nme", "name"},
		"description":   {"desc", "description", "descriptin", "descrption"},
		"ref":           {"reference", "id", "key"},
		"kongctl":       {"kong_ctl", "kong-ctl", "kongcontrol"},
		"namespace":     {"namspace", "namesapce", "ns"},
		"protected":     {"protect", "potected", "proteced"},
		"is_public":     {"public", "ispublic", "is-public"},
		"custom_domain": {"domain", "customdomain", "custom-domain"},
		"customization": {"customize", "custom", "theme"},
		"pages":         {"page", "content"},
		"snippets":      {"snippet", "code"},
		"email_config":  {"email_configs", "email-config", "email"},

		// API fields
		"versions":        {"version", "api_versions", "api-versions"},
		"publications":    {"publication", "publish", "published"},
		"implementations": {"implementation", "impl", "service"},

		// Auth strategy fields
		"strategy_type": {"type", "auth_type", "strategy-type", "strategytype"},
		"configs":       {"config", "configuration", "settings"},
		"display_name":  {"displayname", "display-name", "title"},

		// Common across resources
		"created_at": {"created", "createdat", "created-at"},
		"updated_at": {"updated", "updatedat", "updated-at"},
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

// Context-aware wrapper methods for internal use
func (l *Loader) loadSingleFileWithContext(
	_ context.Context,
	path string,
	rootDir string,
	accumulated *resources.ResourceSet,
	refIndex map[string]resources.ResourceType,
) error {
	// For now, we just call the non-context version
	// The context will be used by ResolveReferences in LoadFromSourcesWithContext
	return l.loadSingleFile(path, rootDir, accumulated, refIndex)
}

func (l *Loader) loadDirectorySourceWithContext(
	_ context.Context,
	dirPath string,
	rootDir string,
	recursive bool,
	accumulated *resources.ResourceSet,
	refIndex map[string]resources.ResourceType,
) error {
	// For now, we just call the non-context version
	// The context will be used by ResolveReferences in LoadFromSourcesWithContext
	return l.loadDirectorySource(dirPath, rootDir, recursive, accumulated, refIndex)
}

func (l *Loader) loadSTDINWithContext(
	_ context.Context,
	rootDir string,
	accumulated *resources.ResourceSet,
	refIndex map[string]resources.ResourceType,
) error {
	// For now, we just call the non-context version
	// The context will be used by ResolveReferences in LoadFromSourcesWithContext
	return l.loadSTDIN(rootDir, accumulated, refIndex)
}
