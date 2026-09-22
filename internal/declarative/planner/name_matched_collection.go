package planner

// nameMatchedCollectionOperations separates collection traversal from one
// resource's lifecycle, which may also plan nested children. A nil observation
// means no name match; a non-nil observation is the original listed value.
type nameMatchedCollectionOperations[D, C any] struct {
	desiredName func(D) string
	// The boolean controls indexing only. Unmatchable observations still take
	// part in sync pruning, using the returned name.
	currentName func(C) (string, bool)
	reconcile   func(D, *C) error
	remove      func(C)
	// Optional for collections whose members support deletion protection.
	protected func(C) bool
	// Optional ordering of the same observations, used only for sync pruning.
	pruneOrder func([]C) []C
}

// reconcileNameMatchedCollection visits desired resources in declaration order,
// then prunes observations in sync mode. Any reconciliation or protection error
// stops traversal. Matching always uses the last eligible observation per name.
func reconcileNameMatchedCollection[D, C any](
	p *Planner,
	resourceType string,
	desired []D,
	current []C,
	ops nameMatchedCollectionOperations[D, C],
	plan *Plan,
) error {
	byName := make(map[string]C, len(current))
	for _, item := range current {
		if name, matchable := ops.currentName(item); matchable {
			byName[name] = item
		}
	}

	desiredNames := make(map[string]bool, len(desired))
	for _, item := range desired {
		name := ops.desiredName(item)
		desiredNames[name] = true
		var matched *C
		if observed, exists := byName[name]; exists {
			matched = &observed
		}
		if err := ops.reconcile(item, matched); err != nil {
			return err
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		pruning := current
		if ops.pruneOrder != nil {
			pruning = ops.pruneOrder(current)
		}
		for _, item := range pruning {
			name, _ := ops.currentName(item)
			if desiredNames[name] {
				continue
			}
			if ops.protected != nil {
				if err := p.validateProtection(resourceType, name, ops.protected(item), ActionDelete); err != nil {
					return err
				}
			}
			ops.remove(item)
		}
	}
	return nil
}
