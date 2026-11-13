package export

import (
	"context"
	"fmt"
	"slices"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/fieldcontracts"
	decllabels "github.com/kong/kongctl/internal/declarative/labels"
	declresources "github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util/pagination"
)

func MapPortalToDeclarativeResource(portal kkComps.ListPortalsResponsePortal) declresources.PortalResource {
	result := declresources.PortalResource{
		BaseResource: declresources.BaseResource{Ref: portal.GetID()},
		CreatePortal: kkComps.CreatePortal{
			Name: portal.GetName(),
		},
	}

	fieldcontracts.ExportPortalFields(portal, &result.CreatePortal)

	result.DefaultApplicationAuthStrategyID = portal.GetDefaultApplicationAuthStrategyID()

	if userLabels := decllabels.GetUserLabels(portal.GetLabels()); len(userLabels) > 0 {
		result.Labels = decllabels.DenormalizeLabels(userLabels)
	}
	result.Kongctl = KongctlMetaFromLabels(portal.GetLabels())

	return result
}

func MapPortalResponseToDeclarativeResource(portal kkComps.PortalResponse) declresources.PortalResource {
	listPortal := kkComps.ListPortalsResponsePortal{
		ID:                               portal.GetID(),
		Name:                             portal.GetName(),
		DefaultApplicationAuthStrategyID: portal.GetDefaultApplicationAuthStrategyID(),
		Labels:                           portal.GetLabels(),
	}

	result := MapPortalToDeclarativeResource(listPortal)
	fieldcontracts.ExportPortalResponseFields(portal, &result.CreatePortal)
	return result
}

func MapAPIToDeclarativeResource(api kkComps.APIResponseSchema) declresources.APIResource {
	result := declresources.APIResource{
		BaseResource:     declresources.BaseResource{Ref: api.ID},
		CreateAPIRequest: kkComps.CreateAPIRequest{Name: api.Name},
	}

	if labels := decllabels.GetUserLabels(api.Labels); len(labels) > 0 {
		result.Labels = labels
	}
	result.Kongctl = KongctlMetaFromLabels(api.Labels)

	fieldcontracts.ExportAPIFields(api, &result.CreateAPIRequest)

	return result
}

func MapDashboardToDeclarativeResource(dashboard kkComps.DashboardResponse) declresources.DashboardResource {
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
	result.Kongctl = KongctlMetaFromLabels(dashboard.Labels)

	return result
}

func MapEventGatewayToDeclarativeResource(egw kkComps.EventGatewayInfo) declresources.EventGatewayControlPlaneResource {
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
	result.Kongctl = KongctlMetaFromLabels(egw.Labels)

	return result
}

func MapAIGatewayToDeclarativeResource(gateway kkComps.AIGateway) declresources.AIGatewayResource {
	var deploymentType *kkComps.CreateAIGatewayRequestDeploymentType
	if gateway.DeploymentType != nil {
		value := kkComps.CreateAIGatewayRequestDeploymentType(*gateway.DeploymentType)
		deploymentType = &value
	}

	result := declresources.AIGatewayResource{
		BaseResource: declresources.BaseResource{Ref: gateway.ID},
		CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
			MinRuntimeVersion:  gateway.MinRuntimeVersion,
			RuntimeAutoUpgrade: gateway.RuntimeAutoUpgrade,
			Name:               gateway.Name,
			DisplayName:        gateway.DisplayName,
			DeploymentType:     deploymentType,
			Description:        gateway.Description,
			ProxyUrls:          gateway.ProxyUrls,
		},
	}

	if result.Ref == "" {
		result.Ref = gateway.DisplayName
	}

	if labels := decllabels.GetUserLabels(gateway.Labels); len(labels) > 0 {
		result.Labels = labels
	}

	result.Kongctl = KongctlMetaFromLabels(gateway.Labels)

	return result
}

func MapOrganizationTeamToDeclarativeResource(team kkComps.TeamResponse) declresources.OrganizationTeamResource {
	result := declresources.OrganizationTeamResource{
		BaseResource: declresources.BaseResource{Ref: team.ID},
		CreateTeam: kkComps.CreateTeam{
			Name:        team.Name,
			Description: team.Description,
		},
	}

	if labels := decllabels.GetUserLabels(team.Labels); len(labels) > 0 {
		result.Labels = labels
	}

	result.Kongctl = KongctlMetaFromLabels(team.Labels)

	return result
}

func KongctlMetaFromLabels(labels map[string]string) *declresources.KongctlMeta {
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

func MapAuthStrategyToDeclarativeResource(
	strategy kkComps.AppAuthStrategy,
) (declresources.ApplicationAuthStrategyResource, error) {
	switch strategy.Type {
	case kkComps.AppAuthStrategyTypeKeyAuth:
		return MapKeyAuthStrategyToDeclarativeResource(strategy)
	case kkComps.AppAuthStrategyTypeOpenidConnect:
		return MapOIDCStrategyToDeclarativeResource(strategy)
	default:
		return declresources.ApplicationAuthStrategyResource{},
			fmt.Errorf("unsupported application auth strategy type: %s", strategy.Type)
	}
}

func MapKeyAuthStrategyToDeclarativeResource(
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
			Ref:     BuildRef(resp.ID, resp.Name),
			Kongctl: KongctlMetaFromLabels(resp.Labels),
		},
		CreateAppAuthStrategyRequest: kkComps.CreateCreateAppAuthStrategyRequestKeyAuth(req),
	}

	return resource, nil
}

func MapOIDCStrategyToDeclarativeResource(
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
			Ref:     BuildRef(resp.ID, resp.Name),
			Kongctl: KongctlMetaFromLabels(resp.Labels),
		},
		CreateAppAuthStrategyRequest: kkComps.CreateCreateAppAuthStrategyRequestOpenidConnect(req),
	}

	return resource, nil
}

func BuildRef(id, name string) string {
	if trimmed := strings.TrimSpace(id); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(name)
}

func MapDCRProviderToDeclarativeResource(data any) (declresources.DCRProviderResource, error) {
	provider, err := helpers.NormalizeDCRProviderPayload(data)
	if err != nil {
		return declresources.DCRProviderResource{}, err
	}

	return MapNormalizedDCRProvider(*provider), nil
}

func MapControlPlaneToDeclarativeResource(
	cp kkComps.ControlPlane,
	memberIDs []string,
) declresources.ControlPlaneResource {
	mapped := declresources.ControlPlaneResource{
		BaseResource: declresources.BaseResource{Ref: cp.ID},
		CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{
			Name: cp.Name,
		},
	}
	fieldcontracts.ExportControlPlaneFields(cp, &mapped.CreateControlPlaneRequest)

	userLabels := decllabels.GetUserLabels(cp.Labels)
	if len(userLabels) > 0 {
		mapped.Labels = userLabels
	}

	mapped.Kongctl = KongctlMetaFromLabels(cp.Labels)

	if len(memberIDs) > 0 && cp.Config.ClusterType == kkComps.ControlPlaneClusterTypeClusterTypeControlPlaneGroup {
		mapped.Members = make([]declresources.ControlPlaneGroupMember, 0, len(memberIDs))
		for _, id := range memberIDs {
			mapped.Members = append(mapped.Members, declresources.ControlPlaneGroupMember{ID: id})
		}
	}

	return mapped
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

// Root projects one adopted parent using the same mappers as dump. It does not
// traverse descendants; related independently owned resources remain untouched.
func Root(payload any) (declresources.Resource, error) {
	switch value := payload.(type) {
	case kkComps.PortalResponse:
		r := MapPortalResponseToDeclarativeResource(value)
		return &r, nil
	case kkComps.APIResponseSchema:
		r := MapAPIToDeclarativeResource(value)
		return &r, nil
	case kkComps.DashboardResponse:
		r := MapDashboardToDeclarativeResource(value)
		return &r, nil
	case kkComps.EventGatewayInfo:
		r := MapEventGatewayToDeclarativeResource(value)
		return &r, nil
	case kkComps.AIGateway:
		r := MapAIGatewayToDeclarativeResource(value)
		return &r, nil
	case kkComps.TeamResponse:
		r := MapOrganizationTeamToDeclarativeResource(value)
		return &r, nil
	case kkComps.ControlPlane:
		if value.Config.ClusterType == kkComps.ControlPlaneClusterTypeClusterTypeControlPlaneGroup {
			return nil, fmt.Errorf("control plane group export requires a declaration with verified memberships")
		}
		r := MapControlPlaneToDeclarativeResource(value, nil)
		return &r, nil
	case kkComps.AppAuthStrategy:
		r, err := MapAuthStrategyToDeclarativeResource(value)
		return &r, err
	case helpers.NormalizedDCRProviderPayload:
		r := MapNormalizedDCRProvider(value)
		return &r, nil
	case kkComps.KongDirectory:
		r := MapIdentityDirectory(value)
		return &r, nil
	case kkComps.AuthServer:
		r := MapIdentityAuthServer(value)
		return &r, nil
	case declresources.Resource:
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported export response %T", payload)
	}
}

func MapIdentityDirectory(value kkComps.KongDirectory) declresources.IdentityDirectoryResource {
	return declresources.IdentityDirectoryResource{
		BaseResource: declresources.BaseResource{
			Ref:     value.ID,
			Kongctl: KongctlMetaFromLabels(declresources.NormalizedChildOwnership(value.ManagedBy)),
		},
		Name: value.Name, Description: &value.Description,
		AllowedControlPlanes: slices.Clone(value.AllowedControlPlanes), AllowAllControlPlanes: value.AllowAllControlPlanes,
		TTLSecs: value.TTLSecs, NegativeTTLSecs: value.NegativeTTLSecs,
		IdentityDirectoryVault: declresources.IdentityDirectoryVault{VaultEnabled: value.VaultEnabled},
		Labels:                 decllabels.GetUserLabels(value.Labels),
	}
}

func MapIdentityAuthServer(value kkComps.AuthServer) declresources.IdentityAuthServerResource {
	r := declresources.IdentityAuthServerResource{
		BaseResource: declresources.BaseResource{Ref: value.ID, Kongctl: KongctlMetaFromLabels(value.Labels)},
		Name:         value.Name, Audience: value.Audience, Description: &value.Description,
		SigningAlgorithm: value.SigningAlgorithm, TrustedOrigins: slices.Clone(value.TrustedOrigins),
		Labels: decllabels.GetUserLabels(value.Labels),
	}
	if r.TrustedOrigins == nil {
		r.TrustedOrigins = []string{}
	}
	if r.Labels == nil {
		r.Labels = map[string]string{}
	}
	if value.Dcr != nil {
		r.DCR = &declresources.IdentityAuthServerDCR{DefaultAccessTokenDuration: value.Dcr.DefaultAccessTokenDuration}
	}
	return r
}

func FetchControlPlaneGroupMembers(
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

func MapNormalizedDCRProvider(provider helpers.NormalizedDCRProviderPayload) declresources.DCRProviderResource {
	resource := declresources.DCRProviderResource{
		BaseResource: declresources.BaseResource{
			Ref:     BuildRef(provider.ID, provider.Name),
			Kongctl: KongctlMetaFromLabels(provider.Labels),
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

	return resource
}

func MapIdentityClient(
	client kkComps.Client,
	scopes []string,
) declresources.IdentityAuthServerClientResource {
	userLabels := decllabels.GetUserLabels(client.Labels)
	if userLabels == nil {
		userLabels = map[string]string{}
	}
	redirects := slices.Clone(client.RedirectUris)
	if redirects == nil {
		redirects = []string{}
	}
	return declresources.IdentityAuthServerClientResource{
		BaseResource: declresources.BaseResource{Ref: client.ID, Kongctl: KongctlMetaFromLabels(client.Labels)},
		Name:         client.Name, GrantTypes: append([]kkComps.GrantType{}, client.GrantTypes...),
		ResponseTypes: append([]kkComps.ResponseType{}, client.ResponseTypes...), RedirectURIs: redirects,
		LoginURI: client.LoginURI, AccessTokenDuration: client.AccessTokenDuration, IDTokenDuration: client.IDTokenDuration,
		AllowAllScopes: client.AllowAllScopes, AllowScopes: scopes, TokenEndpointAuthMethod: client.TokenEndpointAuthMethod,
		Labels: userLabels,
	}
}
