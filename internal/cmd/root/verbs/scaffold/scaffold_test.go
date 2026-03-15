package scaffold

import (
	"bytes"
	"context"
	"testing"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRootWithScaffold(t *testing.T, defaultOutput string) *cobra.Command {
	t.Helper()

	cobra.EnableTraverseRunHooks = true

	scaffoldCmd, err := NewScaffoldCmd()
	require.NoError(t, err)

	outputFormat := cmdpkg.NewEnum([]string{
		common.JSON.String(),
		common.YAML.String(),
		common.TEXT.String(),
	}, defaultOutput)

	root := &cobra.Command{
		Use:              "kongctl",
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
		PersistentPreRun: func(c *cobra.Command, _ []string) {
			c.SetContext(context.Background())
		},
	}
	root.PersistentFlags().VarP(outputFormat, common.OutputFlagName, common.OutputFlagShort, "Output format")
	root.AddCommand(scaffoldCmd)

	return root
}

func TestNewScaffoldCmd(t *testing.T) {
	cmd, err := NewScaffoldCmd()
	require.NoError(t, err)
	require.NotNil(t, cmd)

	assert.Equal(t, "scaffold <resource-path>", cmd.Use)
	assert.Contains(t, cmd.Short, "YAML scaffold")
	assert.Contains(t, cmd.Long, "commented YAML starter")
	assert.Contains(t, cmd.Example, "scaffold api")
}

func TestScaffoldCmd_RejectsExplicitOutputFlag(t *testing.T) {
	root := newTestRootWithScaffold(t, common.TEXT.String())

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"scaffold", "api", "--output", "json"})

	err := root.Execute()
	require.Error(t, err)
	assert.Equal(t, "flags -o/--output are not supported for the scaffold command", err.Error())
}

func TestScaffoldCmd_RejectsParseTimeOutputMisuse(t *testing.T) {
	root := newTestRootWithScaffold(t, common.TEXT.String())

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"scaffold", "api", "-o", "docs/out.yaml"})

	err := root.Execute()
	require.Error(t, err)
	assert.Equal(t, "flags -o/--output are not supported for the scaffold command", err.Error())
}

func TestScaffoldCmd_IgnoresConfiguredOutputDefault(t *testing.T) {
	root := newTestRootWithScaffold(t, common.JSON.String())

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"scaffold", "api"})

	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, output.String(), "apis:")
	assert.Contains(t, output.String(), "ref:")
}
