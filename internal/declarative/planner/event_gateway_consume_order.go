package planner

import (
	"slices"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
)

func (p *Planner) planConsumePolicyOrder(
	namespace, gatewayID, gatewayRef, clusterID, clusterRef, clusterName string,
	desired []resources.EventGatewayConsumePolicyResource, current []state.EventGatewayConsumePolicyInfo,
	changeStart int, plan *Plan,
) {
	byRef := make(map[string]string)
	byID := make(map[string]string)
	declared := make(map[string]bool)
	for _, policy := range desired {
		byRef[policy.Ref] = policy.GetMoniker()
		declared[policy.GetMoniker()] = true
	}
	for _, policy := range current {
		if policy.Name != nil {
			byID[policy.ID] = *policy.Name
		}
	}
	// Canonicalize parent IDs and !refs to the same scope where possible.
	scope := func(parent string) string {
		if tags.IsRefPlaceholder(parent) {
			return "name:" + byRef[referenceTargetRef(parent)]
		}
		if name, ok := byID[parent]; ok {
			return "name:" + name
		}
		return parent
	}
	groups := make(map[string][]string)
	parents := make(map[string]string)
	var keys []string
	for _, policy := range desired {
		parent := consumePolicyParentPolicyID(policy)
		key := scope(parent)
		if _, ok := groups[key]; !ok {
			keys = append(keys, key)
			parents[key] = parent
		}
		groups[key] = append(groups[key], policy.GetMoniker())
	}
	var dependencies []string
	recreated := make(map[string]bool)
	removed := make(map[string]bool)
	previous := make(map[string]string)
	refScope := make(map[string]string)
	for _, policy := range desired {
		refScope[policy.Ref] = scope(consumePolicyParentPolicyID(policy))
	}
	for i := changeStart; i < len(plan.Changes); i++ {
		change := &plan.Changes[i]
		dependencies = append(dependencies, change.ID)
		if change.Action == ActionDelete {
			removed[change.ResourceID] = true
		}
		if change.Action == ActionCreate {
			key := refScope[change.ResourceRef]
			if prev := previous[key]; prev != "" {
				change.DependsOn = append(change.DependsOn, prev)
			}
			previous[key] = change.ID
			recreated[key] = true
		}
	}
	for _, key := range keys {
		names := groups[key]
		var old, extras []string
		for _, policy := range current {
			parent := ""
			if policy.ParentPolicyID != nil {
				parent = *policy.ParentPolicyID
			}
			if scope(parent) != key {
				continue
			}
			name := policy.ID
			if policy.Name != nil {
				name = *policy.Name
			}
			old = append(old, name)
			if !declared[name] && !removed[policy.ID] {
				extras = append(extras, name)
			}
		}
		target := append(slices.Clone(names), extras...)
		if len(target) < 2 || (slices.Equal(old, target) && !recreated[key]) {
			continue
		}
		intent := &ConsumePolicyOrder{Names: names, ParentID: parents[key]}
		if name, ok := byID[intent.ParentID]; ok {
			intent.ParentName = name
			intent.ParentID = ""
		} else if tags.IsRefPlaceholder(intent.ParentID) {
			intent.ParentName = byRef[referenceTargetRef(intent.ParentID)]
			intent.ParentID = ""
		}
		id := clusterID
		if id == "" {
			id = resources.UnknownReferenceID
		}
		ref := clusterRef + ":consume-policy-order:" + key
		plan.AddChange(PlannedChange{
			ID:           p.nextChangeID(ActionUpdate, ResourceTypeEventGatewayConsumePolicy, ref),
			ResourceType: ResourceTypeEventGatewayConsumePolicy, ResourceRef: ref,
			ResourceID: id, Action: ActionUpdate, Namespace: namespace,
			ConsumePolicyOrder: intent, DependsOn: slices.Clone(dependencies),
			Parent:        &ParentInfo{Ref: clusterRef, ID: clusterID},
			ChangedFields: map[string]FieldChange{FieldPolicyOrder: {Old: old, New: target}},
			References: map[string]ReferenceInfo{
				FieldEventGatewayID: {Ref: gatewayRef, ID: gatewayID},
				FieldEventGatewayVirtualClusterID: {
					Ref: clusterRef, ID: clusterID,
					LookupFields: map[string]string{FieldName: clusterName},
				},
			},
		})
	}
}
