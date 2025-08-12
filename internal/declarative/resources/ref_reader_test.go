package resources

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestResourceSet_RefReader(t *testing.T) {
	tests := []struct {
		name           string
		resourceSet    *ResourceSet
		searchRef      string
		expectFound    bool
		expectType     ResourceType
	}{
		{
			name: "find portal by ref",
			resourceSet: &ResourceSet{
				Portals: []PortalResource{
					{Ref: "portal1"},
				},
			},
			searchRef:     "portal1",
			expectFound:   true,
			expectType:    ResourceTypePortal,
		},
		{
			name: "find api by ref",
			resourceSet: &ResourceSet{
				APIs: []APIResource{
					{Ref: "api1"},
				},
			},
			searchRef:     "api1",
			expectFound:   true,
			expectType:    ResourceTypeAPI,
		},
		{
			name: "find nested api version by ref",
			resourceSet: &ResourceSet{
				APIVersions: []APIVersionResource{
					{Ref: "v1", API: "api1"},
				},
			},
			searchRef:     "v1",
			expectFound:   true,
			expectType:    ResourceTypeAPIVersion,
		},
		{
			name: "ref not found",
			resourceSet: &ResourceSet{
				Portals: []PortalResource{
					{Ref: "portal1"},
				},
			},
			searchRef:   "nonexistent",
			expectFound: false,
		},
		{
			name: "check across multiple resource types",
			resourceSet: &ResourceSet{
				Portals: []PortalResource{
					{Ref: "portal1"},
				},
				APIs: []APIResource{
					{Ref: "api1"},
				},
				ApplicationAuthStrategies: []ApplicationAuthStrategyResource{
					{Ref: "auth1"},
				},
			},
			searchRef:     "auth1",
			expectFound:   true,
			expectType:    ResourceTypeApplicationAuthStrategy,
		},
		{
			name: "find api publication by ref",
			resourceSet: &ResourceSet{
				APIPublications: []APIPublicationResource{
					{Ref: "pub1", PortalID: "portal1"},
				},
			},
			searchRef:     "pub1",
			expectFound:   true,
			expectType:    ResourceTypeAPIPublication,
		},
		{
			name: "find api document by ref",
			resourceSet: &ResourceSet{
				APIDocuments: []APIDocumentResource{
					{Ref: "doc1", API: "api1"},
				},
			},
			searchRef:     "doc1",
			expectFound:   true,
			expectType:    ResourceTypeAPIDocument,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test HasRef
			found := tt.resourceSet.HasRef(tt.searchRef)
			assert.Equal(t, tt.expectFound, found, "HasRef result mismatch")
			
			// Test GetResourceByRef
			resource, found := tt.resourceSet.GetResourceByRef(tt.searchRef)
			assert.Equal(t, tt.expectFound, found, "GetResourceByRef found mismatch")
			
			if tt.expectFound {
				assert.NotNil(t, resource, "GetResourceByRef should return resource when found")
				assert.Equal(t, tt.searchRef, resource.GetRef(), "GetResourceByRef should return correct resource")
			} else {
				assert.Nil(t, resource, "GetResourceByRef should return nil when not found")
			}
			
			// Test GetResourceTypeByRef
			resourceType, found := tt.resourceSet.GetResourceTypeByRef(tt.searchRef)
			assert.Equal(t, tt.expectFound, found, "GetResourceTypeByRef found mismatch")
			if tt.expectFound {
				assert.Equal(t, tt.expectType, resourceType, "GetResourceTypeByRef type mismatch")
			}
		})
	}
}

func TestResourceSet_RefReader_GlobalUniqueness(t *testing.T) {
	// Test that refs are checked globally across all resource types
	rs := &ResourceSet{
		Portals: []PortalResource{
			{Ref: "shared-ref"},
		},
		APIs: []APIResource{
			{Ref: "different-ref"},
		},
	}
	
	// First ref should be found as portal
	resource, found := rs.GetResourceByRef("shared-ref")
	assert.True(t, found)
	assert.Equal(t, "shared-ref", resource.GetRef())
	
	// Check type through GetResourceTypeByRef
	resourceType, found := rs.GetResourceTypeByRef("shared-ref")
	assert.True(t, found)
	assert.Equal(t, ResourceTypePortal, resourceType)
	
	// Different ref should be found as API
	resource, found = rs.GetResourceByRef("different-ref")
	assert.True(t, found)
	assert.Equal(t, "different-ref", resource.GetRef())
	
	// Check type through GetResourceTypeByRef
	resourceType, found = rs.GetResourceTypeByRef("different-ref")
	assert.True(t, found)
	assert.Equal(t, ResourceTypeAPI, resourceType)
	
	// This test documents that if we had duplicate refs across types,
	// the first one found (in check order) would be returned
	// Phase 2 will prevent this by checking HasRef during loading
}

func TestResourceSet_RefReader_EmptyResourceSet(t *testing.T) {
	// Test with empty ResourceSet
	rs := &ResourceSet{}
	
	// Should not find any ref
	assert.False(t, rs.HasRef("any-ref"))
	
	resource, found := rs.GetResourceByRef("any-ref")
	assert.False(t, found)
	assert.Nil(t, resource)
	
	resourceType, found := rs.GetResourceTypeByRef("any-ref")
	assert.False(t, found)
	assert.Equal(t, ResourceType(""), resourceType)
}

func TestResourceSet_RefReader_InterfaceMethods(t *testing.T) {
	// Test that ResourceSet implements RefReader interface
	portal := PortalResource{Ref: "portal1"}
	portal.SetDefaults() // This sets Name from Ref if not set
	
	var refReader RefReader = &ResourceSet{
		Portals: []PortalResource{portal},
	}
	
	// Test interface methods work correctly
	assert.True(t, refReader.HasRef("portal1"))
	
	resource, found := refReader.GetResourceByRef("portal1")
	assert.True(t, found)
	assert.NotNil(t, resource)
	assert.Equal(t, "portal1", resource.GetRef())
	
	// Test that resource implements full Resource interface
	assert.Equal(t, ResourceTypePortal, resource.GetType())
	assert.NotNil(t, resource.GetDependencies())
	assert.NotEmpty(t, resource.GetMoniker())
	
	resourceType, found := refReader.GetResourceTypeByRef("portal1")
	assert.True(t, found)
	assert.Equal(t, ResourceTypePortal, resourceType)
}

func TestAllResourceTypes_ImplementResourceInterface(t *testing.T) {
	// Test that all resource types properly implement the Resource interface
	rs := &ResourceSet{
		Portals: []PortalResource{{Ref: "portal1"}},
		ApplicationAuthStrategies: []ApplicationAuthStrategyResource{{Ref: "auth1"}},
		ControlPlanes: []ControlPlaneResource{{Ref: "cp1"}},
		APIs: []APIResource{{Ref: "api1"}},
		APIVersions: []APIVersionResource{{Ref: "v1", API: "api1"}},
		APIPublications: []APIPublicationResource{{Ref: "pub1", PortalID: "portal1"}},
		APIImplementations: []APIImplementationResource{{Ref: "impl1"}},
		APIDocuments: []APIDocumentResource{{Ref: "doc1", API: "api1"}},
		PortalCustomizations: []PortalCustomizationResource{{Ref: "cust1"}},
		PortalCustomDomains: []PortalCustomDomainResource{{Ref: "dom1"}},
		PortalPages: []PortalPageResource{{Ref: "page1"}},
		PortalSnippets: []PortalSnippetResource{{Ref: "snip1"}},
	}
	
	// Test each resource type can be retrieved as Resource interface
	testCases := []struct {
		ref          string
		expectedKind ResourceType
	}{
		{"portal1", ResourceTypePortal},
		{"auth1", ResourceTypeApplicationAuthStrategy},
		{"cp1", ResourceTypeControlPlane},
		{"api1", ResourceTypeAPI},
		{"v1", ResourceTypeAPIVersion},
		{"pub1", ResourceTypeAPIPublication},
		{"impl1", ResourceTypeAPIImplementation},
		{"doc1", ResourceTypeAPIDocument},
		{"cust1", ResourceTypePortalCustomization},
		{"dom1", ResourceTypePortalCustomDomain},
		{"page1", ResourceTypePortalPage},
		{"snip1", ResourceTypePortalSnippet},
	}
	
	for _, tc := range testCases {
		t.Run(string(tc.expectedKind), func(t *testing.T) {
			resource, found := rs.GetResourceByRef(tc.ref)
			assert.True(t, found, "Resource with ref %s should be found", tc.ref)
			assert.NotNil(t, resource)
			
			// Verify Resource interface methods
			assert.Equal(t, tc.ref, resource.GetRef())
			assert.Equal(t, tc.expectedKind, resource.GetType())
			assert.NotNil(t, resource.GetDependencies())
			
			// Verify GetKonnectID and other methods don't panic
			_ = resource.GetKonnectID()
			_ = resource.GetKonnectMonikerFilter()
			
			// Verify resource type
			resourceType, found := rs.GetResourceTypeByRef(tc.ref)
			assert.True(t, found)
			assert.Equal(t, tc.expectedKind, resourceType)
		})
	}
}