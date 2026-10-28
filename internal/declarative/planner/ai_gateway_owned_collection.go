package planner

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
)

type ownedChildCollectionScope struct{ gatewayRef, gatewayID string }

func childObservationMetadata(value any) (map[string]string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("cannot encode AI Gateway child observation")
	}
	var payload struct {
		ManagedBy map[string]string `json:"managed_by"`
	}
	if json.Unmarshal(data, &payload) != nil {
		return nil, fmt.Errorf("cannot decode AI Gateway child ownership")
	}
	return payload.ManagedBy, nil
}

func ownedDesiredMetadata[D any](desired *D) (*resources.KongctlMeta, error) {
	resource, ok := any(desired).(resources.Resource)
	if !ok {
		return nil, fmt.Errorf("owned child does not implement resource contract")
	}
	meta, enrolled := resources.IndependentChildMetadata(resource)
	if !enrolled {
		return nil, fmt.Errorf("child ownership capability is missing")
	}
	return meta, nil
}

func childMetaProtected(meta *resources.KongctlMeta) bool {
	return meta.IsProtected()
}

// Reuse typed lifecycle callbacks, but reconcile the complete canonical gateway
// once and restrict pruning to explicitly captured source/namespace scopes.
func reconcileOwnedNameMatchedCollection[D, C any](
	p *Planner,
	kind string,
	desired []D,
	current []C,
	ops nameMatchedCollectionOperations[D, C],
	plan *Plan,
) error {
	scope := ops.ownership
	ops.ownership = nil
	key := kind + "/" + scope.gatewayID
	if p.resourceCache != nil && p.resourceCache.ownedChildrenPlanned[key] {
		return nil
	}
	aliases := map[string]bool{scope.gatewayRef: true}
	rs := p.resources
	if rs != nil {
		for _, gateway := range rs.AIGateways {
			if gateway.GetKonnectID() == scope.gatewayID {
				aliases[gateway.Ref] = true
			}
		}
	}
	if rs != nil {
		provided := desired
		desired = nil
		for _, resource := range rs.AllResourcesByType(resources.ResourceType(kind)) {
			parent := resource.(resources.ResourceWithParent).GetParentRef()
			if parent != nil && aliases[resources.NormalizeResourceRef(parent.Ref)] {
				desired = append(desired, *any(resource).(*D))
			}
		}
		if len(desired) == 0 && len(rs.AllResourcesByType(resources.ResourceType(kind))) == 0 {
			desired = provided
		}
	}
	if ops.desiredOrder != nil {
		desired = ops.desiredOrder(desired)
	}
	names := map[string]string{}
	namespaces := map[string]bool{}
	if rs != nil && rs.SyncScope != nil {
		for alias := range aliases {
			for namespace := range rs.SyncScope.OwnedChildNamespaces(resources.ResourceType(kind), alias) {
				namespaces[namespace] = true
			}
		}
	}
	for i := range desired {
		meta, err := ownedDesiredMetadata(&desired[i])
		if err != nil {
			return err
		}
		namespaces[resources.GetNamespace(meta)] = true
		name := ops.desiredName(desired[i])
		resource := any(&desired[i]).(resources.Resource)
		if first, exists := names[name]; exists {
			return fmt.Errorf(
				"duplicate desired %s name %q on gateway %q (refs %q and %q)",
				kind,
				name,
				scope.gatewayRef,
				first,
				resource.GetRef(),
			)
		}
		names[name] = resource.GetRef()
	}
	if len(namespaces) == 0 && (rs == nil || rs.SyncScope == nil) {
		if rs != nil {
			for _, namespace := range rs.DefaultNamespaces {
				namespaces[namespace] = true
			}
		}
		if len(namespaces) == 0 {
			namespaces[DefaultNamespace] = true
		}
	}
	metadata := ops.ownershipMetadata
	if metadata == nil {
		metadata = func(value C) (map[string]string, error) { return childObservationMetadata(value) }
	}
	filtered := make([]C, 0, len(current))
	for _, value := range current {
		name, _ := ops.currentName(value)
		m, err := metadata(value)
		if err != nil {
			return err
		}
		if _, matched := names[name]; matched ||
			(m[resources.ChildNamespaceKey] != "" && namespaces[m[resources.ChildNamespaceKey]]) {
			// Prune candidates need the same validation as name matches before
			// malformed protection can be interpreted as unprotected.
			if plan.Metadata.Mode == PlanModeSync && !matched {
				if err := resources.ValidateChildOwnership(kind, m, m[resources.ChildNamespaceKey]); err != nil {
					return err
				}
			}
			filtered = append(filtered, value)
		}
	}
	reconcile := ops.reconcile
	remove := ops.remove
	ops.reconcile = func(child D, observed *C) error {
		meta, err := ownedDesiredMetadata(&child)
		if err != nil {
			return err
		}
		namespace := resources.GetNamespace(meta)
		if observed != nil {
			m, err := metadata(*observed)
			if err != nil {
				return err
			}
			if err := resources.ValidateChildOwnership(kind, m, namespace); err != nil {
				return err
			}
		}
		start := len(plan.Changes)
		if plan.Metadata.Mode == PlanModeDelete {
			if observed == nil {
				return nil
			}
			m, err := metadata(*observed)
			if err != nil {
				return err
			}
			if resources.IsChildProtected(m) {
				return fmt.Errorf(
					"%s %q is protected and cannot be deleted",
					kind,
					ops.desiredName(child),
				)
			}
			remove(*observed)
		} else if err := reconcile(child, observed); err != nil {
			return err
		}
		for i := start; i < len(plan.Changes); i++ {
			change := &plan.Changes[i]
			change.Namespace = namespace
			if change.ResourceType == kind {
				stampOwnedChildChange(change, meta)
				if change.Action == ActionUpdate &&
					declaredOwnedChildLabels(any(&child).(resources.Resource)) == nil {
					delete(change.Fields, FieldLabels)
				}
			}
		}
		return nil
	}
	ops.protected = func(value C) bool { m, _ := metadata(value); return resources.IsChildProtected(m) }
	ops.remove = func(value C) {
		m, _ := metadata(value)
		start := len(plan.Changes)
		remove(value)
		for i := start; i < len(plan.Changes); i++ {
			plan.Changes[i].Namespace = m[resources.ChildNamespaceKey]
		}
	}
	if err := reconcileNameMatchedCollection(p, kind, desired, filtered, ops, plan); err != nil {
		return err
	}
	if p.resourceCache != nil {
		p.resourceCache.ownedChildrenPlanned[key] = true
	}
	return nil
}

func stampOwnedChildChange(change *PlannedChange, meta *resources.KongctlMeta) {
	change.Namespace = resources.GetNamespace(meta)
	if change.Action == ActionUpdate {
		if field, ok := change.ChangedFields[FieldManagedBy]; ok {
			old := childManagedByField(map[string]any{FieldManagedBy: field.Old})
			if resources.IsChildProtected(old) != childMetaProtected(meta) {
				change.Protection = ProtectionChange{
					Old: resources.IsChildProtected(old),
					New: childMetaProtected(meta),
				}
			}
		}
	}
	if change.Action == ActionCreate {
		change.Fields[FieldManagedBy] = resources.BuildChildManagedBy(
			nil,
			childManagedByField(change.Fields),
			change.Namespace,
			childMetaProtected(meta),
			"",
		)
		change.Protection = childMetaProtected(meta)
	}
}

func childManagedByField(payload map[string]any) map[string]string {
	result := map[string]string{}
	switch value := payload[FieldManagedBy].(type) {
	case map[string]string:
		maps.Copy(result, value)
	case map[string]any:
		for k, v := range value {
			if text, ok := v.(string); ok {
				result[k] = text
			}
		}
	}
	return result
}

func prepareOwnedChildDiff(
	current, desired map[string]any,
	resource resources.Resource,
	labels map[string]string,
) error {
	meta, enrolled := resources.IndependentChildMetadata(resource)
	if !enrolled {
		return nil
	}
	observed := childManagedByField(current)
	merged := resources.BuildChildManagedBy(
		observed,
		childManagedByField(desired),
		resources.GetNamespace(meta),
		childMetaProtected(meta),
		observed[resources.ManagedByVersionKey],
	)
	if err := resources.ValidateChildManagedBy(merged, false); err != nil {
		return err
	}
	desired[FieldManagedBy] = merged
	if labels == nil {
		desired[FieldLabels] = current[FieldLabels]
	} else {
		desired[FieldLabels] = labels
	}
	return nil
}

func validateOwnedChildReconciliation[D, C any](
	kind string,
	full C,
	desired D,
	fields map[string]any,
	changed map[string]FieldChange,
) error {
	meta, err := ownedDesiredMetadata(&desired)
	if err != nil {
		return err
	}
	current, err := childObservationMetadata(full)
	if err != nil {
		return err
	}
	if err := resources.ValidateChildOwnership(kind, current, resources.GetNamespace(meta)); err != nil {
		return err
	}
	return validateOwnedPayloadChange(kind, current, fields, changed)
}

func validateOwnedPayloadChange(kind string, current map[string]string,
	fields map[string]any, changed map[string]FieldChange,
) error {
	if resources.IsChildProtected(current) &&
		(len(changed) != 1 || changed[FieldManagedBy].New == nil ||
			!resources.ChildUnprotectionMetadataOnly(current,
				childManagedByField(fields))) {
		return fmt.Errorf(
			"%s is protected; unprotect it in a separate update (changed fields: %s)",
			kind,
			strings.Join(slices.Sorted(maps.Keys(changed)), ", "),
		)
	}
	return nil
}

func declaredOwnedChildLabels(resource resources.Resource) map[string]string {
	if typed, ok := resource.(interface{ PublicLabels() map[string]string }); ok {
		return typed.PublicLabels()
	}
	if typed, ok := resource.(interface{ GetLabels() map[string]string }); ok {
		return typed.GetLabels()
	}
	return nil
}
