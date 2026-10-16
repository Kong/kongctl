package planner

import (
	"context"
	"fmt"
	"maps"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

type meterPlanner struct{ *BasePlanner }

func NewMeterPlanner(base *BasePlanner) ResourcePlanner { return &meterPlanner{base} }
func (p *meterPlanner) PlannerComponent() string        { return ResourceTypeMeter }
func (p *meterPlanner) PlanChanges(ctx context.Context, cfg *Config, plan *Plan) error {
	desired := resources.GetResourcesByNamespace[resources.MeterResource](p.planner.resources, cfg.Namespace)
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}
	current, err := p.planner.listMeters(ctx)
	if err != nil {
		return err
	}
	byID, byKey := map[string]state.Meter{}, map[string]state.Meter{}
	for _, m := range current {
		byID[m.ID] = m
		byKey[m.Key] = m
	}
	root := func(m state.Meter) managedRoot[state.Meter] {
		return managedRoot[state.Meter]{resource: m, name: m.Key, protected: labels.IsProtectedResource(m.Labels)}
	}
	reconciler := newManagedRootReconciler(
		p.BasePlanner,
		ResourceTypeMeter,
		managedRootOperations[resources.MeterResource, state.Meter]{
			diff: meterDiff,
			create: func(m resources.MeterResource, plan *Plan) string {
				fields := meterFields(m)
				change := PlannedChange{
					ID:           p.planner.nextChangeID(ActionCreate, ResourceTypeMeter, m.Ref),
					Action:       ActionCreate,
					ResourceType: ResourceTypeMeter,
					ResourceRef:  m.Ref,
					Fields:       fields,
					Namespace:    cfg.Namespace,
					Protection:   meterProtected(m),
				}
				plan.AddChange(change)
				return change.ID
			},
			update: p.planUpdate,
			changeProtection: func(
				c state.Meter, d resources.MeterResource, _, _ bool, f map[string]any,
				changed map[string]FieldChange, plan *Plan,
			) {
				p.planUpdate(c, d, f, changed, plan)
			},
			remove: func(m state.Meter, plan *Plan) {
				plan.AddChange(
					PlannedChange{
						ID:           p.planner.nextChangeID(ActionDelete, ResourceTypeMeter, m.Key),
						Action:       ActionDelete,
						ResourceType: ResourceTypeMeter,
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
		c, exists := byKey[d.Key]
		if d.ID != "" {
			c, exists = byID[d.ID]
			if !exists {
				return fmt.Errorf("meter %q id %q not found; remove id to create a new meter", d.Ref, d.ID)
			}
			if c.Key != d.Key {
				return fmt.Errorf("meter %q id and key select different identities; key is immutable", d.Ref)
			}
		}
		var observed *managedRoot[state.Meter]
		if exists {
			if !labels.IsManagedResource(c.Labels) {
				return fmt.Errorf(
					"meter key %q exists but is unmanaged; run kongctl adopt metering-billing meter %s --namespace %s",
					d.Key,
					c.ID,
					cfg.Namespace,
				)
			}
			if c.Labels[labels.NamespaceKey] != cfg.Namespace {
				return fmt.Errorf(
					"meter key %q belongs to namespace %q, not %q",
					d.Key,
					c.Labels[labels.NamespaceKey],
					cfg.Namespace,
				)
			}
			if plan.Metadata.Mode != PlanModeDelete {
				if err := validateMeterImmutable(c, d); err != nil {
					return err
				}
			}
			retained[c.ID] = true
			value := root(c)
			observed = &value
			if resource, ok := p.planner.resources.GetResourceByRef(d.Ref); ok {
				if target, ok := resource.(*resources.MeterResource); ok {
					target.SetKonnectID(c.ID)
				}
			}
		}
		if plan.Metadata.Mode == PlanModeDelete {
			reconciler.deleteDesired(d.Key, observed)
		} else {
			reconciler.reconcileLifecycle(
				managedRoot[resources.MeterResource]{resource: d, name: d.Key, protected: meterProtected(d)},
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

func meterProtected(m resources.MeterResource) bool {
	return m.Kongctl != nil && m.Kongctl.Protected != nil && *m.Kongctl.Protected
}

func validateMeterImmutable(c state.Meter, d resources.MeterResource) error {
	field := ""
	eventsFrom := d.CreateRequest().EventsFrom
	switch {
	case string(c.Aggregation) != string(d.Aggregation):
		field = FieldAggregation
	case c.EventType != d.EventType:
		field = FieldEventType
	case d.ValueProperty != nil && (c.ValueProperty == nil || *d.ValueProperty != *c.ValueProperty):
		field = FieldValueProperty
	case d.EventsFrom != nil && (c.EventsFrom == nil || eventsFrom == nil || !eventsFrom.Equal(*c.EventsFrom)):
		field = FieldEventsFrom
	}
	if field != "" {
		return fmt.Errorf(
			"meter %q cannot update immutable field %s; create a meter with a new key "+
				"and migrate its consumers before deleting the old meter",
			d.Ref,
			field,
		)
	}
	return nil
}

func meterFields(d resources.MeterResource) map[string]any {
	fields := map[string]any{
		FieldName:        d.Name,
		FieldKey:         d.Key,
		FieldAggregation: d.Aggregation,
		FieldEventType:   d.EventType,
	}
	if d.Description != nil {
		fields[FieldDescription] = *d.Description
	}
	if d.ValueProperty != nil {
		fields[FieldValueProperty] = *d.ValueProperty
	}
	if d.EventsFrom != nil {
		fields[FieldEventsFrom] = *d.EventsFrom
	}
	if d.Dimensions != nil {
		fields[FieldDimensions] = d.Dimensions
	}
	if d.Labels != nil {
		fields[FieldLabels] = d.Labels
	}
	return fields
}

func meterDiff(c state.Meter, d resources.MeterResource) (bool, map[string]any, map[string]FieldChange) {
	fields, changed := map[string]any{}, map[string]FieldChange{}
	put := func(field string, old, next any) {
		fields[field] = next
		changed[field] = FieldChange{Old: old, New: next}
	}
	if c.Name != d.Name {
		put(FieldName, c.Name, d.Name)
	}
	description := ""
	if c.Description != nil {
		description = *c.Description
	}
	if d.Description != nil && *d.Description != description {
		put(FieldDescription, description, *d.Description)
	}
	if d.Dimensions != nil && !maps.Equal(c.Dimensions, d.Dimensions) {
		put(FieldDimensions, c.Dimensions, d.Dimensions)
	}
	if d.Labels != nil && labels.CompareUserLabels(c.Labels, d.Labels) {
		put(FieldLabels, labels.GetUserLabels(c.Labels), d.Labels)
	}
	return len(fields) > 0, fields, changed
}

func (p *meterPlanner) planUpdate(
	c state.Meter,
	d resources.MeterResource,
	fields map[string]any,
	changed map[string]FieldChange,
	plan *Plan,
) {
	if _, changed := fields[FieldLabels]; changed {
		fields[FieldCurrentLabels] = c.Labels
	}
	plan.AddChange(
		PlannedChange{
			ID:            p.planner.nextChangeID(ActionUpdate, ResourceTypeMeter, d.Ref),
			Action:        ActionUpdate,
			ResourceType:  ResourceTypeMeter,
			ResourceRef:   d.Ref,
			ResourceID:    c.ID,
			Fields:        fields,
			ChangedFields: changed,
			Namespace:     resources.GetNamespace(d.Kongctl),
			Protection:    ProtectionChange{Old: labels.IsProtectedResource(c.Labels), New: meterProtected(d)},
		},
	)
}
