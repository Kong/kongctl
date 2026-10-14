package planner

import (
	"io"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func testMeter() resources.MeterResource {
	return resources.MeterResource{
		BaseResource: resources.BaseResource{
			Ref:     "local-reference",
			Kongctl: &resources.KongctlMeta{Namespace: new("billing")},
		},
		Name:        "Usage",
		Key:         "usage",
		Aggregation: kkComps.AggregationCount,
		EventType:   "request",
	}
}

func observedMeter() kkComps.Meter {
	return kkComps.Meter{
		ID:          fixture.MeterID,
		Name:        "Usage",
		Key:         "usage",
		Aggregation: kkComps.MeterAggregationCount,
		EventType:   "request",
		Labels:      map[string]string{labels.NamespaceKey: "billing"},
	}
}

func TestMeterPlanningIdentityOwnershipAndProtection(t *testing.T) {
	tests := []struct {
		name   string
		change func(*resources.MeterResource, *kkComps.Meter)
		mode   PlanMode
		action ActionType
		error  string
	}{
		{name: "idempotent", mode: PlanModeApply},
		{
			name:   "display rename matches key",
			mode:   PlanModeApply,
			change: func(d *resources.MeterResource, _ *kkComps.Meter) { d.Name = "Renamed" },
			action: ActionUpdate,
		},
		{
			name:   "explicit ULID",
			mode:   PlanModeApply,
			change: func(d *resources.MeterResource, _ *kkComps.Meter) { d.ID = fixture.MeterID },
		},
		{
			name:   "ref never selects ID",
			mode:   PlanModeApply,
			change: func(d *resources.MeterResource, _ *kkComps.Meter) { d.Ref = fixture.MeterID; d.Key = "another" },
			action: ActionCreate,
		},
		{
			name:   "unmanaged key collision",
			mode:   PlanModeApply,
			change: func(_ *resources.MeterResource, c *kkComps.Meter) { c.Labels = nil },
			error:  "unmanaged",
		},
		{
			name:   "other namespace",
			mode:   PlanModeSync,
			change: func(_ *resources.MeterResource, c *kkComps.Meter) { c.Labels[labels.NamespaceKey] = "another" },
			error:  "belongs to namespace",
		},
		{
			name: "immutable aggregation with unprotection",
			mode: PlanModeApply,
			change: func(d *resources.MeterResource, c *kkComps.Meter) {
				d.Aggregation = kkComps.AggregationSum
				c.Labels[labels.ProtectedKey] = "true"
			},
			error: "immutable field aggregation",
		},
		{
			name:   "immutable events",
			mode:   PlanModeApply,
			change: func(d *resources.MeterResource, _ *kkComps.Meter) { d.EventType = "different" },
			error:  "immutable field event_type",
		},
		{
			name:   "protected delete",
			mode:   PlanModeDelete,
			change: func(_ *resources.MeterResource, c *kkComps.Meter) { c.Labels[labels.ProtectedKey] = "true" },
			error:  "protected",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, c := testMeter(), observedMeter()
			if tt.change != nil {
				tt.change(&d, &c)
			}
			server := fixture.NewMeterServer(t, c)
			client := state.NewClient(
				state.ClientConfig{
					MetersAPI:   server.SDK.GetMetersAPI(),
					FeaturesAPI: fixture.NewFeatureServer(t).SDK.GetFeaturesAPI(),
				},
			)
			p := NewPlanner(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
			rs := &resources.ResourceSet{Meters: []resources.MeterResource{d}}
			plan, err := p.GeneratePlan(t.Context(), rs, Options{Mode: tt.mode})
			if tt.error != "" {
				require.ErrorContains(t, err, tt.error)
				return
			}
			require.NoError(t, err)
			if tt.action == "" {
				require.Empty(t, plan.Changes)
				id, err := p.resolver.resolveReference(t.Context(), ResourceTypeMeter, d.Ref)
				require.NoError(t, err)
				require.Equal(t, fixture.MeterID, id)
			} else {
				require.Len(t, plan.Changes, 1)
				require.Equal(t, tt.action, plan.Changes[0].Action)
				require.Equal(t, d.Ref, plan.Changes[0].ResourceRef)
			}
		})
	}
}

func TestMeterSyncOnlyPrunesOwnedScope(t *testing.T) {
	owned := observedMeter()
	other := observedMeter()
	other.ID = "01G65Z755AFWAKHE12NY0CQ9FJ"
	other.Key = "other"
	other.Labels = map[string]string{labels.NamespaceKey: "other"}
	unmanaged := other
	unmanaged.ID = "01G65Z755AFWAKHE12NY0CQ9FK"
	unmanaged.Labels = nil
	server := fixture.NewMeterServer(t, owned, other, unmanaged)
	rs := &resources.ResourceSet{DefaultNamespace: "billing", SyncScope: resources.NewSyncScope()}
	rs.SyncScope.AddRoot(resources.ResourceTypeMeter)
	p := NewPlanner(
		state.NewClient(
			state.ClientConfig{
				MetersAPI:   server.SDK.GetMetersAPI(),
				FeaturesAPI: fixture.NewFeatureServer(t).SDK.GetFeaturesAPI(),
			},
		),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	plan, err := p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, fixture.MeterID, plan.Changes[0].ResourceID)
	rs.SyncScope = resources.NewSyncScope()
	plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
}

func TestMeterPlannerPropagatesAPIReadErrors(t *testing.T) {
	server := fixture.NewMeterServer(t)
	server.Fail(403)
	p := NewPlanner(
		state.NewClient(
			state.ClientConfig{
				MetersAPI:   server.SDK.GetMetersAPI(),
				FeaturesAPI: fixture.NewFeatureServer(t).SDK.GetFeaturesAPI(),
			},
		),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	_, err := p.GeneratePlan(
		t.Context(),
		&resources.ResourceSet{Meters: []resources.MeterResource{testMeter()}},
		Options{Mode: PlanModeApply},
	)
	require.ErrorContains(t, err, "list meters")
}
