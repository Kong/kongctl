package planner

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPortalParentSelectorRefCreatesChildren(t *testing.T) {
	for _, selector := range []string{
		"repro-portal", "!ref repro-portal", "!ref repro-portal#id", "!ref repro-portal#name",
	} {
		t.Run(selector, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "repro.yaml")
			require.NoError(t, os.WriteFile(path, fmt.Appendf(nil, `_defaults:
  kongctl:
    namespace: kongctl-repro
portals:
  - ref: repro-portal
    name: kongctl-repro-portal
portal_pages:
  - ref: repro-page
    portal: %s
    slug: repro
    title: Repro
    visibility: public
    status: published
    content: "# Repro"
portal_ip_allow_lists:
  - ref: repro-allow-list
    portal: %s
    allowed_ips:
      - 10.0.0.0/8
`, selector, selector), 0o600))
			rs, err := loader.New().LoadFromSources([]loader.Source{{Path: path, Type: loader.SourceTypeFile}}, false)
			require.NoError(t, err)
			api := new(MockPortalAPI)
			api.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
				ListPortalsResponse: &kkComps.ListPortalsResponse{}, StatusCode: 200,
			}, nil)
			client := state.NewClient(state.ClientConfig{
				PortalAPI: api, PortalPageAPI: &mockPortalPageAPI{}, PortalIPAllowListAPI: &stubPortalIPAllowListAPI{},
			})
			plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
			require.NoError(t, err)
			requirePlanChange(t, plan, ResourceTypePortal, ActionCreate)
			requirePlanChange(t, plan, ResourceTypePortalPage, ActionCreate)
			requirePlanChange(t, plan, ResourceTypePortalIPAllowList, ActionCreate)
			for _, change := range plan.Changes {
				if change.ResourceType == ResourceTypePortalPage {
					require.NotNil(t, change.Parent)
					require.Equal(t, "repro-portal", change.Parent.Ref)
					require.Equal(t, "repro-portal", change.References[FieldPortalID].Ref)
					require.Contains(t, change.DependsOn, plan.Changes[0].ID)
				}
			}
		})
	}
}

func TestAPIParentSelectorRefCreatesChildren(t *testing.T) {
	for _, selector := range []string{"repro-api", "!ref repro-api", "!ref repro-api#id", "!ref repro-api#name"} {
		t.Run(selector, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "repro.yaml")
			require.NoError(t, os.WriteFile(path, fmt.Appendf(nil, `apis:
  - ref: repro-api
    name: repro-api-name
api_versions:
  - ref: repro-version
    api: %s
    version: v1
api_documents:
  - ref: repro-document
    api: %s
    title: Repro
    slug: repro
    content: "# Repro"
`, selector, selector), 0o600))
			rs, err := loader.New().LoadFromSources([]loader.Source{{Path: path, Type: loader.SourceTypeFile}}, false)
			require.NoError(t, err)
			api := new(MockAPIAPI)
			mockEmptyAPIsList(t.Context(), api)
			plan, err := NewPlanner(state.NewClient(state.ClientConfig{APIAPI: api}), slog.Default()).GeneratePlan(
				t.Context(), rs, Options{Mode: PlanModeApply},
			)
			require.NoError(t, err)
			requirePlanChange(t, plan, ResourceTypeAPI, ActionCreate)
			requirePlanChange(t, plan, ResourceTypeAPIVersion, ActionCreate)
			requirePlanChange(t, plan, ResourceTypeAPIDocument, ActionCreate)
		})
	}
}

func TestPortalParentSelectorRefExistingPage(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, mode := range []PlanMode{PlanModeApply, PlanModeSync, PlanModeDelete} {
			if !external && mode == PlanModeDelete {
				continue
			}
			for _, selector := range []string{"repro-portal", "!ref repro-portal#id", "!ref repro-portal#name"} {
				t.Run(fmt.Sprintf("external=%v/%s/%s", external, mode, selector), func(t *testing.T) {
					parent := ""
					if external {
						parent = "    _external:\n      selector:\n        matchFields:\n          name: kongctl-repro-portal\n"
					}
					path := filepath.Join(t.TempDir(), "repro.yaml")
					require.NoError(t, os.WriteFile(path, fmt.Appendf(nil, `_defaults:
  kongctl:
    namespace: kongctl-repro
portals:
  - ref: repro-portal
    name: kongctl-repro-portal
%sportal_pages:
  - ref: repro-page
    portal: %s
    slug: repro
    content: "# Repro"
`, parent, selector), 0o600))
					rs, err := loader.New().LoadFromSources([]loader.Source{{Path: path, Type: loader.SourceTypeFile}}, false)
					require.NoError(t, err)
					api := new(MockPortalAPI)
					api.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
						StatusCode: 200,
						ListPortalsResponse: &kkComps.ListPortalsResponse{Data: []kkComps.ListPortalsResponsePortal{
							newListPortal("portal-id", "kongctl-repro-portal", map[string]string{labels.NamespaceKey: "kongctl-repro"}),
						}},
					}, nil)
					pages := &stubPortalPageAPI{
						listData: []kkComps.PortalPageInfo{{ID: "page-id", Slug: "repro"}},
						getData:  map[string]kkComps.PortalPageResponse{"page-id": {ID: "page-id", Slug: "repro", Content: "# Existing"}},
					}
					plan, err := NewPlanner(state.NewClient(state.ClientConfig{
						PortalAPI: api, PortalPageAPI: pages,
					}), slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: mode})
					require.NoError(t, err)
					if mode == PlanModeDelete {
						requirePlanChange(t, plan, ResourceTypePortalPage, ActionDelete)
						return
					}
					requirePlanChange(t, plan, ResourceTypePortalPage, ActionUpdate)
					for _, change := range plan.Changes {
						if change.ResourceType == ResourceTypePortalPage {
							require.Equal(t, ActionUpdate, change.Action, "declared live page must be updated, never deleted")
							require.Equal(t, "page-id", change.ResourceID)
						}
					}
				})
			}
		}
	}
}

func TestPortalParentSelectorRefRejectsMissingTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repro.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`portals:
  - ref: repro-portal
    name: Repro Portal
portal_pages:
  - ref: home
    portal: !ref repro-portl#id
    slug: /
    content: Home
`), 0o600))
	_, err := loader.New().LoadFromSources([]loader.Source{{Path: path, Type: loader.SourceTypeFile}}, false)
	require.ErrorContains(t, err, "resource not found: repro-portl")
	require.ErrorContains(t, err, "/portal")
}

func TestPortalTeamParentSelectorRefDependencies(t *testing.T) {
	for _, selector := range []string{"portal-team", "!ref portal-team", "!ref portal-team#id", "!ref portal-team#name"} {
		t.Run(selector, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "repro.yaml")
			require.NoError(t, os.WriteFile(path, fmt.Appendf(nil, `portals:
  - ref: repro-portal
    name: Repro Portal
portal_teams:
  - ref: portal-team
    portal: !ref repro-portal
    name: Readers
portal_team_group_mappings:
  - ref: reader-groups
    portal: !ref repro-portal
    team: %s
    groups: [readers]
portal_team_roles:
  - ref: reader-role
    portal: !ref repro-portal
    team: %s
    role_name: API Viewer
    entity_id: 11111111-1111-1111-1111-111111111111
    entity_type_name: APIs
    entity_region: us
`, selector, selector), 0o600))
			rs, err := loader.New().LoadFromSources([]loader.Source{{Path: path, Type: loader.SourceTypeFile}}, false)
			require.NoError(t, err)
			api := new(MockPortalAPI)
			api.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
				ListPortalsResponse: &kkComps.ListPortalsResponse{}, StatusCode: 200,
			}, nil)
			plan, err := NewPlanner(state.NewClient(state.ClientConfig{PortalAPI: api}), slog.Default()).GeneratePlan(
				t.Context(), rs, Options{Mode: PlanModeApply},
			)
			require.NoError(t, err)
			teamChangeID := ""
			for _, change := range plan.Changes {
				if change.ResourceType == ResourceTypePortalTeam {
					teamChangeID = change.ID
				}
			}
			require.NotEmpty(t, teamChangeID)
			requirePlanChange(t, plan, ResourceTypePortalTeamGroupMapping, ActionUpdate)
			requirePlanChange(t, plan, ResourceTypePortalTeamRole, ActionCreate)
			for _, change := range plan.Changes {
				if change.ResourceType == ResourceTypePortalTeamGroupMapping || change.ResourceType == ResourceTypePortalTeamRole {
					require.Equal(t, "portal-team", change.References[FieldTeamID].Ref)
					require.Equal(t, "Readers", change.References[FieldTeamID].LookupFields[FieldName])
					require.Contains(t, change.DependsOn, teamChangeID)
				}
			}
		})
	}
}
