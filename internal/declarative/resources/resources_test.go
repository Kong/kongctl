package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetPortalsByNamespaceIncludesExternalsWhenRequested(t *testing.T) {
	team := "team-one"
	rs := &ResourceSet{
		Portals: []PortalResource{
			{
				Ref: "managed",
				Kongctl: &KongctlMeta{
					Namespace: &team,
				},
			},
			{
				Ref:      "external",
				External: &ExternalBlock{Selector: &ExternalSelector{MatchFields: map[string]string{"name": "ext"}}},
			},
		},
		PortalPages: []PortalPageResource{
			{Ref: "page-1", Portal: "managed"},
			{Ref: "page-2", Portal: "external"},
		},
	}

	managed := rs.GetPortalsByNamespace(team)
	require.Len(t, managed, 1)
	require.Equal(t, "managed", managed[0].Ref)

	external := rs.GetPortalsByNamespace(NamespaceExternal)
	require.Len(t, external, 1)
	require.Equal(t, "external", external[0].Ref)

	managedPages := rs.GetPortalPagesByNamespace(team)
	require.Len(t, managedPages, 1)
	require.Equal(t, "page-1", managedPages[0].Ref)

	externalPages := rs.GetPortalPagesByNamespace(NamespaceExternal)
	require.Len(t, externalPages, 1)
	require.Equal(t, "page-2", externalPages[0].Ref)
}
