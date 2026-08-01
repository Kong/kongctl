package plugin

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestPluginDetailViewIncludesConfig(t *testing.T) {
	plugin := &kkComps.Plugin{
		Name: "rate-limiting",
		Config: map[string]any{
			"minute": 11,
		},
	}

	detail := pluginDetailView(plugin)

	require.Contains(t, detail, "name: rate-limiting")
	require.Contains(t, detail, `config: {"minute":11}`)
}
