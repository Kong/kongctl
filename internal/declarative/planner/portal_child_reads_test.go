package planner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdk "github.com/Kong/sdk-konnect-go"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

type portalChildReadsTransport struct {
	calls []string
}

func (a *portalChildReadsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	a.calls = append(a.calls, req.URL.Path)
	return &http.Response{
		StatusCode: http.StatusForbidden,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"message":"forbidden"}`)),
		Request:    req,
	}, nil
}

func portalChildReadsConfig(transport http.RoundTripper) state.ClientConfig {
	api := &helpers.KonnectSDK{SDK: sdk.New(
		sdk.WithServerURL("https://portal.test"), sdk.WithClient(&http.Client{Transport: transport}),
	)}
	return state.ClientConfig{
		PortalPageAPI: api.GetPortalPageAPI(), PortalSnippetAPI: api.GetPortalSnippetAPI(),
		PortalTeamAPI: api.GetPortalTeamAPI(), PortalTeamRolesAPI: api.GetPortalTeamRolesAPI(),
		PortalAuthSettingsAPI: api.GetPortalAuthSettingsAPI(), PortalIPAllowListAPI: api.GetPortalIPAllowListAPI(),
		PortalIntegrationsAPI: api.GetPortalIntegrationsAPI(), PortalIdentityProviderAPI: api.GetPortalIdentityProviderAPI(),
		PortalCustomizationAPI: api.GetPortalCustomizationAPI(), PortalCustomDomainAPI: api.GetPortalCustomDomainAPI(),
		PortalEmailsAPI: api.GetPortalEmailsAPI(), PortalAuditLogsAPI: api.GetPortalAuditLogsAPI(),
		AssetsAPI: api.GetAssetsAPI(),
	}
}

func TestPortalParentOnlyPlanSkipsChildReads(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
			t.Run(fmt.Sprintf("external=%v/%s", external, mode), func(t *testing.T) {
				var logs bytes.Buffer
				transport := &portalChildReadsTransport{}
				root, _, _, cfg := portalTraversalFixture(t, true, external, "portal-id",
					portalChildReadsConfig(transport), &logs)
				plan := NewPlan(CurrentPlanVersion, "test", mode)
				require.NoError(t, root.PlanChanges(t.Context(), cfg, plan))
				require.Empty(t, transport.calls, "parent-only planning must work even when every child API returns 403")
			})
		}
	}
}

func TestPortalChildSelectionCoversRegisteredTypes(t *testing.T) {
	// Derive the umbrella from registrations so new portal child types also
	// exercise apply selection and sync key-presence policy.
	for _, collection := range resources.SyncCollections() {
		if collection.ParentType != resources.ResourceTypePortal {
			continue
		}
		t.Run(string(collection.ResourceType), func(t *testing.T) {
			for _, external := range []bool{false, true} {
				var logs bytes.Buffer
				root, p, desired, _ := portalTraversalFixture(t, true, external, "portal-id", state.ClientConfig{}, &logs)
				portal := root.(*portalPlannerImpl)
				apply := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
				sync := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
				kind := collection.ResourceType
				require.False(t, portal.shouldPlanPortalChild(apply, *desired, kind, false))
				require.True(t, portal.shouldPlanPortalChild(apply, *desired, kind, true))
				require.False(t, portal.shouldPlanPortalChild(sync, *desired, kind, false))
				p.resources.SyncScope.AddChild(resources.ResourceTypePortal, desired.Ref, kind)
				require.True(t, portal.shouldPlanPortalChild(sync, *desired, kind, false))
				require.False(t, portal.shouldPlanPortalChild(apply, *desired, kind, false))
				for _, mode := range []PlanMode{PlanModeDelete, "", "unknown"} {
					other := NewPlan(CurrentPlanVersion, "test", mode)
					require.Equal(t, !external, portal.shouldPlanPortalChild(other, *desired, kind, false),
						"preserve existing selection outside apply and sync: %s", mode)
				}
			}
		})
	}
}

func TestPortalCreateOnlySingletonSelection(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, declared := range []bool{false, true} {
			t.Run(fmt.Sprintf("external=%v/declared=%v", external, declared), func(t *testing.T) {
				var logs bytes.Buffer
				transport := &portalChildReadsTransport{}
				root, p, desired, cfg := portalTraversalFixture(t, false, external, "",
					portalChildReadsConfig(transport), &logs)
				p.desiredPortals = p.resources.Portals
				if declared {
					p.desiredPortalIPAllowLists = []resources.PortalIPAllowListResource{{
						Ref: "allow-list", Portal: desired.Ref, AllowedIPs: []string{"192.0.2.1"},
					}}
					p.desiredPortalAuthSettings = []resources.PortalAuthSettingsResource{{
						Ref: "auth", Portal: desired.Ref,
					}}
					p.desiredPortalAuthSettings[0].BasicAuthEnabled = new(true)
				}
				plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
				require.NoError(t, root.PlanChanges(t.Context(), cfg, plan))
				require.Empty(t, transport.calls, "create-only planning must not observe child APIs")
				require.Equal(t, declared, plan.HasChange(ResourceTypePortalIPAllowList, "allow-list"))
				require.Equal(t, declared, plan.HasChange(ResourceTypePortalAuthSettings, "auth"))
				for _, change := range plan.Changes {
					if change.ResourceType != ResourceTypePortal {
						require.True(t, declared, "omitted children must not synthesize singleton defaults")
					}
				}
			})
		}
	}
}

func TestPortalDeclaredChildrenReadCurrentState(t *testing.T) {
	for _, child := range []struct {
		rootKey, nestedKey, fields, route string
	}{
		{"portal_pages", "pages", `"slug":"guide","title":"Guide","content":"Guide"`, "pages"},
		{"portal_snippets", "snippets", `"name":"snippet","content":"text"`, "snippets"},
		{"portal_ip_allow_lists", "ip_allow_list", `"allowed_ips":["192.0.2.1"]`, "ip-allow-list"},
		{"portal_integrations", "integrations", ``, "integrations"},
		{
			"portal_identity_providers", "identity_providers",
			`"type":"oidc","config":{"issuer_url":"https://idp.test","client_id":"client","scopes":["openid"]}`,
			"identity-providers",
		},
		{"portal_auth_settings", "auth_settings", `"basic_auth_enabled":true`, "authentication-settings"},
		{"portal_email_configs", "email_config", ``, "email-config"},
		{"portal_audit_log_webhooks", "audit_log_webhook", `"enabled":false`, "audit-log-webhook"},
		{"portal_teams", "teams", `"name":"team"`, "teams"},
		{"portal_custom_domains", "custom_domain", `"hostname":"portal.example.com"`, "custom-domain"},
	} {
		for _, nested := range []bool{false, true} {
			for _, external := range []bool{false, true} {
				// Auth settings currently compare through the managed parent list.
				if external && child.nestedKey == "auth_settings" {
					continue
				}
				t.Run(fmt.Sprintf("%s/nested=%v/external=%v", child.rootKey, nested, external), func(t *testing.T) {
					var logs bytes.Buffer
					transport := &portalChildReadsTransport{}
					_, p, desired, _ := portalTraversalFixture(t, true, external, "portal-id",
						portalChildReadsConfig(transport), &logs)
					item := map[string]any{"ref": "child"}
					if child.fields != "" {
						require.NoError(t, json.Unmarshal([]byte("{"+child.fields+"}"), &item))
					}
					parent := map[string]any{"ref": desired.Ref, "name": desired.Name}
					input := map[string]any{"portals": []any{parent}}
					if external {
						parent["_external"] = map[string]any{"id": "portal-id"}
					}
					if nested {
						switch child.nestedKey {
						case "pages", "snippets", "identity_providers", "teams":
							parent[child.nestedKey] = []any{item}
						default:
							parent[child.nestedKey] = item
						}
					} else {
						item["portal"] = desired.Ref
						input[child.rootKey] = []any{item}
					}
					input["_defaults"] = map[string]any{"kongctl": map[string]any{"namespace": "child-space"}}
					data, err := json.Marshal(input)
					require.NoError(t, err)
					path := filepath.Join(t.TempDir(), "portal.yaml")
					require.NoError(t, os.WriteFile(path, data, 0o600))
					rs, err := loader.New().LoadFile(path)
					require.NoError(t, err)
					_, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
					require.Error(t, err, "required child reads must still fail on 403")
					require.ErrorContains(t, err, "403")
					require.Equal(t, []string{"/v3/portals/portal-id/" + child.route}, transport.calls)
				})
			}
		}
	}
}

func TestPortalDeclaredTeamChildrenPreserveDependencyReads(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, kind := range []resources.ResourceType{
			resources.ResourceTypePortalTeamRole, resources.ResourceTypePortalTeamGroupMapping,
		} {
			t.Run(fmt.Sprintf("%s/external=%v", kind, external), func(t *testing.T) {
				var logs bytes.Buffer
				transport := &portalChildReadsTransport{}
				root, p, desired, cfg := portalTraversalFixture(t, true, external, "portal-id",
					portalChildReadsConfig(transport), &logs)
				if kind == resources.ResourceTypePortalTeamRole {
					p.desiredPortalTeamRoles = []resources.PortalTeamRoleResource{{
						Ref: "role", Portal: desired.Ref, Team: "team", RoleName: "Viewer",
					}}
				} else {
					p.desiredPortalTeamGroupMappings = []resources.PortalTeamGroupMappingResource{{
						Ref: "mapping", Portal: desired.Ref, Team: "team", Groups: []string{"developers"},
					}}
				}
				plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
				err := root.PlanChanges(t.Context(), cfg, plan)
				require.ErrorContains(t, err, "403")
				require.Equal(t, []string{"/v3/portals/portal-id/teams"}, transport.calls,
					"a declared team child must resolve teams even when the teams collection is omitted")
			})
		}
	}
}
