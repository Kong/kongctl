package sdk

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOutputRenderJSONAppliesJQ(t *testing.T) {
	var out bytes.Buffer
	runtimeCtx := RuntimeContext{
		OutputSettings: OutputContext{
			Format: "json",
			JQ: JQContext{
				Expression: ".id",
				Color:      "never",
			},
		},
	}

	err := runtimeCtx.Output().WithWriter(&out).Render(
		map[string]string{"id": "display"},
		map[string]string{"id": "raw"},
	)

	require.NoError(t, err)
	require.JSONEq(t, `"raw"`, out.String())
}

func TestOutputRenderRawJQ(t *testing.T) {
	var out bytes.Buffer
	runtimeCtx := RuntimeContext{
		OutputSettings: OutputContext{
			Format: "json",
			JQ: JQContext{
				Expression: ".id",
				RawOutput:  true,
				Color:      "never",
			},
		},
	}

	err := runtimeCtx.Output().WithWriter(&out).Render(map[string]string{"id": "raw"})

	require.NoError(t, err)
	require.Equal(t, "raw\n", out.String())
}
