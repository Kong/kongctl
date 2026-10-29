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
	gatewayID          string
	targetType         string
	usesTarget         func(string) bool
	observe            func(context.Context) ([]observedReferenceUser, error)
	observedReferences func(PlannedChange, []string) bool
	plannedReferences  func(PlannedChange, PlannedChange) bool
	updateDetaches     func(PlannedChange, PlannedChange) bool
	conflict           func(PlannedChange, observedReferenceUser, bool) error
}

func resolveObservedReferenceDeletes(
	ctx context.Context, _ string, gatewayRef string, plan *Plan, policy observedReferenceDeletePolicy,
) error {
	type resourceKey struct{ kind, id string }
	changes := make(map[resourceKey]*PlannedChange)
	var deletes []*PlannedChange
	var plannedUsers []*PlannedChange
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if !aiGatewayChangeMatchesGateway(*change, gatewayRef, policy.gatewayID) {
			continue
		}
		if change.ResourceType == policy.targetType && change.Action == ActionDelete {
			deletes = append(deletes, change)
		}
		if !policy.usesTarget(change.ResourceType) {
			continue
		}
		if change.ResourceID != "" {
			key := resourceKey{change.ResourceType, change.ResourceID}
			// A final deletion wins over intermediate updates, independent of source
			// order. New replacement creates have their own identity/lifecycle.
			if previous := changes[key]; previous == nil || previous.Action != ActionDelete {
				changes[key] = change
			}
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
		replacement := false
		name, named := deletion.Fields[FieldName].(string)
		for _, candidate := range plan.Changes {
			if named && name != "" && candidate.Action == ActionCreate && candidate.ResourceType == deletion.ResourceType &&
				candidate.Fields[FieldName] == name && aiGatewayChangeMatchesGateway(candidate, gatewayRef, policy.gatewayID) {
				replacement = true
			}
		}
		for _, change := range plannedUsers {
			if change.Action == ActionUpdate && change.ResourceID != "" {
				final := changes[resourceKey{change.ResourceType, change.ResourceID}]
				if final != nil && final.Action == ActionDelete {
					final.DependsOn = appendDependsOn(final.DependsOn, change.ID)
					continue
				}
			}
			// Only a new user can attach after same-name replacement. A retained user
			// cannot detach from a name by updating it to that identical name.
			if replacement && change.Action == ActionCreate {
				continue
			}
			if policy.plannedReferences(*deletion, *change) {
				return policy.conflict(
					*deletion,
					observedReferenceUser{resourceType: change.ResourceType, name: change.ResourceRef},
					true,
				)
			}
		}
		for _, user := range users {
			if user.resourceType == policy.targetType && user.id == deletion.ResourceID {
				continue
			}
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
