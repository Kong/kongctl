package planner

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayConsumerPlannerCreatesChildForExistingGateway(t *testing.T) {
	consumer := testAIGatewayConsumerResource(t, nil)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayConsumersAPI: &testAIGatewayConsumerAPI{},
	})
	rs := testAIGatewayConsumerResourceSet(consumer)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionCreate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayConsumer, change.ResourceType)
	require.Equal(t, "support-user", change.ResourceRef)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
	require.Equal(t, "support-gateway", change.Parent.Ref)
	require.Equal(t, "Support User", change.Fields[FieldDisplayName])
	require.Equal(t, "api-key", change.Fields[FieldType])
}

func TestAIGatewayConsumerPlannerUpdatesExistingConsumer(t *testing.T) {
	consumer := testAIGatewayConsumerResource(t, nil)
	consumer.DisplayName = "Support User Updated"
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayConsumersAPI: &testAIGatewayConsumerAPI{
			consumers: []kkComps.AIGatewayConsumer{
				testAIGatewayConsumer(nil),
			},
		},
	})
	rs := testAIGatewayConsumerResourceSet(consumer)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionUpdate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayConsumer, change.ResourceType)
	require.Equal(t, "consumer-id", change.ResourceID)
	require.Equal(t, "Support User Updated", change.Fields[FieldDisplayName])
	require.Contains(t, change.ChangedFields, FieldDisplayName)
}

func TestAIGatewayConsumerPlannerSyncDeletesScopedConsumers(t *testing.T) {
	scope := resources.NewSyncScope()
	scope.AddRoot(resources.ResourceTypeAIGateway)
	scope.AddChild(resources.ResourceTypeAIGateway, "support-gateway", resources.ResourceTypeAIGatewayConsumer)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayConsumersAPI: &testAIGatewayConsumerAPI{
			consumers: []kkComps.AIGatewayConsumer{
				testAIGatewayConsumer(nil),
			},
		},
	})
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{testAIGatewayResource()},
		SyncScope:  scope,
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionDelete, change.Action)
	require.Equal(t, ResourceTypeAIGatewayConsumer, change.ResourceType)
	require.Equal(t, "consumer-id", change.ResourceID)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
}

func TestAIGatewayConsumerPlannerDependsOnPolicyCreate(t *testing.T) {
	policy := testAIGatewayPolicyResource(t)
	consumer := testAIGatewayConsumerResource(t, []string{tags.RefPlaceholderPrefix + "mask-sensitive-data#id"})
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{},
	})
	rs := &resources.ResourceSet{
		AIGateways:          []resources.AIGatewayResource{testAIGatewayResource()},
		AIGatewayPolicies:   []resources.AIGatewayPolicyResource{policy},
		AIGatewayConsumers:  []resources.AIGatewayConsumerResource{consumer},
		AIGatewayProviders:  nil,
		AIGatewayModels:     nil,
		AIGatewayMCPServers: nil,
		AIGatewayVaults:     nil,
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)

	gatewayCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGateway, "support-gateway")
	policyCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGatewayPolicy, "mask-sensitive-data")
	consumerCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGatewayConsumer, "support-user")

	require.Contains(t, policyCreate.DependsOn, gatewayCreate.ID)
	require.Contains(t, consumerCreate.DependsOn, policyCreate.ID)
	require.Equal(t, resources.UnknownReferenceID, consumerCreate.References[FieldPolicies+".0"].ID)
	require.Equal(
		t,
		tags.RefPlaceholderPrefix+"mask-sensitive-data#name",
		consumerCreate.References[FieldPolicies+".0"].Ref,
	)
}

func TestAIGatewayConsumerPlannerPolicyRefNoopForExistingConsumer(t *testing.T) {
	for _, currentPolicyRef := range []string{"policy-id", "mask-sensitive-data"} {
		t.Run(currentPolicyRef, func(t *testing.T) {
			policy := testAIGatewayPolicyResource(t)
			consumer := testAIGatewayConsumerResource(t, []string{tags.RefPlaceholderPrefix + "mask-sensitive-data#id"})
			client := state.NewClient(state.ClientConfig{
				AIGatewayAPI: &testAIGatewayAPI{
					gateways: []kkComps.AIGateway{testAIGateway()},
				},
				AIGatewayPoliciesAPI: &testAIGatewayPolicyAPI{
					policies: []kkComps.AIGatewayPolicy{testAIGatewayPolicy()},
				},
				AIGatewayConsumersAPI: &testAIGatewayConsumerAPI{
					consumers: []kkComps.AIGatewayConsumer{
						testAIGatewayConsumer([]string{currentPolicyRef}),
					},
				},
			})
			rs := &resources.ResourceSet{
				AIGateways:         []resources.AIGatewayResource{testAIGatewayResource()},
				AIGatewayPolicies:  []resources.AIGatewayPolicyResource{policy},
				AIGatewayConsumers: []resources.AIGatewayConsumerResource{consumer},
			}

			plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
			require.NoError(t, err)
			require.Empty(t, plan.Changes)
		})
	}
}

func TestAIGatewayConsumerPlannerCreatesCredentialForExistingConsumer(t *testing.T) {
	consumer := testAIGatewayConsumerResource(t, nil)
	credential := testAIGatewayConsumerCredentialResource(t, "support-user-key", "Support User API Key", "create")
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayConsumersAPI: &testAIGatewayConsumerAPI{
			consumers: []kkComps.AIGatewayConsumer{
				testAIGatewayConsumer(nil),
			},
		},
	})
	rs := testAIGatewayConsumerResourceSet(consumer)
	rs.AIGatewayConsumerCredentials = []resources.AIGatewayConsumerCredentialResource{credential}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)

	change := plan.Changes[0]
	require.Equal(t, ActionCreate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayConsumerCredential, change.ResourceType)
	require.Equal(t, "support-user-key", change.ResourceRef)
	require.Equal(t, "support-user-key", change.Fields[FieldName])
	require.Equal(t, "api-key", change.Fields[FieldType])
	require.Equal(t, "Support User API Key", change.Fields[FieldDisplayName])
	require.Equal(t, map[string]any{"phase": "create"}, change.Fields[FieldLabels])
	require.NotNil(t, change.Parent)
	require.Equal(t, "support-user", change.Parent.Ref)
	require.Equal(t, "consumer-id", change.Parent.ID)
	require.Equal(t, "gateway-id", change.References[FieldAIGatewayID].ID)
	require.Equal(t, "consumer-id", change.References[FieldAIGatewayConsumerID].ID)
}

func TestAIGatewayConsumerPlannerCreatesCredentialAfterNewConsumer(t *testing.T) {
	consumer := testAIGatewayConsumerResource(t, nil)
	credential := testAIGatewayConsumerCredentialResource(t, "support-user-key", "Support User API Key", "create")
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayConsumersAPI: &testAIGatewayConsumerAPI{},
	})
	rs := testAIGatewayConsumerResourceSet(consumer)
	rs.AIGatewayConsumerCredentials = []resources.AIGatewayConsumerCredentialResource{credential}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)

	consumerCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGatewayConsumer, "support-user")
	credentialCreate := findAIGatewayModelTestChange(
		t,
		plan,
		ResourceTypeAIGatewayConsumerCredential,
		"support-user-key",
	)
	require.Equal(t, ActionCreate, credentialCreate.Action)
	require.Contains(t, credentialCreate.DependsOn, consumerCreate.ID)
	require.Equal(t, "support-user", credentialCreate.Parent.Ref)
	require.Empty(t, credentialCreate.Parent.ID)
	require.Equal(t, "gateway-id", credentialCreate.References[FieldAIGatewayID].ID)
	require.Empty(t, credentialCreate.References[FieldAIGatewayConsumerID].ID)
}

func TestAIGatewayConsumerPlannerReplacesCredentialWhenMetadataChanges(t *testing.T) {
	consumer := testAIGatewayConsumerResource(t, nil)
	credential := testAIGatewayConsumerCredentialResource(t, "support-user-key", "Support User API Key Updated", "update")
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayConsumersAPI: &testAIGatewayConsumerAPI{
			consumers: []kkComps.AIGatewayConsumer{
				testAIGatewayConsumer(nil),
			},
			credentials: []kkComps.AIGatewayConsumerCredential{
				testAIGatewayConsumerCredential("credential-id", "support-user-key", "Support User API Key", "create"),
			},
		},
	})
	rs := testAIGatewayConsumerResourceSet(consumer)
	rs.AIGatewayConsumerCredentials = []resources.AIGatewayConsumerCredentialResource{credential}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 2)

	deleteChange := plan.Changes[0]
	createChange := plan.Changes[1]
	require.Equal(t, ActionDelete, deleteChange.Action)
	require.Equal(t, ResourceTypeAIGatewayConsumerCredential, deleteChange.ResourceType)
	require.Equal(t, "credential-id", deleteChange.ResourceID)
	require.Contains(t, deleteChange.ChangedFields, FieldDisplayName)
	require.Contains(t, deleteChange.ChangedFields, FieldLabels)
	require.Equal(t, ActionCreate, createChange.Action)
	require.Equal(t, ResourceTypeAIGatewayConsumerCredential, createChange.ResourceType)
	require.Contains(t, createChange.DependsOn, deleteChange.ID)
	require.Equal(t, "Support User API Key Updated", createChange.Fields[FieldDisplayName])
}

func testAIGatewayConsumerResourceSet(
	consumer resources.AIGatewayConsumerResource,
) *resources.ResourceSet {
	return &resources.ResourceSet{
		AIGateways:         []resources.AIGatewayResource{testAIGatewayResource()},
		AIGatewayConsumers: []resources.AIGatewayConsumerResource{consumer},
	}
}

func testAIGatewayConsumerResource(
	t *testing.T,
	policies []string,
) resources.AIGatewayConsumerResource {
	t.Helper()
	payload := map[string]any{
		"ref":          "support-user",
		"ai_gateway":   "support-gateway",
		"name":         "support-user",
		"type":         "api-key",
		"display_name": "Support User",
	}
	if policies != nil {
		payload[FieldPolicies] = policies
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var consumer resources.AIGatewayConsumerResource
	require.NoError(t, json.Unmarshal(data, &consumer))
	return consumer
}

func testAIGatewayConsumerCredentialResource(
	t *testing.T,
	name string,
	displayName string,
	phase string,
) resources.AIGatewayConsumerCredentialResource {
	t.Helper()
	payload := map[string]any{
		"ref":                 name,
		"ai_gateway_consumer": "support-user",
		"name":                name,
		"type":                "api-key",
		"display_name":        displayName,
		"labels": map[string]string{
			"phase": phase,
		},
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var credential resources.AIGatewayConsumerCredentialResource
	require.NoError(t, json.Unmarshal(data, &credential))
	credential.SetDefaults()
	return credential
}

func testAIGatewayConsumer(policies []string) kkComps.AIGatewayConsumer {
	return kkComps.AIGatewayConsumer{
		ID:          "consumer-id",
		Name:        "support-user",
		Type:        kkComps.AIGatewayConsumerTypeAPIKey,
		DisplayName: "Support User",
		Policies:    policies,
	}
}

func testAIGatewayConsumerCredential(
	id string,
	name string,
	displayName string,
	phase string,
) kkComps.AIGatewayConsumerCredential {
	ttl := int64(0)
	return kkComps.AIGatewayConsumerCredential{
		ID:          id,
		Name:        name,
		Type:        kkComps.AIGatewayConsumerCredentialTypeAPIKey,
		DisplayName: displayName,
		Labels:      map[string]string{"phase": phase},
		TTL:         &ttl,
	}
}

type testAIGatewayConsumerAPI struct {
	consumers   []kkComps.AIGatewayConsumer
	credentials []kkComps.AIGatewayConsumerCredential
}

func (t *testAIGatewayConsumerAPI) ListAiGatewayConsumers(
	context.Context,
	kkOps.ListAiGatewayConsumersRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayConsumersResponse, error) {
	return &kkOps.ListAiGatewayConsumersResponse{
		ListAIGatewayConsumersResponse: &kkComps.ListAIGatewayConsumersResponse{
			Data: t.consumers,
		},
	}, nil
}

func (t *testAIGatewayConsumerAPI) CreateAiGatewayConsumer(
	context.Context,
	string,
	kkComps.CreateAIGatewayConsumerRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayConsumerResponse, error) {
	return nil, nil
}

func (t *testAIGatewayConsumerAPI) GetAiGatewayConsumer(
	_ context.Context,
	_ string,
	consumerID string,
	_ ...kkOps.Option,
) (*kkOps.GetAiGatewayConsumerResponse, error) {
	for _, consumer := range t.consumers {
		if consumer.ID == consumerID || consumer.Name == consumerID {
			return &kkOps.GetAiGatewayConsumerResponse{AIGatewayConsumer: &consumer}, nil
		}
	}
	return &kkOps.GetAiGatewayConsumerResponse{}, nil
}

func (t *testAIGatewayConsumerAPI) UpdateAiGatewayConsumer(
	context.Context,
	kkOps.UpdateAiGatewayConsumerRequest,
	...kkOps.Option,
) (*kkOps.UpdateAiGatewayConsumerResponse, error) {
	return nil, nil
}

func (t *testAIGatewayConsumerAPI) DeleteAiGatewayConsumer(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayConsumerResponse, error) {
	return nil, nil
}

func (t *testAIGatewayConsumerAPI) ListAiGatewayConsumerCredentials(
	context.Context,
	kkOps.ListAiGatewayConsumerCredentialsRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayConsumerCredentialsResponse, error) {
	return &kkOps.ListAiGatewayConsumerCredentialsResponse{
		ListAIGatewayConsumerCredentialsResponse: &kkComps.ListAIGatewayConsumerCredentialsResponse{
			Data: t.credentials,
		},
	}, nil
}

func (t *testAIGatewayConsumerAPI) CreateAiGatewayConsumerCredential(
	context.Context,
	kkOps.CreateAiGatewayConsumerCredentialRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayConsumerCredentialResponse, error) {
	return nil, nil
}

func (t *testAIGatewayConsumerAPI) GetAiGatewayConsumerCredential(
	_ context.Context,
	request kkOps.GetAiGatewayConsumerCredentialRequest,
	_ ...kkOps.Option,
) (*kkOps.GetAiGatewayConsumerCredentialResponse, error) {
	for _, credential := range t.credentials {
		if credential.ID == request.CredentialID || credential.Name == request.CredentialID {
			return &kkOps.GetAiGatewayConsumerCredentialResponse{AIGatewayConsumerCredential: &credential}, nil
		}
	}
	return &kkOps.GetAiGatewayConsumerCredentialResponse{}, nil
}

func (t *testAIGatewayConsumerAPI) DeleteAiGatewayConsumerCredential(
	context.Context,
	kkOps.DeleteAiGatewayConsumerCredentialRequest,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayConsumerCredentialResponse, error) {
	return nil, nil
}
