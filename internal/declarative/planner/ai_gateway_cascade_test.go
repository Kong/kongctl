package planner

import (
	"encoding/json"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestGatewayCascadeWarningsPersistForeignUnownedProtectedDescendants(t *testing.T) {
	gateway := PlannedChange{
		ID:           "gateway-delete",
		ResourceRef:  "shared",
		ResourceType: ResourceTypeAIGateway,
		Action:       ActionDelete,
		Namespace:    "platform",
	}
	plan := &Plan{Changes: []PlannedChange{gateway}}
	providers,
		err := cascadePublicMetadata([]state.AIGatewayProvider{{
		Name: "foreign-provider",
		ManagedBy: map[string]string{
			resources.ChildNamespaceKey: "team-b",
			resources.ChildProtectedKey: "true",
		},
	}},
		nil)
	require.NoError(t, err)
	addAIGatewayCascadeWarnings(plan, gateway, ResourceTypeAIGatewayProvider, providers)
	addAIGatewayCascadeWarnings(plan, gateway, ResourceTypeAIGatewayModel, []aiGatewayDescendant{{name: "legacy"}})
	data, err := json.Marshal(plan)
	require.NoError(t, err)
	var saved Plan
	require.NoError(t, json.Unmarshal(data, &saved))
	require.Len(t, saved.Warnings, 2)
	require.Contains(t, saved.Warnings[0].Message, "team-b")
	require.Contains(t, saved.Warnings[0].Message, "foreign-provider")
	require.Contains(t, saved.Warnings[0].Message, "protected descendants")
	require.Contains(t, saved.Warnings[1].Message, "unowned")
	require.Len(t, saved.Changes, 1, "cascade does not plan unauthorized direct child deletes")
}
