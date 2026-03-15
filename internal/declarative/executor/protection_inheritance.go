package executor

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	logctx "github.com/kong/kongctl/internal/log"
)

func (e *Executor) validateInheritedProtection(ctx context.Context, change planner.PlannedChange) error {
	if e == nil || e.client == nil || change.ProtectionParent == nil {
		return nil
	}

	protected, err := e.isProtectionParentProtected(ctx, change.ProtectionParent)
	if err != nil {
		return err
	}
	if !protected {
		return nil
	}

	resourceName := common.ExtractResourceName(change.Fields)
	if resourceName == "[unknown]" && change.ResourceRef != "" && change.ResourceRef != "[unknown]" {
		resourceName = change.ResourceRef
	}

	return fmt.Errorf(
		"resource %q (%s) is protected via parent %q (%s) and cannot be %s",
		resourceName,
		change.ResourceType,
		change.ProtectionParent.ResourceName,
		change.ProtectionParent.ResourceType,
		actionToVerb(change.Action),
	)
}

func (e *Executor) isProtectionParentProtected(
	ctx context.Context,
	info *planner.ProtectionParentInfo,
) (bool, error) {
	if e == nil || e.client == nil || info == nil || info.ResourceName == "" {
		return false, nil
	}
	ctx = withProtectionLookupLogger(ctx)

	switch resources.ResourceType(info.ResourceType) {
	case resources.ResourceTypePortal:
		portal, err := e.client.GetPortalByName(ctx, info.ResourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch portal %q for inherited protection: %w", info.ResourceName, err)
		}
		return portal != nil && labels.IsProtectedResource(portal.NormalizedLabels), nil
	case resources.ResourceTypeAPI:
		api, err := e.client.GetAPIByName(ctx, info.ResourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch API %q for inherited protection: %w", info.ResourceName, err)
		}
		return api != nil && labels.IsProtectedResource(api.NormalizedLabels), nil
	case resources.ResourceTypeControlPlane:
		controlPlane, err := e.client.GetControlPlaneByName(ctx, info.ResourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch control plane %q for inherited protection: %w", info.ResourceName, err)
		}
		return controlPlane != nil && labels.IsProtectedResource(controlPlane.NormalizedLabels), nil
	case resources.ResourceTypeCatalogService:
		service, err := e.client.GetCatalogServiceByName(ctx, info.ResourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch catalog service %q for inherited protection: %w", info.ResourceName, err)
		}
		return service != nil && labels.IsProtectedResource(service.NormalizedLabels), nil
	case resources.ResourceTypeApplicationAuthStrategy:
		strategy, err := e.client.GetAuthStrategyByName(ctx, info.ResourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch auth strategy %q for inherited protection: %w", info.ResourceName, err)
		}
		return strategy != nil && labels.IsProtectedResource(strategy.NormalizedLabels), nil
	case resources.ResourceTypeEventGatewayControlPlane:
		gateway, err := e.client.GetEventGatewayControlPlaneByName(ctx, info.ResourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch event gateway %q for inherited protection: %w", info.ResourceName, err)
		}
		return gateway != nil && labels.IsProtectedResource(gateway.NormalizedLabels), nil
	case resources.ResourceTypeOrganizationTeam:
		team, err := e.client.GetOrganizationTeamByName(ctx, info.ResourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch organization team %q for inherited protection: %w", info.ResourceName, err)
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
		resources.ResourceTypeEventGatewayListener,
		resources.ResourceTypeEventGatewayListenerPolicy,
		resources.ResourceTypeEventGatewayDataPlaneCertificate:
		return false, nil
	}
	return false, nil
}

func withProtectionLookupLogger(ctx context.Context) context.Context {
	if ctx != nil && ctx.Value(logctx.LoggerKey) != nil {
		return ctx
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return context.WithValue(ctx, logctx.LoggerKey, logger)
}
