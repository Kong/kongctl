package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/konnect/helpers"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

type featurePlanner struct{ *BasePlanner }

func NewFeaturePlanner(base *BasePlanner) ResourcePlanner { return &featurePlanner{base} }
func (p *featurePlanner) PlannerComponent() string        { return ResourceTypeFeature }
func (p *featurePlanner) PlanChanges(ctx context.Context, cfg *Config, plan *Plan) error {
	desired := resources.GetResourcesByNamespace[resources.FeatureResource](p.planner.resources, cfg.Namespace)
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}
	current, err := p.planner.listFeatures(ctx)
	if err != nil {
		return err
	}
	byID, byKey := map[string]state.Feature{}, map[string]state.Feature{}
	for _, m := range current {
		byID[m.ID] = m
		byKey[m.Key] = m
	}
	root := func(m state.Feature) managedRoot[state.Feature] {
		return managedRoot[state.Feature]{resource: m, name: m.Key, protected: labels.IsProtectedResource(m.Labels)}
	}
	reconciler := newManagedRootReconciler(
		p.BasePlanner,
		ResourceTypeFeature,
		managedRootOperations[resources.FeatureResource, state.Feature]{
			diff: featureDiff,
			create: func(m resources.FeatureResource, plan *Plan) string {
				fields := featureFields(m)
				change := PlannedChange{
					ID:           p.planner.nextChangeID(ActionCreate, ResourceTypeFeature, m.Ref),
					Action:       ActionCreate,
					ResourceType: ResourceTypeFeature,
					ResourceRef:  m.Ref,
					Fields:       fields,
					Namespace:    cfg.Namespace,
					Protection:   featureProtected(m),
				}
				plan.AddChange(change)
				return change.ID
			},
			update: p.planUpdate,
			changeProtection: func(
				c state.Feature, d resources.FeatureResource, _, _ bool, f map[string]any,
				changed map[string]FieldChange, plan *Plan,
			) {
				p.planUpdate(c, d, f, changed, plan)
			},
			remove: func(m state.Feature, plan *Plan) {
				plan.AddChange(
					PlannedChange{
						ID:           p.planner.nextChangeID(ActionDelete, ResourceTypeFeature, m.Key),
						Action:       ActionDelete,
						ResourceType: ResourceTypeFeature,
						ResourceRef:  m.Key,
						ResourceID:   m.ID,
						Fields:       map[string]any{FieldName: m.Name},
						Namespace:    m.Labels[labels.NamespaceKey],
					},
				)
			},
		},
		plan,
	)
	retained := map[string]bool{}
	for _, d := range desired {
		if plan.Metadata.Mode != PlanModeDelete && d.Meter != nil && !helpers.IsMeterULID(d.Meter.ID) {
			resolved, err := p.resolveFeatureMeter(ctx, d.Meter.ID)
			if err != nil {
				return fmt.Errorf("feature %q: %w", d.Ref, err)
			}
			meter := *d.Meter
			meter.ID = resolved
			d.Meter = &meter
		}

		c, exists := byKey[d.Key]
		if d.ID != "" {
			c, exists = byID[d.ID]
			if !exists {
				return fmt.Errorf("feature %q id %q not found; remove id to create a new feature", d.Ref, d.ID)
			}
			if c.Key != d.Key {
				return fmt.Errorf("feature %q id and key select different identities; key is immutable", d.Ref)
			}
		}
		var observed *managedRoot[state.Feature]
		if exists {
			if !labels.IsManagedResource(c.Labels) {
				return fmt.Errorf(
					"feature key %q exists but is unmanaged; feature adoption is unsupported "+
						"because PATCH cannot update labels; use a new key", d.Key,
				)
			}
			if c.Labels[labels.NamespaceKey] != cfg.Namespace {
				return fmt.Errorf(
					"feature key %q belongs to namespace %q, not %q",
					d.Key,
					c.Labels[labels.NamespaceKey],
					cfg.Namespace,
				)
			}
			if plan.Metadata.Mode != PlanModeDelete {
				if err := validateFeatureImmutable(c, d); err != nil {
					return err
				}
			}
			retained[c.ID] = true
			value := root(c)
			observed = &value
			if resource, ok := p.planner.resources.GetResourceByRef(d.Ref); ok {
				if target, ok := resource.(*resources.FeatureResource); ok {
					target.SetKonnectID(c.ID)
				}
			}
		}
		if plan.Metadata.Mode == PlanModeDelete {
			reconciler.deleteDesired(d.Key, observed)
		} else {
			reconciler.reconcileLifecycle(
				managedRoot[resources.FeatureResource]{resource: d, name: d.Key, protected: featureProtected(d)},
				observed,
			)
		}
	}
	if plan.Metadata.Mode == PlanModeSync {
		for _, c := range current {
			if labels.IsManagedResource(c.Labels) && c.Labels[labels.NamespaceKey] == cfg.Namespace && !retained[c.ID] {
				reconciler.remove(root(c))
			}
		}
	}
	return reconciler.errors.Error()
}

func featureProtected(m resources.FeatureResource) bool {
	return m.Kongctl != nil && m.Kongctl.Protected != nil && *m.Kongctl.Protected
}

func validateFeatureImmutable(c state.Feature, d resources.FeatureResource) error {
	field := ""
	switch {
	case c.Name != d.Name:
		field = FieldName
	case d.Description != nil && !reflect.DeepEqual(c.Description, d.Description):
		field = FieldDescription
	case d.Meter == nil && c.Meter != nil || d.Meter != nil &&
		(c.Meter == nil || c.Meter.ID != d.Meter.ID || !featureFiltersEqual(c.Meter.Filters, d.Meter.Filters)):
		field = FieldMeter
	case d.Labels != nil && labels.CompareUserLabels(c.Labels, d.Labels):
		field = FieldLabels
	case labels.IsProtectedResource(c.Labels) != featureProtected(d):
		field = FieldProtected
	}
	if field != "" {
		return fmt.Errorf("feature %q cannot update %s: PATCH supports only unit_cost; "+
			"create a feature with a new key and migrate consumers", d.Ref, field)
	}
	return nil
}

func featureFields(d resources.FeatureResource) map[string]any {
	fields := map[string]any{FieldName: d.Name, FieldKey: d.Key}
	if d.Description != nil {
		fields[FieldDescription] = *d.Description
	}
	if d.Labels != nil {
		fields[FieldLabels] = d.Labels
	}
	if d.Meter != nil {
		fields[FieldMeter] = map[string]any{FieldID: d.Meter.ID}
		if d.Meter.Filters != nil {
			fields[FieldMeter].(map[string]any)[FieldFilters] = d.Meter.Filters
		}
	}
	if d.UnitCostSet || d.UnitCost != nil {
		fields[FieldUnitCost] = d.UnitCost
	}
	return fields
}

func featureFiltersEqual(a, b map[string]kkComps.QueryFilterStringMapItem) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return featureJSONEqual(a, b)
}

func featureCostsEqual(a, b *helpers.FeatureUnitCost) bool {
	if a != nil && b != nil && a.Type == "manual" && b.Type == "manual" && a.Amount != nil && b.Amount != nil {
		x, okX := new(big.Rat).SetString(*a.Amount)
		y, okY := new(big.Rat).SetString(*b.Amount)
		return okX && okY && x.Cmp(y) == 0
	}
	return featureJSONEqual(a, b)
}

func featureJSONEqual(a, b any) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	var x, y any
	if json.Unmarshal(left, &x) != nil || json.Unmarshal(right, &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

func featureDiff(c state.Feature, d resources.FeatureResource) (bool, map[string]any, map[string]FieldChange) {
	fields, changed := map[string]any{}, map[string]FieldChange{}
	old := helpers.FeatureCostConfiguration(c.UnitCost)
	if (d.UnitCostSet || d.UnitCost != nil) && !featureCostsEqual(old, d.UnitCost) {
		fields[FieldUnitCost] = d.UnitCost
		changed[FieldUnitCost] = FieldChange{Old: old, New: d.UnitCost}
	}
	return len(fields) > 0, fields, changed
}

func (p *featurePlanner) resolveFeatureMeter(ctx context.Context, value string) (string, error) {
	ref := value
	if tags.IsRefPlaceholder(value) {
		var selector string
		var ok bool
		ref, selector, ok = tags.ParseRefPlaceholder(value)
		if !ok || (selector != FieldID && selector != "ID") {
			return "", fmt.Errorf("meter reference must select id")
		}
	}

	resource, ok := p.planner.resources.GetResourceByRef(ref)
	if !ok || resource.GetType() != resources.ResourceTypeMeter {
		return "", fmt.Errorf("meter.id must be a meter ref or an uppercase ULID; meter %q not found", ref)
	}
	if id := resource.GetKonnectID(); id != "" {
		return id, nil
	}
	// Namespace planning may visit the feature before its meter. Observe
	// globally so existing meter identities are available in either order.
	current, err := p.planner.listMeters(ctx)
	if err != nil {
		return "", err
	}
	desired := resource.(*resources.MeterResource)
	for _, meter := range current {
		if desired.ID != "" && meter.ID == desired.ID || desired.ID == "" && meter.Key == desired.Key {
			return meter.ID, nil
		}
	}
	// The complete plan must supply the meter creation before this binding
	// can execute. Validate that dependency after namespace plans are merged.
	return tags.RefPlaceholderPrefix + ref + "#" + FieldID, nil
}

func (p *featurePlanner) planUpdate(
	c state.Feature,
	d resources.FeatureResource,
	fields map[string]any,
	changed map[string]FieldChange,
	plan *Plan,
) {
	if _, changed := fields[FieldLabels]; changed {
		fields[FieldCurrentLabels] = c.Labels
	}
	plan.AddChange(
		PlannedChange{
			ID:            p.planner.nextChangeID(ActionUpdate, ResourceTypeFeature, d.Ref),
			Action:        ActionUpdate,
			ResourceType:  ResourceTypeFeature,
			ResourceRef:   d.Ref,
			ResourceID:    c.ID,
			Fields:        fields,
			ChangedFields: changed,
			Namespace:     resources.GetNamespace(d.Kongctl),
			Protection:    ProtectionChange{Old: labels.IsProtectedResource(c.Labels), New: featureProtected(d)},
		},
	)
}

// Bind new meter dependencies against the complete plan, across namespaces.
func bindFeatureMeterReferences(plan *Plan) error {
	creates := map[string]string{}
	for _, change := range plan.Changes {
		if change.ResourceType == ResourceTypeMeter && change.Action == ActionCreate {
			creates[change.ResourceRef] = change.ID
		}
	}
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.ResourceType != ResourceTypeFeature || change.Action != ActionCreate {
			continue
		}
		meter, _ := change.Fields[FieldMeter].(map[string]any)
		value, _ := meter[FieldID].(string)
		if !tags.IsRefPlaceholder(value) {
			continue
		}
		ref, _, ok := tags.ParseRefPlaceholder(value)
		dependency := creates[ref]
		if !ok || dependency == "" {
			return fmt.Errorf(
				"feature %q: meter %q has no resolved ID or creation operation in this plan",
				change.ResourceRef,
				ref,
			)
		}
		change.PayloadReferences = append(change.PayloadReferences, PayloadReference{
			Path: "/meter/id", ResourceType: ResourceTypeMeter, Ref: ref, Selector: FieldID,
		})
		change.DependsOn = appendDependsOn(change.DependsOn, dependency)
	}
	return nil
}
