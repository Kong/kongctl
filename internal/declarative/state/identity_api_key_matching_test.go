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

func TestGeneratedKeyMatchingRejectsAmbiguousAndLegacyIdentity(t *testing.T) {
	primary := generatedKeyFixture("first", "primary")
	foreign := generatedKeyFixture("foreign", "primary")
	foreign.ManagedBy[resources.ManagedByToolKey] = "other"
	unmanaged := IdentityAPIKey{helpers.IdentityAPIKey{ID: "unmanaged"}}
	values := []IdentityAPIKey{unmanaged, foreign, primary}
	matched, err := FindIdentityAPIKey(values, "primary")
	require.NoError(t, err)
	require.Equal(t, primary.ID, matched.ID)
	indexed, err := IndexIdentityAPIKeys(values)
	require.NoError(t, err)
	require.Len(t, indexed, 1)

	faults := []string{"duplicate", "legacy", "namespace", "protection", "no-version", "bad-ref", "extra-marker"}
	for _, fault := range faults {
		t.Run(fault, func(t *testing.T) {
			extra := generatedKeyFixture("second", "primary")
			switch fault {
			case "legacy":
				delete(extra.ManagedBy, resources.IdentityAPIKeyCreateTypeKey)
			case "namespace":
				extra.ManagedBy[resources.ChildNamespaceKey] = "independent"
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
	legacy := generatedKeyFixture("legacy", "other")
	delete(legacy.ManagedBy, resources.IdentityAPIKeyCreateTypeKey)
	indexed, err = IndexIdentityAPIKeys(append(values, legacy))
	require.NoError(t, err)
	require.Len(t, indexed, 1, "unknown/legacy keys are outside pruning authority")
	before := maps.Clone(primary.ManagedBy)
	_, err = FindIdentityAPIKey(values, "missing")
	require.NoError(t, err)
	require.Equal(t, before, primary.ManagedBy)
}
