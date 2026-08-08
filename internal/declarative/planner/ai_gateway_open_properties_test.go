package planner

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPreserveAIGatewayOpenPropertiesHonorsDeclaredAndMutableFields(t *testing.T) {
	t.Parallel()

	currentAdditional := map[string]any{
		"preserved":   "live",
		"overridden":  "live",
		"server_only": "live",
	}
	currentPayload := map[string]any{
		"name":       "resource",
		"preserved":  "live",
		"overridden": "live",
	}
	currentCompare := clonePayloadMap(currentPayload)
	desiredPayload := map[string]any{
		"name":       "resource",
		"overridden": "declared",
	}

	update := preserveAIGatewayOpenProperties(
		currentAdditional,
		currentPayload,
		currentCompare,
		desiredPayload,
	)

	require.Equal(t, "live", update["preserved"])
	require.Equal(t, "declared", update["overridden"])
	require.NotContains(t, update, "server_only")
	require.NotContains(t, currentCompare, "preserved")
	require.Equal(t, "live", currentCompare["overridden"])
}
