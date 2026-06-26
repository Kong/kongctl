package resources

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

const aiGatewayPolicyJSON = `{
  "ref": "mask-sensitive-data",
  "ai_gateway": "support-gateway",
  "type": "ai-sanitizer",
  "name": "mask-sensitive-data",
  "display_name": "Mask Sensitive Data",
  "enabled": true,
  "global": false,
  "config": {
    "anonymize": ["email"]
  },
  "labels": {
    "team": "platform"
  }
}`

func TestAIGatewayPolicyResourceUnmarshal(t *testing.T) {
	var policy AIGatewayPolicyResource
	require.NoError(t, json.Unmarshal([]byte(aiGatewayPolicyJSON), &policy))

	require.Equal(t, "mask-sensitive-data", policy.Ref)
	require.Equal(t, "support-gateway", policy.AIGateway)
	require.Equal(t, "mask-sensitive-data", policy.Name)
	require.Equal(t, "ai-sanitizer", policy.Type)
	require.Equal(t, "Mask Sensitive Data", policy.DisplayName)
	require.NotNil(t, policy.Config)
	require.NoError(t, policy.Validate())
}

func TestAIGatewayPolicyResourceDefaults(t *testing.T) {
	policy := AIGatewayPolicyResource{
		BaseResource: BaseResource{Ref: "mask-sensitive-data"},
		AIGateway:    "support-gateway",
		CreateAIGatewayPolicyRequest: kkComps.CreateAIGatewayPolicyRequest{
			Type:   "ai-sanitizer",
			Config: map[string]any{},
		},
	}

	policy.SetDefaults()

	require.Equal(t, "mask-sensitive-data", policy.Name)
	require.Equal(t, "mask-sensitive-data", policy.DisplayName)
	require.NotNil(t, policy.Enabled)
	require.True(t, *policy.Enabled)
	require.NotNil(t, policy.Global)
	require.False(t, *policy.Global)
}

func TestAIGatewayPolicyResourceRejectsKongctlMetadata(t *testing.T) {
	payload := strings.Replace(aiGatewayPolicyJSON, `"ai_gateway": "support-gateway",`,
		`"ai_gateway": "support-gateway", "kongctl": {"namespace": "default"},`, 1)

	var policy AIGatewayPolicyResource
	err := json.Unmarshal([]byte(payload), &policy)
	require.Error(t, err)
	require.Contains(t, err.Error(), "kongctl metadata")
}

func TestAIGatewayPolicyResourceMarshalPreservesParentAndPayload(t *testing.T) {
	var policy AIGatewayPolicyResource
	require.NoError(t, json.Unmarshal([]byte(aiGatewayPolicyJSON), &policy))

	data, err := json.Marshal(policy)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(data, &payload))
	require.Equal(t, "mask-sensitive-data", payload["ref"])
	require.Equal(t, "support-gateway", payload["ai_gateway"])
	require.Equal(t, "ai-sanitizer", payload["type"])
	require.Equal(t, "mask-sensitive-data", payload["name"])
	require.NotContains(t, payload, "id")
}

func TestAIGatewayPolicyResourceParentRefNormalizesDeferredRef(t *testing.T) {
	var policy AIGatewayPolicyResource
	require.NoError(t, json.Unmarshal([]byte(aiGatewayPolicyJSON), &policy))
	policy.AIGateway = tags.RefPlaceholderPrefix + "support-gateway#id"

	parent := policy.GetParentRef()
	require.NotNil(t, parent)
	require.Equal(t, ResourceTypeAIGateway, parent.Kind)
	require.Equal(t, "support-gateway", parent.Ref)

	deps := policy.GetDependencies()
	require.Len(t, deps, 1)
	require.Equal(t, "support-gateway", deps[0].Ref)
}

func TestAIGatewayPolicyResourceFromResponse(t *testing.T) {
	enabled := true
	global := false
	policy, err := AIGatewayPolicyResourceFromResponse("support-gateway", kkComps.AIGatewayPolicy{
		ID:          "policy-id",
		Name:        "mask-sensitive-data",
		Type:        "ai-sanitizer",
		DisplayName: "Mask Sensitive Data",
		Enabled:     &enabled,
		Global:      &global,
		Config:      map[string]any{"anonymize": []any{"email"}},
		Labels:      map[string]string{"team": "platform"},
		CreatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	require.Equal(t, "policy-id", policy.Ref)
	require.Equal(t, "support-gateway", policy.AIGateway)
	require.Equal(t, "mask-sensitive-data", policy.Name)
	require.NoError(t, policy.Validate())
}
