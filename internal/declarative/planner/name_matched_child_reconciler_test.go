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

func TestNameMatchedChildReconcilerIdentityAndSyncPruning(t *testing.T) {
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
		desiredName   string
		missingDetail bool
		wantGetIDs    []string
		wantAction    ActionType
		wantID        string
		wantDeleteIDs []string
	}{
		{
			name:          "name overrides cached ID and UUID ref",
			boundID:       boundID,
			ref:           refID,
			wantGetIDs:    []string{nameID},
			wantAction:    ActionUpdate,
			wantID:        nameID,
			wantDeleteIDs: []string{boundID, refID, staleID},
		},
		{
			name:          "name overrides UUID ref",
			ref:           refID,
			wantGetIDs:    []string{nameID},
			wantAction:    ActionUpdate,
			wantID:        nameID,
			wantDeleteIDs: []string{boundID, refID, staleID},
		},
		{
			name:          "missing UUID ref does not hide name match",
			ref:           missingID,
			wantGetIDs:    []string{nameID},
			wantAction:    ActionUpdate,
			wantID:        nameID,
			wantDeleteIDs: []string{boundID, refID, staleID},
		},
		{
			name:          "missing cached ID does not hide name match",
			boundID:       missingID,
			ref:           refID,
			wantGetIDs:    []string{nameID},
			wantAction:    ActionUpdate,
			wantID:        nameID,
			wantDeleteIDs: []string{boundID, refID, staleID},
		},
		{
			name:          "new name creates and sync deletes resources named only by ID or ref",
			boundID:       boundID,
			ref:           refID,
			desiredName:   "new-agent",
			wantAction:    ActionCreate,
			wantDeleteIDs: []string{boundID, refID, nameID, staleID},
		},
		{
			name:          "missing detail recreates and sync retains only the desired name",
			boundID:       boundID,
			ref:           refID,
			missingDetail: true,
			wantGetIDs:    []string{nameID},
			wantAction:    ActionCreate,
			wantDeleteIDs: []string{boundID, refID, staleID},
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
			api := &nameMatchedChildAgentAPI{
				testAIGatewayAgentAPI: &testAIGatewayAgentAPI{agents: current},
				missingDetail:         tt.missingDetail,
			}
			desired := testAIGatewayAgentResource(t, nil)
			desired.Ref = tt.ref
			desired.SetKonnectID(tt.boundID)
			if tt.desiredName != "" {
				desired.Name = tt.desiredName
			}
			desired.DisplayName = "Updated Agent"
			rs := testAIGatewayAgentResourceSet(desired)
			p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayAgentsAPI: api}), slog.Default())
			p.resources = rs
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)

			err := p.planAIGatewayAgentChanges(t.Context(), "default", "support-gateway", "Support Gateway",
				"gateway-id", "", nil, rs.AIGatewayAgents, plan)

			require.NoError(t, err)
			require.Equal(t, tt.wantGetIDs, api.getIDs)
			if len(tt.wantGetIDs) > 0 {
				require.Equal(t, nameID, rs.AIGatewayAgents[0].GetKonnectID())
			}
			require.Len(t, plan.Changes, 1+len(tt.wantDeleteIDs))
			change := plan.Changes[0]
			require.Equal(t, tt.wantAction, change.Action)
			require.Equal(t, tt.wantID, change.ResourceID)
			require.Equal(t, tt.ref, change.ResourceRef)
			require.Equal(t, "Updated Agent", change.Fields[FieldDisplayName])
			if tt.wantAction == ActionUpdate {
				require.Contains(t, change.ChangedFields, FieldDisplayName)
			}
			// UUID refs and cached IDs cannot retain differently named children.
			// Unmatched children are deleted in observed order.
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

func TestNameMatchedChildResourceMatchers(t *testing.T) {
	const (
		id      = "11111111-1111-4111-8111-111111111111"
		otherID = "22222222-2222-4222-8222-222222222222"
	)
	for _, tt := range []struct {
		name     string
		ref      string
		cachedID string
		sameName bool
	}{
		{name: "UUID ref cannot override name", ref: id},
		{name: "cached ID cannot override name", ref: "local-ref", cachedID: id},
		{name: "unrelated UUID and cached ID cannot hide name match", ref: otherID, cachedID: otherID, sameName: true},
		{name: "matching UUID and cached ID allow name match", ref: id, cachedID: id, sameName: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			agent := testAIGatewayAgentResource(t, nil)
			model := testAIGatewayModelResource(t)
			vault := testAIGatewayVaultResource(t)
			agent.Ref, model.Ref, vault.Ref = tt.ref, tt.ref, tt.ref
			agent.SetKonnectID(tt.cachedID)
			model.SetKonnectID(tt.cachedID)
			vault.SetKonnectID(tt.cachedID)

			currentAgent := testAIGatewayAgent(nil)
			currentAgent.ID = id
			modelName, vaultName := model.Name(), vault.Name()
			if !tt.sameName {
				currentAgent.Name = "different-name"
				modelName, vaultName = "different-name", "different-name"
			}
			for _, resource := range []struct {
				kind    string
				desired interface {
					TryMatchKonnectResource(any) bool
					GetKonnectID() string
				}
				current any
			}{
				{ResourceTypeAIGatewayAgent, &agent, currentAgent},
				{ResourceTypeAIGatewayModel, &model, testAIGatewayModel(id, modelName)},
				{ResourceTypeAIGatewayVault, &vault, testAIGatewayVault(t, id, vaultName, "SUPPORT_")},
			} {
				t.Run(resource.kind, func(t *testing.T) {
					require.Equal(t, tt.sameName, resource.desired.TryMatchKonnectResource(resource.current))
					if tt.sameName {
						require.Equal(t, id, resource.desired.GetKonnectID())
					}
				})
			}
		})
	}
}

type nameMatchedChildAgentAPI struct {
	*testAIGatewayAgentAPI
	missingDetail bool
	getIDs        []string
}

func (a *nameMatchedChildAgentAPI) GetAiGatewayAgent(
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
