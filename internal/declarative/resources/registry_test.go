package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllResources(t *testing.T) {
	t.Run("returns all registered resources", func(t *testing.T) {
		rs := &ResourceSet{
			Portals: []PortalResource{
				{BaseResource: BaseResource{Ref: "portal-1"}},
			},
			APIs: []APIResource{
				{BaseResource: BaseResource{Ref: "api-1"}},
				{BaseResource: BaseResource{Ref: "api-2"}},
			},
		}

		all := rs.AllResources()
		refs := extractRefs(all)

		assert.Contains(t, refs, "portal-1")
		assert.Contains(t, refs, "api-1")
		assert.Contains(t, refs, "api-2")
		assert.Len(t, all, 3)
	})

	t.Run("empty set", func(t *testing.T) {
		rs := &ResourceSet{}
		all := rs.AllResources()
		assert.Empty(t, all)
	})
}

func TestResourceCount(t *testing.T) {
	t.Run("matches AllResources length", func(t *testing.T) {
		rs := &ResourceSet{
			APIs: []APIResource{
				{BaseResource: BaseResource{Ref: "api-1"}},
			},
			ControlPlanes: []ControlPlaneResource{
				{BaseResource: BaseResource{Ref: "cp-1"}},
				{BaseResource: BaseResource{Ref: "cp-2"}},
			},
		}

		assert.Equal(t, len(rs.AllResources()), rs.ResourceCount())
	})

	t.Run("empty set", func(t *testing.T) {
		rs := &ResourceSet{}
		assert.Equal(t, 0, rs.ResourceCount())
	})
}

func TestIsEmpty(t *testing.T) {
	t.Run("true for empty set", func(t *testing.T) {
		rs := &ResourceSet{}
		assert.True(t, rs.IsEmpty())
	})

	t.Run("false when resources exist", func(t *testing.T) {
		rs := &ResourceSet{
			APIs: []APIResource{
				{BaseResource: BaseResource{Ref: "api-1"}},
			},
		}
		assert.False(t, rs.IsEmpty())
	})
}

func TestForEachResource(t *testing.T) {
	t.Run("visits all resources", func(t *testing.T) {
		rs := &ResourceSet{
			Portals: []PortalResource{
				{BaseResource: BaseResource{Ref: "portal-1"}},
			},
			APIs: []APIResource{
				{BaseResource: BaseResource{Ref: "api-1"}},
			},
		}

		var visited []string
		completed := rs.ForEachResource(func(r Resource) bool {
			visited = append(visited, r.GetRef())
			return true
		})

		assert.True(t, completed)
		assert.Len(t, visited, 2)
		assert.Contains(t, visited, "portal-1")
		assert.Contains(t, visited, "api-1")
	})

	t.Run("stops early on condition", func(t *testing.T) {
		rs := &ResourceSet{
			APIs: []APIResource{
				{BaseResource: BaseResource{Ref: "api-1"}},
				{BaseResource: BaseResource{Ref: "api-2"}},
				{BaseResource: BaseResource{Ref: "api-3"}},
			},
		}

		var visited []string
		completed := rs.ForEachResource(func(r Resource) bool {
			visited = append(visited, r.GetRef())
			return r.GetRef() != "api-2" // stop when we find api-2
		})

		assert.False(t, completed)
		assert.Contains(t, visited, "api-2")
		assert.NotContains(t, visited, "api-3")
	})

	t.Run("mutates in place", func(t *testing.T) {
		rs := &ResourceSet{
			APIs: []APIResource{
				{BaseResource: BaseResource{Ref: "api-1"}},
			},
		}

		// SetKonnectID through ForEachResource should mutate the original slice
		rs.ForEachResource(func(r Resource) bool {
			if r.GetRef() == "api-1" {
				r.(*APIResource).SetKonnectID("konnect-123")
			}
			return true
		})

		assert.Equal(t, "konnect-123", rs.APIs[0].GetKonnectID())
	})

	t.Run("empty set", func(t *testing.T) {
		rs := &ResourceSet{}
		completed := rs.ForEachResource(func(_ Resource) bool {
			t.Fatal("should not be called")
			return true
		})
		assert.True(t, completed)
	})
}

func TestAllResourcesByType(t *testing.T) {
	t.Run("returns correct type", func(t *testing.T) {
		rs := &ResourceSet{
			Portals: []PortalResource{
				{BaseResource: BaseResource{Ref: "portal-1"}},
			},
			APIs: []APIResource{
				{BaseResource: BaseResource{Ref: "api-1"}},
				{BaseResource: BaseResource{Ref: "api-2"}},
			},
		}

		apis := rs.AllResourcesByType(ResourceTypeAPI)
		require.Len(t, apis, 2)
		assert.Equal(t, "api-1", apis[0].GetRef())
		assert.Equal(t, "api-2", apis[1].GetRef())

		portals := rs.AllResourcesByType(ResourceTypePortal)
		require.Len(t, portals, 1)
		assert.Equal(t, "portal-1", portals[0].GetRef())
	})

	t.Run("unregistered type returns nil", func(t *testing.T) {
		rs := &ResourceSet{}
		result := rs.AllResourcesByType(ResourceType("nonexistent_type"))
		assert.Nil(t, result)
	})

	t.Run("empty slice", func(t *testing.T) {
		rs := &ResourceSet{}
		result := rs.AllResourcesByType(ResourceTypeAPI)
		assert.Empty(t, result)
	})
}

func TestAppendAll(t *testing.T) {
	t.Run("merges resources", func(t *testing.T) {
		dest := &ResourceSet{
			APIs: []APIResource{
				{BaseResource: BaseResource{Ref: "api-1"}},
			},
		}
		src := &ResourceSet{
			APIs: []APIResource{
				{BaseResource: BaseResource{Ref: "api-2"}},
			},
			Portals: []PortalResource{
				{BaseResource: BaseResource{Ref: "portal-1"}},
			},
		}

		dest.AppendAll(src)

		assert.Len(t, dest.APIs, 2)
		assert.Len(t, dest.Portals, 1)
		assert.Equal(t, "api-1", dest.APIs[0].Ref)
		assert.Equal(t, "api-2", dest.APIs[1].Ref)
		assert.Equal(t, "portal-1", dest.Portals[0].Ref)
	})

	t.Run("from empty source", func(t *testing.T) {
		dest := &ResourceSet{
			APIs: []APIResource{
				{BaseResource: BaseResource{Ref: "api-1"}},
			},
		}
		src := &ResourceSet{}

		dest.AppendAll(src)
		assert.Len(t, dest.APIs, 1)
	})
}

func TestIsRegistered(t *testing.T) {
	t.Run("known types", func(t *testing.T) {
		knownTypes := []ResourceType{
			ResourceTypePortal,
			ResourceTypeAPI,
			ResourceTypeControlPlane,
			ResourceTypeCatalogService,
			ResourceTypeApplicationAuthStrategy,
			ResourceTypeGatewayService,
			ResourceTypeOrganizationTeam,
			ResourceTypeEventGatewayControlPlane,
		}

		for _, rt := range knownTypes {
			assert.True(t, IsRegistered(rt), "expected %s to be registered", rt)
		}
	})

	t.Run("unknown type", func(t *testing.T) {
		assert.False(t, IsRegistered(ResourceType("unknown_type")))
	})
}

func TestRegisteredTypesContainsKnownTypes(t *testing.T) {
	types := RegisteredTypes()
	assert.NotEmpty(t, types)

	typeSet := make(map[ResourceType]bool)
	for _, rt := range types {
		typeSet[rt] = true
	}

	assert.True(t, typeSet[ResourceTypePortal])
	assert.True(t, typeSet[ResourceTypeAPI])
	assert.True(t, typeSet[ResourceTypeControlPlane])

	assert.False(t, typeSet[ResourceType("unknown_type")])
}

// extractRefs is a test helper that collects refs from a slice of Resources.
func extractRefs(resources []Resource) []string {
	refs := make([]string, len(resources))
	for i, r := range resources {
		refs[i] = r.GetRef()
	}
	return refs
}
