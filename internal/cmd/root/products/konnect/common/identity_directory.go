package common

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	IdentityDirectoryIDFlagName   = "directory-id"
	IdentityDirectoryNameFlagName = "directory-name"
	DefaultIdentityDirectoryName  = "default"
)

// AddIdentityDirectoryFlags registers directory selectors with a name fallback.
func AddIdentityDirectoryFlags(cmd *cobra.Command) {
	cmd.Flags().String(IdentityDirectoryIDFlagName, "", "Parent directory UUID (overrides the default name)")
	cmd.Flags().String(IdentityDirectoryNameFlagName, DefaultIdentityDirectoryName, "Exact parent directory name")
	cmd.MarkFlagsMutuallyExclusive(IdentityDirectoryIDFlagName, IdentityDirectoryNameFlagName)
}

// IdentityDirectorySelector returns one selector, ignoring the default name when an ID is supplied.
func IdentityDirectorySelector(cmd *cobra.Command) (id, name string, err error) {
	id, err = cmd.Flags().GetString(IdentityDirectoryIDFlagName)
	if err != nil {
		return "", "", err
	}
	name, err = cmd.Flags().GetString(IdentityDirectoryNameFlagName)
	if err != nil {
		return "", "", err
	}
	if cmd.Flags().Changed(IdentityDirectoryIDFlagName) && cmd.Flags().Changed(IdentityDirectoryNameFlagName) {
		return "", "", fmt.Errorf("provide exactly one of --directory-id or --directory-name")
	}
	if cmd.Flags().Changed(IdentityDirectoryIDFlagName) {
		if id == "" {
			return "", "", fmt.Errorf("directory-id must not be empty")
		}
		return id, "", nil
	}
	if name == "" {
		return "", "", fmt.Errorf("directory-name must not be empty")
	}
	return "", name, nil
}
