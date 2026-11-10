package identity

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestDirectorySelectionResolvesDefaultWithoutCreatingIt(t *testing.T) {
	for _, tc := range []struct {
		name      string
		directory string
		flags     []string
		missing   bool
	}{
		{name: "default exists", directory: "default"},
		{name: "default missing", directory: "another", missing: true},
		{name: "explicit name", directory: "another", flags: []string{"--directory-name", "another"}},
		{
			name: "explicit id", directory: "another",
			flags: []string{"--directory-id", "7a5e1b58-48a4-4586-b02d-0d91eccca152"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := fixture.Directory(tc.directory, "", false)
			api := &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{value}, Current: value}
			cmd := newGroupReadCommand(verbs.Get, false, nil, nil)
			require.NoError(t, cmd.ParseFlags(tc.flags))
			require.NoError(t, cmd.Args(cmd, nil))
			require.NoError(t, cmd.ValidateFlagGroups())
			id, err := groupDirectoryIDFromFlags(t.Context(), cmd, api, 10)
			if tc.missing {
				require.ErrorContains(t, err, "default")
				require.Empty(t, id)
			} else {
				require.NoError(t, err)
				require.Equal(t, value.ID, id)
			}
			require.Zero(t, api.Creates)
			require.Zero(t, api.Replaces)
			require.Zero(t, api.Deletes)
			if tc.name == "explicit id" {
				require.Zero(t, api.Lists)
				require.Zero(t, api.Gets)
			}
		})
	}
}
