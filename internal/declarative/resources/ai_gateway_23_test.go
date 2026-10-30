package resources

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestMCPRateLimitValidation(t *testing.T) {
	valid := kkComps.AIGatewayMCPServerRateLimit{
		Method:     new("tools/call"),
		Limit:      10,
		WindowSize: 60,
	}
	require.NoError(
		t,
		validateAIGatewayMCPRateLimiting(
			kkComps.AIGatewayMCPServerRateLimiting{
				Limits: []kkComps.AIGatewayMCPServerRateLimit{valid},
			},
		),
	)
	for _, change := range []func(*kkComps.AIGatewayMCPServerRateLimit){
		func(l *kkComps.AIGatewayMCPServerRateLimit) { l.Method = nil },
		func(l *kkComps.AIGatewayMCPServerRateLimit) { l.Tool = new("search") },
		func(l *kkComps.AIGatewayMCPServerRateLimit) { l.Limit = 0 },
		func(l *kkComps.AIGatewayMCPServerRateLimit) { l.WindowSize = 30 },
	} {
		limit := valid
		change(&limit)
		require.Error(
			t,
			validateAIGatewayMCPRateLimiting(
				kkComps.AIGatewayMCPServerRateLimiting{
					Limits: []kkComps.AIGatewayMCPServerRateLimit{limit},
				},
			),
		)
	}
	require.ErrorContains(
		t,
		validateAIGatewayMCPRateLimiting(
			kkComps.AIGatewayMCPServerRateLimiting{
				Limits: []kkComps.AIGatewayMCPServerRateLimit{valid, valid},
			},
		),
		"duplicate",
	)
}

func TestTokenVaultRequiresOneDatastore(t *testing.T) {
	require.Error(t, validateAIGatewayTokenVault(nil, true))
	require.Error(t, validateAIGatewayTokenVault([]kkComps.AIGatewayDatastoreRef{}, false))
	require.Error(
		t,
		validateAIGatewayTokenVault(
			[]kkComps.AIGatewayDatastoreRef{{Name: "one"}, {Name: "two"}},
			true,
		),
	)
	require.NoError(
		t,
		validateAIGatewayTokenVault([]kkComps.AIGatewayDatastoreRef{{Name: "one"}}, true),
	)
}

func TestAICostManagementTagsRequireExplicitArrays(t *testing.T) {
	r := AICostManagementTagsResource{BaseResource: BaseResource{Ref: "capture"}}
	require.ErrorContains(t, r.Validate(), "explicit arrays")
	r.Headers = []kkComps.CostTagHeader{}
	require.ErrorContains(t, r.Validate(), "explicit arrays")
	r.Prefixes = []kkComps.CostTagPrefix{}
	require.NoError(t, r.Validate())
	r.Headers = []kkComps.CostTagHeader{{Name: "my-header", TagKey: "tenant"}, {Name: "MY-HEADER", TagKey: "other"}}
	require.ErrorContains(t, r.Validate(), "duplicate")
}
