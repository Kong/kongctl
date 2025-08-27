package loader

import (
	"reflect"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/assert"
)

func TestLoader_validateResourceSet_EmptyResourceSet(t *testing.T) {
	loader := New()
	rs := &resources.ResourceSet{}

	err := loader.validateResourceSet(rs)
	assert.NoError(t, err, "Empty resource set should be valid")
}

func TestLoader_validatePortals(t *testing.T) {
	loader := New()

	tests := []struct {
		name        string
		portals     []resources.PortalResource
		wantErr     bool
		expectedErr string
	}{
		{
			name: "valid portals",
			portals: []resources.PortalResource{
				{
					Ref: "portal1",
					CreatePortal: kkComps.CreatePortal{
						Name: "Portal One",
					},
				},
				{
					Ref: "portal2",
					CreatePortal: kkComps.CreatePortal{
						Name: "Portal Two",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate refs within same type not detected by validator",
			portals: []resources.PortalResource{
				{
					Ref: "portal1",
					CreatePortal: kkComps.CreatePortal{
						Name: "Portal One",
					},
				},
				{
					Ref: "portal1",
					CreatePortal: kkComps.CreatePortal{
						Name: "Portal Two", // Different name to avoid name duplicate error
					},
				},
			},
			wantErr:     false, // Validator doesn't check same-type duplicates anymore
			expectedErr: "",
		},
		{
			name: "missing ref",
			portals: []resources.PortalResource{
				{Ref: ""},
			},
			wantErr:     true,
			expectedErr: "invalid portal ref: ref cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a ResourceSet with the test portals
			rs := &resources.ResourceSet{
				Portals: tt.portals,
			}

			err := loader.validatePortals(tt.portals, rs)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
				// Verify all portals are in the ResourceSet
				for _, portal := range tt.portals {
					assert.True(t, rs.HasRef(portal.GetRef()))
				}
			}
		})
	}
}

func TestLoader_validateAuthStrategies(t *testing.T) {
	loader := New()

	tests := []struct {
		name        string
		strategies  []resources.ApplicationAuthStrategyResource
		wantErr     bool
		expectedErr string
	}{
		{
			name: "valid strategies",
			strategies: []resources.ApplicationAuthStrategyResource{
				{
					Ref: "oauth1",
					CreateAppAuthStrategyRequest: kkComps.CreateAppAuthStrategyRequest{
						Type: kkComps.CreateAppAuthStrategyRequestTypeKeyAuth,
						AppAuthStrategyKeyAuthRequest: &kkComps.AppAuthStrategyKeyAuthRequest{
							Name: "Key Auth One",
						},
					},
				},
				{
					Ref: "oauth2",
					CreateAppAuthStrategyRequest: kkComps.CreateAppAuthStrategyRequest{
						Type: kkComps.CreateAppAuthStrategyRequestTypeKeyAuth,
						AppAuthStrategyKeyAuthRequest: &kkComps.AppAuthStrategyKeyAuthRequest{
							Name: "Key Auth Two",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate refs within same type not detected by validator",
			strategies: []resources.ApplicationAuthStrategyResource{
				{
					Ref: "oauth1",
					CreateAppAuthStrategyRequest: kkComps.CreateAppAuthStrategyRequest{
						Type: kkComps.CreateAppAuthStrategyRequestTypeKeyAuth,
						AppAuthStrategyKeyAuthRequest: &kkComps.AppAuthStrategyKeyAuthRequest{
							Name: "Key Auth One",
						},
					},
				},
				{
					Ref: "oauth1",
					CreateAppAuthStrategyRequest: kkComps.CreateAppAuthStrategyRequest{
						Type: kkComps.CreateAppAuthStrategyRequestTypeKeyAuth,
						AppAuthStrategyKeyAuthRequest: &kkComps.AppAuthStrategyKeyAuthRequest{
							Name: "Key Auth Two", // Different name to avoid name duplicate error
						},
					},
				},
			},
			wantErr:     false, // Validator doesn't check same-type duplicates anymore
			expectedErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a ResourceSet with the test strategies
			rs := &resources.ResourceSet{
				ApplicationAuthStrategies: tt.strategies,
			}

			err := loader.validateAuthStrategies(tt.strategies, rs)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
				// Verify all strategies are in the ResourceSet
				for _, strategy := range tt.strategies {
					assert.True(t, rs.HasRef(strategy.GetRef()))
				}
			}
		})
	}
}

func TestLoader_validateControlPlanes(t *testing.T) {
	loader := New()

	tests := []struct {
		name        string
		cps         []resources.ControlPlaneResource
		wantErr     bool
		expectedErr string
	}{
		{
			name: "valid control planes",
			cps: []resources.ControlPlaneResource{
				{
					Ref: "cp1",
					CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{
						Name: "Control Plane One",
					},
				},
				{
					Ref: "cp2",
					CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{
						Name: "Control Plane Two",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate refs within same type not detected by validator",
			cps: []resources.ControlPlaneResource{
				{
					Ref: "cp1",
					CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{
						Name: "Control Plane One",
					},
				},
				{
					Ref: "cp1",
					CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{
						Name: "Control Plane Two", // Different name to avoid name duplicate error
					},
				},
			},
			wantErr:     false, // Validator doesn't check same-type duplicates anymore
			expectedErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a ResourceSet with the test control planes
			rs := &resources.ResourceSet{
				ControlPlanes: tt.cps,
			}

			err := loader.validateControlPlanes(tt.cps, rs)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
				// Verify all control planes are in the ResourceSet
				for _, cp := range tt.cps {
					assert.True(t, rs.HasRef(cp.GetRef()))
				}
			}
		})
	}
}

func TestLoader_validateAPIs(t *testing.T) {
	loader := New()

	tests := []struct {
		name        string
		apis        []resources.APIResource
		wantErr     bool
		expectedErr string
	}{
		{
			name: "valid APIs",
			apis: []resources.APIResource{
				{
					Ref: "api1",
					CreateAPIRequest: kkComps.CreateAPIRequest{
						Name: "API One",
					},
				},
				{
					Ref: "api2",
					CreateAPIRequest: kkComps.CreateAPIRequest{
						Name: "API Two",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate API refs within same type not detected by validator",
			apis: []resources.APIResource{
				{
					Ref: "api1",
					CreateAPIRequest: kkComps.CreateAPIRequest{
						Name: "API One",
					},
				},
				{
					Ref: "api1",
					CreateAPIRequest: kkComps.CreateAPIRequest{
						Name: "API Two", // Different name to avoid name duplicate error
					},
				},
			},
			wantErr:     false, // Validator doesn't check same-type duplicates anymore
			expectedErr: "",
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
			expectedErr: "duplicate ref 'v1' (already defined as api_version)",
		},
		{
			name: "API with duplicate publication refs",
			apis: []resources.APIResource{
				{
					Ref: "api1",
					Publications: []resources.APIPublicationResource{
						{Ref: "pub1", PortalID: "dummy-portal"}, // Use dummy value for required field
						{Ref: "pub1", PortalID: "dummy-portal"},
					},
				},
			},
			wantErr:     true,
			expectedErr: "duplicate ref 'pub1' (already defined as api_publication)",
		},
		{
			name: "API with duplicate implementation refs",
			apis: []resources.APIResource{
				{
					Ref: "api1",
					Implementations: []resources.APIImplementationResource{
						{
							Ref: "impl1",
							APIImplementation: kkComps.APIImplementation{
								Service: &kkComps.APIImplementationService{
									ID:             "12345678-1234-1234-1234-123456789012",
									ControlPlaneID: "dummy-cp", // Use dummy value for required field
								},
							},
						},
						{
							Ref: "impl1",
							APIImplementation: kkComps.APIImplementation{
								Service: &kkComps.APIImplementationService{
									ID:             "12345678-1234-1234-1234-123456789012",
									ControlPlaneID: "dummy-cp",
								},
							},
						},
					},
				},
			},
			wantErr:     true,
			expectedErr: "duplicate ref 'impl1' (already defined as api_implementation)",
		},
		{
			name: "API with multiple versions - should pass",
			apis: []resources.APIResource{
				{
					Ref: "api1",
					CreateAPIRequest: kkComps.CreateAPIRequest{
						Name: "API One",
					},
					Versions: []resources.APIVersionResource{
						{
							Ref: "v1",
							CreateAPIVersionRequest: kkComps.CreateAPIVersionRequest{
								Version: stringPtr("v1.0"),
							},
						},
						{
							Ref: "v2",
							CreateAPIVersionRequest: kkComps.CreateAPIVersionRequest{
								Version: stringPtr("v2.0"),
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "API with single version - should pass",
			apis: []resources.APIResource{
				{
					Ref: "api1",
					CreateAPIRequest: kkComps.CreateAPIRequest{
						Name: "API One",
					},
					Versions: []resources.APIVersionResource{
						{
							Ref: "v1",
							CreateAPIVersionRequest: kkComps.CreateAPIVersionRequest{
								Version: stringPtr("v1.0"),
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a ResourceSet with the test APIs
			rs := &resources.ResourceSet{
				APIs: tt.apis,
			}

			// Add dummy resources for cross-reference validation
			// These are needed because validateCrossReferences checks that references exist
			rs.Portals = []resources.PortalResource{
				{Ref: "dummy-portal"},
			}
			rs.ControlPlanes = []resources.ControlPlaneResource{
				{Ref: "dummy-cp"},
			}

			// Extract nested resources to match real loader behavior
			// This moves versions, publications, and implementations to top-level arrays
			extractNestedResourcesForTest(rs)

			// Now validate the entire ResourceSet (not just APIs)
			// This will check both the APIs and the extracted child resources
			err := loader.validateResourceSet(rs)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
				// Verify all APIs are in the ResourceSet
				for _, api := range tt.apis {
					assert.True(t, rs.HasRef(api.GetRef()))
				}
			}
		})
	}
}

func TestLoader_validateCrossReferences(t *testing.T) {
	loader := New()

	// Create a base ResourceSet with some resources for reference validation
	baseResources := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{Ref: "portal1"},
		},
		ApplicationAuthStrategies: []resources.ApplicationAuthStrategyResource{
			{Ref: "oauth1"},
		},
		ControlPlanes: []resources.ControlPlaneResource{
			{Ref: "cp1"},
		},
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
						CreatePortal: kkComps.CreatePortal{
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
						CreatePortal: kkComps.CreatePortal{
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
			// Merge test resources with base resources for reference validation
			fullRS := &resources.ResourceSet{
				Portals:                   append(baseResources.Portals, tt.rs.Portals...),
				ApplicationAuthStrategies: baseResources.ApplicationAuthStrategies,
				ControlPlanes:             baseResources.ControlPlanes,
				APIs:                      tt.rs.APIs,
				APIPublications:           tt.rs.APIPublications,
				APIImplementations:        tt.rs.APIImplementations,
			}

			err := loader.validateCrossReferences(fullRS)
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
	loader := New()

	// Test with a mock struct that has various field types
	type TestStruct struct {
		SimpleField  string `yaml:"simple_field"`
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
	loader := New()

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

// extractNestedResourcesForTest extracts nested resources to match real loader behavior
// This mimics what happens in loader.extractNestedResources during parseYAML
func extractNestedResourcesForTest(rs *resources.ResourceSet) {
	// Extract nested API child resources
	for i := range rs.APIs {
		api := &rs.APIs[i]

		// Extract versions
		for _, v := range api.Versions {
			v.API = api.Ref
			rs.APIVersions = append(rs.APIVersions, v)
		}
		api.Versions = nil

		// Extract publications
		for _, p := range api.Publications {
			p.API = api.Ref
			rs.APIPublications = append(rs.APIPublications, p)
		}
		api.Publications = nil

		// Extract implementations
		for _, impl := range api.Implementations {
			impl.API = api.Ref
			rs.APIImplementations = append(rs.APIImplementations, impl)
		}
		api.Implementations = nil
	}
}

// Helper function for tests
func stringPtr(s string) *string {
	return &s
}
