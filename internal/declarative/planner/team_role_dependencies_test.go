package planner

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdjustTeamRoleDeleteDependencies(t *testing.T) {
	plan := NewPlan("1.0", "test", PlanModeDelete)
	plan.AddChange(PlannedChange{
		ID:           "3:d:api:org-team-roles-api",
		ResourceType: ResourceTypeAPI,
		ResourceRef:  "org-team-roles-api",
		ResourceID:   "api-id",
		Action:       ActionDelete,
	})
	plan.AddChange(PlannedChange{
		ID:           "4:d:organization_team_role:Admin",
		ResourceType: ResourceTypeOrganizationTeamRole,
		ResourceRef:  "Admin",
		ResourceID:   "admin-role-id",
		Action:       ActionDelete,
		Fields: map[string]any{
			FieldEntityID: "api-id",
		},
		References: map[string]ReferenceInfo{
			FieldTeamID: {
				Ref: "platform-role-team",
				ID:  "team-id",
			},
		},
		Parent: &ParentInfo{
			Ref: "platform-role-team",
			ID:  "team-id",
		},
	})
	plan.AddChange(PlannedChange{
		ID:           "1:d:organization_team_role:Viewer",
		ResourceType: ResourceTypeOrganizationTeamRole,
		ResourceRef:  "Viewer",
		ResourceID:   "viewer-role-id",
		Action:       ActionDelete,
		Fields: map[string]any{
			FieldEntityID: "api-id",
		},
		References: map[string]ReferenceInfo{
			FieldTeamID: {
				Ref: "platform-role-team",
				ID:  "team-id",
			},
		},
		Parent: &ParentInfo{
			Ref: "platform-role-team",
			ID:  "team-id",
		},
	})
	plan.AddChange(PlannedChange{
		ID:           "2:d:organization_team:Platform Role Team",
		ResourceType: ResourceTypeOrganizationTeam,
		ResourceRef:  "Platform Role Team",
		ResourceID:   "team-id",
		Action:       ActionDelete,
	})

	adjustTeamRoleDependencies(plan)

	apiDelete := findPlannedChange(t, plan, "3:d:api:org-team-roles-api")
	teamDelete := findPlannedChange(t, plan, "2:d:organization_team:Platform Role Team")
	require.ElementsMatch(t, []string{
		"1:d:organization_team_role:Viewer",
		"4:d:organization_team_role:Admin",
	}, apiDelete.DependsOn)
	require.ElementsMatch(t, []string{
		"1:d:organization_team_role:Viewer",
		"4:d:organization_team_role:Admin",
	}, teamDelete.DependsOn)

	order, err := NewDependencyResolver().ResolveDependencies(plan.Changes)
	require.NoError(t, err)

	viewerIndex := findChangeIndex(order, "1:d:organization_team_role:Viewer")
	adminIndex := findChangeIndex(order, "4:d:organization_team_role:Admin")
	apiIndex := findChangeIndex(order, "3:d:api:org-team-roles-api")
	teamIndex := findChangeIndex(order, "2:d:organization_team:Platform Role Team")
	require.NotEqual(t, -1, viewerIndex)
	require.NotEqual(t, -1, adminIndex)
	require.NotEqual(t, -1, apiIndex)
	require.NotEqual(t, -1, teamIndex)
	require.Less(t, viewerIndex, apiIndex)
	require.Less(t, adminIndex, apiIndex)
	require.Less(t, viewerIndex, teamIndex)
	require.Less(t, adminIndex, teamIndex)
}

func findPlannedChange(t *testing.T, plan *Plan, id string) PlannedChange {
	t.Helper()

	for _, change := range plan.Changes {
		if change.ID == id {
			return change
		}
	}
	t.Fatalf("planned change %q not found", id)
	return PlannedChange{}
}
