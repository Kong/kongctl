package dump

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	fixtures "github.com/kong/kongctl/test/declarative"
	"github.com/stretchr/testify/require"
)

func TestEventGatewayV13PolicyDumpRoundTrip(t *testing.T) {
	for _, fixture := range fixtures.EventGatewayV13Policies(t) {
		t.Run(fixture.Body["name"].(string), func(t *testing.T) {
			body, err := json.Marshal(fixture.Body)
			require.NoError(t, err)
			var policy kkComps.EventGatewayPolicy
			require.NoError(t, json.Unmarshal(body, &policy))
			policy.ID = "policy-id"
			config := fixture.Body["config"].(map[string]any)
			var resource resources.Resource
			switch fixture.Phase {
			case "produce":
				result, err := convertProducePolicyToResource(policy, config)
				require.NoError(t, err)
				resource = &result
			case "consume":
				result, err := convertConsumePolicyToResource(policy, config)
				require.NoError(t, err)
				resource = &result
			case "cluster":
				result, err := convertClusterPolicyToResource(policy, config)
				require.NoError(t, err)
				resource = &result
			}
			require.NoError(t, resource.Validate())
			encoded, err := json.Marshal(resource)
			require.NoError(t, err)
			fixture.Body["ref"] = "policy-id"
			expected, err := json.Marshal(fixture.Body)
			require.NoError(t, err)
			require.JSONEq(t, string(expected), string(encoded))
			// Reload the dumped declaration using kongctl's validation, not just SDK decoding.
			require.NoError(t, json.Unmarshal(encoded, resource))
			require.NoError(t, resource.Validate())
		})
	}
}
