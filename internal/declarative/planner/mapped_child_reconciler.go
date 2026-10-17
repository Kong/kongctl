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
// Successful matched reconciliation additionally records payload identity.
func reconcileMappedChildren[D, C any](
	p *Planner,
	desired []D,
	currentByName map[string]C,
	ops mappedChildOperations[D, C],
	mode PlanMode,
) error {
	return reconcileIndexedChildren(desired, currentByName,
		indexedChildOperations[D, C]{
			key:    func(item D) (string, bool) { return ops.desiredName(item), true },
			create: ops.create,
			matched: func(item D, current C) error {
				if err := ops.matched(item, current); err != nil {
					return err
				}
				p.recordMatchedIdentity(&item, current)
				return nil
			},
			remove: ops.remove,
		}, mode == PlanModeSync)
}
