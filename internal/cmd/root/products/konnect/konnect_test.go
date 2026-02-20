package konnect

import (
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestNewKonnectCmdCreateDoesNotExposeAuditLogs(t *testing.T) {
	t.Parallel()

	cmd, err := NewKonnectCmd(verbs.Create)
	require.NoError(t, err)
	require.False(t, hasSubcommandNamed(cmd, "audit-logs"))
}

func TestNewKonnectCmdListenExposesAuditLogs(t *testing.T) {
	t.Parallel()

	cmd, err := NewKonnectCmd(verbs.Listen)
	require.NoError(t, err)
	require.True(t, hasSubcommandNamed(cmd, "audit-logs"))
}

func TestNewKonnectCmdGetExposesAuditLogs(t *testing.T) {
	t.Parallel()

	cmd, err := NewKonnectCmd(verbs.Get)
	require.NoError(t, err)
	require.True(t, hasSubcommandNamed(cmd, "audit-logs"))
}

func hasSubcommandNamed(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}

	for _, sub := range cmd.Commands() {
		if sub.Name() == name {
			return true
		}
	}

	return false
}
