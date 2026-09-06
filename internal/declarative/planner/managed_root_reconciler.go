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
	create           func(D, *Plan)
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

	protectionErrors := &ProtectionErrorCollector{}
	planDelete := func(root managedRoot[C]) {
		err := base.ValidateProtection(resourceType, root.name, root.protected, ActionDelete)
		protectionErrors.Add(err)
		if err == nil {
			operations.remove(root.resource, plan)
		}
	}

	if plan.Metadata.Mode == PlanModeDelete {
		for _, root := range desired {
			observed, exists := currentByName[root.name]
			if !exists {
				plan.AddWarning("", fmt.Sprintf("%s %q not found in Konnect, skipping delete", resourceType, root.name))
				continue
			}
			planDelete(observed)
		}
		return protectionErrors.Error()
	}

	desiredNames := make(map[string]bool, len(desired))
	for _, root := range desired {
		desiredNames[root.name] = true
		observed, exists := currentByName[root.name]
		if !exists {
			operations.create(root.resource, plan)
			continue
		}

		needsUpdate, updateFields, changedFields := operations.diff(observed.resource, root.resource)
		if observed.protected != root.protected {
			protectionChange := &ProtectionChange{Old: observed.protected, New: root.protected}
			err := base.ValidateProtectionWithChange(
				resourceType, root.name, observed.protected, ActionUpdate, protectionChange, needsUpdate,
			)
			protectionErrors.Add(err)
			if err == nil {
				operations.changeProtection(
					observed.resource,
					root.resource,
					observed.protected,
					root.protected,
					updateFields,
					changedFields,
					plan,
				)
			}
			continue
		}

		if needsUpdate {
			// Preserve the existing diff error contract for ordinary updates.
			if errMsg, hasError := updateFields[FieldError].(string); hasError {
				protectionErrors.Add(fmt.Errorf("%s", errMsg))
			} else {
				err := base.ValidateProtection(resourceType, root.name, observed.protected, ActionUpdate)
				protectionErrors.Add(err)
				if err == nil {
					operations.update(observed.resource, root.resource, updateFields, changedFields, plan)
				}
			}
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		for name, root := range currentByName {
			if !desiredNames[name] {
				planDelete(root)
			}
		}
	}

	return protectionErrors.Error()
}
