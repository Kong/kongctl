package executor

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	fixtures "github.com/kong/kongctl/test/declarative"
	"github.com/stretchr/testify/require"
)

func TestEventGatewayV13SavedPlanPayloads(t *testing.T) {
	for _, fixture := range fixtures.EventGatewayV13Policies(t) {
		t.Run(fixture.Body[planner.FieldName].(string), func(t *testing.T) {
			delete(fixture.Body, "ref")
			fields := fixture.Body
			var create, update any
			switch fixture.Phase {
			case "produce":
				adapter := &EventGatewayProducePolicyAdapter{}
				createRequest := &kkComps.EventGatewayProducePolicyCreate{}
				updateRequest := &kkComps.EventGatewayProducePolicyUpdate{}
				require.NoError(t, adapter.MapCreateFields(t.Context(), nil, fields, createRequest))
				require.NoError(t, adapter.MapUpdateFields(t.Context(), nil, fields, updateRequest, nil))
				create, update = createRequest, updateRequest
			case "consume":
				adapter := &EventGatewayConsumePolicyAdapter{}
				createRequest := &kkComps.EventGatewayConsumePolicyCreate{}
				updateRequest := &kkComps.EventGatewayConsumePolicyUpdate{}
				require.NoError(t, adapter.MapCreateFields(t.Context(), nil, fields, createRequest))
				require.NoError(t, adapter.MapUpdateFields(t.Context(), nil, fields, updateRequest, nil))
				create, update = createRequest, updateRequest
			case "cluster":
				adapter := &EventGatewayClusterPolicyAdapter{}
				createRequest := &kkComps.EventGatewayClusterPolicyModify{}
				updateRequest := &kkComps.EventGatewayClusterPolicyModify{}
				require.NoError(t, adapter.MapCreateFields(t.Context(), nil, fields, createRequest))
				require.NoError(t, adapter.MapUpdateFields(t.Context(), nil, fields, updateRequest, nil))
				create, update = createRequest, updateRequest
			}
			encoded, err := json.Marshal(create)
			require.NoError(t, err)
			expected, err := json.Marshal(fields)
			require.NoError(t, err)
			require.JSONEq(t, string(expected), string(encoded))
			// Parent assignment is create-only; PUT must retain every other field.
			delete(fields, planner.FieldParentPolicyID)
			resourceType := map[string]string{
				"produce": planner.ResourceTypeEventGatewayProducePolicy,
				"consume": planner.ResourceTypeEventGatewayConsumePolicy,
				"cluster": planner.ResourceTypeEventGatewayClusterPolicy,
			}[fixture.Phase]
			require.NoError(t, validateMappedPayload(resourceType, planner.ActionUpdate, fields, update))
			encoded, err = json.Marshal(update)
			require.NoError(t, err)
			expected, err = json.Marshal(fields)
			require.NoError(t, err)
			require.JSONEq(t, string(expected), string(encoded))
		})
	}
}
