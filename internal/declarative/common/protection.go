package common

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
)

// ValidateResourceProtection validates that protected resources are not being modified inappropriately
func ValidateResourceProtection(
	resourceType, resourceName string, isProtected bool, 
	change planner.PlannedChange, isProtectionChange bool,
) error {
	// Block protected resources unless it's a protection change
	if isProtected && !isProtectionChange && 
		(change.Action == planner.ActionUpdate || change.Action == planner.ActionDelete) {
		return fmt.Errorf("resource '%s' (%s) is protected and cannot be %s", 
			resourceName, resourceType, actionToVerb(change.Action))
	}
	return nil
}

// IsProtectionChange checks if the change is specifically updating the protection status
func IsProtectionChange(protection any) bool {
	switch p := protection.(type) {
	case planner.ProtectionChange:
		return true
	case map[string]any:
		// From JSON deserialization
		if _, hasOld := p["old"].(bool); hasOld {
			if _, hasNew := p["new"].(bool); hasNew {
				return true
			}
		}
	}
	return false
}

// GetProtectionStatus extracts the protection status from normalized labels
func GetProtectionStatus(normalizedLabels map[string]string) bool {
	return normalizedLabels != nil && normalizedLabels["KONGCTL/protected"] == "true"
}

// FormatProtectionError creates a standardized error message for protection violations
func FormatProtectionError(resourceType, resourceName, action string) error {
	return fmt.Errorf("resource '%s' (%s) is protected and cannot be %s", 
		resourceName, resourceType, actionToStringVerb(action))
}

// actionToVerb converts an ActionType to a past-tense verb for error messages
func actionToVerb(action planner.ActionType) string {
	switch action {
	case planner.ActionCreate:
		return "created"
	case planner.ActionUpdate:
		return "updated"
	case planner.ActionDelete:
		return "deleted"
	default:
		return string(action)
	}
}

// actionToStringVerb converts a string action to a past-tense verb for error messages
func actionToStringVerb(action string) string {
	switch action {
	case "create":
		return "created"
	case "update":
		return "updated"
	case "delete":
		return "deleted"
	default:
		return action
	}
}