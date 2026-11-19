package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAICostSlackProjectionPreservesPublicFields(t *testing.T) {
	const webhook = "redacted-value"
	slack := map[string]any{"webhook_url": webhook, "channel_name": "Alerts"}
	payload := AICostWriteFields(ResourceTypeAICostChannel, map[string]any{"type": "slack", "slack": slack})
	require.Equal(t, map[string]any{"channel_name": "Alerts"}, payload["slack"])
	require.Equal(t, webhook, slack["webhook_url"])
}
