package executor

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestConsumerCredentialInitialProtectionExceptionDoesNotGrantOtherWrites(t *testing.T) {
	for _, owner := range []string{"team-a", "team-b", ""} {
		for _, initial := range []bool{false, true} {
			transport := customPolicyHTTPClient(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodGet, req.Method)
				body, err := json.Marshal(map[string]any{
					"id": "consumer-id", "name": "consumer", "display_name": "Consumer", "type": "api-key",
					"managed_by": resources.BuildChildManagedBy(nil, nil, owner, true, "dev"),
					"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
				})
				require.NoError(t, err)
				return &http.Response{
					StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}},
					Body: io.NopCloser(strings.NewReader(string(body))), Request: req,
				}, nil
			})
			api := kkSDK.New(kkSDK.WithServerURL("https://example.test"), kkSDK.WithClient(transport))
			client := state.NewClient(state.ClientConfig{AIGatewayConsumersAPI: api.AIGatewayConsumers})
			adapter := NewAIGatewayConsumerCredentialAdapter(client)
			adapter.initiallyProtectedConsumer = func(id string) bool { return initial && id == "consumer-id" }
			err := adapter.validateConsumerWrite(t.Context(), "gateway-id", "consumer-id", "team-a", true)
			if owner == "team-a" && initial {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			// Even an initially protected consumer cannot authorize deletion/rotation.
			require.Error(t, adapter.validateConsumerWrite(t.Context(), "gateway-id", "consumer-id", "team-a", false))
		}
	}
}

func TestAIGatewayConsumerCredentialAdapterMapCreateFields(t *testing.T) {
	t.Parallel()

	adapter := NewAIGatewayConsumerCredentialAdapter(nil)
	fields := map[string]any{
		planner.FieldName:        "support-user-key",
		planner.FieldType:        "api-key",
		planner.FieldDisplayName: "Support User API Key",
		planner.FieldLabels:      map[string]string{"team": "support"},
		"ttl":                    float64(60),
		"api_key":                "secret-value",
	}

	var req kkComps.CreateAIGatewayConsumerCredentialRequest
	require.NoError(t, adapter.MapCreateFields(context.Background(), nil, fields, &req))
	require.Equal(t, "support-user-key", req.Name)
	require.Equal(t, kkComps.CreateAIGatewayConsumerCredentialRequestTypeAPIKey, req.Type)
	require.Equal(t, "Support User API Key", req.DisplayName)
	require.Equal(t, map[string]string{"team": "support"}, req.Labels)
	require.NotNil(t, req.TTL)
	require.Equal(t, int64(60), *req.TTL)
	require.NotNil(t, req.APIKey)
	require.Equal(t, "secret-value", *req.APIKey)
}

func TestAIGatewayConsumerCredentialAdapterMapCreateFieldsRequiresType(t *testing.T) {
	t.Parallel()

	adapter := NewAIGatewayConsumerCredentialAdapter(nil)
	fields := map[string]any{
		planner.FieldName:        "support-user-key",
		planner.FieldDisplayName: "Support User API Key",
	}

	var req kkComps.CreateAIGatewayConsumerCredentialRequest
	err := adapter.MapCreateFields(context.Background(), nil, fields, &req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "type")
}
