package planner

import (
	"testing"

	"github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestShouldUpdateVirtualClusterDetectsTopicAliasChanges(t *testing.T) {
	currentAliases := virtualClusterTopicAliases("tenant-a.orders")
	desiredAliases := virtualClusterTopicAliases("tenant-b.orders")

	needsUpdate, updates, changed := (&Planner{}).shouldUpdateVirtualCluster(
		virtualClusterState(currentAliases),
		virtualClusterResource(desiredAliases),
	)

	require.True(t, needsUpdate)
	require.Equal(t, currentAliases, changed[FieldTopicAliases].Old)
	require.Equal(t, desiredAliases, changed[FieldTopicAliases].New)
	require.Equal(t, desiredAliases, updates[FieldTopicAliases])
}

func TestShouldUpdateVirtualClusterTreatsNilAndEmptyTopicAliasesAsEqual(t *testing.T) {
	emptyAliases := []components.VirtualClusterTopicAlias{}

	needsUpdate, updates, changed := (&Planner{}).shouldUpdateVirtualCluster(
		virtualClusterState(nil),
		virtualClusterResource(emptyAliases),
	)

	require.False(t, needsUpdate)
	require.Empty(t, updates)
	require.Empty(t, changed)
}

func TestShouldUpdateVirtualClusterPlansEmptyTopicAliasesWhenCurrentHasAliases(t *testing.T) {
	currentAliases := virtualClusterTopicAliases("tenant-a.orders")
	desiredAliases := []components.VirtualClusterTopicAlias{}

	needsUpdate, updates, changed := (&Planner{}).shouldUpdateVirtualCluster(
		virtualClusterState(currentAliases),
		virtualClusterResource(desiredAliases),
	)

	require.True(t, needsUpdate)
	require.Equal(t, currentAliases, changed[FieldTopicAliases].Old)
	require.Equal(t, desiredAliases, changed[FieldTopicAliases].New)
	require.Equal(t, desiredAliases, updates[FieldTopicAliases])
}

func TestShouldUpdateVirtualClusterTreatsTopicAliasesAsOrderIndependent(t *testing.T) {
	currentAliases := []components.VirtualClusterTopicAlias{
		{
			Alias: "public-payments",
			Topic: "tenant-a.payments",
		},
		{
			Alias: "public-orders",
			Topic: "tenant-a.orders",
		},
	}
	desiredAliases := []components.VirtualClusterTopicAlias{
		{
			Alias: "public-orders",
			Topic: "tenant-a.orders",
		},
		{
			Alias: "public-payments",
			Topic: "tenant-a.payments",
		},
	}

	needsUpdate, updates, changed := (&Planner{}).shouldUpdateVirtualCluster(
		virtualClusterState(currentAliases),
		virtualClusterResource(desiredAliases),
	)

	require.False(t, needsUpdate)
	require.Empty(t, updates)
	require.Empty(t, changed)
}

func TestShouldUpdateVirtualClusterTreatsTopicAliasAPIDefaultsAsEqual(t *testing.T) {
	emptyCondition := ""
	defaultConflict := components.VirtualClusterTopicAliasConflictWarn
	currentAliases := []components.VirtualClusterTopicAlias{{
		Alias:     "public-orders",
		Topic:     "tenant-a.orders",
		Condition: &emptyCondition,
		Conflict:  &defaultConflict,
	}}
	desiredAliases := []components.VirtualClusterTopicAlias{{
		Alias: "public-orders",
		Topic: "tenant-a.orders",
	}}

	needsUpdate, updates, changed := (&Planner{}).shouldUpdateVirtualCluster(
		virtualClusterState(currentAliases),
		virtualClusterResource(desiredAliases),
	)

	require.False(t, needsUpdate)
	require.Empty(t, updates)
	require.Empty(t, changed)
}

func TestShouldUpdateVirtualClusterPreservesOmittedTopicAliasesOnUnrelatedUpdate(t *testing.T) {
	currentAliases := virtualClusterTopicAliases("tenant-a.orders")
	desired := virtualClusterResource(nil)
	newDescription := "new description"
	desired.Description = &newDescription

	needsUpdate, updates, changed := (&Planner{}).shouldUpdateVirtualCluster(
		virtualClusterState(currentAliases),
		desired,
	)

	require.True(t, needsUpdate)
	require.Contains(t, changed, FieldDescription)
	require.NotContains(t, changed, FieldTopicAliases)
	require.Equal(t, currentAliases, updates[FieldTopicAliases])
}

func virtualClusterState(aliases []components.VirtualClusterTopicAlias) state.EventGatewayVirtualCluster {
	description := "description"

	return state.EventGatewayVirtualCluster{
		VirtualCluster: components.VirtualCluster{
			ID:          "virtual-cluster-id",
			Name:        "virtual-cluster",
			Description: &description,
			Destination: components.BackendClusterReference{
				ID:   "backend-cluster-id",
				Name: "backend-cluster",
			},
			Authentication: []components.VirtualClusterAuthenticationSensitiveDataAwareScheme{
				components.CreateVirtualClusterAuthenticationSensitiveDataAwareSchemeAnonymous(
					components.VirtualClusterAuthenticationAnonymous{},
				),
			},
			TopicAliases: aliases,
			ACLMode:      components.VirtualClusterACLModePassthrough,
			DNSLabel:     "vc-default",
		},
	}
}

func virtualClusterResource(
	aliases []components.VirtualClusterTopicAlias,
) resources.EventGatewayVirtualClusterResource {
	description := "description"

	return resources.EventGatewayVirtualClusterResource{
		CreateVirtualClusterRequest: components.CreateVirtualClusterRequest{
			Name:        "virtual-cluster",
			Description: &description,
			Destination: components.CreateBackendClusterReferenceModifyBackendClusterReferenceByID(
				components.BackendClusterReferenceByID{ID: "backend-cluster-id"},
			),
			Authentication: []components.VirtualClusterAuthenticationScheme{
				components.CreateVirtualClusterAuthenticationSchemeAnonymous(
					components.VirtualClusterAuthenticationAnonymous{},
				),
			},
			TopicAliases: aliases,
			ACLMode:      components.VirtualClusterACLModePassthrough,
			DNSLabel:     "vc-default",
		},
		Ref: "virtual-cluster-ref",
	}
}

func virtualClusterTopicAliases(topic string) []components.VirtualClusterTopicAlias {
	condition := "context.auth.type == 'anonymous'"
	conflict := components.VirtualClusterTopicAliasConflictWarn

	return []components.VirtualClusterTopicAlias{{
		Alias:     "public-orders",
		Topic:     topic,
		Condition: &condition,
		Conflict:  &conflict,
	}}
}
