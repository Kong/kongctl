package planner

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

const (
	childCachedID  = "11111111-1111-4111-8111-111111111111"
	childRefID     = "22222222-2222-4222-8222-222222222222"
	childNameID    = "33333333-3333-4333-8333-333333333333"
	childMissingID = "44444444-4444-4444-8444-444444444444"
)

type remainingChildIdentityCase struct {
	name, ref, cachedID, desiredName string
	existing                         bool
}

func remainingChildIdentityCases() []remainingChildIdentityCase {
	return []remainingChildIdentityCase{
		{"name wins over UUID ref", childRefID, "", "desired-name", true},
		{"name wins over cached ID", "local-ref", childCachedID, "desired-name", true},
		{"missing IDs do not hide name", childMissingID, childMissingID, "desired-name", true},
		{"UUID ref does not rename or retain", childRefID, "", "new-name", false},
		{"cached ID does not rename or retain", "local-ref", childCachedID, "new-name", false},
	}
}

func checkRemainingChildPruning(t *testing.T, changes []PlannedChange, mode PlanMode, existing bool) {
	t.Helper()
	var ids []string
	if mode == PlanModeSync {
		ids = []string{childCachedID, childRefID}
		if !existing {
			ids = append(ids, childNameID)
		}
	}
	require.Len(t, changes, len(ids))
	for i, id := range ids {
		require.Equal(t, ActionDelete, changes[i].Action)
		require.Equal(t, id, changes[i].ResourceID)
	}
}

func TestAIGatewayConfigStoreNameIdentity(t *testing.T) {
	for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
		for _, tt := range remainingChildIdentityCases() {
			t.Run(string(mode)+"/"+tt.name, func(t *testing.T) {
				api := &storeNameIdentityAPI{testAIGatewayConfigStoreAPI: &testAIGatewayConfigStoreAPI{
					stores: []kkComps.AIGatewayConfigStore{
						{ID: childCachedID, Name: "cached-name"},
						{ID: childRefID, Name: "ref-name"},
						{ID: childNameID, Name: "desired-name"},
					},
				}}
				desired := resources.AIGatewayConfigStoreResource{
					BaseResource: resources.BaseResource{Ref: tt.ref}, Name: tt.desiredName,
					AIGateway: "support-gateway", DisplayName: new("Updated store"),
				}
				desired.SetKonnectID(tt.cachedID)
				rs := testAIGatewayConfigStoreResourceSet(desired)
				rs.AIGatewayConfigStoreSecrets = []resources.AIGatewayConfigStoreSecretResource{{
					BaseResource:         resources.BaseResource{Ref: "support-openai-header"},
					AIGatewayConfigStore: tt.ref, Key: "openai-auth-header", Value: testSecretPlaceholder(t),
				}}
				addTestConfigStoreSecretSource(rs)
				rs.EnsureSyncScope().AddChild(resources.ResourceTypeAIGatewayConfigStore,
					tt.ref, resources.ResourceTypeAIGatewayConfigStoreSecret)
				p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayConfigStoresAPI: api}), slog.Default())
				p.resources = rs
				plan := NewPlan(CurrentPlanVersion, "test", mode)
				require.NoError(t, p.planAIGatewayConfigStoreChanges(t.Context(),
					"test-namespace", "support-gateway", "Gateway", "gateway-id", "", rs.AIGatewayConfigStores, plan))
				require.Equal(t, []string{"gateway-id"}, api.lists)
				require.GreaterOrEqual(t, len(plan.Changes), 2)
				store, secret := plan.Changes[0], plan.Changes[1]
				require.Equal(t, ResourceTypeAIGatewayConfigStore, store.ResourceType)
				require.Equal(t, tt.ref, store.ResourceRef)
				require.Equal(t, "Updated store", store.Fields[FieldDisplayName])
				require.Equal(t, ResourceTypeAIGatewayConfigStoreSecret, secret.ResourceType)
				require.Equal(t, ActionCreate, secret.Action)
				require.Equal(t, "openai-auth-header", secret.Fields[FieldKey])
				require.Equal(t, "gateway-id", secret.References[FieldAIGatewayID].ID)
				require.Equal(t, "test-namespace", secret.Namespace)
				if tt.existing {
					require.Equal(t, ActionUpdate, store.Action)
					require.Equal(t, childNameID, store.ResourceID)
					require.Equal(t, map[string]any{FieldDisplayName: "Updated store"}, store.Fields)
					require.Equal(t, childNameID, rs.AIGatewayConfigStores[0].GetKonnectID())
					require.Equal(t, [][2]string{{"gateway-id", childNameID}}, api.secretReads)
					require.Equal(t, &ParentInfo{Ref: tt.ref, ID: childNameID}, secret.Parent)
					require.Empty(t, secret.DependsOn)
				} else {
					require.Equal(t, ActionCreate, store.Action)
					require.Equal(t, tt.desiredName, store.Fields[FieldName])
					require.Empty(t, store.ResourceID)
					require.Empty(t, api.secretReads)
					require.Nil(t, secret.Parent)
					require.Equal(t, tt.ref, secret.References[FieldConfigStoreID].Ref)
					require.Equal(t, []string{store.ID}, secret.DependsOn)
				}
				checkRemainingChildPruning(t, plan.Changes[2:], mode, tt.existing)
				for i, change := range plan.Changes {
					if i != 1 {
						require.Equal(t, &ParentInfo{Ref: "support-gateway", ID: "gateway-id"}, change.Parent)
						require.Equal(t, "test-namespace", change.Namespace)
					}
				}
			})
		}
	}
}

func TestAIGatewayConsumerGroupNameIdentity(t *testing.T) {
	for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
		for _, tt := range remainingChildIdentityCases() {
			t.Run(string(mode)+"/"+tt.name, func(t *testing.T) {
				api := &groupNameIdentityAPI{testAIGatewayConsumerGroupAPI: &testAIGatewayConsumerGroupAPI{}}
				for _, identity := range []struct{ id, name string }{
					{childCachedID, "cached-name"}, {childRefID, "ref-name"}, {childNameID, "desired-name"},
				} {
					group := testAIGatewayConsumerGroup(nil)
					group.ID, group.Name = identity.id, identity.name
					api.groups = append(api.groups, group)
				}
				desired := testAIGatewayConsumerGroupResourceWithConsumers(t, []string{"support-user"})
				desired.Ref, desired.Name = tt.ref, tt.desiredName
				desired.DisplayName = "Updated group"
				desired.SetKonnectID(tt.cachedID)
				rs := testAIGatewayConsumerGroupResourceSet(desired)
				p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayConsumerGroupsAPI: api}), slog.Default())
				p.resources = rs
				plan := NewPlan(CurrentPlanVersion, "test", mode)
				require.NoError(t, p.planAIGatewayConsumerGroupChanges(t.Context(),
					"test-namespace", "support-gateway", "Gateway", "gateway-id", "", nil,
					rs.AIGatewayConsumerGroups, plan))
				require.Equal(t, []string{"gateway-id"}, api.lists)
				require.NotEmpty(t, plan.Changes)
				change := plan.Changes[0]
				require.Equal(t, tt.ref, change.ResourceRef)
				require.Equal(t, tt.desiredName, change.Fields[FieldName])
				require.Equal(t, "Updated group", change.Fields[FieldDisplayName])
				if tt.existing {
					require.Equal(t, ActionUpdate, change.Action)
					require.Equal(t, childNameID, change.ResourceID)
					require.Equal(t, childNameID, rs.AIGatewayConsumerGroups[0].GetKonnectID())
					require.Equal(t, [][2]string{{"gateway-id", childNameID}}, api.reads)
					require.Equal(t, api.reads, api.membershipReads)
					require.Equal(t, []string{"support-user"}, change.Fields[FieldConsumers])
					require.Contains(t, change.ChangedFields, FieldConsumers)
				} else {
					require.Equal(t, ActionCreate, change.Action)
					require.Empty(t, change.ResourceID)
					require.Empty(t, api.reads)
					require.Empty(t, api.membershipReads)
				}
				checkRemainingChildPruning(t, plan.Changes[1:], mode, tt.existing)
				for _, change := range plan.Changes {
					require.Equal(t, ResourceTypeAIGatewayConsumerGroup, change.ResourceType)
					require.Equal(t, "test-namespace", change.Namespace)
					require.Equal(t, &ParentInfo{Ref: "support-gateway", ID: "gateway-id"}, change.Parent)
				}
			})
		}
	}
}

func TestAIGatewayConsumerCredentialNameIdentity(t *testing.T) {
	for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
		for _, tt := range remainingChildIdentityCases() {
			t.Run(string(mode)+"/"+tt.name, func(t *testing.T) {
				api := &credentialNameIdentityAPI{testAIGatewayConsumerAPI: &testAIGatewayConsumerAPI{}}
				for _, identity := range []struct{ id, name string }{
					{childCachedID, "cached-name"}, {childRefID, "ref-name"}, {childNameID, "desired-name"},
				} {
					api.credentials = append(api.credentials,
						testAIGatewayConsumerCredential(identity.id, identity.name, "Original credential", "create"))
				}
				desired := testAIGatewayConsumerCredentialResource(t, tt.desiredName, "Updated credential", "create")
				desired.Ref = tt.ref
				desired.SetKonnectID(tt.cachedID)
				rs := &resources.ResourceSet{
					AIGatewayConsumerCredentials: []resources.AIGatewayConsumerCredentialResource{desired},
				}
				p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayConsumersAPI: api}), slog.Default())
				p.resources = rs
				plan := NewPlan(CurrentPlanVersion, "test", mode)
				require.NoError(t, p.planAIGatewayConsumerCredentialChanges(t.Context(),
					"test-namespace", "support-gateway", "gateway-id", "support-user", "User", "consumer-id",
					rs.AIGatewayConsumerCredentials, plan))
				require.Equal(t, [][2]string{{"gateway-id", "consumer-id"}}, api.lists)
				offset := 0
				if tt.existing {
					require.GreaterOrEqual(t, len(plan.Changes), 2)
					deletion := plan.Changes[0]
					require.Equal(t, ActionDelete, deletion.Action)
					require.Equal(t, childNameID, deletion.ResourceID)
					require.Equal(t, tt.ref, deletion.ResourceRef)
					require.Contains(t, deletion.ChangedFields, FieldDisplayName)
					require.Equal(t, []string{deletion.ID}, plan.Changes[1].DependsOn)
					require.Equal(t, childNameID, rs.AIGatewayConsumerCredentials[0].GetKonnectID())
					offset = 1
				}
				require.Greater(t, len(plan.Changes), offset)
				creation := plan.Changes[offset]
				require.Equal(t, ActionCreate, creation.Action)
				require.Equal(t, tt.ref, creation.ResourceRef)
				require.Equal(t, tt.desiredName, creation.Fields[FieldName])
				require.Equal(t, "Updated credential", creation.Fields[FieldDisplayName])
				require.Empty(t, creation.ResourceID)
				if !tt.existing {
					require.Empty(t, creation.DependsOn)
				}
				checkRemainingChildPruning(t, plan.Changes[offset+1:], mode, tt.existing)
				for _, change := range plan.Changes {
					require.Equal(t, ResourceTypeAIGatewayConsumerCredential, change.ResourceType)
					require.Equal(t, "test-namespace", change.Namespace)
					require.Equal(t, &ParentInfo{Ref: "support-user", ID: "consumer-id"}, change.Parent)
					require.Equal(t, "gateway-id", change.References[FieldAIGatewayID].ID)
				}
			})
		}
	}
}

func TestAIGatewayRemainingChildResourceNameIdentity(t *testing.T) {
	for _, tt := range remainingChildIdentityCases() {
		t.Run(tt.name, func(t *testing.T) {
			store := resources.AIGatewayConfigStoreResource{
				BaseResource: resources.BaseResource{Ref: tt.ref}, Name: tt.desiredName,
			}
			group := testAIGatewayConsumerGroupResource(t, nil)
			group.Ref, group.Name = tt.ref, tt.desiredName
			credential := testAIGatewayConsumerCredentialResource(t, tt.desiredName, "Credential", "create")
			credential.Ref = tt.ref
			for _, identity := range []struct{ id, name string }{
				{childCachedID, "cached-name"}, {childRefID, "ref-name"}, {childNameID, "desired-name"},
			} {
				store.SetKonnectID(tt.cachedID)
				group.SetKonnectID(tt.cachedID)
				credential.SetKonnectID(tt.cachedID)
				for _, pair := range []struct {
					desired resources.Resource
					current any
				}{
					{&store, kkComps.AIGatewayConfigStore{ID: identity.id, Name: identity.name}},
					{&group, kkComps.AIGatewayConsumerGroup{ID: identity.id, Name: identity.name}},
					{&credential, testAIGatewayConsumerCredential(identity.id, identity.name, "Credential", "create")},
				} {
					t.Run(string(pair.desired.GetType())+"/"+identity.name, func(t *testing.T) {
						match := identity.name == tt.desiredName
						require.Equal(t, match, pair.desired.TryMatchKonnectResource(pair.current),
							"%s observed %s", pair.desired.GetType(), identity.name)
						wantID := tt.cachedID
						if match {
							wantID = identity.id
						}
						require.Equal(t, wantID, pair.desired.GetKonnectID())
					})
				}
			}
		})
	}
}

type storeNameIdentityAPI struct {
	*testAIGatewayConfigStoreAPI
	lists       []string
	secretReads [][2]string
}

func (a *storeNameIdentityAPI) ListAiGatewayConfigStores(
	ctx context.Context, req kkOps.ListAiGatewayConfigStoresRequest, opts ...kkOps.Option,
) (*kkOps.ListAiGatewayConfigStoresResponse, error) {
	a.lists = append(a.lists, req.GatewayID)
	return a.testAIGatewayConfigStoreAPI.ListAiGatewayConfigStores(ctx, req, opts...)
}

func (a *storeNameIdentityAPI) ListAiGatewayConfigStoreSecrets(
	ctx context.Context, req kkOps.ListAiGatewayConfigStoreSecretsRequest, opts ...kkOps.Option,
) (*kkOps.ListAiGatewayConfigStoreSecretsResponse, error) {
	a.secretReads = append(a.secretReads, [2]string{req.GatewayID, req.ConfigStoreIDOrName})
	return a.testAIGatewayConfigStoreAPI.ListAiGatewayConfigStoreSecrets(ctx, req, opts...)
}

type groupNameIdentityAPI struct {
	*testAIGatewayConsumerGroupAPI
	lists                  []string
	reads, membershipReads [][2]string
}

func (a *groupNameIdentityAPI) ListAiGatewayConsumerGroups(
	ctx context.Context, req kkOps.ListAiGatewayConsumerGroupsRequest, opts ...kkOps.Option,
) (*kkOps.ListAiGatewayConsumerGroupsResponse, error) {
	a.lists = append(a.lists, req.GatewayID)
	return a.testAIGatewayConsumerGroupAPI.ListAiGatewayConsumerGroups(ctx, req, opts...)
}

func (a *groupNameIdentityAPI) GetAiGatewayConsumerGroup(
	ctx context.Context, gatewayID, groupID string, opts ...kkOps.Option,
) (*kkOps.GetAiGatewayConsumerGroupResponse, error) {
	a.reads = append(a.reads, [2]string{gatewayID, groupID})
	return a.testAIGatewayConsumerGroupAPI.GetAiGatewayConsumerGroup(ctx, gatewayID, groupID, opts...)
}

func (a *groupNameIdentityAPI) ListAiGatewayConsumersInConsumerGroup(
	ctx context.Context, req kkOps.ListAiGatewayConsumersInConsumerGroupRequest, opts ...kkOps.Option,
) (*kkOps.ListAiGatewayConsumersInConsumerGroupResponse, error) {
	a.membershipReads = append(a.membershipReads, [2]string{req.GatewayID, req.ConsumerGroupID})
	return a.testAIGatewayConsumerGroupAPI.ListAiGatewayConsumersInConsumerGroup(ctx, req, opts...)
}

type credentialNameIdentityAPI struct {
	*testAIGatewayConsumerAPI
	lists [][2]string
}

func (a *credentialNameIdentityAPI) ListAiGatewayConsumerCredentials(
	ctx context.Context, req kkOps.ListAiGatewayConsumerCredentialsRequest, opts ...kkOps.Option,
) (*kkOps.ListAiGatewayConsumerCredentialsResponse, error) {
	a.lists = append(a.lists, [2]string{req.GatewayID, req.ConsumerID})
	return a.testAIGatewayConsumerAPI.ListAiGatewayConsumerCredentials(ctx, req, opts...)
}
