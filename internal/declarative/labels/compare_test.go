package labels

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareUserLabels(t *testing.T) {
	tests := []struct {
		name     string
		current  map[string]string
		desired  map[string]string
		expected bool
	}{
		{
			name:     "both nil",
			current:  nil,
			desired:  nil,
			expected: false,
		},
		{
			name:     "both empty",
			current:  map[string]string{},
			desired:  map[string]string{},
			expected: false,
		},
		{
			name: "same user labels",
			current: map[string]string{
				"env":                  "prod",
				"KONGCTL-managed":      "true",
				"KONGCTL-protected":    "false",
				"KONGCTL-last-updated": "2024-01-01T00:00:00Z",
			},
			desired: map[string]string{
				"env": "prod",
			},
			expected: false,
		},
		{
			name: "different user labels",
			current: map[string]string{
				"env":                  "prod",
				"KONGCTL-managed":      "true",
				"KONGCTL-protected":    "false",
				"KONGCTL-last-updated": "2024-01-01T00:00:00Z",
			},
			desired: map[string]string{
				"env": "staging",
			},
			expected: true,
		},
		{
			name: "additional user label",
			current: map[string]string{
				"env":                  "prod",
				"KONGCTL-managed":      "true",
				"KONGCTL-protected":    "false",
				"KONGCTL-last-updated": "2024-01-01T00:00:00Z",
			},
			desired: map[string]string{
				"env":  "prod",
				"team": "platform",
			},
			expected: true,
		},
		{
			name: "removed user label",
			current: map[string]string{
				"env":                  "prod",
				"team":                 "platform",
				"KONGCTL-managed":      "true",
				"KONGCTL-protected":    "false",
				"KONGCTL-last-updated": "2024-01-01T00:00:00Z",
			},
			desired: map[string]string{
				"env": "prod",
			},
			expected: true,
		},
		{
			name: "empty desired removes all user labels",
			current: map[string]string{
				"env":                  "prod",
				"team":                 "platform",
				"KONGCTL-managed":      "true",
				"KONGCTL-protected":    "false",
				"KONGCTL-last-updated": "2024-01-01T00:00:00Z",
			},
			desired:  map[string]string{},
			expected: true,
		},
		{
			name: "only system labels differ",
			current: map[string]string{
				"env":                  "prod",
				"KONGCTL-managed":      "true",
				"KONGCTL-protected":    "false",
				"KONGCTL-last-updated": "2024-01-01T00:00:00Z",
			},
			desired: map[string]string{
				"env":                  "prod",
				"KONGCTL-managed":      "true",
				"KONGCTL-protected":    "true",
				"KONGCTL-last-updated": "2024-01-02T00:00:00Z",
			},
			expected: false, // System labels are ignored
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompareUserLabels(tt.current, tt.desired)
			assert.Equal(t, tt.expected, result)
		})
	}
}