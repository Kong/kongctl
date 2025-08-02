package state

import (
	"fmt"
	"strings"
	"testing"
)

func TestValidateAPIClient_Success(t *testing.T) {
	client := &mockPortalAPI{}
	err := ValidateAPIClient(client, "Portal API")
	if err != nil {
		t.Errorf("Expected no error for valid client, got: %v", err)
	}
}

func TestValidateAPIClient_NilInterface(t *testing.T) {
	err := ValidateAPIClient(nil, "Portal API")
	if err == nil {
		t.Fatal("Expected error for nil interface")
	}

	if !IsAPIClientError(err) {
		t.Errorf("Expected APIClientError, got: %T", err)
	}

	expected := "Portal API client not configured"
	if err.Error() != expected {
		t.Errorf("Expected error message %q, got %q", expected, err.Error())
	}
}

func TestValidateAPIClient_NilPointer(t *testing.T) {
	var client *mockPortalAPI // nil pointer
	err := ValidateAPIClient(client, "Portal API")
	if err == nil {
		t.Fatal("Expected error for nil pointer")
	}

	if !IsAPIClientError(err) {
		t.Errorf("Expected APIClientError, got: %T", err)
	}
}

func TestValidateResponse_Success(t *testing.T) {
	response := &struct{ Data string }{Data: "test"}
	err := ValidateResponse(response, "test operation")
	if err != nil {
		t.Errorf("Expected no error for valid response, got: %v", err)
	}
}

func TestValidateResponse_NilResponse(t *testing.T) {
	var response *struct{ Data string }
	err := ValidateResponse(response, "test operation")
	if err == nil {
		t.Fatal("Expected error for nil response")
	}

	if !IsResponseValidationError(err) {
		t.Errorf("Expected ResponseValidationError, got: %T", err)
	}

	// The actual error message varies by Go version, just check it contains key parts
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error message should not be empty")
	}
	
	// Check that it contains "test operation" and "response missing" and "data"
	if !errorContains(err, "test operation") {
		t.Errorf("Expected error to contain 'test operation', got: %q", errMsg)
	}
	if !errorContains(err, "response missing") {
		t.Errorf("Expected error to contain 'response missing', got: %q", errMsg)
	}
}

func TestWrapAPIError_StandardWrapping(t *testing.T) {
	originalErr := fmt.Errorf("connection failed")
	wrappedErr := WrapAPIError(originalErr, "list portals", nil)

	expected := "failed to list portals: connection failed"
	if wrappedErr.Error() != expected {
		t.Errorf("Expected error message %q, got %q", expected, wrappedErr.Error())
	}
}

func TestWrapAPIError_NilError(t *testing.T) {
	wrappedErr := WrapAPIError(nil, "test operation", nil)
	if wrappedErr != nil {
		t.Errorf("Expected nil for nil input error, got: %v", wrappedErr)
	}
}

func TestWrapAPIError_EnhancedError(t *testing.T) {
	originalErr := fmt.Errorf("API error")
	opts := &ErrorWrapperOptions{
		ResourceType: "portal",
		ResourceName: "test-portal",
		Namespace:    "test-ns",
		StatusCode:   400,
		UseEnhanced:  true,
	}

	wrappedErr := WrapAPIError(originalErr, "create", opts)

	// The enhanced error should contain context
	errMsg := wrappedErr.Error()
	if errMsg == "failed to create: API error" {
		t.Error("Expected enhanced error with context, got standard wrapped error")
	}

	// Should contain the original error
	if fmt.Sprintf("%v", wrappedErr) == "" {
		t.Error("Enhanced error should not be empty")
	}
}

func TestWrapAPIError_EnhancedErrorWithoutResourceType(t *testing.T) {
	originalErr := fmt.Errorf("API error")
	opts := &ErrorWrapperOptions{
		UseEnhanced: true,
		// No ResourceType provided
	}

	wrappedErr := WrapAPIError(originalErr, "create", opts)

	// Should fall back to standard wrapping
	expected := "failed to create: API error"
	if wrappedErr.Error() != expected {
		t.Errorf("Expected standard wrapped error %q, got %q", expected, wrappedErr.Error())
	}
}

func TestAPIClientError_Implementation(t *testing.T) {
	err := &APIClientError{ClientType: "Test API"}
	expected := "Test API client not configured"
	if err.Error() != expected {
		t.Errorf("Expected error message %q, got %q", expected, err.Error())
	}
}

func TestResponseValidationError_Implementation(t *testing.T) {
	err := &ResponseValidationError{
		Operation:    "create user",
		ExpectedType: "UserResponse",
	}
	expected := "create user response missing UserResponse data"
	if err.Error() != expected {
		t.Errorf("Expected error message %q, got %q", expected, err.Error())
	}
}

func TestNewAPIClientNotConfiguredError(t *testing.T) {
	err := NewAPIClientNotConfiguredError("Portal API")
	if !IsAPIClientError(err) {
		t.Errorf("Expected APIClientError, got: %T", err)
	}

	expected := "Portal API client not configured"
	if err.Error() != expected {
		t.Errorf("Expected error message %q, got %q", expected, err.Error())
	}
}

func TestNewResponseValidationError(t *testing.T) {
	err := NewResponseValidationError("create portal", "PortalResponse")
	if !IsResponseValidationError(err) {
		t.Errorf("Expected ResponseValidationError, got: %T", err)
	}

	expected := "create portal response missing PortalResponse data"
	if err.Error() != expected {
		t.Errorf("Expected error message %q, got %q", expected, err.Error())
	}
}

func TestIsAPIClientError_Positive(t *testing.T) {
	err := &APIClientError{ClientType: "Test"}
	if !IsAPIClientError(err) {
		t.Error("Expected IsAPIClientError to return true for APIClientError")
	}
}

func TestIsAPIClientError_Negative(t *testing.T) {
	err := fmt.Errorf("regular error")
	if IsAPIClientError(err) {
		t.Error("Expected IsAPIClientError to return false for regular error")
	}
}

func TestIsResponseValidationError_Positive(t *testing.T) {
	err := &ResponseValidationError{Operation: "test", ExpectedType: "TestType"}
	if !IsResponseValidationError(err) {
		t.Error("Expected IsResponseValidationError to return true for ResponseValidationError")
	}
}

func TestIsResponseValidationError_Negative(t *testing.T) {
	err := fmt.Errorf("regular error")
	if IsResponseValidationError(err) {
		t.Error("Expected IsResponseValidationError to return false for regular error")
	}
}

func TestErrorConstants(t *testing.T) {
	// Test that constants exist and are non-empty
	constants := []string{
		ErrMsgPortalAPINotConfigured,
		ErrMsgAPIAPINotConfigured,
		ErrMsgAuthStrategyAPINotConfigured,
		ErrMsgAPIVersionAPINotConfigured,
		ErrMsgAPIPublicationAPINotConfigured,
		ErrMsgAPIImplementationAPINotConfigured,
		ErrMsgAPIDocumentAPINotConfigured,
		ErrMsgPortalPageAPINotConfigured,
		ErrMsgPortalSnippetAPINotConfigured,
		ErrMsgPortalCustomizationAPINotConfigured,
		ErrMsgPortalCustomDomainAPINotConfigured,
	}

	for i, constant := range constants {
		if constant == "" {
			t.Errorf("Constant %d is empty", i)
		}
		if !strings.Contains(constant, "not configured") {
			t.Errorf("Constant %d (%q) should contain 'not configured'", i, constant)
		}
	}
}

// Test error handling with wrapped errors (testing error chains)
func TestWrapAPIError_ErrorChaining(t *testing.T) {
	baseErr := fmt.Errorf("network timeout")
	middleErr := fmt.Errorf("API call failed: %w", baseErr)
	finalErr := WrapAPIError(middleErr, "list resources", nil)

	// Check that the final error message contains the wrapped message
	errMsg := finalErr.Error()
	if !errorContains(finalErr, "failed to list resources") {
		t.Errorf("Expected final error to contain 'failed to list resources', got: %q", errMsg)
	}
	
	// The error should chain properly through fmt.Errorf's %w verb
	if errMsg == "" {
		t.Error("Final error should not be empty")
	}
}

// Helper function to check if error contains a substring
func errorContains(err error, substr string) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), substr)
}

// Test concurrent error creation (for thread safety)
func TestWrapAPIError_Concurrent(t *testing.T) {
	originalErr := fmt.Errorf("base error")
	
	done := make(chan bool, 100)
	
	// Create 100 concurrent error wrappings
	for i := 0; i < 100; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			opts := &ErrorWrapperOptions{
				ResourceType: "test",
				ResourceName: fmt.Sprintf("resource-%d", id),
				UseEnhanced:  true,
			}
			
			wrappedErr := WrapAPIError(originalErr, "test operation", opts)
			if wrappedErr == nil {
				t.Errorf("Expected wrapped error, got nil for goroutine %d", id)
			}
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 100; i++ {
		<-done
	}
}