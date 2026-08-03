package get

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIGatewayAliasCommandPaths(t *testing.T) {
	cmd, err := NewGetCmd()
	require.NoError(t, err)

	for _, test := range []struct {
		name string
		path []string
	}{
		{name: "Konnect-first", path: []string{"api-gateway", "control-planes"}},
		{name: "explicit Konnect", path: []string{"konnect", "api-gateway", "control-planes"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			resolved, remaining, err := cmd.Find(test.path)
			require.NoError(t, err)
			assert.Equal(t, "control-plane", resolved.Name())
			assert.Empty(t, remaining)
		})
	}
}
