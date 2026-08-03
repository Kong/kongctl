package get

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIGatewayAliasCommandPaths(t *testing.T) {
	cmd, err := NewGetCmd()
	require.NoError(t, err)

	for _, path := range [][]string{
		{"api-gateway", "control-planes"},
		{"konnect", "api-gateway", "control-planes"},
	} {
		resolved, remaining, err := cmd.Find(path)
		require.NoError(t, err)
		assert.Equal(t, "control-plane", resolved.Name())
		assert.Empty(t, remaining)
	}
}
