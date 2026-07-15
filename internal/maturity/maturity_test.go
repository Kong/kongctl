package maturity

import (
	"bytes"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLevelValidationAndDisplay(t *testing.T) {
	tests := []struct {
		level   Level
		display string
	}{
		{LevelGA, "GA"},
		{LevelBeta, "Beta"},
		{LevelTechPreview, "Tech Preview"},
	}
	for _, tt := range tests {
		require.NoError(t, Validate(Metadata{Level: tt.level}))
		assert.Equal(t, tt.display, tt.level.DisplayName())
	}
	require.Error(t, Validate(Metadata{Level: "preview"}))
	assert.True(t, LevelTechPreview.LessThan(LevelBeta))
	assert.True(t, LevelBeta.LessThan(LevelGA))
	assert.False(t, LevelGA.LessThan(LevelBeta))
}

func TestCommandMaturityInheritance(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	parent := &cobra.Command{Use: "parent"}
	child := &cobra.Command{Use: "child"}
	leaf := &cobra.Command{Use: "leaf"}
	root.AddCommand(parent)
	parent.AddCommand(child)
	child.AddCommand(leaf)

	resolved, err := ResolveCommand(leaf)
	require.NoError(t, err)
	assert.Equal(t, LevelGA, resolved.Effective.Level)
	assert.Equal(t, KindDefault, resolved.Source.Kind)

	require.NoError(t, AnnotateCommand(parent, Metadata{Level: LevelBeta, Message: "parent beta"}))
	require.NoError(t, AnnotateCommand(child, Metadata{Level: LevelGA, Message: "cannot raise"}))
	resolved, err = ResolveCommand(child)
	require.NoError(t, err)
	require.NotNil(t, resolved.Declared)
	assert.Equal(t, LevelGA, resolved.Declared.Level)
	assert.Equal(t, LevelBeta, resolved.Effective.Level)
	assert.Equal(t, "root parent", resolved.Source.Path)
	assert.Equal(t, "parent beta", resolved.Effective.Message)

	require.NoError(t, AnnotateCommand(leaf, Metadata{Level: LevelTechPreview, Message: "leaf preview"}))
	resolved, err = ResolveCommand(leaf)
	require.NoError(t, err)
	assert.Equal(t, LevelTechPreview, resolved.Effective.Level)
	assert.Equal(t, "root parent child leaf", resolved.Source.Path)

	require.NoError(t, AnnotateCommand(child, Metadata{Level: LevelBeta, Message: "nearest beta"}))
	resolved, err = ResolveCommand(child)
	require.NoError(t, err)
	assert.Equal(t, "nearest beta", resolved.Effective.Message)
	assert.Equal(t, "root parent child", resolved.Source.Path)
}

func TestCapabilityAnnotationsAndResolution(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	child := &cobra.Command{Use: "child <mode>"}
	root.PersistentFlags().String("shared", "", "shared flag")
	child.Flags().String("format", "", "format")
	root.AddCommand(child)

	root.Annotations = map[string]string{"unrelated": "preserved"}
	root.PersistentFlags().Lookup("shared").Annotations = map[string][]string{"unrelated": {"preserved"}}
	require.NoError(t, AnnotateFlag(root, "shared", Metadata{Level: LevelBeta}))
	require.NoError(t, AnnotateCommand(child, Metadata{Level: LevelBeta}))
	require.NoError(t, AnnotateFlagValue(child, "format", "preview", Metadata{Level: LevelTechPreview}))
	require.NoError(t, AnnotateArgument(child, "mode", Metadata{Level: LevelTechPreview}))
	require.NoError(t, AnnotateArgumentValue(child, "mode", "unstable", Metadata{Level: LevelTechPreview}))

	assert.Equal(t, "preserved", root.Annotations["unrelated"])
	assert.Equal(t, []string{"preserved"}, root.PersistentFlags().Lookup("shared").Annotations["unrelated"])

	shared, err := ResolveFlag(child, "shared")
	require.NoError(t, err)
	assert.Equal(t, LevelBeta, shared.Effective.Level)
	assert.Equal(t, "root", shared.Source.Path)

	format, err := ResolveFlag(child, "format")
	require.NoError(t, err)
	assert.Equal(t, LevelBeta, format.Effective.Level)
	formatValue, err := ResolveFlagValue(child, "format", "preview")
	require.NoError(t, err)
	assert.Equal(t, LevelTechPreview, formatValue.Effective.Level)
	assert.Equal(t, KindFlagValue, formatValue.Source.Kind)

	argument, err := ResolveArgument(child, "mode")
	require.NoError(t, err)
	assert.Equal(t, LevelTechPreview, argument.Effective.Level)
	argumentValue, err := ResolveArgumentValue(child, "mode", "unstable")
	require.NoError(t, err)
	assert.Equal(t, LevelTechPreview, argumentValue.Effective.Level)

	arguments, err := DeclaredArguments(child)
	require.NoError(t, err)
	assert.Equal(t, LevelTechPreview, arguments["mode"].Level)
	values, err := DeclaredFlagValues(child.LocalFlags().Lookup("format"))
	require.NoError(t, err)
	assert.Equal(t, LevelTechPreview, values["preview"].Level)
}

func TestAliasUsesCanonicalCommandMetadata(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	child := &cobra.Command{Use: "canonical", Aliases: []string{"alias"}}
	require.NoError(t, AnnotateCommand(child, Metadata{Level: LevelBeta}))
	root.AddCommand(child)

	found, _, err := root.Find([]string{"alias"})
	require.NoError(t, err)
	resolved, err := ResolveCommand(found)
	require.NoError(t, err)
	assert.Equal(t, "root canonical", resolved.Source.Path)
}

func TestAnnotationValidationErrors(t *testing.T) {
	command := &cobra.Command{Use: "command"}
	command.Flags().String("local", "", "local")

	require.Error(t, AnnotateCommand(nil, Metadata{Level: LevelBeta}))
	require.Error(t, AnnotateCommand(command, Metadata{Level: "invalid"}))
	require.Error(t, AnnotateFlag(command, "missing", Metadata{Level: LevelBeta}))
	require.Error(t, AnnotateArgument(command, "", Metadata{Level: LevelBeta}))
	require.Error(t, AnnotateFlagValue(command, "local", "", Metadata{Level: LevelBeta}))

	command.Annotations = map[string]string{annotationKey: "{"}
	_, err := ResolveCommand(command)
	require.Error(t, err)
	command.Annotations[annotationKey] = `{"command":{"level":"invalid"}}`
	_, err = ResolveCommand(command)
	require.Error(t, err)
}

func TestAnnotatedCommandExecutionIsUnchanged(t *testing.T) {
	var stdout, stderr bytes.Buffer
	command := &cobra.Command{
		Use:          "preview",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := cmd.OutOrStdout().Write([]byte("result\n"))
			return err
		},
	}
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	require.NoError(t, AnnotateCommand(command, Metadata{Level: LevelBeta, Message: "beta"}))
	require.NoError(t, command.Execute())
	assert.Equal(t, "result\n", stdout.String())
	assert.Empty(t, stderr.String())

	command.RunE = func(*cobra.Command, []string) error { return errors.New("same error") }
	err := command.Execute()
	require.EqualError(t, err, "same error")
}
