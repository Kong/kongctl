package planner

import (
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/declarative/resources"
)

// adjustPortalTeamRoleDependencies wires portal_team_role changes to depend on API creations
// when the entity_id is a reference placeholder pointing to an API being created in the same plan.
func adjustPortalTeamRoleDependencies(plan *Plan) {
	if plan == nil {
		return
	}

	changeByKey := make(map[string]string) // resource_type|ref -> changeID
	for _, change := range plan.Changes {
		key := change.ResourceType + "|" + change.ResourceRef
		changeByKey[key] = change.ID
	}

	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.ResourceType != ResourceTypePortalTeamRole {
			continue
		}

		refInfo, ok := change.References["entity_id"]
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
