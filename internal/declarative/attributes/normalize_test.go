package attributes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeAPIAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		expected map[string]any
	}{
		{
			name: "string values become single-element slices",
			input: map[string]any{
				"owner": "team-a",
			},
			expected: map[string]any{
				"owner": []string{"team-a"},
			},
		},
		{
			name: "mixed slice types convert to []string",
			input: map[string]any{
				"domains": []any{"web", "mobile"},
			},
			expected: map[string]any{
				"domains": []string{"web", "mobile"},
			},
		},
		{
			name: "map[string][]string passes through",
			input: map[string][]string{
				"env": {"prod"},
			},
			expected: map[string]any{
				"env": []string{"prod"},
			},
		},
		{
			name: "non-string entries fall back to fmt.Sprint",
			input: map[string]any{
				"priority": 1,
			},
			expected: map[string]any{
				"priority": []string{"1"},
			},
		},
		{
			name: "null values are preserved for per-key clears",
			input: map[string]any{
				"owner": nil,
			},
			expected: map[string]any{
				"owner": nil,
			},
		},
		{
			name: "empty slices stay empty instead of becoming null",
			input: map[string][]string{
				"owner": {},
			},
			expected: map[string]any{
				"owner": []string{},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, ok := NormalizeAPIAttributes(tc.input)
			require.True(t, ok, "expected normalization to succeed")
			require.Equal(t, tc.expected, out)
		})
	}

	t.Run("nil input returns false", func(t *testing.T) {
		t.Parallel()
		_, ok := NormalizeAPIAttributes(nil)
		require.False(t, ok)
	})
}
