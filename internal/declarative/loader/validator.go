package loader

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/declarative/validator"
)

// validateResourceSet validates all resources and checks for ref uniqueness
func (l *Loader) validateResourceSet(rs *resources.ResourceSet) error {
	// Validate portals
	if err := l.validatePortals(rs.Portals, rs); err != nil {
		return err
	}

	// Validate auth strategies
	if err := l.validateAuthStrategies(rs.ApplicationAuthStrategies, rs); err != nil {
		return err
	}

	// Validate control planes
	if err := l.validateControlPlanes(rs.ControlPlanes, rs); err != nil {
		return err
	}

	// Validate gateway services
	if err := l.validateGatewayServices(rs.GatewayServices, rs); err != nil {
		return err
	}

	// Validate APIs and their children
	if err := l.validateAPIs(rs.APIs, rs); err != nil {
		return err
	}

	// Validate separate API child resources (extracted from nested resources)
	if err := l.validateSeparateAPIChildResources(rs); err != nil {
		return err
	}

	// Validate cross-resource references
	if err := l.validateCrossReferences(rs); err != nil {
		return err
	}

	// Validate namespaces
	if err := l.validateNamespaces(rs); err != nil {
		return err
	}

	return nil
}

// validateGatewayServices validates gateway service resources
func (l *Loader) validateGatewayServices(
	services []resources.GatewayServiceResource,
	rs *resources.ResourceSet,
) error {
	for i := range services {
		service := &services[i]

		if err := service.Validate(); err != nil {
			return fmt.Errorf("invalid gateway_service %q: %w", service.GetRef(), err)
		}

		if existing, found := rs.GetResourceByRef(service.GetRef()); found {
			if existing.GetType() != resources.ResourceTypeGatewayService {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					service.GetRef(), existing.GetType())
			}
		}
	}

	return nil
}

// validatePortals validates portal resources
func (l *Loader) validatePortals(portals []resources.PortalResource, rs *resources.ResourceSet) error {
	names := make(map[string]string) // name -> ref mapping (names unique per type)

	for i := range portals {
		portal := &portals[i]

		// Validate resource
		if err := portal.Validate(); err != nil {
			return fmt.Errorf("invalid portal %q: %w", portal.GetRef(), err)
		}

		// Check global ref uniqueness across different resource types
		// Don't check within same type - that's handled by the loader during append
		if existing, found := rs.GetResourceByRef(portal.GetRef()); found {
			if existing.GetType() != resources.ResourceTypePortal {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					portal.GetRef(), existing.GetType())
			}
		}

		// Check name uniqueness (within portal type only)
		portalName := portal.GetMoniker()
		if existingRef, exists := names[portalName]; exists {
			return fmt.Errorf("duplicate portal name '%s' (ref: %s conflicts with ref: %s)",
				portalName, portal.GetRef(), existingRef)
		}

		names[portalName] = portal.GetRef()
	}

	return nil
}

// validateAuthStrategies validates auth strategy resources
func (l *Loader) validateAuthStrategies(
	strategies []resources.ApplicationAuthStrategyResource,
	rs *resources.ResourceSet,
) error {
	names := make(map[string]string) // name -> ref mapping (names unique per type)

	for i := range strategies {
		strategy := &strategies[i]

		// Validate resource
		if err := strategy.Validate(); err != nil {
			return fmt.Errorf("invalid application_auth_strategy %q: %w", strategy.GetRef(), err)
		}

		// Check global ref uniqueness across different resource types
		// Don't check within same type - that's handled by the loader during append
		if existing, found := rs.GetResourceByRef(strategy.GetRef()); found {
			if existing.GetType() != resources.ResourceTypeApplicationAuthStrategy {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					strategy.GetRef(), existing.GetType())
			}
		}

		// Check name uniqueness (within auth strategy type only)
		stratName := strategy.GetMoniker()
		if existingRef, exists := names[stratName]; exists {
			return fmt.Errorf("duplicate application_auth_strategy name '%s' (ref: %s conflicts with ref: %s)",
				stratName, strategy.GetRef(), existingRef)
		}

		names[stratName] = strategy.GetRef()
	}

	return nil
}

// validateControlPlanes validates control plane resources
func (l *Loader) validateControlPlanes(
	cps []resources.ControlPlaneResource,
	rs *resources.ResourceSet,
) error {
	names := make(map[string]string) // name -> ref mapping (names unique per type)

	for i := range cps {
		cp := &cps[i]

		// Validate resource
		if err := cp.Validate(); err != nil {
			return fmt.Errorf("invalid control_plane %q: %w", cp.GetRef(), err)
		}

		// Check global ref uniqueness across different resource types
		// Don't check within same type - that's handled by the loader during append
		if existing, found := rs.GetResourceByRef(cp.GetRef()); found {
			if existing.GetType() != resources.ResourceTypeControlPlane {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					cp.GetRef(), existing.GetType())
			}
		}

		// Check name uniqueness (within control plane type only)
		if existingRef, exists := names[cp.Name]; exists {
			return fmt.Errorf("duplicate control_plane name '%s' (ref: %s conflicts with ref: %s)",
				cp.Name, cp.GetRef(), existingRef)
		}

		names[cp.Name] = cp.GetRef()
	}

	return nil
}

// validateAPIs validates API resources and their children
func (l *Loader) validateAPIs(apis []resources.APIResource, rs *resources.ResourceSet) error {
	apiNames := make(map[string]string) // name -> ref mapping (names unique per type)

	for i := range apis {
		api := &apis[i]

		// Validate API resource
		if err := api.Validate(); err != nil {
			return fmt.Errorf("invalid api %q: %w", api.GetRef(), err)
		}

		// Check global ref uniqueness across different resource types
		// Don't check within same type - that's handled by the loader during append
		if existing, found := rs.GetResourceByRef(api.GetRef()); found {
			if existing.GetType() != resources.ResourceTypeAPI {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					api.GetRef(), existing.GetType())
			}
		}

		// Check API name uniqueness (within API type only)
		apiName := api.GetMoniker()
		if existingRef, exists := apiNames[apiName]; exists {
			return fmt.Errorf("duplicate api name '%s' (ref: %s conflicts with ref: %s)",
				apiName, api.GetRef(), existingRef)
		}

		apiNames[apiName] = api.GetRef()

		// Validate nested versions (these should be empty after extraction)
		for j := range api.Versions {
			version := &api.Versions[j]
			if err := version.Validate(); err != nil {
				return fmt.Errorf("invalid api_version %q in api %q: %w", version.GetRef(), api.GetRef(), err)
			}
			// Check global ref uniqueness for nested version
			if existing, found := rs.GetResourceByRef(version.GetRef()); found {
				if existing.GetType() != resources.ResourceTypeAPIVersion {
					return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
						version.GetRef(), existing.GetType())
				}
			}
		}

		// Validate nested publications (these should be empty after extraction)
		for j := range api.Publications {
			publication := &api.Publications[j]
			if err := publication.Validate(); err != nil {
				return fmt.Errorf("invalid api_publication %q in api %q: %w", publication.GetRef(), api.GetRef(), err)
			}
			// Check global ref uniqueness for nested publication
			if existing, found := rs.GetResourceByRef(publication.GetRef()); found {
				if existing.GetType() != resources.ResourceTypeAPIPublication {
					return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
						publication.GetRef(), existing.GetType())
				}
			}
		}

		// Validate nested implementations (these should be empty after extraction)
		for j := range api.Implementations {
			implementation := &api.Implementations[j]
			if err := implementation.Validate(); err != nil {
				return fmt.Errorf(
					"invalid api_implementation %q in api %q: %w",
					implementation.GetRef(),
					api.GetRef(),
					err,
				)
			}
			// Check global ref uniqueness for nested implementation
			if existing, found := rs.GetResourceByRef(implementation.GetRef()); found {
				if existing.GetType() != resources.ResourceTypeAPIImplementation {
					return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
						implementation.GetRef(), existing.GetType())
				}
			}
		}
	}

	return nil
}

// validateCrossReferences validates that all cross-resource references are valid
func (l *Loader) validateCrossReferences(rs *resources.ResourceSet) error {
	// Validate portal references
	for i := range rs.Portals {
		if err := l.validateResourceReferences(&rs.Portals[i], rs); err != nil {
			return err
		}
	}

	// Validate API child resource references
	for i := range rs.APIs {
		api := &rs.APIs[i]
		// Validate publication references
		for j := range api.Publications {
			if err := l.validateResourceReferences(&api.Publications[j], rs); err != nil {
				return err
			}
		}

		// Validate implementation references
		for j := range api.Implementations {
			if err := l.validateResourceReferences(&api.Implementations[j], rs); err != nil {
				return err
			}
		}
	}

	// Validate separate API child resources (extracted from nested resources)
	for i := range rs.APIPublications {
		if err := l.validateResourceReferences(&rs.APIPublications[i], rs); err != nil {
			return err
		}
	}

	for i := range rs.APIImplementations {
		if err := l.validateResourceReferences(&rs.APIImplementations[i], rs); err != nil {
			return err
		}
	}

	for i := range rs.APIDocuments {
		if err := l.validateResourceReferences(&rs.APIDocuments[i], rs); err != nil {
			return err
		}
	}

	// Note: API versions don't have outbound references, so no validation needed

	return nil
}

// validateSeparateAPIChildResources validates individual API child resources that were extracted
func (l *Loader) validateSeparateAPIChildResources(rs *resources.ResourceSet) error {
	// Count versions per API to enforce single-version constraint
	// This is a safety check in case the early validation was bypassed
	versionCountByAPI := make(map[string]int)
	for i := range rs.APIVersions {
		version := &rs.APIVersions[i]
		if version.API != "" {
			versionCountByAPI[version.API]++
		}
	}

	// Validate separate API versions
	for i := range rs.APIVersions {
		version := &rs.APIVersions[i]
		if err := version.Validate(); err != nil {
			return fmt.Errorf("invalid api_version %q: %w", version.GetRef(), err)
		}
		// Check global ref uniqueness using RefReader (duplicates were extracted from nested)
		// We need to check against self since these are already in the ResourceSet
		for j := i + 1; j < len(rs.APIVersions); j++ {
			if rs.APIVersions[j].GetRef() == version.GetRef() {
				return fmt.Errorf("duplicate ref '%s' (already defined as api_version)", version.GetRef())
			}
		}
	}

	// Validate separate API publications
	for i := range rs.APIPublications {
		publication := &rs.APIPublications[i]
		if err := publication.Validate(); err != nil {
			return fmt.Errorf("invalid api_publication %q: %w", publication.GetRef(), err)
		}
		// Check for duplicates within extracted publications
		for j := i + 1; j < len(rs.APIPublications); j++ {
			if rs.APIPublications[j].GetRef() == publication.GetRef() {
				return fmt.Errorf("duplicate ref '%s' (already defined as api_publication)", publication.GetRef())
			}
		}
	}

	// Validate separate API implementations
	for i := range rs.APIImplementations {
		implementation := &rs.APIImplementations[i]
		if err := implementation.Validate(); err != nil {
			return fmt.Errorf("invalid api_implementation %q: %w", implementation.GetRef(), err)
		}
		// Check for duplicates within extracted implementations
		for j := i + 1; j < len(rs.APIImplementations); j++ {
			if rs.APIImplementations[j].GetRef() == implementation.GetRef() {
				return fmt.Errorf("duplicate ref '%s' (already defined as api_implementation)", implementation.GetRef())
			}
		}
	}

	// Validate separate API documents
	for i := range rs.APIDocuments {
		document := &rs.APIDocuments[i]
		if err := document.Validate(); err != nil {
			return fmt.Errorf("invalid api_document %q: %w", document.GetRef(), err)
		}
		// Check for duplicates within extracted documents
		for j := i + 1; j < len(rs.APIDocuments); j++ {
			if rs.APIDocuments[j].GetRef() == document.GetRef() {
				return fmt.Errorf("duplicate ref '%s' (already defined as api_document)", document.GetRef())
			}
		}
	}

	// Validate portal pages
	for i := range rs.PortalPages {
		page := &rs.PortalPages[i]
		if err := page.Validate(); err != nil {
			return fmt.Errorf("invalid portal_page %q: %w", page.GetRef(), err)
		}
		// Check for duplicates within extracted pages
		for j := i + 1; j < len(rs.PortalPages); j++ {
			if rs.PortalPages[j].GetRef() == page.GetRef() {
				return fmt.Errorf("duplicate ref '%s' (already defined as portal_page)", page.GetRef())
			}
		}
	}

	// Validate portal snippets
	for i := range rs.PortalSnippets {
		snippet := &rs.PortalSnippets[i]
		if err := snippet.Validate(); err != nil {
			return fmt.Errorf("invalid portal_snippet %q: %w", snippet.GetRef(), err)
		}
		// Check for duplicates within extracted snippets
		for j := i + 1; j < len(rs.PortalSnippets); j++ {
			if rs.PortalSnippets[j].GetRef() == snippet.GetRef() {
				return fmt.Errorf("duplicate ref '%s' (already defined as portal_snippet)", snippet.GetRef())
			}
		}
	}

	// Validate portal customizations
	for i := range rs.PortalCustomizations {
		customization := &rs.PortalCustomizations[i]
		if err := customization.Validate(); err != nil {
			return fmt.Errorf("invalid portal_customization %q: %w", customization.GetRef(), err)
		}
		// Check for duplicates within extracted customizations
		for j := i + 1; j < len(rs.PortalCustomizations); j++ {
			if rs.PortalCustomizations[j].GetRef() == customization.GetRef() {
				return fmt.Errorf(
					"duplicate ref '%s' (already defined as portal_customization)",
					customization.GetRef(),
				)
			}
		}
	}

	// Validate portal custom domains
	for i := range rs.PortalCustomDomains {
		domain := &rs.PortalCustomDomains[i]
		if err := domain.Validate(); err != nil {
			return fmt.Errorf("invalid portal_custom_domain %q: %w", domain.GetRef(), err)
		}
		// Check for duplicates within extracted domains
		for j := i + 1; j < len(rs.PortalCustomDomains); j++ {
			if rs.PortalCustomDomains[j].GetRef() == domain.GetRef() {
				return fmt.Errorf("duplicate ref '%s' (already defined as portal_custom_domain)", domain.GetRef())
			}
		}
	}

	return nil
}

// validateResourceReferences validates references for a single resource using its mapping
func (l *Loader) validateResourceReferences(resource any, rs *resources.ResourceSet) error {
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

		// Skip validation for unresolved reference placeholders
		if strings.HasPrefix(fieldValue, tags.RefPlaceholderPrefix) {
			// This will be resolved during planning/execution phase
			continue
		}

		// Check if the referenced resource exists using RefReader
		if !rs.HasRef(fieldValue) {
			return fmt.Errorf("resource %q references unknown %s: %s (field: %s)",
				refResource.GetRef(), expectedType, fieldValue, fieldPath)
		}

		// Verify the referenced resource is of the expected type
		if actualType, _ := rs.GetResourceTypeByRef(fieldValue); string(actualType) != expectedType {
			return fmt.Errorf("resource %q references %s but expected %s: %s (field: %s)",
				refResource.GetRef(), actualType, expectedType, fieldValue, fieldPath)
		}
	}

	return nil
}

// getFieldValue extracts field value using reflection, supporting qualified field names
func (l *Loader) getFieldValue(resource any, fieldPath string) string {
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
