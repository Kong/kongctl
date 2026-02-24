package listen

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

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
