package planner

// mappedChildOperations keeps observation eligibility, resource actions, and
// nested planning with adapters. Callers supply the same index used for any
// reference preparation, including its duplicate-name and empty-name policy.
type mappedChildOperations[D, C any] struct {
	desiredName func(D) string
	create      func(D) error
	matched     func(D, C) error
	remove      func(string, C)
}

// reconcileMappedChildren visits desired resources in their supplied order,
// then prunes the name index in sync mode. Map traversal deliberately has no
// ordering guarantee and visits only the indexed observation for each name.
// This differs from pruning every item of an ordered observation collection.
func reconcileMappedChildren[D, C any](
	desired []D,
	currentByName map[string]C,
	ops mappedChildOperations[D, C],
	mode PlanMode,
) error {
	desiredNames := make(map[string]bool, len(desired))
	for _, item := range desired {
		name := ops.desiredName(item)
		desiredNames[name] = true
		var err error
		if current, exists := currentByName[name]; exists {
			err = ops.matched(item, current)
		} else {
			err = ops.create(item)
		}
		if err != nil {
			return err
		}
	}
	if mode == PlanModeSync {
		for name, current := range currentByName {
			if !desiredNames[name] {
				ops.remove(name, current)
			}
		}
	}
	return nil
}
