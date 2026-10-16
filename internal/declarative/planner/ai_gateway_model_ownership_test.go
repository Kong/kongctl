package planner

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func federatedModel(t *testing.T, name, namespace, variant string) resources.AIGatewayModelResource {
	t.Helper()
	model := testAIGatewayModelResource(t)
	if variant == "api" {
		payload, err := model.PayloadMap()
		require.NoError(t, err)
		payload[FieldType] = "api"
		payload["capabilities"] = []string{"files"}
		data, err := json.Marshal(payload)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(data, &model))
	}
	model.Ref = name
	model.AIGateway = "shared"
	model.Kongctl = &resources.KongctlMeta{Namespace: new(namespace), Protected: new(false)}
	if model.AIGatewayModelAPI != nil {
		model.AIGatewayModelAPI.Name = name
	} else {
		model.AIGatewayModelModel.Name = name
	}
	return model
}

func observedFederatedModel(t *testing.T, model resources.AIGatewayModelResource) kkComps.AIGatewayModel {
	t.Helper()
	payload, err := model.MutablePayloadMap()
	require.NoError(t, err)
	payload[FieldManagedBy] = resources.BuildModelManagedBy(map[string]string{"other-tool": "retained"}, nil,
		resources.GetNamespace(model.Kongctl), model.Protected(), "dev")
	payload[FieldID] = model.Ref + "-id"
	payload["created_at"], payload["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var observed kkComps.AIGatewayModel
	require.NoError(t, json.Unmarshal(data, &observed))
	return observed
}

func federatedResourceSet(models ...resources.AIGatewayModelResource) *resources.ResourceSet {
	gateway := resources.AIGatewayResource{
		BaseResource: resources.BaseResource{Ref: "shared"}, External: &resources.ExternalBlock{ID: "gateway-id"},
	}
	gateway.SetKonnectID("gateway-id")
	scope := resources.NewSyncScope()
	scope.AddRoot(resources.ResourceTypeAIGateway)
	scope.AddChild(resources.ResourceTypeAIGateway, gateway.Ref, resources.ResourceTypeAIGatewayModel)
	for _, model := range models {
		scope.AddModelNamespace(gateway.Ref, resources.GetNamespace(model.Kongctl))
	}
	return &resources.ResourceSet{
		AIGateways:      []resources.AIGatewayResource{gateway},
		AIGatewayModels: models,
		SyncScope:       scope,
	}
}

func TestFederatedModelsIsolateSyncAndLastModelRemoval(t *testing.T) {
	for _, variant := range []string{"model", "api"} {
		t.Run(variant, func(t *testing.T) {
			a := federatedModel(t, "team-a-model", "team-a", variant)
			b := federatedModel(t, "team-b-model", "team-b", variant)
			unowned := observedFederatedModel(t, federatedModel(t, "legacy", "unused", variant))
			delete(resources.AIGatewayModelManagedBy(unowned), resources.ModelNamespaceKey)
			api := &ownershipModelAPI{providerOrderModelAPI: providerOrderModelAPI{
				testAIGatewayModelAPI: testAIGatewayModelAPI{models: []kkComps.AIGatewayModel{
					observedFederatedModel(t, a), observedFederatedModel(t, b), unowned,
				}},
			}}
			client := state.NewClient(
				state.ClientConfig{
					AIGatewayAPI:      &testAIGatewayAPI{gateways: []kkComps.AIGateway{testAIGateway()}},
					AIGatewayModelAPI: api,
				},
			)
			planner := NewPlanner(client, slog.Default())
			rs := federatedResourceSet(a, b)
			plan, err := planner.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
			require.NoError(t, err)
			require.Empty(t, plan.Changes)
			require.Equal(t, 1, api.lists, "complete observations are reused across namespaces")
			for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
				empty := federatedResourceSet()
				empty.SyncScope.AddModelNamespace("shared", "team-a")
				plan, err = planner.GeneratePlan(t.Context(), empty, Options{Mode: mode})
				require.NoError(t, err)
				if mode == PlanModeApply {
					require.Empty(t, plan.Changes)
				} else {
					require.Len(t, plan.Changes, 1)
					require.Equal(t, "team-a-model-id", plan.Changes[0].ResourceID)
					require.Equal(t, "team-a", plan.Changes[0].Namespace)
					require.Equal(t, ActionDelete, plan.Changes[0].Action)
				}
			}
			// Omitted model collections must preserve every model.
			rs = federatedResourceSet()
			rs.SyncScope = resources.NewSyncScope()
			rs.SyncScope.AddRoot(resources.ResourceTypeAIGateway)
			plan, err = planner.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
			require.NoError(t, err)
			require.Empty(t, plan.Changes)
		})
	}
}

func TestFederatedModelCollisionsRequireAdoptionOrTransfer(t *testing.T) {
	for _, owner := range []string{"", "team-b"} {
		t.Run("owner="+owner, func(t *testing.T) {
			model := federatedModel(t, "shared-name", "team-a", "model")
			observed := observedFederatedModel(t, model)
			resources.AIGatewayModelManagedBy(observed)[resources.ModelNamespaceKey] = owner
			api := &providerOrderModelAPI{
				testAIGatewayModelAPI: testAIGatewayModelAPI{models: []kkComps.AIGatewayModel{observed}},
			}
			planner := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayModelAPI: api}), slog.Default())
			plan, err := planner.GeneratePlan(t.Context(), federatedResourceSet(model), Options{Mode: PlanModeApply})
			require.Error(t, err)
			require.Nil(t, plan)
			if owner == "" {
				require.ErrorContains(t, err, "explicitly adopt")
			} else {
				require.ErrorContains(t, err, "explicit ownership transfer")
			}
		})
	}
}

func TestModelNamespacesDoNotScopeGatewayDeletion(t *testing.T) {
	model := federatedModel(t, "team-chat", "team-a", "model")
	parent := testAIGatewayResource()
	parent.Ref = "shared"
	parent.Kongctl.Namespace = new("platform")
	currentParent := testAIGateway()
	currentParent.Labels[labels.NamespaceKey] = "platform"
	unrelated := testAIGateway()
	unrelated.Name = "unrelated"
	unrelated.ID = "unrelated-id"
	unrelated.Labels = map[string]string{labels.NamespaceKey: "team-a"}
	api := &providerOrderModelAPI{testAIGatewayModelAPI: testAIGatewayModelAPI{
		models: []kkComps.AIGatewayModel{observedFederatedModel(t, model)},
	}}
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{currentParent, unrelated},
		}, AIGatewayModelAPI: api,
	})
	rs := federatedResourceSet(model)
	rs.AIGateways = []resources.AIGatewayResource{parent}
	rs.CaptureAIGatewayNamespaceScopes("team-a")
	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	require.Equal(t, []string{"platform"}, plan.Metadata.SyncScope.GatewayNamespaces)
}

func TestFederatedModelsPreserveMetadataAndRoundTripProtection(t *testing.T) {
	for _, variant := range []string{"model", "api"} {
		t.Run(variant, func(t *testing.T) {
			model := federatedModel(t, "owned", "team-a", variant)
			observed := observedFederatedModel(t, model)
			model.Kongctl.Protected = new(true)
			api := &providerOrderModelAPI{
				testAIGatewayModelAPI: testAIGatewayModelAPI{models: []kkComps.AIGatewayModel{observed}},
			}
			planner := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayModelAPI: api}), slog.Default())
			plan, err := planner.GeneratePlan(t.Context(), federatedResourceSet(model), Options{Mode: PlanModeApply})
			require.NoError(t, err)
			require.Len(t, plan.Changes, 1)
			metadata := plan.Changes[0].Fields[FieldManagedBy].(map[string]any)
			require.Equal(t, "retained", metadata["other-tool"])
			require.Equal(t, "true", metadata[resources.ModelProtectedKey])
			api.models[0] = observedFederatedModel(t, model)
			dumped, err := resources.AIGatewayModelResourceFromResponse("shared", api.models[0])
			require.NoError(t, err)
			encoded, err := json.Marshal(dumped)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(encoded, &dumped))
			require.NoError(t, dumped.Validate())
			require.True(t, dumped.Protected())
			require.NotContains(t, dumped.ManagedBy(), resources.ModelNamespaceKey)
			plan, err = planner.GeneratePlan(t.Context(), federatedResourceSet(dumped), Options{Mode: PlanModeApply})
			require.NoError(t, err)
			require.Empty(t, plan.Changes)
			empty := federatedResourceSet()
			empty.SyncScope.AddModelNamespace("shared", "team-a")
			plan, err = planner.GeneratePlan(t.Context(), empty, Options{Mode: PlanModeSync})
			require.ErrorContains(t, err, "protected")
			require.Nil(t, plan)
			model.Kongctl.Protected = new(false)
			plan, err = planner.GeneratePlan(t.Context(), federatedResourceSet(model), Options{Mode: PlanModeApply})
			require.NoError(t, err)
			require.Len(t, plan.Changes, 1)
			require.Equal(t, ProtectionChange{Old: true, New: false}, plan.Changes[0].Protection)
		})
	}
}

func TestFederatedModelPaginationFailsClosed(t *testing.T) {
	for _, failure := range []string{"", "error", "missing", "cycle", "invalid"} {
		t.Run(failure, func(t *testing.T) {
			a := federatedModel(t, "a", "team-a", "model")
			b := federatedModel(t, "b", "team-b", "model")
			api := &ownershipModelAPI{providerOrderModelAPI: providerOrderModelAPI{
				testAIGatewayModelAPI: testAIGatewayModelAPI{
					models: []kkComps.AIGatewayModel{observedFederatedModel(t, a), observedFederatedModel(t, b)},
				},
			}, paginated: true, failure: failure}
			rs := federatedResourceSet()
			rs.SyncScope.AddModelNamespace("shared", "team-b")
			planner := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayModelAPI: api}), slog.Default())
			plan, err := planner.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
			if failure != "" {
				require.Error(t, err)
				require.Nil(t, plan)
			} else {
				require.NoError(t, err)
				require.Len(t, plan.Changes, 1)
				require.Equal(t, "b-id", plan.Changes[0].ResourceID)
			}
			if failure == "invalid" {
				require.Equal(t, 1, api.lists)
			} else {
				require.Equal(t, 2, api.lists)
			}
		})
	}
}

type ownershipModelAPI struct {
	providerOrderModelAPI
	lists     int
	paginated bool
	failure   string
}

func (a *ownershipModelAPI) ListAiGatewayModels(
	ctx context.Context,
	req kkOps.ListAiGatewayModelsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayModelsResponse, error) {
	a.lists++
	if !a.paginated {
		return a.providerOrderModelAPI.ListAiGatewayModels(ctx, req, opts...)
	}
	if req.PageAfter != nil {
		switch a.failure {
		case "error":
			return nil, errors.New("page two failed")
		case "missing":
			return nil, nil
		}
	}
	resp, err := a.providerOrderModelAPI.ListAiGatewayModels(ctx, req, opts...)
	if err != nil {
		return nil, err
	}
	if req.PageAfter == nil || a.failure == "cycle" {
		resp.ListAIGatewayModelsResponse.Data = a.models[:1]
		resp.ListAIGatewayModelsResponse.Meta.Page.Next = new("https://example.test/models?page[after]=second")
		if a.failure == "invalid" {
			resp.ListAIGatewayModelsResponse.Meta.Page.Next = new("https://example.test/models")
		}
	} else {
		resp.ListAIGatewayModelsResponse.Data = a.models[1:]
	}
	return resp, nil
}

func TestDirectModelDeleteUsesModelOwnership(t *testing.T) {
	model := federatedModel(t, "owned", "team-a", "model")
	api := &providerOrderModelAPI{testAIGatewayModelAPI: testAIGatewayModelAPI{
		models: []kkComps.AIGatewayModel{observedFederatedModel(t, model)},
	}}
	planner := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayModelAPI: api}), slog.Default())
	plan, err := planner.GeneratePlan(t.Context(), federatedResourceSet(model), Options{Mode: PlanModeDelete})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ResourceTypeAIGatewayModel, plan.Changes[0].ResourceType)
	require.Equal(t, ActionDelete, plan.Changes[0].Action)
	require.Equal(t, "team-a", plan.Changes[0].Namespace)
	resources.AIGatewayModelManagedBy(api.models[0])[resources.ModelNamespaceKey] = "team-b"
	plan, err = planner.GeneratePlan(t.Context(), federatedResourceSet(model), Options{Mode: PlanModeDelete})
	require.ErrorContains(t, err, "ownership transfer")
	require.Nil(t, plan)
}

func TestGatewayAliasesReconcileOneCompleteModelCollection(t *testing.T) {
	a := federatedModel(t, "a", "team-a", "model")
	b := federatedModel(t, "b", "team-a", "api")
	b.AIGateway = "alias"
	rs := federatedResourceSet(a, b)
	alias := rs.AIGateways[0]
	alias.Ref = "alias"
	rs.AIGateways = append(rs.AIGateways, alias)
	rs.SyncScope.AddChild(resources.ResourceTypeAIGateway, "alias", resources.ResourceTypeAIGatewayModel)
	rs.SyncScope.AddModelNamespace("alias", "team-a")
	api := &ownershipModelAPI{providerOrderModelAPI: providerOrderModelAPI{
		testAIGatewayModelAPI: testAIGatewayModelAPI{models: []kkComps.AIGatewayModel{
			observedFederatedModel(t, a), observedFederatedModel(t, b),
		}},
	}}
	planner := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayModelAPI: api}), slog.Default())
	plan, err := planner.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	require.Equal(t, 1, api.lists)
	// Remote names are gateway-wide, even when parent refs and namespaces differ.
	b = federatedModel(t, "a", "team-b", "model")
	b.Ref, b.AIGateway = "other-ref", "alias"
	rs.AIGatewayModels[1] = b
	api.models = nil
	plan, err = planner.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.ErrorContains(t, err, "duplicate desired AI Gateway model name")
	require.Nil(t, plan)
}
