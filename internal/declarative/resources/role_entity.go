package resources

import (
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/declarative/tags"
)

// RoleEntityResourceType maps Konnect role entity_type_name values to the
// declarative resource type that can supply the referenced entity_id.
func RoleEntityResourceType(entityTypeName string) (ResourceType, bool) {
	switch normalizeRoleEntityTypeName(entityTypeName) {
	case "api", "apis", "apiproduct", "apiproducts", "service", "services":
		return ResourceTypeAPI, true
	case string(ResourceTypePortal), "portals":
		return ResourceTypePortal, true
	case "controlplane", "controlplanes":
		return ResourceTypeControlPlane, true
	default:
		return "", false
	}
}

func normalizeRoleEntityTypeName(entityTypeName string) string {
	normalized := strings.ToLower(strings.TrimSpace(entityTypeName))
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	return normalized
}

func roleEntityDependency(entityID, entityTypeName string) []ResourceRef {
	if !tags.IsRefPlaceholder(entityID) {
		return nil
	}

	ref, _, ok := tags.ParseRefPlaceholder(entityID)
	if !ok || ref == "" {
		return nil
	}

	resourceType, ok := RoleEntityResourceType(entityTypeName)
	if !ok {
		return nil
	}

	return []ResourceRef{{Kind: resourceType, Ref: ref}}
}

// ValidateRoleEntityReference checks a declarative entity selector against the
// Konnect role entity type. Literal entity IDs require no resource lookup.
func ValidateRoleEntityReference(
	roleType ResourceType,
	roleRef string,
	entityID string,
	entityTypeName string,
	rs *ResourceSet,
) error {
	if !tags.IsRefPlaceholder(entityID) {
		return nil
	}

	entityRef, _, ok := tags.ParseRefPlaceholder(entityID)
	if !ok || entityRef == "" {
		return fmt.Errorf("%s %q has invalid entity_id reference: %s", roleType, roleRef, entityID)
	}

	expectedType, ok := RoleEntityResourceType(entityTypeName)
	if !ok {
		return fmt.Errorf(
			"%s %q has unsupported entity_type_name for entity_id reference: %s",
			roleType,
			roleRef,
			entityTypeName,
		)
	}

	if resource, found := rs.GetResourceByRef(entityRef); !found {
		return fmt.Errorf("%s %q references unknown %s: %s (field: entity_id)",
			roleType, roleRef, expectedType, entityRef)
	} else if resource.GetType() != expectedType {
		return fmt.Errorf("%s %q references %s but expected %s: %s (field: entity_id)",
			roleType, roleRef, resource.GetType(), expectedType, entityRef)
	}

	return nil
}
