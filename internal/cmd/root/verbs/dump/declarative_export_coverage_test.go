package dump

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
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

func dumpCoverageRegistrations() []declarativeCollector {
	var registrations []declarativeCollector
	for _, selector := range slices.Sorted(maps.Keys(declarativeCollectors)) {
		collector := declarativeCollectors[selector]
		collector.children = slices.Clone(collector.children)
		registrations = append(registrations, collector)
	}
	// Deliberately omitted roots have no entry in the dispatch map.
	return append(registrations, declarativeCollector{
		kind: declresources.ResourceTypeCatalogService, omittedReason: "Catalog services do not support dump",
	})
}

func TestDeclarativeDumpManagedCoverage(t *testing.T) {
	_, err := indexDeclarativeCollectors(dumpCoverageRegistrations())
	require.NoError(t, err)

	for _, kind := range []declresources.ResourceType{
		declresources.ResourceTypeControlPlaneDataPlaneCertificate,
		declresources.ResourceTypeOrganizationTeamRole,
		declresources.ResourceTypeOrganizationUserTeamMembership,
		declresources.ResourceTypeOrganizationUserRole,
		declresources.ResourceTypeOrganizationSystemAccountTeamMembership,
		declresources.ResourceTypeOrganizationSystemAccountRole,
		declresources.ResourceTypeAIGatewayConfigStoreSecret,
		declresources.ResourceTypePortalTeamRole,
		declresources.ResourceTypePortalAssetFavicon,
		declresources.ResourceTypePortalTeamGroupMapping,
	} {
		t.Run("missing/"+string(kind), func(t *testing.T) {
			registrations := dumpCoverageRegistrations()
			found := false
			for i := range registrations {
				if j := slices.Index(registrations[i].children, kind); j >= 0 {
					registrations[i].children = slices.Delete(registrations[i].children, j, j+1)
					found = true
				}
			}
			require.True(t, found, "the runtime registration must account for this kind")
			_, err := indexDeclarativeCollectors(registrations)
			require.EqualError(t, err, fmt.Sprintf("declarative dump is missing resource types [%s]", kind))
		})
	}

	for _, tt := range []struct {
		kind declresources.ResourceType
		want string
	}{
		{declresources.ResourceTypeControlPlaneDataPlaneCertificate, "more than once"},
		{declresources.ResourceTypeControlPlane, "more than once"},
		{declresources.ResourceTypeGatewayService, "unexpected resource type"},
		{declresources.ResourceTypeOrganizationUser, "unexpected resource type"},
		{declresources.ResourceTypeOrganizationSystemAccount, "unexpected resource type"},
	} {
		t.Run("extra/"+string(tt.kind), func(t *testing.T) {
			registrations := dumpCoverageRegistrations()
			registrations[0].children = append(registrations[0].children, tt.kind)
			_, err := indexDeclarativeCollectors(registrations)
			require.ErrorContains(t, err, tt.want)
			require.ErrorContains(t, err, string(tt.kind))
		})
	}

	t.Run("omitted root cannot claim children", func(t *testing.T) {
		registrations := dumpCoverageRegistrations()
		registrations[len(registrations)-1].children = []declresources.ResourceType{
			declresources.ResourceTypeControlPlaneDataPlaneCertificate,
		}
		_, err := indexDeclarativeCollectors(registrations)
		require.ErrorContains(t, err, "omission conflicts with collector")
	})

	t.Run("coverage requires population", func(t *testing.T) {
		require.PanicsWithValue(
			t,
			"declarative dump control_planes requires child population for child coverage",
			func() {
				rootCollector(
					"control_planes",
					func(_ context.Context, _ *declarativeDumpContext) ([]declresources.ControlPlaneResource, error) {
						return nil, nil
					},
					func(rs *declresources.ResourceSet) *[]declresources.ControlPlaneResource { return &rs.ControlPlanes },
					nil,
					declresources.ResourceTypeControlPlaneDataPlaneCertificate,
				)
			},
		)
	})
}

type exportCoverageHTTPClient func(*http.Request) (*http.Response, error)

func (f exportCoverageHTTPClient) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestControlPlaneChildExportTraversal(t *testing.T) {
	const servicesPath = "core-entities/services"
	const certificatesPath = "dp-client-certificates"
	for _, tt := range []struct {
		name        string
		failurePath string
		empty       bool
		warning     string
	}{
		{name: "services precede certificates"},
		{name: "service failure continues", failurePath: servicesPath, warning: "failed to load gateway services"},
		{
			name: "certificate failure retains output", failurePath: certificatesPath,
			warning: "failed to load data plane certificates",
		},
		{name: "empty results retain output", empty: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			httpClient := exportCoverageHTTPClient(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodGet, req.Method)
				const prefix = "/v2/control-planes/cp-id/"
				require.True(t, strings.HasPrefix(req.URL.Path, prefix), req.URL.Path)
				path := strings.TrimPrefix(req.URL.Path, prefix)
				calls = append(calls, path)
				status := http.StatusOK
				var body string
				switch path {
				case servicesPath:
					body = `{"data":[{"id":"service-id","name":"service-name"}]}`
					if tt.empty {
						body = `{"data":[]}`
					}
				case certificatesPath:
					body = `{"items":[{"id":"certificate-id","cert":"public-cert"}]}`
					if tt.empty {
						body = `{"items":[]}`
					}
				default:
					t.Fatalf("unexpected request: %s", req.URL)
				}
				if path == tt.failurePath {
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
				GatewayServiceAPI: sdk.GetGatewayServiceAPI(), DataPlaneCertificateAPI: sdk.GetDataPlaneCertificateAPI(),
			})
			retainedServices := []declresources.GatewayServiceResource{{Ref: "retained-service"}}
			retainedCertificates := []declresources.ControlPlaneDataPlaneCertificateResource{
				{Ref: "retained-certificate"},
			}
			planes := []declresources.ControlPlaneResource{
				{BaseResource: declresources.BaseResource{Ref: "  "}},
				{
					BaseResource:              declresources.BaseResource{Ref: " cp-id "},
					CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{Name: "cp-name"},
					GatewayServices:           retainedServices, DataPlaneCertificates: retainedCertificates,
				},
			}
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			populateControlPlaneChildren(t.Context(), logger, nil, planes)
			require.Empty(t, calls, "nil clients must skip traversal")
			populateControlPlaneChildren(t.Context(), logger, client, planes)
			require.Equal(t, []string{servicesPath, certificatesPath}, calls, logs.String())
			cp := planes[1]
			if tt.empty || tt.failurePath == servicesPath {
				require.Equal(t, retainedServices, cp.GatewayServices)
			} else {
				require.Len(t, cp.GatewayServices, 1)
				require.Equal(t, "service-id", cp.GatewayServices[0].External.ID)
				require.Empty(t, cp.GatewayServices[0].ControlPlane)
			}
			if tt.empty || tt.failurePath == certificatesPath {
				require.Equal(t, retainedCertificates, cp.DataPlaneCertificates)
			} else {
				require.Len(t, cp.DataPlaneCertificates, 1)
				require.Equal(t, "certificate-id", cp.DataPlaneCertificates[0].Ref)
				require.Equal(t, "public-cert", cp.DataPlaneCertificates[0].Cert)
				require.Empty(t, cp.DataPlaneCertificates[0].ControlPlane)
			}
			if tt.warning == "" {
				require.Empty(t, logs.String())
			} else {
				require.Contains(t, logs.String(), tt.warning)
				require.Contains(t, logs.String(), "cp-id")
				require.Contains(t, logs.String(), "cp-name")
				require.Equal(t, 1, strings.Count(logs.String(), "level=WARN"))
			}
		})
	}
}
