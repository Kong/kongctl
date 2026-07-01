package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayAgentAdapterMapCreateFields(t *testing.T) {
	t.Parallel()

	adapter := NewAIGatewayAgentAdapter(nil)
	fields := map[string]any{
		planner.FieldName:        "booking-agent",
		planner.FieldType:        "a2a",
		planner.FieldDisplayName: "Booking Agent",
		planner.FieldConfig: map[string]any{
			"url": "https://booking-agent.example.com",
		},
		planner.FieldPolicies: []string{"mask-sensitive-data"},
		planner.FieldLabels:   map[string]string{"team": "support"},
	}

	var req kkComps.CreateAIGatewayAgentRequest
	require.NoError(t, adapter.MapCreateFields(context.Background(), nil, fields, &req))
	require.Equal(t, "booking-agent", req.Name)
	require.Equal(t, kkComps.CreateAIGatewayAgentRequestTypeA2a, req.Type)
	require.Equal(t, "Booking Agent", req.DisplayName)
	require.Equal(t, "https://booking-agent.example.com", req.Config.URL)
	require.Equal(t, []string{"mask-sensitive-data"}, req.Policies)
	require.Equal(t, map[string]string{"team": "support"}, req.Labels)
}

func TestAIGatewayAgentAdapterMapCreateFieldsRequiresConfigURL(t *testing.T) {
	t.Parallel()

	adapter := NewAIGatewayAgentAdapter(nil)
	fields := map[string]any{
		planner.FieldName:        "booking-agent",
		planner.FieldType:        "a2a",
		planner.FieldDisplayName: "Booking Agent",
		planner.FieldConfig:      map[string]any{},
	}

	var req kkComps.CreateAIGatewayAgentRequest
	err := adapter.MapCreateFields(context.Background(), nil, fields, &req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "config.url")
}
