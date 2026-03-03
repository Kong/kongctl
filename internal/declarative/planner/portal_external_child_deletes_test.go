package planner

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
)

type stubExternalPortalEmailsAPI struct {
	getEmailConfigFn                 func() *kkComps.PortalEmailConfig
	listPortalCustomEmailTemplatesFn func() []kkComps.EmailTemplate
}

func (s *stubExternalPortalEmailsAPI) ListEmailDomains(
	_ context.Context, _ kkOps.ListEmailDomainsRequest, _ ...kkOps.Option,
) (*kkOps.ListEmailDomainsResponse, error) {
	return &kkOps.ListEmailDomainsResponse{}, nil
}

func (s *stubExternalPortalEmailsAPI) GetEmailDomain(
	_ context.Context, _ string, _ ...kkOps.Option,
) (*kkOps.GetEmailDomainResponse, error) {
	return &kkOps.GetEmailDomainResponse{}, nil
}

func (s *stubExternalPortalEmailsAPI) GetEmailConfig(
	_ context.Context, _ string, _ ...kkOps.Option,
) (*kkOps.GetEmailConfigResponse, error) {
	config := &kkComps.PortalEmailConfig{}
	if s.getEmailConfigFn != nil {
		config = s.getEmailConfigFn()
	}

	return &kkOps.GetEmailConfigResponse{
		StatusCode:        200,
		PortalEmailConfig: config,
	}, nil
}

func (s *stubExternalPortalEmailsAPI) CreatePortalEmailConfig(
	_ context.Context, _ string, _ kkComps.PostPortalEmailConfig, _ ...kkOps.Option,
) (*kkOps.CreatePortalEmailConfigResponse, error) {
	return &kkOps.CreatePortalEmailConfigResponse{}, nil
}

func (s *stubExternalPortalEmailsAPI) UpdatePortalEmailConfig(
	_ context.Context, _ string, _ *kkComps.PatchPortalEmailConfig, _ ...kkOps.Option,
) (*kkOps.UpdatePortalEmailConfigResponse, error) {
	return &kkOps.UpdatePortalEmailConfigResponse{}, nil
}

func (s *stubExternalPortalEmailsAPI) DeletePortalEmailConfig(
	_ context.Context, _ string, _ ...kkOps.Option,
) (*kkOps.DeletePortalEmailConfigResponse, error) {
	return &kkOps.DeletePortalEmailConfigResponse{}, nil
}

func (s *stubExternalPortalEmailsAPI) ListPortalCustomEmailTemplates(
	_ context.Context, _ string, _ ...kkOps.Option,
) (*kkOps.ListPortalCustomEmailTemplatesResponse, error) {
	templates := []kkComps.EmailTemplate{}
	if s.listPortalCustomEmailTemplatesFn != nil {
		templates = s.listPortalCustomEmailTemplatesFn()
	}

	return &kkOps.ListPortalCustomEmailTemplatesResponse{
		StatusCode: 200,
		ListEmailTemplates: &kkComps.ListEmailTemplates{
			Data: templates,
		},
	}, nil
}

func (s *stubExternalPortalEmailsAPI) GetPortalCustomEmailTemplate(
	_ context.Context, _ string, _ kkComps.EmailTemplateName, _ ...kkOps.Option,
) (*kkOps.GetPortalCustomEmailTemplateResponse, error) {
	return &kkOps.GetPortalCustomEmailTemplateResponse{}, nil
}

func (s *stubExternalPortalEmailsAPI) UpdatePortalCustomEmailTemplate(
	_ context.Context, _ kkOps.UpdatePortalCustomEmailTemplateRequest, _ ...kkOps.Option,
) (*kkOps.UpdatePortalCustomEmailTemplateResponse, error) {
	return &kkOps.UpdatePortalCustomEmailTemplateResponse{}, nil
}

func (s *stubExternalPortalEmailsAPI) DeletePortalCustomEmailTemplate(
	_ context.Context, _ string, _ kkComps.EmailTemplateName, _ ...kkOps.Option,
) (*kkOps.DeletePortalCustomEmailTemplateResponse, error) {
	return &kkOps.DeletePortalCustomEmailTemplateResponse{}, nil
}

type stubExternalPortalTeamAPI struct {
	listPortalTeamsFn func() []kkComps.PortalTeamResponse
}

func (s *stubExternalPortalTeamAPI) ListPortalTeams(
	_ context.Context, _ kkOps.ListPortalTeamsRequest, _ ...kkOps.Option,
) (*kkOps.ListPortalTeamsResponse, error) {
	teams := []kkComps.PortalTeamResponse{}
	if s.listPortalTeamsFn != nil {
		teams = s.listPortalTeamsFn()
	}

	return &kkOps.ListPortalTeamsResponse{
		StatusCode: 200,
		ListPortalTeamsResponse: &kkComps.ListPortalTeamsResponse{
			Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: float64(len(teams))}},
			Data: teams,
		},
	}, nil
}

func (s *stubExternalPortalTeamAPI) GetPortalTeam(
	_ context.Context, _, _ string, _ ...kkOps.Option,
) (*kkOps.GetPortalTeamResponse, error) {
	return &kkOps.GetPortalTeamResponse{}, nil
}

func (s *stubExternalPortalTeamAPI) CreatePortalTeam(
	_ context.Context, _ string, _ *kkComps.PortalCreateTeamRequest, _ ...kkOps.Option,
) (*kkOps.CreatePortalTeamResponse, error) {
	return &kkOps.CreatePortalTeamResponse{}, nil
}

func (s *stubExternalPortalTeamAPI) UpdatePortalTeam(
	_ context.Context, _ kkOps.UpdatePortalTeamRequest, _ ...kkOps.Option,
) (*kkOps.UpdatePortalTeamResponse, error) {
	return &kkOps.UpdatePortalTeamResponse{}, nil
}

func (s *stubExternalPortalTeamAPI) DeletePortalTeam(
	_ context.Context, _, _ string, _ ...kkOps.Option,
) (*kkOps.DeletePortalTeamResponse, error) {
	return &kkOps.DeletePortalTeamResponse{}, nil
}

type stubExternalPortalTeamRolesAPI struct {
	listPortalTeamRolesFn func(teamID string) []kkComps.PortalAssignedRoleResponse
}

func (s *stubExternalPortalTeamRolesAPI) ListPortalTeamRoles(
	_ context.Context, request kkOps.ListPortalTeamRolesRequest, _ ...kkOps.Option,
) (*kkOps.ListPortalTeamRolesResponse, error) {
	roles := []kkComps.PortalAssignedRoleResponse{}
	if s.listPortalTeamRolesFn != nil {
		roles = s.listPortalTeamRolesFn(request.TeamID)
	}

	return &kkOps.ListPortalTeamRolesResponse{
		StatusCode: 200,
		AssignedPortalRoleCollectionResponse: &kkComps.AssignedPortalRoleCollectionResponse{
			Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: float64(len(roles))}},
			Data: roles,
		},
	}, nil
}

func (s *stubExternalPortalTeamRolesAPI) AssignRoleToPortalTeams(
	_ context.Context, _ kkOps.AssignRoleToPortalTeamsRequest, _ ...kkOps.Option,
) (*kkOps.AssignRoleToPortalTeamsResponse, error) {
	return &kkOps.AssignRoleToPortalTeamsResponse{}, nil
}

func (s *stubExternalPortalTeamRolesAPI) RemoveRoleFromPortalTeam(
	_ context.Context, _ kkOps.RemoveRoleFromPortalTeamRequest, _ ...kkOps.Option,
) (*kkOps.RemoveRoleFromPortalTeamResponse, error) {
	return &kkOps.RemoveRoleFromPortalTeamResponse{}, nil
}

func TestPlanPortalCustomDomain_ExternalPortalSyncSkipsDeleteWhenOmitted(t *testing.T) {
	t.Parallel()

	stub := &stubPortalCustomDomainAPI{
		getFn: func(
			_ context.Context,
			_ string,
			_ ...kkOps.Option,
		) (*kkOps.GetPortalCustomDomainResponse, error) {
			return &kkOps.GetPortalCustomDomainResponse{
				StatusCode: 200,
				PortalCustomDomain: buildPortalCustomDomain(
					"developer.example.com",
					true,
				),
			}, nil
		},
	}

	planner := &Planner{
		client: state.NewClient(state.ClientConfig{
			PortalCustomDomainAPI: stub,
		}),
		logger:         slog.Default(),
		desiredPortals: []resources.PortalResource{externalPortalResource()},
	}

	plan := NewPlan("1.0", "test", PlanModeSync)

	err := planner.planPortalCustomDomainsChanges(
		context.Background(),
		DefaultNamespace,
		"portal-123",
		"ext-portal-ref",
		nil,
		plan,
	)
	assert.NoError(t, err)
	assert.Empty(t, plan.Changes)
}

func TestPlanPortalEmailConfig_ExternalPortalSyncSkipsDeleteWhenOmitted(t *testing.T) {
	t.Parallel()

	planner := &Planner{
		client: state.NewClient(state.ClientConfig{
			PortalEmailsAPI: &stubExternalPortalEmailsAPI{},
		}),
		logger:         slog.Default(),
		desiredPortals: []resources.PortalResource{externalPortalResource()},
	}

	plan := NewPlan("1.0", "test", PlanModeSync)

	err := planner.planPortalEmailConfigsChanges(
		context.Background(),
		DefaultNamespace,
		"portal-123",
		"ext-portal-ref",
		nil,
		plan,
	)
	assert.NoError(t, err)
	assert.Empty(t, plan.Changes)
}

func TestPlanPortalEmailTemplates_ExternalPortalSyncSkipsDeleteWhenOmitted(t *testing.T) {
	t.Parallel()

	planner := &Planner{
		client: state.NewClient(state.ClientConfig{
			PortalEmailsAPI: &stubExternalPortalEmailsAPI{
				listPortalCustomEmailTemplatesFn: func() []kkComps.EmailTemplate {
					return []kkComps.EmailTemplate{
						{Name: kkComps.EmailTemplateName("account_invitation"), Enabled: true},
					}
				},
			},
		}),
		logger:         slog.Default(),
		desiredPortals: []resources.PortalResource{externalPortalResource()},
	}

	plan := NewPlan("1.0", "test", PlanModeSync)

	err := planner.planPortalEmailTemplatesChanges(
		context.Background(),
		DefaultNamespace,
		"portal-123",
		"ext-portal-ref",
		"ext-portal",
		nil,
		plan,
	)
	assert.NoError(t, err)
	assert.Empty(t, plan.Changes)
}

func TestPlanPortalTeams_ExternalPortalSyncSkipsDeleteWhenOmitted(t *testing.T) {
	t.Parallel()

	planner := &Planner{
		client: state.NewClient(state.ClientConfig{
			PortalTeamAPI: &stubExternalPortalTeamAPI{
				listPortalTeamsFn: func() []kkComps.PortalTeamResponse {
					return []kkComps.PortalTeamResponse{
						{ID: new("team-a-id"), Name: new("team-a")},
						{ID: new("team-b-id"), Name: new("team-b")},
					}
				},
			},
		}),
		logger:         slog.Default(),
		desiredPortals: []resources.PortalResource{externalPortalResource()},
	}

	plan := NewPlan("1.0", "test", PlanModeSync)

	err := planner.planPortalTeamsChanges(
		context.Background(),
		DefaultNamespace,
		"portal-123",
		"ext-portal-ref",
		[]resources.PortalTeamResource{
			{
				PortalCreateTeamRequest: kkComps.PortalCreateTeamRequest{Name: "team-a"},
				Ref:                     "team-a-ref",
				Portal:                  "ext-portal-ref",
			},
		},
		plan,
	)
	assert.NoError(t, err)
	assertNoDeleteChangeForResourceType(t, plan, ResourceTypePortalTeam)
}

func TestPlanPortalTeamRoles_ExternalPortalSyncSkipsDeleteWhenOmitted(t *testing.T) {
	t.Parallel()

	region := kkComps.EntityRegion("us")
	planner := &Planner{
		client: state.NewClient(state.ClientConfig{
			PortalTeamAPI: &stubExternalPortalTeamAPI{
				listPortalTeamsFn: func() []kkComps.PortalTeamResponse {
					return []kkComps.PortalTeamResponse{
						{ID: new("team-a-id"), Name: new("team-a")},
						{ID: new("team-b-id"), Name: new("team-b")},
					}
				},
			},
			PortalTeamRolesAPI: &stubExternalPortalTeamRolesAPI{
				listPortalTeamRolesFn: func(teamID string) []kkComps.PortalAssignedRoleResponse {
					switch teamID {
					case "team-a-id":
						return []kkComps.PortalAssignedRoleResponse{
							{
								ID:             new("role-1"),
								RoleName:       new("viewer"),
								EntityID:       new("api-1"),
								EntityTypeName: new("api"),
								EntityRegion:   &region,
							},
						}
					case "team-b-id":
						return []kkComps.PortalAssignedRoleResponse{
							{
								ID:             new("role-2"),
								RoleName:       new("editor"),
								EntityID:       new("api-2"),
								EntityTypeName: new("api"),
								EntityRegion:   &region,
							},
						}
					default:
						return nil
					}
				},
			},
		}),
		logger:         slog.Default(),
		desiredPortals: []resources.PortalResource{externalPortalResource()},
		desiredPortalTeams: []resources.PortalTeamResource{
			{
				PortalCreateTeamRequest: kkComps.PortalCreateTeamRequest{Name: "team-a"},
				Ref:                     "team-a-ref",
				Portal:                  "ext-portal-ref",
			},
			{
				PortalCreateTeamRequest: kkComps.PortalCreateTeamRequest{Name: "team-b"},
				Ref:                     "team-b-ref",
				Portal:                  "ext-portal-ref",
			},
		},
		desiredPortalTeamRoles: []resources.PortalTeamRoleResource{
			{
				Ref:            "team-a-viewer-role",
				Portal:         "ext-portal-ref",
				Team:           "team-a-ref",
				RoleName:       "viewer",
				EntityID:       "api-1",
				EntityTypeName: "api",
				EntityRegion:   "us",
			},
		},
	}

	plan := NewPlan("1.0", "test", PlanModeSync)

	err := planner.planPortalTeamRolesChanges(
		context.Background(),
		DefaultNamespace,
		"portal-123",
		"ext-portal-ref",
		"ext-portal",
		plan,
	)
	assert.NoError(t, err)
	assertNoDeleteChangeForResourceType(t, plan, ResourceTypePortalTeamRole)
}

func externalPortalResource() resources.PortalResource {
	return resources.PortalResource{
		CreatePortal: kkComps.CreatePortal{Name: "ext-portal"},
		BaseResource: resources.BaseResource{Ref: "ext-portal-ref"},
		External:     &resources.ExternalBlock{ID: "portal-123"},
	}
}

func assertNoDeleteChangeForResourceType(t *testing.T, plan *Plan, resourceType string) {
	t.Helper()

	for _, change := range plan.Changes {
		if change.ResourceType == resourceType && change.Action == ActionDelete {
			t.Fatalf("unexpected %s delete planned for external portal: %+v", resourceType, change)
		}
	}
}

//go:fix inline
func stringPtr(value string) *string {
	return new(value)
}
