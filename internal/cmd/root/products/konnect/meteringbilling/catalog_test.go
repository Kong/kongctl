package meteringbilling

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
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
	"sigs.k8s.io/yaml"
)

func TestBillingCatalogCommands(t *testing.T) {
	server := fixture.NewBillingCatalogServer(t)
	cfg := config.BuildProfiledConfig("default", "", viper.New())
	cfg.Set(cmdcommon.OutputConfigPath, "json")
	cfg.Set(common.RequestPageSizeConfigPath, 1)
	var output bytes.Buffer
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
		helpers.SDKAPIFactory(func(config.Hook, *slog.Logger) (helpers.SDKAPI, error) { return server.SDK, nil }),
	)
	run := func(
		verb verbs.VerbValue,
		command []string,
		args []string,
		flags map[string]string,
	) (string, error) {
		t.Helper()
		family := newTestCommand(t, verb)
		leaf, remaining, err := family.Find(command)
		require.NoError(t, err)
		require.Empty(t, remaining)
		leaf.SetContext(ctx)
		output.Reset()
		for key, value := range flags {
			require.NoError(t, leaf.Flags().Set(key, value))
		}
		if verb == verbs.Delete {
			cmdpkg.SetDeleteAutoApprove(leaf, true)
		}
		require.NoError(t, leaf.Args(leaf, args))
		err = leaf.RunE(leaf, args)
		return output.String(), err
	}
	for _, entry := range []struct {
		selector string
		kind     helpers.BillingKind
		request  string
	}{
		{"tax-codes", helpers.BillingTaxCode, `name: Tax
key: tax
app_mappings: []
`},
		{"currencies", helpers.BillingCurrency, `name: Credits
code: CREDITS
precision: 0
decimal_mark: '.'
thousand_separator: ','
`},
		{"apps", helpers.BillingApp, `name: Sandbox
type: sandbox
create_billing_profile: false
`},
		{"plans", helpers.BillingPlan, `name: Plan
key: plan
currency: USD
billing_cadence: P1M
phases: [{name: Phase, key: phase, rate_cards: []}]
`},
	} {
		data, err := yaml.YAMLToJSONStrict([]byte(entry.request))
		require.NoError(t, err)
		var fields map[string]any
		require.NoError(t, json.Unmarshal(data, &fields))
		created, err := server.SDK.GetBillingCatalogAPI().Create(t.Context(), entry.kind, fields)
		require.NoError(t, err)
		for _, format := range []string{"json", "yaml", "text"} {
			cfg.Set(cmdcommon.OutputConfigPath, format)
			out, err := run(verbs.Get, []string{entry.selector}, nil, nil)
			require.NoError(t, err)
			require.Contains(t, out, created.ID())
			_, err = run(verbs.List, []string{entry.selector}, nil, nil)
			require.NoError(t, err)
		}
		cfg.Set(cmdcommon.OutputConfigPath, "json")
	}
	apps, _ := server.Snapshot(helpers.BillingApp)
	appID := apps[0].ID()
	_, err := server.SDK.GetBillingCatalogAPI().Create(t.Context(), helpers.BillingProfile, map[string]any{
		"name": "Profile", "supplier": map[string]any{"name": "Supplier"}, "workflow": map[string]any{},
		"apps": map[string]any{
			"tax": map[string]any{"id": appID}, "payment": map[string]any{"id": appID},
			"invoicing": map[string]any{"id": appID},
		}, "default": false,
	})
	require.NoError(t, err)
	profiles, _ := server.Snapshot(helpers.BillingProfile)
	profileID := profiles[0].ID()
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		_, err = run(verb, []string{"profiles"}, nil, nil)
		require.NoError(t, err)
	}
	_, err = run(verbs.Get, []string{"profiles"}, []string{profileID}, nil)
	require.NoError(t, err)
	_, err = run(verbs.Get, []string{"apps", "catalog"}, nil, nil)
	require.NoError(t, err)
	_, err = run(verbs.Get, []string{"apps", "catalog"}, []string{"sandbox"}, nil)
	require.NoError(t, err)
	_, err = run(verbs.Get, []string{"plans"}, []string{"plan@1"}, nil)
	require.NoError(t, err)
	_, err = run(verbs.Adopt, []string{"apps"}, []string{appID}, map[string]string{"namespace": "billing"})
	require.NoError(t, err)
	_, err = run(verbs.Delete, []string{"apps"}, []string{appID}, nil)
	require.ErrorContains(t, err, "still references")
	_, err = run(verbs.Delete, []string{"profiles"}, []string{profileID}, nil)
	require.NoError(t, err)
	_, err = run(verbs.Delete, []string{"apps"}, []string{appID}, nil)
	require.NoError(t, err)
	taxes, _ := server.Snapshot(helpers.BillingTaxCode)
	_, err = run(
		verbs.Adopt,
		[]string{"tax-codes"},
		[]string{taxes[0].ID()},
		map[string]string{"namespace": "billing"},
	)
	require.NoError(t, err)
	currency, _ := server.Snapshot(helpers.BillingCurrency)
	_, err = run(verbs.Get, []string{"currencies", "cost-bases"}, []string{currency[0].ID()}, nil)
	require.NoError(t, err)
	plans, _ := server.Snapshot(helpers.BillingPlan)
	server.Mutate(helpers.BillingPlan, plans[0].ID(), func(v helpers.BillingObject) { v["status"] = "active" })
	_, err = run(verbs.Delete, []string{"plans"}, []string{plans[0].ID()}, nil)
	require.ErrorContains(t, err, "draft")
}
