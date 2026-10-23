package executor

import (
	"context"
	"encoding/json"
	"maps"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

type providerOwnershipAPI struct {
	helpers.AIGatewayProvidersAPI
	provider         kkComps.AIGatewayModelProvider
	updates, deletes int
	last             kkComps.UpdateAIGatewayModelProviderRequest
}

func (a *providerOwnershipAPI) GetAiGatewayProvider(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetAiGatewayModelProviderResponse, error) {
	return &kkOps.GetAiGatewayModelProviderResponse{AIGatewayModelProvider: &a.provider}, nil
}

func (a *providerOwnershipAPI) ListAiGatewayProviders(
	context.Context,
	kkOps.ListAiGatewayModelProvidersRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayModelProvidersResponse, error) {
	return &kkOps.ListAiGatewayModelProvidersResponse{
		ListAIGatewayModelProvidersResponse: &kkComps.ListAIGatewayModelProvidersResponse{
			Data: []kkComps.AIGatewayModelProvider{a.provider},
		},
	}, nil
}

func (a *providerOwnershipAPI) UpdateAiGatewayProvider(
	_ context.Context,
	r kkOps.UpdateAiGatewayModelProviderRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateAiGatewayModelProviderResponse, error) {
	a.updates++
	a.last = r.UpdateAIGatewayModelProviderRequest
	return &kkOps.UpdateAiGatewayModelProviderResponse{AIGatewayModelProvider: &a.provider}, nil
}

func (a *providerOwnershipAPI) DeleteAiGatewayProvider(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayModelProviderResponse, error) {
	a.deletes++
	return &kkOps.DeleteAiGatewayModelProviderResponse{}, nil
}

func TestProviderWritesRevalidateSavedOwnershipAndProtection(t *testing.T) {
	for _, owner := range []string{"team-a", "team-b", ""} {
		for _, protected := range []bool{false, true} {
			t.Run(owner+"/protected="+map[bool]string{false: "false", true: "true"}[protected], func(t *testing.T) {
				fields := map[string]any{
					planner.FieldName: "openai", planner.FieldType: "openai", planner.FieldDisplayName: "OpenAI",
					planner.FieldConfig: map[string]any{
						"auth": map[string]any{
							"type":    "basic",
							"headers": []any{map[string]any{"name": "Authorization"}},
						},
					},
					planner.FieldManagedBy: resources.BuildChildManagedBy(nil, nil, "team-a", false, "dev"),
				}
				change := planner.PlannedChange{
					ResourceType: planner.ResourceTypeAIGatewayProvider,
					Namespace:    "team-a",
					ResourceID:   "provider-id",
					Fields:       fields,
					Parent:       &planner.ParentInfo{ID: "gateway-id"},
				}
				encoded, err := json.Marshal(change)
				require.NoError(t, err)
				var saved planner.PlannedChange
				require.NoError(t, json.Unmarshal(encoded, &saved))
				observed := map[string]any{}
				maps.Copy(observed, fields)
				observed[planner.FieldManagedBy] = resources.BuildChildManagedBy(
					nil,
					map[string]string{"other-tool": "preserved"},
					owner,
					protected,
					"dev",
				)
				observed[planner.FieldID] = "provider-id"
				observed["created_at"] = "2026-01-01T00:00:00Z"
				observed["updated_at"] = "2026-01-01T00:00:00Z"
				data, err := json.Marshal(observed)
				require.NoError(t, err)
				api := &providerOwnershipAPI{}
				require.NoError(t, json.Unmarshal(data, &api.provider))
				adapter := NewAIGatewayProviderAdapter(state.NewClient(state.ClientConfig{AIGatewayProvidersAPI: api}))
				ctx := NewExecutionContext(&saved)
				var req kkComps.UpdateAIGatewayModelProviderRequest
				require.NoError(t, adapter.MapUpdateFields(t.Context(), ctx, saved.Fields, &req, nil))
				_, err = adapter.Update(t.Context(), "provider-id", req, "team-a", ctx)
				if owner == "team-a" && !protected {
					require.NoError(t, err)
					require.Equal(t, 1, api.updates)
					p, err := providerRequestPayload(api.last)
					require.NoError(t, err)
					require.Equal(t, "preserved", providerPayloadMetadata(p)["other-tool"])
				} else {
					require.Error(t, err)
					require.Zero(t, api.updates)
				}
				err = adapter.Delete(t.Context(), "provider-id", ctx)
				if owner == "team-a" && !protected {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.Zero(t, api.deletes)
				}
				var create kkComps.CreateAIGatewayModelProviderRequest
				require.NoError(t, adapter.MapCreateFields(t.Context(), ctx, saved.Fields, &create))
				_, err = adapter.Create(t.Context(), create, "team-a", ctx)
				require.ErrorContains(t, err, "already exists")
			})
		}
	}
}

func TestProviderUnprotectAllowsOnlyAnUnchangedPublicPayload(t *testing.T) {
	for _, mutation := range []string{"none", "header-name", "secret-value"} {
		t.Run(mutation, func(t *testing.T) {
			var observed kkComps.AIGatewayModelProvider
			require.NoError(t, json.Unmarshal([]byte(`{
"id":"provider-id","name":"openai","display_name":"OpenAI","type":"openai",
"managed_by":{"kongctl-namespace":"team-a","kongctl-protected":"true"},
"config":{"auth":{"type":"basic","headers":[{"name":"Authorization"}]}},
"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"
}`), &observed))
			current, err := state.NormalizeAIGatewayProvider(observed)
			require.NoError(t, err)
			metadata := resources.BuildChildManagedBy(current.ManagedBy, nil, "team-a", false, "dev")
			fields := map[string]any{
				planner.FieldName: current.Name, planner.FieldType: current.Type,
				planner.FieldDisplayName: current.DisplayName, planner.FieldConfig: current.Config,
				planner.FieldManagedBy: metadata,
			}
			change := planner.PlannedChange{
				Namespace: "team-a", Fields: fields, Parent: &planner.ParentInfo{ID: "gateway-id"},
				ChangedFields: map[string]planner.FieldChange{
					planner.FieldManagedBy: {Old: current.ManagedBy, New: metadata},
				},
				Protection: planner.ProtectionChange{Old: true, New: false},
			}
			api := &providerOwnershipAPI{provider: observed}
			adapter := NewAIGatewayProviderAdapter(state.NewClient(state.ClientConfig{AIGatewayProvidersAPI: api}))
			execCtx := NewExecutionContext(&change)
			var req kkComps.UpdateAIGatewayModelProviderRequest
			require.NoError(t, adapter.MapUpdateFields(t.Context(), execCtx, fields, &req, nil))
			if mutation == "header-name" {
				req.AIGatewayModelProviderOpenai.Config.Auth.Headers[0].Name = "X-Other"
			}
			if mutation == "secret-value" {
				req.AIGatewayModelProviderOpenai.Config.Auth.Headers[0].Value = new("synthetic-secret")
			}
			_, err = adapter.Update(t.Context(), "provider-id", req, "team-a", execCtx)
			if mutation == "none" {
				require.NoError(t, err)
				require.Equal(t, 1, api.updates)
			} else {
				require.ErrorContains(t, err, "protected")
				require.Zero(t, api.updates)
			}
		})
	}
}
