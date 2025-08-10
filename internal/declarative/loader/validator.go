package loader

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/validator"
)

// validateResourceSet validates all resources and checks for ref uniqueness
func (l *Loader) validateResourceSet(rs *resources.ResourceSet) error {
	// Use global ref registry to enforce uniqueness across ALL resource types
	globalRefRegistry := resources.NewGlobalRefRegistry()
	
	// Build registry of all resources by type for reference validation
	// Note: We still need this for cross-reference validation
	resourceRegistry := make(map[string]map[string]bool)

	// Validate portals
	if err := l.validatePortals(rs.Portals, globalRefRegistry, resourceRegistry); err != nil {
		return err
	}

	// Validate auth strategies
	if err := l.validateAuthStrategies(rs.ApplicationAuthStrategies, globalRefRegistry, resourceRegistry); err != nil {
		return err
	}

	// Validate control planes
	if err := l.validateControlPlanes(rs.ControlPlanes, globalRefRegistry, resourceRegistry); err != nil {
		return err
	}

	// Validate APIs and their children
	if err := l.validateAPIs(rs.APIs, globalRefRegistry, resourceRegistry); err != nil {
		return err
	}

	// Validate separate API child resources (extracted from nested resources)
	if err := l.validateSeparateAPIChildResources(rs, globalRefRegistry, resourceRegistry); err != nil {
		return err
	}

	// Validate cross-resource references
	if err := l.validateCrossReferences(rs, resourceRegistry); err != nil {
		return err
	}

	// Validate namespaces
	if err := l.validateNamespaces(rs); err != nil {
		return err
	}

	return nil
}

// validatePortals validates portal resources
func (l *Loader) validatePortals(
	portals []resources.PortalResource,
	globalRefRegistry *resources.GlobalRefRegistry,
	registry map[string]map[string]bool,
) error {
	refs := make(map[string]bool)
	names := make(map[string]string) // name -> ref mapping
	registry["portal"] = refs

	for i := range portals {
		portal := &portals[i]

		// Validate resource
		if err := portal.Validate(); err != nil {
			return fmt.Errorf("invalid portal %q: %w", portal.GetRef(), err)
		}

		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(portal.GetRef(), "portal"); err != nil {
			return err
		}
		
		// Check name uniqueness (within type)
		if existingRef, exists := names[portal.Name]; exists {
			return fmt.Errorf("duplicate portal name '%s' (ref: %s conflicts with ref: %s)", 
				portal.Name, portal.GetRef(), existingRef)
		}
		
		refs[portal.GetRef()] = true
		names[portal.Name] = portal.GetRef()
	}

	return nil
}

// validateAuthStrategies validates auth strategy resources
func (l *Loader) validateAuthStrategies(
	strategies []resources.ApplicationAuthStrategyResource,
	globalRefRegistry *resources.GlobalRefRegistry,
	registry map[string]map[string]bool,
) error {
	refs := make(map[string]bool)
	names := make(map[string]string) // name -> ref mapping
	registry["application_auth_strategy"] = refs

	for i := range strategies {
		strategy := &strategies[i]

		// Validate resource
		if err := strategy.Validate(); err != nil {
			return fmt.Errorf("invalid application_auth_strategy %q: %w", strategy.GetRef(), err)
		}

		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(strategy.GetRef(), "application_auth_strategy"); err != nil {
			return err
		}
		
		// Check name uniqueness (within type)
		stratName := strategy.GetMoniker()
		if existingRef, exists := names[stratName]; exists {
			return fmt.Errorf("duplicate application_auth_strategy name '%s' (ref: %s conflicts with ref: %s)", 
				stratName, strategy.GetRef(), existingRef)
		}
		
		refs[strategy.GetRef()] = true
		names[stratName] = strategy.GetRef()
	}

	return nil
}

// validateControlPlanes validates control plane resources
func (l *Loader) validateControlPlanes(
	cps []resources.ControlPlaneResource,
	globalRefRegistry *resources.GlobalRefRegistry,
	registry map[string]map[string]bool,
) error {
	refs := make(map[string]bool)
	names := make(map[string]string) // name -> ref mapping
	registry["control_plane"] = refs

	for i := range cps {
		cp := &cps[i]

		// Validate resource
		if err := cp.Validate(); err != nil {
			return fmt.Errorf("invalid control_plane %q: %w", cp.GetRef(), err)
		}

		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(cp.GetRef(), "control_plane"); err != nil {
			return err
		}
		
		// Check name uniqueness
		if existingRef, exists := names[cp.Name]; exists {
			return fmt.Errorf("duplicate control_plane name '%s' (ref: %s conflicts with ref: %s)", 
				cp.Name, cp.GetRef(), existingRef)
		}
		
		refs[cp.GetRef()] = true
		names[cp.Name] = cp.GetRef()
	}

	return nil
}

// validateAPIs validates API resources and their children
func (l *Loader) validateAPIs(
	apis []resources.APIResource,
	globalRefRegistry *resources.GlobalRefRegistry,
	registry map[string]map[string]bool,
) error {
	apiRefs := make(map[string]bool)
	apiNames := make(map[string]string) // name -> ref mapping
	registry["api"] = apiRefs

	// Also create registries for child resources
	versionRefs := make(map[string]bool)
	registry["api_version"] = versionRefs
	
	publicationRefs := make(map[string]bool)
	registry["api_publication"] = publicationRefs
	
	implementationRefs := make(map[string]bool)
	registry["api_implementation"] = implementationRefs

	for i := range apis {
		api := &apis[i]

		// Validate API resource
		if err := api.Validate(); err != nil {
			return fmt.Errorf("invalid api %q: %w", api.GetRef(), err)
		}

		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(api.GetRef(), "api"); err != nil {
			return err
		}
		
		// Check API name uniqueness
		if existingRef, exists := apiNames[api.Name]; exists {
			return fmt.Errorf("duplicate api name '%s' (ref: %s conflicts with ref: %s)", 
				api.Name, api.GetRef(), existingRef)
		}
		
		apiRefs[api.GetRef()] = true
		apiNames[api.Name] = api.GetRef()

		// Validate nested versions
		for j := range api.Versions {
			version := &api.Versions[j]
			if err := version.Validate(); err != nil {
				return fmt.Errorf("invalid api_version %q in api %q: %w", version.GetRef(), api.GetRef(), err)
			}
			if versionRefs[version.GetRef()] {
				return fmt.Errorf("duplicate api_version ref: %s", version.GetRef())
			}
			versionRefs[version.GetRef()] = true
		}

		// Validate Konnect's single-version constraint
		if len(api.Versions) > 1 {
			return fmt.Errorf("api %q defines %d versions, but Konnect currently supports only one version per API. "+
				"Ensure each API versions key has only 1 version defined", api.GetRef(), len(api.Versions))
		}

		// Validate nested publications
		for j := range api.Publications {
			publication := &api.Publications[j]
			if err := publication.Validate(); err != nil {
				return fmt.Errorf("invalid api_publication %q in api %q: %w", publication.GetRef(), api.GetRef(), err)
			}
			if publicationRefs[publication.GetRef()] {
				return fmt.Errorf("duplicate api_publication ref: %s", publication.GetRef())
			}
			publicationRefs[publication.GetRef()] = true
		}

		// Validate nested implementations
		for j := range api.Implementations {
			implementation := &api.Implementations[j]
			if err := implementation.Validate(); err != nil {
				return fmt.Errorf("invalid api_implementation %q in api %q: %w", implementation.GetRef(), api.GetRef(), err)
			}
			if implementationRefs[implementation.GetRef()] {
				return fmt.Errorf("duplicate api_implementation ref: %s", implementation.GetRef())
			}
			implementationRefs[implementation.GetRef()] = true
		}
	}

	return nil
}

// validateCrossReferences validates that all cross-resource references are valid
func (l *Loader) validateCrossReferences(rs *resources.ResourceSet, registry map[string]map[string]bool) error {
	// Validate portal references
	for i := range rs.Portals {
		if err := l.validateResourceReferences(&rs.Portals[i], registry); err != nil {
			return err
		}
	}

	// Validate API child resource references
	for i := range rs.APIs {
		api := &rs.APIs[i]
		// Validate publication references
		for j := range api.Publications {
			if err := l.validateResourceReferences(&api.Publications[j], registry); err != nil {
				return err
			}
		}

		// Validate implementation references
		for j := range api.Implementations {
			if err := l.validateResourceReferences(&api.Implementations[j], registry); err != nil {
				return err
			}
		}
	}

	// Validate separate API child resources (extracted from nested resources)
	for i := range rs.APIPublications {
		if err := l.validateResourceReferences(&rs.APIPublications[i], registry); err != nil {
			return err
		}
	}

	for i := range rs.APIImplementations {
		if err := l.validateResourceReferences(&rs.APIImplementations[i], registry); err != nil {
			return err
		}
	}

	// Note: API versions don't have outbound references, so no validation needed

	return nil
}

// validateSeparateAPIChildResources validates individual API child resources that were extracted
func (l *Loader) validateSeparateAPIChildResources(
	rs *resources.ResourceSet,
	globalRefRegistry *resources.GlobalRefRegistry,
	registry map[string]map[string]bool,
) error {
	// Count versions per API to enforce single-version constraint
	// This is a safety check in case the early validation was bypassed
	versionCountByAPI := make(map[string]int)
	for i := range rs.APIVersions {
		version := &rs.APIVersions[i]
		if version.API != "" {
			versionCountByAPI[version.API]++
		}
	}
	
	// Check if any API has multiple versions
	for apiRef, count := range versionCountByAPI {
		if count > 1 {
			return fmt.Errorf("api %q has %d versions defined, but Konnect currently supports only one version per API. "+
				"Ensure each API versions key has only 1 version defined", apiRef, count)
		}
	}
	
	// Validate separate API versions
	versionRefs := make(map[string]bool)
	registry["api_version"] = versionRefs
	for i := range rs.APIVersions {
		version := &rs.APIVersions[i]
		if err := version.Validate(); err != nil {
			return fmt.Errorf("invalid api_version %q: %w", version.GetRef(), err)
		}
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(version.GetRef(), "api_version"); err != nil {
			return err
		}
		versionRefs[version.GetRef()] = true
	}

	// Validate separate API publications  
	publicationRefs := make(map[string]bool)
	registry["api_publication"] = publicationRefs
	for i := range rs.APIPublications {
		publication := &rs.APIPublications[i]
		if err := publication.Validate(); err != nil {
			return fmt.Errorf("invalid api_publication %q: %w", publication.GetRef(), err)
		}
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(publication.GetRef(), "api_publication"); err != nil {
			return err
		}
		publicationRefs[publication.GetRef()] = true
	}

	// Validate separate API implementations
	implementationRefs := make(map[string]bool)
	registry["api_implementation"] = implementationRefs
	for i := range rs.APIImplementations {
		implementation := &rs.APIImplementations[i]
		if err := implementation.Validate(); err != nil {
			return fmt.Errorf("invalid api_implementation %q: %w", implementation.GetRef(), err)
		}
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(implementation.GetRef(), "api_implementation"); err != nil {
			return err
		}
		implementationRefs[implementation.GetRef()] = true
	}

	// Validate separate API documents
	documentRefs := make(map[string]bool)
	registry["api_document"] = documentRefs
	for i := range rs.APIDocuments {
		document := &rs.APIDocuments[i]
		if err := document.Validate(); err != nil {
			return fmt.Errorf("invalid api_document %q: %w", document.GetRef(), err)
		}
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(document.GetRef(), "api_document"); err != nil {
			return err
		}
		documentRefs[document.GetRef()] = true
	}

	// Validate portal pages
	pageRefs := make(map[string]bool)
	registry["portal_page"] = pageRefs
	for i := range rs.PortalPages {
		page := &rs.PortalPages[i]
		if err := page.Validate(); err != nil {
			return fmt.Errorf("invalid portal_page %q: %w", page.GetRef(), err)
		}
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(page.GetRef(), "portal_page"); err != nil {
			return err
		}
		pageRefs[page.GetRef()] = true
	}

	// Validate portal snippets
	snippetRefs := make(map[string]bool)
	registry["portal_snippet"] = snippetRefs
	for i := range rs.PortalSnippets {
		snippet := &rs.PortalSnippets[i]
		if err := snippet.Validate(); err != nil {
			return fmt.Errorf("invalid portal_snippet %q: %w", snippet.GetRef(), err)
		}
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(snippet.GetRef(), "portal_snippet"); err != nil {
			return err
		}
		snippetRefs[snippet.GetRef()] = true
	}

	// Validate portal customizations
	customizationRefs := make(map[string]bool)
	registry["portal_customization"] = customizationRefs
	for i := range rs.PortalCustomizations {
		customization := &rs.PortalCustomizations[i]
		if err := customization.Validate(); err != nil {
			return fmt.Errorf("invalid portal_customization %q: %w", customization.GetRef(), err)
		}
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(customization.GetRef(), "portal_customization"); err != nil {
			return err
		}
		customizationRefs[customization.GetRef()] = true
	}

	// Validate portal custom domains
	domainRefs := make(map[string]bool)
	registry["portal_custom_domain"] = domainRefs
	for i := range rs.PortalCustomDomains {
		domain := &rs.PortalCustomDomains[i]
		if err := domain.Validate(); err != nil {
			return fmt.Errorf("invalid portal_custom_domain %q: %w", domain.GetRef(), err)
		}
		// Check global ref uniqueness
		if err := globalRefRegistry.AddRef(domain.GetRef(), "portal_custom_domain"); err != nil {
			return err
		}
		domainRefs[domain.GetRef()] = true
	}

	return nil
}

// validateResourceReferences validates references for a single resource using its mapping
func (l *Loader) validateResourceReferences(resource interface{}, registry map[string]map[string]bool) error {
	// fmt.Printf("DEBUG: validateResourceReferences called with resource type: %T\n", resource)
	
	// Check if resource implements ReferenceMapping
	refMapper, ok := resource.(resources.ReferenceMapping)
	if !ok {
		return nil // Resource doesn't have reference fields
	}

	// Get the resource's ref for error messages
	refResource, ok := resource.(resources.ReferencedResource)
	if !ok {
		return nil // Shouldn't happen, but be safe
	}

	mappings := refMapper.GetReferenceFieldMappings()
	// fmt.Printf("DEBUG: Reference mappings: %v\n", mappings)
	
	for fieldPath, expectedType := range mappings {
		fieldValue := l.getFieldValue(resource, fieldPath)
		// fmt.Printf("DEBUG: Field %s = '%s' (expected type: %s)\n", fieldPath, fieldValue, expectedType)
		
		if fieldValue == "" {
			continue // Empty references are allowed (optional fields)
		}

		// Special handling for array fields (e.g., auth_strategy_ids)
		if strings.HasSuffix(fieldPath, "_ids") {
			// For now, skip array validation - would need reflection to handle properly
			continue
		}

		// Check if the referenced resource exists
		if !registry[expectedType][fieldValue] {
			return fmt.Errorf("resource %q references unknown %s: %s (field: %s)",
				refResource.GetRef(), expectedType, fieldValue, fieldPath)
		}
	}

	return nil
}

// getFieldValue extracts field value using reflection, supporting qualified field names
func (l *Loader) getFieldValue(resource interface{}, fieldPath string) string {
	v := reflect.ValueOf(resource)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	
	// fmt.Printf("DEBUG: getFieldValue - type: %v, fieldPath: %s\n", v.Type(), fieldPath)

	// Split field path for nested fields (e.g., "service.control_plane_id")
	parts := strings.Split(fieldPath, ".")
	
	for i, part := range parts {
		// Handle both struct field names and YAML tags
		field := l.findField(v, part)
		if !field.IsValid() {
			return ""
		}

		// For the last part, get the string value
		if i == len(parts)-1 {
			if field.Kind() == reflect.String {
				return field.String()
			} else if field.Kind() == reflect.Ptr && !field.IsNil() {
				elem := field.Elem()
				if elem.Kind() == reflect.String {
					return elem.String()
				}
			}
			return ""
		}

		// For intermediate parts, navigate deeper
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				return ""
			}
			v = field.Elem()
		} else {
			v = field
		}
	}

	return ""
}

// findField finds a field by name or YAML tag
func (l *Loader) findField(v reflect.Value, name string) reflect.Value {
	if v.Kind() != reflect.Struct {
		return reflect.Value{}
	}

	t := v.Type()
	
	// First try direct field name
	if field, ok := t.FieldByName(name); ok {
		return v.FieldByIndex(field.Index)
	}

	// Then try by YAML tag
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" {
			continue
		}

		// Handle yaml tags like "field_name,omitempty"
		tagParts := strings.Split(yamlTag, ",")
		if tagParts[0] == name {
			return v.Field(i)
		}
	}

	// Special case for embedded structs (like SDK types)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Anonymous {
			if embeddedField := l.findField(v.Field(i), name); embeddedField.IsValid() {
				return embeddedField
			}
		}
	}
	
	// Try converting snake_case to PascalCase for SDK field names
	pascalCase := l.toPascalCase(name)
	if pascalCase != name {
		return l.findField(v, pascalCase)
	}

	return reflect.Value{}
}

// toPascalCase converts snake_case to PascalCase
func (l *Loader) toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		if len(parts[i]) > 0 {
			// Special handling for common abbreviations
			if parts[i] == "id" || parts[i] == "api" || parts[i] == "url" {
				parts[i] = strings.ToUpper(parts[i])
			} else {
				parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
			}
		}
	}
	return strings.Join(parts, "")
}

// validateNamespaces validates all namespace values in the resource set
func (l *Loader) validateNamespaces(rs *resources.ResourceSet) error {
	nsValidator := validator.NewNamespaceValidator()
	namespaces := make(map[string]bool)
	
	// Collect all unique namespaces from parent resources
	// Portals
	for _, portal := range rs.Portals {
		if portal.Kongctl != nil && portal.Kongctl.Namespace != nil {
			namespaces[*portal.Kongctl.Namespace] = true
		}
	}
	
	// APIs
	for _, api := range rs.APIs {
		if api.Kongctl != nil && api.Kongctl.Namespace != nil {
			namespaces[*api.Kongctl.Namespace] = true
		}
	}
	
	// Application Auth Strategies
	for _, strategy := range rs.ApplicationAuthStrategies {
		if strategy.Kongctl != nil && strategy.Kongctl.Namespace != nil {
			namespaces[*strategy.Kongctl.Namespace] = true
		}
	}
	
	// Control Planes
	for _, cp := range rs.ControlPlanes {
		if cp.Kongctl != nil && cp.Kongctl.Namespace != nil {
			namespaces[*cp.Kongctl.Namespace] = true
		}
	}
	
	// Convert to slice for validation
	namespaceList := make([]string, 0, len(namespaces))
	for ns := range namespaces {
		namespaceList = append(namespaceList, ns)
	}
	
	// Validate all namespaces
	if err := nsValidator.ValidateNamespaces(namespaceList); err != nil {
		return fmt.Errorf("namespace validation failed: %w", err)
	}
	
	return nil
}