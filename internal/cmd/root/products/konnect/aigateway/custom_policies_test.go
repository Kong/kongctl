package aigateway

import (
	"context"
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

type customPolicyReadAPI struct {
	helpers.AIGatewayCustomPoliciesAPI
	t      *testing.T
	calls  int
	policy kkComps.AIGatewayCustomPolicy
}

func (a *customPolicyReadAPI) ListAiGatewayCustomPolicies(
	_ context.Context,
	r kkOps.ListAiGatewayCustomPoliciesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAiGatewayCustomPoliciesResponse, error) {
	require.Equal(a.t, "gateway-id", r.GatewayID)
	page := kkComps.CursorMetaPage{}
	if a.calls == 0 {
		require.Nil(a.t, r.PageAfter)
		page.Next = new("https://example.test/custom-policies?page%5Bafter%5D=next")
	} else {
		require.Equal(a.t, "next", *r.PageAfter)
	}
	a.calls++
	return &kkOps.ListAiGatewayCustomPoliciesResponse{
		ListAIGatewayCustomPoliciesResponse: &kkComps.ListAIGatewayCustomPoliciesResponse{
			Data: []kkComps.AIGatewayCustomPolicy{a.policy},
			Meta: kkComps.CursorMeta{Page: page},
		},
	}, nil
}

func TestAIGatewayCustomPolicyCommands(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		root, err := NewAIGatewayCmd(verb, nil, nil)
		require.NoError(t, err)
		child, args, err := root.Find([]string{"custom-policy"})
		require.NoError(t, err)
		require.Empty(t, args)
		require.Equal(t, "custom-policies", child.Name())
		require.NotNil(t, child.Flags().Lookup("gateway-id"))
		require.NotNil(t, child.Flags().Lookup("custom-policy-name"))
		require.NotNil(t, child.RunE)
	}
}

func TestAIGatewayCustomPolicyPaginationAndPresentation(t *testing.T) {
	var policy kkComps.AIGatewayCustomPolicy
	require.NoError(
		t,
		json.Unmarshal([]byte(`{"id":"custom-id","name":"plugin","type":"streaming","display_name":"Plugin",
"schema":"return {}","handler":"return { VERSION = '1.0' }",
"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`), &policy),
	)
	helper := cmd.NewMockHelper(t)
	helper.EXPECT().GetContext().Return(t.Context())
	api := &customPolicyReadAPI{t: t, policy: policy}
	result, err := fetchAIGatewayCustomPolicies(helper, api, "gateway-id", aiGatewayTestConfig{})
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, 2, api.calls)
	record := aiGatewayCustomPolicyToRecord(policy)
	require.Equal(t, "custom-id", record.ID)
	require.Equal(t, "streaming", record.Type)
	detail := aiGatewayCustomPolicyDetailView(policy)
	require.Contains(t, detail, "schema: return {}")
	require.Contains(t, detail, "handler: return { VERSION = '1.0' }")
	view := buildAIGatewayCustomPolicyChildView(result)
	require.NotNil(t, view)
}
