package planner

import (
	"encoding/json"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestTLSListenerPolicyCertificateComparison(t *testing.T) {
	var desired resources.EventGatewayListenerPolicyResource
	require.NoError(t, json.Unmarshal([]byte(`{
		"ref":"policy", "name":"policy", "type":"tls_server",
		"config":{"certificates":[{"certificate":"certificate", "key":"private-key"}]}
	}`), &desired))

	for _, tt := range []struct {
		name        string
		certificate map[string]any
		wantUpdate  bool
	}{
		{
			name:        "API omits private key",
			certificate: map[string]any{"certificate": "certificate"},
		},
		{
			name:        "matching readable key",
			certificate: map[string]any{"certificate": "certificate", "key": "private-key"},
		},
		{
			name:        "changed readable key",
			certificate: map[string]any{"certificate": "certificate", "key": "other-key"},
			wantUpdate:  true,
		},
		{
			name:        "changed certificate with omitted key",
			certificate: map[string]any{"certificate": "other-certificate"},
			wantUpdate:  true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			current := map[string]any{"certificates": []any{tt.certificate}}
			require.Equal(t, tt.wantUpdate, (&Planner{}).tlsPolicyConfigNeedsUpdate(current, desired))
		})
	}
}
