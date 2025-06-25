package loader

import (
	"reflect"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
	"github.com/stretchr/testify/assert"
)

func TestLoader_validateResourceSet_EmptyResourceSet(t *testing.T) {
	loader := New(".")
	rs := &resources.ResourceSet{}
	
	err := loader.validateResourceSet(rs)
	assert.NoError(t, err, "Empty resource set should be valid")
}

func TestLoader_validatePortals(t *testing.T) {
	loader := New(".")
	registry := make(map[string]map[string]bool)
	
	tests := []struct {
		name        string
		portals     []resources.PortalResource
		wantErr     bool
		expectedErr string
	}{
		{
			name: "valid portals",
			portals: []resources.PortalResource{
				{Ref: "portal1"},
				{Ref: "portal2"},
			},
			wantErr: false,
		},
		{
			name: "duplicate refs",
			portals: []resources.PortalResource{
				{Ref: "portal1"},
				{Ref: "portal1"},
			},
			wantErr:     true,
			expectedErr: "duplicate portal ref: portal1",
		},
		{
			name: "missing ref",
			portals: []resources.PortalResource{
				{Ref: ""},
			},
			wantErr:     true,
			expectedErr: "portal ref is required",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset registry for each test
			registry = make(map[string]map[string]bool)
			
			err := loader.validatePortals(tt.portals, registry)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
				// Check that registry was populated
				assert.Contains(t, registry, "portal")
				for _, portal := range tt.portals {
					assert.True(t, registry["portal"][portal.GetRef()])
				}
			}
		})
	}
}

func TestLoader_validateAuthStrategies(t *testing.T) {
	loader := New(".")
	registry := make(map[string]map[string]bool)
	
	tests := []struct {
		name        string
		strategies  []resources.ApplicationAuthStrategyResource
		wantErr     bool
		expectedErr string
	}{
		{
			name: "valid strategies",
			strategies: []resources.ApplicationAuthStrategyResource{
				{Ref: "oauth1"},
				{Ref: "oauth2"},
			},
			wantErr: false,
		},
		{
			name: "duplicate refs",
			strategies: []resources.ApplicationAuthStrategyResource{
				{Ref: "oauth1"},
				{Ref: "oauth1"},
			},
			wantErr:     true,
			expectedErr: "duplicate application_auth_strategy ref: oauth1",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry = make(map[string]map[string]bool)
			
			err := loader.validateAuthStrategies(tt.strategies, registry)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, registry, "application_auth_strategy")
			}
		})
	}
}

func TestLoader_validateControlPlanes(t *testing.T) {
	loader := New(".")
	registry := make(map[string]map[string]bool)
	
	tests := []struct {
		name        string
		cps         []resources.ControlPlaneResource
		wantErr     bool
		expectedErr string
	}{
		{
			name: "valid control planes",
			cps: []resources.ControlPlaneResource{
				{Ref: "cp1"},
				{Ref: "cp2"},
			},
			wantErr: false,
		},
		{
			name: "duplicate refs",
			cps: []resources.ControlPlaneResource{
				{Ref: "cp1"},
				{Ref: "cp1"},
			},
			wantErr:     true,
			expectedErr: "duplicate control_plane ref: cp1",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry = make(map[string]map[string]bool)
			
			err := loader.validateControlPlanes(tt.cps, registry)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, registry, "control_plane")
			}
		})
	}
}

func TestLoader_validateAPIs(t *testing.T) {
	loader := New(".")
	registry := make(map[string]map[string]bool)
	
	tests := []struct {
		name        string
		apis        []resources.APIResource
		wantErr     bool
		expectedErr string
	}{
		{
			name: "valid APIs",
			apis: []resources.APIResource{
				{Ref: "api1"},
				{Ref: "api2"},
			},
			wantErr: false,
		},
		{
			name: "duplicate API refs",
			apis: []resources.APIResource{
				{Ref: "api1"},
				{Ref: "api1"},
			},
			wantErr:     true,
			expectedErr: "duplicate api ref: api1",
		},
		{
			name: "API with duplicate version refs",
			apis: []resources.APIResource{
				{
					Ref: "api1",
					Versions: []resources.APIVersionResource{
						{Ref: "v1"},
						{Ref: "v1"},
					},
				},
			},
			wantErr:     true,
			expectedErr: "duplicate api_version ref: v1",
		},
		{
			name: "API with duplicate publication refs",
			apis: []resources.APIResource{
				{
					Ref: "api1",
					Publications: []resources.APIPublicationResource{
						{Ref: "pub1", PortalID: "portal1"},
						{Ref: "pub1", PortalID: "portal1"},
					},
				},
			},
			wantErr:     true,
			expectedErr: "duplicate api_publication ref: pub1",
		},
		{
			name: "API with duplicate implementation refs",
			apis: []resources.APIResource{
				{
					Ref: "api1",
					Implementations: []resources.APIImplementationResource{
						{
							Ref: "impl1",
							APIImplementation: kkInternalComps.APIImplementation{
								Service: &kkInternalComps.APIImplementationServiceInput{
									ID:             "12345678-1234-1234-1234-123456789012",
									ControlPlaneID: "cp1",
								},
							},
						},
						{
							Ref: "impl1",
							APIImplementation: kkInternalComps.APIImplementation{
								Service: &kkInternalComps.APIImplementationServiceInput{
									ID:             "12345678-1234-1234-1234-123456789012",
									ControlPlaneID: "cp1",
								},
							},
						},
					},
				},
			},
			wantErr:     true,
			expectedErr: "duplicate api_implementation ref: impl1",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry = make(map[string]map[string]bool)
			
			err := loader.validateAPIs(tt.apis, registry)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, registry, "api")
			}
		})
	}
}

func TestLoader_validateCrossReferences(t *testing.T) {
	loader := New(".")
	
	// Create a registry with some resources
	registry := map[string]map[string]bool{
		"portal":                     {"portal1": true},
		"application_auth_strategy":  {"oauth1": true},
		"control_plane":             {"cp1": true},
	}
	
	tests := []struct {
		name        string
		rs          *resources.ResourceSet
		wantErr     bool
		expectedErr string
	}{
		{
			name: "valid references",
			rs: &resources.ResourceSet{
				Portals: []resources.PortalResource{
					{
						Ref: "portal1",
						CreatePortal: kkInternalComps.CreatePortal{
							DefaultApplicationAuthStrategyID: stringPtr("oauth1"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid portal reference",
			rs: &resources.ResourceSet{
				Portals: []resources.PortalResource{
					{
						Ref: "portal1",
						CreatePortal: kkInternalComps.CreatePortal{
							DefaultApplicationAuthStrategyID: stringPtr("nonexistent"),
						},
					},
				},
			},
			wantErr:     true,
			expectedErr: "references unknown application_auth_strategy: nonexistent",
		},
		{
			name: "empty references should be allowed",
			rs: &resources.ResourceSet{
				Portals: []resources.PortalResource{
					{
						Ref: "portal1",
						// No default auth strategy - should be fine
					},
				},
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := loader.validateCrossReferences(tt.rs, registry)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoader_getFieldValue(t *testing.T) {
	loader := New(".")
	
	// Test with a mock struct that has various field types
	type TestStruct struct {
		SimpleField string `yaml:"simple_field"`
		NestedStruct struct {
			NestedField string `yaml:"nested_field"`
		} `yaml:"nested_struct"`
	}
	
	testObj := TestStruct{
		SimpleField: "simple_value",
	}
	testObj.NestedStruct.NestedField = "nested_value"
	
	tests := []struct {
		name      string
		fieldPath string
		expected  string
	}{
		{
			name:      "simple field",
			fieldPath: "simple_field",
			expected:  "simple_value",
		},
		{
			name:      "nested field",
			fieldPath: "nested_struct.nested_field",
			expected:  "nested_value",
		},
		{
			name:      "nonexistent field",
			fieldPath: "nonexistent",
			expected:  "",
		},
		{
			name:      "nonexistent nested field",
			fieldPath: "nested_struct.nonexistent",
			expected:  "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := loader.getFieldValue(testObj, tt.fieldPath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoader_findField(t *testing.T) {
	loader := New(".")
	
	type TestStruct struct {
		DirectField string `yaml:"direct_field"`
		YAMLField   string `yaml:"yaml_tagged_field"`
	}
	
	testObj := TestStruct{
		DirectField: "direct_value",
		YAMLField:   "yaml_value",
	}
	
	v := reflect.ValueOf(testObj)
	
	tests := []struct {
		name      string
		fieldName string
		wantValid bool
		expected  string
	}{
		{
			name:      "direct field name",
			fieldName: "DirectField",
			wantValid: true,
			expected:  "direct_value",
		},
		{
			name:      "yaml tag",
			fieldName: "yaml_tagged_field",
			wantValid: true,
			expected:  "yaml_value",
		},
		{
			name:      "nonexistent field",
			fieldName: "nonexistent",
			wantValid: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := loader.findField(v, tt.fieldName)
			if tt.wantValid {
				assert.True(t, field.IsValid())
				assert.Equal(t, tt.expected, field.String())
			} else {
				assert.False(t, field.IsValid())
			}
		})
	}
}

// Helper function for tests
func stringPtr(s string) *string {
	return &s
}