package dump

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	r "github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestBillingCatalogDumpRoundTrip(t *testing.T) {
	server := fixture.NewBillingCatalogServer(t)
	api := server.SDK.GetBillingCatalogAPI()
	owned := map[string]string{labels.NamespaceKey: "billing", "team": "billing"}
	tax, err := api.Create(
		t.Context(),
		helpers.BillingTaxCode,
		map[string]any{"name": "Tax", "key": "tax", "app_mappings": []any{}, "labels": owned},
	)
	require.NoError(t, err)
	_, err = api.Create(
		t.Context(),
		helpers.BillingCurrency,
		map[string]any{
			"name":               "Credits",
			"code":               "CREDITS",
			"precision":          0,
			"decimal_mark":       ".",
			"thousand_separator": ",",
		},
	)
	require.NoError(t, err)
	app, err := api.Create(
		t.Context(),
		helpers.BillingApp,
		map[string]any{"name": "Sandbox", "type": "sandbox", "create_billing_profile": false, "labels": owned},
	)
	require.NoError(t, err)
	_, err = api.Create(
		t.Context(),
		helpers.BillingProfile,
		map[string]any{
			"name":     "Profile",
			"supplier": map[string]any{"name": "Supplier"},
			"workflow": map[string]any{},
			"apps": map[string]any{
				"tax":       map[string]any{"id": app.ID()},
				"invoicing": map[string]any{"id": app.ID()},
				"payment":   map[string]any{"id": app.ID()},
			},
			"default": false,
			"labels":  owned,
		},
	)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(
		t,
		json.Unmarshal(
			[]byte(
				`{"name":"Plan" ,
"key":"plan" ,
"currency":"CREDITS" ,
"billing_cadence":"P1M" ,
"phases":[{"name":"Phase" ,
"key":"phase" ,
"rate_cards":[{"name":"Rate" ,
"key":"rate" ,
"price":{"type":"unit" ,
"amount":"0.1"} ,
"tax_config":{"code":{"id":"`+tax.ID()+`"}}}]}]}`,
			),
			&fields,
		),
	)
	fields["labels"] = owned
	_, err = api.Create(t.Context(), helpers.BillingPlan, fields)
	require.NoError(t, err)
	d := &declarativeDumpContext{sdk: server.SDK, pageSize: 1}
	var rs r.ResourceSet
	rs.TaxCodes, err = collectBillingCatalog[r.TaxCodeResource](t.Context(), d, r.ResourceTypeTaxCode)
	require.NoError(t, err)
	rs.Currencies, err = collectBillingCatalog[r.CurrencyResource](t.Context(), d, r.ResourceTypeCurrency)
	require.NoError(t, err)
	rs.BillingApps, err = collectBillingCatalog[r.BillingAppResource](t.Context(), d, r.ResourceTypeBillingApp)
	require.NoError(t, err)
	rs.BillingProfiles, err = collectBillingCatalog[r.BillingProfileResource](
		t.Context(),
		d,
		r.ResourceTypeBillingProfile,
	)
	require.NoError(t, err)
	rs.BillingPlans, err = collectBillingCatalog[r.BillingPlanResource](t.Context(), d, r.ResourceTypeBillingPlan)
	require.NoError(t, err)
	require.Equal(t, app.ID(), rs.BillingApps[0].ID)
	require.Equal(t, tax.ID(), rs.TaxCodes[0].ID)
	data, err := json.Marshal(rs)
	require.NoError(t, err)
	require.NotContains(t, string(data), "api_key")
	require.NotContains(t, string(data), "masked_api_key")
	path := filepath.Join(t.TempDir(), "dump.yaml")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	loaded, err := loader.New().LoadFile(path)
	require.NoError(t, err)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	plan, err := planner.NewPlanner(state.NewClient(state.ClientConfig{BillingCatalogAPI: api}), logger).
		GeneratePlan(t.Context(), loaded, planner.Options{Mode: planner.PlanModeApply})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	d.filter = filterOptions{id: tax.ID()}
	selected, err := collectBillingCatalog[r.TaxCodeResource](t.Context(), d, r.ResourceTypeTaxCode)
	require.NoError(t, err)
	require.Len(t, selected, 1)
}
