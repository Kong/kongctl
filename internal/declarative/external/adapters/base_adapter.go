package adapters

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"syscall"

	"github.com/kong/kongctl/internal/declarative/external"
	"github.com/kong/kongctl/internal/declarative/state"
)

// BaseAdapter provides common functionality for all resolution adapters
type BaseAdapter struct {
	client *state.Client
}

// NewBaseAdapter creates a new base adapter with state client
func NewBaseAdapter(client *state.Client) *BaseAdapter {
	return &BaseAdapter{client: client}
}

// ValidateParentContext validates parent context for child resources
func (b *BaseAdapter) ValidateParentContext(parent *external.ResolvedParent, expectedType string) error {
	if parent == nil {
		return fmt.Errorf("parent context required for child resource")
	}
	if parent.ResourceType != expectedType {
		return fmt.Errorf("invalid parent type: expected %s, got %s", expectedType, parent.ResourceType)
	}
	if parent.ID == "" {
		return fmt.Errorf("parent ID is required")
	}
	return nil
}

// FilterBySelector filters resources by selector fields and ensures exactly one match
func (b *BaseAdapter) FilterBySelector(resources []interface{}, selector map[string]string, 
	getField func(interface{}, string) string) (interface{}, error) {
	
	var matches []interface{}
	for _, resource := range resources {
		match := true
		for field, value := range selector {
			if getField(resource, field) != value {
				match = false
				break
			}
		}
		if match {
			matches = append(matches, resource)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no resources found matching selector: %v", selector)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("selector matched %d resources, expected 1: %v", len(matches), selector)
	}

	return matches[0], nil
}

// GetClient returns the state client for use by concrete adapters
func (b *BaseAdapter) GetClient() *state.Client {
	return b.client
}

// ClassifySDKError classifies an SDK error into a specific error type
func (b *BaseAdapter) ClassifySDKError(err error) external.SDKErrorType {
	if err == nil {
		return external.SDKErrorUnknown
	}
	
	errStr := err.Error()
	
	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return external.SDKErrorNetwork
	}
	
	// Check for URL errors (often network-related)
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return external.SDKErrorNetwork
	}
	
	// Check for syscall errors (connection refused, etc.)
	var syscallErr syscall.Errno
	if errors.As(err, &syscallErr) {
		return external.SDKErrorNetwork
	}
	
	// String-based classification for HTTP status codes and common error patterns
	switch {
	case strings.Contains(errStr, "401") || strings.Contains(errStr, "unauthorized"):
		return external.SDKErrorAuthentication
	case strings.Contains(errStr, "403") || strings.Contains(errStr, "forbidden"):
		return external.SDKErrorAuthorization
	case strings.Contains(errStr, "404") || strings.Contains(errStr, "not found"):
		return external.SDKErrorNotFound
	case strings.Contains(errStr, "400") || strings.Contains(errStr, "bad request"):
		return external.SDKErrorValidation
	case strings.Contains(errStr, "422") || strings.Contains(errStr, "unprocessable"):
		return external.SDKErrorValidation
	case strings.Contains(errStr, "500") || strings.Contains(errStr, "internal server"):
		return external.SDKErrorServerError
	case strings.Contains(errStr, "502") || strings.Contains(errStr, "bad gateway"):
		return external.SDKErrorServerError
	case strings.Contains(errStr, "503") || strings.Contains(errStr, "service unavailable"):
		return external.SDKErrorServerError
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded"):
		return external.SDKErrorNetwork
	case strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "no such host"):
		return external.SDKErrorNetwork
	case strings.Contains(errStr, "invalid token") || strings.Contains(errStr, "token expired"):
		return external.SDKErrorAuthentication
	default:
		return external.SDKErrorUnknown
	}
}

// TranslateSDKError translates an SDK error into a user-friendly external resource error
func (b *BaseAdapter) TranslateSDKError(err error, ref, resourceType, operation string) error {
	if err == nil {
		return nil
	}
	
	errorType := b.ClassifySDKError(err)
	
	var userMessage string
	var suggestions []string
	
	switch errorType {
	case external.SDKErrorNetwork:
		userMessage = "Network connection error while accessing Konnect"
		suggestions = []string{
			"Check your internet connection",
			"Verify Konnect API is reachable",
			"Check if you're behind a proxy or firewall",
			"Try again in a few moments",
		}
	case external.SDKErrorAuthentication:
		userMessage = "Authentication failed"
		suggestions = []string{
			"Verify your PAT is valid: kongctl login --pat YOUR_PAT",
			"Check if your token has expired",
			"Ensure you have the correct permissions",
		}
	case external.SDKErrorAuthorization:
		userMessage = "Access denied"
		suggestions = []string{
			"Verify you have permission to access this " + resourceType,
			"Check your role and permissions in Konnect",
			"Contact your administrator for access",
		}
	case external.SDKErrorNotFound:
		userMessage = "Resource not found in Konnect"
		suggestions = []string{
			"Verify the " + resourceType + " exists in Konnect",
			"Check if the resource was recently deleted",
			"Ensure you're targeting the correct environment",
		}
	case external.SDKErrorValidation:
		userMessage = "Invalid request"
		suggestions = []string{
			"Check the selector fields are valid for " + resourceType,
			"Verify the field values match the expected format",
			"Review the Konnect API documentation for this resource type",
		}
	case external.SDKErrorServerError:
		userMessage = "Konnect server error"
		suggestions = []string{
			"The Konnect service may be experiencing issues",
			"Try again in a few moments",
			"Check the Kong status page for any ongoing incidents",
		}
	case external.SDKErrorUnknown:
		userMessage = "Unexpected error occurred"
		suggestions = []string{
			"Check the error details below",
			"Enable debug logging for more information: --log-level debug",
		}
	}
	
	// Extract HTTP status if available
	httpStatus := 0
	errStr := err.Error()
	// Simple extraction of common HTTP status patterns
	if strings.Contains(errStr, "401") {
		httpStatus = 401
	} else if strings.Contains(errStr, "403") {
		httpStatus = 403
	} else if strings.Contains(errStr, "404") {
		httpStatus = 404
	} else if strings.Contains(errStr, "500") {
		httpStatus = 500
	}
	
	return &external.ResourceSDKError{
		Ref:          ref,
		ResourceType: resourceType,
		Operation:    operation,
		SDKErrorType: errorType,
		HTTPStatus:   httpStatus,
		Message:      err.Error(),
		UserMessage:  userMessage,
		Suggestions:  suggestions,
		Cause:        err,
	}
}

// WrapSDKError wraps an SDK error with context for external resource operations
func (b *BaseAdapter) WrapSDKError(err error, ref, resourceType, operation string) error {
	if err == nil {
		return nil
	}
	
	// Check if it's already a structured error
	var extErr *external.ResourceSDKError
	if errors.As(err, &extErr) {
		return err
	}
	
	return b.TranslateSDKError(err, ref, resourceType, operation)
}