package planner

import "github.com/kong/kongctl/internal/util"

type childIdentity struct {
	id   string
	name string
}

func desiredChildIdentity(ref, id, name string) childIdentity {
	if id == "" && util.IsValidUUID(ref) {
		id = ref
	}
	return childIdentity{id: id, name: name}
}

// idMatchedChildOperations keeps observation, comparison, and change construction
// typed. Parent scope and creation of children under new parents stay with callers.
type idMatchedChildOperations[D, C any] struct {
	desiredIdentity func(D) childIdentity
	currentIdentity func(C) childIdentity
	fetch           func(D, C) (*C, error)
	diff            func(C, D) (bool, map[string]any, map[string]FieldChange, error)
	create          func(D)
	update          func(C, D, map[string]any, map[string]FieldChange)
	protected       func(C) bool
	remove          func(C)
}

// reconcileIDMatchedChildren matches explicit IDs before names, refreshes matched
// observations, and prunes in observed order. An absent detail response recreates
// the desired child; an observation or protection error stops reconciliation.
func reconcileIDMatchedChildren[D, C any](
	p *Planner,
	resourceType string,
	desired []D,
	current []C,
	ops idMatchedChildOperations[D, C],
	plan *Plan,
) error {
	byID := make(map[string]C)
	byName := make(map[string]C)
	for _, child := range current {
		identity := ops.currentIdentity(child)
		if identity.id != "" {
			byID[identity.id] = child
		}
		if identity.name != "" {
			byName[identity.name] = child
		}
	}

	desiredKeys := make(map[string]bool)
	for _, child := range desired {
		identity := ops.desiredIdentity(child)
		observed, exists := byName[identity.name]
		if identity.id != "" {
			observed, exists = byID[identity.id]
		}
		desiredKeys[identity.name] = true
		if identity.id != "" {
			desiredKeys[identity.id] = true
		}
		if !exists {
			ops.create(child)
			continue
		}

		full, err := ops.fetch(child, observed)
		if err != nil {
			return err
		}
		if full == nil {
			ops.create(child)
			continue
		}
		needsUpdate, fields, changed, err := ops.diff(*full, child)
		if err != nil {
			return err
		}
		if needsUpdate {
			ops.update(observed, child, fields, changed)
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		for _, child := range current {
			identity := ops.currentIdentity(child)
			if desiredKeys[identity.id] || desiredKeys[identity.name] {
				continue
			}
			if err := p.validateProtection(resourceType, identity.name, ops.protected(child), ActionDelete); err != nil {
				return err
			}
			ops.remove(child)
		}
	}
	return nil
}
