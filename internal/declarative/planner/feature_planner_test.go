package planner

import (
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/kong/kongctl/internal/konnect/helpers"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func testFeature() resources.FeatureResource {
	return resources.FeatureResource{
		BaseResource: resources.BaseResource{
			Ref:     "local-reference",
			Kongctl: &resources.KongctlMeta{Namespace: new("billing")},
		},
		Name: "Usage",
		Key:  "usage",
	}
}

func observedFeature() kkComps.Feature {
	return kkComps.Feature{
		ID:     fixture.FeatureID,
		Name:   "Usage",
		Key:    "usage",
		Labels: map[string]string{labels.NamespaceKey: "billing"},
	}
}

func TestFeatureSyncOnlyPrunesOwnedScope(t *testing.T) {
	owned := observedFeature()
	other := observedFeature()
	other.ID = "01G65Z755AFWAKHE12NY0CQ9FJ"
	other.Key = "other"
	other.Labels = map[string]string{labels.NamespaceKey: "other"}
	unmanaged := other
	unmanaged.ID = "01G65Z755AFWAKHE12NY0CQ9FK"
	unmanaged.Labels = nil
	server := fixture.NewFeatureServer(t, owned, other, unmanaged)
	rs := &resources.ResourceSet{DefaultNamespace: "billing", SyncScope: resources.NewSyncScope()}
	rs.SyncScope.AddRoot(resources.ResourceTypeFeature)
	p := NewPlanner(
		state.NewClient(state.ClientConfig{FeaturesAPI: server.SDK.GetFeaturesAPI()}),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	plan, err := p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, fixture.FeatureID, plan.Changes[0].ResourceID)
	rs.SyncScope = resources.NewSyncScope()
	plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
}

func TestFeaturePlannerPropagatesAPIReadErrors(t *testing.T) {
	server := fixture.NewFeatureServer(t)
	server.Fail(403)
	p := NewPlanner(
		state.NewClient(state.ClientConfig{FeaturesAPI: server.SDK.GetFeaturesAPI()}),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	_, err := p.GeneratePlan(
		t.Context(),
		&resources.ResourceSet{Features: []resources.FeatureResource{testFeature()}},
		Options{Mode: PlanModeApply},
	)
	require.ErrorContains(t, err, "list features")
}

func TestFeaturePlanningUnsupportedChangesAndOwnership(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*resources.FeatureResource, *kkComps.Feature)
		want   string
	}{
		{"idempotent", func(*resources.FeatureResource, *kkComps.Feature) {}, ""},
		{"name", func(d *resources.FeatureResource, _ *kkComps.Feature) { d.Name = "changed" }, "cannot update name"},
		{
			"description",
			func(d *resources.FeatureResource, _ *kkComps.Feature) { d.Description = new("changed") },
			"cannot update description",
		},
		{
			"labels",
			func(d *resources.FeatureResource, _ *kkComps.Feature) { d.Labels = map[string]string{"team": "new"} },
			"cannot update labels",
		},
		{
			"protection",
			func(d *resources.FeatureResource, _ *kkComps.Feature) { d.Kongctl.Protected = new(true) },
			"cannot update protected",
		},
		{
			"namespace",
			func(_ *resources.FeatureResource, c *kkComps.Feature) { c.Labels[labels.NamespaceKey] = "other" },
			"belongs to namespace",
		},
		{
			"unmanaged",
			func(_ *resources.FeatureResource, c *kkComps.Feature) { c.Labels = nil },
			"adoption is unsupported",
		},
		{
			"missing id",
			func(d *resources.FeatureResource, _ *kkComps.Feature) { d.ID = "01G65Z755AFWAKHE12NY0CQ9FK" },
			"not found",
		},
		{
			"id/key mismatch",
			func(d *resources.FeatureResource, _ *kkComps.Feature) { d.ID = fixture.FeatureID; d.Key = "different" },
			"different identities",
		},
		{"meter", func(d *resources.FeatureResource, _ *kkComps.Feature) {
			d.Meter = &kkComps.CreateFeatureRequestMeterReference{ID: fixture.MeterID}
		}, "cannot update meter"},
	} {
		t.Run(test.name, func(t *testing.T) {
			d, c := testFeature(), observedFeature()
			test.change(&d, &c)
			server := fixture.NewFeatureServer(t, c)
			p := NewPlanner(
				state.NewClient(state.ClientConfig{FeaturesAPI: server.SDK.GetFeaturesAPI()}),
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			)
			plan, err := p.GeneratePlan(
				t.Context(),
				&resources.ResourceSet{Features: []resources.FeatureResource{d}},
				Options{Mode: PlanModeApply},
			)
			if test.want != "" {
				require.ErrorContains(t, err, test.want)
			} else {
				require.NoError(t, err)
				require.Empty(t, plan.Changes)
			}
		})
	}
}

func TestMeterDeletionRespectsObservedFeatureReferences(t *testing.T) {
	for _, removeFeature := range []bool{false, true} {
		t.Run(fmt.Sprint(removeFeature), func(t *testing.T) {
			feature := observedFeature()
			feature.Meter = &kkComps.MeterReference{ID: fixture.MeterID}
			server := fixture.NewFeatureServer(t, feature)
			meters := fixture.NewMeterServer(t, observedMeter())
			p := NewPlanner(
				state.NewClient(
					state.ClientConfig{FeaturesAPI: server.SDK.GetFeaturesAPI(), MetersAPI: meters.SDK.GetMetersAPI()},
				),
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			)
			rs := &resources.ResourceSet{DefaultNamespace: "billing", SyncScope: resources.NewSyncScope()}
			rs.SyncScope.AddRoot(resources.ResourceTypeMeter)
			if removeFeature {
				rs.SyncScope.AddRoot(resources.ResourceTypeFeature)
			}
			plan, err := p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
			if !removeFeature {
				require.ErrorContains(t, err, "still references it")
				return
			}
			require.NoError(t, err)
			require.Len(t, plan.Changes, 2)
			require.Contains(t, plan.Changes[0].DependsOn, plan.Changes[1].ID)
			require.Equal(t, plan.Changes[1].ID, plan.ExecutionOrder[0])
		})
	}
}

func TestFeatureCostAndFilterNormalization(t *testing.T) {
	require.True(
		t,
		featureCostsEqual(
			&helpers.FeatureUnitCost{Type: "manual", Amount: new("0.010")},
			&helpers.FeatureUnitCost{Type: "manual", Amount: new("0.01")},
		),
	)
	require.True(t, featureFiltersEqual(nil, map[string]kkComps.QueryFilterStringMapItem{}))
}
