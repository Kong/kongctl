package dump

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	konnectCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	decllabels "github.com/kong/kongctl/internal/declarative/labels"
	declresources "github.com/kong/kongctl/internal/declarative/resources"
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
}

var declarativeAllowedResources = map[string]struct{}{
	"portals":                     {},
	"apis":                        {},
	"application_auth_strategies": {},
	"control_planes":              {},
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
			if err := ensureNonNegativePageSize(helper); err != nil {
				return err
			}
			return runDeclarativeDump(helper, *opts)
		},
	}

	cmd.Flags().String("resources", "",
		"Comma separated list of resource types to dump (portals, apis, application_auth_strategies, control_planes).")
	_ = cmd.MarkFlagRequired("resources")

	cmd.Flags().BoolVar(&opts.includeChildResources, "include-child-resources", false,
		"Include child resources in the dump.")

	cmd.Flags().StringVar(&opts.outputFile, "output-file", "",
		"File to write the output to. If not specified, output is written to stdout.")

	cmd.Flags().StringVar(&opts.defaultNamespace, "default-namespace", "",
		"Default namespace to include in declarative output (_defaults.kongctl.namespace).")

	cmd.Flags().Int(
		konnectCommon.RequestPageSizeFlagName,
		konnectCommon.DefaultRequestPageSize,
		fmt.Sprintf(`Max number of results to include per response page.
- Config path: [ %s ]`, konnectCommon.RequestPageSizeConfigPath))

	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		helper := cmdpkg.BuildHelper(c, args)
		cfg, err := helper.GetConfig()
		if err != nil {
			return err
		}
		return cfg.BindFlag(konnectCommon.RequestPageSizeConfigPath,
			c.Flags().Lookup(konnectCommon.RequestPageSizeFlagName))
	}

	return cmd
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

	if opts.includeChildResources {
		logger.Warn("include-child-resources is not yet supported for declarative dump; child resources are omitted")
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
		konnectCommon.DefaultRequestPageSize))

	for _, resource := range opts.resources {
		switch resource {
		case "portals":
			portals, err := collectDeclarativePortals(ctx, sdk.GetPortalAPI(), requestPageSize)
			if err != nil {
				return err
			}
			resourceSet.Portals = append(resourceSet.Portals, portals...)
		case "apis":
			apis, err := collectDeclarativeAPIs(ctx, sdk.GetAPIAPI(), requestPageSize)
			if err != nil {
				return err
			}
			resourceSet.APIs = append(resourceSet.APIs, apis...)
		case "application_auth_strategies":
			authStrategies, err := collectDeclarativeAuthStrategies(ctx, sdk.GetAppAuthStrategiesAPI(), requestPageSize)
			if err != nil {
				return err
			}
			resourceSet.ApplicationAuthStrategies = append(resourceSet.ApplicationAuthStrategies, authStrategies...)
		case "control_planes":
			controlPlanes, err := collectDeclarativeControlPlanes(
				ctx,
				sdk.GetControlPlaneAPI(),
				sdk.GetControlPlaneGroupsAPI(),
				requestPageSize,
			)
			if err != nil {
				return err
			}
			resourceSet.ControlPlanes = append(resourceSet.ControlPlanes, controlPlanes...)
		}
	}

	resourceSet.DefaultNamespace = opts.defaultNamespace

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
) ([]declresources.PortalResource, error) {
	if portalAPI == nil {
		return nil, fmt.Errorf("portal API client is not configured")
	}

	var results []declresources.PortalResource

	err := processPaginatedRequests(func(pageNumber int64) (bool, error) {
		req := kkOps.ListPortalsRequest{
			PageSize:   Int64(requestPageSize),
			PageNumber: Int64(pageNumber),
		}

		resp, err := portalAPI.ListPortals(ctx, req)
		if err != nil {
			return false, fmt.Errorf("failed to list portals: %w", err)
		}

		if resp.ListPortalsResponse == nil || len(resp.ListPortalsResponse.Data) == 0 {
			return false, nil
		}

		for _, portal := range resp.ListPortalsResponse.Data {
			results = append(results, mapPortalToDeclarativeResource(portal))
		}

		return true, nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results, nil
}

func collectDeclarativeAPIs(
	ctx context.Context,
	apiClient helpers.APIAPI,
	requestPageSize int64,
) ([]declresources.APIResource, error) {
	if apiClient == nil {
		return nil, fmt.Errorf("API client is not configured")
	}

	var results []declresources.APIResource

	err := processPaginatedRequests(func(pageNumber int64) (bool, error) {
		req := kkOps.ListApisRequest{
			PageSize:   Int64(requestPageSize),
			PageNumber: Int64(pageNumber),
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

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results, nil
}

func mapPortalToDeclarativeResource(portal kkComps.Portal) declresources.PortalResource {
	result := declresources.PortalResource{
		CreatePortal: kkComps.CreatePortal{
			Name: portal.Name,
		},
		Ref: portal.ID,
	}

	if portal.DisplayName != "" {
		displayName := portal.DisplayName
		result.DisplayName = &displayName
	}

	result.Description = portal.Description

	authEnabled := portal.AuthenticationEnabled
	result.AuthenticationEnabled = boolPtr(authEnabled)

	rbacEnabled := portal.RbacEnabled
	result.RbacEnabled = boolPtr(rbacEnabled)

	if portal.DefaultAPIVisibility != "" {
		visibility := kkComps.DefaultAPIVisibility(portal.DefaultAPIVisibility)
		result.DefaultAPIVisibility = &visibility
	}

	if portal.DefaultPageVisibility != "" {
		visibility := kkComps.DefaultPageVisibility(portal.DefaultPageVisibility)
		result.DefaultPageVisibility = &visibility
	}

	result.DefaultApplicationAuthStrategyID = portal.DefaultApplicationAuthStrategyID

	autoApproveDevelopers := portal.AutoApproveDevelopers
	result.AutoApproveDevelopers = boolPtr(autoApproveDevelopers)

	autoApproveApplications := portal.AutoApproveApplications
	result.AutoApproveApplications = boolPtr(autoApproveApplications)

	if userLabels := decllabels.GetUserLabels(portal.Labels); len(userLabels) > 0 {
		result.Labels = decllabels.DenormalizeLabels(userLabels)
	}

	return result
}

func mapAPIToDeclarativeResource(api kkComps.APIResponseSchema) declresources.APIResource {
	result := declresources.APIResource{
		CreateAPIRequest: kkComps.CreateAPIRequest{
			Name:        api.Name,
			Description: api.Description,
			Version:     api.Version,
			Slug:        api.Slug,
			Attributes:  api.Attributes,
		},
		Ref: api.ID,
	}

	if labels := decllabels.GetUserLabels(api.Labels); len(labels) > 0 {
		result.Labels = labels
	}

	normalizeAPIResource(&result)

	return result
}

func boolPtr(v bool) *bool {
	return &v
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

	switch val.Kind() { //nolint:exhaustive
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
	default:
		return false
	}
}

func collectDeclarativeAuthStrategies(
	ctx context.Context,
	api helpers.AppAuthStrategiesAPI,
	requestPageSize int64,
) ([]declresources.ApplicationAuthStrategyResource, error) {
	if api == nil {
		return nil, fmt.Errorf("application auth strategies API is not configured")
	}

	var results []declresources.ApplicationAuthStrategyResource

	err := processPaginatedRequests(func(pageNumber int64) (bool, error) {
		req := kkOps.ListAppAuthStrategiesRequest{
			PageSize:   Int64(requestPageSize),
			PageNumber: Int64(pageNumber),
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

	sort.Slice(results, func(i, j int) bool {
		return results[i].GetMoniker() < results[j].GetMoniker()
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
		CreateAppAuthStrategyRequest: kkComps.CreateCreateAppAuthStrategyRequestKeyAuth(req),
		Ref:                          buildAuthStrategyRef(resp.ID, resp.Name),
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
		CreateAppAuthStrategyRequest: kkComps.CreateCreateAppAuthStrategyRequestOpenidConnect(req),
		Ref:                          buildAuthStrategyRef(resp.ID, resp.Name),
	}

	return resource, nil
}

func buildAuthStrategyRef(id, name string) string {
	if trimmed := strings.TrimSpace(id); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(name)
}

func collectDeclarativeControlPlanes(
	ctx context.Context,
	api helpers.ControlPlaneAPI,
	groupsAPI helpers.ControlPlaneGroupsAPI,
	requestPageSize int64,
) ([]declresources.ControlPlaneResource, error) {
	if api == nil {
		return nil, fmt.Errorf("control plane API is not configured")
	}

	var results []declresources.ControlPlaneResource

	err := processPaginatedRequests(func(pageNumber int64) (bool, error) {
		req := kkOps.ListControlPlanesRequest{
			PageSize:   Int64(requestPageSize),
			PageNumber: Int64(pageNumber),
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

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results, nil
}

func mapControlPlaneToDeclarativeResource(
	cp kkComps.ControlPlane,
	memberIDs []string,
) declresources.ControlPlaneResource {
	mapped := declresources.ControlPlaneResource{
		CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{
			Name:        cp.Name,
			Description: cp.Description,
		},
		Ref: cp.ID,
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

	if ns := strings.TrimSpace(cp.Labels[decllabels.NamespaceKey]); ns != "" {
		mapped.Kongctl = &declresources.KongctlMeta{Namespace: stringPointer(ns)}
	}
	if cp.Labels[decllabels.ProtectedKey] == decllabels.TrueValue {
		if mapped.Kongctl == nil {
			mapped.Kongctl = &declresources.KongctlMeta{}
		}
		protected := true
		mapped.Kongctl.Protected = &protected
	}

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
			ID:       controlPlaneID,
			PageSize: &pageSize,
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
