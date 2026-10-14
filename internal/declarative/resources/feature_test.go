package resources

import (
	"encoding/json"
	"testing"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/stretchr/testify/require"
)

func TestFeatureRequiresExplicitAPIFields(t *testing.T) {
	m := FeatureResource{BaseResource: BaseResource{Ref: "usage"}}
	m.SetDefaults()
	require.Empty(t, m.Name)
	require.Empty(t, m.Key)
	require.ErrorContains(t, m.Validate(), "name is required")
	m.Name = "Usage"
	require.ErrorContains(t, m.Validate(), "key is required")
	m.Key = "api_key"
	require.NoError(t, m.Validate())
	m.ID = "not-a-ulid"
	require.ErrorContains(t, m.Validate(), "ULID")
}

func TestFeatureLabelsRejectManagementMetadata(t *testing.T) {
	m := FeatureResource{
		BaseResource: BaseResource{
			Ref:     "usage",
			Kongctl: &KongctlMeta{Namespace: new("billing"), Protected: new(true)},
		},
		Name: "Usage", Key: "usage",
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

func TestFeatureLocalRefAndIdentityRemainDistinct(t *testing.T) {
	m := FeatureResource{BaseResource: BaseResource{Ref: "local"}, Key: "api_key"}
	require.False(t, m.TryMatchKonnectResource(struct{ ID, Key string }{"01G65Z755AFWAKHE12NY0CQ9FH", "local"}))
	require.True(t, m.TryMatchKonnectResource(struct{ ID, Key string }{"01G65Z755AFWAKHE12NY0CQ9FH", "api_key"}))
	data, err := json.Marshal(m)
	require.NoError(t, err)
	require.Contains(t, string(data), `"ref":"local"`)
	require.Contains(t, string(data), `"key":"api_key"`)
}

func TestFeatureCostPresenceRoundTrip(t *testing.T) {
	for _, data := range []string{
		`{"ref":"r","name":"R","key":"r"}`,
		`{"ref":"r","name":"R","key":"r","unit_cost":null}`,
		`{"ref":"r","name":"R","key":"r","unit_cost":{"type":"manual","amount":"0.01"}}`,
	} {
		var value FeatureResource
		require.NoError(t, json.Unmarshal([]byte(data), &value))
		require.NoError(t, value.Validate())
		encoded, err := json.Marshal(value)
		require.NoError(t, err)
		require.JSONEq(t, data, string(encoded))
	}
}
