package planner

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestIDMatchedChildReconcilerIdentityAndSyncPruning(t *testing.T) {
	const (
		boundID   = "11111111-1111-4111-8111-111111111111"
		refID     = "22222222-2222-4222-8222-222222222222"
		nameID    = "33333333-3333-4333-8333-333333333333"
		staleID   = "44444444-4444-4444-8444-444444444444"
		missingID = "55555555-5555-4555-8555-555555555555"
	)

	for _, tt := range []struct {
		name          string
		boundID       string
		ref           string
		missingDetail bool
		wantGetIDs    []string
		wantAction    ActionType
		wantID        string
		wantBoundID   string
		wantDeleteIDs []string
	}{
		{
			name:          "bound ID takes precedence over UUID ref and name",
			boundID:       boundID,
			ref:           refID,
			wantGetIDs:    []string{boundID},
			wantAction:    ActionUpdate,
			wantID:        boundID,
			wantBoundID:   boundID,
			wantDeleteIDs: []string{refID, staleID},
		},
		{
			name:          "UUID ref takes precedence over name",
			ref:           refID,
			wantGetIDs:    []string{refID},
			wantAction:    ActionUpdate,
			wantID:        refID,
			wantBoundID:   refID,
			wantDeleteIDs: []string{boundID, staleID},
		},
		{
			name:          "missing UUID ref does not fall back to name",
			ref:           missingID,
			wantAction:    ActionCreate,
			wantDeleteIDs: []string{boundID, refID, staleID},
		},
		{
			name:          "missing bound ID does not fall back to UUID ref or name",
			boundID:       missingID,
			ref:           refID,
			wantAction:    ActionCreate,
			wantBoundID:   missingID,
			wantDeleteIDs: []string{boundID, refID, staleID},
		},
		{
			name:          "missing detail recreates and retains desired ID and name",
			boundID:       boundID,
			ref:           refID,
			missingDetail: true,
			wantGetIDs:    []string{boundID},
			wantAction:    ActionCreate,
			wantBoundID:   boundID,
			wantDeleteIDs: []string{refID, staleID},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var current []kkComps.AIGatewayAgent
			for _, identity := range []struct{ id, name string }{
				{boundID, "bound-agent"},
				{refID, "uuid-agent"},
				{nameID, "booking-agent"},
				{staleID, "stale-agent"},
			} {
				agent := testAIGatewayAgent(nil)
				agent.ID, agent.Name = identity.id, identity.name
				current = append(current, agent)
			}
			api := &idMatchedChildAgentAPI{
				testAIGatewayAgentAPI: &testAIGatewayAgentAPI{agents: current},
				missingDetail:         tt.missingDetail,
			}
			desired := testAIGatewayAgentResource(t, nil)
			desired.Ref = tt.ref
			desired.SetKonnectID(tt.boundID)
			desired.DisplayName = "Updated Agent"
			rs := testAIGatewayAgentResourceSet(desired)
			p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayAgentsAPI: api}), slog.Default())
			p.resources = rs
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)

			err := p.planAIGatewayAgentChanges(t.Context(), "default", "support-gateway", "Support Gateway",
				"gateway-id", "", nil, rs.AIGatewayAgents, plan)

			require.NoError(t, err)
			require.Equal(t, tt.wantGetIDs, api.getIDs)
			require.Equal(t, tt.wantBoundID, rs.AIGatewayAgents[0].GetKonnectID())
			require.Len(t, plan.Changes, 1+len(tt.wantDeleteIDs))
			change := plan.Changes[0]
			require.Equal(t, tt.wantAction, change.Action)
			require.Equal(t, tt.wantID, change.ResourceID)
			require.Equal(t, tt.ref, change.ResourceRef)
			require.Equal(t, "Updated Agent", change.Fields[FieldDisplayName])
			if tt.wantAction == ActionUpdate {
				require.Contains(t, change.ChangedFields, FieldDisplayName)
			}
			// Both desired identity keys survive sync, even when they identify
			// different observed agents. Unmatched agents retain list order.
			for i, id := range tt.wantDeleteIDs {
				require.Equal(t, ActionDelete, plan.Changes[i+1].Action)
				require.Equal(t, id, plan.Changes[i+1].ResourceID)
			}
			for _, change := range plan.Changes {
				require.Equal(t, ResourceTypeAIGatewayAgent, change.ResourceType)
				require.Equal(t, "default", change.Namespace)
				require.Equal(t, &ParentInfo{Ref: "support-gateway", ID: "gateway-id"}, change.Parent)
			}
		})
	}
}

type idMatchedChildAgentAPI struct {
	*testAIGatewayAgentAPI
	missingDetail bool
	getIDs        []string
}

func (a *idMatchedChildAgentAPI) GetAiGatewayAgent(
	ctx context.Context,
	gatewayID string,
	agentID string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayAgentResponse, error) {
	a.getIDs = append(a.getIDs, agentID)
	if a.missingDetail {
		return &kkOps.GetAiGatewayAgentResponse{}, nil
	}
	return a.testAIGatewayAgentAPI.GetAiGatewayAgent(ctx, gatewayID, agentID, opts...)
}
