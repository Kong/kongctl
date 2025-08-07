package external

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolutionRegistry_IsSupported(t *testing.T) {
	registry := GetResolutionRegistry()

	tests := []struct {
		name         string
		resourceType string
		want         bool
	}{
		{
			name:         "supported portal",
			resourceType: "portal",
			want:         true,
		},
		{
			name:         "supported api",
			resourceType: "api",
			want:         true,
		},
		{
			name:         "supported control_plane",
			resourceType: "control_plane",
			want:         true,
		},
		{
			name:         "supported ce_service",
			resourceType: "ce_service",
			want:         true,
		},
		{
			name:         "supported api_version",
			resourceType: "api_version",
			want:         true,
		},
		{
			name:         "supported api_publication",
			resourceType: "api_publication",
			want:         true,
		},
		{
			name:         "supported api_implementation",
			resourceType: "api_implementation",
			want:         true,
		},
		{
			name:         "supported api_document",
			resourceType: "api_document",
			want:         true,
		},
		{
			name:         "supported application_auth_strategy",
			resourceType: "application_auth_strategy",
			want:         true,
		},
		{
			name:         "supported portal_customization",
			resourceType: "portal_customization",
			want:         true,
		},
		{
			name:         "supported portal_custom_domain",
			resourceType: "portal_custom_domain",
			want:         true,
		},
		{
			name:         "supported portal_page",
			resourceType: "portal_page",
			want:         true,
		},
		{
			name:         "supported portal_snippet",
			resourceType: "portal_snippet",
			want:         true,
		},
		{
			name:         "unsupported type",
			resourceType: "invalid_type",
			want:         false,
		},
		{
			name:         "empty type",
			resourceType: "",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := registry.IsSupported(tt.resourceType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolutionRegistry_GetSupportedTypes(t *testing.T) {
	registry := GetResolutionRegistry()
	
	supportedTypes := registry.GetSupportedTypes()
	
	// Check that we have the expected number of resource types
	assert.GreaterOrEqual(t, len(supportedTypes), 13)
	
	// Check that key resource types are present
	expectedTypes := []string{
		"portal",
		"api",
		"control_plane",
		"ce_service",
		"api_version",
		"api_publication",
		"api_implementation",
		"api_document",
		"application_auth_strategy",
		"portal_customization",
		"portal_custom_domain",
		"portal_page",
		"portal_snippet",
	}
	
	for _, expected := range expectedTypes {
		assert.Contains(t, supportedTypes, expected)
	}
}

func TestResolutionRegistry_GetSupportedSelectorFields(t *testing.T) {
	registry := GetResolutionRegistry()

	tests := []struct {
		name           string
		resourceType   string
		expectedFields []string
		shouldBeNil    bool
	}{
		{
			name:           "portal selector fields",
			resourceType:   "portal",
			expectedFields: []string{"name", "description"},
		},
		{
			name:           "api selector fields",
			resourceType:   "api",
			expectedFields: []string{"name", "description"},
		},
		{
			name:           "control_plane selector fields",
			resourceType:   "control_plane",
			expectedFields: []string{"name", "description"},
		},
		{
			name:           "ce_service selector fields",
			resourceType:   "ce_service",
			expectedFields: []string{"name"},
		},
		{
			name:           "api_version selector fields",
			resourceType:   "api_version",
			expectedFields: []string{"name", "version"},
		},
		{
			name:           "api_publication selector fields",
			resourceType:   "api_publication",
			expectedFields: []string{"name"},
		},
		{
			name:           "api_document selector fields",
			resourceType:   "api_document",
			expectedFields: []string{"name", "path"},
		},
		{
			name:           "portal_page selector fields",
			resourceType:   "portal_page",
			expectedFields: []string{"name", "slug"},
		},
		{
			name:           "portal_custom_domain selector fields",
			resourceType:   "portal_custom_domain",
			expectedFields: []string{"domain"},
		},
		{
			name:         "invalid type returns nil",
			resourceType: "invalid_type",
			shouldBeNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := registry.GetSupportedSelectorFields(tt.resourceType)
			
			if tt.shouldBeNil {
				assert.Nil(t, fields)
			} else {
				assert.NotNil(t, fields)
				for _, expected := range tt.expectedFields {
					assert.Contains(t, fields, expected)
				}
			}
		})
	}
}

func TestResolutionRegistry_IsValidParentChild(t *testing.T) {
	registry := GetResolutionRegistry()

	tests := []struct {
		name       string
		parentType string
		childType  string
		want       bool
	}{
		// Valid API parent-child relationships
		{
			name:       "api -> api_version",
			parentType: "api",
			childType:  "api_version",
			want:       true,
		},
		{
			name:       "api -> api_publication",
			parentType: "api",
			childType:  "api_publication",
			want:       true,
		},
		{
			name:       "api -> api_implementation",
			parentType: "api",
			childType:  "api_implementation",
			want:       true,
		},
		{
			name:       "api -> api_document",
			parentType: "api",
			childType:  "api_document",
			want:       true,
		},
		// Valid Portal parent-child relationships
		{
			name:       "portal -> portal_customization",
			parentType: "portal",
			childType:  "portal_customization",
			want:       true,
		},
		{
			name:       "portal -> portal_custom_domain",
			parentType: "portal",
			childType:  "portal_custom_domain",
			want:       true,
		},
		{
			name:       "portal -> portal_page",
			parentType: "portal",
			childType:  "portal_page",
			want:       true,
		},
		{
			name:       "portal -> portal_snippet",
			parentType: "portal",
			childType:  "portal_snippet",
			want:       true,
		},
		// Valid Control Plane parent-child relationships
		{
			name:       "control_plane -> ce_service",
			parentType: "control_plane",
			childType:  "ce_service",
			want:       true,
		},
		// Invalid relationships
		{
			name:       "api_version -> api (reversed)",
			parentType: "api_version",
			childType:  "api",
			want:       false,
		},
		{
			name:       "portal -> api (cross-resource)",
			parentType: "portal",
			childType:  "api",
			want:       false,
		},
		{
			name:       "api -> portal (cross-resource)",
			parentType: "api",
			childType:  "portal",
			want:       false,
		},
		{
			name:       "control_plane -> api (invalid child)",
			parentType: "control_plane",
			childType:  "api",
			want:       false,
		},
		{
			name:       "ce_service -> control_plane (reversed)",
			parentType: "ce_service",
			childType:  "control_plane",
			want:       false,
		},
		{
			name:       "invalid parent type",
			parentType: "invalid",
			childType:  "api",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := registry.IsValidParentChild(tt.parentType, tt.childType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolutionRegistry_GetResolutionAdapter(t *testing.T) {
	registry := GetResolutionRegistry()

	// Test that query adapters return nil for now (will be implemented in future steps)
	tests := []string{
		"portal",
		"api",
		"control_plane",
		"api_version",
		"api_publication",
		"api_implementation",
		"api_document",
		"application_auth_strategy",
	}

	for _, resourceType := range tests {
		t.Run(resourceType, func(t *testing.T) {
			adapter, err := registry.GetResolutionAdapter(resourceType)
			// For now, adapters are nil (will be implemented in future steps)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "no resolution adapter configured")
			assert.Nil(t, adapter)
		})
	}

	// Test invalid resource type
	t.Run("invalid type", func(t *testing.T) {
		adapter, err := registry.GetResolutionAdapter("invalid_type")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported resource type")
		assert.Nil(t, adapter)
	})
}

func TestResolutionRegistry_GetResolutionMetadata(t *testing.T) {
	registry := GetResolutionRegistry()

	t.Run("existing resource type", func(t *testing.T) {
		info, exists := registry.GetResolutionMetadata("portal")
		assert.True(t, exists)
		assert.NotNil(t, info)
		assert.Equal(t, "Portal", info.Name)
		assert.Contains(t, info.SelectorFields, "name")
		assert.Contains(t, info.SelectorFields, "description")
		assert.Contains(t, info.SupportedChildren, "portal_customization")
		assert.Contains(t, info.SupportedChildren, "portal_page")
	})

	t.Run("api resource type", func(t *testing.T) {
		info, exists := registry.GetResolutionMetadata("api")
		assert.True(t, exists)
		assert.NotNil(t, info)
		assert.Equal(t, "API", info.Name)
		assert.Contains(t, info.SupportedChildren, "api_version")
		assert.Contains(t, info.SupportedChildren, "api_publication")
	})

	t.Run("child resource type", func(t *testing.T) {
		info, exists := registry.GetResolutionMetadata("api_version")
		assert.True(t, exists)
		assert.NotNil(t, info)
		assert.Equal(t, "API Version", info.Name)
		assert.Contains(t, info.SupportedParents, "api")
		assert.Contains(t, info.SelectorFields, "name")
		assert.Contains(t, info.SelectorFields, "version")
	})

	t.Run("non-existing resource type", func(t *testing.T) {
		info, exists := registry.GetResolutionMetadata("invalid_type")
		assert.False(t, exists)
		assert.Nil(t, info)
	})
}

func TestResolutionRegistry_Register(t *testing.T) {
	// Create a new registry instance for this test to avoid affecting the singleton
	registry := &ResolutionRegistry{
		types: make(map[string]*ResolutionMetadata),
	}

	// Register a new resource type
	newType := &ResolutionMetadata{
		Name:              "Custom Resource",
		SelectorFields:    []string{"id", "name"},
		SupportedParents:  []string{"parent_type"},
		SupportedChildren: []string{"child_type"},
	}

	registry.Register("custom_resource", newType)

	// Verify registration
	assert.True(t, registry.IsSupported("custom_resource"))
	
	info, exists := registry.GetResolutionMetadata("custom_resource")
	assert.True(t, exists)
	assert.Equal(t, "Custom Resource", info.Name)
	assert.Equal(t, newType.SelectorFields, info.SelectorFields)
	assert.Equal(t, newType.SupportedParents, info.SupportedParents)
	assert.Equal(t, newType.SupportedChildren, info.SupportedChildren)
}

func TestResolutionRegistry_Singleton(t *testing.T) {
	// Get registry instances
	registry1 := GetResolutionRegistry()
	registry2 := GetResolutionRegistry()

	// Verify they are the same instance
	assert.Same(t, registry1, registry2)

	// Verify built-in types are initialized
	assert.True(t, registry1.IsSupported("portal"))
	assert.True(t, registry1.IsSupported("api"))
	assert.True(t, registry1.IsSupported("control_plane"))
}