package helpers_test

import (
	"encoding/json"
	"testing"

	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestBillingRequestContracts(t *testing.T) {
	tests := []struct {
		name   string
		kind   helpers.BillingKind
		body   string
		update bool
		want   string
	}{
		{"tax", helpers.BillingTaxCode, `{"name":"Tax","key":"tax","app_mappings":[]}`, false, ""},
		{"missing mappings", helpers.BillingTaxCode, `{"name":"Tax","key":"tax"}`, false, "app_mappings"},
		{"tax put", helpers.BillingTaxCode, `{"name":"Tax","app_mappings":[]}`, true, ""},
		{
			"immutable tax key",
			helpers.BillingTaxCode,
			`{"name":"Tax","key":"tax","app_mappings":[]}`,
			true,
			"unsupported",
		},
		{
			"custom zero precision",
			helpers.BillingCurrency,
			`{
  "name": "Credits",
  "code": "CREDITS",
  "precision": 0,
  "decimal_mark": ".",
  "thousand_separator": ","
}`,
			false,
			"",
		},
		{
			"missing precision",
			helpers.BillingCurrency,
			`{"name":"Credits","code":"CREDITS","decimal_mark":".","thousand_separator":","}`,
			false,
			"precision",
		},
		{
			"no currency patch",
			helpers.BillingCurrency,
			`{
  "name": "Credits",
  "code": "CREDITS",
  "precision": 0,
  "decimal_mark": ".",
  "thousand_separator": ","
}`,
			true,
			"cannot be updated",
		},
		{
			"explicit app side effect",
			helpers.BillingApp,
			`{"name":"Sandbox","type":"sandbox","create_billing_profile":false}`,
			false,
			"",
		},
		{
			"missing app side effect",
			helpers.BillingApp,
			`{"name":"Sandbox","type":"sandbox"}`,
			false,
			"create_billing_profile",
		},
		{
			"external install",
			helpers.BillingApp,
			`{"name":"External","type":"external_invoicing","create_billing_profile":false}`,
			false,
			"",
		},
		{
			"external managed",
			helpers.BillingApp,
			`{
  "name": "External",
  "type": "external_invoicing",
  "create_billing_profile": false,
  "labels": {
    "team": "billing"
  },
  "enable_draft_sync_hook": false,
  "enable_issuing_sync_hook": true
}`,
			false,
			"",
		},
		{
			"stripe env",
			helpers.BillingApp,
			`{
  "name": "Stripe",
  "type": "stripe",
  "create_billing_profile": false,
  "api_key_env": "STRIPE_KEY"
}`,
			false,
			"",
		},
		{
			"unknown null",
			helpers.BillingApp,
			`{"name":"Sandbox","type":"sandbox","create_billing_profile":false,"typo":null}`,
			false,
			"unsupported",
		},
		{
			"wrong variant fields",
			helpers.BillingApp,
			`{
  "name": "Sandbox",
  "type": "sandbox",
  "create_billing_profile": false,
  "api_key": "do-not-print-this"
}`,
			false,
			"unsupported",
		},
		{
			"profile false default",
			helpers.BillingProfile,
			`{
  "name": "Profile",
  "supplier": {},
  "workflow": {},
  "apps": {
    "tax": {
      "id": "tax"
    },
    "payment": {
      "id": "pay"
    },
    "invoicing": {
      "id": "invoice"
    }
  },
  "default": false
}`,
			false,
			"",
		},
		{
			"profile missing default",
			helpers.BillingProfile,
			`{
  "name": "Profile",
  "supplier": {},
  "workflow": {},
  "apps": {
    "tax": {
      "id": "tax"
    },
    "payment": {
      "id": "pay"
    },
    "invoicing": {
      "id": "invoice"
    }
  }
}`,
			false,
			"default",
		},
		{
			"profile missing app role",
			helpers.BillingProfile,
			`{
  "name": "Profile",
  "supplier": {},
  "workflow": {},
  "apps": {
    "tax": {
      "id": "tax"
    }
  },
  "default": false
}`,
			false,
			"required",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var fields map[string]any
			require.NoError(t, json.Unmarshal([]byte(test.body), &fields))
			err := helpers.ValidateBillingFields(test.kind, fields, test.update)
			if test.want == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.want)
				require.NotContains(t, err.Error(), "do-not-print-this")
			}
		})
	}
}

func TestBillingPlanPricingUnions(t *testing.T) {
	for _, price := range []string{
		`{"type":"free"}`,
		`{"type":"flat","amount":"10"}`,
		`{"type":"unit","amount":"0.001"}`,
		`{"type":"graduated","tiers":[{"unit_price":{"type":"unit","amount":"1"}}]}`,
		`{"type":"volume","tiers":[{"unit_price":{"type":"unit","amount":"1"}}]}`,
	} {
		for _, entitlement := range []string{
			`{"type":"boolean"}`,
			`{"type":"static","config":{"access":true}}`,
			`{"type":"metered","is_soft_limit":false,"usage_period":"P1M"}`,
		} {
			var fields map[string]any
			require.NoError(
				t,
				json.Unmarshal(
					[]byte(
						`{"name":"Plan" ,
"key":"plan" ,
"currency":"USD" ,
"billing_cadence":"P1M" ,
"phases":[{"name":"Phase" ,
"key":"phase" ,
"rate_cards":[{"name":"Rate" ,
"key":"rate" ,
"feature":{"id":"feature"} ,
"price":`+price+`,"entitlement":`+entitlement+`}]}]}`,
					),
					&fields,
				),
			)
			require.NoError(t, helpers.ValidateBillingFields(helpers.BillingPlan, fields, false))
			normalized, err := helpers.NormalizeBillingFields(helpers.BillingPlan, fields, false)
			require.NoError(t, err)
			require.NotEmpty(t, normalized["phases"])
		}
	}
}

func TestBillingSDKPaginationCatalogAndCostBases(t *testing.T) {
	server := fixture.NewBillingCatalogServer(t)
	api := server.SDK.GetBillingCatalogAPI()
	for _, name := range []string{"One", "Two", "Three"} {
		_, err := api.Create(
			t.Context(),
			helpers.BillingTaxCode,
			map[string]any{"name": name, "key": name, "app_mappings": []any{}},
		)
		require.NoError(t, err)
	}
	values, err := api.List(t.Context(), helpers.BillingTaxCode, 1)
	require.NoError(t, err)
	require.Len(t, values, 3)
	require.NotEqual(t, values[0].ID(), values[2].ID())
	catalog, err := api.Catalog(t.Context(), "", 1)
	require.NoError(t, err)
	require.NotEmpty(t, catalog)
	_, err = api.Catalog(t.Context(), "sandbox", 1)
	require.NoError(t, err)
	currency, err := api.Create(
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
	_, err = api.Get(t.Context(), helpers.BillingCurrency, currency.ID())
	require.NoError(t, err)
	_, err = api.CostBases(t.Context(), currency.ID(), 1)
	require.NoError(t, err)
	require.Error(t, api.Delete(t.Context(), helpers.BillingCurrency, currency.ID()))
}
