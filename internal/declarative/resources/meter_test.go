package resources

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/stretchr/testify/require"
)

func TestMeterRequiresExplicitAPIFields(t *testing.T) {
	m := MeterResource{BaseResource: BaseResource{Ref: "usage"}}
	m.SetDefaults()
	require.Empty(t, m.Name)
	require.Empty(t, m.Key)
	require.ErrorContains(t, m.Validate(), "name is required")
	m.Name = "Usage"
	require.ErrorContains(t, m.Validate(), "key is required")
	m.Key = "api_key"
	m.Aggregation = kkComps.AggregationCount
	require.ErrorContains(t, m.Validate(), "event_type is required")
	m.EventType = "request"
	require.NoError(t, m.Validate())
	m.ID = "not-a-ulid"
	require.ErrorContains(t, m.Validate(), "ULID")
}

func TestMeterLabelsRejectManagementMetadata(t *testing.T) {
	m := MeterResource{
		BaseResource: BaseResource{
			Ref:     "usage",
			Kongctl: &KongctlMeta{Namespace: new("billing"), Protected: new(true)},
		},
		Name: "Usage", Key: "usage", Aggregation: kkComps.AggregationCount, EventType: "request",
		Labels: map[string]string{"team": "billing"},
	}
	require.NoError(t, m.Validate(), "management metadata is supported through the kongctl block")
	for _, key := range []string{labels.NamespaceKey, labels.ProtectedKey, labels.ManagedKey, "KONGCTL-custom"} {
		t.Run(key, func(t *testing.T) {
			m.Labels = map[string]string{key: "true"}
			err := m.Validate()
			require.ErrorContains(t, err, key)
			require.ErrorContains(t, err, "belongs in the kongctl block")
		})
	}
}

func TestMeterLocalRefAndIdentityRemainDistinct(t *testing.T) {
	m := MeterResource{BaseResource: BaseResource{Ref: "local"}, Key: "api_key"}
	require.False(t, m.TryMatchKonnectResource(struct{ ID, Key string }{"01G65Z755AFWAKHE12NY0CQ9FH", "local"}))
	require.True(t, m.TryMatchKonnectResource(struct{ ID, Key string }{"01G65Z755AFWAKHE12NY0CQ9FH", "api_key"}))
	data, err := json.Marshal(m)
	require.NoError(t, err)
	require.Contains(t, string(data), `"ref":"local"`)
	require.Contains(t, string(data), `"key":"api_key"`)
}
