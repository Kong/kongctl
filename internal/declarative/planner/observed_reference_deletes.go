package planner

import "context"

type observedReferenceUser struct {
	resourceType string
	id           string
	name         string
	references   []string
}

// Adapters own API identity and proven detachment. Schema relationships alone
// cannot establish what an omitted field does in an SDK update request.
type observedReferenceDeletePolicy struct {
	targetType         string
	usesTarget         func(string) bool
	observe            func(context.Context) ([]observedReferenceUser, error)
	observedReferences func(PlannedChange, []string) bool
	plannedReferences  func(PlannedChange, PlannedChange) bool
	updateDetaches     func(PlannedChange, PlannedChange) bool
	conflict           func(PlannedChange, observedReferenceUser, bool) error
}

func resolveObservedReferenceDeletes(
	ctx context.Context, namespace, gatewayRef string, plan *Plan, policy observedReferenceDeletePolicy,
) error {
	type resourceKey struct{ kind, id string }
	changes := make(map[resourceKey]*PlannedChange)
	var deletes []*PlannedChange
	var plannedUsers []*PlannedChange
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.Namespace != namespace || !aiGatewayChildChangeMatchesParent(*change, gatewayRef) {
			continue
		}
		if change.ResourceType == policy.targetType && change.Action == ActionDelete {
			deletes = append(deletes, change)
		}
		if !policy.usesTarget(change.ResourceType) {
			continue
		}
		if change.ResourceID != "" {
			changes[resourceKey{change.ResourceType, change.ResourceID}] = change
		}
		if change.Action == ActionCreate || change.Action == ActionUpdate {
			plannedUsers = append(plannedUsers, change)
		}
	}
	if len(deletes) == 0 {
		return nil
	}
	users, err := policy.observe(ctx)
	if err != nil {
		return err
	}
	for _, deletion := range deletes {
		for _, change := range plannedUsers {
			if policy.plannedReferences(*deletion, *change) {
				return policy.conflict(
					*deletion,
					observedReferenceUser{resourceType: change.ResourceType, name: change.ResourceRef},
					true,
				)
			}
		}
		for _, user := range users {
			if !policy.observedReferences(*deletion, user.references) {
				continue
			}
			change := changes[resourceKey{user.resourceType, user.id}]
			if change != nil && (change.Action == ActionDelete ||
				(change.Action == ActionUpdate && policy.updateDetaches(*deletion, *change))) {
				deletion.DependsOn = appendDependsOn(deletion.DependsOn, change.ID)
				continue
			}
			return policy.conflict(*deletion, user, false)
		}
	}
	return nil
}
