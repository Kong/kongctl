package executor

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayConsumerGroupAdapterRejectsAdditionalProperties(t *testing.T) {
	t.Parallel()

	fields := map[string]any{
		planner.FieldName:        "premium-users",
		planner.FieldDisplayName: "Premium Users",
		"future_group_field":     "group-value",
	}
	var request kkComps.CreateAIGatewayConsumerGroupRequest
	err := NewAIGatewayConsumerGroupAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request)
	require.Error(t, err)
	require.Contains(t, err.Error(), "future_group_field")
}

func TestAIGatewayMCPServerAdapterRejectsAdditionalProperties(t *testing.T) {
	t.Parallel()

	fields := map[string]any{
		planner.FieldType:        "conversion-only",
		planner.FieldName:        "support-tools",
		planner.FieldDisplayName: "Support Tools",
		planner.FieldConfig: map[string]any{
			"url": "https://support-tools.example.com",
		},
		"future_mcp_server_field": "mcp-server-value",
	}
	var request kkComps.CreateAIGatewayMCPServerRequest
	err := NewAIGatewayMCPServerAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request)
	require.Error(t, err)
	require.Contains(t, err.Error(), "future_mcp_server_field")
}
