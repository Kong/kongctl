package resources

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/values"
	"github.com/stretchr/testify/require"
)

func TestPayloadReferenceSelectorsAndTypes(t *testing.T) {
	portal := PortalResource{BaseResource: BaseResource{Ref: "portal"}}
	portal.Name = "Portal"
	portal.DisplayName = new("Display")
	rs := &ResourceSet{Portals: []PortalResource{portal}}
	for _, selector := range []string{"name", "Name"} {
		value, err := rs.ResolvePayloadReference("__REF__:portal#" + selector)
		require.NoError(t, err)
		require.Equal(t, "Portal", value)
	}
	_, err := rs.ResolvePayloadReference("__REF__:portal#unknown")
	require.Error(t, err)
	// A boolean selector retains its type in an arbitrary value slot.
	policy := AIGatewayPolicyResource{BaseResource: BaseResource{Ref: "policy"}}
	policy.Enabled = new(true)
	rs.AIGatewayPolicies = []AIGatewayPolicyResource{policy}
	payload := map[string]any{"boolean": "__REF__:policy#enabled"}
	err = values.Transform(payload, func(_ string, value string) (any, error) {
		return rs.ResolvePayloadReference(value)
	})
	require.NoError(t, err)
	require.Equal(t, true, payload["boolean"])
	text := map[string]string{"boolean": "__REF__:policy#enabled"}
	err = values.Transform(text, func(_ string, value string) (any, error) {
		return rs.ResolvePayloadReference(value)
	})
	require.ErrorContains(t, err, "incompatible with string destination")
}

func TestPayloadReferenceAmbiguousAndSecretTargets(t *testing.T) {
	rs := &ResourceSet{
		APIs:    []APIResource{{BaseResource: BaseResource{Ref: "same"}}},
		Portals: []PortalResource{{BaseResource: BaseResource{Ref: "same"}}},
	}
	_, err := rs.ResolvePayloadReference("__REF__:same#id")
	require.ErrorContains(t, err, "ambiguous")
	rs.Portals = nil
	rs.APIs[0].Description = new("__SECRET__:do-not-disclose")
	_, err = rs.ResolvePayloadReference("__REF__:same#description")
	require.ErrorContains(t, err, "write-only secret")
	require.NotContains(t, err.Error(), "do-not-disclose")
}
