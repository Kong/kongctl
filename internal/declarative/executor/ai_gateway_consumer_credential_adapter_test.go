package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayConsumerCredentialAdapterMapCreateFields(t *testing.T) {
	t.Parallel()

	adapter := NewAIGatewayConsumerCredentialAdapter(nil)
	fields := map[string]any{
		planner.FieldName:        "support-user-key",
		planner.FieldType:        "api-key",
		planner.FieldDisplayName: "Support User API Key",
		planner.FieldLabels:      map[string]string{"team": "support"},
		"ttl":                    float64(60),
		"api_key":                "secret-value",
	}

	var req kkComps.CreateAIGatewayConsumerCredentialRequest
	require.NoError(t, adapter.MapCreateFields(context.Background(), nil, fields, &req))
	require.Equal(t, "support-user-key", req.Name)
	require.Equal(t, kkComps.CreateAIGatewayConsumerCredentialRequestTypeAPIKey, req.Type)
	require.Equal(t, "Support User API Key", req.DisplayName)
	require.Equal(t, map[string]string{"team": "support"}, req.Labels)
	require.NotNil(t, req.TTL)
	require.Equal(t, int64(60), *req.TTL)
	require.Nil(t, req.APIKey)
}

func TestAIGatewayConsumerCredentialAdapterMapCreateFieldsRequiresType(t *testing.T) {
	t.Parallel()

	adapter := NewAIGatewayConsumerCredentialAdapter(nil)
	fields := map[string]any{
		planner.FieldName:        "support-user-key",
		planner.FieldDisplayName: "Support User API Key",
	}

	var req kkComps.CreateAIGatewayConsumerCredentialRequest
	err := adapter.MapCreateFields(context.Background(), nil, fields, &req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "type")
}
