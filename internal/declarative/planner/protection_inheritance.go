package planner

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/protection"
	"github.com/kong/kongctl/internal/declarative/resources"
	logctx "github.com/kong/kongctl/internal/log"
)

type inheritedProtectionState struct {
	info      ProtectingParentInfo
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

		change.ProtectingParent = &state.info

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
	topLevel, err := p.resolveTopLevelProtectingParent(parentRef)
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

	protected, err := p.isTopLevelProtectingParentProtected(ctx, topLevel)
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

func (p *Planner) resolveTopLevelProtectingParent(parentRef string) (*ProtectingParentInfo, error) {
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
			return &ProtectingParentInfo{
				ResourceType: string(resource.GetType()),
				ResourceRef:  resource.GetRef(),
				ResourceID:   resource.GetKonnectID(),
				ResourceName: resource.GetMoniker(),
			}, nil
		}

		parent := withParent.GetParentRef()
		if parent == nil || parent.Ref == "" {
			return &ProtectingParentInfo{
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

func (p *Planner) isTopLevelProtectingParentProtected(ctx context.Context, info *ProtectingParentInfo) (bool, error) {
	if p == nil || info == nil {
		return false, nil
	}
	ctx = p.withProtectionLookupLogger(ctx)

	return protection.IsManagedResourceProtected(
		ctx,
		p.client,
		resources.ResourceType(info.ResourceType),
		info.ResourceName,
	)
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
