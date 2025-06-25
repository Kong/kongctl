package loader

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
)

// validateResourceSet validates all resources and checks for ref uniqueness
func (l *Loader) validateResourceSet(rs *resources.ResourceSet) error {
	// Build registry of all resources by type for reference validation
	resourceRegistry := make(map[string]map[string]bool)

	// Validate portals
	if err := l.validatePortals(rs.Portals, resourceRegistry); err != nil {
		return err
	}

	// Validate auth strategies
	if err := l.validateAuthStrategies(rs.ApplicationAuthStrategies, resourceRegistry); err != nil {
		return err
	}

	// Validate control planes
	if err := l.validateControlPlanes(rs.ControlPlanes, resourceRegistry); err != nil {
		return err
	}

	// Validate APIs and their children
	if err := l.validateAPIs(rs.APIs, resourceRegistry); err != nil {
		return err
	}

	// Validate cross-resource references
	if err := l.validateCrossReferences(rs, resourceRegistry); err != nil {
		return err
	}

	return nil
}

// validatePortals validates portal resources
func (l *Loader) validatePortals(portals []resources.PortalResource, registry map[string]map[string]bool) error {
	refs := make(map[string]bool)
	registry["portal"] = refs

	for i := range portals {
		portal := &portals[i]

		// Validate resource
		if err := portal.Validate(); err != nil {
			return fmt.Errorf("invalid portal %q: %w", portal.GetRef(), err)
		}

		// Check uniqueness
		if refs[portal.GetRef()] {
			return fmt.Errorf("duplicate portal ref: %s", portal.GetRef())
		}
		refs[portal.GetRef()] = true
	}

	return nil
}

// validateAuthStrategies validates auth strategy resources
func (l *Loader) validateAuthStrategies(
	strategies []resources.ApplicationAuthStrategyResource,
	registry map[string]map[string]bool,
) error {
	refs := make(map[string]bool)
	registry["application_auth_strategy"] = refs

	for i := range strategies {
		strategy := &strategies[i]

		// Validate resource
		if err := strategy.Validate(); err != nil {
			return fmt.Errorf("invalid application_auth_strategy %q: %w", strategy.GetRef(), err)
		}

		// Check uniqueness
		if refs[strategy.GetRef()] {
			return fmt.Errorf("duplicate application_auth_strategy ref: %s", strategy.GetRef())
		}
		refs[strategy.GetRef()] = true
	}

	return nil
}

// validateControlPlanes validates control plane resources
func (l *Loader) validateControlPlanes(
	cps []resources.ControlPlaneResource,
	registry map[string]map[string]bool,
) error {
	refs := make(map[string]bool)
	registry["control_plane"] = refs

	for i := range cps {
		cp := &cps[i]

		// Validate resource
		if err := cp.Validate(); err != nil {
			return fmt.Errorf("invalid control_plane %q: %w", cp.GetRef(), err)
		}

		// Check uniqueness
		if refs[cp.GetRef()] {
			return fmt.Errorf("duplicate control_plane ref: %s", cp.GetRef())
		}
		refs[cp.GetRef()] = true
	}

	return nil
}

// validateAPIs validates API resources and their children
func (l *Loader) validateAPIs(apis []resources.APIResource, registry map[string]map[string]bool) error {
	apiRefs := make(map[string]bool)
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

		// Check API uniqueness
		if apiRefs[api.GetRef()] {
			return fmt.Errorf("duplicate api ref: %s", api.GetRef())
		}
		apiRefs[api.GetRef()] = true

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