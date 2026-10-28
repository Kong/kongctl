package planner

import (
	"encoding/json"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	testdecl "github.com/kong/kongctl/test/declarative"
	"github.com/stretchr/testify/require"
)

func TestDirectDeleteDispatchesEveryEnrolledAIGatewayFamily(t *testing.T) {
	for _, tc := range testdecl.AIGatewayOwnedChildren(t) {
		t.Run(tc.Collection, func(t *testing.T) {
			kind := resources.ResourceType(tc.ResourceType)
			require.True(t, resources.HasIndependentChildOwnership(kind))
			declaration := maps.Clone(tc.Body)
			declaration["ref"], declaration[FieldName], declaration[FieldDisplayName] = "owned", "owned", "Owned"
			if kind == resources.ResourceTypeAIGatewayVault || kind == resources.ResourceTypeAIGatewayCACertificate {
				delete(declaration, FieldDisplayName)
			}
			missing := maps.Clone(declaration)
			missing["ref"], missing[FieldName] = "missing", "missing"
			manifest := map[string]any{
				"_defaults": map[string]any{"kongctl": map[string]any{"namespace": "team-a"}},
				"ai_gateways": []any{map[string]any{
					"ref": "shared", "_external": map[string]any{"id": "gateway-id"},
					tc.Collection: []any{declaration, missing},
				}},
			}
			data, err := json.Marshal(manifest)
			require.NoError(t, err)
			path := filepath.Join(t.TempDir(), "team.yaml")
			require.NoError(t, os.WriteFile(path, data, 0o600))
			rs, err := loader.New().LoadFile(path)
			require.NoError(t, err)
			api := &remainingOwnershipTransport{path: tc.Path}
			for _, item := range []struct{ name, ns string }{
				{"owned", "team-a"}, {"not-declared", "team-a"}, {"foreign", "team-b"}, {"legacy", ""},
			} {
				value := maps.Clone(tc.Body)
				value[FieldID], value[FieldName], value[FieldDisplayName] = item.name+"-id", item.name, "Owned"
				value[FieldManagedBy] = resources.BuildChildManagedBy(nil, nil, item.ns, false, "dev")
				value["created_at"], value["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
				api.values = append(api.values, value)
			}
			p := NewPlanner(state.NewClient(aigatewayCollectionsConfig(api)), slog.Default())
			plan, err := p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeDelete})
			require.NoError(t, err)
			require.Len(t, plan.Changes, 1)
			change := plan.Changes[0]
			require.Equal(t, ActionDelete, change.Action)
			require.Equal(t, tc.ResourceType, change.ResourceType)
			require.Equal(t, "owned-id", change.ResourceID)
			require.Equal(t, "team-a", change.Namespace)
			for _, owner := range []string{"team-b", ""} {
				api.values[0][FieldManagedBy] = resources.BuildChildManagedBy(nil, nil, owner, false, "dev")
				plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeDelete})
				require.Error(t, err)
				require.Nil(t, plan)
			}
			api.values[0][FieldManagedBy] = resources.BuildChildManagedBy(nil, nil, "team-a", true, "dev")
			plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeDelete})
			require.ErrorContains(t, err, "protected")
			require.Nil(t, plan)
		})
	}
}

func TestExternalGatewayDeleteDoesNotEnrollExcludedFamilies(t *testing.T) {
	rs := federatedResourceSet()
	rs.AIGatewayConfigStores = []resources.AIGatewayConfigStoreResource{{
		BaseResource: resources.BaseResource{Ref: "store"}, AIGateway: "shared", Name: "store",
	}}
	rs.AIGatewayCertificates = []resources.AIGatewayCertificateResource{{
		BaseResource: resources.BaseResource{Ref: "cert"}, AIGateway: "shared", Name: "cert", Cert: "public-cert",
	}}
	rs.AIGatewayDataPlaneCertificates = []resources.AIGatewayDataPlaneCertificateResource{{
		BaseResource: resources.BaseResource{Ref: "dp-cert"}, AIGateway: "shared",
	}}
	p := NewPlanner(state.NewClient(state.ClientConfig{}), slog.Default())
	plan, err := p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeDelete})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	require.Len(t, plan.Warnings, 3)
}

func TestSyncRejectsMalformedOwnershipBeforePruning(t *testing.T) {
	for _, tc := range testdecl.AIGatewayOwnedChildren(t) {
		t.Run(tc.Collection, func(t *testing.T) {
			manifest := map[string]any{
				"_defaults": map[string]any{"kongctl": map[string]any{"namespace": "team-a"}},
				"ai_gateways": []any{map[string]any{
					"ref": "shared", "_external": map[string]any{"id": "gateway-id"}, tc.Collection: []any{},
				}},
			}
			data, err := json.Marshal(manifest)
			require.NoError(t, err)
			path := filepath.Join(t.TempDir(), "team.yaml")
			require.NoError(t, os.WriteFile(path, data, 0o600))
			rs, err := loader.New().LoadFile(path)
			require.NoError(t, err)
			api := &remainingOwnershipTransport{path: tc.Path}
			for _, namespace := range []string{"team-b", ""} {
				value := maps.Clone(tc.Body)
				value[FieldID], value[FieldName], value[FieldDisplayName] = namespace+"-id", namespace+"-child", "Retained"
				metadata := resources.BuildChildManagedBy(nil, nil, namespace, false, "dev")
				metadata[resources.ChildProtectedKey] = "TRUE"
				value[FieldManagedBy] = metadata
				value["created_at"], value["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
				api.values = append(api.values, value)
			}
			p := NewPlanner(state.NewClient(aigatewayCollectionsConfig(api)), slog.Default())
			plan, err := p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
			require.NoError(t, err, "malformed metadata outside this namespace must not expand pruning scope")
			require.Empty(t, plan.Changes)

			owned := maps.Clone(api.values[0])
			owned[FieldID], owned[FieldName] = "owned-id", "owned"
			metadata := resources.BuildChildManagedBy(nil, nil, "team-a", false, "dev")
			metadata[resources.ChildProtectedKey] = "TRUE"
			owned[FieldManagedBy] = metadata
			api.values = append(api.values, owned)
			plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
			require.ErrorContains(t, err, "invalid kongctl-protected metadata")
			require.Nil(t, plan, "sync must not return an executable plan with a malformed prune candidate")

			metadata[resources.ChildProtectedKey] = "false"
			plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
			require.NoError(t, err)
			require.Len(t, plan.Changes, 1)
			require.Equal(t, "owned-id", plan.Changes[0].ResourceID)
		})
	}
}
