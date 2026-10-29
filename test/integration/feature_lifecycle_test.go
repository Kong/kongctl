//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kong/kongctl/internal/declarative/executor"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/log"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestFeatureDeclarativeLifecycle(t *testing.T) {
	for _, namespaces := range []struct{ meter, feature string }{
		{"billing", "billing"}, {"a-meter", "z-feature"}, {"z-meter", "a-feature"},
	} {
		for _, reference := range []string{"local-usage", "!ref local-usage#id"} {
			t.Run(namespaces.meter+"/"+namespaces.feature+"/"+reference, func(t *testing.T) {
				testFeatureDeclarativeLifecycle(t, reference, namespaces.meter, namespaces.feature)
			})
		}
	}
}

func testFeatureDeclarativeLifecycle(t *testing.T, meterReference, meterNamespace, featureNamespace string) {
	server := fixture.NewFeatureServer(t)
	meters := fixture.NewMeterServer(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.WithValue(t.Context(), log.LoggerKey, logger)
	client := state.NewClient(
		state.ClientConfig{
			FeaturesAPI:       server.SDK.GetFeaturesAPI(),
			BillingCatalogAPI: fixture.NewBillingCatalogServer(t).SDK.GetBillingCatalogAPI(),
			MetersAPI:         meters.SDK.GetMetersAPI(),
		},
	)
	p := planner.NewPlanner(client, logger)
	path := filepath.Join(t.TempDir(), "features.yaml")
	write := func(cost string) {
		t.Helper()
		require.NoError(t, os.WriteFile(path, []byte(`_defaults:
  kongctl:
    namespace: billing
meters:
  - ref: local-usage
    kongctl:
      namespace: `+meterNamespace+`
    name: Usage
    key: usage
    aggregation: count
    event_type: request
features:
  - ref: local-feature
    kongctl:
      namespace: `+featureNamespace+`
    name: Requests
    key: requests
    meter:
      id: `+meterReference+`
      filters:
        region:
          eq: us
`+cost), 0o600))
	}
	plan := func(mode planner.PlanMode) *planner.Plan {
		t.Helper()
		rs, err := loader.New().LoadFile(path)
		require.NoError(t, err)
		result, err := p.GeneratePlan(ctx, rs, planner.Options{Mode: mode})
		require.NoError(t, err)
		return result
	}
	apply := func(plan *planner.Plan) {
		t.Helper()
		// All executions use serialized plans to test the durable contract.
		data, err := json.Marshal(plan)
		require.NoError(t, err)
		var saved planner.Plan
		require.NoError(t, json.Unmarshal(data, &saved))
		result := executor.New(client, nil, false).Execute(ctx, &saved)
		require.Zero(t, result.FailureCount, "%+v", result.Errors)
	}
	write("    unit_cost: {type: manual, amount: '0.02'}\n")
	first := plan(planner.PlanModeApply)
	require.Len(t, first.Changes, 2)
	featureIndex := slices.IndexFunc(first.Changes, func(change planner.PlannedChange) bool {
		return change.ResourceType == planner.ResourceTypeFeature
	})
	meterIndex := slices.IndexFunc(first.Changes, func(change planner.PlannedChange) bool {
		return change.ResourceType == planner.ResourceTypeMeter
	})
	require.Contains(t, first.Changes[featureIndex].DependsOn, first.Changes[meterIndex].ID)
	apply(first)
	values, writes := server.Snapshot()
	require.Equal(t, fixture.MeterID, values[0].Meter.ID)
	require.Equal(t, featureNamespace, values[0].Labels[labels.NamespaceKey])
	require.Len(t, writes, 1)
	require.Empty(t, plan(planner.PlanModeApply).Changes)
	write("")
	require.Empty(t, plan(planner.PlanModeApply).Changes, "omission leaves cost unchanged")
	write("    unit_cost: null\n")
	clearCost := plan(planner.PlanModeApply)
	require.Len(t, clearCost.Changes, 1)
	require.Contains(t, clearCost.Changes[0].Fields, planner.FieldUnitCost)
	apply(clearCost)
	values, writes = server.Snapshot()
	require.Nil(t, values[0].UnitCost)
	require.Contains(t, writes[1], "unit_cost")
	require.Nil(t, writes[1]["unit_cost"])
	require.Empty(t, plan(planner.PlanModeApply).Changes)
	write("    unit_cost: {type: llm, provider: openai, model: model, token_type: input}\n")
	update := plan(planner.PlanModeApply)
	server.SetLabels(fixture.FeatureID, map[string]string{labels.NamespaceKey: "another"})
	result := executor.New(client, nil, false).Execute(ctx, update)
	require.Equal(t, 1, result.FailureCount)
	require.Contains(t, result.Errors[0].Error, "no longer owned")
	server.SetLabels(
		fixture.FeatureID,
		map[string]string{labels.NamespaceKey: featureNamespace, labels.ProtectedKey: "true"},
	)
	result = executor.New(client, nil, false).Execute(ctx, update)
	require.Equal(t, 1, result.FailureCount)
	require.Contains(t, result.Errors[0].Error, "protected")
	server.SetLabels(fixture.FeatureID, map[string]string{labels.NamespaceKey: featureNamespace})
	apply(update)
	require.Empty(t, plan(planner.PlanModeApply).Changes)
	server.Fail(403)
	result = executor.New(client, nil, false).Execute(ctx, update)
	require.Equal(t, 1, result.FailureCount)
	require.Contains(t, result.Errors[0].Error, "get feature")
	server.Fail(0)
	apply(plan(planner.PlanModeDelete))
	values, _ = server.Snapshot()
	require.Empty(t, values)
}
