package dump

import (
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestDumpMeterIdentityAndFields(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	server := fixture.NewMeterServer(
		t,
		kkComps.Meter{
			ID:            fixture.MeterID,
			Name:          "Request Usage",
			Key:           "usage",
			Aggregation:   kkComps.MeterAggregationSum,
			EventType:     "request",
			EventsFrom:    &start,
			ValueProperty: new("$.tokens"),
			Dimensions:    map[string]string{"region": "$.region"},
			Labels: map[string]string{
				labels.NamespaceKey: "billing",
				labels.ProtectedKey: "true",
				"team":              "billing",
			},
		},
	)
	values, err := collectDeclarativeMeters(t.Context(), server.SDK.GetMetersAPI(), 1, filterOptions{name: "*Usage*"})
	require.NoError(t, err)
	require.Len(t, values, 1)
	m := values[0]
	require.NoError(t, m.Validate())
	require.Equal(t, "usage", m.Ref)
	require.Equal(t, fixture.MeterID, m.ID)
	require.Equal(t, "billing", *m.Kongctl.Namespace)
	require.True(t, *m.Kongctl.Protected)
	require.Equal(t, "2026-01-01T00:00:00Z", *m.EventsFrom)
	require.NotContains(t, m.Labels, labels.NamespaceKey)
	values, err = collectDeclarativeMeters(
		t.Context(),
		server.SDK.GetMetersAPI(),
		1,
		filterOptions{id: fixture.MeterID},
	)
	require.NoError(t, err)
	require.Len(t, values, 1)
}
