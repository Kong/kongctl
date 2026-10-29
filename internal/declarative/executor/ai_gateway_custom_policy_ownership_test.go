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

type customPolicyOwnershipAPI struct {
	helpers.AIGatewayCustomPoliciesAPI
	definition       kkComps.AIGatewayCustomPolicy
	updates, deletes int
	last             kkComps.UpdateAIGatewayCustomPolicyRequest
}

func (a *customPolicyOwnershipAPI) GetAiGatewayCustomPolicy(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetAiGatewayCustomPolicyResponse, error) {
	return &kkOps.GetAiGatewayCustomPolicyResponse{AIGatewayCustomPolicy: &a.definition}, nil
}

func (a *customPolicyOwnershipAPI) ListAiGatewayCustomPolicies(
	context.Context,
	kkOps.ListAiGatewayCustomPoliciesRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayCustomPoliciesResponse, error) {
	return &kkOps.ListAiGatewayCustomPoliciesResponse{
		ListAIGatewayCustomPoliciesResponse: &kkComps.ListAIGatewayCustomPoliciesResponse{
			Data: []kkComps.AIGatewayCustomPolicy{a.definition},
		},
	}, nil
}

func (a *customPolicyOwnershipAPI) UpdateAiGatewayCustomPolicy(
	_ context.Context,
	r kkOps.UpdateAiGatewayCustomPolicyRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateAiGatewayCustomPolicyResponse, error) {
	a.updates++
	a.last = r.UpdateAIGatewayCustomPolicyRequest
	return &kkOps.UpdateAiGatewayCustomPolicyResponse{AIGatewayCustomPolicy: &a.definition}, nil
}

func (a *customPolicyOwnershipAPI) DeleteAiGatewayCustomPolicy(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayCustomPolicyResponse, error) {
	a.deletes++
	return &kkOps.DeleteAiGatewayCustomPolicyResponse{}, nil
}

func TestCustomPolicyWritesRevalidateSavedOwnershipAndProtection(t *testing.T) {
	for _, variant := range []string{"installed", "streaming"} {
		for _, owner := range []string{"team-a", "team-b", ""} {
			for _, protected := range []bool{false, true} {
				t.Run(variant+"/"+owner+"/protected="+map[bool]string{false: "false", true: "true"}[protected], func(t *testing.T) {
					fields := map[string]any{
						planner.FieldName: "shared-custom", planner.FieldType: "installed", planner.FieldDisplayName: "Shared custom",
						planner.FieldSchema:    "return {}",
						planner.FieldManagedBy: resources.BuildChildManagedBy("team-a", false, "dev"),
					}
					fields[planner.FieldType] = variant
					if variant == "streaming" {
						fields[planner.FieldHandler] = "return { VERSION = '1.0', PRIORITY = 1000 }"
					}

					change := planner.PlannedChange{
						ResourceType: planner.ResourceTypeAIGatewayCustomPolicy,
						Namespace:    "team-a",
						ResourceID:   "definition-id",
						Fields:       fields,
						Parent:       &planner.ParentInfo{ID: "gateway-id"},
					}
					encoded, err := json.Marshal(change)
					require.NoError(t, err)
					var saved planner.PlannedChange
					require.NoError(t, json.Unmarshal(encoded, &saved))
					observed := map[string]any{}
					maps.Copy(observed, fields)
					observed[planner.FieldLabels] = map[string]string{"added-after-planning": "retained"}
					observed[planner.FieldManagedBy] = resources.BuildChildManagedBy(
						owner,
						protected,
						"dev",
					)
					observed[planner.FieldID] = "definition-id"
					observed["created_at"] = "2026-01-01T00:00:00Z"
					observed["updated_at"] = "2026-01-01T00:00:00Z"
					data, err := json.Marshal(observed)
					require.NoError(t, err)
					api := &customPolicyOwnershipAPI{}
					require.NoError(t, json.Unmarshal(data, &api.definition))
					adapter := NewAIGatewayCustomPolicyAdapter(state.NewClient(state.ClientConfig{AIGatewayCustomPoliciesAPI: api}))
					ctx := NewExecutionContext(&saved)
					var req kkComps.UpdateAIGatewayCustomPolicyRequest
					require.NoError(t, adapter.MapUpdateFields(t.Context(), ctx, saved.Fields, &req, nil))
					_, err = adapter.Update(t.Context(), "definition-id", req, "team-a", ctx)
					if owner == "team-a" && !protected {
						require.NoError(t, err)
						require.Equal(t, 1, api.updates)
						p, err := ownedChildPayload(api.last)
						require.NoError(t, err)
						require.NotContains(t, childPayloadManagedBy(p), "other-tool")
						require.Equal(t, map[string]any{"added-after-planning": "retained"}, p[planner.FieldLabels])
					} else {
						require.Error(t, err)
						require.Zero(t, api.updates)
					}
					err = adapter.Delete(t.Context(), "definition-id", ctx)
					if owner == "team-a" && !protected {
						require.NoError(t, err)
					} else {
						require.Error(t, err)
						require.Zero(t, api.deletes)
					}
					var create kkComps.CreateAIGatewayCustomPolicyRequest
					require.NoError(t, adapter.MapCreateFields(t.Context(), ctx, saved.Fields, &create))
					_, err = adapter.Create(t.Context(), create, "team-a", ctx)
					require.ErrorContains(t, err, "already exists")
				})
			}
		}
	}
}

func TestCustomPolicyUnprotectRequiresAnUnchangedDefinition(t *testing.T) {
	for _, variant := range []string{"installed", "streaming"} {
		for _, mutation := range []string{"none", "schema", "handler", "display-name", "labels", "metadata", "name", "type"} {
			if variant == "installed" && mutation == "handler" {
				continue
			}
			t.Run(variant+"/"+mutation, func(t *testing.T) {
				metadata := resources.BuildChildManagedBy("team-a", true, "dev")
				observed := map[string]any{
					planner.FieldID: "definition-id", planner.FieldName: "definition", planner.FieldType: variant,
					planner.FieldDisplayName: "Definition", planner.FieldSchema: "return {}", planner.FieldManagedBy: metadata,
					planner.FieldLabels: map[string]string{"fresh": "retained"},
					"created_at":        "2026-01-01T00:00:00Z",
					"updated_at":        "2026-01-01T00:00:00Z",
				}
				if variant == "streaming" {
					observed[planner.FieldHandler] = "return { VERSION = '1.0', PRIORITY = 1000 }"
				}
				data, err := json.Marshal(observed)
				require.NoError(t, err)
				api := &customPolicyOwnershipAPI{}
				require.NoError(t, json.Unmarshal(data, &api.definition))
				fields, err := resources.AIGatewayCustomPolicyMutablePayloadMap(api.definition)
				require.NoError(t, err)
				delete(fields, planner.FieldLabels)
				requested := resources.BuildChildManagedBy("team-a", false, "dev")
				fields[planner.FieldManagedBy] = requested
				change := planner.PlannedChange{
					Namespace: "team-a", Fields: fields, Parent: &planner.ParentInfo{ID: "gateway-id"},
					Protection:    planner.ProtectionChange{Old: true, New: false},
					ChangedFields: map[string]planner.FieldChange{planner.FieldManagedBy: {Old: metadata, New: requested}},
				}
				data, err = json.Marshal(change)
				require.NoError(t, err)
				var saved planner.PlannedChange
				require.NoError(t, json.Unmarshal(data, &saved))
				adapter := NewAIGatewayCustomPolicyAdapter(state.NewClient(state.ClientConfig{AIGatewayCustomPoliciesAPI: api}))
				ctx := NewExecutionContext(&saved)
				var req kkComps.UpdateAIGatewayCustomPolicyRequest
				require.NoError(t, adapter.MapUpdateFields(t.Context(), ctx, saved.Fields, &req, nil))
				payload, err := ownedChildPayload(req)
				require.NoError(t, err)
				switch mutation {
				case "schema":
					payload[planner.FieldSchema] = "return { fields = {} }"
				case "handler":
					payload[planner.FieldHandler] = "return { VERSION = '2.0', PRIORITY = 1000 }"
				case "display-name":
					payload[planner.FieldDisplayName] = "Changed"
				case "labels":
					payload[planner.FieldLabels] = map[string]string{"mutated": "label"}
				case "metadata":
					requested["inventory"] = "changed"
					payload[planner.FieldManagedBy] = requested
				case "name":
					payload[planner.FieldName] = "renamed"
				case "type":
					if variant == "installed" {
						payload[planner.FieldType] = "streaming"
						payload[planner.FieldHandler] = "return {}"
					} else {
						payload[planner.FieldType] = "installed"
						delete(payload, planner.FieldHandler)
					}
				}
				require.NoError(t, mapAIGatewaySDKRequest("custom policy update", payload, &req))
				_, err = adapter.Update(t.Context(), "definition-id", req, "team-a", ctx)
				if mutation != "none" && mutation != "metadata" {
					require.Error(t, err)
					require.Zero(t, api.updates)
					return
				}
				require.NoError(t, err)
				result, err := ownedChildPayload(api.last)
				require.NoError(t, err)
				require.Equal(t, observed[planner.FieldSchema], result[planner.FieldSchema])
				require.Equal(t, observed[planner.FieldHandler], result[planner.FieldHandler])
				require.Equal(t, map[string]any{"fresh": "retained"}, result[planner.FieldLabels])
				require.Equal(t, "false", childPayloadManagedBy(result)[resources.ChildProtectedKey])
			})
		}
	}
}
