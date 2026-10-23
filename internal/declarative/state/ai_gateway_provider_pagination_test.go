package state

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

type providerPaginationAPI struct {
	helpers.AIGatewayProvidersAPI
	list func(kkOps.ListAiGatewayModelProvidersRequest) (*kkOps.ListAiGatewayModelProvidersResponse, error)
}

func (a providerPaginationAPI) ListAiGatewayProviders(_ context.Context,
	req kkOps.ListAiGatewayModelProvidersRequest,
	_ ...kkOps.Option) (*kkOps.ListAiGatewayModelProvidersResponse,
	error,
) {
	return a.list(req)
}

func TestProviderObservationRejectsIncompleteAndCyclicPagination(t *testing.T) {
	for _, failure := range []string{"none", "nil-page", "error", "cycle", "invalid-cursor"} {
		t.Run(failure, func(t *testing.T) {
			calls := 0
			var provider kkComps.AIGatewayModelProvider
			require.NoError(t,
				json.Unmarshal([]byte(`{
 "id":"provider-id", "name":"provider", "display_name":"Provider", "type":"openai",
 "config":{"auth":{"type":"basic"}},
 "created_at":"2026-01-01T00:00:00Z", "updated_at":"2026-01-01T00:00:00Z"
}`),
					&provider))
			list := func(
				req kkOps.ListAiGatewayModelProvidersRequest,
			) (*kkOps.ListAiGatewayModelProvidersResponse, error) {
				calls++
				require.LessOrEqual(t, calls, 2)
				require.Equal(t, "gateway-id", req.GatewayID)
				var next *string
				data := []kkComps.AIGatewayModelProvider{provider}
				if calls == 1 {
					next = new("opaque+cursor/==")
				} else {
					require.Equal(t, "opaque+cursor/==", *req.PageAfter)
					switch failure {
					case "nil-page":
						return nil, nil
					case "error":
						return nil, errors.New("page unavailable")
					case "cycle":
						next = new("opaque+cursor/==")
					case "invalid-cursor":
						next = new("https://example.test/providers?unexpected=next")
					}
					data = nil
				}
				return &kkOps.ListAiGatewayModelProvidersResponse{
						ListAIGatewayModelProvidersResponse: &kkComps.ListAIGatewayModelProvidersResponse{
							Data: data,
							Meta: kkComps.CursorMeta{Page: kkComps.CursorMetaPage{Next: next}},
						},
					},
					nil
			}
			client := NewClient(ClientConfig{AIGatewayProvidersAPI: &providerPaginationAPI{list: list}})
			result, err := client.ListAIGatewayProviders(t.Context(), "gateway-id")
			if failure == "none" {
				require.NoError(t, err)
				require.Len(t, result, 1)
			} else {
				require.Error(t, err)
				require.Nil(t, result, "partial observations must never be returned")
			}
		})
	}
}
