package executor

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
)

// Participating adapters call these checks on the actual write path, including
// direct calls and secret-only PUTs. Parent linkage supplies identity only.
func validateChildPayloadOwnership(kind string, metadata map[string]string, namespace string) error {
	if err := resources.ValidateChildManagedBy(metadata, false); err != nil {
		return err
	}
	if err := resources.ValidateChildOwnership(kind, metadata, namespace); err != nil {
		return err
	}
	if value := metadata[resources.ChildProtectedKey]; value != "true" && value != "false" {
		return fmt.Errorf("%s payload requires generated protection metadata; regenerate the plan", kind)
	}
	return nil
}

// A protection-only plan must also pass adapter-specific public payload and
// secret-write checks. ChangedFields alone is not evidence of unchanged data.
func childUnprotectOnly(change *planner.PlannedChange, current, desired map[string]string) bool {
	if change == nil || len(change.SecretWrites) != 0 || len(change.ChangedFields) != 1 ||
		change.ChangedFields[planner.FieldManagedBy].New == nil || desired[resources.ChildProtectedKey] != "false" {
		return false
	}
	return resources.ChildUnprotectionMetadataOnly(current, desired)
}

// Saved plans pin protection intent as well as namespace. Reject a protection
// transition observed since planning, before even an unprotect-only write.
func validateChildPlannedProtection(kind string, current map[string]string, change *planner.PlannedChange) error {
	if change == nil {
		return fmt.Errorf("%s execution context required", kind)
	}
	if protection, ok := change.Protection.(planner.ProtectionChange); ok {
		if protection.Old != (current[resources.ChildProtectedKey] == "true") {
			return fmt.Errorf("%s protection changed after planning; regenerate the plan", kind)
		}
	}
	if protection, ok := change.Protection.(map[string]any); ok {
		if old, valid := protection["old"].(bool); valid && old != (current[resources.ChildProtectedKey] == "true") {
			return fmt.Errorf("%s protection changed after planning; regenerate the plan", kind)
		}
	}
	return nil
}

func childCreationCollision(kind, name string) error {
	return fmt.Errorf("%s %q already exists; replan and explicitly adopt it if unowned", kind, name)
}
