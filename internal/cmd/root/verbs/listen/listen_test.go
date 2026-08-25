package listen

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestNewTailCmdUsesPullAPIAndPreservesListenerChild(t *testing.T) {
	t.Parallel()

	command, err := NewTailCmd()
	require.NoError(t, err)
	require.NotNil(t, command.RunE)
	require.NotNil(t, command.Flags().Lookup("follow"))
	require.Equal(t, "true", command.Flags().Lookup("follow").DefValue)

	auditLogs, _, err := command.Find([]string{"audit-logs"})
	require.NoError(t, err)
	require.Equal(t, "audit-logs", auditLogs.Name())
	require.Equal(t, []string{"audit-log"}, auditLogs.Aliases)
	require.NotNil(t, auditLogs.Flags().Lookup("since"))
	require.Equal(t, "100", auditLogs.Flags().Lookup("page-size").DefValue)

	listener, _, err := command.Find([]string{"audit-logs", "listener"})
	require.NoError(t, err)
	require.Equal(t, "listener", listener.Name())
	require.NotNil(t, listener.Flags().Lookup("endpoint"))
	require.NotNil(t, listener.Flags().Lookup("authorization"))

	explicit, _, err := command.Find([]string{"konnect", "audit-logs"})
	require.NoError(t, err)
	require.Equal(t, "audit-logs", explicit.Name())
	require.Equal(t, []string{"audit-log"}, explicit.Aliases)
	require.Equal(t, "100", explicit.Flags().Lookup("page-size").DefValue)
}

func TestNewTailCmdHelpDescribesPullBehavior(t *testing.T) {
	t.Parallel()

	command, err := NewTailCmd()
	require.NoError(t, err)
	var output strings.Builder
	command.SetOut(&output)
	command.SetArgs([]string{"audit-logs", "--help"})
	require.NoError(t, command.Execute())
	require.Contains(t, output.String(), "five-minute audit-log catch-up")
	require.Contains(t, output.String(), "--output jsonl")
}

func TestSetTailFlagDefaultIfUnset(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool(tailFlagName, false, "")

	err := setTailFlagDefaultIfUnset(cmd)
	require.NoError(t, err)

	value, err := cmd.Flags().GetBool(tailFlagName)
	require.NoError(t, err)
	require.True(t, value)
}

func TestSetTailFlagDefaultIfUnsetRespectsUserInput(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool(tailFlagName, true, "")
	require.NoError(t, cmd.Flags().Set(tailFlagName, "false"))

	err := setTailFlagDefaultIfUnset(cmd)
	require.NoError(t, err)

	value, err := cmd.Flags().GetBool(tailFlagName)
	require.NoError(t, err)
	require.False(t, value)
}

func TestSetTailFlagDefaultIfUnsetNoTailFlag(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "test"}
	require.NoError(t, setTailFlagDefaultIfUnset(cmd))
}
