package resources

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayCustomPolicyValidation(t *testing.T) {
	valid := AIGatewayCustomPolicyResource{
		BaseResource: BaseResource{Ref: "plugin"}, AIGateway: "gateway",
		Name: "plugin", Type: "installed", DisplayName: "Plugin", Schema: "return {}",
	}
	require.NoError(t, valid.Validate())
	for _, tc := range []struct {
		name    string
		change  func(*AIGatewayCustomPolicyResource)
		message string
	}{
		{"name", func(p *AIGatewayCustomPolicyResource) { p.Name = "" }, "name, display_name, and schema are required"},
		{"parent", func(p *AIGatewayCustomPolicyResource) { p.AIGateway = "" }, "ai_gateway is required"},
		{"schema", func(p *AIGatewayCustomPolicyResource) { p.Schema = "" }, "schema are required"},
		{"display name", func(p *AIGatewayCustomPolicyResource) { p.DisplayName = "" }, "display_name"},
		{"type", func(p *AIGatewayCustomPolicyResource) { p.Type = "unknown" }, "installed or streaming"},
		{"streaming handler", func(p *AIGatewayCustomPolicyResource) { p.Type = "streaming" }, "handler is required"},
		{
			"installed handler",
			func(p *AIGatewayCustomPolicyResource) { p.Handler = new("return {}") },
			"only supported for streaming",
		},
		{"metadata", func(p *AIGatewayCustomPolicyResource) { p.Kongctl = &KongctlMeta{} }, "metadata not supported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := valid
			tc.change(&policy)
			require.ErrorContains(t, policy.Validate(), tc.message)
		})
	}
	valid.Type = "streaming"
	valid.Handler = new("return {}")
	require.NoError(t, valid.Validate())
}

func TestAIGatewayCustomPolicyResponseRoundTrip(t *testing.T) {
	for _, kind := range []string{"installed", "streaming"} {
		t.Run(kind, func(t *testing.T) {
			payload := map[string]any{
				"id": "custom-id", "name": "plugin", "type": kind, "display_name": "Plugin",
				"schema":     "return {\n name = 'plugin'\n}",
				"created_at": "2026-01-01T00:00:00Z",
				"updated_at": "2026-01-01T00:00:00Z",
			}
			if kind == "streaming" {
				payload["handler"] = "return {\n VERSION = '1.0'\n}"
			}
			data, err := json.Marshal(payload)
			require.NoError(t, err)
			var response kkComps.AIGatewayCustomPolicy
			require.NoError(t, json.Unmarshal(data, &response))
			resource, err := AIGatewayCustomPolicyResourceFromResponse("gateway", response)
			require.NoError(t, err)
			require.NoError(t, resource.Validate())
			require.Equal(t, "plugin", resource.Ref)
			desired, err := resource.MutablePayloadMap()
			require.NoError(t, err)
			current, err := AIGatewayCustomPolicyMutablePayloadMap(response)
			require.NoError(t, err)
			require.Equal(t, current, desired)
			require.NotContains(t, desired, "id")
			require.NotContains(t, desired, "ref")
		})
	}
}
