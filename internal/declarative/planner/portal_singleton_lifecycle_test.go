package planner

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestPortalSingletonLifecycleSelectionAndObservation(t *testing.T) {
	for _, family := range []struct {
		kind    string
		run     func(context.Context, *Planner, string, []string, *Plan) error
		failing func(error) state.ClientConfig
	}{
		{
			kind: ResourceTypePortalCustomDomain,
			run: func(ctx context.Context, p *Planner, parentID string, refs []string, plan *Plan) error {
				var desired []resources.PortalCustomDomainResource
				for _, ref := range refs {
					desired = append(desired, resources.PortalCustomDomainResource{
						Ref: ref, Portal: "portal",
						CreatePortalCustomDomainRequest: kkComps.CreatePortalCustomDomainRequest{
							Hostname: "developer.example.com", Enabled: true,
							Ssl: kkComps.CreateCreatePortalCustomDomainSSLHTTP(kkComps.HTTP{}),
						},
					})
				}
				return p.planPortalCustomDomainsChanges(ctx, "default", parentID, "", desired, plan)
			},
			failing: func(err error) state.ClientConfig {
				return state.ClientConfig{PortalCustomDomainAPI: &stubPortalCustomDomainAPI{
					getFn: func(context.Context, string, ...kkOps.Option) (*kkOps.GetPortalCustomDomainResponse, error) {
						return nil, err
					},
				}}
			},
		},
		{
			kind: ResourceTypePortalEmailConfig,
			run: func(ctx context.Context, p *Planner, parentID string, refs []string, plan *Plan) error {
				var desired []resources.PortalEmailConfigResource
				for _, ref := range refs {
					desired = append(desired, resources.PortalEmailConfigResource{Ref: ref, Portal: "portal"})
				}
				return p.planPortalEmailConfigsChanges(ctx, "default", parentID, "", desired, plan)
			},
			failing: func(err error) state.ClientConfig {
				return state.ClientConfig{PortalEmailsAPI: &singletonFailingEmailsAPI{err: err}}
			},
		},
		{
			kind: ResourceTypePortalAuditLogWebhook,
			run: func(ctx context.Context, p *Planner, parentID string, refs []string, plan *Plan) error {
				var desired []resources.PortalAuditLogWebhookResource
				for _, ref := range refs {
					desired = append(desired, resources.PortalAuditLogWebhookResource{
						Ref: ref, Portal: "portal", Enabled: new(true), AuditLogDestinationID: "destination-id",
					})
				}
				return p.planPortalAuditLogWebhooksChanges(ctx, "default", parentID, "", desired, plan)
			},
			failing: func(err error) state.ClientConfig {
				return state.ClientConfig{PortalAuditLogsAPI: &singletonFailingAuditLogsAPI{err: err}}
			},
		},
	} {
		t.Run(family.kind, func(t *testing.T) {
			for _, tc := range []struct {
				name        string
				parentID    string
				refs        []string
				readErr     error
				wantCreated bool
				wantWarning bool
			}{
				{
					name: "new parent skips reads and selects first unplanned ref",
					refs: []string{"already", "selected", "ignored"}, readErr: errors.New("must not read"),
					wantCreated: true,
				},
				{
					name: "missing client creates selected ref with warning", parentID: "portal-id",
					refs: []string{"already", "selected", "ignored"}, wantCreated: true, wantWarning: true,
				},
				{name: "missing client with omitted singleton", parentID: "portal-id"},
				{name: "all declarations already planned", parentID: "portal-id", refs: []string{"already"}},
				{
					name: "read failure must not create", parentID: "portal-id", refs: []string{"selected"},
					readErr: errors.New("read failed"),
				},
				{
					name: "unrelated missing client is fatal", parentID: "portal-id", refs: []string{"selected"},
					readErr: &state.APIClientError{ClientType: "unrelated API"},
				},
			} {
				t.Run(tc.name, func(t *testing.T) {
					cfg := state.ClientConfig{}
					if tc.readErr != nil {
						cfg = family.failing(tc.readErr)
					}
					p := NewPlanner(state.NewClient(cfg), slog.Default())
					p.resources = &resources.ResourceSet{}
					plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
					plan.AddChange(PlannedChange{
						ID: "already-planned", ResourceType: family.kind, ResourceRef: "already", Action: ActionCreate,
					})
					err := family.run(t.Context(), p, tc.parentID, tc.refs, plan)
					if tc.readErr != nil && tc.parentID != "" {
						require.ErrorIs(t, err, tc.readErr)
						require.ErrorContains(t, err, `for portal "portal-id"`)
					} else {
						require.NoError(t, err)
					}
					if tc.wantCreated {
						require.Len(t, plan.Changes, 2)
						require.Equal(t, "selected", plan.Changes[1].ResourceRef)
						require.Equal(t, ActionCreate, plan.Changes[1].Action)
					} else {
						require.Len(t, plan.Changes, 1)
					}
					if tc.wantWarning {
						require.Len(t, plan.Warnings, 1)
						require.Contains(t, plan.Warnings[0].Message, "assuming create is required")
					} else {
						require.Empty(t, plan.Warnings)
					}
				})
			}
		})
	}
}

func TestPortalSingletonLifecyclePrunesOnlyInSync(t *testing.T) {
	for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
		t.Run(string(mode), func(t *testing.T) {
			p := NewPlanner(state.NewClient(state.ClientConfig{
				PortalEmailsAPI: &stubExternalPortalEmailsAPI{},
			}), slog.Default())
			plan := NewPlan(CurrentPlanVersion, "test", mode)
			err := p.planPortalEmailConfigsChanges(t.Context(), "default", "portal-id", "portal", nil, plan)
			require.NoError(t, err)
			if mode == PlanModeSync {
				require.Len(t, plan.Changes, 1)
				require.Equal(t, ActionDelete, plan.Changes[0].Action)
				require.Equal(t, ResourceTypePortalEmailConfig, plan.Changes[0].ResourceType)
			} else {
				require.Empty(t, plan.Changes)
			}
		})
	}
}

func TestPortalSingletonLifecycleEmptyWebhookIsAbsent(t *testing.T) {
	for _, declared := range []bool{false, true} {
		name := "omitted does not delete an empty response"
		if declared {
			name = "declared creates instead of updating an empty response"
		}
		t.Run(name, func(t *testing.T) {
			p := NewPlanner(state.NewClient(state.ClientConfig{
				PortalAuditLogsAPI: &stubPortalAuditLogsAPI{current: &kkComps.PortalAuditLogWebhook{}},
			}), slog.Default())
			p.resources = &resources.ResourceSet{}
			var desired []resources.PortalAuditLogWebhookResource
			if declared {
				desired = []resources.PortalAuditLogWebhookResource{
					{Ref: "webhook", Portal: "portal", Enabled: new(true)},
				}
			}
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
			err := p.planPortalAuditLogWebhooksChanges(t.Context(), "default", "portal-id", "portal", desired, plan)
			require.NoError(t, err)
			if declared {
				require.Len(t, plan.Changes, 1)
				require.Equal(t, ActionCreate, plan.Changes[0].Action)
			} else {
				require.Empty(t, plan.Changes)
			}
		})
	}
}

type singletonFailingEmailsAPI struct {
	stubExternalPortalEmailsAPI
	err error
}

func (s *singletonFailingEmailsAPI) GetEmailConfig(
	context.Context, string, ...kkOps.Option,
) (*kkOps.GetEmailConfigResponse, error) {
	return nil, s.err
}

type singletonFailingAuditLogsAPI struct {
	stubPortalAuditLogsAPI
	err error
}

func (s *singletonFailingAuditLogsAPI) GetPortalAuditLogWebhook(
	context.Context, string, ...kkOps.Option,
) (*kkOps.GetPortalAuditLogWebhookResponse, error) {
	return nil, s.err
}
