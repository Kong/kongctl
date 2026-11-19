package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChildOwnershipDistinguishesForeignOwner(t *testing.T) {
	for _, metadata := range []map[string]string{
		nil,
		{},
		{ChildProtectedKey: "true"},
		{ManagedByToolKey: ManagedByTool, ManagedByVersionKey: "1"},
		{ChildNamespaceKey: "", "kongctl-legacy": "value", ManagedByVersionKey: "1"},
	} {
		require.ErrorContains(t, ValidateChildOwnership("policy", metadata, "team-a"), "unowned; explicitly adopt")
	}
	for _, metadata := range []map[string]string{
		{"tool": "cost-attribution"},
		{"product": "cost-attribution", "installation": "present"},
		{ManagedByToolKey: ManagedByTool, "product": "cost-attribution"},
	} {
		err := ValidateChildOwnership("policy", metadata, "team-a")
		require.ErrorContains(t, err, "unrecognized managed_by metadata")
		require.NotContains(t, err.Error(), "explicitly adopt")
		require.NotContains(t, err.Error(), "owned by another tool")
		require.NotContains(t, err.Error(), "cannot adopt")
	}
	require.ErrorContains(t, ValidateChildOwnership("policy", map[string]string{
		"tool": "cost-attribution",
	}, "team-a"), `"cost-attribution"`)
	require.NoError(t, ValidateChildOwnership("policy", BuildChildManagedBy("team-a", false, "test"), "team-a"))
}

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
