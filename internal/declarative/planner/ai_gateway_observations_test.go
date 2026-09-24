package planner

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayPolicyObservationsSharedAndReset(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		name := "unscoped"
		if scoped {
			name = "scoped"
		}
		t.Run(name, func(t *testing.T) {
			api := &policyObservationAPI{
				testAIGatewayAgentAPI:         &testAIGatewayAgentAPI{},
				testAIGatewayConsumerAPI:      &testAIGatewayConsumerAPI{},
				testAIGatewayConsumerGroupAPI: &testAIGatewayConsumerGroupAPI{},
				testAIGatewayModelAPI:         &testAIGatewayModelAPI{},
				testAIGatewayMCPServerAPI:     &testAIGatewayMCPServerAPI{},
				calls:                         make(map[string]int),
			}
			cfg := policyOrderClientConfig(t, "", "")
			cfg.AIGatewayAgentsAPI = api
			cfg.AIGatewayConsumersAPI = api
			cfg.AIGatewayConsumerGroupsAPI = api
			cfg.AIGatewayModelAPI = api
			cfg.AIGatewayMCPServersAPI = api
			scope := resources.NewSyncScope()
			scope.AddRoot(resources.ResourceTypeAIGateway)
			scope.AddChild(resources.ResourceTypeAIGateway, "support-gateway", resources.ResourceTypeAIGatewayPolicy)
			kinds := []resources.ResourceType{
				resources.ResourceTypeAIGatewayAgent, resources.ResourceTypeAIGatewayConsumer,
				resources.ResourceTypeAIGatewayConsumerGroup, resources.ResourceTypeAIGatewayModel,
				resources.ResourceTypeAIGatewayMCPServer,
			}
			if scoped {
				for _, kind := range kinds {
					scope.AddChild(resources.ResourceTypeAIGateway, "support-gateway", kind)
				}
			}
			rs := &resources.ResourceSet{AIGateways: []resources.AIGatewayResource{testAIGatewayResource()}, SyncScope: scope}
			p := NewPlanner(state.NewClient(cfg), slog.Default())
			for run := range 2 {
				plan, err := p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
				require.NoError(t, err)
				require.Len(t, plan.Changes, 1)
				for _, kind := range kinds {
					require.Equal(t, run+1, api.calls[string(kind)])
				}
			}
		})
	}
}

func TestCachedAIGatewayChildrenIsolationAndErrors(t *testing.T) {
	cache := make(map[string][]string)
	calls := make(map[string]int)
	unavailable := errors.New("unavailable")
	fetch := func(_ context.Context, gatewayID string) ([]string, error) {
		calls[gatewayID]++
		if gatewayID == "retry" && calls[gatewayID] == 1 {
			return nil, unavailable
		}
		return []string{gatewayID}, nil
	}
	for _, gatewayID := range []string{"a", "b", "a", "b"} {
		got, err := listCachedAIGatewayChildren(t.Context(), gatewayID, cache, fetch)
		require.NoError(t, err)
		require.Equal(t, []string{gatewayID}, got)
	}
	require.Equal(t, 1, calls["a"])
	require.Equal(t, 1, calls["b"])
	_, err := listCachedAIGatewayChildren(t.Context(), "retry", cache, fetch)
	require.ErrorIs(t, err, unavailable)
	_, err = listCachedAIGatewayChildren(t.Context(), "retry", cache, fetch)
	require.NoError(t, err)
	require.Equal(t, 2, calls["retry"])
}

type policyObservationAPI struct {
	*testAIGatewayAgentAPI
	*testAIGatewayConsumerAPI
	*testAIGatewayConsumerGroupAPI
	*testAIGatewayModelAPI
	*testAIGatewayMCPServerAPI
	calls map[string]int
}

func (a *policyObservationAPI) ListAiGatewayAgents(
	ctx context.Context, req kkOps.ListAiGatewayAgentsRequest, opts ...kkOps.Option,
) (*kkOps.ListAiGatewayAgentsResponse, error) {
	a.calls[ResourceTypeAIGatewayAgent]++
	return a.testAIGatewayAgentAPI.ListAiGatewayAgents(ctx, req, opts...)
}

func (a *policyObservationAPI) ListAiGatewayConsumers(
	ctx context.Context, req kkOps.ListAiGatewayConsumersRequest, opts ...kkOps.Option,
) (*kkOps.ListAiGatewayConsumersResponse, error) {
	a.calls[ResourceTypeAIGatewayConsumer]++
	return a.testAIGatewayConsumerAPI.ListAiGatewayConsumers(ctx, req, opts...)
}

func (a *policyObservationAPI) ListAiGatewayConsumerGroups(
	ctx context.Context, req kkOps.ListAiGatewayConsumerGroupsRequest, opts ...kkOps.Option,
) (*kkOps.ListAiGatewayConsumerGroupsResponse, error) {
	a.calls[ResourceTypeAIGatewayConsumerGroup]++
	return a.testAIGatewayConsumerGroupAPI.ListAiGatewayConsumerGroups(ctx, req, opts...)
}

func (a *policyObservationAPI) ListAiGatewayModels(
	ctx context.Context, req kkOps.ListAiGatewayModelsRequest, opts ...kkOps.Option,
) (*kkOps.ListAiGatewayModelsResponse, error) {
	a.calls[ResourceTypeAIGatewayModel]++
	return a.testAIGatewayModelAPI.ListAiGatewayModels(ctx, req, opts...)
}

func (a *policyObservationAPI) ListAiGatewayMcpServers(
	ctx context.Context, req kkOps.ListAiGatewayMcpServersRequest, opts ...kkOps.Option,
) (*kkOps.ListAiGatewayMcpServersResponse, error) {
	a.calls[ResourceTypeAIGatewayMCPServer]++
	return a.testAIGatewayMCPServerAPI.ListAiGatewayMcpServers(ctx, req, opts...)
}
