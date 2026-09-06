package loader

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoaderAIGatewayUniquenessUsesName(t *testing.T) {
	for _, tc := range []struct{ name, displayName, wantError string }{
		{name: "second-gateway", displayName: "Shared Display"},
		{name: "first-gateway", displayName: "Different Display", wantError: "duplicate ai_gateway name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := fmt.Sprintf(`
ai_gateways:
  - ref: first-local-ref
    name: first-gateway
    display_name: Shared Display
  - ref: second-local-ref
    name: %s
    display_name: %s
`, tc.name, tc.displayName)
			rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
				return
			}
			require.NoError(t, err)
			require.Len(t, rs.AIGateways, 2)
		})
	}
}

func TestLoaderPreservesAIGatewayDeploymentType(t *testing.T) {
	input := `
ai_gateways:
  - ref: support-gateway
    name: support-gateway
    display_name: Support Gateway
    deployment_type: managed
`

	resourceSet, err := New().LoadFromSources([]Source{{
		Path: writeLoaderTestFile(t, input),
		Type: SourceTypeFile,
	}}, false)
	require.NoError(t, err)
	require.Len(t, resourceSet.AIGateways, 1)
	require.NotNil(t, resourceSet.AIGateways[0].DeploymentType)
	require.Equal(t, "managed", string(*resourceSet.AIGateways[0].DeploymentType))
}
