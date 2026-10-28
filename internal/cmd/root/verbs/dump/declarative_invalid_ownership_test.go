package dump

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"strings"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	testdecl "github.com/kong/kongctl/test/declarative"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayDumpSkipsMalformedOwnershipWithoutLosingSiblings(t *testing.T) {
	for _, tc := range testdecl.AIGatewayOwnedChildren(t) {
		t.Run(tc.Collection, func(t *testing.T) {
			var children []any
			for _, name := range []string{"first", "invalid", "last"} {
				child := maps.Clone(tc.Body)
				child["id"], child["name"], child["display_name"] = name+"-id", name, name
				metadata := resources.BuildChildManagedBy(nil, map[string]string{"inventory": "retained"}, "team-a", false, "dev")
				if name == "invalid" {
					metadata[resources.ChildProtectedKey] = "TRUE"
				}
				child["managed_by"] = metadata
				child["created_at"], child["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
				children = append(children, child)
			}
			transport := aiChildDumpHTTPClient(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodGet, req.Method)
				data := []any{}
				if strings.HasSuffix(req.URL.Path, "/"+tc.Path) {
					data = children
				}
				body, err := json.Marshal(map[string]any{"data": data})
				require.NoError(t, err)
				return &http.Response{
					StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}},
					Body: io.NopCloser(bytes.NewReader(body)), Request: req,
				}, nil
			})
			api := &helpers.KonnectSDK{SDK: kkSDK.New(
				kkSDK.WithServerURL("https://example.test"), kkSDK.WithClient(transport),
			)}
			client := state.NewClient(state.ClientConfig{
				AIGatewayProvidersAPI:             api.GetAIGatewayProvidersAPI(),
				AIGatewayAuthStrategiesAPI:        api.GetAIGatewayAuthStrategiesAPI(),
				AIGatewayPoliciesAPI:              api.GetAIGatewayPoliciesAPI(),
				AIGatewayCustomPoliciesAPI:        api.GetAIGatewayCustomPoliciesAPI(),
				AIGatewayAgentsAPI:                api.GetAIGatewayAgentsAPI(),
				AIGatewayModelAPI:                 api.GetAIGatewayModelAPI(),
				AIGatewayConsumersAPI:             api.GetAIGatewayConsumersAPI(),
				AIGatewayConsumerGroupsAPI:        api.GetAIGatewayConsumerGroupsAPI(),
				AIGatewayMCPServersAPI:            api.GetAIGatewayMCPServersAPI(),
				AIGatewayVaultsAPI:                api.GetAIGatewayVaultsAPI(),
				AIGatewayConfigStoresAPI:          api.GetAIGatewayConfigStoresAPI(),
				AIGatewayDataPlaneCertificatesAPI: api.GetAIGatewayDataPlaneCertificatesAPI(),
				AIGatewayCertificatesAPI:          api.GetAIGatewayCertificatesAPI(),
				AIGatewayCACertificatesAPI:        api.GetAIGatewayCACertificatesAPI(),
				AIGatewaySNIsAPI:                  api.GetAIGatewaySNIsAPI(),
			})
			gateways := []resources.AIGatewayResource{{
				BaseResource:           resources.BaseResource{Ref: "gateway-id"},
				CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{Name: "shared", DisplayName: "Shared"},
			}}
			var logs bytes.Buffer
			populateAIGatewayChildren(t.Context(), slog.New(slog.NewTextHandler(&logs, nil)), client, gateways)
			data, err := json.Marshal(gateways[0])
			require.NoError(t, err)
			var output map[string]any
			require.NoError(t, json.Unmarshal(data, &output))
			require.Equal(t, "shared", output["name"])
			exported, ok := output[tc.Collection].([]any)
			require.True(t, ok, logs.String())
			require.Len(t, exported, 2, logs.String())
			for _, value := range exported {
				child := value.(map[string]any)
				require.NotEqual(t, "invalid", child["name"])
				require.Equal(t, map[string]any{"inventory": "retained"}, child["managed_by"])
			}
			require.Contains(t, logs.String(), "invalid ownership metadata; skipping child in dump")
			require.Contains(t, logs.String(), "resource_id=invalid-id")
		})
	}
}
