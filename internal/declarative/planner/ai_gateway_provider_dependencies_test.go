package planner

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayProviderModelSyncOrdering(t *testing.T) {
	for _, tt := range []struct {
		name                                                        string
		replace, retain, unscoped, plannedReference, inspectFailure bool
		wantError                                                   string
		embeddings                                                  bool
		unrelatedUpdate                                             bool
	}{
		{name: "delete model before provider"},
		{name: "delete semantic model before embedding provider", embeddings: true},
		{name: "replacement provider then model update then old provider", replace: true},
		{name: "retained model blocks deletion", retain: true, wantError: "still references it"},
		{name: "unrelated model update retains provider", unrelatedUpdate: true, wantError: "planned model"},
		{name: "unscoped model blocks deletion", unscoped: true, wantError: "still references it"},
		{name: "new model cannot reference deleted provider", plannedReference: true, wantError: "planned model"},
		{name: "failed observation blocks deletion", unscoped: true, inspectFailure: true, wantError: "failed to inspect"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			desiredModel := testAIGatewayModelResource(t)
			currentPayload, err := desiredModel.MutablePayloadMap()
			require.NoError(t, err)
			if tt.embeddings {
				currentPayload[FieldTargets].([]any)[0].(map[string]any)[FieldProvider] = "unrelated"
				currentPayload[FieldConfig].(map[string]any)[FieldBalancer] = map[string]any{
					"algorithm": "semantic",
					FieldEmbeddings: map[string]any{
						FieldProvider: "support-openai", FieldName: "embed-model",
						FieldConfig: map[string]any{FieldType: "openai"},
					},
					"vectordb": map[string]any{"type": "pgvector", "dimensions": 384, "distance_metric": "cosine"},
				}
			}
			currentPayload["id"] = "model-id"
			currentPayload["created_at"], currentPayload["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
			data, err := json.Marshal(currentPayload)
			require.NoError(t, err)
			var current kkComps.AIGatewayModel
			require.NoError(t, json.Unmarshal(data, &current))
			modelAPI := &providerOrderModelAPI{
				testAIGatewayModelAPI: testAIGatewayModelAPI{models: []kkComps.AIGatewayModel{current}},
			}
			if tt.plannedReference {
				modelAPI.models = nil
			}
			if tt.inspectFailure {
				modelAPI.listError = errors.New("observation unavailable")
			}

			rawProvider := map[string]any{
				"id": "provider-id", "name": "support-openai", "display_name": "OpenAI", "type": "openai",
				"config":     map[string]any{"auth": map[string]any{"type": "basic"}},
				"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
			}
			data, err = json.Marshal(rawProvider)
			require.NoError(t, err)
			var provider kkComps.AIGatewayModelProvider
			require.NoError(t, json.Unmarshal(data, &provider))
			scope := resources.NewSyncScope()
			scope.AddRoot(resources.ResourceTypeAIGateway)
			scope.AddChild(resources.ResourceTypeAIGateway, "support-gateway", resources.ResourceTypeAIGatewayProvider)
			if !tt.unscoped {
				scope.AddChild(resources.ResourceTypeAIGateway, "support-gateway", resources.ResourceTypeAIGatewayModel)
			}
			rs := &resources.ResourceSet{
				AIGateways: []resources.AIGatewayResource{testAIGatewayResource()}, SyncScope: scope,
			}
			if tt.replace {
				rs.AIGatewayProviders = []resources.AIGatewayProviderResource{{
					BaseResource: resources.BaseResource{Ref: "replacement-ref"},
					AIGateway:    "support-gateway", Name: "replacement", DisplayName: "Replacement", Type: "openai",
					Config: map[string]any{"auth": map[string]any{"type": "basic"}},
				}}
				payload, err := desiredModel.MutablePayloadMap()
				require.NoError(t, err)
				payload[FieldTargets].([]any)[0].(map[string]any)[FieldProvider] = "replacement"
				payload["ref"], payload["ai_gateway"] = desiredModel.Ref, desiredModel.AIGateway
				data, err = json.Marshal(payload)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(data, &desiredModel))
			}
			if tt.unrelatedUpdate {
				payload, err := desiredModel.MutablePayloadMap()
				require.NoError(t, err)
				payload[FieldDisplayName] = "Updated Model"
				payload["ref"], payload["ai_gateway"] = desiredModel.Ref, desiredModel.AIGateway
				data, err = json.Marshal(payload)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(data, &desiredModel))
			}
			if tt.replace || tt.retain || tt.plannedReference || tt.unrelatedUpdate {
				rs.AIGatewayModels = []resources.AIGatewayModelResource{desiredModel}
			}
			p := NewPlanner(state.NewClient(state.ClientConfig{
				AIGatewayAPI:          &testAIGatewayAPI{gateways: []kkComps.AIGateway{testAIGateway()}},
				AIGatewayProvidersAPI: &nameIdentityProviderAPI{children: []kkComps.AIGatewayModelProvider{provider}},
				AIGatewayModelAPI:     modelAPI,
			}), slog.Default())
			plan, err := p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				return
			}
			require.NoError(t, err)
			var deletion, model, creation PlannedChange
			for _, change := range plan.Changes {
				switch {
				case change.ResourceType == ResourceTypeAIGatewayProvider && change.Action == ActionDelete:
					deletion = change
				case change.ResourceType == ResourceTypeAIGatewayProvider && change.Action == ActionCreate:
					creation = change
				case change.ResourceType == ResourceTypeAIGatewayModel:
					model = change
				}
			}
			require.NotEmpty(t, deletion.ID)
			require.NotEmpty(t, model.ID)
			require.Contains(t, deletion.DependsOn, model.ID)
			require.Less(t, indexOf(plan.ExecutionOrder, model.ID), indexOf(plan.ExecutionOrder, deletion.ID))
			if tt.replace {
				require.Equal(t, ActionUpdate, model.Action)
				require.NotEmpty(t, creation.ID)
				require.Contains(t, model.DependsOn, creation.ID)
				require.Less(t, indexOf(plan.ExecutionOrder, creation.ID), indexOf(plan.ExecutionOrder, model.ID))
			} else {
				require.Equal(t, ActionDelete, model.Action)
			}
		})
	}
}

type providerOrderModelAPI struct {
	testAIGatewayModelAPI
	listError error
}

func (a *providerOrderModelAPI) ListAiGatewayModels(
	ctx context.Context, req kkOps.ListAiGatewayModelsRequest, opts ...kkOps.Option,
) (*kkOps.ListAiGatewayModelsResponse, error) {
	if a.listError != nil {
		return nil, a.listError
	}
	return a.testAIGatewayModelAPI.ListAiGatewayModels(ctx, req, opts...)
}

func (a *providerOrderModelAPI) GetAiGatewayModel(
	_ context.Context, _, id string, _ ...kkOps.Option,
) (*kkOps.GetAiGatewayModelResponse, error) {
	for _, model := range a.models {
		if resources.AIGatewayModelID(model) == id {
			return &kkOps.GetAiGatewayModelResponse{AIGatewayModel: &model}, nil
		}
	}
	return &kkOps.GetAiGatewayModelResponse{}, nil
}

func TestAIGatewaySerializationRespectsDependencies(t *testing.T) {
	changes := []PlannedChange{
		{
			ID: "delete-a-provider", ResourceType: ResourceTypeAIGatewayProvider, Action: ActionDelete,
			Parent: &ParentInfo{Ref: "gateway-a"}, DependsOn: []string{"update-a-model"},
		},
		{
			ID:           "create-a-provider",
			ResourceType: ResourceTypeAIGatewayProvider,
			Action:       ActionCreate,
			Parent:       &ParentInfo{Ref: "gateway-a"},
		},
		{
			ID: "update-a-model", ResourceType: ResourceTypeAIGatewayModel, Action: ActionUpdate,
			Parent: &ParentInfo{Ref: "gateway-a"}, DependsOn: []string{"create-a-provider"},
		},
		{
			ID: "delete-b-provider", ResourceType: ResourceTypeAIGatewayProvider, Action: ActionDelete,
			Parent: &ParentInfo{Ref: "gateway-b"}, DependsOn: []string{"delete-b-model"},
		},
		{
			ID:           "delete-b-model",
			ResourceType: ResourceTypeAIGatewayModel,
			Action:       ActionDelete,
			Parent:       &ParentInfo{Ref: "gateway-b"},
		},
	}
	result, err := NewDependencyResolver().ResolveDependenciesWithGroups(changes)
	require.NoError(t, err)
	require.Equal(t, [][]string{
		{"create-a-provider", "delete-b-model"}, {"delete-b-provider", "update-a-model"}, {"delete-a-provider"},
	}, result.ExecutionGroups)
	// Persisted serialization must be safe to resolve again.
	for i := range changes {
		changes[i].DependsOn = result.FullDepsMap[changes[i].ID]
	}
	again, err := NewDependencyResolver().ResolveDependenciesWithGroups(changes)
	require.NoError(t, err)
	require.Equal(t, result, again)
}

func TestAIGatewayProviderOrderingWithRuntimeDependencies(t *testing.T) {
	for _, downgrade := range []bool{false, true} {
		t.Run(map[bool]string{false: "upgrade", true: "downgrade"}[downgrade], func(t *testing.T) {
			plan := &Plan{Changes: []PlannedChange{
				{ID: "gateway", Action: ActionUpdate, ResourceType: ResourceTypeAIGateway},
				{
					ID: "delete-provider", Action: ActionDelete, ResourceType: ResourceTypeAIGatewayProvider,
					Parent: &ParentInfo{Ref: "gateway"}, DependsOn: []string{"update-model"},
				},
				{
					ID: "create-provider", Action: ActionCreate, ResourceType: ResourceTypeAIGatewayProvider,
					Parent: &ParentInfo{Ref: "gateway"},
				},
				{
					ID: "update-model", Action: ActionUpdate, ResourceType: ResourceTypeAIGatewayModel,
					Parent: &ParentInfo{Ref: "gateway"}, DependsOn: []string{"create-provider"},
				},
			}}
			// Runtime upgrades precede child writes; downgrades follow all children.
			if downgrade {
				plan.Changes[0].DependsOn = []string{"delete-provider", "create-provider", "update-model"}
			} else {
				plan.Changes[2].DependsOn = []string{"gateway"}
				plan.Changes[3].DependsOn = append(plan.Changes[3].DependsOn, "gateway")
			}
			result, err := NewDependencyResolver().ResolveDependenciesWithGroups(plan.Changes)
			require.NoError(t, err)
			expected := []string{"gateway", "create-provider", "update-model", "delete-provider"}
			if downgrade {
				expected = []string{"create-provider", "update-model", "delete-provider", "gateway"}
			}
			require.Equal(t, expected, result.ExecutionOrder)
		})
	}
}

func TestAIGatewayModelProviderReferencesIncludeEmbeddings(t *testing.T) {
	payload := map[string]any{
		FieldTargets: []any{map[string]any{FieldProvider: "target"}, map[string]any{FieldProvider: "target"}},
		FieldConfig: map[string]any{FieldBalancer: map[string]any{
			FieldEmbeddings: map[string]any{FieldProvider: "embedding"},
		}},
	}
	require.Equal(t, []string{"target", "embedding"}, aiGatewayModelProviderNames(payload))
	desired := testAIGatewayModelResource(t)
	data, err := json.Marshal(desired)
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	raw[FieldConfig].(map[string]any)[FieldBalancer] = map[string]any{
		"algorithm": "semantic",
		FieldEmbeddings: map[string]any{
			FieldProvider: "embedding", FieldName: "embed-model",
			FieldConfig: map[string]any{FieldType: "openai"},
		},
		"vectordb": map[string]any{"type": "pgvector", "dimensions": 384, "distance_metric": "cosine"},
	}
	data, err = json.Marshal(raw)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &desired))
	require.Contains(
		t,
		aiGatewayModelProviderCreateDependencies(desired, map[string]string{"embedding": "create-embedding"}),
		"create-embedding",
	)
}
