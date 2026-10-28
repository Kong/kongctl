package executor

import (
	"context"
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

type cascadeAuthorityAPI struct {
	helpers.AIGatewayAPI
	gateway kkComps.AIGateway
	deletes int
}

func (a *cascadeAuthorityAPI) GetAiGateway(
	_ context.Context, _ string, _ ...kkOps.Option,
) (*kkOps.GetAiGatewayResponse, error) {
	return &kkOps.GetAiGatewayResponse{AIGateway: &a.gateway}, nil
}

func (a *cascadeAuthorityAPI) DeleteAiGateway(
	_ context.Context, _ string, _ ...kkOps.Option,
) (*kkOps.DeleteAiGatewayResponse, error) {
	a.deletes++
	return &kkOps.DeleteAiGatewayResponse{}, nil
}

func TestSavedGatewayCascadeEnforcesCurrentOwnerAndGatewayProtection(t *testing.T) {
	for _, tt := range []struct{ name, owner, protection, want string }{
		{"owner", "platform", "false", ""},
		{"ownership changed", "other-team", "false", "only its owner"},
		{"unowned", "", "false", "unowned; explicitly adopt it into namespace \"platform\" before deleting it"},
		{"gateway protected", "platform", "true", "protected"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			change := planner.PlannedChange{
				ResourceType: planner.ResourceTypeAIGateway, Action: planner.ActionDelete,
				ResourceID: "gateway-id", Namespace: "platform",
			}
			data, err := json.Marshal(change)
			require.NoError(t, err)
			var saved planner.PlannedChange
			require.NoError(t, json.Unmarshal(data, &saved))
			api := &cascadeAuthorityAPI{gateway: kkComps.AIGateway{ID: "gateway-id", Labels: map[string]string{
				labels.NamespaceKey: tt.owner, labels.ProtectedKey: tt.protection,
			}}}
			adapter := NewAIGatewayAdapter(state.NewClient(state.ClientConfig{AIGatewayAPI: api}))
			err = adapter.Delete(t.Context(), "gateway-id", NewExecutionContext(&saved))
			if tt.want != "" {
				require.ErrorContains(t, err, tt.want)
				require.Zero(t, api.deletes)
			} else {
				require.NoError(t, err)
				require.Equal(t, 1, api.deletes)
			}
		})
	}
}
