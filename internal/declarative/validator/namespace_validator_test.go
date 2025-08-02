package validator

import (
	"strings"
	"testing"
)

func TestValidateNamespace(t *testing.T) {
	validator := NewNamespaceValidator()
	
	tests := []struct {
		name      string
		namespace string
		wantErr   bool
		errMsg    string
	}{
		// Valid cases
		{
			name:      "simple lowercase",
			namespace: "default",
			wantErr:   false,
		},
		{
			name:      "with hyphens",
			namespace: "my-team-namespace",
			wantErr:   false,
		},
		{
			name:      "with numbers",
			namespace: "team123",
			wantErr:   false,
		},
		{
			name:      "single character",
			namespace: "a",
			wantErr:   false,
		},
		{
			name:      "max length",
			namespace: strings.Repeat("a", 63),
			wantErr:   false,
		},
		{
			name:      "starts with number",
			namespace: "123team",
			wantErr:   false,
		},
		{
			name:      "complex valid",
			namespace: "team-123-prod-v2",
			wantErr:   false,
		},
		
		// Invalid cases
		{
			name:      "empty string",
			namespace: "",
			wantErr:   true,
			errMsg:    "namespace cannot be empty",
		},
		{
			name:      "uppercase letters",
			namespace: "MyNamespace",
			wantErr:   true,
			errMsg:    "must consist of lowercase alphanumeric",
		},
		{
			name:      "starts with hyphen",
			namespace: "-namespace",
			wantErr:   true,
			errMsg:    "must start and end with an alphanumeric character",
		},
		{
			name:      "ends with hyphen",
			namespace: "namespace-",
			wantErr:   true,
			errMsg:    "must start and end with an alphanumeric character",
		},
		{
			name:      "contains underscore",
			namespace: "my_namespace",
			wantErr:   true,
			errMsg:    "must consist of lowercase alphanumeric",
		},
		{
			name:      "contains space",
			namespace: "my namespace",
			wantErr:   true,
			errMsg:    "must consist of lowercase alphanumeric",
		},
		{
			name:      "contains special chars",
			namespace: "my.namespace",
			wantErr:   true,
			errMsg:    "must consist of lowercase alphanumeric",
		},
		{
			name:      "double hyphen",
			namespace: "my--namespace",
			wantErr:   true,
			errMsg:    "cannot contain consecutive hyphens",
		},
		{
			name:      "exceeds max length",
			namespace: strings.Repeat("a", 64),
			wantErr:   true,
			errMsg:    "exceeds maximum length of 63 characters",
		},
		{
			name:      "just hyphens",
			namespace: "---",
			wantErr:   true,
			errMsg:    "must start and end with an alphanumeric character",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateNamespace(tt.namespace)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateNamespace() expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateNamespace() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateNamespace() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateNamespaces(t *testing.T) {
	validator := NewNamespaceValidator()
	
	tests := []struct {
		name       string
		namespaces []string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "all valid",
			namespaces: []string{"default", "team-a", "team-b", "platform"},
			wantErr:    false,
		},
		{
			name:       "with duplicates (allowed)",
			namespaces: []string{"default", "team-a", "default", "team-a"},
			wantErr:    false,
		},
		{
			name:       "one invalid",
			namespaces: []string{"default", "INVALID", "team-a"},
			wantErr:    true,
			errMsg:     "must consist of lowercase alphanumeric",
		},
		{
			name:       "empty list",
			namespaces: []string{},
			wantErr:    false,
		},
		{
			name:       "single namespace",
			namespaces: []string{"default"},
			wantErr:    false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateNamespaces(tt.namespaces)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateNamespaces() expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateNamespaces() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateNamespaces() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestReservedNamespaces(t *testing.T) {
	// Test that reserved namespaces structure is properly initialized
	if ReservedNamespaces == nil {
		t.Error("ReservedNamespaces should not be nil")
	}
	
	// Currently no reserved namespaces, but test the mechanism
	originalReserved := ReservedNamespaces
	defer func() {
		ReservedNamespaces = originalReserved
	}()
	
	// Add a test reserved namespace
	ReservedNamespaces = map[string]bool{
		"system": true,
		"kube":   true,
	}
	
	validator := NewNamespaceValidator()
	
	tests := []struct {
		namespace string
		wantErr   bool
	}{
		{"system", true},
		{"kube", true},
		{"user", false},
		{"default", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			err := validator.ValidateNamespace(tt.namespace)
			if tt.wantErr && err == nil {
				t.Errorf("Expected error for reserved namespace %s", tt.namespace)
			} else if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error for namespace %s: %v", tt.namespace, err)
			}
		})
	}
}