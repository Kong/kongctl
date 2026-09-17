package dump

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	declresources "github.com/kong/kongctl/internal/declarative/resources"
	declstate "github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type declarativeDumpContext struct {
	sdk             helpers.SDKAPI
	stateClient     *declstate.Client
	logger          *slog.Logger
	pageSize        int64
	filter          filterOptions
	includeChildren bool
}

type declarativeCollector struct {
	kind          declresources.ResourceType
	selector      string
	collect       func(context.Context, *declarativeDumpContext, *declresources.ResourceSet) error
	omittedReason string
}

// Keep supported selectors, collection dispatch, and deliberate omissions in
// one inventory. Child traversal and API-specific mapping remain with collectors.
var declarativeCollectors = buildDeclarativeCollectors()

func buildDeclarativeCollectors() map[string]declarativeCollector {
	registrations := []declarativeCollector{
		rootCollector(
			"portals",
			func(ctx context.Context, d *declarativeDumpContext) ([]declresources.PortalResource, error) {
				return collectDeclarativePortals(ctx, d.sdk.GetPortalAPI(), d.pageSize, d.filter)
			},
			func(rs *declresources.ResourceSet) *[]declresources.PortalResource { return &rs.Portals },
			func(ctx context.Context, d *declarativeDumpContext, values []declresources.PortalResource) error {
				return populatePortalChildren(ctx, d.logger, d.stateClient, values)
			},
		),
		rootCollector(
			resourceAPIs,
			func(ctx context.Context, d *declarativeDumpContext) ([]declresources.APIResource, error) {
				return collectDeclarativeAPIs(ctx, d.sdk.GetAPIAPI(), d.pageSize, d.filter)
			},
			func(rs *declresources.ResourceSet) *[]declresources.APIResource { return &rs.APIs },
			func(ctx context.Context, d *declarativeDumpContext, values []declresources.APIResource) error {
				populateAPIChildren(ctx, d.logger, d.stateClient, values)
				return nil
			},
		),
		rootCollector(
			"application_auth_strategies",
			func(ctx context.Context, d *declarativeDumpContext) ([]declresources.ApplicationAuthStrategyResource, error) {
				return collectDeclarativeAuthStrategies(ctx, d.sdk.GetAppAuthStrategiesAPI(), d.pageSize, d.filter)
			},
			func(rs *declresources.ResourceSet) *[]declresources.ApplicationAuthStrategyResource {
				return &rs.ApplicationAuthStrategies
			},
			nil,
		),
		rootCollector(
			"dcr_providers",
			func(ctx context.Context, d *declarativeDumpContext) ([]declresources.DCRProviderResource, error) {
				return collectDeclarativeDCRProviders(ctx, d.sdk.GetDCRProvidersAPI(), d.pageSize, d.filter)
			},
			func(rs *declresources.ResourceSet) *[]declresources.DCRProviderResource { return &rs.DCRProviders }, nil,
		),
		rootCollector(
			"control_planes",
			func(ctx context.Context, d *declarativeDumpContext) ([]declresources.ControlPlaneResource, error) {
				return collectDeclarativeControlPlanes(
					ctx, d.sdk.GetControlPlaneAPI(), d.sdk.GetControlPlaneGroupsAPI(), d.pageSize, d.filter,
				)
			},
			func(rs *declresources.ResourceSet) *[]declresources.ControlPlaneResource { return &rs.ControlPlanes },
			func(ctx context.Context, d *declarativeDumpContext, values []declresources.ControlPlaneResource) error {
				populateControlPlaneChildren(ctx, d.logger, d.stateClient, values)
				return nil
			},
		),
		rootCollector(
			resourceAnalyticsDashboards,
			func(ctx context.Context, d *declarativeDumpContext) ([]declresources.DashboardResource, error) {
				return collectDeclarativeDashboards(ctx, d.sdk.GetDashboardsAPI(), d.pageSize, d.filter)
			},
			func(rs *declresources.ResourceSet) *[]declresources.DashboardResource {
				if rs.Analytics == nil {
					rs.Analytics = &declresources.AnalyticsResource{}
				}
				return &rs.Analytics.Dashboards
			}, nil,
		),
		rootCollector(
			"event_gateways",
			func(ctx context.Context, d *declarativeDumpContext) ([]declresources.EventGatewayControlPlaneResource, error) {
				return collectDeclarativeEventGateways(
					ctx,
					d.sdk.GetEventGatewayControlPlaneAPI(),
					d.pageSize,
					d.filter,
				)
			},
			func(rs *declresources.ResourceSet) *[]declresources.EventGatewayControlPlaneResource {
				return &rs.EventGatewayControlPlanes
			},
			func(ctx context.Context, d *declarativeDumpContext, values []declresources.EventGatewayControlPlaneResource) error {
				populateEventGatewayChildren(ctx, d.logger, d.stateClient, values)
				return nil
			},
		),
		rootCollector(
			"ai_gateways",
			func(ctx context.Context, d *declarativeDumpContext) ([]declresources.AIGatewayResource, error) {
				return collectDeclarativeAIGateways(ctx, d.sdk.GetAIGatewayAPI(), d.pageSize, d.filter)
			},
			func(rs *declresources.ResourceSet) *[]declresources.AIGatewayResource { return &rs.AIGateways },
			func(ctx context.Context, d *declarativeDumpContext, values []declresources.AIGatewayResource) error {
				populateAIGatewayChildren(ctx, d.logger, d.stateClient, values)
				return nil
			},
		),
		{
			kind:     declresources.ResourceTypeOrganizationTeam,
			selector: "organization.teams",
			collect:  collectOrganizationForDump,
		},
		{
			kind:          declresources.ResourceTypeCatalogService,
			omittedReason: "Catalog services do not yet support declarative dump",
		},
	}

	collectors := make(map[string]declarativeCollector)
	kinds := make([]declresources.ResourceType, 0, len(registrations))
	for _, registration := range registrations {
		kinds = append(kinds, registration.kind)
		if strings.TrimSpace(registration.omittedReason) != "" {
			if registration.selector != "" || registration.collect != nil {
				panic("declarative dump omission conflicts with collector: " + string(registration.kind))
			}
			continue
		}
		if registration.selector == "" || registration.collect == nil {
			panic("declarative dump requires a selector and collector: " + string(registration.kind))
		}
		if _, exists := collectors[registration.selector]; exists {
			panic("duplicate declarative dump selector: " + registration.selector)
		}
		collectors[registration.selector] = registration
	}
	if err := declresources.ValidateManagedRootCoverage("declarative dump", kinds); err != nil {
		panic(err)
	}
	return collectors
}

func rootCollector[R any, RPtr interface {
	*R
	declresources.Resource
}](
	selector string,
	collect func(context.Context, *declarativeDumpContext) ([]R, error),
	destination func(*declresources.ResourceSet) *[]R,
	populate func(context.Context, *declarativeDumpContext, []R) error,
) declarativeCollector {
	if collect == nil || destination == nil {
		panic(fmt.Sprintf("declarative dump %s requires collection and storage", selector))
	}
	return declarativeCollector{
		kind:     RPtr(new(R)).GetType(),
		selector: selector,
		collect: func(ctx context.Context, d *declarativeDumpContext, rs *declresources.ResourceSet) error {
			values, err := collect(ctx, d)
			if err != nil {
				return err
			}
			if d.includeChildren && populate != nil {
				if err := populate(ctx, d, values); err != nil {
					return err
				}
			}
			target := destination(rs)
			*target = append(*target, values...)
			return nil
		},
	}
}

// Organization dump also derives user/system-account selectors from the
// exported teams. Preserve that order and the grouped output shape.
func collectOrganizationForDump(
	ctx context.Context,
	d *declarativeDumpContext,
	rs *declresources.ResourceSet,
) error {
	teams, err := collectDeclarativeOrganizationTeams(ctx, d.sdk.GetOrganizationTeamAPI(), d.pageSize, d.filter)
	if err != nil {
		return err
	}
	if d.includeChildren {
		populateOrganizationTeamChildren(ctx, d.logger, d.stateClient, teams)
	}
	if rs.Organization == nil {
		rs.Organization = &declresources.OrganizationResource{}
	}
	rs.Organization.Teams = append(rs.Organization.Teams, teams...)
	if d.includeChildren {
		rs.Organization.Users = append(
			rs.Organization.Users,
			collectOrganizationUsersFromTeamMemberships(ctx, d.logger, d.stateClient, teams)...,
		)
		rs.Organization.SystemAccounts = append(
			rs.Organization.SystemAccounts,
			collectOrganizationSystemAccountsFromTeamMemberships(ctx, d.logger, d.stateClient, teams)...,
		)
	}
	return nil
}
