package dump

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestDumpFeatureIdentityAndFields(t *testing.T) {
	server := fixture.NewFeatureServer(
		t,
		kkComps.Feature{
			ID:   fixture.FeatureID,
			Name: "Request Usage",
			Key:  "usage",
			Meter: &kkComps.MeterReference{
				ID:      fixture.MeterID,
				Filters: map[string]kkComps.QueryFilterStringMapItem{"region": {Eq: new("us")}},
			},
			Labels: map[string]string{
				labels.NamespaceKey: "billing",
				labels.ProtectedKey: "true",
				"team":              "billing",
			},
		},
	)
	values, err := collectDeclarativeFeatures(
		t.Context(),
		server.SDK.GetFeaturesAPI(),
		1,
		filterOptions{name: "*Usage*"},
	)
	require.NoError(t, err)
	require.Len(t, values, 1)
	m := values[0]
	require.NoError(t, m.Validate())
	require.Equal(t, "usage", m.Ref)
	require.Equal(t, fixture.FeatureID, m.ID)
	require.Equal(t, "billing", *m.Kongctl.Namespace)
	require.True(t, *m.Kongctl.Protected)
	require.Equal(t, fixture.MeterID, m.Meter.ID)
	require.Equal(t, "us", *m.Meter.Filters["region"].Eq)
	require.NotContains(t, m.Labels, labels.NamespaceKey)
	values, err = collectDeclarativeFeatures(
		t.Context(),
		server.SDK.GetFeaturesAPI(),
		1,
		filterOptions{id: fixture.FeatureID},
	)
	require.NoError(t, err)
	require.Len(t, values, 1)
}

func TestDumpFeatureExcludesResolvedLLMPricing(t *testing.T) {
	cost := kkComps.CreateUnitCostLlm(
		kkComps.BillingFeatureLLMUnitCost{
			Provider:  new("openai"),
			Model:     new("model"),
			TokenType: kkComps.TokenTypeInput.ToPointer(),
			Pricing:   &kkComps.ResolvedPricing{InputPerToken: "0.001", OutputPerToken: "0.002"},
		},
	)
	value := mapFeatureToDeclarativeResource(
		kkComps.Feature{ID: fixture.FeatureID, Name: "LLM", Key: "llm", UnitCost: &cost},
	)
	require.NoError(t, value.Validate())
	data, err := json.Marshal(value)
	require.NoError(t, err)
	require.NotContains(t, string(data), "pricing")
	require.Contains(t, string(data), `"provider":"openai"`)
}
