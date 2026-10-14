//go:build integration

package integration

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
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

func TestMeterDeclarativeLifecycle(t *testing.T) {
	server := fixture.NewMeterServer(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.WithValue(t.Context(), log.LoggerKey, logger)
	client := state.NewClient(
		state.ClientConfig{
			MetersAPI:   server.SDK.GetMetersAPI(),
			FeaturesAPI: fixture.NewFeatureServer(t).SDK.GetFeaturesAPI(),
		},
	)
	p := planner.NewPlanner(client, logger)
	path := filepath.Join(t.TempDir(), "meters.yaml")
	write := func(name, dimensions string) {
		t.Helper()
		require.NoError(t, os.WriteFile(path, []byte(`_defaults:
  kongctl:
    namespace: billing
meters:
  - ref: local-usage
    name: `+name+`
    key: usage
    aggregation: sum
    event_type: request
    value_property: $.tokens
    events_from: '2026-01-01T00:00:00Z'
    dimensions: `+dimensions+`
`), 0o600))
	}
	plan := func() *planner.Plan {
		t.Helper()
		rs, err := loader.New().LoadFile(path)
		require.NoError(t, err)
		plan, err := p.GeneratePlan(ctx, rs, planner.Options{Mode: planner.PlanModeApply})
		require.NoError(t, err)
		return plan
	}
	apply := func(plan *planner.Plan) {
		t.Helper()
		result := executor.New(client, nil, false).Execute(ctx, plan)
		require.Zero(t, result.FailureCount, "%+v", result.Errors)
	}
	write("Usage", "{region: '$.region'}")
	first := plan()
	require.Len(t, first.Changes, 1)
	apply(first)
	values, writes := server.Snapshot()
	require.Equal(t, "billing", values[0].Labels[labels.NamespaceKey])
	require.Len(t, writes, 1)
	require.Empty(t, plan().Changes)
	write("Renamed", "{}")
	update := plan()
	require.Len(t, update.Changes, 1)
	require.Equal(t, planner.ActionUpdate, update.Changes[0].Action)
	apply(update)
	values, writes = server.Snapshot()
	require.Equal(t, "Renamed", values[0].Name)
	require.Empty(t, values[0].Dimensions)
	require.NotContains(t, writes[1], "aggregation")
	require.NotContains(t, writes[1], "event_type")
	require.Empty(t, plan().Changes)
	// Ownership can change after planning; saved plans must fail before mutation.
	write("Changed", "{}")
	stale := plan()
	server.SetLabels(fixture.MeterID, map[string]string{labels.NamespaceKey: "another"})
	result := executor.New(client, nil, false).Execute(ctx, stale)
	require.Equal(t, 1, result.FailureCount)
	require.Contains(t, result.Errors[0].Error, "no longer owned")
	server.SetLabels(fixture.MeterID, map[string]string{labels.NamespaceKey: "billing", "external": "retained"})
	apply(stale)
	values, _ = server.Snapshot()
	require.Equal(t, "retained", values[0].Labels["external"])
	server.Fail(403)
	result = executor.New(client, nil, false).Execute(ctx, stale)
	require.Equal(t, 1, result.FailureCount)
	require.Contains(t, result.Errors[0].Error, "get meter")
	server.Fail(0)
	rs, err := loader.New().LoadFile(path)
	require.NoError(t, err)
	deletion, err := p.GeneratePlan(ctx, rs, planner.Options{Mode: planner.PlanModeDelete})
	require.NoError(t, err)
	apply(deletion)
	values, _ = server.Snapshot()
	require.Empty(t, values)
}
