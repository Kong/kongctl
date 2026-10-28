package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChildOwnershipMetadataIsCanonical(t *testing.T) {
	metadata := BuildChildManagedBy("team-a", true, "1.2.3")
	require.Equal(t, map[string]string{
		ChildNamespaceKey: "team-a", ChildProtectedKey: "true",
		ManagedByToolKey: "kongctl", ManagedByVersionKey: "1.2.3",
	}, metadata)
	require.NoError(t, ValidateChildManagedBy(metadata, false))
	for _, key := range []string{ChildNamespaceKey, ChildProtectedKey, ManagedByToolKey, ManagedByVersionKey} {
		require.ErrorContains(t, ValidateChildManagedBy(map[string]string{key: "raw"}, true), "reserved")
	}
	require.ErrorContains(t, ValidateChildManagedBy(map[string]string{
		"a": "1", "b": "2", "c": "3", "d": "4", "e": "5", "f": "6",
	}, false), "5-entry")
}

func TestOwnedChildScopeMergeAndCanonicalRebind(t *testing.T) {
	first, second := NewSyncScope(), NewSyncScope()
	first.AddChildNamespace(ResourceTypeAIGatewayProvider, "alias-a", "team-a")
	second.AddChildNamespace(ResourceTypeAIGatewayProvider, "alias-b", "team-b")
	first.Merge(second)
	first.RebindChildParent(ResourceTypeAIGateway, "alias-a", "gateway-id")
	first.RebindChildParent(ResourceTypeAIGateway, "alias-b", "gateway-id")
	require.Len(t, first.OwnedChildNamespaces(ResourceTypeAIGatewayProvider, "gateway-id"), 2)
	require.Nil(t, first.OwnedChildNamespaces(ResourceTypeAIGatewayProvider, "omitted"))
	require.Empty(t, first.GatewayNamespaces)
	require.False(t, first.RootInScope(ResourceTypeAIGateway))
}

func TestSharedInfrastructureDoesNotEnrollUnverifiedFamilies(t *testing.T) {
	for _, kind := range []ResourceType{
		ResourceTypeAIGatewayMCPServer,
		ResourceTypeAIGatewayVault,
		ResourceTypeAIGatewayConsumer,
		ResourceTypeAIGatewayConsumerGroup,
		ResourceTypeAIGatewayCACertificate,
		ResourceTypeAIGatewaySNI,
	} {
		require.True(t, HasIndependentChildOwnership(kind))
	}
	require.True(t, HasIndependentChildOwnership(ResourceTypeAIGatewayAgent))
	require.True(t, HasIndependentChildOwnership(ResourceTypeAIGatewayModel))
	require.True(t, HasIndependentChildOwnership(ResourceTypeAIGatewayProvider))
	require.True(t, HasIndependentChildOwnership(ResourceTypeAIGatewayAuthStrategy))
	require.True(t, HasIndependentChildOwnership(ResourceTypeAIGatewayPolicy))
	require.True(t, HasIndependentChildOwnership(ResourceTypeAIGatewayCustomPolicy))
	for _, kind := range []ResourceType{
		ResourceTypeAIGatewayCertificate,
		ResourceTypeAIGatewayConfigStore, ResourceTypeAIGatewayDataPlaneCertificate,
	} {
		require.False(t, HasIndependentChildOwnership(kind))
	}
}
