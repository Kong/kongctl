package aigateway

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayHelpListsModelsOnce(t *testing.T) {
	t.Parallel()

	cmd, err := NewAIGatewayCmd(
		verbs.Get,
		func(verbs.VerbValue, *cobra.Command) {},
		func(*cobra.Command, []string) error { return nil },
	)
	require.NoError(t, err)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	require.NoError(t, cmd.Execute())
	require.Equal(t, 1, strings.Count(out.String(), "\n  models "))
}
