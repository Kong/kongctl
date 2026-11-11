package state

import (
	"maps"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func generatedKeyFixture(id, ref string) IdentityAPIKey {
	return IdentityAPIKey{helpers.IdentityAPIKey{ID: id, ManagedBy: map[string]string{
		resources.ManagedByToolKey:            resources.ManagedByTool,
		resources.ManagedByVersionKey:         "creator-version",
		resources.ManagedByRefKey:             ref,
		resources.IdentityAPIKeyCreateTypeKey: resources.IdentityAPIKeyGeneratedType,
	}}}
}

func TestAPIKeyMatchingRejectsAmbiguousAndLegacyIdentity(t *testing.T) {
	for _, creationType := range []string{resources.IdentityAPIKeyGeneratedType, resources.IdentityAPIKeyImportedType} {
		t.Run(creationType, func(t *testing.T) { testAPIKeyMatching(t, creationType) })
	}
}

func testAPIKeyMatching(t *testing.T, creationType string) {
	keyFixture := func(id, ref string) IdentityAPIKey {
		key := generatedKeyFixture(id, ref)
		key.ManagedBy[resources.IdentityAPIKeyCreateTypeKey] = creationType
		return key
	}
	primary := keyFixture("first", "primary")
	foreign := keyFixture("foreign", "primary")
	foreign.ManagedBy[resources.ManagedByToolKey] = "other"
	unmanaged := IdentityAPIKey{helpers.IdentityAPIKey{ID: "unmanaged"}}
	values := []IdentityAPIKey{unmanaged, foreign, primary}
	matched, err := FindIdentityAPIKey(values, "primary")
	require.NoError(t, err)
	require.Equal(t, primary.ID, matched.ID)
	indexed, err := IndexIdentityAPIKeys(values)
	require.NoError(t, err)
	require.Len(t, indexed, 1)

	faults := []string{"duplicate", "legacy", "protection", "no-version", "bad-ref", "extra-marker"}
	for _, fault := range faults {
		t.Run(fault, func(t *testing.T) {
			extra := keyFixture("second", "primary")
			switch fault {
			case "legacy":
				delete(extra.ManagedBy, resources.IdentityAPIKeyCreateTypeKey)
			case "protection":
				extra.ManagedBy[resources.ChildProtectedKey] = "false"
			case "no-version":
				delete(extra.ManagedBy, resources.ManagedByVersionKey)
			case "bad-ref":
				extra.ManagedBy[resources.ManagedByRefKey] = "invalid ref"
			case "extra-marker":
				extra.ManagedBy["unexpected"] = "value"
			}
			_, err := FindIdentityAPIKey(append(values, extra), "primary")
			require.Error(t, err)
		})
	}
	legacy := keyFixture("legacy", "other")
	delete(legacy.ManagedBy, resources.IdentityAPIKeyCreateTypeKey)
	indexed, err = IndexIdentityAPIKeys(append(values, legacy))
	require.NoError(t, err)
	require.Len(t, indexed, 1, "unknown/legacy keys are outside pruning authority")
	before := maps.Clone(primary.ManagedBy)
	_, err = FindIdentityAPIKey(values, "missing")
	require.NoError(t, err)
	require.Equal(t, before, primary.ManagedBy)
}

func TestAPIKeysUnderExternalPrincipalsMatchNamespaceAndRef(t *testing.T) {
	for _, creationType := range []string{resources.IdentityAPIKeyGeneratedType, resources.IdentityAPIKeyImportedType} {
		t.Run(creationType, func(t *testing.T) { testExternalAPIKeyMatching(t, creationType) })
	}
}

func testExternalAPIKeyMatching(t *testing.T, creationType string) {
	keyFixture := func(id string) IdentityAPIKey {
		key := generatedKeyFixture(id, "primary")
		key.ManagedBy[resources.IdentityAPIKeyCreateTypeKey] = creationType
		return key
	}
	first := keyFixture("first")
	first.ManagedBy[resources.ChildNamespaceKey] = "team-a"
	other := keyFixture("other")
	other.ManagedBy[resources.ChildNamespaceKey] = "team-b"
	legacy := keyFixture("inherited")
	values := []IdentityAPIKey{first, other, legacy}
	for _, namespace := range []string{"team-a", "team-b"} {
		value, err := FindIdentityAPIKey(values, "primary", namespace)
		require.NoError(t, err)
		require.Equal(t, namespace, value.ManagedBy[resources.ChildNamespaceKey])
	}
	value, err := FindIdentityAPIKey(values, "primary")
	require.NoError(t, err)
	require.Equal(t, legacy.ID, value.ID)
	duplicate := keyFixture("duplicate")
	duplicate.ManagedBy[resources.ChildNamespaceKey] = "team-a"
	_, err = FindIdentityAPIKey(append(values, duplicate), "primary", "team-a")
	require.ErrorContains(t, err, "multiple API keys")
	_, err = FindIdentityAPIKey(append(values, duplicate), "primary", "team-b")
	require.NoError(t, err, "another namespace's duplicates must not widen authority")
}
