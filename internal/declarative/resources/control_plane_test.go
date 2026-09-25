package resources

import (
	"testing"

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

func TestControlPlaneResourceManagedDefaultsNameToRef(t *testing.T) {
	t.Parallel()

	cp := ControlPlaneResource{
		BaseResource: BaseResource{Ref: "managed-control-plane"},
	}
	cp.SetDefaults()
	require.False(t, cp.IsExternal())
	require.Equal(t, "managed-control-plane", cp.Name)
}
