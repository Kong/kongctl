package resources

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/stretchr/testify/require"
)

func TestManagedByWriteCapabilityIsIndependentOfLabelOwnership(t *testing.T) {
	// Simulate a later resource migration without enrolling Portal in production.
	original := registry[ResourceTypePortal]
	t.Cleanup(func() { registry[ResourceTypePortal] = original })
	require.False(t, SupportsManagedBy(ResourceTypePortal))
	metadata := map[string]string{"external": "keep"}
	unchanged, err := PrepareManagedBy(ResourceTypePortal, metadata, "1.2.3")
	require.NoError(t, err)
	require.Equal(t, metadata, unchanged)

	ops := original
	require.NoError(t, WithManagedBy(ManagedByPolicy{MaxEntries: 7})(&ops))
	registry[ResourceTypePortal] = ops
	require.True(t, SupportsManagedBy(ResourceTypePortal))
	require.Same(t, original.namespace, ops.namespace)
	require.Equal(t, original.namespaceOwner, ops.namespaceOwner)
	require.False(t, HasIndependentChildOwnership(ResourceTypePortal))

	userLabels := map[string]string{"user": "keep"}
	before := labels.BuildCreateLabels(userLabels, "team-a", true)
	prepared, err := PrepareManagedBy(ResourceTypePortal, metadata, "1.2.3")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"tool": "kongctl", "version": "1.2.3"}, prepared)
	require.Equal(t, map[string]string{"external": "keep"}, metadata)
	require.NotContains(t, prepared, ChildNamespaceKey)
	require.NotContains(t, prepared, ChildProtectedKey)
	require.Equal(t, before, labels.BuildCreateLabels(userLabels, "team-a", true))

	full := map[string]string{"a": "1", "b": "2", "c": "3", "d": "4", "e": "5"}
	_, err = PrepareManagedBy(ResourceTypePortal, full, "1.2.3")
	require.NoError(t, err, "the registered API limit is seven, not the AI Gateway limit")
	_, err = PrepareManagedBy(ResourceTypeAIGatewayProvider, full, "1.2.3")
	require.NoError(t, err, "foreign entries are replaced by the canonical map")
	require.Len(t, full, 5, "capacity errors cannot discard unrelated entries")
}

func TestManagedByRegistrationRejectsInvalidOrDuplicateCapabilities(t *testing.T) {
	for _, limit := range []int{-1, 0, 1} {
		var ops resourceOps
		require.ErrorContains(t, WithManagedBy(ManagedByPolicy{MaxEntries: limit})(&ops), "tool and version")
	}
	var ops resourceOps
	require.NoError(t, WithManagedBy(ManagedByPolicy{MaxEntries: 5})(&ops))
	require.ErrorContains(t, WithManagedBy(ManagedByPolicy{MaxEntries: 7})(&ops), "already registered")
}

func TestManagedByDirectoryRootOwnsLifecycleWithoutChildScope(t *testing.T) {
	require.True(t, UsesManagedByOwnership(ResourceTypeIdentityDirectory))
	require.False(t, HasIndependentChildOwnership(ResourceTypeIdentityDirectory))
	metadata := BuildChildManagedBy("identity-team", true, "old")
	metadata["outside"] = "discarded"
	prepared, err := PrepareManagedBy(ResourceTypeIdentityDirectory, metadata, "current")
	require.NoError(t, err)
	require.Equal(t, BuildChildManagedBy("identity-team", true, "current"), prepared)
	var ops resourceOps
	require.ErrorContains(t, WithManagedBy(ManagedByPolicy{MaxEntries: 5, OwnsLifecycle: true})(&ops),
		"namespace participation")
}
