package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChildOwnershipMetadataMergeAndCapacity(t *testing.T) {
	current := map[string]string{ChildNamespaceKey: "team-a", ChildProtectedKey: "false", "other-tool": "keep"}
	merged := BuildChildManagedBy(current, nil, "team-a", true, "1.2.3")
	require.NoError(t, ValidateChildManagedBy(merged, false))
	require.Equal(t, "keep", merged["other-tool"])
	require.Equal(t, "kongctl", merged[ManagedByToolKey])
	require.Equal(t, "1.2.3", merged[ManagedByVersionKey])
	require.Len(t, merged, 5)
	require.NotContains(t, current, ManagedByVersionKey)
	require.Equal(t, "false", current[ChildProtectedKey], "merge cannot mutate observations")
	require.ErrorContains(t,
		ValidateChildManagedBy(BuildChildManagedBy(merged,
			map[string]string{"sixth": "overflow"},
			"team-a",
			true, "1.2.3"),
			false),
		"5-entry")
	for _, key := range []string{ChildNamespaceKey, ChildProtectedKey, ManagedByToolKey, ManagedByVersionKey} {
		require.ErrorContains(t, ValidateChildManagedBy(map[string]string{key: "raw"}, true), "reserved")
	}
	require.ErrorContains(t, ValidateChildOwnership("child", nil, "team-a"), "explicitly adopt")
	require.ErrorContains(t, ValidateChildOwnership("child", current, "team-b"), "ownership transfer")
	require.NoError(t, ValidateChildOwnership("child", current, "team-a"))
	require.Equal(t, "team-a", NormalizedChildOwnership(current)["KONGCTL-namespace"])
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
	require.True(t, HasIndependentChildOwnership(ResourceTypeAIGatewayModel))
	require.True(t, HasIndependentChildOwnership(ResourceTypeAIGatewayProvider))
	require.True(t, HasIndependentChildOwnership(ResourceTypeAIGatewayAuthStrategy))
	require.True(t, HasIndependentChildOwnership(ResourceTypeAIGatewayPolicy))
	for _, kind := range []ResourceType{
		ResourceTypeAIGatewayConsumer, ResourceTypeAIGatewayConsumerGroup,
		ResourceTypeAIGatewayConfigStore, ResourceTypeAIGatewayDataPlaneCertificate,
	} {
		require.False(t, HasIndependentChildOwnership(kind))
	}
}
