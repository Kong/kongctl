package executor

import (
	"context"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutorAIGatewayRegistry(t *testing.T) {
	exec := New(nil, nil, false)
	for _, tc := range []struct {
		kind      string
		canUpdate bool
	}{
		{planner.ResourceTypeAIGateway, true},
		{planner.ResourceTypeAIGatewayProvider, true},
		{planner.ResourceTypeAIGatewayAuthStrategy, true},
		{planner.ResourceTypeAIGatewayPolicy, true},
		{planner.ResourceTypeAIGatewayAgent, true},
		{planner.ResourceTypeAIGatewayConsumer, true},
		{planner.ResourceTypeAIGatewayConsumerCredential, false},
		{planner.ResourceTypeAIGatewayConsumerGroup, true},
		{planner.ResourceTypeAIGatewayModel, true},
		{planner.ResourceTypeAIGatewayMCPServer, true},
		{planner.ResourceTypeAIGatewayVault, true},
		{planner.ResourceTypeAIGatewayConfigStore, true},
		{planner.ResourceTypeAIGatewayConfigStoreSecret, true},
		{planner.ResourceTypeAIGatewayDataPlaneCertificate, false},
		{planner.ResourceTypeAIGatewayCertificate, true},
		{planner.ResourceTypeAIGatewayCACertificate, true},
		{planner.ResourceTypeAIGatewaySNI, true},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			resource, ok := exec.resourceExecutors[tc.kind]
			require.True(t, ok, "missing runtime registration")
			require.NotNil(t, resource.contract)
			assert.Equal(t, tc.kind, resource.contract.ResourceType())
			assert.Same(t, resource.contract, exec.payloadContracts[tc.kind])
			assert.NotNil(t, resource.create)
			assert.NotNil(t, resource.remove)
			if tc.canUpdate {
				assert.NotNil(t, resource.update)
			} else {
				assert.Nil(t, resource.update)
				change := &planner.PlannedChange{
					ResourceType: tc.kind,
					References: map[string]planner.ReferenceInfo{
						planner.FieldAIGatewayID: {Ref: "gateway-ref", ID: resources.UnknownReferenceID},
					},
				}
				// Unsupported updates must bypass preparation, even with an unresolved gateway.
				id, err := exec.updateResource(testContextWithLogger(), change)
				assert.Empty(t, id)
				assert.EqualError(t, err, "update operation not yet implemented for "+tc.kind)
				assert.Equal(t, resources.UnknownReferenceID, change.References[planner.FieldAIGatewayID].ID)
			}
		})
	}
}

func TestExecutorAIGatewayConsumerGroupDispatch(t *testing.T) {
	for _, tc := range []struct {
		name              string
		action            planner.ActionType
		unresolvedGateway bool
		failAt            string
		wantCalls         []string
	}{
		{
			name: "create", action: planner.ActionCreate,
			wantCalls: []string{"create", "list consumers", "add consumer"},
		},
		{
			name: "update", action: planner.ActionUpdate,
			wantCalls: []string{"get", "update", "list consumers", "add consumer"},
		},
		{
			name: "delete without membership synchronization", action: planner.ActionDelete,
			wantCalls: []string{"get", "delete"},
		},
		{name: "create preparation failure", action: planner.ActionCreate, unresolvedGateway: true},
		{name: "update preparation failure", action: planner.ActionUpdate, unresolvedGateway: true},
		{name: "delete preparation failure", action: planner.ActionDelete, unresolvedGateway: true},
		{
			name: "create failure", action: planner.ActionCreate, failAt: "create",
			wantCalls: []string{"create"},
		},
		{
			name: "update failure", action: planner.ActionUpdate, failAt: "update",
			wantCalls: []string{"get", "update"},
		},
		{
			name: "delete failure", action: planner.ActionDelete, failAt: "delete",
			wantCalls: []string{"get", "delete"},
		},
		{
			name: "create membership failure", action: planner.ActionCreate, failAt: "add consumer",
			wantCalls: []string{"create", "list consumers", "add consumer"},
		},
		{
			name: "update membership failure", action: planner.ActionUpdate, failAt: "add consumer",
			wantCalls: []string{"get", "update", "list consumers", "add consumer"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			change := &planner.PlannedChange{
				ResourceType: planner.ResourceTypeAIGatewayConsumerGroup,
				ResourceRef:  "group-ref",
				Action:       tc.action,
				Fields: map[string]any{
					planner.FieldDisplayName: "Support group",
					planner.FieldConsumers:   []string{"support-agent"},
				},
				References: map[string]planner.ReferenceInfo{
					planner.FieldAIGatewayID: {Ref: "gateway-ref", ID: resources.UnknownReferenceID},
				},
				Parent: &planner.ParentInfo{Ref: "gateway-ref", ID: resources.UnknownReferenceID},
			}
			if tc.action == planner.ActionCreate {
				change.Fields[planner.FieldName] = "support-group"
			} else {
				change.ResourceID = "group-id"
			}
			api := &dispatchAIGatewayConsumerGroupsAPI{t: t, change: change, failAt: tc.failAt}
			exec := New(state.NewClient(state.ClientConfig{AIGatewayConsumerGroupsAPI: api}), nil, false)
			if !tc.unresolvedGateway {
				exec.setRef(planner.ResourceTypeAIGateway, "gateway-ref", "gateway-id")
			}

			ctx := testContextWithLogger()
			var id string
			var err error
			switch tc.action {
			case planner.ActionCreate:
				id, err = exec.createResource(ctx, change)
			case planner.ActionUpdate:
				id, err = exec.updateResource(ctx, change)
			case planner.ActionDelete:
				err = exec.deleteResource(ctx, change)
			case planner.ActionExternalTool:
				t.Fatalf("unexpected action %s", tc.action)
			}

			assert.Equal(t, tc.wantCalls, api.calls)
			switch {
			case tc.unresolvedGateway:
				assert.ErrorContains(t, err, "failed to resolve AI Gateway reference")
				assert.ErrorContains(t, err, "AI Gateway API not configured")
				assert.Empty(t, id)
				assert.Equal(t, resources.UnknownReferenceID, change.References[planner.FieldAIGatewayID].ID)
			case tc.failAt != "":
				assert.ErrorContains(t, err, tc.failAt+" failed")
				assert.Empty(t, id)
			default:
				require.NoError(t, err)
				if tc.action != planner.ActionDelete {
					assert.Equal(t, "group-id", id)
				}
			}
		})
	}
}

// Unused methods remain nil so an unexpected API operation fails the test.
type dispatchAIGatewayConsumerGroupsAPI struct {
	helpers.AIGatewayConsumerGroupsAPI
	t      *testing.T
	change *planner.PlannedChange
	failAt string
	calls  []string
}

func (a *dispatchAIGatewayConsumerGroupsAPI) record(operation, gatewayID, groupID string) error {
	a.t.Helper()
	a.calls = append(a.calls, operation)
	assert.Equal(a.t, "gateway-id", a.change.References[planner.FieldAIGatewayID].ID,
		"gateway reference must be resolved before %s", operation)
	assert.Equal(a.t, "gateway-id", gatewayID)
	if operation != "create" {
		assert.Equal(a.t, "group-id", groupID)
	}
	if a.failAt == operation {
		return fmt.Errorf("%s failed", operation)
	}
	return nil
}

func (a *dispatchAIGatewayConsumerGroupsAPI) CreateAiGatewayConsumerGroup(
	_ context.Context,
	gatewayID string,
	req kkComps.CreateAIGatewayConsumerGroupRequest,
	_ ...kkOps.Option,
) (*kkOps.CreateAiGatewayConsumerGroupResponse, error) {
	a.t.Helper()
	assert.Equal(a.t, "support-group", req.Name)
	assert.Equal(a.t, "Support group", req.DisplayName)
	if err := a.record("create", gatewayID, ""); err != nil {
		return nil, err
	}
	return &kkOps.CreateAiGatewayConsumerGroupResponse{
		AIGatewayConsumerGroup: &kkComps.AIGatewayConsumerGroup{ID: "group-id"},
	}, nil
}

func (a *dispatchAIGatewayConsumerGroupsAPI) GetAiGatewayConsumerGroup(
	_ context.Context,
	gatewayID, groupID string,
	_ ...kkOps.Option,
) (*kkOps.GetAiGatewayConsumerGroupResponse, error) {
	a.t.Helper()
	if err := a.record("get", gatewayID, groupID); err != nil {
		return nil, err
	}
	return &kkOps.GetAiGatewayConsumerGroupResponse{
		AIGatewayConsumerGroup: &kkComps.AIGatewayConsumerGroup{ID: groupID, Name: "support-group"},
	}, nil
}

func (a *dispatchAIGatewayConsumerGroupsAPI) UpdateAiGatewayConsumerGroup(
	_ context.Context,
	req kkOps.UpdateAiGatewayConsumerGroupRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateAiGatewayConsumerGroupResponse, error) {
	a.t.Helper()
	assert.Equal(a.t, "Support group", req.UpdateAIGatewayConsumerGroupRequest.DisplayName)
	if err := a.record("update", req.GatewayID, req.ConsumerGroupIDOrName); err != nil {
		return nil, err
	}
	return &kkOps.UpdateAiGatewayConsumerGroupResponse{
		AIGatewayConsumerGroup: &kkComps.AIGatewayConsumerGroup{ID: req.ConsumerGroupIDOrName},
	}, nil
}

func (a *dispatchAIGatewayConsumerGroupsAPI) DeleteAiGatewayConsumerGroup(
	_ context.Context,
	gatewayID, groupID string,
	_ ...kkOps.Option,
) (*kkOps.DeleteAiGatewayConsumerGroupResponse, error) {
	a.t.Helper()
	if err := a.record("delete", gatewayID, groupID); err != nil {
		return nil, err
	}
	return &kkOps.DeleteAiGatewayConsumerGroupResponse{}, nil
}

func (a *dispatchAIGatewayConsumerGroupsAPI) ListAiGatewayConsumersInConsumerGroup(
	_ context.Context,
	req kkOps.ListAiGatewayConsumersInConsumerGroupRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAiGatewayConsumersInConsumerGroupResponse, error) {
	a.t.Helper()
	if err := a.record("list consumers", req.GatewayID, req.ConsumerGroupID); err != nil {
		return nil, err
	}
	return &kkOps.ListAiGatewayConsumersInConsumerGroupResponse{
		ListAIGatewayConsumersResponse: &kkComps.ListAIGatewayConsumersResponse{},
	}, nil
}

func (a *dispatchAIGatewayConsumerGroupsAPI) AddAiGatewayConsumerToConsumerGroup(
	_ context.Context,
	req kkOps.AddAiGatewayConsumerToConsumerGroupRequest,
	_ ...kkOps.Option,
) (*kkOps.AddAiGatewayConsumerToConsumerGroupResponse, error) {
	a.t.Helper()
	assert.Equal(a.t, "support-agent", req.AddAIGatewayConsumerToGroupRequest.Consumer)
	if err := a.record("add consumer", req.GatewayID, req.ConsumerGroupID); err != nil {
		return nil, err
	}
	return &kkOps.AddAiGatewayConsumerToConsumerGroupResponse{}, nil
}
