package root

import (
	"bytes"
	"testing"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/maturity"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMaturityHelpRoot() *cobra.Command {
	root := &cobra.Command{Use: "kongctl", Short: "root"}
	root.SetUsageTemplate(mergedFlagsUsageTemplate)
	return root
}

func maturityTestRun(*cobra.Command, []string) {}

func renderHelp(t *testing.T, command *cobra.Command) string {
	t.Helper()
	var output bytes.Buffer
	command.SetOut(&output)
	require.NoError(t, command.Help())
	return output.String()
}

func TestMaturityCommandListingLabelsOnlyLowerChildren(t *testing.T) {
	root := newMaturityHelpRoot()
	beta := &cobra.Command{Use: "beta", Short: "Beta command", Run: maturityTestRun}
	inherited := &cobra.Command{Use: "inherited", Short: "Inherited Beta command", Run: maturityTestRun}
	preview := &cobra.Command{Use: "preview", Short: "Preview command", Run: maturityTestRun}
	require.NoError(t, maturity.AnnotateCommand(beta, maturity.Metadata{Level: maturity.LevelBeta}))
	require.NoError(t, maturity.AnnotateCommand(preview, maturity.Metadata{Level: maturity.LevelTechPreview}))
	root.AddCommand(beta)
	beta.AddCommand(inherited)
	inherited.AddCommand(preview)

	rootHelp := renderHelp(t, root)
	assert.Contains(t, rootHelp, "Beta command [Beta]")
	betaHelp := renderHelp(t, beta)
	assert.Contains(t, betaHelp, "Inherited Beta command")
	assert.NotContains(t, betaHelp, "Inherited Beta command [Beta]")
	inheritedHelp := renderHelp(t, inherited)
	assert.Contains(t, inheritedHelp, "Preview command [Tech Preview]")
}

func TestMaturityCommandHelpAndExceptions(t *testing.T) {
	root := newMaturityHelpRoot()
	root.PersistentFlags().String("shared", "", "shared inherited flag")
	command := &cobra.Command{
		Use:     "feature <mode>",
		Aliases: []string{"feat"},
		Short:   "Feature command",
		Long:    "Feature command long description.",
		Example: "  kongctl feature",
		Run:     maturityTestRun,
	}
	command.Flags().String("experimental", "", "experimental flag")
	command.Flags().String("resources", "", "resources")
	root.AddCommand(command)
	require.NoError(t, maturity.AnnotateCommand(command, maturity.Metadata{
		Level:        maturity.LevelBeta,
		Message:      "This interface may change before GA.",
		ReferenceURL: "https://example.test/maturity",
	}))
	require.NoError(t, maturity.AnnotateFlag(
		command, "experimental", maturity.Metadata{Level: maturity.LevelTechPreview},
	))
	require.NoError(t, maturity.AnnotateFlagValue(
		command, "resources", "preview_services", maturity.Metadata{Level: maturity.LevelTechPreview},
	))
	require.NoError(t, maturity.AnnotateArgument(
		command, "mode", maturity.Metadata{Level: maturity.LevelTechPreview},
	))

	help := renderHelp(t, command)
	assert.Equal(t, 1, bytes.Count([]byte(help), []byte("Maturity:")))
	assert.Contains(t, help, "Maturity:\n  Beta\n  This interface may change before GA.")
	assert.Contains(t, help, "  Learn more: https://example.test/maturity")
	assert.Contains(t, help, "  --experimental: Tech Preview")
	assert.Contains(t, help, "  <mode>: Tech Preview")
	assert.Contains(t, help, "  --resources values:\n    Tech Preview: preview_services")
	assert.NotContains(t, help, "  --shared: Beta")
	assert.Less(t, bytes.Index([]byte(help), []byte("Maturity:")), bytes.Index([]byte(help), []byte("Aliases:")))
	assert.Less(t, bytes.Index([]byte(help), []byte("Maturity:")), bytes.Index([]byte(help), []byte("Flags:")))
}

func TestMaturityGAHelpRemainsUnlabeled(t *testing.T) {
	root := newMaturityHelpRoot()
	command := &cobra.Command{Use: "stable", Short: "Stable command", Run: maturityTestRun}
	command.Flags().Bool("enabled", false, "enabled")
	root.AddCommand(command)

	help := renderHelp(t, command)
	assert.NotContains(t, help, "Maturity:")
	assert.NotContains(t, help, "[GA]")
}

func TestMaturityGACommandLabelsOnlyNarrowExceptions(t *testing.T) {
	root := newMaturityHelpRoot()
	command := &cobra.Command{Use: "stable <mode>", Short: "Stable command", Run: maturityTestRun}
	command.Flags().String("resources", "", "resources")
	root.AddCommand(command)
	require.NoError(t, maturity.AnnotateFlagValue(
		command, "resources", "preview_services", maturity.Metadata{Level: maturity.LevelBeta},
	))
	require.NoError(t, maturity.AnnotateArgumentValue(
		command, "mode", "preview", maturity.Metadata{Level: maturity.LevelTechPreview},
	))

	help := renderHelp(t, command)
	assert.Contains(t, help, "Maturity:\n  --resources values:\n    Beta: preview_services")
	assert.Contains(t, help, "  <mode> values:\n    Tech Preview: preview")
	assert.NotContains(t, help, "Maturity:\n  GA")

	resolved, err := maturity.ResolveCommand(command)
	require.NoError(t, err)
	assert.Equal(t, maturity.LevelGA, resolved.Effective.Level)
}

func TestMaturityRootOverviewLabelsChildren(t *testing.T) {
	root := newMaturityHelpRoot()
	child := &cobra.Command{Use: "preview", Short: "Preview command", Run: maturityTestRun}
	require.NoError(t, maturity.AnnotateCommand(child, maturity.Metadata{Level: maturity.LevelBeta}))
	root.AddCommand(child)

	var output bytes.Buffer
	require.NoError(t, renderRootOverview(&output, root))
	assert.Contains(t, output.String(), "Preview command [Beta]")
}

func TestMaturityMissingSubcommandLabelsLowerMaturityChildren(t *testing.T) {
	root := newMaturityHelpRoot()
	stable := &cobra.Command{Use: "stable", Short: "Stable command", Run: maturityTestRun}
	beta := &cobra.Command{Use: "beta", Short: "Beta command", Run: maturityTestRun}
	require.NoError(t, maturity.AnnotateCommand(beta, maturity.Metadata{Level: maturity.LevelBeta}))
	root.AddCommand(stable, beta)

	var output bytes.Buffer
	err := renderCommandUsageError(&output, root, cmdpkg.MissingSubcommandError(root))
	require.NoError(t, err)
	assert.Contains(t, output.String(), "Available subcommands:\n  beta [Beta]\n  stable\n")
}
