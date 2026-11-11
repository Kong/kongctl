package planner

import (
	"encoding/json"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestImportedPrincipalAPIKeyPlanningAndSecretSelection(t *testing.T) {
	p, rs, _, api := apiKeyPlanFixture()
	rs.Identity.PrincipalAPIKeys[0].Type = resources.IdentityAPIKeyImportedType
	_, err := p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.ErrorContains(t, err, "requires secret: !secret")
	rs.AddSecretSource("alice-key", "/secret", envSecretExpression("UNAVAILABLE_IMPORTED_KEY"), false)
	plan := seedAPIKeyPlan(t, p, rs, api)
	for _, change := range plan.Changes {
		if change.ResourceType != ResourceTypeIdentityPrincipalAPIKey {
			continue
		}
		require.Equal(t, resources.IdentityAPIKeyImportedType, change.CreationType)
		require.Equal(t, resources.IdentityAPIKeyImportedType,
			change.Fields[FieldManagedBy].(map[string]string)[resources.IdentityAPIKeyCreateTypeKey])
		require.NotContains(t, change.Fields, FieldSecret)
		require.Len(t, change.SecretWrites, 1)
	}
	plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply, WriteSecrets: true})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	data, err := json.Marshal(plan.Warnings)
	require.NoError(t, err)
	require.Contains(t, string(data), "create-only")
	_, err = p.GeneratePlan(t.Context(), rs, Options{
		Mode: PlanModeApply, WriteSecretSelectors: []string{"alice-key#secret"},
	})
	require.ErrorContains(t, err, "declare a new resource to rotate")
	rs.SecretSources = nil
	_, err = p.GeneratePlan(t.Context(), rs, Options{
		Mode: PlanModeApply, WriteSecretSelectors: []string{"alice-key#secret"},
	})
	require.ErrorContains(t, err, "declare a new resource to rotate")
	rs.Identity.PrincipalAPIKeys[0].Name = new("Renamed imported key")
	plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Empty(t, plan.Changes[0].SecretWrites)
	require.Equal(t, resources.IdentityAPIKeyImportedType, plan.Changes[0].CreationType)
	rs.Identity.PrincipalAPIKeys[0].Type = resources.IdentityAPIKeyGeneratedType
	_, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.ErrorContains(t, err, "creation type is immutable")
}
