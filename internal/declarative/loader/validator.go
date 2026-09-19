package loader

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/declarative/validator"
	"github.com/kong/kongctl/internal/util"
)

// validateResourceSet validates all resources and checks for ref uniqueness
func (l *Loader) validateResourceSet(rs *resources.ResourceSet) error {
	normalizeOrganizationTeamSelectors(rs)

	if err := rs.ValidateRegisteredCollections(); err != nil {
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

func normalizeOrganizationTeamSelectors(rs *resources.ResourceSet) {
	if rs == nil {
		return
	}
	for i := range rs.OrganizationUserTeamMemberships {
		rs.OrganizationUserTeamMemberships[i].Team = resources.NormalizeResourceRef(
			rs.OrganizationUserTeamMemberships[i].Team,
		)
	}
	for i := range rs.OrganizationSystemAccountTeamMemberships {
		rs.OrganizationSystemAccountTeamMemberships[i].Team = resources.NormalizeResourceRef(
			rs.OrganizationSystemAccountTeamMemberships[i].Team,
		)
	}
	for i := range rs.OrganizationTeamRoles {
		role := &rs.OrganizationTeamRoles[i]
		original := role.Team
		role.Team = resources.NormalizeResourceRef(role.Team)
		if rs.SyncScope != nil {
			rs.SyncScope.RebindChildParent(resources.ResourceTypeOrganizationTeam, original, role.Team)
		}
	}
}

func (l *Loader) validateOrganizationTeamRoles(
	roles []resources.OrganizationTeamRoleResource,
	rs *resources.ResourceSet,
) error {
	return resources.ValidateOrganizationTeamRoles(rs, roles)
}

func (l *Loader) validateUserRoleEntityReference(
	role *resources.OrganizationUserRoleResource,
	rs *resources.ResourceSet,
) error {
	return resources.ValidateRoleEntityReference(
		resources.ResourceTypeOrganizationUserRole, role.GetRef(), role.EntityID, role.EntityTypeName, rs,
	)
}

func (l *Loader) validateSystemAccountRoleEntityReference(
	role *resources.OrganizationSystemAccountRoleResource,
	rs *resources.ResourceSet,
) error {
	return resources.ValidateRoleEntityReference(
		resources.ResourceTypeOrganizationSystemAccountRole, role.GetRef(), role.EntityID, role.EntityTypeName, rs,
	)
}

// validatePortals validates portal resources
func (l *Loader) validatePortals(portals []resources.PortalResource, rs *resources.ResourceSet) error {
	return resources.ValidateResourceCollection(rs, portals)
}

// validateAuthStrategies validates auth strategy resources
func (l *Loader) validateAuthStrategies(
	strategies []resources.ApplicationAuthStrategyResource,
	rs *resources.ResourceSet,
) error {
	return resources.ValidateResourceCollection(rs, strategies)
}

// validateControlPlanes validates control plane resources
func (l *Loader) validateControlPlanes(
	cps []resources.ControlPlaneResource,
	rs *resources.ResourceSet,
) error {
	return resources.ValidateResourceCollection(rs, cps)
}

// Retain the existing test-facing entry point while dispatch uses registration.
func (l *Loader) validateAIGatewayConfigStoreSecrets(rs *resources.ResourceSet) error {
	return rs.ValidateRegisteredResource(resources.ResourceTypeAIGatewayConfigStoreSecret)
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

	for i := range rs.PortalIPAllowLists {
		if err := l.validateResourceReferences(&rs.PortalIPAllowLists[i], rs); err != nil {
			return err
		}
	}

	for i := range rs.PortalAuditLogWebhooks {
		if err := l.validateResourceReferences(&rs.PortalAuditLogWebhooks[i], rs); err != nil {
			return err
		}
	}

	// Note: API versions don't have outbound references, so no validation needed

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
		if tags.IsExternalPlaceholder(fieldValue) {
			if _, ok := tags.ParseExternalPlaceholder(fieldValue); !ok {
				return fmt.Errorf(
					"resource %q has invalid external lookup placeholder (field: %s)",
					refResource.GetRef(),
					fieldPath,
				)
			}
			// External lookups are validated and resolved by the planner once the
			// target type and any parent scope are available.
			continue
		}

		// Skip validation for raw UUIDs — these are already-resolved Konnect
		// resource IDs that cannot be matched against local refs.
		if util.IsValidUUID(fieldValue) {
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
	if v.Kind() == reflect.Pointer {
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
			} else if field.Kind() == reflect.Pointer && !field.IsNil() {
				elem := field.Elem()
				if elem.Kind() == reflect.String {
					return elem.String()
				}
			}
			return ""
		}

		// For intermediate parts, navigate deeper
		if field.Kind() == reflect.Pointer {
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
	for i := range t.NumField() {
		field := t.Field(i)
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" {
			continue
		}

		// Handle yaml tags like "field_name,omitempty"
		tagName, _, _ := strings.Cut(yamlTag, ",")
		if tagName == name {
			return v.Field(i)
		}
	}

	// Special case for embedded structs (like SDK types)
	for i := range t.NumField() {
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

	// Validate all namespaces
	if err := nsValidator.ValidateNamespaces(rs.NamespaceValues()); err != nil {
		return fmt.Errorf("namespace validation failed: %w", err)
	}

	return nil
}
