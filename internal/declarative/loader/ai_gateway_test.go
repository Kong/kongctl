package loader

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
