package export

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestRootRejectsControlPlaneGroupWithoutObservedMemberships(t *testing.T) {
	current := kkComps.ControlPlane{ID: "group-id", Name: "group", Config: kkComps.ControlPlaneConfig{
		ClusterType: new(kkComps.ControlPlaneClusterTypeClusterTypeControlPlaneGroup),
	}}
	_, err := Root(current)
	require.ErrorContains(t, err, "verified memberships")
	declaration := MapControlPlaneToDeclarativeResource(current, []string{"member-id"})
	resource, err := Root(&declaration)
	require.NoError(t, err)
	require.Equal(
		t,
		[]resources.ControlPlaneGroupMember{{ID: "member-id"}},
		resource.(*resources.ControlPlaneResource).Members,
	)
}

func TestProjectionPreservesDumpConfigAndLeavesSecretPolicyToCaller(t *testing.T) {
	payload := map[string]any{
		"name": "provider", "display_name": "Provider", "type": "openai",
		"config": map[string]any{"auth": map[string]any{"type": "basic", "headers": []any{
			map[string]any{"name": "Authorization", "value": "existing-config-value"},
		}}},
	}
	declaration, err := ProjectAs[resources.AIGatewayProviderResource](payload)
	require.NoError(t, err)
	require.Equal(t, payload["config"], declaration.Config)
	fields, err := PublicFields(payload)
	require.NoError(t, err)
	OmitSecrets(resources.ResourceTypeAIGatewayProvider, fields, "")
	scrubbed, err := ProjectAs[resources.AIGatewayProviderResource](fields)
	require.NoError(t, err)
	require.NotEqual(t, declaration.Config, scrubbed.Config)
	require.Equal(t, payload["config"], declaration.Config)
}
