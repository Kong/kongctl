package dump

import (
	"bytes"
	"encoding/json"
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
)

func TestEventGatewayChildCollectorCoverage(t *testing.T) {
	validate := func(c []eventGatewayChildCollector) error {
		return validateChildCollectors("Event Gateway child dump", c)
	}
	require.NoError(t, validate(eventGatewayChildCollectors))
	for _, tt := range []struct {
		name   string
		change func([]eventGatewayChildCollector) []eventGatewayChildCollector
		want   string
	}{
		{
			name:   "missing direct children",
			change: func(c []eventGatewayChildCollector) []eventGatewayChildCollector { return c[1:6] },
			want: fmt.Sprintf("missing resource types [%s %s]",
				declresources.ResourceTypeEventGatewayBackendCluster, declresources.ResourceTypeEventGatewayTLSTrustBundle),
		},
		{
			name: "missing nested policies",
			change: func(c []eventGatewayChildCollector) []eventGatewayChildCollector {
				c[1].nested = nil
				return c
			},
			want: fmt.Sprintf("missing resource types [%s %s %s]",
				declresources.ResourceTypeEventGatewayClusterPolicy,
				declresources.ResourceTypeEventGatewayConsumePolicy,
				declresources.ResourceTypeEventGatewayProducePolicy),
		},
		{
			name:   "duplicate collector",
			change: func(c []eventGatewayChildCollector) []eventGatewayChildCollector { return append(c, c[0]) },
			want:   "more than once",
		},
		{
			name: "wrong policy owner",
			change: func(c []eventGatewayChildCollector) []eventGatewayChildCollector {
				c[0].nested, c[2].nested = c[2].nested, nil
				return c
			},
			want: fmt.Sprintf("%s is not a managed child of %s",
				declresources.ResourceTypeEventGatewayListenerPolicy, declresources.ResourceTypeEventGatewayBackendCluster),
		},
		{
			name: "other gateway family",
			change: func(c []eventGatewayChildCollector) []eventGatewayChildCollector {
				c[0].kind = declresources.ResourceTypeAIGatewayProvider
				return c
			},
			want: "is not a managed child",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := slices.Clone(eventGatewayChildCollectors)
			require.ErrorContains(t, validate(tt.change(c)), tt.want)
		})
	}
	t.Run("child used as root", func(t *testing.T) {
		err := validateChildCollectors("invalid root",
			[]childCollector[declresources.EventGatewayListenerResource]{})
		require.ErrorContains(t, err, "requires a managed root")
	})
}

type eventChildDumpHTTPClient func(*http.Request) (*http.Response, error)

func (f eventChildDumpHTTPClient) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestPopulateEventGatewayChildrenExportContract(t *testing.T) {
	for _, tt := range []struct {
		name            string
		failurePath     string
		malformedPolicy bool
		warning         string
	}{
		{name: "success"},
		{
			name: "failed direct collection", failurePath: "backend-clusters",
			warning: "failed to load event gateway backend clusters",
		},
		{
			name: "failed cluster policy", failurePath: "virtual-clusters/vc-id/cluster-policies",
			warning: "failed to load cluster policies",
		},
		{
			name: "failed listener policy", failurePath: "listeners/listener-id/policies",
			warning: "failed to load listener policies",
		},
		{
			name: "malformed individual policy", malformedPolicy: true,
			warning: "failed to convert cluster policy",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			httpClient := eventChildDumpHTTPClient(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodGet, req.Method)
				const prefix = "/v1/event-gateways/gateway-id/"
				require.True(t, strings.HasPrefix(req.URL.Path, prefix), req.URL.Path)
				path := strings.TrimPrefix(req.URL.Path, prefix)
				calls = append(calls, path)
				data := []any{}
				switch path {
				case "virtual-clusters":
					data = append(data, kkComps.VirtualCluster{
						ID: "vc-id", Name: "virtual-cluster", DNSLabel: "vc", ACLMode: "passthrough",
						Destination:    kkComps.BackendClusterReference{ID: "backend-id", Name: "backend"},
						Authentication: []kkComps.VirtualClusterAuthenticationSensitiveDataAwareScheme{},
					})
				case "listeners":
					data = append(data, kkComps.EventGatewayListener{
						ID: "listener-id", Name: "listener", Addresses: []string{}, Ports: []kkComps.EventGatewayListenerPort{},
					})
				case "virtual-clusters/vc-id/cluster-policies":
					policy := map[string]any{
						"id": "policy-id", "name": "policy", "type": "acls", "config": map[string]any{"rules": []any{}},
						"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
					}
					if tt.malformedPolicy {
						bad := map[string]any{
							"id": "bad-policy", "type": "unsupported", "config": map[string]any{},
							"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
						}
						data = append(data, bad)
					}
					data = append(data, policy)
				case "tls-trust-bundles":
					data = append(data, kkComps.TLSTrustBundle{
						ID: "bundle-id", Name: "bundle", Config: kkComps.TLSTrustBundleConfig{TrustedCa: "public-ca"},
					})
				}
				var payload any = map[string]any{"data": data, "meta": map[string]any{"page": map[string]any{"size": 100}}}
				if strings.HasSuffix(path, "policies") {
					payload = data
				}
				status := http.StatusOK
				if path == tt.failurePath {
					status, payload = http.StatusForbidden, map[string]any{"message": "denied"}
				}
				body, err := json.Marshal(payload)
				require.NoError(t, err)
				return &http.Response{
					StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
					Body: io.NopCloser(bytes.NewReader(body)), Request: req,
				}, nil
			})
			sdk := &helpers.KonnectSDK{SDK: kkSDK.New(
				kkSDK.WithServerURL("https://example.test"), kkSDK.WithClient(httpClient),
			)}
			client := declstate.NewClient(declstate.ClientConfig{
				EventGatewayBackendClusterAPI:       sdk.GetEventGatewayBackendClusterAPI(),
				EventGatewayVirtualClusterAPI:       sdk.GetEventGatewayVirtualClusterAPI(),
				EventGatewayListenerAPI:             sdk.GetEventGatewayListenerAPI(),
				EventGatewayClusterPolicyAPI:        sdk.GetEventGatewayClusterPolicyAPI(),
				EventGatewayProducePolicyAPI:        sdk.GetEventGatewayProducePolicyAPI(),
				EventGatewayConsumePolicyAPI:        sdk.GetEventGatewayConsumePolicyAPI(),
				EventGatewayListenerPolicyAPI:       sdk.GetEventGatewayListenerPolicyAPI(),
				EventGatewayDataPlaneCertificateAPI: sdk.GetEventGatewayDataPlaneCertificateAPI(),
				EventGatewaySchemaRegistryAPI:       sdk.GetEventGatewaySchemaRegistryAPI(),
				EventGatewayStaticKeyAPI:            sdk.GetEventGatewayStaticKeyAPI(),
				EventGatewayTLSTrustBundleAPI:       sdk.GetEventGatewayTLSTrustBundleAPI(),
			})
			retained := []declresources.EventGatewayBackendClusterResource{{Ref: "retained"}}
			gateways := []declresources.EventGatewayControlPlaneResource{
				{BaseResource: declresources.BaseResource{Ref: "  "}},
				{
					BaseResource:         declresources.BaseResource{Ref: " gateway-id "},
					CreateGatewayRequest: kkComps.CreateGatewayRequest{Name: "gateway-name"},
					BackendClusters:      retained,
				},
			}
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			populateEventGatewayChildren(t.Context(), logger, nil, gateways)
			require.Empty(t, calls, "nil client must skip traversal")
			populateEventGatewayChildren(t.Context(), logger, client, gateways)
			require.Equal(t, []string{
				"backend-clusters", "virtual-clusters", "virtual-clusters/vc-id/cluster-policies",
				"virtual-clusters/vc-id/produce-policies", "virtual-clusters/vc-id/consume-policies",
				"listeners", "listeners/listener-id/policies", "data-plane-certificates", "schema-registries",
				"static-keys", "tls-trust-bundles",
			}, calls, logs.String())
			gateway := gateways[1]
			require.Equal(t, retained, gateway.BackendClusters, "empty or failed reads must preserve existing output")
			require.Nil(t, gateway.StaticKeys, "empty reads must not allocate destination slices")
			require.Len(t, gateway.VirtualClusters, 1, "nested policy errors must retain the parent")
			require.Empty(t, gateway.VirtualClusters[0].EventGateway)
			if tt.failurePath == "virtual-clusters/vc-id/cluster-policies" {
				require.Empty(t, gateway.VirtualClusters[0].ClusterPolicies)
			} else {
				require.Len(t, gateway.VirtualClusters[0].ClusterPolicies, 1, logs.String())
				require.Equal(t, "policy-id", gateway.VirtualClusters[0].ClusterPolicies[0].Ref)
				require.Empty(t, gateway.VirtualClusters[0].ClusterPolicies[0].VirtualCluster)
			}
			require.Len(t, gateway.Listeners, 1, "nested listener policy errors must retain the listener")
			require.Empty(t, gateway.Listeners[0].EventGateway)
			require.Len(t, gateway.TrustBundles, 1, "later collectors must run after errors")
			require.Empty(t, gateway.TrustBundles[0].EventGateway)
			require.Equal(t, "bundle-id", gateway.TrustBundles[0].Ref)
			if tt.warning == "" {
				require.Empty(t, logs.String())
			} else {
				require.Contains(t, logs.String(), tt.warning)
				require.Equal(t, 1, strings.Count(logs.String(), "level=WARN"), logs.String())
			}
		})
	}
}
