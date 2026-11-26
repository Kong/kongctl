package planner

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
)

const (
	FieldAICostGatewayID      = "gateway_id"
	FieldAICostInstallVersion = "install_version"
	FieldAICostVersion        = "version"
)

// aiCostPluginPlanner runs after the AI Gateway planner, so a declared gateway
// in its namespace has a matched identity or a planned creation to reference.
type aiCostPluginPlanner struct {
	*BasePlanner
}

func (p *aiCostPluginPlanner) PlannerComponent() string {
	return string(resources.ResourceTypeAICostPlugin)
}

func (p *aiCostPluginPlanner) PlanChanges(ctx context.Context, config *Config, plan *Plan) error {
	return p.planner.planAICostPlugins(ctx, config.Namespace, plan)
}

func (p *Planner) planAICostPlugins(ctx context.Context, namespace string, plan *Plan) error {
	var desired []*resources.AICostPlugin
	if cost := p.resources.CostManagement(); cost != nil {
		for i := range cost.AttributionPlugins {
			if plugin := &cost.AttributionPlugins[i]; resources.GetNamespace(plugin.Kongctl) == namespace {
				desired = append(desired, plugin)
			}
		}
	}
	prune := false
	if scope := p.resources.SyncScope; scope != nil && plan.Metadata.Mode == PlanModeSync {
		_, prune = scope.AICostNamespaces[resources.ResourceTypeAICostPlugin][namespace]
	}
	if len(desired) == 0 && !prune {
		return nil
	}
	observed, err := p.listAICost(ctx, resources.ResourceTypeAICostPlugin)
	if err != nil {
		return err
	}
	return p.planAICostPluginNamespace(plan, namespace, desired, observed, prune)
}

func (p *Planner) planAICostPluginNamespace(
	plan *Plan,
	namespace string,
	desired []*resources.AICostPlugin,
	observed []state.AICostObject,
	prune bool,
) error {
	seen := make(map[string]string)
	byRef := make(map[string]state.AICostObject)
	byGateway := make(map[string]state.AICostObject)
	for _, item := range observed {
		byGateway[item.GetID()] = item
		metadata := item.GetManagedBy()
		ref := metadata[resources.ManagedByRefKey]
		if ref != "" && resources.AICostMatch(metadata, namespace, ref) {
			byRef[ref] = item
		}
	}
	for _, plugin := range desired {
		current := byRef[plugin.Ref]
		delete(byRef, plugin.Ref)
		gatewayID, err := p.aiCostPluginGateway(plugin, plan)
		if err != nil {
			return err
		}
		if gatewayID == "" {
			if plan.Metadata.Mode == PlanModeDelete {
				continue
			}
			return fmt.Errorf("attribution plugin %q: AI Gateway %q has no resolved ID", plugin.Ref, plugin.AIGateway)
		}
		if other, duplicate := seen[gatewayID]; duplicate {
			return fmt.Errorf("attribution plugins %q and %q declare the same AI Gateway", other, plugin.Ref)
		}
		seen[gatewayID] = plugin.Ref
		if current != nil && current.GetID() != gatewayID {
			return fmt.Errorf(
				"attribution plugin %q is installed on another AI Gateway; declare it with a new ref to move it",
				plugin.Ref,
			)
		}
		if current == nil && plan.Metadata.Mode != PlanModeDelete {
			if err := aiCostPluginUnclaimed(plugin, byGateway[gatewayID]); err != nil {
				return err
			}
		}
		if err := p.planAICostChange(plan, plugin, current, gatewayID); err != nil {
			return err
		}
	}
	if !prune {
		return nil
	}
	for _, ref := range slices.Sorted(maps.Keys(byRef)) {
		if err := p.planAICostDelete(plan, resources.ResourceTypeAICostPlugin, namespace, ref, byRef[ref],
			""); err != nil {
			return err
		}
	}
	return nil
}

// A gateway's installation that this declaration doesn't own must not be
// overwritten: one made in Konnect has no kongctl ownership, and one in
// another namespace or under another ref belongs to that declaration.
func aiCostPluginUnclaimed(plugin *resources.AICostPlugin, installed state.AICostObject) error {
	if installed == nil {
		return nil
	}
	metadata := installed.GetManagedBy()
	if metadata[resources.ManagedByToolKey] != resources.ManagedByTool {
		return fmt.Errorf("AI Gateway %q already has an attribution plugin outside kongctl management",
			plugin.AIGateway)
	}
	return fmt.Errorf("AI Gateway %q already has an attribution plugin managed as %q in namespace %q",
		plugin.AIGateway, metadata[resources.ManagedByRefKey], metadata[resources.ChildNamespaceKey])
}

// aiCostPluginGateway returns the gateway's ID, a reference to the gateway this
// plan creates, or "" when neither exists.
func (p *Planner) aiCostPluginGateway(plugin *resources.AICostPlugin, plan *Plan) (string, error) {
	ref := resources.NormalizeResourceRef(plugin.AIGateway)
	target, ok := p.resources.GetResourceByRef(ref)
	gateway, isGateway := target.(*resources.AIGatewayResource)
	if !ok || !isGateway {
		return "", fmt.Errorf("attribution plugin %q requires a declared AI Gateway %q", plugin.Ref, ref)
	}
	if id := gateway.GetKonnectID(); id != "" {
		return id, nil
	}
	if matched, ok := p.matchedIdentities[ref]; ok {
		if observed, ok := matched.observation.(state.AIGateway); ok && observed.ID != "" {
			return observed.ID, nil
		}
	}
	for _, change := range plan.Changes {
		if change.Action == ActionCreate && change.ResourceType == ResourceTypeAIGateway &&
			change.ResourceRef == ref {
			return tags.RefPlaceholderPrefix + ref + "#id", nil
		}
	}
	return "", nil
}

// A gateway removed in the same plan goes after its plugin's removal.
func orderAICostPluginDeletes(plan *Plan) {
	pluginDeletes := make(map[string]string)
	for _, change := range plan.Changes {
		if change.Action == ActionDelete && change.ResourceType == string(resources.ResourceTypeAICostPlugin) {
			pluginDeletes[change.ResourceID] = change.ID
		}
	}
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.Action != ActionDelete || change.ResourceType != ResourceTypeAIGateway {
			continue
		}
		if dependency := pluginDeletes[change.ResourceID]; dependency != "" {
			change.DependsOn = append(change.DependsOn, dependency)
		}
	}
}
