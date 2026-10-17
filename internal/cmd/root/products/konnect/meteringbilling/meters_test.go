package meteringbilling

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/log"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestMeterCommandsAndOutput(t *testing.T) {
	for _, format := range []string{"text", "json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			server := fixture.NewMeterServer(t, kkComps.Meter{ID: fixture.MeterID, Name: "Usage", Key: "usage"})
			family := newTestCommand(t, verbs.Get)
			leaf, _, err := family.Find([]string{"meter"})
			require.NoError(t, err)
			var output bytes.Buffer
			cfg := config.BuildProfiledConfig("default", "", viper.New())
			cfg.Set(cmdcommon.OutputConfigPath, format)
			cfg.Set(common.RequestPageSizeConfigPath, 1)
			ctx := context.WithValue(t.Context(), config.ConfigKey, cfg)
			ctx = context.WithValue(ctx, log.LoggerKey, slog.New(slog.NewTextHandler(io.Discard, nil)))
			ctx = context.WithValue(
				ctx,
				iostreams.StreamsKey,
				&iostreams.IOStreams{Out: &output, ErrOut: io.Discard, In: bytes.NewReader(nil)},
			)
			ctx = context.WithValue(
				ctx,
				helpers.SDKAPIFactoryKey,
				helpers.SDKAPIFactory(
					func(config.Hook, *slog.Logger) (helpers.SDKAPI, error) { return server.SDK, nil },
				),
			)
			leaf.SetContext(ctx)
			require.NoError(t, leaf.RunE(leaf, []string{"usage"}))
			require.Contains(t, output.String(), "usage")
		})
	}
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List, verbs.Create, verbs.Update, verbs.Delete, verbs.Adopt} {
		family := newTestCommand(t, verb)
		require.NotNil(t, family)
		require.Contains(t, family.Aliases, "mb")
		leaf, _, err := family.Find([]string{"meter"})
		require.NoError(t, err)
		require.Error(t, leaf.Args(leaf, []string{"one", "two"}))
	}
}

func TestMeterImperativeMutationAndStrictRequests(t *testing.T) {
	server := fixture.NewMeterServer(t)
	cfg := config.BuildProfiledConfig("default", "", viper.New())
	cfg.Set(cmdcommon.OutputConfigPath, "json")
	ctx := context.WithValue(t.Context(), config.ConfigKey, cfg)
	ctx = context.WithValue(ctx, log.LoggerKey, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx = context.WithValue(
		ctx,
		iostreams.StreamsKey,
		&iostreams.IOStreams{Out: io.Discard, ErrOut: io.Discard, In: bytes.NewReader(nil)},
	)
	ctx = context.WithValue(
		ctx,
		helpers.SDKAPIFactoryKey,
		helpers.SDKAPIFactory(func(config.Hook, *slog.Logger) (helpers.SDKAPI, error) { return server.SDK, nil }),
	)
	path := filepath.Join(t.TempDir(), "request.yaml")
	run := func(verb verbs.VerbValue, request string, args []string) error {
		family := newTestCommand(t, verb)
		leaf, _, err := family.Find([]string{"meters"})
		require.NoError(t, err)
		leaf.SetContext(ctx)
		if request != "" {
			require.NoError(t, os.WriteFile(path, []byte(request), 0o600))
			require.NoError(t, leaf.Flags().Set("file", path))
		}
		return leaf.RunE(leaf, args)
	}
	require.NoError(t, run(verbs.Create, "name: Usage\nkey: usage\naggregation: count\nevent_type: request\n", nil))
	require.ErrorContains(t, run(verbs.Update, "aggregation: sum\n", []string{"usage"}), "unknown field")
	require.NoError(t, run(verbs.Update, "name: Renamed\ndimensions: {}\n", []string{"usage"}))
	{
		family := newTestCommand(t, verbs.Get)
		query, _, err := family.Find([]string{"meters", "query"})
		require.NoError(t, err)
		query.SetContext(ctx)
		require.NoError(t, os.WriteFile(path, []byte("group_by_dimensions: [region]\n"), 0o600))
		require.NoError(t, query.Flags().Set("file", path))
		require.NoError(t, query.RunE(query, []string{"usage"}))
	}
	family := newTestCommand(t, verbs.Adopt)
	leaf, _, err := family.Find([]string{"meter"})
	require.NoError(t, err)
	leaf.SetContext(ctx)
	require.NoError(t, leaf.Flags().Set("namespace", "billing"))
	require.NoError(t, leaf.RunE(leaf, []string{"usage"}))
	require.ErrorContains(t, leaf.RunE(leaf, []string{"usage"}), "already belongs")
	server.SetLabels(fixture.MeterID, map[string]string{labels.NamespaceKey: "billing", labels.ProtectedKey: "true"})
	require.ErrorContains(t, run(verbs.Delete, "", []string{"usage"}), "protected")
	server.SetLabels(fixture.MeterID, map[string]string{labels.NamespaceKey: "billing"})
	family = newTestCommand(t, verbs.Delete)
	leaf, _, err = family.Find([]string{"meter"})
	require.NoError(t, err)
	leaf.SetContext(ctx)
	require.NoError(t, leaf.Flags().Set("auto-approve", "true"))
	cmdpkg.SetDeleteAutoApprove(leaf, true)
	require.NoError(t, leaf.RunE(leaf, []string{"usage"}))
	values, _ := server.Snapshot()
	require.Empty(t, values)
}

func newTestCommand(t *testing.T, verb verbs.VerbValue) *cobra.Command {
	t.Helper()
	command, err := NewCommand(verb)
	require.NoError(t, err)
	return command
}
