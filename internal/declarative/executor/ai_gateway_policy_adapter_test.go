package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayPolicyAdapterMapCreateFields(t *testing.T) {
	t.Parallel()

	adapter := NewAIGatewayPolicyAdapter(nil)
	fields := map[string]any{
		planner.FieldName:        "mask-sensitive-data",
		planner.FieldType:        "ai-sanitizer",
		planner.FieldDisplayName: "Mask Sensitive Data",
		planner.FieldEnabled:     true,
		"global":                 false,
		planner.FieldConfig: map[string]any{
			"anonymize": []any{"email"},
		},
		planner.FieldLabels: map[string]string{"team": "platform"},
	}

	var req kkComps.CreateAIGatewayPolicyRequest
	require.NoError(t, adapter.MapCreateFields(context.Background(), nil, fields, &req))
	require.Equal(t, "mask-sensitive-data", req.Name)
	require.Equal(t, "ai-sanitizer", req.Type)
	require.Equal(t, "Mask Sensitive Data", req.DisplayName)
	require.NotNil(t, req.Enabled)
	require.True(t, *req.Enabled)
	require.NotNil(t, req.Global)
	require.False(t, *req.Global)
	require.Equal(t, map[string]string{"team": "platform"}, req.Labels)
	require.Equal(t, []any{"email"}, req.Config["anonymize"])
}
