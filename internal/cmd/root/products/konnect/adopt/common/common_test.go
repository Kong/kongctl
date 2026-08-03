package common

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestReadAdoptFlagsAcceptsConsecutiveHyphens(t *testing.T) {
	cmd := &cobra.Command{Use: "adopt"}
	require.NoError(t, AddAdoptFlags(cmd))
	require.NoError(t, cmd.ParseFlags([]string{"--namespace", "team--service--prod"}))

	flags, err := ReadAdoptFlags(cmd)
	require.NoError(t, err)
	require.Equal(t, "team--service--prod", flags.Namespace)
}
