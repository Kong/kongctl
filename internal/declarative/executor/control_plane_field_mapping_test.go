package executor

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

func TestControlPlaneSavedPlanURLMapping(t *testing.T) {
	seed := []kkComps.ProxyURL{{Host: "existing", Port: 443, Protocol: "https"}}
	tests := []struct {
		name           string
		value          any
		create, update []kkComps.ProxyURL
		err            string
	}{
		{name: "nil leaves create untouched but clears update", value: nil, create: seed},
		{name: "typed nil clears both", value: []kkComps.ProxyURL(nil)},
		{name: "explicit empty", value: []any{}, create: []kkComps.ProxyURL{}, update: []kkComps.ProxyURL{}},
		{name: "typed URLs", value: seed, create: seed, update: seed},
		{
			name: "JSON decoded URLs", value: []any{map[string]any{"host": "proxy", "port": float64(80), "protocol": "http"}},
			create: []kkComps.ProxyURL{{Host: "proxy", Port: 80, Protocol: "http"}},
			update: []kkComps.ProxyURL{{Host: "proxy", Port: 80, Protocol: "http"}},
		},
		{
			name: "map slice and integer port", value: []map[string]any{{"port": int(80)}},
			create: []kkComps.ProxyURL{{Port: 80}}, update: []kkComps.ProxyURL{{Port: 80}},
		},
		{
			name: "unsupported collection", value: "invalid",
			err: "invalid proxy_urls value: unsupported proxy_urls type string",
		},
		{
			name: "invalid entry", value: []any{"invalid"},
			err: "invalid proxy_urls value: proxy url entry must be object, got string",
		},
		{
			name: "invalid port", value: []any{map[string]any{"port": "80"}},
			err: "invalid proxy_urls value: invalid proxy url port type string",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewControlPlaneAdapter(nil)
			fields := map[string]any{planner.FieldProxyURLs: tt.value}
			create := kkComps.CreateControlPlaneRequest{ProxyUrls: seed}
			update := kkComps.UpdateControlPlaneRequest{ProxyUrls: seed}
			err := adapter.MapCreateFields(t.Context(), &ExecutionContext{}, fields, &create)
			updateErr := adapter.MapUpdateFields(t.Context(), &ExecutionContext{}, fields, &update, nil)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				require.EqualError(t, updateErr, tt.err)
				require.Equal(t, seed, create.ProxyUrls)
				require.Equal(t, seed, update.ProxyUrls)
				return
			}
			require.NoError(t, err)
			require.NoError(t, updateErr)
			require.Equal(t, tt.create, create.ProxyUrls)
			require.Equal(t, tt.update, update.ProxyUrls)
		})
	}
}
