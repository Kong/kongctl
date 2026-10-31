package adopt

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestAdoptIdentityDirectoryPreservesSettingsAndProtection(t *testing.T) {
	for _, protected := range []bool{false, true} {
		current := fixture.Directory("default", "", false)
		if protected {
			current.ManagedBy[resources.ChildProtectedKey] = "true"
		}
		api := &fixture.DirectoryAPI{Current: current}
		result, err := adoptIdentityDirectory(t.Context(), api, current.ID, "identity-team", false)
		require.NoError(t, err)
		require.Equal(t, "identity-team", result.Namespace)
		require.Equal(t, current.Labels, api.Replaced.Labels)
		require.Equal(t, current.Description, *api.Replaced.Description)
		require.Equal(t, current.TTLSecs, api.Replaced.TTLSecs)
		require.Equal(t, current.NegativeTTLSecs, api.Replaced.NegativeTTLSecs)
		require.Equal(t, current.AllowAllControlPlanes, api.Replaced.AllowAllControlPlanes)
		require.Equal(t, current.VaultEnabled, api.Replaced.VaultEnabled)
		require.Equal(t, resources.BuildChildManagedBy("identity-team", protected, "dev"), api.Replaced.ManagedBy)
	}
}

func TestAdoptIdentityDirectoryRequiresExplicitNamespaceOverwrite(t *testing.T) {
	current := fixture.Directory("default", "foreign", true)
	api := &fixture.DirectoryAPI{Current: current}
	_, err := adoptIdentityDirectory(t.Context(), api, current.ID, "identity-team", false)
	require.ErrorContains(t, err, "--overwrite-namespace")
	require.Zero(t, api.Replaces)
	_, err = adoptIdentityDirectory(t.Context(), api, current.ID, "identity-team", true)
	require.NoError(t, err)
	require.Equal(t, "true", api.Replaced.ManagedBy[resources.ChildProtectedKey])
}
