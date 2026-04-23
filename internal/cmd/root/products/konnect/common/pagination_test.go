package common

import "testing"

import "github.com/stretchr/testify/require"

func TestResolveRequestPageSize(t *testing.T) {
	t.Run("configured page size", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			RequestPageSizeConfigPath: "25",
		})

		require.Equal(t, int64(25), ResolveRequestPageSize(cfg))
	})

	t.Run("defaults when unset", func(t *testing.T) {
		cfg, _ := newTestConfig(nil)

		require.Equal(t, int64(DefaultRequestPageSize), ResolveRequestPageSize(cfg))
	})

	t.Run("defaults when invalid", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			RequestPageSizeConfigPath: "0",
		})

		require.Equal(t, int64(DefaultRequestPageSize), ResolveRequestPageSize(cfg))
	})
}

func TestHasMorePageNumberResults(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		collected int
		pageItems int
		want      bool
	}{
		{
			name:      "continues when more items remain",
			total:     11,
			collected: 10,
			pageItems: 10,
			want:      true,
		},
		{
			name:      "stops on exact boundary",
			total:     10,
			collected: 10,
			pageItems: 10,
			want:      false,
		},
		{
			name:      "stops when server reports zero total",
			total:     0,
			collected: 0,
			pageItems: 10,
			want:      false,
		},
		{
			name:      "stops on empty page",
			total:     25,
			collected: 10,
			pageItems: 0,
			want:      false,
		},
		{
			name:      "stops once collected exceeds total",
			total:     10,
			collected: 11,
			pageItems: 1,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, HasMorePageNumberResults(tt.total, tt.collected, tt.pageItems))
		})
	}
}
