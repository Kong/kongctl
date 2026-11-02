package common

import (
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestIdentityDirectorySelection(t *testing.T) {
	const directoryID = "731559d3-39bd-48c7-b33c-eb192847eb20"
	for _, tc := range []struct {
		name, id, directoryName, errorText string
		args                               []string
	}{
		{name: "default", directoryName: "default"},
		{name: "explicit name", args: []string{"--directory-name", "engineering"}, directoryName: "engineering"},
		{name: "explicit id", args: []string{"--directory-id", directoryID}, id: directoryID},
		{
			name: "both selectors", args: []string{"--directory-id", directoryID, "--directory-name", "default"},
			errorText: "directory-id directory-name",
		},
		{name: "empty id", args: []string{"--directory-id="}, errorText: "directory-id must not be empty"},
		{name: "empty name", args: []string{"--directory-name="}, errorText: "directory-name must not be empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := &cobra.Command{Use: "test", SilenceErrors: true, SilenceUsage: true}
			command.SetOut(io.Discard)
			command.SetErr(io.Discard)
			AddIdentityDirectoryFlags(command)
			var id, name string
			command.RunE = func(cmd *cobra.Command, _ []string) error {
				var err error
				id, name, err = IdentityDirectorySelector(cmd)
				return err
			}
			command.SetArgs(tc.args)
			err := command.Execute()
			if tc.errorText != "" {
				require.ErrorContains(t, err, tc.errorText)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.id, id)
			require.Equal(t, tc.directoryName, name)
		})
	}
}
