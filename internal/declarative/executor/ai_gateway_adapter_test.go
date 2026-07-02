package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayAdapterMapCreateFieldsUsesNameAndDisplayName(t *testing.T) {
	adapter := NewAIGatewayAdapter(nil)
	execCtx := NewExecutionContext(&planner.PlannedChange{
		ResourceRef: "customer-support-ai-gateway",
		Namespace:   "ai-gateway-example",
		Protection:  true,
	})
	description := "AI Gateway for customer support traffic"
	fields := map[string]any{
		planner.FieldName:        "support-gateway",
		planner.FieldDisplayName: "Customer Support AI Gateway",
		planner.FieldDescription: description,
		planner.FieldLabels: map[string]any{
			"team": "support",
		},
	}

	var req kkComps.CreateAIGatewayRequest
	require.NoError(t, adapter.MapCreateFields(context.Background(), execCtx, fields, &req))

	assert.Equal(t, "support-gateway", req.Name)
	assert.Equal(t, "Customer Support AI Gateway", req.DisplayName)
	require.NotNil(t, req.Description)
	assert.Equal(t, description, *req.Description)
	assert.Equal(t, "support", req.Labels["team"])
	assert.Equal(t, "ai-gateway-example", req.Labels[labels.NamespaceKey])
	assert.Equal(t, labels.TrueValue, req.Labels[labels.ProtectedKey])
}

func TestAIGatewayAdapterMapUpdateFieldsPreservesCurrentName(t *testing.T) {
	adapter := NewAIGatewayAdapter(nil)
	execCtx := NewExecutionContext(&planner.PlannedChange{
		ResourceRef: "customer-support-ai-gateway",
		Namespace:   "ai-gateway-example",
	})
	fields := map[string]any{
		planner.FieldName:        "support-gateway",
		planner.FieldDisplayName: "Customer Support AI Gateway Renamed",
	}

	var req kkComps.UpdateAIGatewayRequest
	require.NoError(t, adapter.MapUpdateFields(context.Background(), execCtx, fields, &req, map[string]string{
		labels.NamespaceKey: "ai-gateway-example",
	}))

	assert.Equal(t, "support-gateway", req.Name)
	assert.Equal(t, "Customer Support AI Gateway Renamed", req.DisplayName)
	assert.Equal(t, "ai-gateway-example", req.Labels[labels.NamespaceKey])
}
