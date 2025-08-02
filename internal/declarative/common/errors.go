package common

import (
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/declarative/planner"
)

// FormatExecutionError creates a standardized error message for execution failures
func FormatExecutionError(resourceType, resourceName, action, errorMsg string) error {
	return fmt.Errorf("failed to %s %s '%s': %s", 
		actionToExecutionVerb(action), resourceType, resourceName, errorMsg)
}

// FormatValidationError creates a standardized error message for validation failures
func FormatValidationError(resourceType, resourceName, reason string) error {
	return fmt.Errorf("validation failed for %s '%s': %s", 
		resourceType, resourceName, reason)
}

// FormatResourceNotFoundError creates a standardized error for missing resources
func FormatResourceNotFoundError(resourceType, resourceName string) error {
	return fmt.Errorf("%s '%s' not found", resourceType, resourceName)
}

// FormatResourceExistsError creates a standardized error for resources that already exist
func FormatResourceExistsError(resourceType, resourceName string) error {
	return fmt.Errorf("%s '%s' already exists", resourceType, resourceName)
}

// FormatAPIError wraps API errors with resource context
func FormatAPIError(resourceType, resourceName, operation string, err error) error {
	if err == nil {
		return nil
	}
	
	// Check if it's already a formatted error to avoid double wrapping
	errStr := err.Error()
	if strings.Contains(errStr, resourceType) && strings.Contains(errStr, resourceName) {
		return err
	}
	
	return fmt.Errorf("API error during %s of %s '%s': %w", 
		operation, resourceType, resourceName, err)
}

// FormatDependencyError creates an error for dependency resolution failures
func FormatDependencyError(resourceType, resourceName, dependencyType, dependencyRef string) error {
	return fmt.Errorf("failed to resolve %s dependency '%s' for %s '%s'", 
		dependencyType, dependencyRef, resourceType, resourceName)
}

// WrapWithResourceContext adds resource context to an existing error
func WrapWithResourceContext(err error, resourceType, resourceName string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s '%s': %w", resourceType, resourceName, err)
}

// ExtractResourceInfoFromChange gets resource type and name from a planned change
func ExtractResourceInfoFromChange(change planner.PlannedChange) (resourceType, resourceName string) {
	resourceType = change.ResourceType
	resourceName = change.ResourceRef
	
	// Try to get a better name from fields if ref is generic
	if resourceName == "" || resourceName == "[unknown]" {
		if change.Fields != nil {
			resourceName = ExtractResourceName(change.Fields)
		}
	}
	
	return resourceType, resourceName
}

// actionToExecutionVerb converts an action string to a verb for execution error messages
func actionToExecutionVerb(action string) string {
	switch strings.ToLower(action) {
	case "create":
		return "create"
	case "update":
		return "update"
	case "delete":
		return "delete"
	default:
		return "process"
	}
}