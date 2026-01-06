package decerrors

import (
	"fmt"
	"strings"
)

// FormatValidationError formats validation errors with clear context
func FormatValidationError(resourceType, resourceName, field, issue string) error {
	if resourceName != "" {
		return fmt.Errorf("validation failed for %s \"%s\": %s %s", resourceType, resourceName, field, issue)
	}
	return fmt.Errorf("validation failed for %s: %s %s", resourceType, field, issue)
}

// FormatDependencyError formats dependency resolution errors
func FormatDependencyError(resourceType, resourceName, dependencyType, dependencyRef string) error {
	return fmt.Errorf("%s \"%s\" references unknown %s \"%s\". Ensure the %s exists in your configuration",
		resourceType, resourceName, dependencyType, dependencyRef, dependencyType)
}

// FormatProtectionError formats protection violation errors
func FormatProtectionError(resourceType, resourceName, operation string) error {
	return fmt.Errorf("cannot %s %s \"%s\": resource is protected. "+
		"Remove the 'protected: true' setting to allow modifications",
		operation, resourceType, resourceName)
}

// FormatConfigurationError formats configuration parsing errors with file context
func FormatConfigurationError(filePath string, lineNumber int, issue string) error {
	if lineNumber > 0 {
		return fmt.Errorf("configuration error in %s (line %d): %s", filePath, lineNumber, issue)
	}
	return fmt.Errorf("configuration error in %s: %s", filePath, issue)
}

// FormatNetworkError formats network-related errors with retry suggestions
func FormatNetworkError(operation, resourceType, resourceName string, err error) error {
	baseMsg := fmt.Sprintf("network error during %s of %s", operation, resourceType)
	if resourceName != "" {
		baseMsg = fmt.Sprintf("network error during %s of %s \"%s\"", operation, resourceType, resourceName)
	}

	errMsg := strings.ToLower(err.Error())
	if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline") {
		return fmt.Errorf("%s: %w. The operation timed out - check your network connection and try again", baseMsg, err)
	}
	if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "no such host") {
		return fmt.Errorf("%s: %w. Cannot connect to Konnect API - "+
			"check your network connection and DNS resolution", baseMsg, err)
	}

	return fmt.Errorf("%s: %w. Check your network connection and try again", baseMsg, err)
}
