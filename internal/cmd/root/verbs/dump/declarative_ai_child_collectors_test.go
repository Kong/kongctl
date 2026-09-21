package dump

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	declresources "github.com/kong/kongctl/internal/declarative/resources"
	declstate "github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestAIGatewayChildCollectorCoverage(t *testing.T) {
	require.NoError(t, validateAIGatewayChildCollectors(aiGatewayChildCollectors))
	for _, tt := range []struct {
		name   string
		change func([]aiGatewayChildCollector) []aiGatewayChildCollector
		want   string
	}{
		{
			name:   "missing direct children are sorted",
			change: func(c []aiGatewayChildCollector) []aiGatewayChildCollector { return c[2:] },
			want: fmt.Sprintf("missing resource types [%s %s]",
				declresources.ResourceTypeAIGatewayAuthStrategy, declresources.ResourceTypeAIGatewayProvider),
		},
		{
			name: "missing nested child",
			change: func(c []aiGatewayChildCollector) []aiGatewayChildCollector {
				c[4].nested = nil
				return c
			},
			want: fmt.Sprintf("missing resource types [%s]", declresources.ResourceTypeAIGatewayConsumerCredential),
		},
		{
			name:   "duplicate direct child",
			change: func(c []aiGatewayChildCollector) []aiGatewayChildCollector { return append(c, c[0]) },
			want:   "more than once",
		},
		{
			name: "duplicate nested child",
			change: func(c []aiGatewayChildCollector) []aiGatewayChildCollector {
				c[4].nested = append(c[4].nested, c[4].nested[0])
				return c
			},
			want: "more than once",
		},
		{
			name: "wrong nested owner",
			change: func(c []aiGatewayChildCollector) []aiGatewayChildCollector {
				c[0].nested, c[4].nested = c[4].nested, nil
				return c
			},
			want: fmt.Sprintf("%s is not a managed child of %s",
				declresources.ResourceTypeAIGatewayConsumerCredential, declresources.ResourceTypeAIGatewayProvider),
		},
		{
			name: "other family",
			change: func(c []aiGatewayChildCollector) []aiGatewayChildCollector {
				c[0].kind = declresources.ResourceTypePortalPage
				return c
			},
			want: "is not a managed child",
		},
		{
			name: "root",
			change: func(c []aiGatewayChildCollector) []aiGatewayChildCollector {
				c[0].kind = declresources.ResourceTypeAIGateway
				return c
			},
			want: "is not a managed child",
		},
		{
			name: "missing collector",
			change: func(c []aiGatewayChildCollector) []aiGatewayChildCollector {
				c[0].collect = nil
				return c
			},
			want: "requires a collector and warning",
		},
		{
			name: "blank warning",
			change: func(c []aiGatewayChildCollector) []aiGatewayChildCollector {
				c[0].warning = "  "
				return c
			},
			want: "requires a collector and warning",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			collectors := slices.Clone(aiGatewayChildCollectors)
			for i := range collectors {
				collectors[i].nested = slices.Clone(collectors[i].nested)
			}
			require.ErrorContains(t, validateAIGatewayChildCollectors(tt.change(collectors)), tt.want)
		})
	}
}

type aiChildDumpHTTPClient func(*http.Request) (*http.Response, error)

func (f aiChildDumpHTTPClient) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestPopulateAIGatewayChildrenExportContract(t *testing.T) {
	for _, failProviders := range []bool{false, true} {
		t.Run(fmt.Sprintf("provider-error=%t", failProviders), func(t *testing.T) {
			var calls []string
			httpClient := aiChildDumpHTTPClient(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodGet, req.Method)
				const prefix = "/v1/ai-gateways/gateway-id/"
				require.True(t, strings.HasPrefix(req.URL.Path, prefix), req.URL.Path)
				path := strings.TrimPrefix(req.URL.Path, prefix)
				calls = append(calls, path)
				data := "[]"
				switch path {
				case "consumers":
					data = `[{"id":"consumer-id","name":"consumer-name","type":"api-key","display_name":"Consumer",` +
						`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]`
				case "consumers/consumer-id/credentials":
					data = `[{"id":"credential-id","name":"key-name","type":"api-key","api_key":"never-export",` +
						`"display_name":"Key","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]`
				case "config-stores":
					data = `[{"id":"store-id","name":"store-name","display_name":"Store",` +
						`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]`
				case "config-stores/store-id/secrets":
					data = `[{"key":"secret-key","value":"never-export",` +
						`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]`
				case "snis":
					data = `[{"id":"sni-id","name":"sni-name","certificate":"certificate-name","hostname":"example.test",` +
						`"display_name":"SNI","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]`
				}
				status := http.StatusOK
				body := `{"data":` + data + `,"meta":{"page":{"size":100}}}`
				if failProviders && path == "model-providers" {
					status, body = http.StatusForbidden, `{"message":"denied"}`
				}
				return &http.Response{
					StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
					Body: io.NopCloser(strings.NewReader(body)), Request: req,
				}, nil
			})
			sdk := &helpers.KonnectSDK{SDK: kkSDK.New(
				kkSDK.WithServerURL("https://example.test"), kkSDK.WithClient(httpClient),
			)}
			client := declstate.NewClient(declstate.ClientConfig{
				AIGatewayProvidersAPI:             sdk.GetAIGatewayProvidersAPI(),
				AIGatewayAuthStrategiesAPI:        sdk.GetAIGatewayAuthStrategiesAPI(),
				AIGatewayPoliciesAPI:              sdk.GetAIGatewayPoliciesAPI(),
				AIGatewayAgentsAPI:                sdk.GetAIGatewayAgentsAPI(),
				AIGatewayConsumersAPI:             sdk.GetAIGatewayConsumersAPI(),
				AIGatewayConsumerGroupsAPI:        sdk.GetAIGatewayConsumerGroupsAPI(),
				AIGatewayModelAPI:                 sdk.GetAIGatewayModelAPI(),
				AIGatewayMCPServersAPI:            sdk.GetAIGatewayMCPServersAPI(),
				AIGatewayConfigStoresAPI:          sdk.GetAIGatewayConfigStoresAPI(),
				AIGatewayVaultsAPI:                sdk.GetAIGatewayVaultsAPI(),
				AIGatewayDataPlaneCertificatesAPI: sdk.GetAIGatewayDataPlaneCertificatesAPI(),
				AIGatewayCertificatesAPI:          sdk.GetAIGatewayCertificatesAPI(),
				AIGatewayCACertificatesAPI:        sdk.GetAIGatewayCACertificatesAPI(),
				AIGatewaySNIsAPI:                  sdk.GetAIGatewaySNIsAPI(),
			})
			retained := []declresources.AIGatewayProviderResource{
				{BaseResource: declresources.BaseResource{Ref: "retained"}},
			}
			gateways := []declresources.AIGatewayResource{
				{BaseResource: declresources.BaseResource{Ref: "  "}},
				{
					BaseResource:           declresources.BaseResource{Ref: " gateway-id "},
					CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{DisplayName: "Gateway display name"},
					Providers:              retained,
				},
			}
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			populateAIGatewayChildren(t.Context(), logger, nil, gateways)
			require.Empty(t, calls, "nil client must skip traversal")
			populateAIGatewayChildren(t.Context(), logger, client, gateways)
			require.Equal(t, []string{
				"model-providers", "auth-strategies", "policies", "agents",
				"consumers", "consumers/consumer-id/credentials", "consumer-groups", "models", "mcp-servers",
				"config-stores", "config-stores/store-id/secrets", "vaults", "data-plane-certificates",
				"certificates", "ca-certificates", "snis",
			}, calls, logs.String())
			gateway := gateways[1]
			require.Equal(t, retained, gateway.Providers, "empty or failed reads must preserve existing output")
			require.Nil(t, gateway.Agents, "empty reads must not allocate destination slices")
			require.Len(t, gateway.Consumers, 1)
			require.Empty(t, gateway.Consumers[0].AIGateway)
			require.Len(t, gateway.Consumers[0].Credentials, 1)
			require.Empty(t, gateway.Consumers[0].Credentials[0].AIGatewayConsumer)
			require.Equal(t, "credential-id", gateway.Consumers[0].Credentials[0].Ref)
			require.Len(t, gateway.ConfigStores, 1)
			require.Empty(t, gateway.ConfigStores[0].AIGateway)
			require.Len(t, gateway.ConfigStores[0].Secrets, 1)
			require.Equal(t, "secret-key", gateway.ConfigStores[0].Secrets[0].Key)
			require.Equal(t, "store-id", gateway.ConfigStores[0].Secrets[0].AIGatewayConfigStore)
			require.Len(t, gateway.SNIs, 1)
			require.Empty(t, gateway.SNIs[0].AIGateway)
			require.Equal(t, "sni-name", gateway.SNIs[0].Name)
			// Marshal only exported children; the retained provider is a sentinel, not an SDK union.
			gateway.Providers = nil
			output, err := yaml.Marshal(gateway)
			require.NoError(t, err)
			require.NotContains(t, string(output), "never-export")
			if failProviders {
				require.Contains(t, logs.String(), "failed to load AI Gateway Model Providers")
				require.Contains(t, logs.String(), "gateway-id")
				require.Contains(t, logs.String(), "Gateway display name")
				require.Equal(t, 1, strings.Count(logs.String(), "level=WARN"))
			} else {
				require.Empty(t, logs.String())
			}
		})
	}
}
