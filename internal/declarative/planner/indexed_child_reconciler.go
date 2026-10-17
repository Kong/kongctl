package planner

// indexedChildOperations separates collection decisions from observation,
// comparison, change construction, and identity binding. key returns false to
// exclude an item from both reconciliation and sync retention. To retain a
// skipped item's key, return true and skip work in create/matched instead.
type indexedChildOperations[D, C any] struct {
	key     func(D) (string, bool)
	create  func(D) error
	matched func(D, C) error
	remove  func(string, C)
}

// reconcileIndexedChildren preserves desired order and prunes only after all
// selected items succeed. The caller's index retains its eligibility and
// duplicate-collapse rules; map pruning has no ordering guarantee. This helper
// does not record matched payload identities or mutate the supplied index.
func reconcileIndexedChildren[D, C any](
	desired []D,
	currentByKey map[string]C,
	ops indexedChildOperations[D, C],
	prune bool,
) error {
	desiredKeys := make(map[string]bool, len(desired))
	for _, item := range desired {
		key, selected := ops.key(item)
		if !selected {
			continue
		}
		desiredKeys[key] = true
		var err error
		if current, exists := currentByKey[key]; exists {
			err = ops.matched(item, current)
		} else {
			err = ops.create(item)
		}
		if err != nil {
			return err
		}
	}
	if prune {
		for key, current := range currentByKey {
			if !desiredKeys[key] {
				ops.remove(key, current)
			}
		}
	}
	return nil
}
