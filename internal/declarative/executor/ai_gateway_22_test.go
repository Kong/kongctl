package executor

import (
	"encoding/json"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

func TestAIGateway22ManifestPreservesModelAndProviderPayloads(t *testing.T) {
	t.Setenv("JEV_AUTHORIZATION", "Bearer test-placeholder")
	path := filepath.Join("..", "..", "..", "docs", "examples", "declarative", "ai-gateway", "pre-ga-2.2.yaml")
	rs, err := loader.New().LoadFromSources([]loader.Source{{Path: path, Type: loader.SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayProviders, 1)
	require.Len(t, rs.AIGatewayModels, 2)

	providerPayload, err := rs.AIGatewayProviders[0].MutablePayloadMap()
	require.NoError(t, err)
	var provider kkComps.CreateAIGatewayModelProviderRequest
	require.NoError(t, NewAIGatewayProviderAdapter(nil).MapCreateFields(t.Context(), nil, providerPayload, &provider))
	require.NotNil(t, provider.AIGatewayModelProviderTypesafe)
	require.Equal(t, tags.EnvPlaceholderPrefix+"JEV_AUTHORIZATION",
		*provider.AIGatewayModelProviderTypesafe.Config.Auth.Headers[0].Value)

	adapter := NewAIGatewayModelAdapter(nil)
	for _, model := range rs.AIGatewayModels {
		payload, err := model.MutablePayloadMap()
		require.NoError(t, err)
		var create kkComps.CreateAIGatewayModelRequest
		require.NoError(t, adapter.MapCreateFields(t.Context(), nil, payload, &create))
		var update kkComps.UpdateAIGatewayModelRequest
		require.NoError(t, adapter.MapUpdateFields(t.Context(), nil, payload, &update, nil))
		for _, request := range []any{create, update} {
			encoded, err := json.Marshal(request)
			require.NoError(t, err)
			var roundTrip resources.AIGatewayModelResource
			require.NoError(t, json.Unmarshal(encoded, &roundTrip))
			actual, err := roundTrip.MutablePayloadMap()
			require.NoError(t, err)
			require.Equal(t, payload, actual)
		}
		if model.Name() == "decisions" {
			require.Equal(
				t,
				[]kkComps.AIGatewayModelModelCapabilities{kkComps.AIGatewayModelModelCapabilitiesDecisions},
				create.AIGatewayModelModel.Capabilities,
			)
			require.NotNil(t, create.AIGatewayModelModel.Targets[0].Config.AIGatewayTargetTypesafeConfig)
			require.Equal(t, []string{"decisions", "decisions-alias"},
				create.AIGatewayModelModel.Config.Route.Model.Values)
		} else {
			require.Equal(t, []kkComps.Capabilities{kkComps.CapabilitiesSkills}, create.AIGatewayModelAPI.Capabilities)
			require.Equal(t, kkComps.AIGatewayModelFormatTypePassthrough, *create.AIGatewayModelAPI.Formats[0].Type)
		}
	}
}

func TestAIGatewayPolicyConfigRemainsOpaque(t *testing.T) {
	for _, policyType := range []string{"ai-prompt-compressor", "ai-rate-limiting-advanced", "opentelemetry"} {
		t.Run(policyType, func(t *testing.T) {
			// These deliberately synthetic options exercise transport preservation,
			// not validation of a plugin schema owned by the runtime.
			config := map[string]any{
				"future_option": map[string]any{
					"enabled": false,
					"values":  []any{"one", "two"},
					"limit":   float64(0),
				},
			}
			policy := resources.AIGatewayPolicyResource{
				BaseResource: resources.BaseResource{Ref: "policy"},
				AIGateway:    "gateway",
				CreateAIGatewayPolicyRequest: kkComps.CreateAIGatewayPolicyRequest{
					Name: "policy", DisplayName: "Policy", Type: policyType, Config: config,
				},
			}
			require.NoError(t, policy.Validate())
			payload, err := policy.MutablePayloadMap()
			require.NoError(t, err)
			adapter := NewAIGatewayPolicyAdapter(nil)
			var create kkComps.CreateAIGatewayPolicyRequest
			require.NoError(t, adapter.MapCreateFields(t.Context(), nil, payload, &create))
			require.Equal(t, config, create.Config)
			var update kkComps.UpdateAIGatewayPolicyRequest
			require.NoError(t, adapter.MapUpdateFields(t.Context(), nil,
				map[string]any{planner.FieldConfig: config}, &update, nil))
			require.Equal(t, config, update.Config)
		})
	}
}
