package planner

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	logctx "github.com/kong/kongctl/internal/log"
)

type inheritedProtectionState struct {
	info      ProtectionParentInfo
	protected bool
}

func (p *Planner) applyInheritedProtection(ctx context.Context, plan *Plan) error {
	if p == nil || p.resources == nil || plan == nil {
		return nil
	}

	cache := make(map[string]*inheritedProtectionState)
	collector := &ProtectionErrorCollector{}

	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.Parent == nil || change.Parent.Ref == "" {
			continue
		}

		state, err := p.getInheritedProtectionState(ctx, cache, change.Parent.Ref)
		if err != nil {
			return err
		}
		if state == nil {
			continue
		}

		change.ProtectionParent = &state.info

		if state.protected && (change.Action == ActionUpdate || change.Action == ActionDelete) {
			collector.Add(fmt.Errorf(
				"%s %q is protected via parent %s %q and cannot be %s",
				change.ResourceType,
				extractInheritedProtectionResourceName(*change),
				state.info.ResourceType,
				state.info.ResourceName,
				actionVerb(change.Action),
			))
		}
	}

	return collector.Error()
}

func (p *Planner) getInheritedProtectionState(
	ctx context.Context,
	cache map[string]*inheritedProtectionState,
	parentRef string,
) (*inheritedProtectionState, error) {
	topLevel, err := p.resolveTopLevelProtectionParent(parentRef)
	if err != nil || topLevel == nil {
		return nil, err
	}

	cacheKey := topLevel.ResourceRef
	if cacheKey == "" {
		cacheKey = topLevel.ResourceType + ":" + topLevel.ResourceName
	}
	if cached, ok := cache[cacheKey]; ok {
		return cached, nil
	}

	protected, err := p.isTopLevelParentProtected(ctx, topLevel)
	if err != nil {
		return nil, err
	}

	state := &inheritedProtectionState{
		info:      *topLevel,
		protected: protected,
	}
	cache[cacheKey] = state
	return state, nil
}

func (p *Planner) resolveTopLevelProtectionParent(parentRef string) (*ProtectionParentInfo, error) {
	currentRef := parentRef
	visited := make(map[string]struct{})

	for currentRef != "" {
		if _, seen := visited[currentRef]; seen {
			return nil, fmt.Errorf("circular parent reference detected while resolving %q", parentRef)
		}
		visited[currentRef] = struct{}{}

		resource, ok := p.resources.GetResourceByRef(currentRef)
		if !ok || resource == nil {
			return nil, nil
		}

		withParent, ok := resource.(resources.ResourceWithParent)
		if !ok {
			return &ProtectionParentInfo{
				ResourceType: string(resource.GetType()),
				ResourceRef:  resource.GetRef(),
				ResourceID:   resource.GetKonnectID(),
				ResourceName: resource.GetMoniker(),
			}, nil
		}

		parent := withParent.GetParentRef()
		if parent == nil || parent.Ref == "" {
			return &ProtectionParentInfo{
				ResourceType: string(resource.GetType()),
				ResourceRef:  resource.GetRef(),
				ResourceID:   resource.GetKonnectID(),
				ResourceName: resource.GetMoniker(),
			}, nil
		}

		currentRef = parent.Ref
	}

	return nil, nil
}

func (p *Planner) isTopLevelParentProtected(ctx context.Context, info *ProtectionParentInfo) (bool, error) {
	if p == nil || p.client == nil || info == nil || info.ResourceName == "" {
		return false, nil
	}
	ctx = p.withProtectionLookupLogger(ctx)

	switch resources.ResourceType(info.ResourceType) {
	case resources.ResourceTypePortal:
		portal, err := p.client.GetPortalByName(ctx, info.ResourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch portal %q for inherited protection: %w", info.ResourceName, err)
		}
		return portal != nil && labels.IsProtectedResource(portal.NormalizedLabels), nil
	case resources.ResourceTypeAPI:
		api, err := p.client.GetAPIByName(ctx, info.ResourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch API %q for inherited protection: %w", info.ResourceName, err)
		}
		return api != nil && labels.IsProtectedResource(api.NormalizedLabels), nil
	case resources.ResourceTypeControlPlane:
		controlPlane, err := p.client.GetControlPlaneByName(ctx, info.ResourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch control plane %q for inherited protection: %w", info.ResourceName, err)
		}
		return controlPlane != nil && labels.IsProtectedResource(controlPlane.NormalizedLabels), nil
	case resources.ResourceTypeCatalogService:
		service, err := p.client.GetCatalogServiceByName(ctx, info.ResourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch catalog service %q for inherited protection: %w", info.ResourceName, err)
		}
		return service != nil && labels.IsProtectedResource(service.NormalizedLabels), nil
	case resources.ResourceTypeApplicationAuthStrategy:
		strategy, err := p.client.GetAuthStrategyByName(ctx, info.ResourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch auth strategy %q for inherited protection: %w", info.ResourceName, err)
		}
		return strategy != nil && labels.IsProtectedResource(strategy.NormalizedLabels), nil
	case resources.ResourceTypeEventGatewayControlPlane:
		gateway, err := p.client.GetEventGatewayControlPlaneByName(ctx, info.ResourceName)
		if err != nil {
			if state.IsAPIClientError(err) {
				return false, nil
			}
			return false, fmt.Errorf("fetch event gateway %q for inherited protection: %w", info.ResourceName, err)
		}
		return gateway != nil && labels.IsProtectedResource(gateway.NormalizedLabels), nil
	case resources.ResourceTypeOrganizationTeam:
		team, err := p.client.GetOrganizationTeamByName(ctx, info.ResourceName)
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

func (p *Planner) withProtectionLookupLogger(ctx context.Context) context.Context {
	if ctx != nil && ctx.Value(logctx.LoggerKey) != nil {
		return ctx
	}
	if p != nil && p.logger != nil {
		return context.WithValue(ctx, logctx.LoggerKey, p.logger)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return context.WithValue(ctx, logctx.LoggerKey, logger)
}

func extractInheritedProtectionResourceName(change PlannedChange) string {
	if name, ok := change.Fields["name"].(string); ok && name != "" {
		return name
	}
	if version, ok := change.Fields["version"].(string); ok && version != "" {
		return version
	}
	if title, ok := change.Fields["title"].(string); ok && title != "" {
		return title
	}
	if change.ResourceRef != "" && change.ResourceRef != "[unknown]" {
		return change.ResourceRef
	}
	if change.ResourceID != "" {
		return change.ResourceID
	}
	return "[unknown]"
}

func actionVerb(action ActionType) string {
	switch action {
	case ActionCreate:
		return "created"
	case ActionDelete:
		return "deleted"
	case ActionUpdate:
		return "updated"
	case ActionExternalTool:
		return "processed"
	}

	return "modified"
}
