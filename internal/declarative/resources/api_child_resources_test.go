package resources

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/assert"
)

func TestAPIVersionResource_Interfaces(t *testing.T) {
	version := &APIVersionResource{
		Ref: "v1",
		API: "my-api",
	}

	// Test Resource interface
	var _ Resource = version
	assert.Equal(t, ResourceTypeAPIVersion, version.GetType())
	assert.Equal(t, "v1", version.GetRef())
	assert.Equal(t, "", version.GetMoniker()) // No version set

	deps := version.GetDependencies()
	assert.Len(t, deps, 1)
	assert.Equal(t, ResourceTypeAPI, deps[0].Kind)
	assert.Equal(t, "my-api", deps[0].Ref)

	// Test ResourceWithParent interface
	var _ ResourceWithParent = version
	parentRef := version.GetParentRef()
	assert.NotNil(t, parentRef)
	assert.Equal(t, ResourceTypeAPI, parentRef.Kind)
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
	assert.Equal(t, ResourceTypeAPIPublication, pub.GetType())
	assert.Equal(t, "pub1", pub.GetRef())
	assert.Equal(t, "dev-portal", pub.GetMoniker()) // Uses portal_id as moniker

	deps := pub.GetDependencies()
	assert.Len(t, deps, 1)
	assert.Equal(t, ResourceTypeAPI, deps[0].Kind)
	assert.Equal(t, "my-api", deps[0].Ref)

	// Test ResourceWithParent interface
	var _ ResourceWithParent = pub
	parentRef := pub.GetParentRef()
	assert.NotNil(t, parentRef)
	assert.Equal(t, ResourceTypeAPI, parentRef.Kind)
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
	assert.Equal(t, ResourceTypeAPIImplementation, impl.GetType())
	assert.Equal(t, "impl1", impl.GetRef())
	assert.Equal(t, "", impl.GetMoniker()) // API implementations have no moniker

	deps := impl.GetDependencies()
	assert.Len(t, deps, 1)
	assert.Equal(t, ResourceTypeAPI, deps[0].Kind)
	assert.Equal(t, "my-api", deps[0].Ref)

	// Test ResourceWithParent interface
	var _ ResourceWithParent = impl
	parentRef := impl.GetParentRef()
	assert.NotNil(t, parentRef)
	assert.Equal(t, ResourceTypeAPI, parentRef.Kind)
	assert.Equal(t, "my-api", parentRef.Ref)
}

func TestAPIImplementationResource_ControlPlaneReferencesAndMatching(t *testing.T) {
	implementation := &APIImplementationResource{
		Ref: "implementation",
		APIImplementation: kkComps.CreateAPIImplementationControlPlaneReference(kkComps.ControlPlaneReference{
			ControlPlane: &kkComps.APIImplementationControlPlaneInput{ID: "control-plane-ref"},
		}),
	}

	assert.Equal(t, map[string]string{
		"control_plane.control_plane_id": string(ResourceTypeControlPlane),
	}, implementation.GetReferenceFieldMappings())
	assert.True(t, implementation.TryMatchKonnectResource(struct {
		ID           string
		ControlPlane *struct{ ID string }
	}{
		ID: "implementation-id",
		ControlPlane: &struct{ ID string }{
			ID: "control-plane-ref",
		},
	}))
	assert.Equal(t, "implementation-id", implementation.GetKonnectID())
}

func TestAPIChildResources_Validation(t *testing.T) {
	// Test version validation
	version := APIVersionResource{}
	err := version.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid API version ref: ref cannot be empty")

	version.Ref = "v1"
	err = version.Validate()
	assert.NoError(t, err)

	// Test publication validation
	pub := APIPublicationResource{}
	err = pub.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid API publication ref: ref cannot be empty")

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
	assert.Contains(t, err.Error(), "invalid API implementation ref: ref cannot be empty")

	impl.Ref = "impl1"
	err = impl.Validate()
	assert.ErrorContains(t, err, "exactly one of service or control_plane")

	impl.APIImplementation = kkComps.CreateAPIImplementationControlPlaneReference(kkComps.ControlPlaneReference{
		ControlPlane: &kkComps.APIImplementationControlPlaneInput{ID: "control-plane"},
	})
	assert.NoError(t, impl.Validate())
}
