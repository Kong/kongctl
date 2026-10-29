//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/executor"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/log"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

const billingCatalogManifest = `_defaults:
  kongctl:
    namespace: billing
metering-billing:
  currencies:
    - ref: credits
      name: Credits
      code: CREDITS
      precision: 0
      decimal_mark: '.'
      thousand_separator: ','
  tax_codes:
    - ref: tax
      name: Service tax
      key: service-tax
      app_mappings: []
  apps:
    - ref: sandbox
      name: Sandbox
      type: sandbox
  profiles:
    - ref: profile
      name: Profile
      supplier: {name: Supplier}
      workflow: {}
      apps:
        tax: {id: sandbox}
        invoicing: {id: sandbox}
        payment: {id: sandbox}
      default: false
  plans:
    - ref: plan
      name: Starter
      key: starter
      currency: credits
      billing_cadence: P1M
      phases:
        - name: Standard
          key: standard
          rate_cards:
            - name: Service
              key: service
              price: {type: flat, amount: '10'}
              tax_config: {code: {id: tax}}
`

func TestBillingCatalogDeclarativeLifecycle(t *testing.T) {
	server := fixture.NewBillingCatalogServer(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.WithValue(t.Context(), log.LoggerKey, logger)
	client := state.NewClient(state.ClientConfig{BillingCatalogAPI: server.SDK.GetBillingCatalogAPI()})
	p := planner.NewPlanner(client, logger)
	path := filepath.Join(t.TempDir(), "catalog.yaml")
	write := func(content string) { t.Helper(); require.NoError(t, os.WriteFile(path, []byte(content), 0o600)) }
	plan := func(mode planner.PlanMode) (*planner.Plan, error) {
		t.Helper()
		rs, err := loader.New().LoadFile(path)
		if err != nil {
			return nil, err
		}
		return p.GeneratePlan(ctx, rs, planner.Options{Mode: mode})
	}
	apply := func(plan *planner.Plan) {
		t.Helper()
		data, err := json.Marshal(plan)
		require.NoError(t, err)
		require.NotContains(t, string(data), "create_billing_profile")
		var saved planner.Plan
		require.NoError(t, json.Unmarshal(data, &saved))
		result := executor.New(client, nil, false).Execute(ctx, &saved)
		require.Zero(t, result.FailureCount, "%+v", result.Errors)
	}
	write(billingCatalogManifest)
	first, err := plan(planner.PlanModeApply)
	require.NoError(t, err)
	require.Len(t, first.Changes, 5)
	byKind := map[string]planner.PlannedChange{}
	for _, change := range first.Changes {
		byKind[change.ResourceType] = change
	}
	require.Contains(t, byKind[planner.ResourceTypeBillingProfile].DependsOn, byKind[planner.ResourceTypeBillingApp].ID)
	require.Contains(t, byKind[planner.ResourceTypeBillingPlan].DependsOn, byKind[planner.ResourceTypeTaxCode].ID)
	require.Contains(t, byKind[planner.ResourceTypeBillingPlan].DependsOn, byKind[planner.ResourceTypeCurrency].ID)
	apply(first)
	apps, writes := server.Snapshot(helpers.BillingApp)
	require.Len(t, apps, 1)
	require.Equal(t, "billing", apps[0].Labels()[labels.NamespaceKey])
	require.Equal(t, "sandbox", apps[0].String("type"))
	var install map[string]any
	for _, write := range writes {
		if write.Kind == helpers.BillingApp && write.Method == "POST" {
			install = write.Fields
		}
	}
	require.Equal(t, false, install["create_billing_profile"])
	require.NotContains(t, install, "labels")
	profiles, _ := server.Snapshot(helpers.BillingProfile)
	require.Equal(t, apps[0].ID(), profiles[0]["apps"].(map[string]any)["tax"].(map[string]any)["id"])
	repeat, err := plan(planner.PlanModeApply)
	require.NoError(t, err)
	require.Empty(t, repeat.Changes)
	// Soft-deleted keys must not be implicitly recreated, even though normal
	// imperative listing excludes deleted records.
	taxBefore, _ := server.Snapshot(helpers.BillingTaxCode)
	server.Mutate(
		helpers.BillingTaxCode,
		taxBefore[0].ID(),
		func(v helpers.BillingObject) { v["deleted_at"] = "2026-10-28T01:00:00Z" },
	)
	_, err = plan(planner.PlanModeApply)
	require.ErrorContains(t, err, "soft-deleted")
	server.Mutate(helpers.BillingTaxCode, taxBefore[0].ID(), func(v helpers.BillingObject) { delete(v, "deleted_at") })
	// Existing unowned apps require explicit adoption rather than reinstallation.
	server.Mutate(helpers.BillingApp, apps[0].ID(), func(v helpers.BillingObject) { delete(v, "labels") })
	_, err = plan(planner.PlanModeApply)
	require.ErrorContains(t, err, "adopt")
	server.Mutate(
		helpers.BillingApp,
		apps[0].ID(),
		func(v helpers.BillingObject) { v["labels"] = map[string]string{labels.NamespaceKey: "billing"} },
	)
	// Every mutable resource supports a full native PUT followed by a no-op.
	updated := strings.ReplaceAll(billingCatalogManifest, "Service tax", "Updated tax")
	updated = strings.ReplaceAll(updated, "name: Starter", "name: Updated starter")
	updated = strings.ReplaceAll(updated, "name: Profile", "name: Updated profile")
	updated = strings.ReplaceAll(updated, "name: Sandbox", "name: Updated sandbox")
	// Names are not unique identities: pin ULIDs before renaming profiles/apps.
	updated = strings.Replace(updated, "    - ref: profile", "    - ref: profile\n      id: "+profiles[0].ID(), 1)
	updated = strings.Replace(updated, "    - ref: sandbox", "    - ref: sandbox\n      id: "+apps[0].ID(), 1)
	write(updated)
	update, err := plan(planner.PlanModeApply)
	require.NoError(t, err)
	require.Len(t, update.Changes, 4)
	apply(update)
	repeat, err = plan(planner.PlanModeApply)
	require.NoError(t, err)
	require.Empty(t, repeat.Changes)
	// Stale saved plans may not mutate resources moved to another namespace.
	write(strings.Replace(updated, "Updated tax", "Another tax", 1))
	stale, err := plan(planner.PlanModeApply)
	require.NoError(t, err)
	tax, _ := server.Snapshot(helpers.BillingTaxCode)
	server.Mutate(
		helpers.BillingTaxCode,
		tax[0].ID(),
		func(v helpers.BillingObject) { v["labels"] = map[string]string{labels.NamespaceKey: "foreign"} },
	)
	result := executor.New(client, nil, false).Execute(ctx, stale)
	require.Equal(t, 1, result.FailureCount)
	require.Contains(t, result.Errors[0].Error, "no longer owned")
	server.Mutate(
		helpers.BillingTaxCode,
		tax[0].ID(),
		func(v helpers.BillingObject) { v["labels"] = map[string]string{labels.NamespaceKey: "billing"} },
	)
	write(updated)
	plans, _ := server.Snapshot(helpers.BillingPlan)
	_, err = server.SDK.GetBillingCatalogAPI().PlanAction(ctx, plans[0].ID(), "publish")
	require.NoError(t, err)
	repeat, err = plan(planner.PlanModeApply)
	require.NoError(t, err)
	require.Empty(t, repeat.Changes, "publication must not create another draft")
	write(strings.Replace(updated, "Updated starter", "Changed published", 1))
	_, err = plan(planner.PlanModeApply)
	require.ErrorContains(t, err, "explicit draft")
	write(strings.Replace(updated, "name: Credits", "name: Other credits", 1))
	_, err = plan(planner.PlanModeApply)
	require.ErrorContains(t, err, "cannot be updated")
}

func TestBillingCatalogDeleteAndUnprotect(t *testing.T) {
	server := fixture.NewBillingCatalogServer(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.WithValue(t.Context(), log.LoggerKey, logger)
	client := state.NewClient(state.ClientConfig{BillingCatalogAPI: server.SDK.GetBillingCatalogAPI()})
	p := planner.NewPlanner(client, logger)
	path := filepath.Join(t.TempDir(), "catalog.yaml")
	require.NoError(t, os.WriteFile(path, []byte(billingCatalogManifest), 0o600))
	rs, err := loader.New().LoadFile(path)
	require.NoError(t, err)
	first, err := p.GeneratePlan(ctx, rs, planner.Options{Mode: planner.PlanModeApply})
	require.NoError(t, err)
	result := executor.New(client, nil, false).Execute(ctx, first)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	// Plan-time checks inspect references even outside the selected resource scope.
	_, err = p.GeneratePlan(
		ctx,
		&resources.ResourceSet{TaxCodes: rs.TaxCodes},
		planner.Options{Mode: planner.PlanModeDelete},
	)
	require.ErrorContains(t, err, "still references")
	_, err = p.GeneratePlan(
		ctx,
		&resources.ResourceSet{BillingApps: rs.BillingApps},
		planner.Options{Mode: planner.PlanModeDelete},
	)
	require.ErrorContains(t, err, "still references")
	tax, _ := server.Snapshot(helpers.BillingTaxCode)
	server.Mutate(helpers.BillingTaxCode, tax[0].ID(), func(v helpers.BillingObject) {
		v["labels"] = map[string]string{labels.NamespaceKey: "billing", labels.ProtectedKey: "true"}
	})
	// Metadata-only unprotection passes, whereas a forged changed-fields map
	// cannot authorize simultaneous public-field changes.
	unprotect, err := p.GeneratePlan(ctx, rs, planner.Options{Mode: planner.PlanModeApply})
	require.NoError(t, err)
	require.Len(t, unprotect.Changes, 1)
	data, err := json.Marshal(unprotect)
	require.NoError(t, err)
	var tampered planner.Plan
	require.NoError(t, json.Unmarshal(data, &tampered))
	tampered.Changes[0].Fields[planner.FieldName] = "Unexpected change"
	result = executor.New(client, nil, false).Execute(ctx, &tampered)
	require.Equal(t, 1, result.FailureCount)
	result = executor.New(client, nil, false).Execute(ctx, unprotect)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	// Omitted currencies are never pruned. Dependents execute before their targets.
	empty := `_defaults:
  kongctl:
    namespace: billing
metering-billing:
  tax_codes: []
  currencies: []
  apps: []
  profiles: []
  plans: []
`
	require.NoError(t, os.WriteFile(path, []byte(empty), 0o600))
	scope, err := loader.New().LoadFile(path)
	require.NoError(t, err)
	deletion, err := p.GeneratePlan(ctx, scope, planner.Options{Mode: planner.PlanModeSync})
	require.NoError(t, err)
	require.Len(t, deletion.Changes, 4)
	byKind := map[string]planner.PlannedChange{}
	for _, c := range deletion.Changes {
		byKind[c.ResourceType] = c
	}
	require.Contains(t, byKind[planner.ResourceTypeTaxCode].DependsOn, byKind[planner.ResourceTypeBillingPlan].ID)
	require.Contains(t, byKind[planner.ResourceTypeBillingApp].DependsOn, byKind[planner.ResourceTypeBillingProfile].ID)
	// A new reference after planning blocks the saved deletion at execution time.
	profiles, _ := server.Snapshot(helpers.BillingProfile)
	server.Mutate(helpers.BillingProfile, profiles[0].ID(), func(v helpers.BillingObject) { v["default"] = true })
	result = executor.New(client, nil, false).Execute(ctx, deletion)
	require.NotZero(t, result.FailureCount)
	server.Mutate(helpers.BillingProfile, profiles[0].ID(), func(v helpers.BillingObject) { v["default"] = false })
	deletion, err = p.GeneratePlan(ctx, scope, planner.Options{Mode: planner.PlanModeSync})
	require.NoError(t, err)
	result = executor.New(client, nil, false).Execute(ctx, deletion)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	currencies, _ := server.Snapshot(helpers.BillingCurrency)
	require.Len(t, currencies, 1)
	for _, kind := range []helpers.BillingKind{
		helpers.BillingTaxCode,
		helpers.BillingApp,
		helpers.BillingProfile,
		helpers.BillingPlan,
	} {
		values, _ := server.Snapshot(kind)
		require.Empty(t, values)
	}
}

func TestBillingCatalogFeatureReferencesAcrossNamespaces(t *testing.T) {
	for _, reference := range []string{"usage-feature", "!ref usage-feature#id"} {
		t.Run(reference, func(t *testing.T) {
			server := fixture.NewBillingCatalogServer(t)
			features := fixture.NewFeatureServer(t)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			ctx := context.WithValue(t.Context(), log.LoggerKey, logger)
			client := state.NewClient(
				state.ClientConfig{
					BillingCatalogAPI: server.SDK.GetBillingCatalogAPI(),
					FeaturesAPI:       features.SDK.GetFeaturesAPI(),
				},
			)
			p := planner.NewPlanner(client, logger)
			manifest := strings.Replace(
				billingCatalogManifest,
				"  plans:\n    - ref: plan",
				"  plans:\n    - ref: plan\n      kongctl: {namespace: a-plan}",
				1,
			)
			manifest = strings.Replace(
				manifest,
				"price: {type: flat, amount: '10'}",
				"feature: {id: "+reference+"}\n              price: {type: unit, amount: '0.1'}",
				1,
			)
			manifest += `  features:
    - ref: usage-feature
      name: Usage
      key: usage
      kongctl: {namespace: z-feature}
`
			path := filepath.Join(t.TempDir(), "catalog.yaml")
			require.NoError(t, os.WriteFile(path, []byte(manifest), 0o600))
			rs, err := loader.New().LoadFile(path)
			require.NoError(t, err)
			plan, err := p.GeneratePlan(ctx, rs, planner.Options{Mode: planner.PlanModeApply})
			require.NoError(t, err)
			require.Len(t, plan.Changes, 6)
			data, err := json.Marshal(plan)
			require.NoError(t, err)
			var saved planner.Plan
			require.NoError(t, json.Unmarshal(data, &saved))
			result := executor.New(client, nil, false).Execute(ctx, &saved)
			require.Zero(t, result.FailureCount, "%+v", result.Errors)
			plans, _ := server.Snapshot(helpers.BillingPlan)
			phases := plans[0]["phases"].([]any)
			rate := phases[0].(map[string]any)["rate_cards"].([]any)[0].(map[string]any)
			require.Equal(t, fixture.FeatureID, rate["feature"].(map[string]any)["id"])
			repeat, err := p.GeneratePlan(ctx, rs, planner.Options{Mode: planner.PlanModeApply})
			require.NoError(t, err)
			require.Empty(t, repeat.Changes)
			_, err = p.GeneratePlan(
				ctx,
				&resources.ResourceSet{Features: rs.Features},
				planner.Options{Mode: planner.PlanModeDelete},
			)
			require.ErrorContains(t, err, "still references")
		})
	}
}

func TestBillingCatalogAppVariantsAndSecrets(t *testing.T) {
	for _, typ := range []string{"stripe", "external_invoicing"} {
		t.Run(typ, func(t *testing.T) {
			server := fixture.NewBillingCatalogServer(t)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			ctx := context.WithValue(t.Context(), log.LoggerKey, logger)
			client := state.NewClient(state.ClientConfig{BillingCatalogAPI: server.SDK.GetBillingCatalogAPI()})
			p := planner.NewPlanner(client, logger)
			manifest := strings.Replace(billingCatalogManifest, "type: sandbox", "type: "+typ, 1)
			if typ == "stripe" {
				t.Setenv("BILLING_TEST_STRIPE_KEY", "stripe-test-secret-value")
				manifest = strings.Replace(
					manifest,
					"      type: stripe",
					"      type: stripe\n      api_key_env: BILLING_TEST_STRIPE_KEY",
					1,
				)
			} else {
				manifest = strings.Replace(manifest, "      type: external_invoicing", `      type: external_invoicing
      enable_draft_sync_hook: false
      enable_issuing_sync_hook: true`, 1)
			}
			path := filepath.Join(t.TempDir(), "catalog.yaml")
			require.NoError(t, os.WriteFile(path, []byte(manifest), 0o600))
			rs, err := loader.New().LoadFile(path)
			require.NoError(t, err)
			plan, err := p.GeneratePlan(ctx, rs, planner.Options{Mode: planner.PlanModeApply})
			require.NoError(t, err)
			data, err := json.Marshal(plan)
			require.NoError(t, err)
			require.NotContains(t, string(data), "stripe-test-secret-value")
			require.NotContains(t, string(data), "create_billing_profile")
			var saved planner.Plan
			require.NoError(t, json.Unmarshal(data, &saved))
			result := executor.New(client, nil, false).Execute(ctx, &saved)
			require.Zero(t, result.FailureCount, "%+v", result.Errors)
			apps, writes := server.Snapshot(helpers.BillingApp)
			require.Len(t, apps, 1)
			require.Equal(t, typ, apps[0].String("type"))
			for _, write := range writes {
				if write.Kind == helpers.BillingApp && write.Method == "POST" {
					require.Equal(t, false, write.Fields["create_billing_profile"])
				}
			}
			if typ == "stripe" {
				var secret any
				for _, write := range writes {
					if write.Kind != helpers.BillingApp {
						continue
					}
					if write.Method == "POST" {
						secret = write.Fields["api_key"]
					} else {
						require.NotContains(t, write.Fields, "api_key")
						require.NotContains(t, write.Fields, "secret_api_key")
					}
				}
				require.Equal(t, "stripe-test-secret-value", secret)
				response, err := json.Marshal(apps[0])
				require.NoError(t, err)
				require.NotContains(t, string(response), "stripe-test-secret-value")
			} else {
				require.Equal(t, false, apps[0]["enable_draft_sync_hook"])
				require.Equal(t, true, apps[0]["enable_issuing_sync_hook"])
			}
			repeat, err := p.GeneratePlan(ctx, rs, planner.Options{Mode: planner.PlanModeApply})
			require.NoError(t, err)
			require.Empty(t, repeat.Changes)
		})
	}
}
