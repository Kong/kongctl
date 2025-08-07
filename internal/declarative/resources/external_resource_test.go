package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExternalResourceResource_Validate(t *testing.T) {
	tests := []struct {
		name     string
		resource ExternalResourceResource
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid with ID",
			resource: ExternalResourceResource{
				Ref:          "my-portal",
				ResourceType: "portal",
				ID:           extStringPtr("portal-123"),
			},
			wantErr: false,
		},
		{
			name: "valid with selector",
			resource: ExternalResourceResource{
				Ref:          "my-portal",
				ResourceType: "portal",
				Selector: &ExternalResourceSelector{
					MatchFields: map[string]string{
						"name": "My Portal",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with selector and multiple fields",
			resource: ExternalResourceResource{
				Ref:          "my-api",
				ResourceType: "api",
				Selector: &ExternalResourceSelector{
					MatchFields: map[string]string{
						"name":        "My API",
						"description": "Test API",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with parent ID",
			resource: ExternalResourceResource{
				Ref:          "my-api-version",
				ResourceType: "api_version",
				ID:           extStringPtr("version-123"),
				Parent: &ExternalResourceParent{
					ResourceType: "api",
					ID:           "api-456",
				},
			},
			wantErr: false,
		},
		{
			name: "valid with parent ref",
			resource: ExternalResourceResource{
				Ref:          "my-api-version",
				ResourceType: "api_version",
				Selector: &ExternalResourceSelector{
					MatchFields: map[string]string{
						"name": "v1.0",
					},
				},
				Parent: &ExternalResourceParent{
					ResourceType: "api",
					Ref:          "parent-api",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid - both ID and selector",
			resource: ExternalResourceResource{
				Ref:          "my-portal",
				ResourceType: "portal",
				ID:           extStringPtr("portal-123"),
				Selector: &ExternalResourceSelector{
					MatchFields: map[string]string{
						"name": "My Portal",
					},
				},
			},
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name: "invalid - neither ID nor selector",
			resource: ExternalResourceResource{
				Ref:          "my-portal",
				ResourceType: "portal",
			},
			wantErr: true,
			errMsg:  "must be specified",
		},
		{
			name: "invalid - empty ref",
			resource: ExternalResourceResource{
				Ref:          "",
				ResourceType: "portal",
				ID:           extStringPtr("portal-123"),
			},
			wantErr: true,
			errMsg:  "invalid external resource ref",
		},
		{
			name: "invalid - empty resource type",
			resource: ExternalResourceResource{
				Ref: "my-resource",
				ID:  extStringPtr("resource-123"),
			},
			wantErr: true,
			errMsg:  "resource_type is required",
		},
		{
			name: "invalid resource type",
			resource: ExternalResourceResource{
				Ref:          "my-resource",
				ResourceType: "invalid_type",
				ID:           extStringPtr("resource-123"),
			},
			wantErr: true,
			errMsg:  "unsupported resource_type",
		},
		{
			name: "invalid selector - empty match fields",
			resource: ExternalResourceResource{
				Ref:          "my-portal",
				ResourceType: "portal",
				Selector: &ExternalResourceSelector{
					MatchFields: map[string]string{},
				},
			},
			wantErr: true,
			errMsg:  "must be specified", // Empty selector is same as no selector
		},
		{
			name: "invalid selector - unsupported field",
			resource: ExternalResourceResource{
				Ref:          "my-portal",
				ResourceType: "portal",
				Selector: &ExternalResourceSelector{
					MatchFields: map[string]string{
						"invalid_field": "value",
					},
				},
			},
			wantErr: true,
			errMsg:  "not supported for selector",
		},
		{
			name: "invalid parent - both ID and ref",
			resource: ExternalResourceResource{
				Ref:          "my-api-version",
				ResourceType: "api_version",
				ID:           extStringPtr("version-123"),
				Parent: &ExternalResourceParent{
					ResourceType: "api",
					ID:           "api-456",
					Ref:          "parent-api",
				},
			},
			wantErr: true,
			errMsg:  "parent 'id' and 'ref' are mutually exclusive",
		},
		{
			name: "invalid parent - neither ID nor ref",
			resource: ExternalResourceResource{
				Ref:          "my-api-version",
				ResourceType: "api_version",
				ID:           extStringPtr("version-123"),
				Parent: &ExternalResourceParent{
					ResourceType: "api",
				},
			},
			wantErr: true,
			errMsg:  "parent must specify either 'id' or 'ref'",
		},
		{
			name: "invalid parent-child relationship",
			resource: ExternalResourceResource{
				Ref:          "my-portal",
				ResourceType: "portal",
				ID:           extStringPtr("portal-123"),
				Parent: &ExternalResourceParent{
					ResourceType: "api",
					ID:           "api-456",
				},
			},
			wantErr: true,
			errMsg:  "cannot have parent of type",
		},
		{
			name: "valid ce_service with control_plane parent ID",
			resource: ExternalResourceResource{
				Ref:          "my-service",
				ResourceType: "ce_service",
				ID:           extStringPtr("service-123"),
				Parent: &ExternalResourceParent{
					ResourceType: "control_plane",
					ID:           "cp-456",
				},
			},
			wantErr: false,
		},
		{
			name: "valid ce_service with control_plane parent ref",
			resource: ExternalResourceResource{
				Ref:          "my-service",
				ResourceType: "ce_service",
				Selector: &ExternalResourceSelector{
					MatchFields: map[string]string{
						"name": "user-service",
					},
				},
				Parent: &ExternalResourceParent{
					ResourceType: "control_plane",
					Ref:          "prod-cp",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid ce_service without parent",
			resource: ExternalResourceResource{
				Ref:          "my-service",
				ResourceType: "ce_service",
				ID:           extStringPtr("service-123"),
			},
			wantErr: true,
			errMsg:  "requires parent",
		},
		{
			name: "invalid ce_service with wrong parent type",
			resource: ExternalResourceResource{
				Ref:          "my-service",
				ResourceType: "ce_service",
				ID:           extStringPtr("service-123"),
				Parent: &ExternalResourceParent{
					ResourceType: "api",
					ID:           "api-456",
				},
			},
			wantErr: true,
			errMsg:  "cannot have parent of type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.resource.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExternalResourceResource_GettersAndSetters(t *testing.T) {
	resource := ExternalResourceResource{
		Ref:          "test-resource",
		ResourceType: "portal",
	}

	// Test GetRef
	assert.Equal(t, "test-resource", resource.GetRef())

	// Test GetResourceType
	assert.Equal(t, "portal", resource.GetResourceType())

	// Test resolved state methods
	assert.False(t, resource.IsResolved())
	assert.Equal(t, "", resource.GetResolvedID())
	assert.Nil(t, resource.GetResolvedResource())

	// Set resolved ID
	resource.SetResolvedID("portal-123")
	assert.True(t, resource.IsResolved())
	assert.Equal(t, "portal-123", resource.GetResolvedID())

	// Set resolved resource
	mockResource := map[string]string{"id": "portal-123", "name": "Test Portal"}
	resource.SetResolvedResource(mockResource)
	assert.Equal(t, mockResource, resource.GetResolvedResource())
}

func TestValidateResourceType(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "valid portal",
			resourceType: "portal",
			wantErr:      false,
		},
		{
			name:         "valid api",
			resourceType: "api",
			wantErr:      false,
		},
		{
			name:         "valid control_plane",
			resourceType: "control_plane",
			wantErr:      false,
		},
		{
			name:         "valid api_version",
			resourceType: "api_version",
			wantErr:      false,
		},
		{
			name:         "empty resource type",
			resourceType: "",
			wantErr:      true,
			errMsg:       "resource_type is required",
		},
		{
			name:         "invalid resource type",
			resourceType: "invalid",
			wantErr:      true,
			errMsg:       "unsupported resource_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateResourceType(tt.resourceType)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateIDXORSelector(t *testing.T) {
	tests := []struct {
		name     string
		id       *string
		selector *ExternalResourceSelector
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid with ID",
			id:       extStringPtr("resource-123"),
			selector: nil,
			wantErr:  false,
		},
		{
			name: "valid with selector",
			id:   nil,
			selector: &ExternalResourceSelector{
				MatchFields: map[string]string{"name": "test"},
			},
			wantErr: false,
		},
		{
			name:     "invalid - both ID and selector",
			id:       extStringPtr("resource-123"),
			selector: &ExternalResourceSelector{
				MatchFields: map[string]string{"name": "test"},
			},
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name:     "invalid - neither ID nor selector",
			id:       nil,
			selector: nil,
			wantErr:  true,
			errMsg:  "must be specified",
		},
		{
			name:     "invalid - empty ID string",
			id:       extStringPtr(""),
			selector: nil,
			wantErr:  true,
			errMsg:  "must be specified",
		},
		{
			name: "invalid - empty selector fields",
			id:   nil,
			selector: &ExternalResourceSelector{
				MatchFields: map[string]string{},
			},
			wantErr: true,
			errMsg:  "must be specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIDXORSelector(tt.id, tt.selector)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateSelector(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		selector     *ExternalResourceSelector
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "valid portal selector with name",
			resourceType: "portal",
			selector: &ExternalResourceSelector{
				MatchFields: map[string]string{"name": "My Portal"},
			},
			wantErr: false,
		},
		{
			name:         "valid api selector with multiple fields",
			resourceType: "api",
			selector: &ExternalResourceSelector{
				MatchFields: map[string]string{
					"name":        "My API",
					"description": "Test Description",
				},
			},
			wantErr: false,
		},
		{
			name:         "valid api_version selector",
			resourceType: "api_version",
			selector: &ExternalResourceSelector{
				MatchFields: map[string]string{
					"name":    "v1",
					"version": "1.0.0",
				},
			},
			wantErr: false,
		},
		{
			name:         "nil selector",
			resourceType: "portal",
			selector:     nil,
			wantErr:      true,
			errMsg:       "selector cannot be nil",
		},
		{
			name:         "empty match fields",
			resourceType: "portal",
			selector: &ExternalResourceSelector{
				MatchFields: map[string]string{},
			},
			wantErr: true,
			errMsg:  "selector.match_fields cannot be empty",
		},
		{
			name:         "unsupported field for resource type",
			resourceType: "portal",
			selector: &ExternalResourceSelector{
				MatchFields: map[string]string{"invalid_field": "value"},
			},
			wantErr: true,
			errMsg:  "not supported for selector",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSelector(tt.resourceType, tt.selector)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateParent(t *testing.T) {
	tests := []struct {
		name             string
		childResourceType string
		parent           *ExternalResourceParent
		wantErr          bool
		errMsg           string
	}{
		{
			name:              "valid api_version with api parent ID",
			childResourceType: "api_version",
			parent: &ExternalResourceParent{
				ResourceType: "api",
				ID:           "api-123",
			},
			wantErr: false,
		},
		{
			name:              "valid api_version with api parent ref",
			childResourceType: "api_version",
			parent: &ExternalResourceParent{
				ResourceType: "api",
				Ref:          "parent-api",
			},
			wantErr: false,
		},
		{
			name:              "valid portal_page with portal parent",
			childResourceType: "portal_page",
			parent: &ExternalResourceParent{
				ResourceType: "portal",
				ID:           "portal-123",
			},
			wantErr: false,
		},
		{
			name:              "nil parent",
			childResourceType: "api_version",
			parent:           nil,
			wantErr:          true,
			errMsg:           "parent cannot be nil",
		},
		{
			name:              "parent with both ID and ref",
			childResourceType: "api_version",
			parent: &ExternalResourceParent{
				ResourceType: "api",
				ID:           "api-123",
				Ref:          "parent-api",
			},
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name:              "parent with neither ID nor ref",
			childResourceType: "api_version",
			parent: &ExternalResourceParent{
				ResourceType: "api",
			},
			wantErr: true,
			errMsg:  "must specify either",
		},
		{
			name:              "invalid parent-child relationship",
			childResourceType: "portal",
			parent: &ExternalResourceParent{
				ResourceType: "api",
				ID:           "api-123",
			},
			wantErr: true,
			errMsg:  "cannot have parent of type",
		},
		{
			name:              "invalid parent resource type",
			childResourceType: "api_version",
			parent: &ExternalResourceParent{
				ResourceType: "invalid",
				ID:           "invalid-123",
			},
			wantErr: true,
			errMsg:  "unsupported resource_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateParent(tt.childResourceType, tt.parent)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExternalResourceError(t *testing.T) {
	err := NewExternalResourceError(
		"test-ref",
		"portal",
		"selector",
		"invalid configuration",
		nil,
	)

	assert.Contains(t, err.Error(), "test-ref")
	assert.Contains(t, err.Error(), "portal")
	assert.Contains(t, err.Error(), "selector")
	assert.Contains(t, err.Error(), "invalid configuration")

	// Test without field
	err2 := NewExternalResourceError(
		"test-ref",
		"api",
		"",
		"general error",
		nil,
	)
	assert.Contains(t, err2.Error(), "test-ref")
	assert.Contains(t, err2.Error(), "api")
	assert.Contains(t, err2.Error(), "general error")
	assert.NotContains(t, err2.Error(), "in field")
}

// Helper functions
func extStringPtr(s string) *string {
	return &s
}