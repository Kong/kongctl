package dump

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestCustomPolicyDumpWarnsAboutProtectionWithoutOwnership(t *testing.T) {
	for _, tc := range []struct {
		name, namespace string
		protection      *string
		warn            bool
	}{
		{name: "unowned"},
		{name: "orphaned protection", protection: new("true"), warn: true},
		{name: "orphaned false protection", protection: new("false"), warn: true},
		{name: "orphaned invalid protection", protection: new("invalid"), warn: true},
		{name: "owned", namespace: "team-a", protection: new("false")},
		{name: "owned protected", namespace: "team-a", protection: new("true")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			metadata := map[string]string{
				"other-tool": "retained", resources.ManagedByToolKey: "kongctl", resources.ManagedByVersionKey: "dev",
			}
			if tc.namespace != "" {
				metadata[resources.ChildNamespaceKey] = tc.namespace
			}
			if tc.protection != nil {
				metadata[resources.ChildProtectedKey] = *tc.protection
			}
			policy := map[string]any{
				"id": "policy-id", "name": "policy", "display_name": "Strategy", "type": "installed",
				"schema": "return {}", "managed_by": metadata,
				"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
			}
			body, err := json.Marshal(map[string]any{"data": []any{policy}})
			require.NoError(t, err)
			httpClient := aiChildDumpHTTPClient(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "/v1/ai-gateways/gateway-id/custom-policies", req.URL.Path)
				return &http.Response{
					StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
					Body: io.NopCloser(bytes.NewReader(body)), Request: req,
				}, nil
			})
			sdk := &helpers.KonnectSDK{SDK: kkSDK.New(
				kkSDK.WithServerURL("https://example.test"), kkSDK.WithClient(httpClient),
			)}
			client := state.NewClient(state.ClientConfig{AIGatewayCustomPoliciesAPI: sdk.GetAIGatewayCustomPoliciesAPI()})
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			policies, err := buildAIGatewayCustomPolicies(t.Context(), logger, client, "gateway-id", "Gateway", "")
			require.NoError(t, err)
			require.Len(t, policies, 1)
			require.Empty(t, policies[0].ManagedBy)
			if tc.namespace == "" {
				// An orphaned protection key must never imply adoption into a default namespace.
				require.Nil(t, policies[0].Kongctl)
			} else {
				require.Equal(t, tc.namespace, resources.GetNamespace(policies[0].Kongctl))
				require.Equal(t, *tc.protection == "true", policies[0].Protected())
			}
			if tc.warn {
				require.Contains(t, logs.String(), "protection metadata without an ownership namespace")
				require.Contains(t, logs.String(), "resource_id=policy-id")
				require.Contains(t, logs.String(), "resource_name=policy")
			} else {
				require.Empty(t, logs.String())
			}
		})
	}
}
