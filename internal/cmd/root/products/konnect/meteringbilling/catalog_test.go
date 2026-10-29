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
	path := filepath.Join(t.TempDir(), "request.yaml")
	run := func(
		verb verbs.VerbValue,
		command []string,
		request string,
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
		if request != "" {
			require.NoError(t, os.WriteFile(path, []byte(request), 0o600))
			require.NoError(t, leaf.Flags().Set("file", path))
		}
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
		out, err := run(verbs.Create, []string{entry.selector}, entry.request, nil, nil)
		require.NoError(t, err)
		require.Contains(t, out, "id")
		for _, format := range []string{"json", "yaml", "text"} {
			cfg.Set(cmdcommon.OutputConfigPath, format)
			_, err = run(verbs.Get, []string{entry.selector}, "", nil, nil)
			require.NoError(t, err)
			_, err = run(verbs.List, []string{entry.selector}, "", nil, nil)
			require.NoError(t, err)
		}
		cfg.Set(cmdcommon.OutputConfigPath, "json")
	}
	apps, _ := server.Snapshot(helpers.BillingApp)
	appID := apps[0].ID()
	_, err := run(
		verbs.Create,
		[]string{"profiles"},
		`name: Profile
supplier: {name: Supplier}
workflow: {}
apps:
  tax: {id: `+appID+`}
  payment: {id: `+appID+`}
  invoicing: {id: `+appID+`}
default: false
`,
		nil,
		nil,
	)
	require.NoError(t, err)
	profiles, _ := server.Snapshot(helpers.BillingProfile)
	profileID := profiles[0].ID()
	_, err = run(verbs.Get, []string{"apps", "catalog"}, "", nil, nil)
	require.NoError(t, err)
	_, err = run(verbs.Get, []string{"apps", "catalog"}, "", []string{"sandbox"}, nil)
	require.NoError(t, err)
	_, err = run(verbs.Get, []string{"plans"}, "", []string{"plan@1"}, nil)
	require.NoError(t, err)
	_, err = run(verbs.Update, []string{"apps"}, `name: Renamed
type: sandbox
`, []string{appID}, nil)
	require.NoError(t, err)
	_, err = run(verbs.Adopt, []string{"apps"}, "", []string{appID}, map[string]string{"namespace": "billing"})
	require.NoError(t, err)
	_, err = run(verbs.Delete, []string{"apps"}, "", []string{appID}, nil)
	require.ErrorContains(t, err, "still references")
	_, err = run(verbs.Delete, []string{"profiles"}, "", []string{profileID}, nil)
	require.NoError(t, err)
	_, err = run(verbs.Delete, []string{"apps"}, "", []string{appID}, nil)
	require.NoError(t, err)
	taxes, _ := server.Snapshot(helpers.BillingTaxCode)
	_, err = run(
		verbs.Adopt,
		[]string{"tax-codes"},
		"",
		[]string{taxes[0].ID()},
		map[string]string{"namespace": "billing"},
	)
	require.NoError(t, err)
	_, err = run(verbs.Update, []string{"tax-codes"}, `name: Updated
app_mappings: []
`, []string{taxes[0].ID()}, nil)
	require.NoError(t, err)
	currency, _ := server.Snapshot(helpers.BillingCurrency)
	_, err = run(verbs.Get, []string{"currencies", "cost-bases"}, "", []string{currency[0].ID()}, nil)
	require.NoError(t, err)
	_, err = run(
		verbs.Create,
		[]string{"currencies", "cost-bases"},
		`fiat_code: USD
rate: '0.1'
effective_from: '2027-01-01T00:00:00Z'
`,
		[]string{currency[0].ID()},
		nil,
	)
	require.NoError(t, err)
	plans, _ := server.Snapshot(helpers.BillingPlan)
	_, err = run(verbs.Update, []string{"plans"}, "", []string{plans[0].ID()}, map[string]string{"publish": "true"})
	require.NoError(t, err)
	_, err = run(verbs.Delete, []string{"plans"}, "", []string{plans[0].ID()}, nil)
	require.ErrorContains(t, err, "draft")
	_, err = run(verbs.Update, []string{"plans"}, "", []string{plans[0].ID()}, map[string]string{"archive": "true"})
	require.NoError(t, err)
	// Generated SDK union errors must never echo a credential-bearing file.
	out, err := run(
		verbs.Create,
		[]string{"apps"},
		`name: Invalid
type: not-an-app
create_billing_profile: false
api_key: do-not-print-this
`,
		nil,
		nil,
	)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "do-not-print-this")
	require.NotContains(t, out, "do-not-print-this")
	// The imperative install shortcut remains available independently of declarative apps.
	_, err = run(verbs.Create, []string{"apps"}, `name: Sandbox with profile
type: sandbox
create_billing_profile: true
`, nil, nil)
	require.NoError(t, err)
	_, writes := server.Snapshot(helpers.BillingApp)
	var installedWithProfile bool
	for _, write := range writes {
		if write.Kind == helpers.BillingApp && write.Method == "POST" &&
			write.Fields["name"] == "Sandbox with profile" {
			installedWithProfile, _ = write.Fields["create_billing_profile"].(bool)
		}
	}
	require.True(t, installedWithProfile)
}
