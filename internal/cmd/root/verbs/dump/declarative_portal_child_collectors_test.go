package dump

import (
	"bytes"
	"encoding/json"
	"errors"
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

func TestPortalChildCollectorCoverage(t *testing.T) {
	collectorIndex := func(kind declresources.ResourceType) int {
		t.Helper()
		index := slices.IndexFunc(portalChildCollectors, func(c portalChildCollector) bool { return c.kind == kind })
		require.NotEqual(t, -1, index, "missing collector for %s", kind)
		return index
	}
	teams := collectorIndex(declresources.ResourceTypePortalTeam)
	assets := collectorIndex(declresources.ResourceTypePortalAssetLogo)
	require.NoError(t, validatePortalChildCollectors(portalChildCollectors, portalChildOmissions))
	for _, tt := range []struct {
		name   string
		change func([]portalChildCollector, []childExportOmission) ([]portalChildCollector, []childExportOmission)
		want   string
	}{
		{
			"missing omission",
			func(c []portalChildCollector, _ []childExportOmission) ([]portalChildCollector, []childExportOmission) {
				return c, nil
			},
			"missing resource types [portal_team_group_mapping]",
		},
		{
			"empty omission reason",
			func(c []portalChildCollector, o []childExportOmission) ([]portalChildCollector, []childExportOmission) {
				o[0].reason = " "
				return c, o
			},
			"requires an omission reason",
		},
		{
			"overlapping omission",
			func(c []portalChildCollector, o []childExportOmission) ([]portalChildCollector, []childExportOmission) {
				o[0].kind = declresources.ResourceTypePortalPage
				return c, o
			},
			"more than once",
		},
		{
			"foreign omission",
			func(c []portalChildCollector, o []childExportOmission) ([]portalChildCollector, []childExportOmission) {
				o[0].kind = declresources.ResourceTypeAPIVersion
				return c, o
			},
			"is not a managed child",
		},
		{
			"missing team roles",
			func(c []portalChildCollector, o []childExportOmission) ([]portalChildCollector, []childExportOmission) {
				c[teams].coScoped = nil
				return c, o
			},
			"missing resource types [portal_team_role]",
		},
		{
			"incorrect role ownership",
			func(c []portalChildCollector, o []childExportOmission) ([]portalChildCollector, []childExportOmission) {
				c[teams].nested, c[teams].coScoped = c[teams].coScoped, nil
				return c, o
			},
			"is not a managed child of portal_team",
		},
		{
			"missing favicon",
			func(c []portalChildCollector, o []childExportOmission) ([]portalChildCollector, []childExportOmission) {
				c[assets].coScoped = nil
				return c, o
			},
			"missing resource types [portal_asset_favicon]",
		},
		{
			"duplicate shared owner",
			func(c []portalChildCollector, o []childExportOmission) ([]portalChildCollector, []childExportOmission) {
				c[teams].coScoped = []declresources.ResourceType{declresources.ResourceTypePortalTeam}
				return c, o
			},
			"more than once",
		},
		{
			"foreign shared owner",
			func(c []portalChildCollector, o []childExportOmission) ([]portalChildCollector, []childExportOmission) {
				c[teams].coScoped = []declresources.ResourceType{declresources.ResourceTypeAPIVersion}
				return c, o
			},
			"is not a managed child",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, o := tt.change(slices.Clone(portalChildCollectors), slices.Clone(portalChildOmissions))
			require.ErrorContains(t, validatePortalChildCollectors(c, o), tt.want)
		})
	}
}

type portalChildDumpHTTPClient func(*http.Request) (*http.Response, error)

func (f portalChildDumpHTTPClient) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestPopulatePortalChildrenExportContract(t *testing.T) {
	for _, tt := range []struct {
		name, failure, warning                       string
		empty, invalidPage, fatal, customCertificate bool
	}{
		{name: "success"},
		{
			name: "unexportable singleton retains existing value", customCertificate: true,
			warning: "portal custom domain uses custom_certificate; skipping",
		},
		{name: "empty collections and missing singletons", empty: true, warning: "failed to load portal auth settings"},
		{name: "page list failure stops all portals", failure: "pages", fatal: true},
		{name: "page conversion failure stops all portals", invalidPage: true, fatal: true},
		{name: "page detail failure continues", failure: "pages/page-id", warning: "failed to fetch portal page"},
		{
			name: "singleton failure retains existing value", failure: "authentication-settings",
			warning: "failed to load portal auth settings",
		},
		{
			name: "map failure retains existing value", failure: "email-templates",
			warning: "failed to load portal email templates",
		},
		{
			name: "team survives role failure", failure: "teams/team-id/assigned-roles",
			warning: "failed to load portal team roles",
		},
		{name: "favicon survives logo failure", failure: "assets/logo", warning: "failed to fetch portal logo"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			payloads := portalChildDumpPayloads()
			if tt.invalidPage {
				payloads["pages/page-id"] = kkComps.PortalPageResponse{
					ID:      "page-id",
					Slug:    "bad/path",
					Content: "# Guide",
				}
			}
			if tt.customCertificate {
				payloads["custom-domain"] = kkComps.PortalCustomDomain{
					Ssl: kkComps.PortalCustomDomainSSL{DomainVerificationMethod: "custom_certificate"},
				}
			}
			var calls []string
			httpClient := portalChildDumpHTTPClient(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodGet, req.Method)
				require.True(t, strings.HasPrefix(req.URL.Path, "/v3/portals/portal-id/"), req.URL.Path)
				path := strings.TrimPrefix(req.URL.Path, "/v3/portals/portal-id/")
				calls = append(calls, path)
				payload, ok := payloads[path]
				require.True(t, ok, "unexpected request: %s", path)
				status := http.StatusOK
				if tt.empty {
					switch path {
					case "identity-providers":
						payload = []any{}
					case "pages", "snippets", "teams", "ip-allow-list", "email-templates":
						payload = map[string]any{
							"data": []any{},
							"meta": map[string]any{"size": 100, "page": map[string]any{"size": 100}},
						}
					default:
						status, payload = http.StatusNotFound, map[string]any{"message": "not found"}
					}
				}
				if path == tt.failure {
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
				PortalPageAPI:             sdk.GetPortalPageAPI(),
				PortalSnippetAPI:          sdk.GetPortalSnippetAPI(),
				PortalTeamAPI:             sdk.GetPortalTeamAPI(),
				PortalTeamRolesAPI:        sdk.GetPortalTeamRolesAPI(),
				PortalAuthSettingsAPI:     sdk.GetPortalAuthSettingsAPI(),
				PortalIPAllowListAPI:      sdk.GetPortalIPAllowListAPI(),
				PortalIntegrationsAPI:     sdk.GetPortalIntegrationsAPI(),
				PortalIdentityProviderAPI: sdk.GetPortalIdentityProviderAPI(),
				PortalCustomizationAPI:    sdk.GetPortalCustomizationAPI(),
				PortalCustomDomainAPI:     sdk.GetPortalCustomDomainAPI(),
				PortalEmailsAPI:           sdk.GetPortalEmailsAPI(),
				PortalAuditLogsAPI:        sdk.GetPortalAuditLogsAPI(),
				AssetsAPI:                 sdk.GetAssetsAPI(),
			})
			before := declresources.PortalResource{
				BaseResource: declresources.BaseResource{
					Ref: " portal-id ",
				}, CreatePortal: kkComps.CreatePortal{Name: "Portal name"},
				Pages:          []declresources.PortalPageResource{{Ref: "existing"}},
				AuthSettings:   &declresources.PortalAuthSettingsResource{Ref: "existing"},
				EmailTemplates: map[string]declresources.PortalEmailTemplateResource{"existing": {Ref: "existing"}},
				CustomDomain:   &declresources.PortalCustomDomainResource{Ref: "existing"},
				Assets:         &declresources.PortalAssetsResource{Logo: new("existing")},
			}
			portals := []declresources.PortalResource{{BaseResource: declresources.BaseResource{Ref: " "}}, before}
			if tt.fatal {
				portals = append(
					portals,
					declresources.PortalResource{BaseResource: declresources.BaseResource{Ref: "later-portal"}},
				)
			}
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			require.NoError(t, populatePortalChildren(t.Context(), logger, nil, portals))
			require.Empty(t, calls)
			err := populatePortalChildren(t.Context(), logger, client, portals)
			if tt.fatal {
				require.ErrorContains(t, err, "portal \"Portal name\" (portal-id): ")
				cause := errors.Unwrap(err)
				require.Error(t, cause)
				want := []string{"pages"}
				if tt.invalidPage {
					want = append(want, "pages/page-id")
					require.EqualError(
						t,
						cause,
						"portal page \"page-id\" in portal \"Portal name\": slug must be a single path segment (got \"bad/path\")",
					)
					require.ErrorContains(t, err, "Portal name")
				} else {
					require.ErrorContains(t, err, "failed to list portal pages")
				}
				require.Equal(t, want, calls)
				require.Equal(t, before, portals[1])
				require.Empty(t, logs.String())
				return
			}
			require.NoError(t, err)
			wantCalls := []string{"pages"}
			if !tt.empty {
				wantCalls = append(wantCalls, "pages/page-id")
			}
			wantCalls = append(wantCalls, "snippets", "teams")
			if !tt.empty {
				wantCalls = append(wantCalls, "teams/team-id/assigned-roles")
			}
			wantCalls = append(wantCalls, "identity-providers", "authentication-settings", "ip-allow-list",
				"integrations", "customization", "custom-domain", "email-config", "email-templates",
				"audit-log-webhook", "assets/logo", "assets/favicon")
			require.Equal(t, wantCalls, calls)
			if tt.warning == "" {
				require.Empty(t, logs.String())
			} else {
				require.Contains(t, logs.String(), tt.warning)
				if !tt.empty {
					require.Equal(t, 1, strings.Count(logs.String(), "level=WARN"))
				}
				require.Contains(t, logs.String(), "portal-id")
				require.Contains(t, logs.String(), "Portal name")
			}
			portal := portals[1]
			if tt.empty {
				require.Equal(t, before, portal)
				return
			}
			require.Len(t, portal.Pages, 1)
			if tt.failure == "pages/page-id" {
				require.Equal(t, before.Pages, portal.Pages)
			} else {
				require.Equal(t, "page-id", portal.Pages[0].Ref)
				require.Equal(t, "guide", portal.Pages[0].Slug)
				require.Empty(t, portal.Pages[0].Portal)
			}
			require.Len(t, portal.Teams, 1)
			require.Equal(t, "team-id", portal.Teams[0].Ref)
			require.Empty(t, portal.Teams[0].Portal)
			if tt.failure == "teams/team-id/assigned-roles" {
				require.Empty(t, portal.Teams[0].Roles)
			} else {
				require.Len(t, portal.Teams[0].Roles, 1)
				require.Equal(t, "role-id", portal.Teams[0].Roles[0].Ref)
				require.Empty(t, portal.Teams[0].Roles[0].Team)
				require.Empty(t, portal.Teams[0].Roles[0].Portal)
			}
			if tt.failure == "authentication-settings" {
				require.Same(t, before.AuthSettings, portal.AuthSettings)
			} else {
				require.NotNil(t, portal.AuthSettings)
				require.True(t, *portal.AuthSettings.BasicAuthEnabled)
			}
			require.NotNil(t, portal.IPAllowList)
			require.Equal(t, []string{"192.0.2.1"}, portal.IPAllowList.AllowedIPs)
			require.NotNil(t, portal.Integrations)
			require.NotNil(t, portal.Customization)
			require.Equal(t, "body {}", *portal.Customization.CSS)
			require.NotNil(t, portal.CustomDomain)
			if tt.customCertificate {
				require.Same(t, before.CustomDomain, portal.CustomDomain)
			} else {
				require.Equal(t, "portal.example.test", portal.CustomDomain.Hostname)
			}
			require.NotNil(t, portal.EmailConfig)
			require.Equal(t, "email-id", portal.EmailConfig.Ref)
			if tt.failure == "email-templates" {
				require.Equal(t, before.EmailTemplates, portal.EmailTemplates)
			} else {
				require.Len(t, portal.EmailTemplates, 1)
				require.True(t, *portal.EmailTemplates["developer-welcome"].Enabled)
			}
			require.NotNil(t, portal.AuditLogWebhook)
			require.True(t, *portal.AuditLogWebhook.Enabled)
			require.NotNil(t, portal.Assets)
			require.Equal(t, "favicon-data", *portal.Assets.Favicon)
			if tt.failure == "assets/logo" {
				require.Nil(t, portal.Assets.Logo)
			} else {
				require.Equal(t, "logo-data", *portal.Assets.Logo)
			}
		})
	}
}

func portalChildDumpPayloads() map[string]any {
	list := func(data any) any {
		return map[string]any{"data": data, "meta": map[string]any{"size": 100, "page": map[string]any{"size": 100}}}
	}
	return map[string]any{
		"pages":         list([]kkComps.PortalPageInfo{{ID: "page-id", Slug: "guide"}}),
		"pages/page-id": kkComps.PortalPageResponse{ID: "page-id", Slug: "/guide/", Content: "# Guide"},
		"snippets":      list([]any{}),
		"teams":         list([]kkComps.PortalTeamResponse{{ID: "team-id", Name: "Team"}}),
		"teams/team-id/assigned-roles": list([]kkComps.PortalAssignedRoleResponse{{
			ID: "role-id", RoleName: "API Viewer", EntityID: "api-id", EntityTypeName: "APIs",
		}}),
		"identity-providers":      []any{},
		"authentication-settings": kkComps.PortalAuthenticationSettingsResponse{BasicAuthEnabled: true},
		"ip-allow-list":           list([]any{map[string]any{"id": "allow-id", "allowed_ips": []string{"192.0.2.1"}}}),
		"integrations": map[string]any{
			"google_tag_manager": map[string]any{"enabled": true, "container_id": "GTM-example"},
		},
		"customization": kkComps.PortalCustomization{CSS: new("body {}")},
		"custom-domain": kkComps.PortalCustomDomain{
			Hostname: "portal.example.test", Enabled: true, Ssl: kkComps.PortalCustomDomainSSL{DomainVerificationMethod: "http"},
		},
		"email-config":      kkComps.PortalEmailConfig{ID: "email-id", FromName: new("Portal")},
		"email-templates":   list([]kkComps.EmailTemplate{{Name: "developer-welcome", Enabled: true}}),
		"audit-log-webhook": kkComps.PortalAuditLogWebhook{Enabled: new(true)},
		"assets/logo":       kkComps.PortalAssetResponse{Data: "logo-data"},
		"assets/favicon":    kkComps.PortalAssetResponse{Data: "favicon-data"},
	}
}
