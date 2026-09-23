package planner

// nameMatchedChildOperations keeps observation, comparison, and change construction
// typed. Parent scope and creation of children under new parents stay with callers.
type nameMatchedChildOperations[D, C any] struct {
	desiredName func(D) string
	currentName func(C) string
	fetch       func(D, C) (*C, error)
	diff        func(C, D) (bool, map[string]any, map[string]FieldChange, error)
	create      func(D)
	update      func(C, D, map[string]any, map[string]FieldChange)
	protected   func(C) bool
	remove      func(C)
	// Optional ordering of the same observations, used only for sync pruning.
	pruneOrder func([]C) []C
}

// reconcileNameMatchedChildren matches names within the caller's parent scope,
// refreshes matched observations, and prunes in observed order unless overridden. An absent detail
// response recreates the desired child; an observation or protection error stops reconciliation.
func reconcileNameMatchedChildren[D, C any](
	p *Planner,
	resourceType string,
	desired []D,
	current []C,
	ops nameMatchedChildOperations[D, C],
	plan *Plan,
) error {
	return reconcileNameMatchedCollection(p, resourceType, desired, current,
		nameMatchedCollectionOperations[D, C]{
			desiredName: ops.desiredName,
			currentName: func(current C) (string, bool) {
				name := ops.currentName(current)
				return name, name != ""
			},
			reconcile: func(child D, observed *C) error {
				if observed == nil {
					ops.create(child)
					return nil
				}
				full, err := ops.fetch(child, *observed)
				if err != nil {
					return err
				}
				if full == nil {
					ops.create(child)
					return nil
				}
				needsUpdate, fields, changed, err := ops.diff(*full, child)
				if err != nil {
					return err
				}
				if needsUpdate {
					ops.update(*observed, child, fields, changed)
				}
				return nil
			},
			protected:  ops.protected,
			remove:     ops.remove,
			pruneOrder: ops.pruneOrder,
		}, plan)
}
