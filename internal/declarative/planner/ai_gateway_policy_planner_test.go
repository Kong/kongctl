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

func TestAIGatewayPolicyPlannerIgnoresPromptGuardAPIDefaults(t *testing.T) {
	policy := testAIGatewayPromptGuardPolicyResource(t)
	current := testAIGatewayPromptGuardPolicy()
	current.Config = map[string]any{
		"allow_all_conversation_history": false,
		"allow_patterns":                 nil,
		"deny_patterns":                  []any{".*(W|w)ar.*"},
		"genai_category":                 "text/generation",
		"llm_format":                     "openai",
		"match_all_roles":                false,
		"max_request_body_size":          float64(8388608),
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

func TestAIGatewayPolicyPlannerIgnoresUndeclaredConfigFields(t *testing.T) {
	policy := testAIGatewayHTTPLogPolicyResource(t, `{
		"http_endpoint": "https://logging.example.com/ai-gateway"
	}`)
	current := testAIGatewayHTTPLogPolicy()
	current.Config = map[string]any{
		"content_type":         "application/json",
		"custom_fields_by_lua": nil,
		"flush_timeout":        nil,
		"headers":              nil,
		"http_endpoint":        "https://logging.example.com/ai-gateway",
		"keepalive":            float64(60000),
		"method":               "POST",
		"queue": map[string]any{
			"concurrency_limit":    float64(1),
			"initial_retry_delay":  float64(0.01),
			"max_batch_size":       float64(1),
			"max_bytes":            nil,
			"max_coalescing_delay": float64(1),
			"max_entries":          float64(10000),
			"max_retry_delay":      float64(60),
			"max_retry_time":       float64(60),
		},
		"queue_size":  nil,
		"retry_count": nil,
		"ssl_verify":  true,
		"timeout":     float64(10000),
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

func TestAIGatewayPolicyPlannerComparesDeclaredNestedConfigFields(t *testing.T) {
	policy := testAIGatewayHTTPLogPolicyResource(t, `{
		"http_endpoint": "https://logging.example.com/ai-gateway",
		"queue": {"max_batch_size": 2}
	}`)
	current := testAIGatewayHTTPLogPolicy()
	current.Config = map[string]any{
		"http_endpoint": "https://logging.example.com/ai-gateway",
		"queue": map[string]any{
			"concurrency_limit": float64(1),
			"max_batch_size":    float64(1),
			"max_entries":       float64(10000),
		},
	}

	needsUpdate, fields, changed, err := shouldUpdateAIGatewayPolicy(
		state.AIGatewayPolicy{AIGatewayPolicy: current},
		policy,
	)

	require.NoError(t, err)
	require.True(t, needsUpdate)
	require.NotNil(t, fields)
	require.Contains(t, changed, FieldConfig)
}

func TestAIGatewayPolicyPlannerIgnoresServerDefaultsUnderDeclaredEmptyConfigObjects(t *testing.T) {
	policy := testAIGatewayResponseTransformerPolicyResource(t)
	current := testAIGatewayResponseTransformerPolicy()
	current.Config = map[string]any{
		"http_timeout": float64(60000),
		"https_verify": true,
		"prompt":       "Mask all credit card numbers.",
		"llm": map[string]any{
			"route_type": "llm/v1/chat",
			"auth": map[string]any{
				"allow_override":  false,
				"azure_client_id": nil,
				"header_name":     "Authorization",
				"header_value":    "{vault://poc-aigw-secrets/response-transformer-token}",
			},
			"model": map[string]any{
				"provider": "bedrock",
				"name":     "anthropic.claude-3-haiku-20240307-v1:0",
				"options": map[string]any{
					"azure_api_version": "2023-05-15",
				},
			},
		},
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

func TestAIGatewayPolicyPlannerIgnoresServerDefaultsInsideDeclaredConfigArrays(t *testing.T) {
	policy := testAIGatewayRateLimitingAdvancedPolicyResource(t)
	current := testAIGatewayRateLimitingAdvancedPolicy()
	current.Config = map[string]any{
		"custom_cost_count_function": nil,
		"dictionary_name":            "kong_rate_limiting_counters",
		"identifier":                 "ip",
		"strategy":                   "local",
		"tokens_count_strategy":      "total_tokens",
		"window_type":                "sliding",
		"policies": []any{
			map[string]any{
				"id":          nil,
				"timezone":    nil,
				"window_type": "sliding",
				"limits": []any{
					map[string]any{
						"limit":                 float64(5),
						"month_day":             nil,
						"period":                nil,
						"tokens_count_strategy": "total_tokens",
						"week_start_day":        nil,
						"window_size":           float64(3600),
					},
				},
				"match": []any{
					map[string]any{
						"key":          nil,
						"partition_by": false,
						"type":         "ip",
						"values":       nil,
					},
				},
			},
		},
		"redis": map[string]any{
			"database":   float64(0),
			"host":       "127.0.0.1",
			"port":       float64(6379),
			"ssl":        false,
			"ssl_verify": true,
			"timeout":    float64(2000),
		},
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

func TestAIGatewayPolicyPlannerComparesDeclaredNullConfigValues(t *testing.T) {
	for _, tc := range []struct {
		name          string
		desiredConfig string
		currentConfig map[string]any
	}{
		{
			name: "null",
			desiredConfig: `{
				"http_endpoint": "https://logging.example.com/ai-gateway",
				"headers": null
			}`,
			currentConfig: map[string]any{
				"http_endpoint": "https://logging.example.com/ai-gateway",
				"headers": map[string]any{
					"X-Team": "platform",
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := testAIGatewayHTTPLogPolicyResource(t, tc.desiredConfig)
			current := testAIGatewayHTTPLogPolicy()
			current.Config = tc.currentConfig

			needsUpdate, fields, changed, err := shouldUpdateAIGatewayPolicy(
				state.AIGatewayPolicy{AIGatewayPolicy: current},
				policy,
			)

			require.NoError(t, err)
			require.True(t, needsUpdate)
			require.NotNil(t, fields)
			require.Contains(t, changed, FieldConfig)
		})
	}
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

func testAIGatewayPromptGuardPolicyResource(t *testing.T) resources.AIGatewayPolicyResource {
	t.Helper()
	payload := `{
		"ref": "repro-prompt-guard",
		"ai_gateway": "repro-gateway",
		"type": "ai-prompt-guard",
		"name": "repro-prompt-guard",
		"display_name": "Repro Prompt Guard",
		"enabled": true,
		"global": false,
		"config": {"deny_patterns": [".*(W|w)ar.*"]}
	}`
	var policy resources.AIGatewayPolicyResource
	require.NoError(t, json.Unmarshal([]byte(payload), &policy))
	return policy
}

func testAIGatewayHTTPLogPolicyResource(t *testing.T, config string) resources.AIGatewayPolicyResource {
	t.Helper()
	payload := `{
		"ref": "repro-http-log",
		"ai_gateway": "repro-gateway",
		"type": "http-log",
		"name": "repro-http-log",
		"display_name": "Repro HTTP Log",
		"enabled": true,
		"global": false,
		"config": CONFIG
	}`
	var policy resources.AIGatewayPolicyResource
	require.NoError(t, json.Unmarshal([]byte(strings.Replace(payload, "CONFIG", config, 1)), &policy))
	return policy
}

func testAIGatewayResponseTransformerPolicyResource(t *testing.T) resources.AIGatewayPolicyResource {
	t.Helper()
	payload := `{
		"ref": "poc-response-transformer",
		"ai_gateway": "support-gateway",
		"type": "ai-response-transformer",
		"name": "poc-response-transformer",
		"display_name": "POC Response Transformer",
		"enabled": true,
		"global": false,
		"config": {
			"prompt": "Mask all credit card numbers.",
			"llm": {
				"route_type": "llm/v1/chat",
				"auth": {
					"header_name": "Authorization",
					"header_value": "{vault://poc-aigw-secrets/response-transformer-token}"
				},
				"model": {
					"provider": "bedrock",
					"name": "anthropic.claude-3-haiku-20240307-v1:0",
					"options": {}
				}
			}
		}
	}`
	var policy resources.AIGatewayPolicyResource
	require.NoError(t, json.Unmarshal([]byte(payload), &policy))
	return policy
}

func testAIGatewayRateLimitingAdvancedPolicyResource(t *testing.T) resources.AIGatewayPolicyResource {
	t.Helper()
	payload := `{
		"ref": "cost-budget",
		"ai_gateway": "support-gateway",
		"type": "ai-rate-limiting-advanced",
		"name": "cost-budget",
		"display_name": "Cost Budget",
		"enabled": true,
		"global": false,
		"config": {
			"identifier": "ip",
			"strategy": "local",
			"policies": [
				{
					"match": [{"type": "ip"}],
					"limits": [
						{
							"limit": 5,
							"window_size": 3600,
							"tokens_count_strategy": "total_tokens"
						}
					]
				}
			]
		}
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
		"targets": [{"name": "gpt-4o", "provider": "support-openai", "config": {"type": "openai"}}],
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

func testAIGatewayPromptGuardPolicy() kkComps.AIGatewayPolicy {
	enabled := true
	global := false
	return kkComps.AIGatewayPolicy{
		ID:          "prompt-guard-policy-id",
		Name:        "repro-prompt-guard",
		Type:        "ai-prompt-guard",
		DisplayName: "Repro Prompt Guard",
		Enabled:     &enabled,
		Global:      &global,
		Config: map[string]any{
			"deny_patterns": []any{".*(W|w)ar.*"},
		},
	}
}

func testAIGatewayHTTPLogPolicy() kkComps.AIGatewayPolicy {
	enabled := true
	global := false
	return kkComps.AIGatewayPolicy{
		ID:          "http-log-policy-id",
		Name:        "repro-http-log",
		Type:        "http-log",
		DisplayName: "Repro HTTP Log",
		Enabled:     &enabled,
		Global:      &global,
		Config: map[string]any{
			"http_endpoint": "https://logging.example.com/ai-gateway",
		},
	}
}

func testAIGatewayResponseTransformerPolicy() kkComps.AIGatewayPolicy {
	enabled := true
	global := false
	return kkComps.AIGatewayPolicy{
		ID:          "response-transformer-policy-id",
		Name:        "poc-response-transformer",
		Type:        "ai-response-transformer",
		DisplayName: "POC Response Transformer",
		Enabled:     &enabled,
		Global:      &global,
		Config: map[string]any{
			"prompt": "Mask all credit card numbers.",
		},
	}
}

func testAIGatewayRateLimitingAdvancedPolicy() kkComps.AIGatewayPolicy {
	enabled := true
	global := false
	return kkComps.AIGatewayPolicy{
		ID:          "cost-budget-policy-id",
		Name:        "cost-budget",
		Type:        "ai-rate-limiting-advanced",
		DisplayName: "Cost Budget",
		Enabled:     &enabled,
		Global:      &global,
		Config:      map[string]any{},
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
