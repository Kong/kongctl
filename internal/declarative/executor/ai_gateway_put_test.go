package executor

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/log"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayPUTPreservesWritableState(t *testing.T) {
	for _, test := range []string{"description and labels", "omitted optional fields", "explicit clearing"} {
		t.Run(test, func(t *testing.T) {
			remote := kkComps.AIGateway{
				ID: "gateway-id", Name: "gateway", DisplayName: "Gateway",
				Description: new("Original"), MinRuntimeVersion: new("2.1"), RuntimeAutoUpgrade: new(false),
				Labels:    map[string]string{labels.NamespaceKey: "default", "team": "platform"},
				ProxyUrls: []kkComps.AIGatewayProxyURL{{Host: "smoke.example.com", Port: 443, Protocol: "https"}},
			}
			desired := resources.AIGatewayResource{
				BaseResource:           resources.BaseResource{Ref: "gateway"},
				CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{Name: "gateway", DisplayName: "Gateway"},
			}
			switch test {
			case "description and labels":
				desired.Description = new("Updated")
				desired.Labels = map[string]string{"team": "updated"}
				desired.ProxyUrls = remote.ProxyUrls
			case "omitted optional fields":
				desired.DisplayName = "Renamed"
			case "explicit clearing":
				remote.RuntimeAutoUpgrade = new(true)
				desired.Description = new("")
				desired.ProxyUrls = []kkComps.AIGatewayProxyURL{}
				desired.Labels = map[string]string{}
				desired.RuntimeAutoUpgrade = new(false)
			}
			var sent map[string]any
			puts := 0
			httpClient := customPolicyHTTPClient(func(r *http.Request) (*http.Response, error) {
				require.True(t, strings.HasPrefix(r.URL.Path, "/v1/ai-gateways"))
				var response any = remote
				switch r.Method {
				case http.MethodGet:
					if r.URL.Path == "/v1/ai-gateways" {
						response = map[string]any{
							"data": []kkComps.AIGateway{remote},
							"meta": map[string]any{"page": map[string]any{"total": 1}},
						}
					}
				case http.MethodPut:
					puts++
					body, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					require.NoError(t, json.Unmarshal(body, &sent))
					// Model replacement semantics: omitted writable fields are lost.
					remote = kkComps.AIGateway{}
					require.NoError(t, json.Unmarshal(body, &remote))
					remote.ID = "gateway-id"
					response = remote
				default:
					t.Fatalf("unexpected method %s", r.Method)
				}
				body, err := json.Marshal(response)
				require.NoError(t, err)
				return &http.Response{
					StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
					Body: io.NopCloser(strings.NewReader(string(body))), Request: r,
				}, nil
			})
			sdk := kkSDK.New(kkSDK.WithServerURL("https://example.test"), kkSDK.WithClient(httpClient))
			newClient := func() *state.Client {
				return state.NewClient(state.ClientConfig{AIGatewayAPI: &helpers.AIGatewayAPIImpl{SDK: sdk}})
			}
			rs := &resources.ResourceSet{AIGateways: []resources.AIGatewayResource{desired}}
			client := newClient()
			plan, err := planner.NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs,
				planner.Options{Mode: planner.PlanModeApply})
			require.NoError(t, err)
			require.Len(t, plan.Changes, 1)
			ctx := context.WithValue(t.Context(), log.LoggerKey, slog.Default())
			result := New(client, nil, false).Execute(ctx, plan)
			require.Empty(t, result.Errors)
			require.Equal(t, 1, puts)
			require.Equal(t, false, sent[planner.FieldRuntimeAutoUpgrade])
			require.Equal(t, "2.1", sent[planner.FieldMinRuntimeVersion])
			require.Equal(t, "default", remote.Labels[labels.NamespaceKey])
			if test == "explicit clearing" {
				require.Empty(t, remote.ProxyUrls)
				require.Equal(t, "", *remote.Description)
				require.NotContains(t, remote.Labels, "team")
			} else {
				require.Len(t, remote.ProxyUrls, 1)
				require.Equal(t, "smoke.example.com", remote.ProxyUrls[0].Host)
				if test == "omitted optional fields" {
					require.Equal(t, "Original", *remote.Description)
					require.Equal(t, "platform", remote.Labels["team"])
				}
			}
			plan, err = planner.NewPlanner(newClient(), slog.Default()).GeneratePlan(t.Context(), rs,
				planner.Options{Mode: planner.PlanModeApply})
			require.NoError(t, err)
			require.Empty(t, plan.Changes)
		})
	}
}
