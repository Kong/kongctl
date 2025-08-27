package errors

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// APIErrorContext provides context for API-related errors
type APIErrorContext struct {
	ResourceType string
	ResourceName string
	Namespace    string
	Operation    string
	StatusCode   int
	ResponseBody string
}

// IsConflictError checks if an error indicates a resource conflict (name already exists)
func IsConflictError(err error, statusCode int) bool {
	if statusCode == http.StatusConflict {
		return true
	}

	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "already exists") ||
		strings.Contains(errMsg, "name is not unique") ||
		strings.Contains(errMsg, "duplicate") ||
		strings.Contains(errMsg, "conflict")
}

// IsValidationError checks if an error indicates validation failure
func IsValidationError(err error, statusCode int) bool {
	if statusCode == http.StatusUnprocessableEntity || statusCode == http.StatusBadRequest {
		return true
	}

	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "validation") ||
		strings.Contains(errMsg, "invalid") ||
		strings.Contains(errMsg, "required")
}

// IsAuthError checks if an error indicates authentication/authorization failure
func IsAuthError(_ error, statusCode int) bool {
	return statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden
}

// IsRateLimitError checks if an error indicates rate limiting
func IsRateLimitError(_ error, statusCode int) bool {
	return statusCode == http.StatusTooManyRequests
}

// EnhanceAPIError enhances an API error with context and helpful hints
func EnhanceAPIError(err error, ctx APIErrorContext) error {
	if err == nil {
		return nil
	}

	// Build base error message with resource context
	var baseMsg string
	if ctx.ResourceName != "" {
		if ctx.Namespace != "" && ctx.Namespace != "*" {
			baseMsg = fmt.Sprintf("failed to %s %s \"%s\" in namespace \"%s\"",
				ctx.Operation, ctx.ResourceType, ctx.ResourceName, ctx.Namespace)
		} else {
			baseMsg = fmt.Sprintf("failed to %s %s \"%s\"",
				ctx.Operation, ctx.ResourceType, ctx.ResourceName)
		}
	} else {
		baseMsg = fmt.Sprintf("failed to %s %s", ctx.Operation, ctx.ResourceType)
	}

	// Add status code if available
	var statusInfo string
	if ctx.StatusCode > 0 {
		statusInfo = fmt.Sprintf(" (HTTP %d)", ctx.StatusCode)
	}

	// Generate helpful hints based on error type
	hint := generateHint(err, ctx)

	if hint != "" {
		return fmt.Errorf("%s%s: %w. %s", baseMsg, statusInfo, err, hint)
	}

	return fmt.Errorf("%s%s: %w", baseMsg, statusInfo, err)
}

// generateHint provides actionable hints based on the error context
func generateHint(err error, ctx APIErrorContext) string {
	// Handle conflict errors (name already exists)
	if IsConflictError(err, ctx.StatusCode) {
		switch ctx.ResourceType {
		case "portal":
			return fmt.Sprintf("Portal names must be unique across the entire Konnect organization "+
				"(including portals not managed by kongctl). Try using a different name like \"%s-v2\" "+
				"or check existing portals with 'kongctl get portals'", ctx.ResourceName)
		case "api":
			return fmt.Sprintf("API names must be unique within your organization. "+
				"Try using a different name like \"%s-v2\" or check existing APIs with 'kongctl get apis'", ctx.ResourceName)
		case "auth-strategy":
			return fmt.Sprintf("Auth strategy names must be unique. "+
				"Try using a different name like \"%s-v2\" or check existing strategies with 'kongctl get auth-strategies'",
				ctx.ResourceName)
		default:
			return "Resource names must be unique. Try using a different name or check existing resources"
		}
	}

	// Handle validation errors
	if IsValidationError(err, ctx.StatusCode) {
		switch ctx.ResourceType {
		case "portal":
			return "Check your portal configuration for required fields (name) and valid values " +
				"for optional fields (authentication_enabled, rbac_enabled, etc.)"
		case "api":
			return "Check your API configuration for required fields (name, version) and ensure version format is valid"
		case "auth-strategy":
			return "Check your auth strategy configuration for required fields and ensure the strategy type is valid"
		default:
			return "Check your resource configuration for required fields and valid values"
		}
	}

	// Handle authentication/authorization errors
	if IsAuthError(err, ctx.StatusCode) {
		if ctx.StatusCode == http.StatusUnauthorized {
			return "Check your authentication token with 'kongctl login' and ensure it hasn't expired"
		}
		return "Check that your account has permission to manage this resource type in Konnect"
	}

	// Handle rate limiting errors
	if IsRateLimitError(err, ctx.StatusCode) {
		return "API rate limit exceeded. Wait a moment and try again, or reduce the number of concurrent operations"
	}

	// Handle server errors
	if ctx.StatusCode >= 500 {
		return "Konnect API is experiencing issues. Check the Konnect status page and try again later"
	}

	return ""
}

// ExtractStatusCodeFromError attempts to extract HTTP status code from an error
func ExtractStatusCodeFromError(err error) int {
	if err == nil {
		return 0
	}

	errMsg := err.Error()

	// Look for common status code patterns
	patterns := []string{
		"status code: ",
		"HTTP ",
		"status ",
	}

	for _, pattern := range patterns {
		if idx := strings.Index(errMsg, pattern); idx != -1 {
			start := idx + len(pattern)
			end := start
			for end < len(errMsg) && errMsg[end] >= '0' && errMsg[end] <= '9' {
				end++
			}
			if end > start {
				if code, err := strconv.Atoi(errMsg[start:end]); err == nil {
					return code
				}
			}
		}
	}

	return 0
}
