package planner

import "fmt"

// managedRoot contains only the identity and protection state needed for
// reconciliation. Resource-specific fields stay in the original typed value.
type managedRoot[T any] struct {
	resource  T
	name      string
	protected bool
}

// managedRootOperations keeps diffing and change construction with each resource
// planner. The reconciler owns the lifecycle for managed roots matched by name;
// resource discovery, external references, and child lifecycles stay outside it.
type managedRootOperations[D, C any] struct {
	diff             func(C, D) (bool, map[string]any, map[string]FieldChange)
	create           func(D, *Plan) string
	update           func(C, D, map[string]any, map[string]FieldChange, *Plan)
	changeProtection func(C, D, bool, bool, map[string]any, map[string]FieldChange, *Plan)
	remove           func(C, *Plan)
}

func reconcileManagedRoots[D, C any](
	base *BasePlanner,
	resourceType string,
	desired []managedRoot[D],
	current []managedRoot[C],
	operations managedRootOperations[D, C],
	plan *Plan,
) error {
	currentByName := make(map[string]managedRoot[C], len(current))
	for _, root := range current {
		currentByName[root.name] = root
	}

	reconciler := newManagedRootReconciler(base, resourceType, operations, plan)
	if plan.Metadata.Mode == PlanModeDelete {
		for _, root := range desired {
			reconciler.deleteDesired(root.name, findManagedRoot(currentByName, root.name))
		}
		return reconciler.errors.Error()
	}

	desiredNames := make(map[string]bool, len(desired))
	for _, root := range desired {
		desiredNames[root.name] = true
		reconciler.reconcile(root, findManagedRoot(currentByName, root.name))
	}
	if plan.Metadata.Mode == PlanModeSync {
		for name, root := range currentByName {
			if !desiredNames[name] {
				reconciler.remove(root)
			}
		}
	}
	return reconciler.errors.Error()
}

// managedRootReconciler owns per-root lifecycle decisions and accumulates
// protection errors. Traversal stays with its caller so child planning can run
// after no-ops or blocked updates, and child errors can abort before pruning.
type managedRootReconciler[D, C any] struct {
	base         *BasePlanner
	resourceType string
	operations   managedRootOperations[D, C]
	plan         *Plan
	errors       ProtectionErrorCollector
}

func newManagedRootReconciler[D, C any](
	base *BasePlanner,
	resourceType string,
	operations managedRootOperations[D, C],
	plan *Plan,
) *managedRootReconciler[D, C] {
	return &managedRootReconciler[D, C]{
		base: base, resourceType: resourceType, operations: operations, plan: plan,
	}
}

func findManagedRoot[C any](currentByName map[string]managedRoot[C], name string) *managedRoot[C] {
	if root, exists := currentByName[name]; exists {
		return &root
	}
	return nil
}

// reconcile returns a creation change ID only for a new root. The caller uses
// it for child dependencies; updates and no-ops keep routing by the observed ID.
func (r *managedRootReconciler[D, C]) reconcile(desired managedRoot[D], current *managedRoot[C]) string {
	if current == nil {
		return r.operations.create(desired.resource, r.plan)
	}
	needsUpdate, updateFields, changedFields := r.operations.diff(current.resource, desired.resource)
	if current.protected != desired.protected {
		change := &ProtectionChange{Old: current.protected, New: desired.protected}
		err := r.base.ValidateProtectionWithChange(
			r.resourceType, desired.name, current.protected, ActionUpdate, change, needsUpdate,
		)
		r.errors.Add(err)
		if err == nil {
			r.operations.changeProtection(
				current.resource, desired.resource, current.protected, desired.protected,
				updateFields, changedFields, r.plan,
			)
		}
	} else if needsUpdate {
		// Preserve the existing diff error contract for ordinary updates.
		if errMsg, hasError := updateFields[FieldError].(string); hasError {
			r.errors.Add(fmt.Errorf("%s", errMsg))
		} else {
			err := r.base.ValidateProtection(r.resourceType, desired.name, current.protected, ActionUpdate)
			r.errors.Add(err)
			if err == nil {
				r.operations.update(current.resource, desired.resource, updateFields, changedFields, r.plan)
			}
		}
	}
	return ""
}

func (r *managedRootReconciler[D, C]) deleteDesired(name string, current *managedRoot[C]) {
	if current == nil {
		r.plan.AddWarning("", fmt.Sprintf("%s %q not found in Konnect, skipping delete", r.resourceType, name))
		return
	}
	r.remove(*current)
}

func (r *managedRootReconciler[D, C]) remove(current managedRoot[C]) {
	err := r.base.ValidateProtection(r.resourceType, current.name, current.protected, ActionDelete)
	r.errors.Add(err)
	if err == nil {
		r.operations.remove(current.resource, r.plan)
	}
}
