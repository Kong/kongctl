package dump

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	konnectCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	decllabels "github.com/kong/kongctl/internal/declarative/labels"
	declresources "github.com/kong/kongctl/internal/declarative/resources"
	declstate "github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/kong/kongctl/internal/util/pagination"
	"sigs.k8s.io/yaml"
)

type declarativeOptions struct {
	resources             []string
	outputFile            string
	defaultNamespace      string
	includeChildResources bool
	filter                filterOptions
}

// DeclarativeDumpOptions configures a declarative resource dump outside Cobra flag parsing.
type DeclarativeDumpOptions struct {
	Resources             []string
	OutputFile            string
	DefaultNamespace      string
	IncludeChildResources bool
	FilterName            string
	FilterID              string
}

var declarativeAllowedResources = map[string]struct{}{
	"portals":                            {},
	resourceAPIs:                         {},
	"application_auth_strategies":        {},
	"dcr_providers":                      {},
	"control_planes":                     {},
	resourceAnalyticsDashboards:          {},
	"event_gateways":                     {},
	"ai_gateways":                        {},
	"ai_gateway_identity_providers":      {},
	"ai_gateway_policies":                {},
	"ai_gateway_agents":                  {},
	"ai_gateway_consumers":               {},
	"ai_gateway_consumer_credentials":    {},
	"ai_gateway_consumer_groups":         {},
	"ai_gateway_models":                  {},
	"ai_gateway_mcp_servers":             {},
	"ai_gateway_vaults":                  {},
	"ai_gateway_data_plane_certificates": {},
	"organization.teams":                 {},
}

func newDeclarativeCmd() *cobra.Command {
	opts := &declarativeOptions{}

	cmd := &cobra.Command{
		Use:   formatDeclarative,
		Short: i18n.T("root.verbs.dump.declarative.short", "Export resources as kongctl declarative configuration"),
		Long: normalizers.LongDesc(i18n.T("root.verbs.dump.declarative.long",
			"Export existing Konnect resources as kongctl declarative configuration.")),
		RunE: func(cmd *cobra.Command, args []string) error {
			helper := cmdpkg.BuildHelper(cmd, args)
			resourcesFlag := cmd.Flags().Lookup("resources").Value.String()
			normalized, err := normalizeResourceList(resourcesFlag, declarativeAllowedResources)
			if err != nil {
				return err
			}
			opts.resources = normalized
			if err := validateFilterOptions(opts.filter); err != nil {
				return err
			}
			if err := ensureNonNegativePageSize(helper); err != nil {
				return err
			}
			return runDeclarativeDump(helper, *opts)
		},
	}

	cmd.Flags().String("resources", "",
		"Comma separated list of resource types to dump "+
			"(portals, apis, application_auth_strategies, dcr_providers, control_planes, "+
			resourceAnalyticsDashboards+", event_gateways, ai_gateways, ai_gateway_identity_providers, "+
			"ai_gateway_policies, ai_gateway_agents, ai_gateway_consumers, ai_gateway_consumer_credentials, "+
			"ai_gateway_consumer_groups, ai_gateway_models, ai_gateway_mcp_servers, ai_gateway_vaults, "+
			"ai_gateway_data_plane_certificates, organization.teams).")
	_ = cmd.MarkFlagRequired("resources")

	cmd.Flags().BoolVar(&opts.includeChildResources, "include-child-resources", false,
		"Include child resources in the dump.")

	cmd.Flags().StringVar(&opts.outputFile, "output-file", "",
		"File to write the output to. If not specified, output is written to stdout.")

	cmd.Flags().StringVar(&opts.defaultNamespace, "default-namespace", "",
		"Default namespace to include in declarative output (_defaults.kongctl.namespace).")

	cmd.Flags().StringVar(&opts.filter.name, filterNameFlagName, "",
		"Filter resources by name. Use '*' wildcards for substring matching (e.g., '*portal*').\n"+
			"Mutually exclusive with --"+filterIDFlagName+".")

	cmd.Flags().StringVar(&opts.filter.id, filterIDFlagName, "",
		"Filter resources by ID (exact match).\n"+
			"Mutually exclusive with --"+filterNameFlagName+".")

	cmd.Flags().String(konnectCommon.BaseURLFlagName, "",
		fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]
- Default   : [ %s ]`,
			konnectCommon.BaseURLConfigPath, konnectCommon.BaseURLDefault))

	cmd.Flags().String(
		konnectCommon.RegionFlagName, "",
		fmt.Sprintf(`Konnect region identifier (for example "eu"). Used to construct the base URL when --%s is not provided.
- Config path: [ %s ]`,
			konnectCommon.BaseURLFlagName, konnectCommon.RegionConfigPath),
	)

	cmd.Flags().String(konnectCommon.PATFlagName, "",
		fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI.
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`,
			konnectCommon.PATConfigPath))

	cmd.Flags().Int(
		konnectCommon.RequestPageSizeFlagName,
		konnectCommon.DefaultRequestPageSize,
		fmt.Sprintf(`Max number of results to include per response page.
- Config path: [ %s ]`, konnectCommon.RequestPageSizeConfigPath),
	)

	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		helper := cmdpkg.BuildHelper(c, args)
		cfg, err := helper.GetConfig()
		if err != nil {
			return err
		}

		bindings := []struct{ flag, configPath string }{
			{konnectCommon.BaseURLFlagName, konnectCommon.BaseURLConfigPath},
			{konnectCommon.RegionFlagName, konnectCommon.RegionConfigPath},
			{konnectCommon.PATFlagName, konnectCommon.PATConfigPath},
			{konnectCommon.RequestPageSizeFlagName, konnectCommon.RequestPageSizeConfigPath},
		}
		for _, b := range bindings {
			if err := bindFlag(cfg, c.Flags(), b.flag, b.configPath); err != nil {
				return err
			}
		}
		return nil
	}

	return cmd
}

// RunDeclarativeDump exports Konnect resources as kongctl declarative configuration.
func RunDeclarativeDump(helper cmdpkg.Helper, opts DeclarativeDumpOptions) error {
	resources, err := normalizeResourceList(strings.Join(opts.Resources, ","), declarativeAllowedResources)
	if err != nil {
		return err
	}

	declarativeOpts := declarativeOptions{
		resources:             resources,
		outputFile:            strings.TrimSpace(opts.OutputFile),
		defaultNamespace:      strings.TrimSpace(opts.DefaultNamespace),
		includeChildResources: opts.IncludeChildResources,
		filter: filterOptions{
			name: strings.TrimSpace(opts.FilterName),
			id:   strings.TrimSpace(opts.FilterID),
		},
	}
	if err := validateFilterOptions(declarativeOpts.filter); err != nil {
		return err
	}
	if err := ensureNonNegativePageSize(helper); err != nil {
		return err
	}

	return runDeclarativeDump(helper, declarativeOpts)
}

func runDeclarativeDump(helper cmdpkg.Helper, opts declarativeOptions) error {
	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	var stateClient *declstate.Client
	if opts.includeChildResources ||
		slices.Contains(opts.resources, "ai_gateway_identity_providers") ||
		slices.Contains(opts.resources, "ai_gateway_policies") ||
		slices.Contains(opts.resources, "ai_gateway_agents") ||
		slices.Contains(opts.resources, "ai_gateway_consumers") ||
		slices.Contains(opts.resources, "ai_gateway_consumer_credentials") ||
		slices.Contains(opts.resources, "ai_gateway_consumer_groups") ||
		slices.Contains(opts.resources, "ai_gateway_models") ||
		slices.Contains(opts.resources, "ai_gateway_mcp_servers") ||
		slices.Contains(opts.resources, "ai_gateway_vaults") ||
		slices.Contains(opts.resources, "ai_gateway_data_plane_certificates") {
		stateClient = declstate.NewClient(declstate.ClientConfig{
			PortalAPI:                           sdk.GetPortalAPI(),
			APIAPI:                              sdk.GetAPIAPI(),
			AppAuthAPI:                          sdk.GetAppAuthStrategiesAPI(),
			DCRProviderAPI:                      sdk.GetDCRProvidersAPI(),
			ControlPlaneAPI:                     sdk.GetControlPlaneAPI(),
			GatewayServiceAPI:                   sdk.GetGatewayServiceAPI(),
			DataPlaneCertificateAPI:             sdk.GetDataPlaneCertificateAPI(),
			ControlPlaneGroupsAPI:               sdk.GetControlPlaneGroupsAPI(),
			CatalogServiceAPI:                   sdk.GetCatalogServicesAPI(),
			AIGatewayAPI:                        sdk.GetAIGatewayAPI(),
			AIGatewayProvidersAPI:               sdk.GetAIGatewayProvidersAPI(),
			AIGatewayIdentityProvidersAPI:       sdk.GetAIGatewayIdentityProvidersAPI(),
			AIGatewayPoliciesAPI:                sdk.GetAIGatewayPoliciesAPI(),
			AIGatewayAgentsAPI:                  sdk.GetAIGatewayAgentsAPI(),
			AIGatewayConsumersAPI:               sdk.GetAIGatewayConsumersAPI(),
			AIGatewayConsumerGroupsAPI:          sdk.GetAIGatewayConsumerGroupsAPI(),
			AIGatewayModelAPI:                   sdk.GetAIGatewayModelAPI(),
			AIGatewayMCPServersAPI:              sdk.GetAIGatewayMCPServersAPI(),
			AIGatewayVaultsAPI:                  sdk.GetAIGatewayVaultsAPI(),
			AIGatewayDataPlaneCertificatesAPI:   sdk.GetAIGatewayDataPlaneCertificatesAPI(),
			DashboardsAPI:                       sdk.GetDashboardsAPI(),
			PortalPageAPI:                       sdk.GetPortalPageAPI(),
			PortalAuthSettingsAPI:               sdk.GetPortalAuthSettingsAPI(),
			PortalIPAllowListAPI:                sdk.GetPortalIPAllowListAPI(),
			PortalIntegrationsAPI:               sdk.GetPortalIntegrationsAPI(),
			PortalIdentityProviderAPI:           sdk.GetPortalIdentityProviderAPI(),
			PortalCustomizationAPI:              sdk.GetPortalCustomizationAPI(),
			PortalCustomDomainAPI:               sdk.GetPortalCustomDomainAPI(),
			PortalSnippetAPI:                    sdk.GetPortalSnippetAPI(),
			PortalTeamAPI:                       sdk.GetPortalTeamAPI(),
			PortalTeamRolesAPI:                  sdk.GetPortalTeamRolesAPI(),
			PortalEmailsAPI:                     sdk.GetPortalEmailsAPI(),
			PortalAuditLogsAPI:                  sdk.GetPortalAuditLogsAPI(),
			AssetsAPI:                           sdk.GetAssetsAPI(),
			AuditLogDestinationsAPI:             sdk.GetAuditLogDestinationsAPI(),
			APIVersionAPI:                       sdk.GetAPIVersionAPI(),
			APIPublicationAPI:                   sdk.GetAPIPublicationAPI(),
			APIImplementationAPI:                sdk.GetAPIImplementationAPI(),
			APIDocumentAPI:                      sdk.GetAPIDocumentAPI(),
			EGWControlPlaneAPI:                  sdk.GetEventGatewayControlPlaneAPI(),
			EventGatewayBackendClusterAPI:       sdk.GetEventGatewayBackendClusterAPI(),
			EventGatewayVirtualClusterAPI:       sdk.GetEventGatewayVirtualClusterAPI(),
			EventGatewayListenerAPI:             sdk.GetEventGatewayListenerAPI(),
			EventGatewayListenerPolicyAPI:       sdk.GetEventGatewayListenerPolicyAPI(),
			EventGatewayDataPlaneCertificateAPI: sdk.GetEventGatewayDataPlaneCertificateAPI(),
			EventGatewayProducePolicyAPI:        sdk.GetEventGatewayProducePolicyAPI(),
			EventGatewayClusterPolicyAPI:        sdk.GetEventGatewayClusterPolicyAPI(),
			EventGatewayConsumePolicyAPI:        sdk.GetEventGatewayConsumePolicyAPI(),
			EventGatewaySchemaRegistryAPI:       sdk.GetEventGatewaySchemaRegistryAPI(),
			EventGatewayStaticKeyAPI:            sdk.GetEventGatewayStaticKeyAPI(),
			EventGatewayTLSTrustBundleAPI:       sdk.GetEventGatewayTLSTrustBundleAPI(),
			OrganizationTeamAPI:                 sdk.GetOrganizationTeamAPI(),
			OrganizationTeamRolesAPI:            sdk.GetOrganizationTeamRolesAPI(),
			OrganizationUsersAPI:                sdk.GetOrganizationUsersAPI(),
			OrganizationMembershipAPI:           sdk.GetOrganizationTeamMembershipAPI(),
		})
	}

	writer, cleanup, err := getDumpWriter(helper, opts.outputFile)
	if err != nil {
		return err
	}
	defer func() {
		_ = cleanup()
	}()

	ctx := helper.GetContext()
	resourceSet := declresources.ResourceSet{}

	requestPageSize := int64(cfg.GetIntOrElse(
		konnectCommon.RequestPageSizeConfigPath,
		konnectCommon.DefaultRequestPageSize,
	))

	for _, resource := range opts.resources {
		switch resource {
		case "portals":
			portals, err := collectDeclarativePortals(ctx, sdk.GetPortalAPI(), requestPageSize, opts.filter)
			if err != nil {
				return err
			}
			if opts.includeChildResources {
				if err := populatePortalChildren(ctx, logger, stateClient, portals); err != nil {
					return err
				}
			}
			resourceSet.Portals = append(resourceSet.Portals, portals...)
		case resourceAPIs:
			apis, err := collectDeclarativeAPIs(ctx, sdk.GetAPIAPI(), requestPageSize, opts.filter)
			if err != nil {
				return err
			}
			if opts.includeChildResources {
				populateAPIChildren(ctx, logger, stateClient, apis)
			}
			resourceSet.APIs = append(resourceSet.APIs, apis...)
		case "application_auth_strategies":
			authStrategies, err := collectDeclarativeAuthStrategies(
				ctx, sdk.GetAppAuthStrategiesAPI(), requestPageSize, opts.filter,
			)
			if err != nil {
				return err
			}
			resourceSet.ApplicationAuthStrategies = append(resourceSet.ApplicationAuthStrategies, authStrategies...)
		case "dcr_providers":
			dcrProviders, err := collectDeclarativeDCRProviders(
				ctx, sdk.GetDCRProvidersAPI(), requestPageSize, opts.filter,
			)
			if err != nil {
				return err
			}
			resourceSet.DCRProviders = append(resourceSet.DCRProviders, dcrProviders...)
		case "control_planes":
			controlPlanes, err := collectDeclarativeControlPlanes(
				ctx,
				sdk.GetControlPlaneAPI(),
				sdk.GetControlPlaneGroupsAPI(),
				requestPageSize,
				opts.filter,
			)
			if err != nil {
				return err
			}
			if opts.includeChildResources {
				populateControlPlaneChildren(ctx, logger, stateClient, controlPlanes)
			}
			resourceSet.ControlPlanes = append(resourceSet.ControlPlanes, controlPlanes...)
		case resourceAnalyticsDashboards:
			dashboards, err := collectDeclarativeDashboards(ctx, sdk.GetDashboardsAPI(), requestPageSize, opts.filter)
			if err != nil {
				return err
			}
			if resourceSet.Analytics == nil {
				resourceSet.Analytics = &declresources.AnalyticsResource{}
			}
			resourceSet.Analytics.Dashboards = append(resourceSet.Analytics.Dashboards, dashboards...)
		case "event_gateways":
			eventGateways, err := collectDeclarativeEventGateways(
				ctx,
				sdk.GetEventGatewayControlPlaneAPI(),
				requestPageSize,
				opts.filter,
			)
			if err != nil {
				return err
			}
			if opts.includeChildResources {
				populateEventGatewayChildren(ctx, logger, stateClient, eventGateways)
			}
			resourceSet.EventGatewayControlPlanes = append(resourceSet.EventGatewayControlPlanes, eventGateways...)
		case "ai_gateways":
			aiGateways, err := collectDeclarativeAIGateways(
				ctx,
				sdk.GetAIGatewayAPI(),
				requestPageSize,
				opts.filter,
			)
			if err != nil {
				return err
			}
			if opts.includeChildResources {
				populateAIGatewayChildren(ctx, logger, stateClient, aiGateways)
			}
			resourceSet.AIGateways = append(resourceSet.AIGateways, aiGateways...)
		case "ai_gateway_identity_providers":
			providers, err := collectDeclarativeAIGatewayIdentityProviders(
				ctx,
				stateClient,
				sdk.GetAIGatewayAPI(),
				requestPageSize,
				opts.filter,
			)
			if err != nil {
				return err
			}
			resourceSet.AIGatewayIdentityProviders = append(resourceSet.AIGatewayIdentityProviders, providers...)
		case "ai_gateway_policies":
			policies, err := collectDeclarativeAIGatewayPolicies(
				ctx,
				stateClient,
				sdk.GetAIGatewayAPI(),
				requestPageSize,
				opts.filter,
			)
			if err != nil {
				return err
			}
			resourceSet.AIGatewayPolicies = append(resourceSet.AIGatewayPolicies, policies...)
		case "ai_gateway_agents":
			agents, err := collectDeclarativeAIGatewayAgents(
				ctx,
				stateClient,
				sdk.GetAIGatewayAPI(),
				requestPageSize,
				opts.filter,
			)
			if err != nil {
				return err
			}
			resourceSet.AIGatewayAgents = append(resourceSet.AIGatewayAgents, agents...)
		case "ai_gateway_consumers":
			consumers, err := collectDeclarativeAIGatewayConsumers(
				ctx,
				stateClient,
				sdk.GetAIGatewayAPI(),
				requestPageSize,
				opts.filter,
			)
			if err != nil {
				return err
			}
			resourceSet.AIGatewayConsumers = append(resourceSet.AIGatewayConsumers, consumers...)
		case "ai_gateway_consumer_credentials":
			credentials, err := collectDeclarativeAIGatewayConsumerCredentials(
				ctx,
				stateClient,
				sdk.GetAIGatewayAPI(),
				requestPageSize,
				opts.filter,
			)
			if err != nil {
				return err
			}
			resourceSet.AIGatewayConsumerCredentials = append(resourceSet.AIGatewayConsumerCredentials, credentials...)
		case "ai_gateway_consumer_groups":
			groups, err := collectDeclarativeAIGatewayConsumerGroups(
				ctx,
				stateClient,
				sdk.GetAIGatewayAPI(),
				requestPageSize,
				opts.filter,
			)
			if err != nil {
				return err
			}
			resourceSet.AIGatewayConsumerGroups = append(resourceSet.AIGatewayConsumerGroups, groups...)
		case "ai_gateway_models":
			models, err := collectDeclarativeAIGatewayModels(
				ctx,
				stateClient,
				sdk.GetAIGatewayAPI(),
				requestPageSize,
				opts.filter,
			)
			if err != nil {
				return err
			}
			resourceSet.AIGatewayModels = append(resourceSet.AIGatewayModels, models...)
		case "ai_gateway_mcp_servers":
			servers, err := collectDeclarativeAIGatewayMCPServers(
				ctx,
				stateClient,
				sdk.GetAIGatewayAPI(),
				requestPageSize,
				opts.filter,
			)
			if err != nil {
				return err
			}
			resourceSet.AIGatewayMCPServers = append(resourceSet.AIGatewayMCPServers, servers...)
		case "ai_gateway_vaults":
			vaults, err := collectDeclarativeAIGatewayVaults(
				ctx,
				stateClient,
				sdk.GetAIGatewayAPI(),
				requestPageSize,
				opts.filter,
			)
			if err != nil {
				return err
			}
			resourceSet.AIGatewayVaults = append(resourceSet.AIGatewayVaults, vaults...)
		case "ai_gateway_data_plane_certificates":
			certs, err := collectDeclarativeAIGatewayDataPlaneCertificates(
				ctx,
				stateClient,
				sdk.GetAIGatewayAPI(),
				requestPageSize,
				opts.filter,
			)
			if err != nil {
				return err
			}
			resourceSet.AIGatewayDataPlaneCertificates = append(resourceSet.AIGatewayDataPlaneCertificates, certs...)
		case "organization.teams":
			teams, err := collectDeclarativeOrganizationTeams(
				ctx,
				sdk.GetOrganizationTeamAPI(),
				requestPageSize,
				opts.filter,
			)
			if err != nil {
				return err
			}
			if opts.includeChildResources {
				populateOrganizationTeamChildren(ctx, logger, stateClient, teams)
			}
			// Wrap teams in organization grouping for the new format
			if resourceSet.Organization == nil {
				resourceSet.Organization = &declresources.OrganizationResource{}
			}
			resourceSet.Organization.Teams = append(resourceSet.Organization.Teams, teams...)
			if opts.includeChildResources {
				resourceSet.Organization.Users = append(
					resourceSet.Organization.Users,
					collectOrganizationUsersFromTeamMemberships(ctx, logger, stateClient, teams)...,
				)
			}
		}
	}

	resourceSet.AddDefaultNamespace(opts.defaultNamespace)

	output := declarativeDumpOutput{
		Defaults:    buildDeclarativeDefaults(opts.defaultNamespace),
		ResourceSet: resourceSet,
	}

	yamlBytes, err := yaml.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal declarative configuration: %w", err)
	}

	if len(yamlBytes) == 0 {
		return nil
	}

	if _, err := writer.Write(yamlBytes); err != nil {
		return fmt.Errorf("failed to write declarative configuration: %w", err)
	}

	return nil
}

type declarativeDumpOutput struct {
	Defaults                  *declresources.FileDefaults `json:"_defaults,omitempty" yaml:"_defaults,omitempty"`
	declresources.ResourceSet `json:",inline" yaml:",inline"`
}

func buildDeclarativeDefaults(namespace string) *declresources.FileDefaults {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		return nil
	}

	return &declresources.FileDefaults{
		Kongctl: &declresources.KongctlMetaDefaults{
			Namespace: &ns,
		},
	}
}

func collectDeclarativePortals(
	ctx context.Context,
	portalAPI helpers.PortalAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.PortalResource, error) {
	if portalAPI == nil {
		return nil, fmt.Errorf("portal API client is not configured")
	}

	var results []declresources.PortalResource

	err := processPaginatedRequests(func(pageNumber int64) (bool, error) {
		req := kkOps.ListPortalsRequest{
			PageSize:   &requestPageSize,
			PageNumber: &pageNumber,
		}

		if filter.name != "" {
			req.Filter = &kkComps.PortalFilterParameters{Name: buildStringFieldFilter(filter.name)}
		} else if filter.id != "" {
			req.Filter = &kkComps.PortalFilterParameters{ID: &kkComps.UUIDFieldFilter{Eq: &filter.id}}
		}

		resp, err := portalAPI.ListPortals(ctx, req)
		if err != nil {
			return false, fmt.Errorf("failed to list portals: %w", err)
		}

		if resp.ListPortalsResponse == nil || len(resp.ListPortalsResponse.Data) == 0 {
			return false, nil
		}

		for _, portal := range resp.ListPortalsResponse.Data {
			resource := mapPortalToDeclarativeResource(portal)
			if portalID := strings.TrimSpace(portal.GetID()); portalID != "" {
				detail, err := portalAPI.GetPortal(ctx, portalID)
				if err != nil {
					return false, fmt.Errorf("failed to get portal %q: %w", portalID, err)
				}
				if detailedPortal := detail.GetPortalResponse(); detailedPortal != nil {
					resource = mapPortalResponseToDeclarativeResource(*detailedPortal)
				}
			}

			results = append(results, resource)
		}

		return true, nil
	})
	if err != nil {
		return nil, err
	}

	slices.SortFunc(results, func(a, b declresources.PortalResource) int {
		if n := cmp.Compare(a.Name, b.Name); n != 0 {
			return n
		}
		return cmp.Compare(a.Ref, b.Ref)
	})

	return results, nil
}

func collectDeclarativeAPIs(
	ctx context.Context,
	apiClient helpers.APIAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.APIResource, error) {
	if apiClient == nil {
		return nil, fmt.Errorf("API client is not configured")
	}

	var results []declresources.APIResource

	err := processPaginatedRequests(func(pageNumber int64) (bool, error) {
		req := kkOps.ListApisRequest{
			PageSize:   &requestPageSize,
			PageNumber: &pageNumber,
		}

		if filter.name != "" {
			req.Filter = &kkComps.APIFilterParameters{Name: buildStringFieldFilter(filter.name)}
		} else if filter.id != "" {
			req.Filter = &kkComps.APIFilterParameters{ID: &kkComps.UUIDFieldFilter{Eq: &filter.id}}
		}

		resp, err := apiClient.ListApis(ctx, req)
		if err != nil {
			return false, fmt.Errorf("failed to list APIs: %w", err)
		}

		if resp == nil || resp.ListAPIResponse == nil || len(resp.ListAPIResponse.Data) == 0 {
			return false, nil
		}

		for _, api := range resp.ListAPIResponse.Data {
			results = append(results, mapAPIToDeclarativeResource(api))
		}

		params := paginationParams{
			pageSize:   requestPageSize,
			pageNumber: pageNumber,
			totalItems: resp.ListAPIResponse.Meta.Page.Total,
		}
		return params.hasMorePages(), nil
	})
	if err != nil {
		return nil, err
	}

	slices.SortFunc(results, func(a, b declresources.APIResource) int {
		if n := cmp.Compare(a.Name, b.Name); n != 0 {
			return n
		}
		return cmp.Compare(a.Ref, b.Ref)
	})

	return results, nil
}

func collectDeclarativeDashboards(
	ctx context.Context,
	api helpers.DashboardsAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.DashboardResource, error) {
	if api == nil {
		return nil, fmt.Errorf("dashboards API client is not configured")
	}

	var results []declresources.DashboardResource

	err := processPaginatedRequests(func(pageNumber int64) (bool, error) {
		req := kkOps.DashboardsListRequest{
			PageSize:   &requestPageSize,
			PageNumber: &pageNumber,
		}

		if filter.name != "" {
			req.Filter = &kkComps.DashboardFilterParameters{Name: buildStringFieldFilter(filter.name)}
		} else if filter.id != "" {
			req.Filter = &kkComps.DashboardFilterParameters{ID: &kkComps.UUIDFieldFilter{Eq: &filter.id}}
		}

		resp, err := api.DashboardsList(ctx, req)
		if err != nil {
			return false, fmt.Errorf("failed to list dashboards: %w", err)
		}

		if resp == nil || resp.Object == nil || len(resp.Object.Data) == 0 {
			return false, nil
		}

		for _, dashboard := range resp.Object.Data {
			results = append(results, mapDashboardToDeclarativeResource(dashboard))
		}

		var total float64
		if resp.Object.Meta != nil {
			total = resp.Object.Meta.Page.Total
		}
		params := paginationParams{
			pageSize:   requestPageSize,
			pageNumber: pageNumber,
			totalItems: total,
		}
		return params.hasMorePages(), nil
	})
	if err != nil {
		return nil, err
	}

	slices.SortFunc(results, func(a, b declresources.DashboardResource) int {
		if n := cmp.Compare(a.Name, b.Name); n != 0 {
			return n
		}
		return cmp.Compare(a.Ref, b.Ref)
	})

	return results, nil
}

func collectDeclarativeEventGateways(
	ctx context.Context,
	eventGatewayClient helpers.EGWControlPlaneAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.EventGatewayControlPlaneResource, error) {
	if eventGatewayClient == nil {
		return nil, fmt.Errorf("event gateway client is not configured")
	}

	var allData []declresources.EventGatewayControlPlaneResource
	var pageAfter *string

	for {
		req := kkOps.ListEventGatewaysRequest{
			PageSize: &requestPageSize,
		}

		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		// Event gateway filter only supports contains on name; use it for
		// server-side narrowing when possible, then apply exact match
		// or ID filtering client-side below.
		if filter.name != "" {
			_, val := parseFilterName(filter.name)
			req.Filter = &kkComps.EventGatewayCommonFilter{
				Name: &kkComps.StringFieldContainsFilter{Contains: val},
			}
		}

		res, err := eventGatewayClient.ListEGWControlPlanes(ctx, req)
		if err != nil {
			return nil, err
		}

		for _, egw := range res.ListEventGatewaysResponse.Data {
			allData = append(allData, mapEventGatewayToDeclarativeResource(egw))
		}

		nextCursor := pagination.ExtractPageAfterCursor(res.ListEventGatewaysResponse.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = stringPointer(nextCursor)
	}

	// Client-side filtering for exact name match or ID (not supported server-side)
	if filter.hasFilter() {
		allData = filterByNameOrID(
			allData,
			filter,
			func(r declresources.EventGatewayControlPlaneResource) (string, string) {
				return r.Name, r.Ref
			},
		)
	}

	slices.SortFunc(allData, func(a, b declresources.EventGatewayControlPlaneResource) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return allData, nil
}

func collectDeclarativeAIGateways(
	ctx context.Context,
	aiGatewayClient helpers.AIGatewayAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.AIGatewayResource, error) {
	if aiGatewayClient == nil {
		return nil, fmt.Errorf("AI Gateway API client is not configured")
	}

	var results []declresources.AIGatewayResource

	err := processPaginatedRequests(func(pageNumber int64) (bool, error) {
		resp, err := aiGatewayClient.ListAiGateways(ctx, &requestPageSize, &pageNumber)
		if err != nil {
			return false, fmt.Errorf("failed to list AI Gateways: %w", err)
		}

		if resp == nil || resp.ListAIGatewaysResponse == nil || len(resp.ListAIGatewaysResponse.Data) == 0 {
			return false, nil
		}

		for _, gateway := range resp.ListAIGatewaysResponse.Data {
			results = append(results, mapAIGatewayToDeclarativeResource(gateway))
		}

		params := paginationParams{
			pageSize:   requestPageSize,
			pageNumber: pageNumber,
			totalItems: resp.ListAIGatewaysResponse.Meta.Page.Total,
		}
		return params.hasMorePages(), nil
	})
	if err != nil {
		return nil, err
	}

	results = filterByNameOrID(results, filter, func(r declresources.AIGatewayResource) (string, string) {
		return r.Name, r.Ref
	})

	slices.SortFunc(results, func(a, b declresources.AIGatewayResource) int {
		if a.Name == b.Name {
			return cmp.Compare(a.Ref, b.Ref)
		}
		return cmp.Compare(a.Name, b.Name)
	})

	return results, nil
}

func collectDeclarativeAIGatewayPolicies(
	ctx context.Context,
	client *declstate.Client,
	aiGatewayClient helpers.AIGatewayAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.AIGatewayPolicyResource, error) {
	if client == nil {
		return nil, fmt.Errorf("AI Gateway Policies API client is not configured")
	}

	gateways, err := collectDeclarativeAIGateways(ctx, aiGatewayClient, requestPageSize, filterOptions{})
	if err != nil {
		return nil, err
	}

	var policies []declresources.AIGatewayPolicyResource
	for _, gateway := range gateways {
		gatewayPolicies, err := buildAIGatewayPolicies(ctx, client, gateway.Ref, gateway.DisplayName, gateway.Ref)
		if err != nil {
			return nil, err
		}
		policies = append(policies, gatewayPolicies...)
	}

	policies = filterByNameOrID(policies, filter, func(r declresources.AIGatewayPolicyResource) (string, string) {
		return r.Name, r.Ref
	})
	slices.SortFunc(policies, func(a, b declresources.AIGatewayPolicyResource) int {
		if a.AIGateway == b.AIGateway {
			return cmp.Compare(a.Name, b.Name)
		}
		return cmp.Compare(a.AIGateway, b.AIGateway)
	})

	return policies, nil
}

func collectDeclarativeAIGatewayIdentityProviders(
	ctx context.Context,
	client *declstate.Client,
	aiGatewayClient helpers.AIGatewayAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.AIGatewayIdentityProviderResource, error) {
	if client == nil {
		return nil, fmt.Errorf("AI Gateway Identity Providers API client is not configured")
	}

	gateways, err := collectDeclarativeAIGateways(ctx, aiGatewayClient, requestPageSize, filterOptions{})
	if err != nil {
		return nil, err
	}

	var providers []declresources.AIGatewayIdentityProviderResource
	for _, gateway := range gateways {
		gatewayProviders, err := buildAIGatewayIdentityProviders(
			ctx,
			slog.Default(),
			client,
			gateway.Ref,
			gateway.DisplayName,
		)
		if err != nil {
			return nil, err
		}
		providers = append(providers, gatewayProviders...)
	}

	providers = filterByNameOrID(providers, filter, func(r declresources.AIGatewayIdentityProviderResource) (
		string,
		string,
	) {
		return r.Name, r.Ref
	})
	slices.SortFunc(providers, func(a, b declresources.AIGatewayIdentityProviderResource) int {
		if a.AIGateway == b.AIGateway {
			return cmp.Compare(a.Name, b.Name)
		}
		return cmp.Compare(a.AIGateway, b.AIGateway)
	})

	return providers, nil
}

func collectDeclarativeAIGatewayAgents(
	ctx context.Context,
	client *declstate.Client,
	aiGatewayClient helpers.AIGatewayAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.AIGatewayAgentResource, error) {
	if client == nil {
		return nil, fmt.Errorf("AI Gateway Agents API client is not configured")
	}

	gateways, err := collectDeclarativeAIGateways(ctx, aiGatewayClient, requestPageSize, filterOptions{})
	if err != nil {
		return nil, err
	}

	var agents []declresources.AIGatewayAgentResource
	for _, gateway := range gateways {
		gatewayAgents, err := buildAIGatewayAgents(ctx, client, gateway.Ref, gateway.DisplayName, gateway.Ref)
		if err != nil {
			return nil, err
		}
		agents = append(agents, gatewayAgents...)
	}

	agents = filterByNameOrID(agents, filter, func(r declresources.AIGatewayAgentResource) (string, string) {
		return r.Name, r.Ref
	})
	slices.SortFunc(agents, func(a, b declresources.AIGatewayAgentResource) int {
		if a.AIGateway == b.AIGateway {
			return cmp.Compare(a.Name, b.Name)
		}
		return cmp.Compare(a.AIGateway, b.AIGateway)
	})

	return agents, nil
}

func collectDeclarativeAIGatewayConsumerGroups(
	ctx context.Context,
	client *declstate.Client,
	aiGatewayClient helpers.AIGatewayAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.AIGatewayConsumerGroupResource, error) {
	if client == nil {
		return nil, fmt.Errorf("AI Gateway Consumer Groups API client is not configured")
	}

	gateways, err := collectDeclarativeAIGateways(ctx, aiGatewayClient, requestPageSize, filterOptions{})
	if err != nil {
		return nil, err
	}

	var groups []declresources.AIGatewayConsumerGroupResource
	for _, gateway := range gateways {
		gatewayGroups, err := buildAIGatewayConsumerGroups(ctx, client, gateway.Ref, gateway.DisplayName, gateway.Ref)
		if err != nil {
			return nil, err
		}
		groups = append(groups, gatewayGroups...)
	}

	groups = filterByNameOrID(groups, filter, func(r declresources.AIGatewayConsumerGroupResource) (string, string) {
		return r.Name, r.Ref
	})
	slices.SortFunc(groups, func(a, b declresources.AIGatewayConsumerGroupResource) int {
		if a.AIGateway == b.AIGateway {
			return cmp.Compare(a.Name, b.Name)
		}
		return cmp.Compare(a.AIGateway, b.AIGateway)
	})

	return groups, nil
}

func collectDeclarativeAIGatewayConsumers(
	ctx context.Context,
	client *declstate.Client,
	aiGatewayClient helpers.AIGatewayAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.AIGatewayConsumerResource, error) {
	if client == nil {
		return nil, fmt.Errorf("AI Gateway Consumers API client is not configured")
	}

	gateways, err := collectDeclarativeAIGateways(ctx, aiGatewayClient, requestPageSize, filterOptions{})
	if err != nil {
		return nil, err
	}

	var consumers []declresources.AIGatewayConsumerResource
	for _, gateway := range gateways {
		gatewayConsumers, err := buildAIGatewayConsumers(ctx, client, gateway.Ref, gateway.DisplayName, gateway.Ref, false)
		if err != nil {
			return nil, err
		}
		consumers = append(consumers, gatewayConsumers...)
	}

	consumers = filterByNameOrID(consumers, filter, func(r declresources.AIGatewayConsumerResource) (string, string) {
		return r.Name, r.Ref
	})
	slices.SortFunc(consumers, func(a, b declresources.AIGatewayConsumerResource) int {
		if a.AIGateway == b.AIGateway {
			return cmp.Compare(a.Name, b.Name)
		}
		return cmp.Compare(a.AIGateway, b.AIGateway)
	})

	return consumers, nil
}

func collectDeclarativeAIGatewayConsumerCredentials(
	ctx context.Context,
	client *declstate.Client,
	aiGatewayClient helpers.AIGatewayAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.AIGatewayConsumerCredentialResource, error) {
	if client == nil {
		return nil, fmt.Errorf("AI Gateway Consumer Credentials API client is not configured")
	}

	gateways, err := collectDeclarativeAIGateways(ctx, aiGatewayClient, requestPageSize, filterOptions{})
	if err != nil {
		return nil, err
	}

	var credentials []declresources.AIGatewayConsumerCredentialResource
	for _, gateway := range gateways {
		consumers, err := buildAIGatewayConsumers(ctx, client, gateway.Ref, gateway.DisplayName, gateway.Ref, false)
		if err != nil {
			return nil, err
		}
		for _, consumer := range consumers {
			consumerCredentials, err := buildAIGatewayConsumerCredentials(
				ctx,
				client,
				gateway.Ref,
				gateway.DisplayName,
				consumer.Ref,
				consumer.Name,
				consumer.Ref,
			)
			if err != nil {
				return nil, err
			}
			credentials = append(credentials, consumerCredentials...)
		}
	}

	credentials = filterByNameOrID(credentials, filter, func(r declresources.AIGatewayConsumerCredentialResource) (
		string,
		string,
	) {
		return r.Name, r.Ref
	})
	slices.SortFunc(credentials, func(a, b declresources.AIGatewayConsumerCredentialResource) int {
		if a.AIGatewayConsumer == b.AIGatewayConsumer {
			return cmp.Compare(a.Name, b.Name)
		}
		return cmp.Compare(a.AIGatewayConsumer, b.AIGatewayConsumer)
	})

	return credentials, nil
}

func collectDeclarativeAIGatewayModels(
	ctx context.Context,
	client *declstate.Client,
	aiGatewayClient helpers.AIGatewayAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.AIGatewayModelResource, error) {
	if client == nil {
		return nil, fmt.Errorf("AI Gateway model API client is not configured")
	}

	gateways, err := collectDeclarativeAIGateways(ctx, aiGatewayClient, requestPageSize, filterOptions{})
	if err != nil {
		return nil, err
	}

	var models []declresources.AIGatewayModelResource
	for _, gateway := range gateways {
		gatewayModels, err := buildAIGatewayModels(ctx, client, gateway.Ref, gateway.DisplayName, gateway.Ref)
		if err != nil {
			return nil, err
		}
		models = append(models, gatewayModels...)
	}

	models = filterByNameOrID(models, filter, func(r declresources.AIGatewayModelResource) (string, string) {
		return r.Name(), r.Ref
	})
	slices.SortFunc(models, func(a, b declresources.AIGatewayModelResource) int {
		if a.AIGateway == b.AIGateway {
			return cmp.Compare(a.Name(), b.Name())
		}
		return cmp.Compare(a.AIGateway, b.AIGateway)
	})

	return models, nil
}

func collectDeclarativeAIGatewayMCPServers(
	ctx context.Context,
	client *declstate.Client,
	aiGatewayClient helpers.AIGatewayAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.AIGatewayMCPServerResource, error) {
	if client == nil {
		return nil, fmt.Errorf("AI Gateway MCP Servers API client is not configured")
	}

	gateways, err := collectDeclarativeAIGateways(ctx, aiGatewayClient, requestPageSize, filterOptions{})
	if err != nil {
		return nil, err
	}

	var servers []declresources.AIGatewayMCPServerResource
	for _, gateway := range gateways {
		gatewayServers, err := buildAIGatewayMCPServers(ctx, client, gateway.Ref, gateway.DisplayName, gateway.Ref)
		if err != nil {
			return nil, err
		}
		servers = append(servers, gatewayServers...)
	}

	servers = filterByNameOrID(servers, filter, func(r declresources.AIGatewayMCPServerResource) (string, string) {
		return r.Name(), r.Ref
	})
	slices.SortFunc(servers, func(a, b declresources.AIGatewayMCPServerResource) int {
		if a.AIGateway == b.AIGateway {
			return cmp.Compare(a.Name(), b.Name())
		}
		return cmp.Compare(a.AIGateway, b.AIGateway)
	})

	return servers, nil
}

func collectDeclarativeAIGatewayVaults(
	ctx context.Context,
	client *declstate.Client,
	aiGatewayClient helpers.AIGatewayAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.AIGatewayVaultResource, error) {
	if client == nil {
		return nil, fmt.Errorf("AI Gateway Vaults API client is not configured")
	}

	gateways, err := collectDeclarativeAIGateways(ctx, aiGatewayClient, requestPageSize, filterOptions{})
	if err != nil {
		return nil, err
	}

	var vaults []declresources.AIGatewayVaultResource
	for _, gateway := range gateways {
		gatewayVaults, err := buildAIGatewayVaults(ctx, client, gateway.Ref, gateway.DisplayName, gateway.Ref)
		if err != nil {
			return nil, err
		}
		vaults = append(vaults, gatewayVaults...)
	}

	vaults = filterByNameOrID(vaults, filter, func(r declresources.AIGatewayVaultResource) (string, string) {
		return r.Name(), r.Ref
	})
	slices.SortFunc(vaults, func(a, b declresources.AIGatewayVaultResource) int {
		if a.AIGateway == b.AIGateway {
			return cmp.Compare(a.Name(), b.Name())
		}
		return cmp.Compare(a.AIGateway, b.AIGateway)
	})

	return vaults, nil
}

func collectDeclarativeAIGatewayDataPlaneCertificates(
	ctx context.Context,
	client *declstate.Client,
	aiGatewayClient helpers.AIGatewayAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.AIGatewayDataPlaneCertificateResource, error) {
	if client == nil {
		return nil, fmt.Errorf("AI Gateway data plane certificates API client is not configured")
	}

	gateways, err := collectDeclarativeAIGateways(ctx, aiGatewayClient, requestPageSize, filterOptions{})
	if err != nil {
		return nil, err
	}

	var certs []declresources.AIGatewayDataPlaneCertificateResource
	for _, gateway := range gateways {
		gatewayCerts, err := buildAIGatewayDataPlaneCertificates(
			ctx,
			nil,
			client,
			gateway.Ref,
			gateway.DisplayName,
			gateway.Ref,
		)
		if err != nil {
			return nil, err
		}
		certs = append(certs, gatewayCerts...)
	}

	certs = filterByNameOrID(certs, filter, func(r declresources.AIGatewayDataPlaneCertificateResource) (string, string) {
		return r.Title, r.Ref
	})
	slices.SortFunc(certs, func(a, b declresources.AIGatewayDataPlaneCertificateResource) int {
		if a.AIGateway == b.AIGateway {
			return cmp.Compare(a.Title, b.Title)
		}
		return cmp.Compare(a.AIGateway, b.AIGateway)
	})

	return certs, nil
}

func collectDeclarativeOrganizationTeams(
	ctx context.Context,
	teamClient helpers.OrganizationTeamAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.OrganizationTeamResource, error) {
	if teamClient == nil {
		return nil, fmt.Errorf("organization team client is not configured")
	}

	var results []declresources.OrganizationTeamResource

	err := processPaginatedRequests(func(pageNumber int64) (bool, error) {
		req := kkOps.ListTeamsRequest{
			PageSize:   &requestPageSize,
			PageNumber: &pageNumber,
		}

		if filter.name != "" {
			op, val := parseFilterName(filter.name)
			nameFilter := &kkComps.LegacyStringFieldFilter{}
			if op == filterOpContains {
				nameFilter.Contains = &val
			} else {
				nameFilter.Eq = &val
			}
			req.Filter = &kkOps.ListTeamsQueryParamFilter{Name: nameFilter}
		}

		resp, err := teamClient.ListOrganizationTeams(ctx, req)
		if err != nil {
			return false, fmt.Errorf("failed to list teams: %w", err)
		}

		if resp == nil || resp.TeamCollection == nil || len(resp.TeamCollection.Data) == 0 {
			return false, nil
		}

		for _, team := range resp.TeamCollection.Data {
			if team.SystemTeam != nil && *team.SystemTeam {
				// skip system teams from declarative dump
				// these can't be updated by users anyway
				continue
			}
			results = append(results, mapOrganizationTeamToDeclarativeResource(team))
		}

		params := paginationParams{
			pageSize:   requestPageSize,
			pageNumber: pageNumber,
			totalItems: resp.TeamCollection.Meta.Page.Total,
		}

		return params.hasMorePages(), nil
	})
	if err != nil {
		return nil, err
	}

	// Client-side ID filtering (not supported server-side for teams)
	if filter.id != "" {
		results = filterByNameOrID(results, filter, func(r declresources.OrganizationTeamResource) (string, string) {
			return r.Name, r.Ref
		})
	}

	slices.SortFunc(results, func(a, b declresources.OrganizationTeamResource) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return results, nil
}

func mapPortalToDeclarativeResource(portal kkComps.ListPortalsResponsePortal) declresources.PortalResource {
	result := declresources.PortalResource{
		BaseResource: declresources.BaseResource{Ref: portal.GetID()},
		CreatePortal: kkComps.CreatePortal{
			Name: portal.GetName(),
		},
	}

	if displayName := portal.GetDisplayName(); displayName != "" {
		result.DisplayName = &displayName
	}

	result.Description = portal.GetDescription()

	if authEnabled := portal.GetAuthenticationEnabled(); authEnabled != nil {
		result.AuthenticationEnabled = authEnabled
	}

	if rbacEnabled := portal.GetRbacEnabled(); rbacEnabled != nil {
		result.RbacEnabled = rbacEnabled
	}

	if visibility := portal.GetDefaultAPIVisibility(); visibility != "" {
		apiVisibility := kkComps.DefaultAPIVisibility(visibility)
		result.DefaultAPIVisibility = &apiVisibility
	}

	if visibility := portal.GetDefaultPageVisibility(); visibility != "" {
		pageVisibility := kkComps.DefaultPageVisibility(visibility)
		result.DefaultPageVisibility = &pageVisibility
	}

	result.DefaultApplicationAuthStrategyID = portal.GetDefaultApplicationAuthStrategyID()

	if developers := portal.GetAutoApproveDevelopers(); developers != nil {
		result.AutoApproveDevelopers = developers
	}

	if applications := portal.GetAutoApproveApplications(); applications != nil {
		result.AutoApproveApplications = applications
	}

	if userLabels := decllabels.GetUserLabels(portal.GetLabels()); len(userLabels) > 0 {
		result.Labels = decllabels.DenormalizeLabels(userLabels)
	}
	result.Kongctl = kongctlMetaFromLabels(portal.GetLabels())

	return result
}

func mapPortalResponseToDeclarativeResource(portal kkComps.PortalResponse) declresources.PortalResource {
	listPortal := kkComps.ListPortalsResponsePortal{
		ID:                               portal.GetID(),
		Name:                             portal.GetName(),
		DisplayName:                      portal.GetDisplayName(),
		Description:                      portal.GetDescription(),
		AuthenticationEnabled:            portal.GetAuthenticationEnabled(),
		RbacEnabled:                      portal.GetRbacEnabled(),
		DefaultApplicationAuthStrategyID: portal.GetDefaultApplicationAuthStrategyID(),
		AutoApproveDevelopers:            portal.GetAutoApproveDevelopers(),
		AutoApproveApplications:          portal.GetAutoApproveApplications(),
		Labels:                           portal.GetLabels(),
	}

	if visibility := portal.GetDefaultAPIVisibility(); visibility != "" {
		listPortal.DefaultAPIVisibility = kkComps.ListPortalsResponseDefaultAPIVisibility(visibility)
	}

	if visibility := portal.GetDefaultPageVisibility(); visibility != "" {
		listPortal.DefaultPageVisibility = kkComps.ListPortalsResponseDefaultPageVisibility(visibility)
	}

	return mapPortalToDeclarativeResource(listPortal)
}

func mapAPIToDeclarativeResource(api kkComps.APIResponseSchema) declresources.APIResource {
	result := declresources.APIResource{
		BaseResource: declresources.BaseResource{Ref: api.ID},
		CreateAPIRequest: kkComps.CreateAPIRequest{
			Name:        api.Name,
			Description: api.Description,
			Version:     api.Version,
			Slug:        api.Slug,
			Attributes:  api.Attributes,
		},
	}

	if labels := decllabels.GetUserLabels(api.Labels); len(labels) > 0 {
		result.Labels = labels
	}
	result.Kongctl = kongctlMetaFromLabels(api.Labels)

	normalizeAPIResource(&result)

	return result
}

func mapDashboardToDeclarativeResource(dashboard kkComps.DashboardResponse) declresources.DashboardResource {
	result := declresources.DashboardResource{
		BaseResource: declresources.BaseResource{Ref: getString(dashboard.ID)},
		Name:         dashboard.Name,
		Definition:   dashboard.Definition,
	}

	if result.Ref == "" {
		result.Ref = dashboard.Name
	}

	if labels := decllabels.GetUserLabels(dashboard.Labels); len(labels) > 0 {
		result.Labels = labels
	}
	result.Kongctl = kongctlMetaFromLabels(dashboard.Labels)

	return result
}

func mapEventGatewayToDeclarativeResource(egw kkComps.EventGatewayInfo) declresources.EventGatewayControlPlaneResource {
	var minRuntimeVersion *string
	if egw.MinRuntimeVersion != "" {
		minRuntimeVersion = &egw.MinRuntimeVersion
	}

	result := declresources.EventGatewayControlPlaneResource{
		BaseResource: declresources.BaseResource{Ref: egw.ID},
		CreateGatewayRequest: kkComps.CreateGatewayRequest{
			Name:              egw.Name,
			Description:       egw.Description,
			MinRuntimeVersion: minRuntimeVersion,
		},
	}

	if labels := decllabels.GetUserLabels(egw.Labels); len(labels) > 0 {
		result.Labels = labels
	}
	result.Kongctl = kongctlMetaFromLabels(egw.Labels)

	return result
}

func mapAIGatewayToDeclarativeResource(gateway kkComps.AIGateway) declresources.AIGatewayResource {
	result := declresources.AIGatewayResource{
		BaseResource: declresources.BaseResource{Ref: gateway.ID},
		CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
			Name:        gateway.Name,
			DisplayName: gateway.DisplayName,
			Description: gateway.Description,
			ProxyUrls:   gateway.ProxyUrls,
		},
	}

	if result.Ref == "" {
		result.Ref = gateway.DisplayName
	}

	if labels := decllabels.GetUserLabels(gateway.Labels); len(labels) > 0 {
		result.Labels = labels
	}

	if ns := strings.TrimSpace(gateway.Labels[decllabels.NamespaceKey]); ns != "" {
		result.Kongctl = &declresources.KongctlMeta{Namespace: stringPointer(ns)}
	}
	if gateway.Labels[decllabels.ProtectedKey] == decllabels.TrueValue {
		if result.Kongctl == nil {
			result.Kongctl = &declresources.KongctlMeta{}
		}
		protected := true
		result.Kongctl.Protected = &protected
	}

	return result
}

func mapOrganizationTeamToDeclarativeResource(team kkComps.Team) declresources.OrganizationTeamResource {
	result := declresources.OrganizationTeamResource{
		BaseResource: declresources.BaseResource{Ref: getString(team.ID)},
		CreateTeam: kkComps.CreateTeam{
			Name:        getString(team.Name),
			Description: team.Description,
		},
	}

	if labels := decllabels.GetUserLabels(team.Labels); len(labels) > 0 {
		result.Labels = labels
	}

	result.Kongctl = kongctlMetaFromLabels(team.Labels)

	return result
}

func kongctlMetaFromLabels(labels map[string]string) *declresources.KongctlMeta {
	namespace := strings.TrimSpace(labels[decllabels.NamespaceKey])
	protected := labels[decllabels.ProtectedKey] == decllabels.TrueValue
	if namespace == "" && !protected {
		return nil
	}

	meta := &declresources.KongctlMeta{}
	if namespace != "" {
		meta.Namespace = stringPointer(namespace)
	}
	if protected {
		value := true
		meta.Protected = &value
	}
	return meta
}

func normalizeAPIResource(api *declresources.APIResource) {
	if api.Attributes == nil {
		return
	}

	if isEmptyValue(api.Attributes) {
		api.Attributes = nil
	}
}

func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}

	val := reflect.ValueOf(v)

	switch val.Kind() {
	case reflect.Pointer, reflect.Interface:
		if val.IsNil() {
			return true
		}
		return isEmptyValue(val.Elem().Interface())
	case reflect.Map, reflect.Slice, reflect.Array, reflect.Chan, reflect.String:
		return val.Len() == 0
	case reflect.Struct:
		zero := reflect.Zero(val.Type())
		return reflect.DeepEqual(v, zero.Interface())
	case reflect.Invalid,
		reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128,
		reflect.Func,
		reflect.UnsafePointer:
		return false
	}

	return false
}

func collectDeclarativeAuthStrategies(
	ctx context.Context,
	api helpers.AppAuthStrategiesAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.ApplicationAuthStrategyResource, error) {
	if api == nil {
		return nil, fmt.Errorf("application auth strategies API is not configured")
	}

	var results []declresources.ApplicationAuthStrategyResource

	err := processPaginatedRequests(func(pageNumber int64) (bool, error) {
		req := kkOps.ListAppAuthStrategiesRequest{
			PageSize:   &requestPageSize,
			PageNumber: &pageNumber,
		}

		if filter.name != "" {
			req.Filter = &kkOps.ListAppAuthStrategiesQueryParamFilter{Name: buildStringFieldFilter(filter.name)}
		}

		resp, err := api.ListAppAuthStrategies(ctx, req)
		if err != nil {
			return false, fmt.Errorf("failed to list application auth strategies: %w", err)
		}

		if resp == nil || resp.ListAppAuthStrategiesResponse == nil ||
			len(resp.ListAppAuthStrategiesResponse.Data) == 0 {
			return false, nil
		}

		for _, strategy := range resp.ListAppAuthStrategiesResponse.Data {
			mapped, mapErr := mapAuthStrategyToDeclarativeResource(strategy)
			if mapErr != nil {
				return false, mapErr
			}
			results = append(results, mapped)
		}

		params := paginationParams{
			pageSize:   requestPageSize,
			pageNumber: pageNumber,
			totalItems: resp.ListAppAuthStrategiesResponse.Meta.Page.Total,
		}
		return params.hasMorePages(), nil
	})
	if err != nil {
		return nil, err
	}

	// Client-side ID filtering (not supported server-side for auth strategies)
	if filter.id != "" {
		results = filterByNameOrID(
			results,
			filter,
			func(r declresources.ApplicationAuthStrategyResource) (string, string) {
				return r.GetMoniker(), r.Ref
			},
		)
	}

	slices.SortFunc(results, func(a, b declresources.ApplicationAuthStrategyResource) int {
		return cmp.Compare(a.GetMoniker(), b.GetMoniker())
	})

	return results, nil
}

func mapAuthStrategyToDeclarativeResource(
	strategy kkComps.AppAuthStrategy,
) (declresources.ApplicationAuthStrategyResource, error) {
	switch strategy.Type {
	case kkComps.AppAuthStrategyTypeKeyAuth:
		return mapKeyAuthStrategyToDeclarativeResource(strategy)
	case kkComps.AppAuthStrategyTypeOpenidConnect:
		return mapOIDCStrategyToDeclarativeResource(strategy)
	default:
		return declresources.ApplicationAuthStrategyResource{},
			fmt.Errorf("unsupported application auth strategy type: %s", strategy.Type)
	}
}

func mapKeyAuthStrategyToDeclarativeResource(
	strategy kkComps.AppAuthStrategy,
) (declresources.ApplicationAuthStrategyResource, error) {
	resp := strategy.AppAuthStrategyKeyAuthResponseAppAuthStrategyKeyAuthResponse
	if resp == nil {
		return declresources.ApplicationAuthStrategyResource{},
			fmt.Errorf("missing key auth strategy payload")
	}

	req := kkComps.AppAuthStrategyKeyAuthRequest{
		Name:        resp.Name,
		DisplayName: resp.DisplayName,
		Configs: kkComps.AppAuthStrategyKeyAuthRequestConfigs{
			KeyAuth: resp.Configs.GetKeyAuth(),
		},
		Labels: decllabels.GetUserLabels(resp.Labels),
	}

	resource := declresources.ApplicationAuthStrategyResource{
		BaseResource: declresources.BaseResource{
			Ref:     buildAuthStrategyRef(resp.ID, resp.Name),
			Kongctl: kongctlMetaFromLabels(resp.Labels),
		},
		CreateAppAuthStrategyRequest: kkComps.CreateCreateAppAuthStrategyRequestKeyAuth(req),
	}

	return resource, nil
}

func mapOIDCStrategyToDeclarativeResource(
	strategy kkComps.AppAuthStrategy,
) (declresources.ApplicationAuthStrategyResource, error) {
	resp := strategy.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyOpenIDConnectResponse
	if resp == nil {
		return declresources.ApplicationAuthStrategyResource{},
			fmt.Errorf("missing openid_connect strategy payload")
	}

	req := kkComps.AppAuthStrategyOpenIDConnectRequest{
		Name:        resp.Name,
		DisplayName: resp.DisplayName,
		Configs: kkComps.AppAuthStrategyOpenIDConnectRequestConfigs{
			OpenidConnect: resp.Configs.GetOpenidConnect(),
		},
		Labels: decllabels.GetUserLabels(resp.Labels),
	}

	if provider := resp.DcrProvider; provider != nil {
		providerID := provider.ID
		if strings.TrimSpace(providerID) != "" {
			req.DcrProviderID = &providerID
		}
	}

	resource := declresources.ApplicationAuthStrategyResource{
		BaseResource: declresources.BaseResource{
			Ref:     buildAuthStrategyRef(resp.ID, resp.Name),
			Kongctl: kongctlMetaFromLabels(resp.Labels),
		},
		CreateAppAuthStrategyRequest: kkComps.CreateCreateAppAuthStrategyRequestOpenidConnect(req),
	}

	return resource, nil
}

func buildAuthStrategyRef(id, name string) string {
	if trimmed := strings.TrimSpace(id); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(name)
}

func buildDCRProviderRef(id, name string) string {
	if trimmed := strings.TrimSpace(id); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(name)
}

func collectDeclarativeDCRProviders(
	ctx context.Context,
	api helpers.DCRProvidersAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.DCRProviderResource, error) {
	if api == nil {
		return nil, fmt.Errorf("DCR providers API is not configured")
	}

	var results []declresources.DCRProviderResource

	err := processPaginatedRequests(func(pageNumber int64) (bool, error) {
		req := kkOps.ListDcrProvidersRequest{
			PageSize:   &requestPageSize,
			PageNumber: &pageNumber,
		}

		resp, err := api.ListDcrProviderPayloads(ctx, req)
		if err != nil {
			return false, fmt.Errorf("failed to list DCR providers: %w", err)
		}

		if resp == nil || len(resp.Data) == 0 {
			return false, nil
		}

		for _, provider := range resp.Data {
			mapped, mapErr := mapDCRProviderToDeclarativeResource(provider)
			if mapErr != nil {
				return false, mapErr
			}
			results = append(results, mapped)
		}

		params := paginationParams{
			pageSize:   requestPageSize,
			pageNumber: pageNumber,
			totalItems: resp.Total,
		}
		return params.hasMorePages(), nil
	})
	if err != nil {
		return nil, err
	}

	results = filterByNameOrID(results, filter, func(r declresources.DCRProviderResource) (string, string) {
		return r.Name, r.Ref
	})

	slices.SortFunc(results, func(a, b declresources.DCRProviderResource) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return results, nil
}

func mapDCRProviderToDeclarativeResource(data any) (declresources.DCRProviderResource, error) {
	provider, err := helpers.NormalizeDCRProviderPayload(data)
	if err != nil {
		return declresources.DCRProviderResource{}, err
	}

	resource := declresources.DCRProviderResource{
		BaseResource: declresources.BaseResource{
			Ref:     buildDCRProviderRef(provider.ID, provider.Name),
			Kongctl: kongctlMetaFromLabels(provider.Labels),
		},
		Name:         provider.Name,
		ProviderType: provider.ProviderType,
		Issuer:       provider.Issuer,
		DCRConfig:    provider.DCRConfig,
		Labels:       decllabels.GetUserLabels(provider.Labels),
	}

	if resource.DCRConfig == nil {
		resource.DCRConfig = map[string]any{}
	}
	if provider.DisplayNameSet {
		resource.DisplayName = provider.DisplayName
	}

	return resource, nil
}

func collectDeclarativeControlPlanes(
	ctx context.Context,
	api helpers.ControlPlaneAPI,
	groupsAPI helpers.ControlPlaneGroupsAPI,
	requestPageSize int64,
	filter filterOptions,
) ([]declresources.ControlPlaneResource, error) {
	if api == nil {
		return nil, fmt.Errorf("control plane API is not configured")
	}

	var results []declresources.ControlPlaneResource

	err := processPaginatedRequests(func(pageNumber int64) (bool, error) {
		req := kkOps.ListControlPlanesRequest{
			PageSize:   &requestPageSize,
			PageNumber: &pageNumber,
		}

		if filter.name != "" {
			op, val := parseFilterName(filter.name)
			nameFilter := &kkComps.Name{}
			if op == filterOpContains {
				nameFilter.Contains = &val
			} else {
				nameFilter.Eq = &val
			}
			req.Filter = &kkComps.ControlPlaneFilterParameters{Name: nameFilter}
		} else if filter.id != "" {
			req.Filter = &kkComps.ControlPlaneFilterParameters{ID: &kkComps.ID{Eq: &filter.id}}
		}

		resp, err := api.ListControlPlanes(ctx, req)
		if err != nil {
			return false, fmt.Errorf("failed to list control planes: %w", err)
		}

		if resp == nil || resp.ListControlPlanesResponse == nil || len(resp.ListControlPlanesResponse.Data) == 0 {
			return false, nil
		}

		for _, cp := range resp.ListControlPlanesResponse.Data {
			memberIDs := []string{}
			if groupsAPI != nil &&
				cp.Config.ClusterType == kkComps.ControlPlaneClusterTypeClusterTypeControlPlaneGroup {
				ids, err := fetchControlPlaneGroupMembers(ctx, groupsAPI, cp.ID)
				if err != nil {
					return false, fmt.Errorf("failed to list group memberships for control plane %s: %w", cp.Name, err)
				}
				memberIDs = normalizers.NormalizeMemberIDs(ids)
			}

			mapped := mapControlPlaneToDeclarativeResource(cp, memberIDs)
			results = append(results, mapped)
		}

		params := paginationParams{
			pageSize:   requestPageSize,
			pageNumber: pageNumber,
			totalItems: resp.ListControlPlanesResponse.Meta.Page.Total,
		}
		return params.hasMorePages(), nil
	})
	if err != nil {
		return nil, err
	}

	slices.SortFunc(results, func(a, b declresources.ControlPlaneResource) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return results, nil
}

func mapControlPlaneToDeclarativeResource(
	cp kkComps.ControlPlane,
	memberIDs []string,
) declresources.ControlPlaneResource {
	mapped := declresources.ControlPlaneResource{
		BaseResource: declresources.BaseResource{Ref: cp.ID},
		CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{
			Name: cp.Name,
		},
	}
	if strings.TrimSpace(cp.Description) != "" {
		mapped.Description = &cp.Description
	}

	config := cp.Config

	if string(config.ClusterType) != "" {
		clusterType := kkComps.CreateControlPlaneRequestClusterType(string(config.ClusterType))
		mapped.ClusterType = &clusterType
	}

	if string(config.AuthType) != "" {
		authType := kkComps.AuthType(string(config.AuthType))
		mapped.AuthType = &authType
	}

	if config.CloudGateway {
		cloudGateway := config.CloudGateway
		mapped.CloudGateway = &cloudGateway
	}

	if len(config.ProxyUrls) > 0 {
		mapped.ProxyUrls = config.ProxyUrls
	}

	userLabels := decllabels.GetUserLabels(cp.Labels)
	if len(userLabels) > 0 {
		mapped.Labels = userLabels
	}

	mapped.Kongctl = kongctlMetaFromLabels(cp.Labels)

	if len(memberIDs) > 0 && cp.Config.ClusterType == kkComps.ControlPlaneClusterTypeClusterTypeControlPlaneGroup {
		mapped.Members = make([]declresources.ControlPlaneGroupMember, 0, len(memberIDs))
		for _, id := range memberIDs {
			mapped.Members = append(mapped.Members, declresources.ControlPlaneGroupMember{ID: id})
		}
	}

	return mapped
}

func fetchControlPlaneGroupMembers(
	ctx context.Context,
	api helpers.ControlPlaneGroupsAPI,
	controlPlaneID string,
) ([]string, error) {
	const defaultPageSize int64 = 100
	pageSize := defaultPageSize

	var (
		members   []string
		pageAfter *string
	)

	for {
		req := kkOps.GetControlPlanesIDGroupMembershipsRequest{
			ControlPlaneID: controlPlaneID,
			PageSize:       &pageSize,
		}

		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		resp, err := api.GetControlPlanesIDGroupMemberships(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list control plane group memberships: %w", err)
		}

		if resp == nil || resp.GetListGroupMemberships() == nil {
			break
		}

		for _, member := range resp.GetListGroupMemberships().GetData() {
			if member.ID != "" {
				members = append(members, member.ID)
			}
		}

		nextCursor := pagination.ExtractPageAfterCursor(resp.GetListGroupMemberships().GetMeta().Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return members, nil
}

func stringPointer(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func getString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
