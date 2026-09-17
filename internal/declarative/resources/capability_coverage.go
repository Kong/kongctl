package resources

import (
	"fmt"
	"slices"
)

// ValidateManagedResourceCoverage checks that a consumer covers exactly the
// kinds opted into managed sync scope. External-only and decK-backed
// declarations without that scope remain outside SDK resource execution.
func ValidateManagedResourceCoverage(consumer string, kinds []ResourceType) error {
	return validateManagedCoverage(consumer, kinds, false)
}

// ValidateManagedRootCoverage checks that a consumer accounts for every
// managed root, either with an implementation or an explicit omission.
func ValidateManagedRootCoverage(consumer string, kinds []ResourceType) error {
	return validateManagedCoverage(consumer, kinds, true)
}

func validateManagedCoverage(consumer string, kinds []ResourceType, rootsOnly bool) error {
	expected := make(map[ResourceType]bool)
	for kind, ops := range registry {
		if scope := ops.syncScope; scope != nil && (!rootsOnly || scope.parentType == "") {
			expected[kind] = false
		}
	}
	// Stable diagnostics even when the consumer assembles kinds from a map.
	kinds = slices.Sorted(slices.Values(kinds))
	for _, kind := range kinds {
		seen, ok := expected[kind]
		if !ok {
			return fmt.Errorf("%s registers unexpected resource type %s", consumer, kind)
		}
		if seen {
			return fmt.Errorf("%s registers resource type %s more than once", consumer, kind)
		}
		expected[kind] = true
	}
	var missing []ResourceType
	for kind, seen := range expected {
		if !seen {
			missing = append(missing, kind)
		}
	}
	slices.Sort(missing)
	if len(missing) != 0 {
		return fmt.Errorf("%s is missing resource types %v", consumer, missing)
	}
	return nil
}
