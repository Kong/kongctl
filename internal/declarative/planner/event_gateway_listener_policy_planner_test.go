package planner

import (
	"encoding/json"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestTLSPolicyConfigNeedsUpdate(t *testing.T) {
	var desired resources.EventGatewayListenerPolicyResource
	require.NoError(t, json.Unmarshal([]byte(`{
  "ref": "tls", "name": "tls", "type": "tls_server",
  "config": {"certificates": [{"certificate": "certificate", "key": "private-key"}]}
}`), &desired))
	for _, tt := range []struct {
		name        string
		certificate map[string]any
		wantUpdate  bool
	}{
		{
			name:        "omitted private key is unchanged",
			certificate: map[string]any{"certificate": "certificate"},
		},
		{
			name:        "null private key is unchanged",
			certificate: map[string]any{"certificate": "certificate", "key": nil},
		},
		{
			name:        "returned empty private key needs update",
			certificate: map[string]any{"certificate": "certificate", "key": ""},
			wantUpdate:  true,
		},
		{
			name:        "returned matching private key is unchanged",
			certificate: map[string]any{"certificate": "certificate", "key": "private-key"},
		},
		{
			name:        "returned different private key needs update",
			certificate: map[string]any{"certificate": "certificate", "key": "different-key"},
			wantUpdate:  true,
		},
		{
			name:        "different certificate with omitted key needs update",
			certificate: map[string]any{"certificate": "different-certificate"},
			wantUpdate:  true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config := map[string]any{"certificates": []any{tt.certificate}}
			require.Equal(t, tt.wantUpdate, (&Planner{}).tlsPolicyConfigNeedsUpdate(config, desired))
		})
	}
}
