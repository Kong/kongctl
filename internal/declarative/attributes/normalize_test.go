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
		expected map[string][]string
	}{
		{
			name: "string values become single-element slices",
			input: map[string]any{
				"owner": "team-a",
			},
			expected: map[string][]string{
				"owner": {"team-a"},
			},
		},
		{
			name: "mixed slice types convert to []string",
			input: map[string]any{
				"domains": []any{"web", "mobile"},
			},
			expected: map[string][]string{
				"domains": {"web", "mobile"},
			},
		},
		{
			name: "map[string][]string passes through",
			input: map[string][]string{
				"env": {"prod"},
			},
			expected: map[string][]string{
				"env": {"prod"},
			},
		},
		{
			name: "non-string entries fall back to fmt.Sprint",
			input: map[string]any{
				"priority": 1,
			},
			expected: map[string][]string{
				"priority": {"1"},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
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
