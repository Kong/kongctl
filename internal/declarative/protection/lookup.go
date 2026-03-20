package protection

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// IsManagedResourceProtected returns whether a top-level managed resource is
// currently protected in Konnect.
func IsManagedResourceProtected(
	ctx context.Context,
	client *state.Client,
	resourceType resources.ResourceType,
	resourceName string,
) (bool, error) {
	if client == nil || resourceName == "" {
		return false, nil
	}

	switch resourceType {
	case resources.ResourceTypePortal:
		portal, err := client.GetPortalByName(ctx, resourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch portal %q for inherited protection: %w", resourceName, err)
		}
		return portal != nil && labels.IsProtectedResource(portal.NormalizedLabels), nil
	case resources.ResourceTypeAPI:
		api, err := client.GetAPIByName(ctx, resourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch API %q for inherited protection: %w", resourceName, err)
		}
		return api != nil && labels.IsProtectedResource(api.NormalizedLabels), nil
	case resources.ResourceTypeControlPlane:
		controlPlane, err := client.GetControlPlaneByName(ctx, resourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch control plane %q for inherited protection: %w", resourceName, err)
		}
		return controlPlane != nil && labels.IsProtectedResource(controlPlane.NormalizedLabels), nil
	case resources.ResourceTypeCatalogService:
		service, err := client.GetCatalogServiceByName(ctx, resourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch catalog service %q for inherited protection: %w", resourceName, err)
		}
		return service != nil && labels.IsProtectedResource(service.NormalizedLabels), nil
	case resources.ResourceTypeApplicationAuthStrategy:
		strategy, err := client.GetAuthStrategyByName(ctx, resourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch auth strategy %q for inherited protection: %w", resourceName, err)
		}
		return strategy != nil && labels.IsProtectedResource(strategy.NormalizedLabels), nil
	case resources.ResourceTypeEventGatewayControlPlane:
		gateway, err := client.GetEventGatewayControlPlaneByName(ctx, resourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch event gateway %q for inherited protection: %w", resourceName, err)
		}
		return gateway != nil && labels.IsProtectedResource(gateway.NormalizedLabels), nil
	case resources.ResourceTypeOrganizationTeam:
		team, err := client.GetOrganizationTeamByName(ctx, resourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch organization team %q for inherited protection: %w", resourceName, err)
		}
		return team != nil && labels.IsProtectedResource(team.NormalizedLabels), nil
	case resources.ResourceTypeAPIVersion,
		resources.ResourceTypeAPIPublication,
		resources.ResourceTypeAPIImplementation,
		resources.ResourceTypeAPIDocument,
		resources.ResourceTypeGatewayService,
		resources.ResourceTypePortalCustomization,
		resources.ResourceTypePortalCustomDomain,
		resources.ResourceTypePortalAuthSettings,
		resources.ResourceTypePortalPage,
		resources.ResourceTypePortalSnippet,
		resources.ResourceTypePortalTeam,
		resources.ResourceTypePortalTeamRole,
		resources.ResourceTypePortalAssetLogo,
		resources.ResourceTypePortalAssetFavicon,
		resources.ResourceTypePortalEmailConfig,
		resources.ResourceTypePortalEmailTemplate,
		resources.ResourceTypeEventGatewayBackendCluster,
		resources.ResourceTypeEventGatewayVirtualCluster,
		resources.ResourceTypeEventGatewayClusterPolicy,
		resources.ResourceTypeEventGatewayListener,
		resources.ResourceTypeEventGatewayListenerPolicy,
		resources.ResourceTypeEventGatewayDataPlaneCertificate:
		return false, nil
	}

	return false, nil
}
