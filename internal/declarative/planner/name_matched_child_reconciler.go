package planner

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
)

// nameMatchedChildOperations keeps observation, comparison, and change construction
// typed. Parent scope and creation of children under new parents stay with callers.
type nameMatchedChildOperations[D, C any] struct {
	desiredOrder      func([]D) []D
	ownership         *ownedChildCollectionScope
	ownershipMetadata func(C) (map[string]string, error)
	desiredName       func(D) string
	currentName       func(C) string
	fetch             func(D, C) (*C, error)
	diff              func(C, D) (bool, map[string]any, map[string]FieldChange, error)
	create            func(D)
	update            func(C, D, map[string]any, map[string]FieldChange)
	protected         func(C) bool
	remove            func(C)
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
			ownership: ops.ownership, ownershipMetadata: ops.ownershipMetadata, desiredOrder: ops.desiredOrder,
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
					if ops.ownership != nil {
						return fmt.Errorf(
							"%s disappeared during planning; retry planning",
							resourceType,
						)
					}
					ops.create(child)
					return nil
				}
				var metadata map[string]string
				var meta *resources.KongctlMeta
				if ops.ownership != nil {
					meta, err = ownedDesiredMetadata(&child)
					if err != nil {
						return err
					}
					if ops.ownershipMetadata != nil {
						metadata, err = ops.ownershipMetadata(*full)
					} else {
						metadata, err = childObservationMetadata(*full)
					}
					if err != nil {
						return err
					}
					if err := resources.ValidateChildOwnership(resourceType, metadata, resources.GetNamespace(meta)); err != nil {
						return err
					}
				}
				p.recordMatchedIdentity(&child, full)
				needsUpdate, fields, changed, err := ops.diff(*full, child)
				if err != nil {
					return err
				}
				if needsUpdate {
					if ops.ownership != nil {
						if err := validateOwnedPayloadChange(
							fmt.Sprintf("%s %q", resourceType, ops.desiredName(child)), metadata, fields, changed,
						); err != nil {
							return err
						}
					}
					start := len(plan.Changes)
					ops.update(*observed, child, fields, changed)
					if ops.ownership != nil &&
						resources.IsChildProtected(metadata) != childMetaProtected(meta) {
						for i := start; i < len(plan.Changes); i++ {
							plan.Changes[i].Protection = ProtectionChange{
								Old: resources.IsChildProtected(metadata),
								New: childMetaProtected(meta),
							}
						}
					}
				}
				return nil
			},
			protected:  ops.protected,
			remove:     ops.remove,
			pruneOrder: ops.pruneOrder,
		}, plan)
}
