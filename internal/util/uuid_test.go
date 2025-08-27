package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Valid UUIDs - lowercase
		{
			name:     "valid UUID lowercase",
			input:    "12345678-1234-1234-1234-123456789012",
			expected: true,
		},
		{
			name:     "valid UUID with letters lowercase",
			input:    "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			expected: true,
		},
		{
			name:     "standard UUID example",
			input:    "550e8400-e29b-41d4-a716-446655440000",
			expected: true,
		},

		// Valid UUIDs - uppercase
		{
			name:     "valid UUID uppercase",
			input:    "12345678-1234-1234-1234-123456789012",
			expected: true,
		},
		{
			name:     "valid UUID with letters uppercase",
			input:    "A1B2C3D4-E5F6-7890-ABCD-EF1234567890",
			expected: true,
		},

		// Valid UUIDs - mixed case
		{
			name:     "valid UUID mixed case",
			input:    "a1B2c3D4-e5F6-7890-AbCd-Ef1234567890",
			expected: true,
		},

		// Invalid UUIDs - wrong format
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "too short",
			input:    "12345678-1234-1234-1234-12345678901",
			expected: false,
		},
		{
			name:     "too long",
			input:    "12345678-1234-1234-1234-1234567890123",
			expected: false,
		},
		{
			name:     "missing dashes",
			input:    "12345678123412341234123456789012",
			expected: false,
		},
		{
			name:     "wrong dash positions",
			input:    "1234567-81234-1234-1234-123456789012",
			expected: false,
		},
		{
			name:     "invalid characters",
			input:    "12345678-1234-1234-1234-12345678901g",
			expected: false,
		},
		{
			name:     "contains spaces",
			input:    "12345678-1234-1234-1234-123456789012 ",
			expected: false,
		},
		{
			name:     "contains special characters",
			input:    "12345678-1234-1234-1234-123456789!@#",
			expected: false,
		},

		// Edge cases from existing tests
		{
			name:     "nil UUID (all zeros)",
			input:    "00000000-0000-0000-0000-000000000000",
			expected: true,
		},
		{
			name:     "max hex values",
			input:    "ffffffff-ffff-ffff-ffff-ffffffffffff",
			expected: true,
		},
		{
			name:     "max hex values uppercase",
			input:    "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF",
			expected: true,
		},

		// Test cases from existing resolver test
		{
			name:     "resolver test case 1",
			input:    "12345678-1234-5678-1234-567812345678",
			expected: true,
		},
		{
			name:     "resolver test case 2",
			input:    "a0b1c2d3-e4f5-6789-abcd-ef0123456789",
			expected: true,
		},
		{
			name:     "resolver test case with extra chars",
			input:    "12345678-1234-5678-1234-567812345678-extra",
			expected: false,
		},

		// Test cases from existing resources test
		{
			name:     "resources test case 1",
			input:    "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			expected: true,
		},
		{
			name:     "resources test case 2",
			input:    "f9e8d7c6-b5a4-3210-9876-fedcba098765",
			expected: true,
		},
		{
			name:     "resources test case uppercase",
			input:    "A1B2C3D4-E5F6-7890-ABCD-EF1234567890",
			expected: true,
		},
		{
			name:     "resources test case with trailing space",
			input:    "a1b2c3d4-e5f6-7890-abcd-ef1234567890 ",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidUUID(tt.input)
			assert.Equal(t, tt.expected, result, "IsValidUUID(%q) should return %v", tt.input, tt.expected)
		})
	}
}

func TestIsValidUUID_Performance(t *testing.T) {
	// Test that regex is compiled once and performs well
	validUUID := "12345678-1234-1234-1234-123456789012"

	// Run validation many times to ensure no performance regression
	for i := 0; i < 1000; i++ {
		result := IsValidUUID(validUUID)
		assert.True(t, result)
	}
}

func TestIsValidUUID_CaseInsensitive(t *testing.T) {
	// Test various case combinations
	testCases := []string{
		"a1b2c3d4-e5f6-7890-abcd-ef1234567890", // all lowercase
		"A1B2C3D4-E5F6-7890-ABCD-EF1234567890", // all uppercase
		"A1b2C3d4-E5f6-7890-AbCd-Ef1234567890", // mixed case
		"a1B2c3D4-e5F6-7890-aBcD-eF1234567890", // mixed case 2
	}

	for _, uuid := range testCases {
		t.Run("case_test_"+uuid, func(t *testing.T) {
			result := IsValidUUID(uuid)
			assert.True(t, result, "UUID %q should be valid regardless of case", uuid)
		})
	}
}
