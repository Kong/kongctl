package state

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpsertAIGatewayConsumerGroupConsumersRemovesByID(t *testing.T) {
	api := &testAIGatewayConsumerGroupsAPI{
		t: t,
		currentConsumers: []kkComps.AIGatewayConsumer{
			{
				ID:          "consumer-id",
				Name:        "consumer-name",
				DisplayName: "Consumer",
				Type:        kkComps.AIGatewayConsumerTypeAPIKey,
			},
		},
		removeConsumer: func(req kkOps.RemoveAiGatewayConsumerFromConsumerGroupRequest) {
			assert.Equal(t, "gateway-id", req.GatewayID)
			assert.Equal(t, "group-id", req.ConsumerGroupID)
			assert.Equal(t, "consumer-id", req.ConsumerIDOrName)
		},
	}
	client := NewClient(ClientConfig{AIGatewayConsumerGroupsAPI: api})

	require.NoError(t, client.UpsertAIGatewayConsumerGroupConsumers(
		context.Background(),
		"gateway-id",
		"group-id",
		nil,
	))
	assert.Equal(t, 1, api.removeCalls)
}

type testAIGatewayConsumerGroupsAPI struct {
	t                *testing.T
	currentConsumers []kkComps.AIGatewayConsumer
	removeCalls      int
	removeConsumer   func(kkOps.RemoveAiGatewayConsumerFromConsumerGroupRequest)
}

func (t *testAIGatewayConsumerGroupsAPI) ListAiGatewayConsumerGroups(
	context.Context,
	kkOps.ListAiGatewayConsumerGroupsRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayConsumerGroupsResponse, error) {
	t.t.Helper()
	return &kkOps.ListAiGatewayConsumerGroupsResponse{}, nil
}

func (t *testAIGatewayConsumerGroupsAPI) CreateAiGatewayConsumerGroup(
	context.Context,
	string,
	kkComps.CreateAIGatewayConsumerGroupRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayConsumerGroupResponse, error) {
	t.t.Helper()
	return &kkOps.CreateAiGatewayConsumerGroupResponse{}, nil
}

func (t *testAIGatewayConsumerGroupsAPI) GetAiGatewayConsumerGroup(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetAiGatewayConsumerGroupResponse, error) {
	t.t.Helper()
	return &kkOps.GetAiGatewayConsumerGroupResponse{}, nil
}

func (t *testAIGatewayConsumerGroupsAPI) UpdateAiGatewayConsumerGroup(
	context.Context,
	kkOps.UpdateAiGatewayConsumerGroupRequest,
	...kkOps.Option,
) (*kkOps.UpdateAiGatewayConsumerGroupResponse, error) {
	t.t.Helper()
	return &kkOps.UpdateAiGatewayConsumerGroupResponse{}, nil
}

func (t *testAIGatewayConsumerGroupsAPI) DeleteAiGatewayConsumerGroup(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayConsumerGroupResponse, error) {
	t.t.Helper()
	return &kkOps.DeleteAiGatewayConsumerGroupResponse{}, nil
}

func (t *testAIGatewayConsumerGroupsAPI) ListAiGatewayConsumersInConsumerGroup(
	context.Context,
	kkOps.ListAiGatewayConsumersInConsumerGroupRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayConsumersInConsumerGroupResponse, error) {
	t.t.Helper()
	return &kkOps.ListAiGatewayConsumersInConsumerGroupResponse{
		ListAIGatewayConsumersResponse: &kkComps.ListAIGatewayConsumersResponse{
			Data: t.currentConsumers,
		},
	}, nil
}

func (t *testAIGatewayConsumerGroupsAPI) AddAiGatewayConsumerToConsumerGroup(
	context.Context,
	kkOps.AddAiGatewayConsumerToConsumerGroupRequest,
	...kkOps.Option,
) (*kkOps.AddAiGatewayConsumerToConsumerGroupResponse, error) {
	t.t.Helper()
	return &kkOps.AddAiGatewayConsumerToConsumerGroupResponse{}, nil
}

func (t *testAIGatewayConsumerGroupsAPI) RemoveAiGatewayConsumerFromConsumerGroup(
	_ context.Context,
	request kkOps.RemoveAiGatewayConsumerFromConsumerGroupRequest,
	_ ...kkOps.Option,
) (*kkOps.RemoveAiGatewayConsumerFromConsumerGroupResponse, error) {
	t.t.Helper()
	t.removeCalls++
	if t.removeConsumer != nil {
		t.removeConsumer(request)
	}
	return &kkOps.RemoveAiGatewayConsumerFromConsumerGroupResponse{}, nil
}
