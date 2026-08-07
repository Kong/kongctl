package executor

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayConsumerGroupAdapterPreservesAdditionalProperties(t *testing.T) {
	t.Parallel()

	fields := map[string]any{
		planner.FieldName:        "premium-users",
		planner.FieldDisplayName: "Premium Users",
		"future_group_field":     "group-value",
	}
	var request kkComps.CreateAIGatewayConsumerGroupRequest
	require.NoError(t, NewAIGatewayConsumerGroupAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request))
	require.Equal(t, "group-value", request.AdditionalProperties["future_group_field"])
}

func TestAIGatewayMCPServerAdapterPreservesAdditionalProperties(t *testing.T) {
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
	require.NoError(t, NewAIGatewayMCPServerAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request))
	require.NotNil(t, request.AIGatewayMCPServerConversionOnly)
	require.Equal(
		t,
		"mcp-server-value",
		request.AIGatewayMCPServerConversionOnly.AdditionalProperties["future_mcp_server_field"],
	)
}
