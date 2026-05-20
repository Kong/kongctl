package planner

import (
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
)

// adjustTeamRoleDependencies wires team role changes to depend on API creations when
// entity_id is a reference placeholder pointing to an API being created in the same plan.
func adjustTeamRoleDependencies(plan *Plan) {
	if plan == nil {
		return
	}

	adjustTeamRoleCreateDependencies(plan)
	adjustTeamRoleDeleteDependencies(plan)
}

// adjustTeamRoleCreateDependencies wires team role changes to depend on API creations when
// entity_id is a reference placeholder pointing to an API being created in the same plan.
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

		apiKey := string(resources.ResourceTypeAPI) + "|" + parsedRef
		if apiChangeID, exists := changeByKey[apiKey]; exists {
			if !containsString(change.DependsOn, apiChangeID) {
				change.DependsOn = append(change.DependsOn, apiChangeID)
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
		case ResourceTypeAPI:
			for _, roleDelete := range roleDeletes {
				if teamRoleReferencesEntity(roleDelete, change.ResourceID) {
					change.DependsOn = appendDependsOn(change.DependsOn, roleDelete.ID)
				}
			}
		}
	}
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
