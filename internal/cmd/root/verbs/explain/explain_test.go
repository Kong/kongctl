package explain

import (
	"bytes"
	"context"
	"testing"

	"github.com/kong/kongctl/internal/cmd/common"
	jqoutput "github.com/kong/kongctl/internal/cmd/output/jq"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRootWithExplain(t *testing.T, configure func(config.Hook)) *cobra.Command {
	t.Helper()

	cobra.EnableTraverseRunHooks = true

	explainCmd, err := NewExplainCmd()
	require.NoError(t, err)

	cfg := config.BuildProfiledConfig("default", "", viper.New())
	if configure != nil {
		configure(cfg)
	}

	root := &cobra.Command{
		Use:              "kongctl",
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
		PersistentPreRun: func(c *cobra.Command, _ []string) {
			if f := c.Flags().Lookup(common.OutputFlagName); f != nil {
				_ = cfg.BindFlag(common.OutputConfigPath, f)
			}
			c.SetContext(context.WithValue(c.Context(), config.ConfigKey, cfg))
		},
	}
	root.PersistentFlags().StringP(
		common.OutputFlagName,
		common.OutputFlagShort,
		common.DefaultOutputFormat,
		"Output format (text|json|yaml)",
	)
	root.AddCommand(explainCmd)

	return root
}

func TestNewExplainCmd_AddsJQFlags(t *testing.T) {
	cmd, err := NewExplainCmd()
	require.NoError(t, err)

	assert.NotNil(t, cmd.PersistentFlags().Lookup(jqoutput.FlagName))
	assert.NotNil(t, cmd.PersistentFlags().Lookup(jqoutput.RawOutputFlagName))
	assert.NotNil(t, cmd.Flags().Lookup(extendedFlagName))
}

func TestExplainCmd_AppliesJQFilterToJSON(t *testing.T) {
	root := newTestRootWithExplain(t, nil)

	var outBuf, errBuf bytes.Buffer
	streams := &iostreams.IOStreams{Out: &outBuf, ErrOut: &errBuf}
	root.SetContext(context.WithValue(context.Background(), iostreams.StreamsKey, streams))
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"explain", "portal.description",
		"--output", "json",
		"--jq", `.["x-kongctl-placement"].yaml_path`,
	})

	err := root.Execute()
	require.NoError(t, err)
	assert.Equal(t, "\"portals[].description\"\n", outBuf.String())
}

func TestExplainCmd_AppliesJQFilterToYAML(t *testing.T) {
	root := newTestRootWithExplain(t, nil)

	var outBuf, errBuf bytes.Buffer
	streams := &iostreams.IOStreams{Out: &outBuf, ErrOut: &errBuf}
	root.SetContext(context.WithValue(context.Background(), iostreams.StreamsKey, streams))
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"explain", "portal.description",
		"--output", "yaml",
		"--jq", `.["x-kongctl-resource"].name`,
	})

	err := root.Execute()
	require.NoError(t, err)
	assert.Equal(t, "portal\n", outBuf.String())
}

func TestExplainCmd_AppliesJQRawOutput(t *testing.T) {
	root := newTestRootWithExplain(t, nil)

	var outBuf, errBuf bytes.Buffer
	streams := &iostreams.IOStreams{Out: &outBuf, ErrOut: &errBuf}
	root.SetContext(context.WithValue(context.Background(), iostreams.StreamsKey, streams))
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"explain", "portal.description",
		"--output", "json",
		"--jq", `.["x-kongctl-placement"].yaml_path`,
		"--jq-raw-output",
	})

	err := root.Execute()
	require.NoError(t, err)
	assert.Equal(t, "portals[].description\n", outBuf.String())
}

func TestExplainCmd_RejectsJQWithTextOutput(t *testing.T) {
	root := newTestRootWithExplain(t, nil)

	var outBuf, errBuf bytes.Buffer
	streams := &iostreams.IOStreams{Out: &outBuf, ErrOut: &errBuf}
	root.SetContext(context.WithValue(context.Background(), iostreams.StreamsKey, streams))
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"explain", "portal.description",
		"--output", "text",
		"--jq", `.["x-kongctl-placement"].yaml_path`,
	})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--jq is only supported with --output json or --output yaml")
}

func TestExplainCmd_ResourceTextIsSummaryByDefault(t *testing.T) {
	root := newTestRootWithExplain(t, nil)

	var outBuf, errBuf bytes.Buffer
	streams := &iostreams.IOStreams{Out: &outBuf, ErrOut: &errBuf}
	root.SetContext(context.WithValue(context.Background(), iostreams.StreamsKey, streams))
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"explain", "portal", "--output", "text"})

	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, outBuf.String(), "FIELD DETAILS: use --extended")
	assert.NotContains(t, outBuf.String(), "\nFIELDS\n- ref: string required")
}

func TestExplainCmd_ResourceTextExtendedIncludesFields(t *testing.T) {
	root := newTestRootWithExplain(t, nil)

	var outBuf, errBuf bytes.Buffer
	streams := &iostreams.IOStreams{Out: &outBuf, ErrOut: &errBuf}
	root.SetContext(context.WithValue(context.Background(), iostreams.StreamsKey, streams))
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"explain", "portal", "--output", "text", "--extended"})

	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, outBuf.String(), "\nFIELDS\n- ref: string required")
	assert.NotContains(t, outBuf.String(), "FIELD DETAILS: use --extended")
}

func TestExplainCmd_RejectsExtendedWithJSONOutput(t *testing.T) {
	root := newTestRootWithExplain(t, nil)

	var outBuf, errBuf bytes.Buffer
	streams := &iostreams.IOStreams{Out: &outBuf, ErrOut: &errBuf}
	root.SetContext(context.WithValue(context.Background(), iostreams.StreamsKey, streams))
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"explain", "portal", "--output", "json", "--extended"})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--extended is only supported with --output text")
}

func TestExplainCmd_TextOutputIgnoresConfiguredDefaultJQExpression(t *testing.T) {
	root := newTestRootWithExplain(t, func(cfg config.Hook) {
		cfg.Set(jqoutput.DefaultExpressionConfigPath, `.["x-kongctl-placement"].yaml_path`)
	})

	var outBuf, errBuf bytes.Buffer
	streams := &iostreams.IOStreams{Out: &outBuf, ErrOut: &errBuf}
	root.SetContext(context.WithValue(context.Background(), iostreams.StreamsKey, streams))
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs([]string{
		"explain", "portal.description",
		"--output", "text",
	})

	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, outBuf.String(), "FIELD\nPATH: portal.description")
}
