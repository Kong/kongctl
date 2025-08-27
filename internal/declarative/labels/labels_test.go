package labels

import (
	"strings"
	"testing"
)

func TestNormalizeLabels(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]*string
		expected map[string]string
	}{
		{
			name:     "nil map",
			input:    nil,
			expected: map[string]string{},
		},
		{
			name:     "empty map",
			input:    map[string]*string{},
			expected: map[string]string{},
		},
		{
			name: "map with values",
			input: map[string]*string{
				"env":  ptr("production"),
				"team": ptr("platform"),
			},
			expected: map[string]string{
				"env":  "production",
				"team": "platform",
			},
		},
		{
			name: "map with nil values",
			input: map[string]*string{
				"env":  ptr("production"),
				"team": nil,
				"app":  ptr("api"),
			},
			expected: map[string]string{
				"env": "production",
				"app": "api",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeLabels(tt.input)
			if !mapsEqual(result, tt.expected) {
				t.Errorf("NormalizeLabels() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDenormalizeLabels(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]string
	}{
		{
			name:  "nil map",
			input: nil,
		},
		{
			name:  "empty map",
			input: map[string]string{},
		},
		{
			name: "map with values",
			input: map[string]string{
				"env":  "production",
				"team": "platform",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DenormalizeLabels(tt.input)

			// Verify structure
			if len(tt.input) == 0 && result == nil {
				return
			}
			if len(tt.input) > 0 && len(result) != len(tt.input) {
				t.Errorf("DenormalizeLabels() length = %d, want %d", len(result), len(tt.input))
			}

			// Verify values
			for k, v := range tt.input {
				if result[k] == nil || *result[k] != v {
					t.Errorf("DenormalizeLabels()[%s] = %v, want %s", k, result[k], v)
				}
			}
		})
	}
}

func TestAddManagedLabels(t *testing.T) {
	tests := []struct {
		name   string
		input  map[string]string
		verify func(t *testing.T, result map[string]string)
	}{
		{
			name:  "nil map",
			input: nil,
			verify: func(t *testing.T, result map[string]string) {
				checkManagedLabels(t, result, "")
				if len(result) != 1 { // only namespace
					t.Errorf("Expected 1 label, got %d", len(result))
				}
			},
		},
		{
			name: "existing labels preserved",
			input: map[string]string{
				"env":  "production",
				"team": "platform",
			},
			verify: func(t *testing.T, result map[string]string) {
				checkManagedLabels(t, result, "")
				if result["env"] != "production" {
					t.Errorf("Expected env=production, got %s", result["env"])
				}
				if result["team"] != "platform" {
					t.Errorf("Expected team=platform, got %s", result["team"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AddManagedLabels(tt.input, "default")
			tt.verify(t, result)
		})
	}
}

func TestIsManagedResource(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected bool
	}{
		{
			name:     "nil labels",
			labels:   nil,
			expected: false,
		},
		{
			name:     "empty labels",
			labels:   map[string]string{},
			expected: false,
		},
		{
			name: "managed resource with namespace",
			labels: map[string]string{
				NamespaceKey: "default",
			},
			expected: true,
		},
		{
			name: "unmanaged resource without namespace",
			labels: map[string]string{
				"env": "production",
			},
			expected: false,
		},
		{
			name: "resource with namespace",
			labels: map[string]string{
				NamespaceKey: "team-a",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsManagedResource(tt.labels)
			if result != tt.expected {
				t.Errorf("IsManagedResource() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetUserLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected map[string]string
	}{
		{
			name:     "nil labels",
			labels:   nil,
			expected: map[string]string{},
		},
		{
			name: "only user labels",
			labels: map[string]string{
				"env":  "production",
				"team": "platform",
			},
			expected: map[string]string{
				"env":  "production",
				"team": "platform",
			},
		},
		{
			name: "mixed labels",
			labels: map[string]string{
				"env":          "production",
				ManagedKey:     "true",
				"team":         "platform",
				LastUpdatedKey: "2024-01-01T00:00:00Z",
			},
			expected: map[string]string{
				"env":  "production",
				"team": "platform",
			},
		},
		{
			name: "only kongctl labels",
			labels: map[string]string{
				ManagedKey:     "true",
				LastUpdatedKey: "2024-01-01T00:00:00Z",
			},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetUserLabels(tt.labels)
			if !mapsEqual(result, tt.expected) {
				t.Errorf("GetUserLabels() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsKongctlLabel(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{
			name:     "kongctl label",
			key:      "KONGCTL-managed",
			expected: true,
		},
		{
			name:     "kongctl label with different suffix",
			key:      "KONGCTL-custom",
			expected: true,
		},
		{
			name:     "user label",
			key:      "env",
			expected: false,
		},
		{
			name:     "similar prefix",
			key:      "KONG/test",
			expected: false,
		},
		{
			name:     "empty string",
			key:      "",
			expected: false,
		},
		{
			name:     "short string",
			key:      "kongctl",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsKongctlLabel(tt.key)
			if result != tt.expected {
				t.Errorf("IsKongctlLabel(%s) = %v, want %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestValidateLabel(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid label",
			key:     "environment",
			wantErr: false,
		},
		{
			name:    "empty label",
			key:     "",
			wantErr: true,
			errMsg:  "label key must be 1-63 characters",
		},
		{
			name:    "too long label",
			key:     strings.Repeat("a", 64),
			wantErr: true,
			errMsg:  "label key must be 1-63 characters",
		},
		{
			name:    "forbidden prefix kong",
			key:     "kong-env",
			wantErr: true,
			errMsg:  "label key cannot start with kong",
		},
		{
			name:    "forbidden prefix konnect",
			key:     "konnect-app",
			wantErr: true,
			errMsg:  "label key cannot start with konnect",
		},
		{
			name:    "forbidden prefix mesh",
			key:     "mesh-service",
			wantErr: true,
			errMsg:  "label key cannot start with mesh",
		},
		{
			name:    "forbidden prefix kic",
			key:     "kic-ingress",
			wantErr: true,
			errMsg:  "label key cannot start with kic",
		},
		{
			name:    "forbidden prefix underscore",
			key:     "_internal",
			wantErr: true,
			errMsg:  "label key cannot start with _",
		},
		{
			name:    "similar but valid",
			key:     "my-kong-app",
			wantErr: false,
		},
		{
			name:    "kongctl prefix allowed",
			key:     "kongctl-managed",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLabel(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateLabel(%s) error = %v, wantErr %v", tt.key, err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateLabel(%s) error = %v, want error containing %s", tt.key, err, tt.errMsg)
			}
		})
	}
}

// Helper functions

func ptr(s string) *string {
	return &s
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func checkManagedLabels(t *testing.T, labels map[string]string, _ string) {
	// Check namespace label exists
	if labels[NamespaceKey] != "default" {
		t.Errorf("Expected %s=default, got %s", NamespaceKey, labels[NamespaceKey])
	}

	// Protected label should not be added by AddManagedLabels anymore
	// It's handled separately by executors
}
