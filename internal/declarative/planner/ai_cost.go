package planner

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"

	"github.com/kong/kongctl/internal/declarative/state"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
)

const (
	FieldAICostBudgetID = "budget_id"
	FieldAICostEffect   = "effect"
)

type aiCostPlanner struct {
	*BasePlanner
	kind resources.ResourceType
}

func (p *aiCostPlanner) PlannerComponent() string { return string(p.kind) }

func (p *aiCostPlanner) PlanChanges(ctx context.Context, config *Config, plan *Plan) error {
	return p.planner.planAICostCollection(ctx, p.kind, config.Namespace, plan)
}

func (p *Planner) planAICostCollection(
	ctx context.Context,
	kind resources.ResourceType,
	namespace string,
	plan *Plan,
) error {
	var desired []resources.AICostResource
	for _, resource := range p.resources.AllResources() {
		if resource.GetType() != kind {
			continue
		}
		declaration := resource.(resources.AICostResource)
		if !declaration.IsExternal() && resources.GetNamespace(declaration.CostMeta()) == namespace {
			desired = append(desired, declaration)
		}
	}
	prune := false
	if scope := p.resources.SyncScope; scope != nil && plan.Metadata.Mode == PlanModeSync {
		_, prune = scope.AICostNamespaces[kind][namespace]
	}
	if len(desired) == 0 && !prune {
		return nil
	}
	observed, err := p.listAICost(ctx, kind)
	if err != nil {
		return err
	}
	byRef := make(map[string]state.AICostObject)
	for _, item := range observed {
		metadata := item.GetManagedBy()
		ref := metadata[resources.ManagedByRefKey]
		if ref == "" || !resources.AICostMatch(metadata, namespace, ref) {
			continue
		}
		if byRef[ref] != nil {
			return fmt.Errorf("multiple %s resources match namespace %q and ref %q", kind, namespace, ref)
		}
		byRef[ref] = item
	}
	for _, declaration := range desired {
		current := byRef[declaration.GetRef()]
		delete(byRef, declaration.GetRef())
		if current != nil {
			declaration.SetKonnectID(current.GetID())
		}
		if err := p.planAICostChange(plan, declaration, current, ""); err != nil {
			return err
		}
	}
	if prune {
		for _, ref := range slices.Sorted(maps.Keys(byRef)) {
			if err := p.planAICostDelete(plan, kind, namespace, ref, byRef[ref], ""); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Planner) listAICost(ctx context.Context, kind resources.ResourceType) ([]state.AICostObject, error) {
	if items, ok := p.resourceCache.aiCost[kind]; ok {
		return items, nil
	}
	items, err := p.client.Cost.List(ctx, kind, 100)
	if err != nil {
		return nil, err
	}
	p.resourceCache.aiCost[kind] = items
	return items, nil
}

func aiCostMetadata(desired resources.AICostResource) map[string]string {
	metadata := resources.BuildChildManagedBy(resources.GetNamespace(desired.CostMeta()), desired.Protected(), "")
	if desired.GetType() != resources.ResourceTypeAICostRule {
		metadata[resources.ManagedByRefKey] = desired.GetRef()
	}
	return metadata
}

func (p *Planner) planAICostChange(
	plan *Plan,
	desired resources.AICostResource,
	current state.AICostObject,
	budgetID string,
) error {
	kind, ref := desired.GetType(), desired.GetRef()
	namespace := resources.GetNamespace(desired.CostMeta())
	if plan.Metadata.Mode == PlanModeDelete {
		if current == nil {
			return nil
		}
		return p.planAICostDelete(plan, kind, namespace, ref, current, budgetID)
	}
	fields, err := resources.AICostFields(desired)
	if err != nil {
		return err
	}
	fields[FieldManagedBy] = aiCostMetadata(desired)
	if budgetID != "" {
		fields[FieldAICostBudgetID] = budgetID
	}
	change := PlannedChange{
		ResourceType: string(kind),
		ResourceRef:  ref,
		Namespace:    namespace,
		Fields:       fields,
		Protection:   desired.Protected(),
	}
	if current == nil {
		if channel, ok := desired.(*resources.AICostChannel); ok && channel.Type == "slack" &&
			channel.Slack.WebhookURL == nil {
			return fmt.Errorf("creating Slack channel %q requires slack.webhook_url from !secret", ref)
		}
		change.Action = ActionCreate
	} else {
		change.Action = ActionUpdate
		change.ResourceID = current.GetID()
		if budgetID != "" {
			change.ResourceID = budgetID
		}
		change.ChangedFields, err = diffAICost(kind, current, fields)
		if err != nil {
			return fmt.Errorf("%s %q: %w", kind, ref, err)
		}
		if len(change.ChangedFields) == 0 {
			return nil
		}
		if resources.IsChildProtected(current.GetManagedBy()) &&
			(desired.Protected() || len(change.ChangedFields) != 1 || change.ChangedFields[FieldManagedBy].New == nil) {
			return fmt.Errorf("%s %q is protected; remove protection in a separate update", kind, ref)
		}
		if old := resources.IsChildProtected(current.GetManagedBy()); old != desired.Protected() {
			change.Protection = ProtectionChange{Old: old, New: desired.Protected()}
		}
	}
	change.ID = p.nextChangeID(change.Action, string(kind), ref)
	plan.AddChange(change)
	return nil
}

func diffAICost(
	kind resources.ResourceType,
	current state.AICostObject,
	fields map[string]any,
) (map[string]FieldChange, error) {
	observed := maps.Clone(current)
	desired := maps.Clone(fields)
	resources.AICostComparable(kind, observed)
	resources.AICostComparable(kind, desired)
	immutable := resources.AICostImmutableFields(kind)
	changes := make(map[string]FieldChange)
	for key, value := range desired {
		switch key {
		case FieldAICostBudgetID:
			continue
		case FieldManagedBy:
			if !managedByValuesEqual(current.GetManagedBy(), value) {
				changes[key] = FieldChange{Old: current.GetManagedBy(), New: value}
			}
		case "slack":
			// Webhook state is write-only, and the secret writer handles rotation.
			continue
		default:
			if equalAICostValue(observed[key], value) {
				continue
			}
			if slices.Contains(immutable, key) {
				return nil, fmt.Errorf("%s is immutable", key)
			}
			changes[key] = FieldChange{Old: observed[key], New: value}
		}
	}
	return changes, nil
}

func equalAICostValue(observed, desired any) bool {
	if left, ok := observed.(map[string]any); ok {
		if right, ok := desired.(map[string]any); ok {
			// API responses can include explicit null optional properties.
			for key, value := range left {
				if value != nil && !equalAICostValue(value, right[key]) {
					return false
				}
			}
			for key, value := range right {
				if value != nil && !equalAICostValue(left[key], value) {
					return false
				}
			}
			return true
		}
	}
	return reflect.DeepEqual(observed, desired)
}

func (p *Planner) planAICostDelete(plan *Plan, kind resources.ResourceType, namespace, ref string,
	current state.AICostObject, budgetID string,
) error {
	if resources.IsChildProtected(current.GetManagedBy()) {
		return fmt.Errorf("%s %q is protected", kind, ref)
	}
	id := current.GetID()
	fields := map[string]any{}
	if budgetID != "" {
		id = budgetID
		fields[FieldAICostBudgetID] = budgetID
	}
	if id == "" {
		return fmt.Errorf("missing ID for %s %q", kind, ref)
	}
	plan.AddChange(PlannedChange{
		ID: p.nextChangeID(ActionDelete, string(kind), ref), Action: ActionDelete, ResourceType: string(kind),
		ResourceRef: ref, ResourceID: id, Namespace: namespace, Fields: fields,
	})
	return nil
}

func (p *Planner) planAICostRules(ctx context.Context, plan *Plan) error {
	cost := p.resources.CostManagement()
	if cost == nil {
		return nil
	}
	seen := make(map[string]bool)
	for i := range cost.NotificationRules {
		rule := &cost.NotificationRules[i]
		ref := resources.NormalizeResourceRef(rule.Budget)
		target, ok := p.resources.GetResourceByRef(ref)
		budget, isBudget := target.(*resources.AICostBudget)
		if !ok || !isBudget {
			return fmt.Errorf("notification rule requires a declared budget %q", ref)
		}
		id := budget.GetKonnectID()
		var current state.AICostObject
		if id != "" {
			if budget.IsExternal() {
				observed, err := p.client.Cost.Get(ctx, resources.ResourceTypeAICostBudget, id)
				if err != nil {
					return err
				}
				if observed == nil || !resources.AICostRuleApplies(observed[FieldAICostEffect]) {
					return fmt.Errorf("budget %q must be an existing limit budget", ref)
				}
			} else if !resources.AICostRuleApplies(budget.Effect) {
				return fmt.Errorf("budget %q must be a limit budget", ref)
			}
			var err error
			current, err = p.client.Cost.Get(ctx, resources.ResourceTypeAICostRule, id)
			if err != nil {
				return err
			}
		} else {
			if plan.Metadata.Mode == PlanModeDelete {
				continue
			}
			if budget.IsExternal() || !resources.AICostRuleApplies(budget.Effect) {
				return fmt.Errorf("budget %q must be a limit budget", ref)
			}
			id = tags.RefPlaceholderPrefix + ref + "#id"
		}
		if seen[id] {
			return fmt.Errorf("multiple notification rules declare the same budget %q", ref)
		}
		seen[id] = true
		if current != nil {
			metadata := current.GetManagedBy()
			if metadata[resources.ManagedByToolKey] != resources.ManagedByTool {
				return fmt.Errorf("budget %q already has a notification rule outside kongctl management", ref)
			}
			namespace := resources.GetNamespace(rule.Kongctl)
			if err := resources.ValidateChildOwnership("notification rule", metadata, namespace); err != nil {
				return fmt.Errorf("budget %q: %w", ref, err)
			}
		}
		if err := p.planAICostChange(plan, rule, current, id); err != nil {
			return err
		}
	}
	return nil
}

func (p *Planner) resolveAICostDeletes(ctx context.Context, plan *Plan) error {
	ruleDeletes := make(map[string]string)
	for _, change := range plan.Changes {
		if change.Action == ActionDelete && change.ResourceType == string(resources.ResourceTypeAICostRule) {
			ruleDeletes[change.ResourceID] = change.ID
		}
	}
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.Action != ActionDelete {
			continue
		}
		//exhaustive:ignore
		switch resources.ResourceType(change.ResourceType) {
		case resources.ResourceTypeAICostBudget:
			budget, err := p.client.Cost.Get(ctx, resources.ResourceTypeAICostBudget, change.ResourceID)
			if err != nil {
				return err
			}
			if budget == nil || !resources.AICostRuleApplies(budget[FieldAICostEffect]) {
				continue
			}
			rule, err := p.client.Cost.Get(ctx, resources.ResourceTypeAICostRule, change.ResourceID)
			if err != nil {
				return err
			}
			if rule == nil {
				continue
			}
			dependency := ruleDeletes[change.ResourceID]
			if dependency == "" {
				return fmt.Errorf(
					"budget %q has a notification rule; explicitly delete the rule first",
					change.ResourceRef,
				)
			}
			change.DependsOn = append(change.DependsOn, dependency)
		case resources.ResourceTypeAICostChannel:
			budgets, err := p.listAICost(ctx, resources.ResourceTypeAICostBudget)
			if err != nil {
				return err
			}
			for _, budget := range budgets {
				if !resources.AICostRuleApplies(budget[FieldAICostEffect]) {
					continue
				}
				rule, err := p.client.Cost.Get(ctx, resources.ResourceTypeAICostRule, budget.GetID())
				if err != nil {
					return err
				}
				if !state.AICostRuleUsesChannel(rule, change.ResourceID) {
					continue
				}
				dependency := ruleDeletes[budget.GetID()]
				if dependency == "" {
					return fmt.Errorf(
						"channel %q is referenced by a notification rule; detach it before deletion",
						change.ResourceRef,
					)
				}
				change.DependsOn = append(change.DependsOn, dependency)
			}
		}
	}
	return nil
}
