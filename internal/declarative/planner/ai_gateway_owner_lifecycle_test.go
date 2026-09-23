package planner

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestConsumerOwnerLifecycleMissingDetail(t *testing.T) {
	for _, failRead := range []bool{false, true} {
		name := "recreate with dependent credential"
		if failRead {
			name = "read failure stops before pruning"
		}
		t.Run(name, func(t *testing.T) {
			desired := testAIGatewayConsumerResource(t, nil)
			rs := testAIGatewayConsumerResourceSet(desired)
			rs.AIGatewayConsumerCredentials = []resources.AIGatewayConsumerCredentialResource{
				testAIGatewayConsumerCredentialResource(t, "new-key", "New key", "create"),
			}
			obsolete := testAIGatewayConsumer(nil)
			obsolete.ID, obsolete.Name = "obsolete-id", "obsolete"
			readErr := errors.New("consumer detail unavailable")
			api := &ownerLifecycleConsumerAPI{
				testAIGatewayConsumerAPI: testAIGatewayConsumerAPI{
					consumers: []kkComps.AIGatewayConsumer{testAIGatewayConsumer(nil), obsolete},
				},
				get: func(id string) (*kkComps.AIGatewayConsumer, error) {
					require.Equal(t, "consumer-id", id)
					require.Equal(t, id, rs.AIGatewayConsumers[0].GetKonnectID(), "bind before reading details")
					if failRead {
						return nil, readErr
					}
					return nil, nil
				},
			}
			p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayConsumersAPI: api}), slog.Default())
			p.resources = rs
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
			err := p.planAIGatewayConsumerChanges(t.Context(), "default", "support-gateway", "Support Gateway",
				"gateway-id", "", nil, rs.AIGatewayConsumers, plan)
			if failRead {
				require.ErrorIs(t, err, readErr)
				require.Empty(t, plan.Changes)
				return
			}
			require.NoError(t, err)
			require.Len(t, plan.Changes, 3)
			require.Equal(t, ResourceTypeAIGatewayConsumer, plan.Changes[0].ResourceType)
			require.Equal(t, ActionCreate, plan.Changes[0].Action)
			credential := plan.Changes[1]
			require.Equal(t, ResourceTypeAIGatewayConsumerCredential, credential.ResourceType)
			require.Equal(t, ActionCreate, credential.Action)
			require.Equal(t, []string{plan.Changes[0].ID}, credential.DependsOn)
			require.Empty(t, credential.Parent.ID, "a recreated consumer must not reuse its stale listed ID")
			require.Equal(t, ActionDelete, plan.Changes[2].Action)
			require.Equal(t, "obsolete-id", plan.Changes[2].ResourceID)
		})
	}
}

func TestConsumerOwnerLifecyclePlansCredentialsAfterNoopAndUpdate(t *testing.T) {
	for _, update := range []bool{false, true} {
		name := "no-op"
		if update {
			name = "update"
		}
		t.Run(name, func(t *testing.T) {
			desired := testAIGatewayConsumerResource(t, nil)
			if update {
				desired.DisplayName = "Updated"
			}
			rs := testAIGatewayConsumerResourceSet(desired)
			rs.EnsureSyncScope().AddChild(resources.ResourceTypeAIGatewayConsumer, desired.Ref,
				resources.ResourceTypeAIGatewayConsumerCredential)
			detail := testAIGatewayConsumer(nil)
			detail.ID = "detail-id"
			api := &ownerLifecycleConsumerAPI{
				testAIGatewayConsumerAPI: testAIGatewayConsumerAPI{
					consumers: []kkComps.AIGatewayConsumer{testAIGatewayConsumer(nil)},
					credentials: []kkComps.AIGatewayConsumerCredential{
						testAIGatewayConsumerCredential("stale-key-id", "stale-key", "Old", "retained"),
					},
				},
				get: func(string) (*kkComps.AIGatewayConsumer, error) { return &detail, nil },
			}
			p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayConsumersAPI: api}), slog.Default())
			p.resources = rs
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
			err := p.planAIGatewayConsumerChanges(t.Context(), "default", "support-gateway", "Support Gateway",
				"gateway-id", "", nil, rs.AIGatewayConsumers, plan)
			require.NoError(t, err)
			require.Equal(t, []string{"consumer-id"}, api.credentialParents)
			if update {
				require.Len(t, plan.Changes, 2)
				require.Equal(t, ActionUpdate, plan.Changes[0].Action)
				require.Equal(t, "consumer-id", plan.Changes[0].ResourceID)
			} else {
				require.Len(t, plan.Changes, 1)
			}
			change := plan.Changes[len(plan.Changes)-1]
			require.Equal(t, ActionDelete, change.Action)
			require.Equal(t, "stale-key-id", change.ResourceID)
			require.Equal(t, "consumer-id", change.Parent.ID)
		})
	}
}

func TestConfigStoreOwnerLifecycleChildErrorStopsTraversal(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "new store"
		if existing {
			name = "unchanged store"
		}
		t.Run(name, func(t *testing.T) {
			rs := testAIGatewayConfigStoreResourceSet(resources.AIGatewayConfigStoreResource{
				BaseResource: resources.BaseResource{Ref: "support-store"},
				Name:         "support-store",
				AIGateway:    "support-gateway",
			}, resources.AIGatewayConfigStoreResource{
				BaseResource: resources.BaseResource{Ref: "later-store"},
				Name:         "later-store",
				AIGateway:    "support-gateway",
			})
			rs.AIGatewayConfigStoreSecrets = []resources.AIGatewayConfigStoreSecretResource{{
				BaseResource:         resources.BaseResource{Ref: "missing-value"},
				AIGatewayConfigStore: "support-store",
				Key:                  "missing-key",
			}}
			api := &testAIGatewayConfigStoreAPI{
				stores: []kkComps.AIGatewayConfigStore{{ID: "obsolete-id", Name: "obsolete"}},
			}
			if existing {
				api.stores = append(api.stores, kkComps.AIGatewayConfigStore{ID: "store-id", Name: "support-store"})
			}
			p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayConfigStoresAPI: api}), slog.Default())
			p.resources = rs
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
			err := p.planAIGatewayConfigStoreChanges(t.Context(), "default", "support-gateway", "Support Gateway",
				"gateway-id", "", rs.AIGatewayConfigStores, plan)
			require.ErrorContains(t, err, "requires value: !secret")
			if existing {
				require.Empty(t, plan.Changes)
				require.Equal(t, "store-id", rs.AIGatewayConfigStores[0].GetKonnectID())
			} else {
				require.Len(t, plan.Changes, 1)
				require.Equal(t, ActionCreate, plan.Changes[0].Action)
				require.Equal(t, "support-store", plan.Changes[0].ResourceRef)
			}
		})
	}
}

type ownerLifecycleConsumerAPI struct {
	testAIGatewayConsumerAPI
	get               func(string) (*kkComps.AIGatewayConsumer, error)
	credentialParents []string
}

func (a *ownerLifecycleConsumerAPI) GetAiGatewayConsumer(
	_ context.Context, _ string, id string, _ ...kkOps.Option,
) (*kkOps.GetAiGatewayConsumerResponse, error) {
	current, err := a.get(id)
	return &kkOps.GetAiGatewayConsumerResponse{AIGatewayConsumer: current}, err
}

func (a *ownerLifecycleConsumerAPI) ListAiGatewayConsumerCredentials(
	ctx context.Context, request kkOps.ListAiGatewayConsumerCredentialsRequest, opts ...kkOps.Option,
) (*kkOps.ListAiGatewayConsumerCredentialsResponse, error) {
	a.credentialParents = append(a.credentialParents, request.ConsumerID)
	return a.testAIGatewayConsumerAPI.ListAiGatewayConsumerCredentials(ctx, request, opts...)
}
