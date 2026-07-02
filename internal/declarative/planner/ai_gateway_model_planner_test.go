package planner

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayModelPlannerCreatesChildForExistingGateway(t *testing.T) {
	model := testAIGatewayModelResource(t)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayModelAPI: &testAIGatewayModelAPI{},
	})
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{
				Ref:     "support-gateway",
				Kongctl: &resources.KongctlMeta{Namespace: new("default")},
			},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				Name:        "support-gateway",
				DisplayName: "Support Gateway",
			},
		}},
		AIGatewayModels: []resources.AIGatewayModelResource{model},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionCreate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayModel, change.ResourceType)
	require.Equal(t, "support-gpt", change.ResourceRef)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
	require.Equal(t, "support-gateway", change.Parent.Ref)
	require.Equal(t, "model", change.Fields[FieldType])
}

func TestAIGatewayModelPlannerCreatesChildForExternalGatewayRef(t *testing.T) {
	model := testAIGatewayModelResource(t)
	model.AIGateway = tags.RefPlaceholderPrefix + "external-support-gateway#id"
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{{
				ID:          "external-gateway-id",
				Name:        "external-gateway",
				DisplayName: "External Support Gateway",
			}},
		},
		AIGatewayModelAPI: &testAIGatewayModelAPI{},
	})
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{Ref: "external-support-gateway"},
			External: &resources.ExternalBlock{
				Selector: &resources.ExternalSelector{
					MatchFields: map[string]string{FieldDisplayName: "External Support Gateway"},
				},
			},
		}},
		AIGatewayModels: []resources.AIGatewayModelResource{model},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionCreate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayModel, change.ResourceType)
	require.Equal(t, "support-gpt", change.ResourceRef)
	require.NotNil(t, change.Parent)
	require.Equal(t, "external-gateway-id", change.Parent.ID)
	require.Equal(t, "external-support-gateway", change.Parent.Ref)
}

func TestAIGatewayModelPlannerSyncDeletesScopedModels(t *testing.T) {
	scope := resources.NewSyncScope()
	scope.AddRoot(resources.ResourceTypeAIGateway)
	scope.AddChild(resources.ResourceTypeAIGateway, "support-gateway", resources.ResourceTypeAIGatewayModel)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayModelAPI: &testAIGatewayModelAPI{
			models: []kkComps.AIGatewayModel{testAIGatewayModel("model-id", "support-gpt")},
		},
	})
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{
				Ref:     "support-gateway",
				Kongctl: &resources.KongctlMeta{Namespace: new("default")},
			},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				Name:        "support-gateway",
				DisplayName: "Support Gateway",
			},
		}},
		SyncScope: scope,
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionDelete, change.Action)
	require.Equal(t, ResourceTypeAIGatewayModel, change.ResourceType)
	require.Equal(t, "model-id", change.ResourceID)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
}

func TestAIGatewayModelPlannerIgnoresAPIDefaults(t *testing.T) {
	model := testAIGatewayModelResource(t)
	var current kkComps.AIGatewayModel
	require.NoError(t, json.Unmarshal([]byte(`{
		"id": "model-id",
		"type": "model",
		"name": "support-gpt",
		"display_name": "Support GPT",
		"enabled": true,
		"config": {
			"route": {
				"https_redirect_status_code": 426,
				"preserve_host": false,
				"protocols": ["http", "https"],
				"regex_priority": 0,
				"request_buffering": true,
				"response_buffering": true,
				"strip_path": true
			},
			"model": {
				"alias": "support-gpt",
				"name_header": true
			},
			"response_streaming": "allow",
			"max_request_body_size": 8388608
		},
		"formats": [{"type": "openai"}],
		"targets": [{
			"name": "gpt-4o",
			"provider": "support-openai",
			"weight": 100,
			"allow_auth_override": false,
			"config": {"type": "openai"}
		}],
		"policies": [],
		"capabilities": ["generate"],
		"created_at": "2026-01-01T00:00:00Z",
		"updated_at": "2026-01-01T00:00:00Z"
	}`), &current))

	needsUpdate, fields, changed, err := (&Planner{}).shouldUpdateAIGatewayModel(
		state.AIGatewayModel{AIGatewayModel: current},
		model,
	)

	require.NoError(t, err)
	require.Falsef(t, needsUpdate, "changed fields: %#v", changed)
	require.Nil(t, fields)
	require.Nil(t, changed)
}

func TestAIGatewayModelPlannerDependsOnTargetProviderCreate(t *testing.T) {
	model := testAIGatewayModelResource(t)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{},
	})
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{
				Ref:     "support-gateway",
				Kongctl: &resources.KongctlMeta{Namespace: new("default")},
			},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				Name:        "support-gateway",
				DisplayName: "Support Gateway",
			},
		}},
		AIGatewayProviders: []resources.AIGatewayProviderResource{{
			BaseResource: resources.BaseResource{Ref: "support-openai"},
			AIGateway:    "support-gateway",
			Name:         "support-openai",
			Type:         "openai",
			DisplayName:  "Support OpenAI",
			Config: map[string]any{
				"auth": map[string]any{"type": "basic"},
			},
		}},
		AIGatewayModels: []resources.AIGatewayModelResource{model},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)

	gatewayCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGateway, "support-gateway")
	providerCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGatewayProvider, "support-openai")
	modelCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGatewayModel, "support-gpt")

	require.Contains(t, providerCreate.DependsOn, gatewayCreate.ID)
	require.Contains(t, modelCreate.DependsOn, providerCreate.ID)
	require.Len(t, plan.ExecutionGroups, 3)
	require.Contains(t, plan.ExecutionGroups[0], gatewayCreate.ID)
	require.Contains(t, plan.ExecutionGroups[1], providerCreate.ID)
	require.Contains(t, plan.ExecutionGroups[2], modelCreate.ID)
}

func findAIGatewayModelTestChange(
	t *testing.T,
	plan *Plan,
	resourceType string,
	resourceRef string,
) PlannedChange {
	t.Helper()
	for _, change := range plan.Changes {
		if change.ResourceType == resourceType && change.ResourceRef == resourceRef {
			return change
		}
	}
	t.Fatalf("change %s %s not found", resourceType, resourceRef)
	return PlannedChange{}
}

func testAIGatewayModelResource(t *testing.T) resources.AIGatewayModelResource {
	t.Helper()
	payload := `{
		"ref": "support-gpt",
		"ai_gateway": "support-gateway",
		"type": "model",
		"name": "support-gpt",
		"display_name": "Support GPT",
		"enabled": true,
		"config": {"route": {}, "model": {}},
		"formats": [{"type": "openai"}],
		"target_models": [{"name": "gpt-4o", "provider": "support-openai", "config": {"type": "openai"}}],
		"policies": [],
		"capabilities": ["generate"]
	}`
	var model resources.AIGatewayModelResource
	require.NoError(t, json.Unmarshal([]byte(payload), &model))
	return model
}

func testAIGateway() kkComps.AIGateway {
	const (
		id          = "gateway-id"
		displayName = "Support Gateway"
	)
	return kkComps.AIGateway{
		ID:          id,
		Name:        "support-gateway",
		DisplayName: displayName,
		Labels: map[string]string{
			labels.NamespaceKey: "default",
		},
	}
}

func testAIGatewayModel(id string, name string) kkComps.AIGatewayModel {
	return kkComps.AIGatewayModel{
		Type: kkComps.AIGatewayModelTypeModel,
		AIGatewayModelAIGatewayModelModel: &kkComps.AIGatewayModelAIGatewayModelModel{
			ID:          id,
			Name:        name,
			DisplayName: name,
			Type:        kkComps.AIGatewayModelModelAIGatewayModelTypeModel,
		},
	}
}

type testAIGatewayAPI struct {
	gateways []kkComps.AIGateway
}

func (t *testAIGatewayAPI) ListAiGateways(
	_ context.Context,
	_ *int64,
	_ *int64,
	_ ...kkOps.Option,
) (*kkOps.ListAiGatewaysResponse, error) {
	return &kkOps.ListAiGatewaysResponse{
		ListAIGatewaysResponse: &kkComps.ListAIGatewaysResponse{
			Data: t.gateways,
			Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: float64(len(t.gateways))}},
		},
	}, nil
}

func (t *testAIGatewayAPI) CreateAiGateway(
	context.Context,
	kkComps.CreateAIGatewayRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayResponse, error) {
	return nil, nil
}

func (t *testAIGatewayAPI) GetAiGateway(
	_ context.Context,
	gatewayID string,
	_ ...kkOps.Option,
) (*kkOps.GetAiGatewayResponse, error) {
	for _, gateway := range t.gateways {
		if gateway.ID == gatewayID {
			return &kkOps.GetAiGatewayResponse{AIGateway: &gateway}, nil
		}
	}
	return &kkOps.GetAiGatewayResponse{}, nil
}

func (t *testAIGatewayAPI) UpdateAiGateway(
	context.Context,
	string,
	kkComps.UpdateAIGatewayRequest,
	...kkOps.Option,
) (*kkOps.UpdateAiGatewayResponse, error) {
	return nil, nil
}

func (t *testAIGatewayAPI) DeleteAiGateway(
	context.Context,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayResponse, error) {
	return nil, nil
}

type testAIGatewayModelAPI struct {
	models []kkComps.AIGatewayModel
}

func (t *testAIGatewayModelAPI) ListAiGatewayModels(
	context.Context,
	kkOps.ListAiGatewayModelsRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayModelsResponse, error) {
	return &kkOps.ListAiGatewayModelsResponse{
		ListAIGatewayModelsResponse: &kkComps.ListAIGatewayModelsResponse{
			Data: t.models,
		},
	}, nil
}

func (t *testAIGatewayModelAPI) CreateAiGatewayModel(
	context.Context,
	string,
	kkComps.CreateAIGatewayModelRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayModelResponse, error) {
	return nil, nil
}

func (t *testAIGatewayModelAPI) GetAiGatewayModel(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetAiGatewayModelResponse, error) {
	return nil, nil
}

func (t *testAIGatewayModelAPI) UpdateAiGatewayModel(
	context.Context,
	kkOps.UpdateAiGatewayModelRequest,
	...kkOps.Option,
) (*kkOps.UpdateAiGatewayModelResponse, error) {
	return nil, nil
}

func (t *testAIGatewayModelAPI) DeleteAiGatewayModel(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayModelResponse, error) {
	return nil, nil
}
