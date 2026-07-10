package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutorSyncAIGatewayConsumerGroupConsumersUsesResolvedGatewayReference(t *testing.T) {
	ctx := testContextWithLogger()
	api := &executorAIGatewayConsumerGroupsAPI{
		t: t,
		addConsumer: func(req kkOps.AddAiGatewayConsumerToConsumerGroupRequest) {
			assert.Equal(t, "gateway-id", req.GatewayID)
			assert.Equal(t, "group-id", req.ConsumerGroupID)
			assert.Equal(t, "support-agent", req.AddAIGatewayConsumerToGroupRequest.Consumer)
		},
	}
	client := state.NewClient(state.ClientConfig{
		AIGatewayConsumerGroupsAPI: api,
	})
	exec := New(client, nil, false)
	change := &planner.PlannedChange{
		Fields: map[string]any{
			planner.FieldConsumers: []string{"support-agent"},
		},
		References: map[string]planner.ReferenceInfo{
			planner.FieldAIGatewayID: {ID: "gateway-id"},
		},
	}

	require.NoError(t, exec.syncAIGatewayConsumerGroupConsumers(ctx, change, "group-id"))
	assert.Equal(t, 1, api.addCalls)
}

type executorAIGatewayConsumerGroupsAPI struct {
	t           *testing.T
	addCalls    int
	addConsumer func(kkOps.AddAiGatewayConsumerToConsumerGroupRequest)
}

func (e *executorAIGatewayConsumerGroupsAPI) ListAiGatewayConsumerGroups(
	context.Context,
	kkOps.ListAiGatewayConsumerGroupsRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayConsumerGroupsResponse, error) {
	e.t.Helper()
	return &kkOps.ListAiGatewayConsumerGroupsResponse{}, nil
}

func (e *executorAIGatewayConsumerGroupsAPI) CreateAiGatewayConsumerGroup(
	context.Context,
	string,
	kkComps.CreateAIGatewayConsumerGroupRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayConsumerGroupResponse, error) {
	e.t.Helper()
	return &kkOps.CreateAiGatewayConsumerGroupResponse{}, nil
}

func (e *executorAIGatewayConsumerGroupsAPI) GetAiGatewayConsumerGroup(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetAiGatewayConsumerGroupResponse, error) {
	e.t.Helper()
	return &kkOps.GetAiGatewayConsumerGroupResponse{}, nil
}

func (e *executorAIGatewayConsumerGroupsAPI) UpdateAiGatewayConsumerGroup(
	context.Context,
	kkOps.UpdateAiGatewayConsumerGroupRequest,
	...kkOps.Option,
) (*kkOps.UpdateAiGatewayConsumerGroupResponse, error) {
	e.t.Helper()
	return &kkOps.UpdateAiGatewayConsumerGroupResponse{}, nil
}

func (e *executorAIGatewayConsumerGroupsAPI) DeleteAiGatewayConsumerGroup(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayConsumerGroupResponse, error) {
	e.t.Helper()
	return &kkOps.DeleteAiGatewayConsumerGroupResponse{}, nil
}

func (e *executorAIGatewayConsumerGroupsAPI) ListAiGatewayConsumersInConsumerGroup(
	context.Context,
	kkOps.ListAiGatewayConsumersInConsumerGroupRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayConsumersInConsumerGroupResponse, error) {
	e.t.Helper()
	return &kkOps.ListAiGatewayConsumersInConsumerGroupResponse{
		ListAIGatewayConsumersResponse: &kkComps.ListAIGatewayConsumersResponse{},
	}, nil
}

func (e *executorAIGatewayConsumerGroupsAPI) AddAiGatewayConsumerToConsumerGroup(
	_ context.Context,
	request kkOps.AddAiGatewayConsumerToConsumerGroupRequest,
	_ ...kkOps.Option,
) (*kkOps.AddAiGatewayConsumerToConsumerGroupResponse, error) {
	e.t.Helper()
	e.addCalls++
	if e.addConsumer != nil {
		e.addConsumer(request)
	}
	return &kkOps.AddAiGatewayConsumerToConsumerGroupResponse{}, nil
}

func (e *executorAIGatewayConsumerGroupsAPI) RemoveAiGatewayConsumerFromConsumerGroup(
	context.Context,
	kkOps.RemoveAiGatewayConsumerFromConsumerGroupRequest,
	...kkOps.Option,
) (*kkOps.RemoveAiGatewayConsumerFromConsumerGroupResponse, error) {
	e.t.Helper()
	return &kkOps.RemoveAiGatewayConsumerFromConsumerGroupResponse{}, nil
}
