package planner

// roleAssignmentOperations supplies identity and change construction for roles
// within one owner. Observation, owner selection and sync scope stay local.
type roleAssignmentOperations[D, C any] struct {
	key       func(D) string
	duplicate func(string) error
	create    func(D)
	remove    func(C)
}

// reconcileRoleAssignments preserves desired order and detects duplicates even
// when the assignment already exists. Earlier creates remain on error; pruning
// starts only after all desired assignments succeed. The supplied index retains
// its caller's observation filtering and last-observation-wins policy.
func reconcileRoleAssignments[D, C any](
	desired []D,
	current map[string]C,
	ops roleAssignmentOperations[D, C],
	prune bool,
) error {
	desiredKeys := make(map[string]bool, len(desired))
	for _, role := range desired {
		key := ops.key(role)
		if desiredKeys[key] {
			return ops.duplicate(key)
		}
		desiredKeys[key] = true
		if _, exists := current[key]; !exists {
			ops.create(role)
		}
	}
	if prune {
		for key, role := range current {
			if !desiredKeys[key] {
				ops.remove(role)
			}
		}
	}
	return nil
}
