package executor

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

type customPolicyHTTPClient func(*http.Request) (*http.Response, error)

func (f customPolicyHTTPClient) Do(r *http.Request) (*http.Response, error) { return f(r) }

func TestAIGatewayCustomPolicyLifecycle(t *testing.T) {
	path := filepath.Join("..", "..", "..", "docs", "examples", "declarative", "ai-gateway", "custom-policies.yaml")
	rs, err := loader.New().LoadFromSources([]loader.Source{{Path: path, Type: loader.SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayCustomPolicies, 1)
	resource := rs.AIGatewayCustomPolicies[0]
	require.Equal(t, "custom-policy-gateway", resource.AIGateway)
	fields, err := resource.MutablePayloadMap()
	require.NoError(t, err)
	remote := map[string]any{
		"id":         "custom-id",
		"created_at": "2026-01-01T00:00:00Z",
		"updated_at": "2026-01-01T00:00:00Z",
	}
	maps.Copy(remote, fields)
	data, err := json.Marshal(remote)
	require.NoError(t, err)
	var calls []string
	client := customPolicyHTTPClient(func(r *http.Request) (*http.Response, error) {
		calls = append(calls, r.Method)
		require.True(t, strings.HasPrefix(r.URL.Path, "/v1/ai-gateways/gateway-id/custom-policies"))
		body := string(data)
		status := http.StatusOK
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			var actual map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&actual))
			require.Equal(t, fields, actual)
		}
		if r.Method == http.MethodPost {
			status = http.StatusCreated
		}
		if r.Method == http.MethodDelete {
			status = http.StatusNoContent
			body = ""
		}
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/custom-policies") {
			body = `{"data":[` + body + `],"meta":{"page":{"size":100}}}`
		}
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})
	sdk := &helpers.KonnectSDK{SDK: kkSDK.New(kkSDK.WithServerURL("https://example.test"), kkSDK.WithClient(client))}
	stateClient := state.NewClient(state.ClientConfig{AIGatewayCustomPoliciesAPI: sdk.GetAIGatewayCustomPoliciesAPI()})
	adapter := NewAIGatewayCustomPolicyAdapter(stateClient)
	execCtx := &ExecutionContext{PlannedChange: &planner.PlannedChange{Parent: &planner.ParentInfo{ID: "gateway-id"}}}
	var create kkComps.CreateAIGatewayCustomPolicyRequest
	require.NoError(t, adapter.MapCreateFields(t.Context(), execCtx, fields, &create))
	id, err := adapter.Create(t.Context(), create, "default", execCtx)
	require.NoError(t, err)
	require.Equal(t, "custom-id", id)
	policies, err := stateClient.ListAIGatewayCustomPolicies(t.Context(), "gateway-id")
	require.NoError(t, err)
	require.Len(t, policies, 1)
	found, err := adapter.GetByID(t.Context(), id, execCtx)
	require.NoError(t, err)
	require.Equal(t, resource.Name, found.GetName())
	fields[planner.FieldDisplayName] = "Updated"
	remote[planner.FieldDisplayName] = "Updated"
	data, err = json.Marshal(remote)
	require.NoError(t, err)
	var update kkComps.UpdateAIGatewayCustomPolicyRequest
	require.NoError(t, adapter.MapUpdateFields(t.Context(), execCtx, fields, &update, nil))
	updated, err := adapter.Update(t.Context(), id, update, "default", execCtx)
	require.NoError(t, err)
	require.Equal(t, id, updated)
	require.NoError(t, adapter.Delete(t.Context(), id, execCtx))
	require.Equal(t, []string{"POST", "GET", "GET", "PUT", "DELETE"}, calls)
	_, err = adapter.Create(t.Context(), create, "default", nil)
	require.ErrorContains(t, err, "execution context required")
}
