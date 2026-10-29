package helpers_test

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestMeterPaginationAndULIDResolution(t *testing.T) {
	server := fixture.NewMeterServer(
		t,
		kkComps.Meter{ID: fixture.MeterID, Key: "first"},
		kkComps.Meter{ID: "01G65Z755AFWAKHE12NY0CQ9FJ", Key: "second"},
	)
	api := server.SDK.GetMetersAPI()
	values, err := helpers.ListAllMeters(t.Context(), api, 1)
	require.NoError(t, err)
	require.Len(t, values, 2)
	for _, id := range []string{"first", fixture.MeterID} {
		m, err := helpers.ResolveMeter(t.Context(), api, id, 1)
		require.NoError(t, err)
		require.Equal(t, fixture.MeterID, m.ID)
	}
	_, err = helpers.ResolveMeter(t.Context(), api, "display-name", 1)
	require.ErrorContains(t, err, "API key or ULID")
}

func TestMeterExplicitEmptyUpdateMapsReachAPI(t *testing.T) {
	server := fixture.NewMeterServer(
		t,
		kkComps.Meter{
			ID:         fixture.MeterID,
			Key:        "usage",
			Dimensions: map[string]string{"region": "$.region"},
			Labels:     map[string]string{"team": "billing"},
		},
	)
	_, err := server.SDK.GetMetersAPI().
		UpdateMeter(t.Context(), fixture.MeterID, kkComps.UpdateMeterRequest{
			Dimensions: map[string]string{}, Labels: map[string]string{},
		})
	require.NoError(t, err)
	values, writes := server.Snapshot()
	require.Empty(t, values[0].Dimensions)
	require.Empty(t, values[0].Labels)
	require.Contains(t, writes[0], "dimensions")
	require.Contains(t, writes[0], "labels")
	require.NotContains(t, writes[0], "aggregation")
	require.NotContains(t, writes[0], "event_type")
}

func TestMeterManagedLabelCompatibility(t *testing.T) {
	require.NoError(
		t,
		helpers.ValidateMeterLabels(map[string]string{"KONGCTL-namespace": "billing", "KONGCTL-protected": "true"}),
	)
	for _, labels := range []map[string]string{
		{"kongctl-namespace": "billing"}, {"team": ""}, {"team": "bad/value"}, {"team": "-bad"},
	} {
		require.Error(t, helpers.ValidateMeterLabels(labels))
	}
	require.True(t, helpers.IsMeterULID(fixture.MeterID))
	require.False(t, helpers.IsMeterULID("81G65Z755AFWAKHE12NY0CQ9FH"))
	require.False(t, helpers.IsMeterULID("01g65z755afwakhe12ny0cq9fh"))
}
