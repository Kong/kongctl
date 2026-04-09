package protection

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// fetchProtection calls fetch to retrieve normalized labels for the named
// resource and returns whether the resource is protected.
func fetchProtection(
	resourceName, kind string,
	fetch func() (map[string]string, error),
) (bool, error) {
	normalizedLabels, err := fetch()
	if err != nil {
		if state.IsAPIClientError(err) {
			return false, nil
		}
		return false, fmt.Errorf("fetch %s %q for inherited protection: %w", kind, resourceName, err)
	}
	return labels.IsProtectedResource(normalizedLabels), nil
}

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
		return fetchProtection(resourceName, "portal", func() (map[string]string, error) {
			r, err := client.GetPortalByName(ctx, resourceName)
			if r == nil {
				return nil, err
			}
			return r.NormalizedLabels, err
		})
	case resources.ResourceTypeAPI:
		return fetchProtection(resourceName, "API", func() (map[string]string, error) {
			r, err := client.GetAPIByName(ctx, resourceName)
			if r == nil {
				return nil, err
			}
			return r.NormalizedLabels, err
		})
	case resources.ResourceTypeControlPlane:
		return fetchProtection(resourceName, "control plane", func() (map[string]string, error) {
			r, err := client.GetControlPlaneByName(ctx, resourceName)
			if r == nil {
				return nil, err
			}
			return r.NormalizedLabels, err
		})
	case resources.ResourceTypeCatalogService:
		return fetchProtection(resourceName, "catalog service", func() (map[string]string, error) {
			r, err := client.GetCatalogServiceByName(ctx, resourceName)
			if r == nil {
				return nil, err
			}
			return r.NormalizedLabels, err
		})
	case resources.ResourceTypeApplicationAuthStrategy:
		return fetchProtection(resourceName, "auth strategy", func() (map[string]string, error) {
			r, err := client.GetAuthStrategyByName(ctx, resourceName)
			if r == nil {
				return nil, err
			}
			return r.NormalizedLabels, err
		})
	case resources.ResourceTypeEventGatewayControlPlane:
		return fetchProtection(resourceName, "event gateway", func() (map[string]string, error) {
			r, err := client.GetEventGatewayControlPlaneByName(ctx, resourceName)
			if r == nil {
				return nil, err
			}
			return r.NormalizedLabels, err
		})
	case resources.ResourceTypeOrganizationTeam:
		return fetchProtection(resourceName, "organization team", func() (map[string]string, error) {
			r, err := client.GetOrganizationTeamByName(ctx, resourceName)
			if r == nil {
				return nil, err
			}
			return r.NormalizedLabels, err
		})
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
		resources.ResourceTypeEventGatewayDataPlaneCertificate,
		resources.ResourceTypeEventGatewayProducePolicy:
		return false, nil
	}

	return false, nil
}
