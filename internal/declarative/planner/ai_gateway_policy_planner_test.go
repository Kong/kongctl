package planner

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayPolicyPlannerCreatesChildForExistingGateway(t *testing.T) {
	policy := testAIGatewayPolicyResource(t)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayPoliciesAPI: &testAIGatewayPolicyAPI{},
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
		AIGatewayPolicies: []resources.AIGatewayPolicyResource{policy},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionCreate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayPolicy, change.ResourceType)
	require.Equal(t, "mask-sensitive-data", change.ResourceRef)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
	require.Equal(t, "support-gateway", change.Parent.Ref)
	require.Equal(t, "ai-sanitizer", change.Fields[FieldType])
}

func TestAIGatewayPolicyPlannerUpdatesExistingPolicy(t *testing.T) {
	policy := testAIGatewayPolicyResource(t)
	policy.DisplayName = "Mask Sensitive Data Updated"
	current := testAIGatewayPolicy("policy-id", "mask-sensitive-data")
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayPoliciesAPI: &testAIGatewayPolicyAPI{
			policies: []kkComps.AIGatewayPolicy{current},
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
		AIGatewayPolicies: []resources.AIGatewayPolicyResource{policy},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionUpdate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayPolicy, change.ResourceType)
	require.Equal(t, "policy-id", change.ResourceID)
	require.Equal(t, "Mask Sensitive Data Updated", change.Fields[FieldDisplayName])
	require.Contains(t, change.ChangedFields, FieldDisplayName)
}

func TestAIGatewayPolicyPlannerSyncDeletesScopedPolicies(t *testing.T) {
	scope := resources.NewSyncScope()
	scope.AddRoot(resources.ResourceTypeAIGateway)
	scope.AddChild(resources.ResourceTypeAIGateway, "support-gateway", resources.ResourceTypeAIGatewayPolicy)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayPoliciesAPI: &testAIGatewayPolicyAPI{
			policies: []kkComps.AIGatewayPolicy{testAIGatewayPolicy("policy-id", "mask-sensitive-data")},
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
	require.Equal(t, ResourceTypeAIGatewayPolicy, change.ResourceType)
	require.Equal(t, "policy-id", change.ResourceID)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
}

func TestAIGatewayModelPlannerDependsOnPolicyCreate(t *testing.T) {
	policy := testAIGatewayPolicyResource(t)
	model := testAIGatewayModelResourceWithPolicy(t, "mask-sensitive-data")
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
		AIGatewayPolicies: []resources.AIGatewayPolicyResource{policy},
		AIGatewayModels:   []resources.AIGatewayModelResource{model},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)

	gatewayCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGateway, "support-gateway")
	policyCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGatewayPolicy, "mask-sensitive-data")
	modelCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGatewayModel, "support-gpt")

	require.Contains(t, policyCreate.DependsOn, gatewayCreate.ID)
	require.Contains(t, modelCreate.DependsOn, policyCreate.ID)
}

func testAIGatewayPolicyResource(t *testing.T) resources.AIGatewayPolicyResource {
	t.Helper()
	payload := `{
		"ref": "mask-sensitive-data",
		"ai_gateway": "support-gateway",
		"type": "ai-sanitizer",
		"name": "mask-sensitive-data",
		"display_name": "Mask Sensitive Data",
		"enabled": true,
		"global": false,
		"config": {"anonymize": ["email"]}
	}`
	var policy resources.AIGatewayPolicyResource
	require.NoError(t, json.Unmarshal([]byte(payload), &policy))
	return policy
}

func testAIGatewayModelResourceWithPolicy(t *testing.T, policyName string) resources.AIGatewayModelResource {
	t.Helper()
	payload := strings.Replace(
		`{
		"ref": "support-gpt",
		"ai_gateway": "support-gateway",
		"type": "model",
		"name": "support-gpt",
		"display_name": "Support GPT",
		"enabled": true,
		"config": {"route": {}, "model": {}},
		"formats": [{"type": "openai"}],
		"target_models": [{"name": "gpt-4o", "provider": "support-openai", "config": {"type": "openai"}}],
		"policies": ["POLICY_NAME"],
		"capabilities": ["generate"]
	}`,
		"POLICY_NAME",
		policyName,
		1,
	)
	var model resources.AIGatewayModelResource
	require.NoError(t, json.Unmarshal([]byte(payload), &model))
	return model
}

func testAIGatewayPolicy(id string, name string) kkComps.AIGatewayPolicy {
	enabled := true
	global := false
	return kkComps.AIGatewayPolicy{
		ID:          id,
		Name:        name,
		Type:        "ai-sanitizer",
		DisplayName: "Mask Sensitive Data",
		Enabled:     &enabled,
		Global:      &global,
		Config:      map[string]any{"anonymize": []any{"email"}},
	}
}

type testAIGatewayPolicyAPI struct {
	policies []kkComps.AIGatewayPolicy
}

func (t *testAIGatewayPolicyAPI) ListAiGatewayPolicies(
	context.Context,
	kkOps.ListAiGatewayPoliciesRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayPoliciesResponse, error) {
	return &kkOps.ListAiGatewayPoliciesResponse{
		ListAIGatewayPoliciesResponse: &kkComps.ListAIGatewayPoliciesResponse{
			Data: t.policies,
		},
	}, nil
}

func (t *testAIGatewayPolicyAPI) CreateAiGatewayPolicy(
	context.Context,
	string,
	kkComps.CreateAIGatewayPolicyRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayPolicyResponse, error) {
	return nil, nil
}

func (t *testAIGatewayPolicyAPI) GetAiGatewayPolicy(
	_ context.Context,
	_ string,
	policyID string,
	_ ...kkOps.Option,
) (*kkOps.GetAiGatewayPolicyResponse, error) {
	for _, policy := range t.policies {
		if policy.ID == policyID || policy.Name == policyID {
			return &kkOps.GetAiGatewayPolicyResponse{AIGatewayPolicy: &policy}, nil
		}
	}
	return &kkOps.GetAiGatewayPolicyResponse{}, nil
}

func (t *testAIGatewayPolicyAPI) UpdateAiGatewayPolicy(
	context.Context,
	kkOps.UpdateAiGatewayPolicyRequest,
	...kkOps.Option,
) (*kkOps.UpdateAiGatewayPolicyResponse, error) {
	return nil, nil
}

func (t *testAIGatewayPolicyAPI) DeleteAiGatewayPolicy(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayPolicyResponse, error) {
	return nil, nil
}
