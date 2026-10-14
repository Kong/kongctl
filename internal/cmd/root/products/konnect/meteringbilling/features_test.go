package meteringbilling

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/maturity"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/log"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestFeatureCommandsAndOutput(t *testing.T) {
	for _, format := range []string{"text", "json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			server := fixture.NewFeatureServer(t, kkComps.Feature{ID: fixture.FeatureID, Name: "Usage", Key: "usage"})
			family := NewCommand(verbs.Get)
			leaf, _, err := family.Find([]string{"feature"})
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
		family := NewCommand(verb)
		require.NotNil(t, family)
		require.Contains(t, family.Aliases, "mb")
		leaf, _, err := family.Find([]string{"feature"})
		require.NoError(t, err)
		require.Error(t, leaf.Args(leaf, []string{"one", "two"}))
	}
}

func TestFeatureRequestsAndCostQuery(t *testing.T) {
	server := fixture.NewFeatureServer(t)
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
	run := func(verb verbs.VerbValue, request string, args ...string) error {
		leaf, _, err := NewCommand(verb).Find([]string{"features"})
		require.NoError(t, err)
		leaf.SetContext(ctx)
		if request != "" {
			require.NoError(t, os.WriteFile(path, []byte(request), 0o600))
			require.NoError(t, leaf.Flags().Set("file", path))
		}
		return leaf.RunE(leaf, args)
	}
	require.ErrorContains(t, run(verbs.Create, "key: usage"), "name is required")
	require.ErrorContains(
		t,
		run(verbs.Create, "name: Usage\nkey: usage\nunit_cost: {type: manual, amount: '1', typo: true}"),
		"unknown field",
	)
	require.NoError(t, run(verbs.Create, "name: Usage\nkey: usage\nunit_cost: {type: manual, amount: '0.1'}"))
	require.ErrorContains(t, run(verbs.Update, "name: Changed", "usage"), "only unit_cost")
	require.NoError(t, run(verbs.Update, "{}", "usage"))
	values, _ := server.Snapshot()
	require.NotNil(t, values[0].UnitCost)
	require.NoError(t, run(verbs.Update, "unit_cost: null", "usage"))
	values, writes := server.Snapshot()
	require.Nil(t, values[0].UnitCost)
	require.Contains(t, writes[len(writes)-1], "unit_cost")
	require.ErrorContains(t, run(verbs.Adopt, "", "usage"), "adoption is unsupported")
	query, _, err := NewCommand(verbs.Get).Find([]string{"features", "cost-query"})
	require.NoError(t, err)
	resolved, err := maturity.ResolveCommand(query)
	require.NoError(t, err)
	require.Equal(t, maturity.LevelBeta, resolved.Effective.Level)
	query.SetContext(ctx)
	require.NoError(t, os.WriteFile(path, []byte("group_by_dimensions: [region]"), 0o600))
	require.NoError(t, query.Flags().Set("file", path))
	require.NoError(t, query.RunE(query, []string{"usage"}))
	server.Fail(403)
	require.ErrorContains(t, run(verbs.Get, "", "usage"), "list features")
	server.Fail(0)
	server.SetLabels(fixture.FeatureID, map[string]string{labels.ProtectedKey: "true"})
	require.ErrorContains(t, run(verbs.Delete, "", "usage"), "protected")
	server.SetLabels(fixture.FeatureID, nil)
	leaf, _, err := NewCommand(verbs.Delete).Find([]string{"features"})
	require.NoError(t, err)
	leaf.SetContext(ctx)
	cmdpkg.SetDeleteAutoApprove(leaf, true)
	require.NoError(t, leaf.RunE(leaf, []string{"usage"}))
	values, _ = server.Snapshot()
	require.Empty(t, values)
}
