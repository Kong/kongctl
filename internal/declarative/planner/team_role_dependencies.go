package planner

import (
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
)

// adjustTeamRoleDependencies wires team role changes to depend on entity creations when
// entity_id is a reference placeholder pointing to a resource being created in the same plan.
func adjustTeamRoleDependencies(plan *Plan) {
	if plan == nil {
		return
	}

	adjustTeamRoleCreateDependencies(plan)
	adjustTeamRoleDeleteDependencies(plan)
}

// adjustTeamRoleCreateDependencies wires team role changes to depend on entity creations when
// entity_id is a reference placeholder pointing to a resource being created in the same plan.
func adjustTeamRoleCreateDependencies(plan *Plan) {
	changeByKey := make(map[string]string) // resource_type|ref -> changeID
	for _, change := range plan.Changes {
		key := change.ResourceType + "|" + change.ResourceRef
		changeByKey[key] = change.ID
	}

	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.ResourceType != ResourceTypePortalTeamRole &&
			change.ResourceType != ResourceTypeOrganizationTeamRole &&
			change.ResourceType != ResourceTypeOrganizationUserRole &&
			change.ResourceType != ResourceTypeOrganizationSystemAccountRole {
			continue
		}

		refInfo, ok := change.References[FieldEntityID]
		if !ok {
			continue
		}

		entityRef := refInfo.Ref
		if !tags.IsRefPlaceholder(entityRef) {
			continue
		}

		parsedRef, _, ok := tags.ParseRefPlaceholder(entityRef)
		if !ok || parsedRef == "" {
			continue
		}

		entityTypeName, _ := change.Fields[FieldEntityTypeName].(string)
		entityResourceType, ok := resources.RoleEntityResourceType(entityTypeName)
		if !ok {
			continue
		}

		entityKey := string(entityResourceType) + "|" + parsedRef
		if entityChangeID, exists := changeByKey[entityKey]; exists {
			if !containsString(change.DependsOn, entityChangeID) {
				change.DependsOn = append(change.DependsOn, entityChangeID)
			}
		}
	}
}

// adjustTeamRoleDeleteDependencies ensures team role assignments are removed
// before deleting the parent team or the API referenced by the role assignment.
func adjustTeamRoleDeleteDependencies(plan *Plan) {
	var roleDeletes []*PlannedChange
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.Action != ActionDelete {
			continue
		}
		if change.ResourceType != ResourceTypeOrganizationTeamRole &&
			change.ResourceType != ResourceTypePortalTeamRole &&
			change.ResourceType != ResourceTypeOrganizationUserRole &&
			change.ResourceType != ResourceTypeOrganizationUserTeamMembership &&
			change.ResourceType != ResourceTypeOrganizationSystemAccountRole &&
			change.ResourceType != ResourceTypeOrganizationSystemAccountTeamMembership {
			continue
		}
		roleDeletes = append(roleDeletes, change)
	}

	if len(roleDeletes) == 0 {
		return
	}

	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.Action != ActionDelete {
			continue
		}

		switch change.ResourceType {
		case ResourceTypeOrganizationTeam, ResourceTypePortalTeam:
			for _, roleDelete := range roleDeletes {
				if teamRoleBelongsToTeam(roleDelete, change.ResourceID) ||
					teamMembershipBelongsToTeam(roleDelete, change.ResourceID) {
					change.DependsOn = appendDependsOn(change.DependsOn, roleDelete.ID)
				}
			}
		case ResourceTypeAPI, ResourceTypePortal, ResourceTypeControlPlane:
			for _, roleDelete := range roleDeletes {
				if teamRoleEntityResourceType(roleDelete) == change.ResourceType &&
					teamRoleReferencesEntity(roleDelete, change.ResourceID) {
					change.DependsOn = appendDependsOn(change.DependsOn, roleDelete.ID)
				}
			}
		}
	}
}

func appendRoleEntityDependency(dependencies []string, plan *Plan, entityID, entityTypeName string) []string {
	entityRef, _, ok := tags.ParseRefPlaceholder(entityID)
	if !ok || entityRef == "" {
		return dependencies
	}

	entityResourceType, ok := resources.RoleEntityResourceType(entityTypeName)
	if !ok {
		return dependencies
	}

	entityChangeID := findChangeID(plan, string(entityResourceType), entityRef)
	if entityChangeID == "" || containsString(dependencies, entityChangeID) {
		return dependencies
	}
	return append(dependencies, entityChangeID)
}

func teamRoleEntityResourceType(roleDelete *PlannedChange) string {
	if roleDelete == nil {
		return ""
	}

	entityTypeName, _ := roleDelete.Fields[FieldEntityTypeName].(string)
	if entityTypeName == "" {
		return ResourceTypeAPI
	}

	resourceType, ok := resources.RoleEntityResourceType(entityTypeName)
	if !ok {
		return ""
	}
	return string(resourceType)
}

func teamMembershipBelongsToTeam(change *PlannedChange, teamID string) bool {
	if change == nil || teamID == "" {
		return false
	}
	if change.ResourceType != ResourceTypeOrganizationUserTeamMembership &&
		change.ResourceType != ResourceTypeOrganizationSystemAccountTeamMembership {
		return false
	}
	refInfo, ok := change.References[FieldTeamID]
	return ok && refInfo.ID == teamID
}

func teamRoleBelongsToTeam(roleDelete *PlannedChange, teamID string) bool {
	if roleDelete == nil || teamID == "" {
		return false
	}

	if roleDelete.Parent != nil && roleDelete.Parent.ID == teamID {
		return true
	}

	refInfo, ok := roleDelete.References[FieldTeamID]
	return ok && refInfo.ID == teamID
}

func teamRoleReferencesEntity(roleDelete *PlannedChange, entityID string) bool {
	if roleDelete == nil || entityID == "" {
		return false
	}

	value, ok := roleDelete.Fields[FieldEntityID]
	if !ok {
		return false
	}

	entity, ok := value.(string)
	return ok && entity == entityID
}
