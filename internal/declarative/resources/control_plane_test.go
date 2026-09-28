package resources

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestControlPlaneResourceExternalDoesNotDefaultManagedName(t *testing.T) {
	t.Parallel()

	cp := ControlPlaneResource{
		BaseResource: BaseResource{Ref: "shared-control-plane"},
		External: &ExternalBlock{Selector: &ExternalSelector{
			MatchFields: map[string]string{"id": "11111111-1111-1111-1111-111111111111"},
		}},
	}
	cp.SetDefaults()
	require.True(t, cp.IsExternal())
	require.Empty(t, cp.Name)
	require.NoError(t, cp.Validate())
}

func TestControlPlaneResourceManagedRequiresExplicitName(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"", " \t", "konnect-control-plane"} {
		t.Run(name, func(t *testing.T) {
			cp := ControlPlaneResource{
				BaseResource:              BaseResource{Ref: "managed-control-plane"},
				CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{Name: name},
			}
			cp.SetDefaults()
			require.Equal(t, name, cp.Name)
			if name == "konnect-control-plane" {
				require.NoError(t, cp.Validate())
			} else {
				require.ErrorContains(t, cp.Validate(), "name is required for control plane managed-control-plane")
			}
		})
	}
}
