package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayConsumerAdapterMapCreateFields(t *testing.T) {
	t.Parallel()

	adapter := NewAIGatewayConsumerAdapter(nil)
	fields := map[string]any{
		planner.FieldName:        "support-user",
		planner.FieldType:        "api-key",
		planner.FieldDisplayName: "Support User",
		planner.FieldPolicies:    []string{"mask-sensitive-data"},
		planner.FieldLabels:      map[string]string{"team": "support"},
	}

	var req kkComps.CreateAIGatewayConsumerRequest
	require.NoError(t, adapter.MapCreateFields(context.Background(), nil, fields, &req))
	require.Equal(t, "support-user", req.Name)
	require.Equal(t, kkComps.CreateAIGatewayConsumerRequestTypeAPIKey, req.Type)
	require.Equal(t, "Support User", req.DisplayName)
	require.Equal(t, []string{"mask-sensitive-data"}, req.Policies)
	require.Equal(t, map[string]string{"team": "support"}, req.Labels)
}

func TestAIGatewayConsumerAdapterMapCreateFieldsRequiresType(t *testing.T) {
	t.Parallel()

	adapter := NewAIGatewayConsumerAdapter(nil)
	fields := map[string]any{
		planner.FieldName:        "support-user",
		planner.FieldDisplayName: "Support User",
	}

	var req kkComps.CreateAIGatewayConsumerRequest
	err := adapter.MapCreateFields(context.Background(), nil, fields, &req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "type")
}
