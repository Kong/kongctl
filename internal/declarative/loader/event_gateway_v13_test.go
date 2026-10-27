package loader

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	fixtures "github.com/kong/kongctl/test/declarative"
	"github.com/stretchr/testify/require"
)

func TestEventGatewayV13ScenarioInputs(t *testing.T) {
	for _, input := range []string{"testdata", "overlays/update", "overlays/prune", "overlays/cleanup"} {
		t.Run(input, func(t *testing.T) {
			path := filepath.Join("../../../test/e2e/scenarios/event-gateway/policies-v1-3", input, "config.yaml")
			rs, err := New().LoadFile(path)
			require.NoError(t, err)
			require.NotNil(t, rs)
		})
	}
}

func TestEventGatewayV13PolicyLoading(t *testing.T) {
	for _, fixture := range fixtures.EventGatewayV13Policies(t) {
		t.Run(fixture.Body["name"].(string), func(t *testing.T) {
			var resource resources.Resource
			switch fixture.Phase {
			case "produce":
				resource = &resources.EventGatewayProducePolicyResource{}
			case "consume":
				resource = &resources.EventGatewayConsumePolicyResource{}
			case "cluster":
				resource = &resources.EventGatewayClusterPolicyResource{}
			}
			data, err := json.Marshal(fixture.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(data, resource))
			require.NoError(t, resource.Validate())
			require.Equal(t, fixture.Body["ref"], resource.GetRef())
			require.Equal(t, fixture.Body["name"], resource.GetMoniker())
			roundTrip, err := json.Marshal(resource)
			require.NoError(t, err)
			require.JSONEq(t, string(data), string(roundTrip))
		})
	}
}
