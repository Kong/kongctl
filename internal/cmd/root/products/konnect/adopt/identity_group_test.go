package adopt

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestAdoptGroupPreservesPublicSettingsAndProtection(t *testing.T) {
	const directory = "4078f559-063b-4f10-9098-9e6553b21d9b"
	group := fixture.Group("Fulfillment", "old-team", true)
	api := &fixture.GroupAPI{Directories: map[string][]helpers.IdentityGroup{directory: {group}}}
	_, err := adoptIdentityGroup(t.Context(), api, directory, group.ID, "new-team", false)
	require.ErrorContains(t, err, "overwrite-namespace")
	require.Empty(t, api.Writes)
	result, err := adoptIdentityGroup(t.Context(), api, directory, group.Name, "new-team", true)
	require.NoError(t, err)
	require.Equal(t, group.ID, result.ID)
	require.Equal(t, "new-team", result.Namespace)
	require.Len(t, api.Writes, 1)
	require.Equal(t, group.Labels, api.Writes[0].Labels)
	require.Equal(t, group.Metadata, api.Writes[0].Metadata)
	require.Equal(t, group.Description, *api.Writes[0].Description)
	require.Equal(t, "true", api.Writes[0].ManagedBy[resources.ChildProtectedKey])
	require.Len(t, api.Writes[0].ManagedBy, 4)
	api.Directories[directory][0].Type = new(kkComps.KongGroupTypeTeam)
	_, err = adoptIdentityGroup(t.Context(), api, directory, group.ID, "new-team", true)
	require.ErrorContains(t, err, "Q32")
	require.Len(t, api.Writes, 1)
}

func TestAdoptGroupRequiresExplicitDirectory(t *testing.T) {
	for _, flags := range [][]string{
		nil, {"--directory-id", "4078f559-063b-4f10-9098-9e6553b21d9b", "--directory-name", "default"},
	} {
		cmd, err := NewIdentityGroupCmd(verbs.Adopt, nil, nil)
		require.NoError(t, err)
		cmd.SetArgs(append([]string{"Fulfillment"}, flags...))
		err = cmd.Execute()
		require.Error(t, err)
		require.Contains(t, err.Error(), "directory")
	}
}
