package planner

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	testdecl "github.com/kong/kongctl/test/declarative"
	"github.com/stretchr/testify/require"
)

func TestOwnedChildrenReconcileWholeManagedByMap(t *testing.T) {
	for _, tc := range testdecl.AIGatewayOwnedChildren(t) {
		t.Run(tc.Collection, func(t *testing.T) {
			declaration := maps.Clone(tc.Body)
			declaration["ref"], declaration[FieldName] = "owned", "owned"
			if tc.Collection != "vaults" && tc.Collection != "ca_certificates" {
				declaration[FieldDisplayName] = "Owned"
			}
			declaration[FieldLabels] = map[string]string{"team": "a"}
			manifest := map[string]any{
				"_defaults": map[string]any{"kongctl": map[string]any{"namespace": "team-a"}},
				"ai_gateways": []any{map[string]any{
					"ref": "shared", "_external": map[string]any{"id": "gateway-id"}, tc.Collection: []any{declaration},
				}},
			}
			data, err := json.Marshal(manifest)
			require.NoError(t, err)
			path := filepath.Join(t.TempDir(), "team.yaml")
			require.NoError(t, os.WriteFile(path, data, 0o600))
			rs, err := loader.New().LoadFile(path)
			require.NoError(t, err)
			observed := maps.Clone(declaration)
			observed[FieldID] = "owned-id"
			observed["created_at"], observed["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
			metadata := resources.BuildChildManagedBy("team-a", false, "older")
			observed[FieldManagedBy] = metadata
			api := &remainingOwnershipTransport{path: tc.Path, values: []map[string]any{observed}}
			p := NewPlanner(state.NewClient(aigatewayCollectionsConfig(api)), slog.Default())
			plan, err := p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
			require.NoError(t, err)
			require.Empty(t, plan.Changes, "version alone must not schedule writes")
			for _, drift := range []string{"foreign-key", "wrong-tool", "missing-tool"} {
				metadata = resources.BuildChildManagedBy("team-a", false, "older")
				switch drift {
				case "foreign-key":
					metadata["outside"] = "remove"
				case "wrong-tool":
					metadata[resources.ManagedByToolKey] = "other"
				case "missing-tool":
					delete(metadata, resources.ManagedByToolKey)
				}
				observed[FieldManagedBy] = metadata
				plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
				require.NoError(t, err, drift)
				require.Len(t, plan.Changes, 1, drift)
				change := plan.Changes[0]
				require.Equal(t, ActionUpdate, change.Action)
				require.Len(t, change.ChangedFields, 1, drift)
				old := childManagedByField(map[string]any{FieldManagedBy: change.ChangedFields[FieldManagedBy].Old})
				require.Equal(t, metadata, old)
				require.Equal(t, resources.BuildChildManagedBy("team-a", false, "dev"), childManagedByField(change.Fields))
			}
			metadata = resources.BuildChildManagedBy("team-a", true, "older")
			metadata["outside"] = "remove"
			observed[FieldManagedBy] = metadata
			meta, _ := resources.IndependentChildMetadata(rs.AllResourcesByType(resources.ResourceType(tc.ResourceType))[0])
			meta.Protected = new(true)
			plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
			require.ErrorContains(t, err, "protected")
			require.Nil(t, plan)
			meta.Protected = new(false)
			plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
			require.NoError(t, err)
			require.Len(t, plan.Changes, 1)
			require.Equal(t, resources.BuildChildManagedBy("team-a", false, "dev"), childManagedByField(plan.Changes[0].Fields))
		})
	}
}

func TestCredentialManagedByDriftDoesNotRotateCredential(t *testing.T) {
	consumer := ownedAIGatewayObservation(t, testAIGatewayConsumer(nil), "team-a", false)
	consumer.ID = "consumer-id"
	credential := testAIGatewayConsumerCredential("key-id", "key", "Key", "create")
	credential.ManagedBy = map[string]string{"tool": "kongctl", "version": "older", "outside": "remove"}
	desired := testAIGatewayConsumerCredentialResource(t, "key", "Key", "create")
	api := &testAIGatewayConsumerAPI{
		consumers: []kkComps.AIGatewayConsumer{consumer}, credentials: []kkComps.AIGatewayConsumerCredential{credential},
	}
	p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayConsumersAPI: api}), slog.Default())
	p.resources = &resources.ResourceSet{
		AIGatewayConsumerCredentials: []resources.AIGatewayConsumerCredentialResource{desired},
	}
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
	ctx := context.WithValue(t.Context(), build.InfoKey, &build.Info{Version: "test-version"})
	require.NoError(t, p.planAIGatewayConsumerCredentialChanges(ctx, "team-a", "shared", "gateway-id",
		"consumer", "Consumer", "consumer-id", nil, p.resources.AIGatewayConsumerCredentials, plan))
	require.Empty(t, plan.Changes, "metadata housekeeping cannot invalidate an API key")
	require.Len(t, plan.Warnings, 1)
	require.Contains(t, plan.Warnings[0].Message, "managed_by drift")
	require.Contains(t, plan.Warnings[0].Message, "no replacement was planned for metadata alone")
	require.Contains(t, plan.Warnings[0].Message, "outside")
	require.Contains(t, plan.Warnings[0].Message, "test-version")
}
