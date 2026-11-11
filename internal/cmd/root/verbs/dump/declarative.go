package dump

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	konnectCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	declexport "github.com/kong/kongctl/internal/declarative/export"
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
	skipDefaults          bool
	filter                filterOptions
}

// DeclarativeDumpOptions configures a declarative resource dump outside Cobra flag parsing.
type DeclarativeDumpOptions struct {
	Resources             []string
	OutputFile            string
	DefaultNamespace      string
	IncludeChildResources bool
	SkipDefaults          bool
	FilterName            string
	FilterID              string
}

// Retain the validation entry point used by existing callers and tests.
var declarativeAllowedResources = func() map[string]struct{} {
	allowed := make(map[string]struct{}, len(declarativeCollectors))
	for selector := range declarativeCollectors {
		allowed[selector] = struct{}{}
	}
	return allowed
}()

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
		"Comma separated list of resource types to dump ("+
			strings.Join(slices.Sorted(maps.Keys(declarativeAllowedResources)), ", ")+").")
	_ = cmd.MarkFlagRequired("resources")

	cmd.Flags().BoolVar(&opts.includeChildResources, "include-child-resources", false,
		"Include child resources in the dump.")
	cmd.Flags().BoolVar(&opts.skipDefaults, "skip-defaults", false,
		"Omit fields whose values match defaults declared by the Konnect API.")

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
		skipDefaults:          opts.SkipDefaults,
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
	if opts.includeChildResources {
		stateClient = declstate.NewClient(declstate.ClientConfig{
			PortalAPI:                           sdk.GetPortalAPI(),
			APIAPI:                              sdk.GetAPIAPI(),
			AppAuthAPI:                          sdk.GetAppAuthStrategiesAPI(),
			IdentityDirectoriesAPI:              sdk.GetIdentityDirectoriesAPI(),
			IdentityVaultKeysAPI:                sdk.GetIdentityVaultKeysAPI(),
			IdentityVaultProvidersAPI:           sdk.GetIdentityVaultProvidersAPI(),
			IdentityStaticVaultCredentialsAPI:   sdk.GetIdentityStaticVaultCredentialsAPI(),
			IdentityTrustedIDPsAPI:              sdk.GetIdentityTrustedIDPsAPI(),
			IdentityAuthServersAPI:              sdk.GetIdentityAuthServersAPI(),
			IdentityAuthServerScopesAPI:         sdk.GetIdentityAuthServerScopesAPI(),
			IdentityAuthServerClaimsAPI:         sdk.GetIdentityAuthServerClaimsAPI(),
			IdentityAuthServerClientsAPI:        sdk.GetIdentityAuthServerClientsReader(),
			IdentityBasicAuthAPI:                sdk.GetIdentityBasicAuthAPI(),
			IdentityPrincipalKeysReader:         sdk.GetIdentityPrincipalKeysReader(),
			IdentityPrincipalKeysAPI:            sdk.GetIdentityPrincipalKeysAPI(),
			IdentityGroupKeysAPI:                sdk.GetIdentityGroupKeysAPI(),
			IdentityGroupsAPI:                   sdk.GetIdentityGroupsAPI(),
			IdentityPrincipalsAPI:               sdk.GetIdentityPrincipalsAPI(),
			IdentityPrincipalIdentitiesAPI:      sdk.GetIdentityPrincipalIdentitiesAPI(),
			DCRProviderAPI:                      sdk.GetDCRProvidersAPI(),
			ControlPlaneAPI:                     sdk.GetControlPlaneAPI(),
			GatewayServiceAPI:                   sdk.GetGatewayServiceAPI(),
			DataPlaneCertificateAPI:             sdk.GetDataPlaneCertificateAPI(),
			ControlPlaneGroupsAPI:               sdk.GetControlPlaneGroupsAPI(),
			CatalogServiceAPI:                   sdk.GetCatalogServicesAPI(),
			AIGatewayAPI:                        sdk.GetAIGatewayAPI(),
			AIGatewayProvidersAPI:               sdk.GetAIGatewayProvidersAPI(),
			AIGatewayAuthStrategiesAPI:          sdk.GetAIGatewayAuthStrategiesAPI(),
			AIGatewayCustomPoliciesAPI:          sdk.GetAIGatewayCustomPoliciesAPI(),
			AIGatewayPoliciesAPI:                sdk.GetAIGatewayPoliciesAPI(),
			AIGatewayAgentsAPI:                  sdk.GetAIGatewayAgentsAPI(),
			AIGatewayConsumersAPI:               sdk.GetAIGatewayConsumersAPI(),
			AIGatewayConsumerGroupsAPI:          sdk.GetAIGatewayConsumerGroupsAPI(),
			AIGatewayModelAPI:                   sdk.GetAIGatewayModelAPI(),
			AIGatewayMCPServersAPI:              sdk.GetAIGatewayMCPServersAPI(),
			AIGatewayConfigStoresAPI:            sdk.GetAIGatewayConfigStoresAPI(),
			AIGatewayVaultsAPI:                  sdk.GetAIGatewayVaultsAPI(),
			AIGatewayDatastoresAPI:              sdk.GetAIGatewayDatastoresAPI(),
			AIGatewayDataPlaneCertificatesAPI:   sdk.GetAIGatewayDataPlaneCertificatesAPI(),
			AIGatewayCertificatesAPI:            sdk.GetAIGatewayCertificatesAPI(),
			AIGatewayCACertificatesAPI:          sdk.GetAIGatewayCACertificatesAPI(),
			AIGatewaySNIsAPI:                    sdk.GetAIGatewaySNIsAPI(),
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
			SystemAccountAPI:                    sdk.GetSystemAccountAPI(),
			SystemAccountRolesAPI:               sdk.GetSystemAccountRolesAPI(),
			SystemAccountMembershipAPI:          sdk.GetSystemAccountTeamMembershipAPI(),
		})
	}

	needsIdentityClient := slices.Contains(opts.resources, resourceIdentityDirectories) ||
		slices.Contains(opts.resources, resourceIdentityAuthServers)
	if stateClient == nil && needsIdentityClient {
		stateClient = declstate.NewClient(
			declstate.ClientConfig{
				IdentityStaticVaultCredentialsAPI: sdk.GetIdentityStaticVaultCredentialsAPI(),

				IdentityDirectoriesAPI:         sdk.GetIdentityDirectoriesAPI(),
				IdentityVaultKeysAPI:           sdk.GetIdentityVaultKeysAPI(),
				IdentityVaultProvidersAPI:      sdk.GetIdentityVaultProvidersAPI(),
				IdentityTrustedIDPsAPI:         sdk.GetIdentityTrustedIDPsAPI(),
				IdentityAuthServersAPI:         sdk.GetIdentityAuthServersAPI(),
				IdentityAuthServerScopesAPI:    sdk.GetIdentityAuthServerScopesAPI(),
				IdentityAuthServerClaimsAPI:    sdk.GetIdentityAuthServerClaimsAPI(),
				IdentityAuthServerClientsAPI:   sdk.GetIdentityAuthServerClientsReader(),
				IdentityBasicAuthAPI:           sdk.GetIdentityBasicAuthAPI(),
				IdentityPrincipalKeysReader:    sdk.GetIdentityPrincipalKeysReader(),
				IdentityPrincipalKeysAPI:       sdk.GetIdentityPrincipalKeysAPI(),
				IdentityGroupKeysAPI:           sdk.GetIdentityGroupKeysAPI(),
				IdentityGroupsAPI:              sdk.GetIdentityGroupsAPI(),
				IdentityPrincipalsAPI:          sdk.GetIdentityPrincipalsAPI(),
				IdentityPrincipalIdentitiesAPI: sdk.GetIdentityPrincipalIdentitiesAPI(),
			},
		)
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

	dumpContext := &declarativeDumpContext{
		sdk: sdk, stateClient: stateClient, logger: logger, pageSize: requestPageSize,
		filter: opts.filter, includeChildren: opts.includeChildResources,
	}
	for _, resource := range opts.resources {
		if collector, ok := declarativeCollectors[resource]; ok {
			if err := collector.collect(ctx, dumpContext, &resourceSet); err != nil {
				return err
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
	if opts.skipDefaults {
		yamlBytes, err = declresources.OmitAPIDefaults(yamlBytes)
		if err != nil {
			return fmt.Errorf("failed to omit API defaults: %w", err)
		}
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
			resource := declexport.MapPortalToDeclarativeResource(portal)
			if portalID := strings.TrimSpace(portal.GetID()); portalID != "" {
				detail, err := portalAPI.GetPortal(ctx, portalID)
				if err != nil {
					return false, fmt.Errorf("failed to get portal %q: %w", portalID, err)
				}
				if detailedPortal := detail.GetPortalResponse(); detailedPortal != nil {
					resource = declexport.MapPortalResponseToDeclarativeResource(*detailedPortal)
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
			results = append(results, declexport.MapAPIToDeclarativeResource(api))
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
			results = append(results, declexport.MapDashboardToDeclarativeResource(dashboard))
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
			req.Filter = &kkComps.EventGatewaysFilter{
				Name: &kkComps.StringFieldContainsFilter{Contains: val},
			}
		}

		res, err := eventGatewayClient.ListEGWControlPlanes(ctx, req)
		if err != nil {
			return nil, err
		}

		for _, egw := range res.ListEventGatewaysResponse.Data {
			allData = append(allData, declexport.MapEventGatewayToDeclarativeResource(egw))
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
			results = append(results, declexport.MapAIGatewayToDeclarativeResource(gateway))
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

		if resp == nil || resp.TeamCollectionResponse == nil || len(resp.TeamCollectionResponse.Data) == 0 {
			return false, nil
		}

		for _, team := range resp.TeamCollectionResponse.Data {
			if team.SystemTeam != nil && *team.SystemTeam {
				// skip system teams from declarative dump
				// these can't be updated by users anyway
				continue
			}
			results = append(results, declexport.MapOrganizationTeamToDeclarativeResource(team))
		}

		params := paginationParams{
			pageSize:   requestPageSize,
			pageNumber: pageNumber,
			totalItems: resp.TeamCollectionResponse.Meta.Page.Total,
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
			mapped, mapErr := declexport.MapAuthStrategyToDeclarativeResource(strategy)
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
			mapped, mapErr := declexport.MapDCRProviderToDeclarativeResource(provider)
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
			nameFilter := &kkComps.ControlPlaneFilterParametersName{}
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
				ids, err := declexport.FetchControlPlaneGroupMembers(ctx, groupsAPI, cp.ID)
				if err != nil {
					return false, fmt.Errorf("failed to list group memberships for control plane %s: %w", cp.Name, err)
				}
				memberIDs = normalizers.NormalizeMemberIDs(ids)
			}

			mapped := declexport.MapControlPlaneToDeclarativeResource(cp, memberIDs)
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

func stringPointer(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
