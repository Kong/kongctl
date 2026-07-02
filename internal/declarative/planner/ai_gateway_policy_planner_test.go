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
	current := testAIGatewayPolicy()
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

func TestAIGatewayPolicyPlannerIgnoresEmptyAPIConfigDefaults(t *testing.T) {
	policy := testAIGatewayPolicyResource(t)
	current := testAIGatewayPolicy()
	current.Config = map[string]any{
		"allow_all_conversation_history": true,
		"anonymize":                      []any{"email"},
		"block_if_detected":              false,
		"custom_patterns":                nil,
		"dictionary_name":                "kong-default",
		"genai_category":                 "text",
		"host":                           "localhost",
		"keepalive_timeout":              float64(60000),
		"llm_format":                     "openai",
		"max_request_body_size":          float64(8388608),
		"port":                           float64(8080),
		"proxy_config": map[string]any{
			"auth_password":    nil,
			"auth_username":    nil,
			"http_proxy_host":  nil,
			"http_proxy_port":  nil,
			"https_proxy_host": nil,
			"https_proxy_port": nil,
			"no_proxy":         nil,
			"proxy_scheme":     "http",
		},
		"redis": map[string]any{
			"database":   float64(0),
			"host":       "127.0.0.1",
			"port":       float64(6379),
			"ssl":        false,
			"ssl_verify": false,
			"timeout":    float64(2000),
		},
		"recover_redacted":             true,
		"redact_type":                  "placeholder",
		"sanitization_mode":            "INPUT",
		"scheme":                       "http",
		"skip_logging_sanitized_items": false,
		"stop_on_error":                true,
		"timeout":                      float64(10000),
	}

	needsUpdate, fields, changed, err := shouldUpdateAIGatewayPolicy(
		state.AIGatewayPolicy{AIGatewayPolicy: current},
		policy,
	)

	require.NoError(t, err)
	require.Falsef(t, needsUpdate, "changed fields: %#v", changed)
	require.Nil(t, fields)
	require.Nil(t, changed)
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
			policies: []kkComps.AIGatewayPolicy{testAIGatewayPolicy()},
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

func testAIGatewayPolicy() kkComps.AIGatewayPolicy {
	enabled := true
	global := false
	return kkComps.AIGatewayPolicy{
		ID:          "policy-id",
		Name:        "mask-sensitive-data",
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
