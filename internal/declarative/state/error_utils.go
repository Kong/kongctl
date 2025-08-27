package state

import (
	"errors"
	"fmt"
	"reflect"

	kErrors "github.com/kong/kongctl/internal/declarative/errors"
)

// APIClientError represents an error when an API client is not configured
type APIClientError struct {
	ClientType string
}

func (e *APIClientError) Error() string {
	return fmt.Sprintf("%s client not configured", e.ClientType)
}

// ValidateAPIClient checks if an API client interface is configured (not nil)
func ValidateAPIClient(client any, clientType string) error {
	if client == nil || reflect.ValueOf(client).IsNil() {
		return &APIClientError{ClientType: clientType}
	}
	return nil
}

// ResponseValidationError represents an error when API response is missing expected data
type ResponseValidationError struct {
	Operation    string
	ExpectedType string
}

func (e *ResponseValidationError) Error() string {
	return fmt.Sprintf("%s response missing %s data", e.Operation, e.ExpectedType)
}

// ValidateResponse checks if a response pointer contains non-nil data
func ValidateResponse[T any](response *T, operation string) error {
	if response == nil {
		return &ResponseValidationError{
			Operation:    operation,
			ExpectedType: reflect.TypeOf((*T)(nil)).Elem().Name(),
		}
	}
	return nil
}

// ErrorWrapperOptions configures how API errors are wrapped
type ErrorWrapperOptions struct {
	ResourceType string
	ResourceName string
	Namespace    string
	StatusCode   int
	UseEnhanced  bool // Whether to use enhanced error with context
}

// WrapAPIError wraps an API error with consistent formatting and optional enhancement
func WrapAPIError(err error, operation string, opts *ErrorWrapperOptions) error {
	if err == nil {
		return nil
	}

	// If enhanced error requested and we have context
	if opts != nil && opts.UseEnhanced && opts.ResourceType != "" {
		// Extract status code from error if not provided
		statusCode := opts.StatusCode
		if statusCode == 0 {
			statusCode = kErrors.ExtractStatusCodeFromError(err)
		}

		// Create enhanced error with context and hints
		ctx := kErrors.APIErrorContext{
			ResourceType: opts.ResourceType,
			ResourceName: opts.ResourceName,
			Namespace:    opts.Namespace,
			Operation:    operation,
			StatusCode:   statusCode,
		}

		return kErrors.EnhanceAPIError(err, ctx)
	}

	// Standard error wrapping
	return fmt.Errorf("failed to %s: %w", operation, err)
}

// Common error creation helpers

// NewAPIClientNotConfiguredError creates a standard "client not configured" error
func NewAPIClientNotConfiguredError(clientType string) error {
	return &APIClientError{ClientType: clientType}
}

// NewResponseValidationError creates a standard response validation error
func NewResponseValidationError(operation, expectedType string) error {
	return &ResponseValidationError{
		Operation:    operation,
		ExpectedType: expectedType,
	}
}

// IsAPIClientError checks if an error is an API client not configured error
func IsAPIClientError(err error) bool {
	var apiErr *APIClientError
	return errors.As(err, &apiErr)
}

// IsResponseValidationError checks if an error is a response validation error
func IsResponseValidationError(err error) bool {
	var validationErr *ResponseValidationError
	return errors.As(err, &validationErr)
}

// Standard error messages for common scenarios
const (
	ErrMsgPortalAPINotConfigured              = "Portal API client not configured"
	ErrMsgAPIAPINotConfigured                 = "API client not configured"
	ErrMsgAuthStrategyAPINotConfigured        = "app auth API client not configured"
	ErrMsgAPIVersionAPINotConfigured          = "API version client not configured"
	ErrMsgAPIPublicationAPINotConfigured      = "API publication client not configured"
	ErrMsgAPIImplementationAPINotConfigured   = "API implementation client not configured"
	ErrMsgAPIDocumentAPINotConfigured         = "API document client not configured"
	ErrMsgPortalPageAPINotConfigured          = "portal page API not configured"
	ErrMsgPortalSnippetAPINotConfigured       = "portal snippet API not configured"
	ErrMsgPortalCustomizationAPINotConfigured = "portal customization API not configured"
	ErrMsgPortalCustomDomainAPINotConfigured  = "portal custom domain API not configured"
)
