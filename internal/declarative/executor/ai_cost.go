package executor

import (
	"context"
	"fmt"
	"maps"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/state"

	"github.com/kong/kongctl/internal/build"
	declerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/util"
)

type aiCostExecutor struct {
	api  *state.AICostAPI
	kind resources.ResourceType
}

func (e *Executor) registerAICostExecutors() {
	var api *state.AICostAPI
	if e.client != nil {
		api = e.client.Cost
	}
	for _, kind := range []resources.ResourceType{
		resources.ResourceTypeAICostBudget, resources.ResourceTypeAICostChannel,
		resources.ResourceTypeAICostSchedule, resources.ResourceTypeAICostRule, resources.ResourceTypeAICostPlugin,
	} {
		handler := &aiCostExecutor{api: api, kind: kind}
		e.registerResourceExecutor(
			resourceExecutor{contract: handler, create: handler.write, update: handler.write, remove: handler.remove},
		)
	}
}

func (a *aiCostExecutor) ResourceType() string { return string(a.kind) }

func (a *aiCostExecutor) ValidatePayload(_ context.Context, change planner.PlannedChange) error {
	switch change.Action {
	case planner.ActionDelete:
		return nil
	case planner.ActionCreate:
		if change.ResourceID != "" {
			return fmt.Errorf("AI Cost Management create cannot select an existing ID")
		}
	case planner.ActionUpdate:
		if !util.IsValidUUID(change.ResourceID) {
			return fmt.Errorf("AI Cost Management update requires an existing UUID")
		}
	case planner.ActionExternalTool:
		return fmt.Errorf("unsupported AI Cost Management action %q", change.Action)
	}
	parentField := planner.AICostParentField(a.kind)
	for key := range change.Fields {
		if parentField != "" && key == parentField {
			continue
		}
		if !resources.AICostWriteField(a.kind, key) {
			return fmt.Errorf("unsupported %s field %q", a.kind, key)
		}
	}
	metadata := childPayloadManagedBy(change.Fields)
	if metadata[resources.ManagedByToolKey] != resources.ManagedByTool ||
		(a.kind != resources.ResourceTypeAICostRule && metadata[resources.ManagedByRefKey] != change.ResourceRef) {
		return fmt.Errorf("%s requires generated kongctl ownership metadata", a.kind)
	}
	if parentField != "" && change.Fields[parentField] == nil {
		return fmt.Errorf("missing %s %s", a.kind, parentField)
	}
	return validateChildPayloadOwnership(string(a.kind), metadata, change.Namespace)
}

func (a *aiCostExecutor) write(ctx context.Context, change *planner.PlannedChange) (string, error) {
	if err := a.ValidatePayload(ctx, *change); err != nil {
		return "", err
	}
	fields := maps.Clone(change.Fields)
	id := change.ResourceID
	if parentField := planner.AICostParentField(a.kind); parentField != "" {
		parent, _ := fields[parentField].(string)
		if !util.IsValidUUID(parent) || (id != "" && id != parent) {
			return "", fmt.Errorf("invalid %s %s", a.kind, parentField)
		}
		id = parent
		delete(fields, parentField)
	}
	if a.kind == resources.ResourceTypeAICostRule {
		budget, err := a.api.Get(ctx, resources.ResourceTypeAICostBudget, id)
		if err != nil {
			return "", err
		}
		if budget == nil || !resources.AICostRuleApplies(budget[planner.FieldAICostEffect]) {
			return "", fmt.Errorf("notification rule requires an existing limit budget")
		}
	}
	metadata, err := resources.PrepareManagedBy(a.kind, childPayloadManagedBy(fields), build.Version(ctx))
	if err != nil {
		return "", err
	}
	fields[planner.FieldManagedBy] = metadata
	if change.Action == planner.ActionCreate {
		if err := a.checkCreate(ctx, change, id); err != nil {
			return "", err
		}
	} else {
		current, err := a.api.Get(ctx, a.kind, id)
		if err != nil {
			return "", err
		}
		if current == nil {
			return "", fmt.Errorf("%s disappeared after planning", a.kind)
		}
		if err := a.checkOwner(current, change); err != nil {
			return "", err
		}
		before := resources.AICostWriteFields(a.kind, current)
		after := maps.Clone(before)
		maps.Copy(after, fields)
		if focus, planned := fields[planner.FieldAICostFocus]; planned && focus == nil {
			delete(after, planner.FieldAICostFocus)
		}
		if slack, ok := before[planner.FieldAICostSlack].(map[string]any); ok {
			desired, _ := fields[planner.FieldAICostSlack].(map[string]any)
			merged := maps.Clone(slack)
			maps.Copy(merged, desired)
			after[planner.FieldAICostSlack] = merged
		}
		for _, field := range resources.AICostImmutableFields(a.kind) {
			if !reflect.DeepEqual(before[field], after[field]) {
				return "", fmt.Errorf("%s is immutable", field)
			}
		}
		if resources.IsChildProtected(current.GetManagedBy()) {
			if !childUnprotectOnly(change, current.GetManagedBy(), metadata) {
				return "", fmt.Errorf("%s is protected", a.kind)
			}
			before[planner.FieldManagedBy] = metadata
			resources.AICostComparable(a.kind, before)
			resources.AICostComparable(a.kind, after)
			if !reflect.DeepEqual(before, after) {
				return "", fmt.Errorf("unprotection cannot include configuration changes")
			}
		}
		fields = after
	}
	if err := validateResolvedRequest(ctx, fields); err != nil {
		return "", err
	}
	result, err := a.api.Write(ctx, a.kind, id, fields)
	if err != nil {
		if slack, _ := fields[planner.FieldAICostSlack].(map[string]any); slack["webhook_url"] != nil {
			return "", fmt.Errorf(
				"notification channel write failed (HTTP status %d)",
				declerrors.ExtractStatusCodeFromError(err),
			)
		}
		return "", err
	}
	if !maps.Equal(result.GetManagedBy(), metadata) {
		return "", fmt.Errorf("%s did not retain kongctl ownership", a.kind)
	}
	if planner.AICostParentField(a.kind) != "" {
		return id, nil
	}
	if result.GetID() == "" {
		return "", fmt.Errorf("%s response is missing its ID", a.kind)
	}
	return result.GetID(), nil
}

func (a *aiCostExecutor) checkCreate(ctx context.Context, change *planner.PlannedChange, id string) error {
	if planner.AICostParentField(a.kind) != "" {
		current, err := a.api.Get(ctx, a.kind, id)
		if err != nil {
			return err
		}
		if current != nil {
			return childCreationCollision(string(a.kind), change.ResourceRef)
		}
		return nil
	}
	values, err := a.api.List(ctx, a.kind, 100)
	if err != nil {
		return err
	}
	for _, value := range values {
		if resources.AICostMatch(value.GetManagedBy(), change.Namespace, change.ResourceRef) {
			return childCreationCollision(string(a.kind), change.ResourceRef)
		}
	}
	return nil
}

func (a *aiCostExecutor) checkOwner(current state.AICostObject, change *planner.PlannedChange) error {
	metadata := current.GetManagedBy()
	if metadata[resources.ManagedByToolKey] != resources.ManagedByTool ||
		(a.kind != resources.ResourceTypeAICostRule && metadata[resources.ManagedByRefKey] != change.ResourceRef) {
		return fmt.Errorf("%s ownership changed; regenerate the plan", a.kind)
	}
	if err := resources.ValidateChildOwnership(string(a.kind), metadata, change.Namespace); err != nil {
		return err
	}
	return validateChildPlannedProtection(string(a.kind), metadata, change)
}

func (a *aiCostExecutor) remove(ctx context.Context, change *planner.PlannedChange) error {
	if !util.IsValidUUID(change.ResourceID) {
		return fmt.Errorf("invalid %s ID", a.kind)
	}
	current, err := a.api.Get(ctx, a.kind, change.ResourceID)
	if err != nil || current == nil {
		return err
	}
	if err := a.checkOwner(current, change); err != nil {
		return err
	}
	if resources.IsChildProtected(current.GetManagedBy()) {
		return fmt.Errorf("%s is protected", a.kind)
	}
	//exhaustive:ignore
	switch a.kind {
	case resources.ResourceTypeAICostBudget:
		if resources.AICostRuleApplies(current[planner.FieldAICostEffect]) {
			rule, err := a.api.Get(ctx, resources.ResourceTypeAICostRule, change.ResourceID)
			if err != nil {
				return err
			}
			if rule != nil {
				return fmt.Errorf("budget still has a notification rule")
			}
		}
	case resources.ResourceTypeAICostChannel:
		budgets, err := a.api.List(ctx, resources.ResourceTypeAICostBudget, 100)
		if err != nil {
			return err
		}
		for _, budget := range budgets {
			if !resources.AICostRuleApplies(budget[planner.FieldAICostEffect]) {
				continue
			}
			rule, err := a.api.Get(ctx, resources.ResourceTypeAICostRule, budget.GetID())
			if err != nil {
				return err
			}
			if state.AICostRuleUsesChannel(rule, change.ResourceID) {
				return fmt.Errorf("channel is still referenced by a notification rule")
			}
		}
	}
	return a.api.Delete(ctx, a.kind, change.ResourceID)
}
