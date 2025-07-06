package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPIVersionResource_Interfaces(t *testing.T) {
	version := &APIVersionResource{
		Ref: "v1",
		API: "my-api",
	}

	// Test Resource interface
	var _ Resource = version
	assert.Equal(t, "api_version", version.GetKind())
	assert.Equal(t, "v1", version.GetRef())
	assert.Equal(t, "", version.GetMoniker()) // No version set
	
	deps := version.GetDependencies()
	assert.Len(t, deps, 1)
	assert.Equal(t, "api", deps[0].Kind)
	assert.Equal(t, "my-api", deps[0].Ref)

	// Test ResourceWithParent interface
	var _ ResourceWithParent = version
	parentRef := version.GetParentRef()
	assert.NotNil(t, parentRef)
	assert.Equal(t, "api", parentRef.Kind)
	assert.Equal(t, "my-api", parentRef.Ref)

	// Test with no parent
	versionNoParent := &APIVersionResource{Ref: "v1"}
	assert.Empty(t, versionNoParent.GetDependencies())
	assert.Nil(t, versionNoParent.GetParentRef())
}

func TestAPIPublicationResource_Interfaces(t *testing.T) {
	pub := &APIPublicationResource{
		Ref:      "pub1",
		API:      "my-api",
		PortalID: "dev-portal",
	}

	// Test Resource interface
	var _ Resource = pub
	assert.Equal(t, "api_publication", pub.GetKind())
	assert.Equal(t, "pub1", pub.GetRef())
	assert.Equal(t, "pub1", pub.GetName()) // Uses ref as name
	
	deps := pub.GetDependencies()
	assert.Len(t, deps, 1)
	assert.Equal(t, "api", deps[0].Kind)
	assert.Equal(t, "my-api", deps[0].Ref)

	// Test ResourceWithParent interface
	var _ ResourceWithParent = pub
	parentRef := pub.GetParentRef()
	assert.NotNil(t, parentRef)
	assert.Equal(t, "api", parentRef.Kind)
	assert.Equal(t, "my-api", parentRef.Ref)

	// Test reference field mappings
	mappings := pub.GetReferenceFieldMappings()
	assert.Equal(t, "portal", mappings["portal_id"])
	assert.Equal(t, "application_auth_strategy", mappings["auth_strategy_ids"])
}

func TestAPIImplementationResource_Interfaces(t *testing.T) {
	impl := &APIImplementationResource{
		Ref: "impl1",
		API: "my-api",
	}

	// Test Resource interface
	var _ Resource = impl
	assert.Equal(t, "api_implementation", impl.GetKind())
	assert.Equal(t, "impl1", impl.GetRef())
	assert.Equal(t, "impl1", impl.GetName()) // Uses ref as name
	
	deps := impl.GetDependencies()
	assert.Len(t, deps, 1)
	assert.Equal(t, "api", deps[0].Kind)
	assert.Equal(t, "my-api", deps[0].Ref)

	// Test ResourceWithParent interface
	var _ ResourceWithParent = impl
	parentRef := impl.GetParentRef()
	assert.NotNil(t, parentRef)
	assert.Equal(t, "api", parentRef.Kind)
	assert.Equal(t, "my-api", parentRef.Ref)
}

func TestAPIChildResources_Validation(t *testing.T) {
	// Test version validation
	version := APIVersionResource{}
	err := version.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ref is required")

	version.Ref = "v1"
	err = version.Validate()
	assert.NoError(t, err)

	// Test publication validation
	pub := APIPublicationResource{}
	err = pub.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ref is required")

	pub.Ref = "pub1"
	err = pub.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "portal_id is required")

	pub.PortalID = "portal1"
	err = pub.Validate()
	assert.NoError(t, err)

	// Test implementation validation
	impl := APIImplementationResource{}
	err = impl.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ref is required")

	impl.Ref = "impl1"
	err = impl.Validate()
	assert.NoError(t, err)
}