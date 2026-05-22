package resources

import (
	"strings"

	"github.com/kong/kongctl/internal/declarative/tags"
)

// RoleEntityResourceType maps Konnect role entity_type_name values to the
// declarative resource type that can supply the referenced entity_id.
func RoleEntityResourceType(entityTypeName string) (ResourceType, bool) {
	switch normalizeRoleEntityTypeName(entityTypeName) {
	case "api", "apis", "apiproduct", "apiproducts", "service", "services":
		return ResourceTypeAPI, true
	case "portal", "portals":
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
