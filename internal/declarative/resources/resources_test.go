package resources

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/util"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

func TestPortalResource_Validation(t *testing.T) {
	tests := []struct {
		name    string
		portal  PortalResource
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid portal with ref",
			portal: PortalResource{
				Ref: "test-portal",
			},
			wantErr: false,
		},
		{
			name:   "missing ref",
			portal: PortalResource{
				// No ref field
			},
			wantErr: true,
			errMsg:  "invalid portal ref: ref cannot be empty",
		},
		{
			name: "empty ref",
			portal: PortalResource{
				Ref: "",
			},
			wantErr: true,
			errMsg:  "invalid portal ref: ref cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.portal.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPortalResource_SetDefaults(t *testing.T) {
	tests := []struct {
		name         string
		portal       PortalResource
		expectedName string
	}{
		{
			name: "name from ref when name is empty",
			portal: PortalResource{
				Ref: "my-portal",
			},
			expectedName: "my-portal",
		},
		{
			name: "existing name is preserved",
			portal: PortalResource{
				Ref: "my-portal",
				CreatePortal: kkComps.CreatePortal{
					Name: "Existing Portal Name",
				},
			},
			expectedName: "Existing Portal Name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			portal := tt.portal
			portal.SetDefaults()
			assert.Equal(t, tt.expectedName, portal.Name)
		})
	}
}

func TestPortalResource_GetRef(t *testing.T) {
	portal := PortalResource{
		Ref: "test-portal-ref",
	}
	assert.Equal(t, "test-portal-ref", portal.GetRef())
}

func TestApplicationAuthStrategyResource_Validation(t *testing.T) {
	tests := []struct {
		name     string
		strategy ApplicationAuthStrategyResource
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid auth strategy",
			strategy: ApplicationAuthStrategyResource{
				Ref: "oauth-strategy",
			},
			wantErr: false,
		},
		{
			name:     "missing ref",
			strategy: ApplicationAuthStrategyResource{
				// No ref field
			},
			wantErr: true,
			errMsg:  "invalid application auth strategy ref: ref cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.strategy.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestControlPlaneResource_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cp      ControlPlaneResource
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid control plane",
			cp: ControlPlaneResource{
				Ref: "test-cp",
			},
			wantErr: false,
		},
		{
			name: "missing ref",
			cp:   ControlPlaneResource{
				// No ref field
			},
			wantErr: true,
			errMsg:  "invalid control plane ref: ref cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cp.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAPIResource_Validation(t *testing.T) {
	tests := []struct {
		name    string
		api     APIResource
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid api",
			api: APIResource{
				Ref: "test-api",
			},
			wantErr: false,
		},
		{
			name: "missing ref",
			api:  APIResource{
				// No ref field
			},
			wantErr: true,
			errMsg:  "invalid API ref: ref cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.api.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAPIVersionResource_Validation(t *testing.T) {
	tests := []struct {
		name    string
		version APIVersionResource
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid api version",
			version: APIVersionResource{
				Ref: "api-v1",
			},
			wantErr: false,
		},
		{
			name:    "missing ref",
			version: APIVersionResource{
				// No ref field
			},
			wantErr: true,
			errMsg:  "invalid API version ref: ref cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.version.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAPIPublicationResource_Validation(t *testing.T) {
	tests := []struct {
		name        string
		publication APIPublicationResource
		wantErr     bool
		errMsg      string
	}{
		{
			name: "valid api publication",
			publication: APIPublicationResource{
				Ref:      "api-pub-1",
				PortalID: "portal-ref",
			},
			wantErr: false,
		},
		{
			name: "missing ref",
			publication: APIPublicationResource{
				PortalID: "portal-ref",
			},
			wantErr: true,
			errMsg:  "invalid API publication ref: ref cannot be empty",
		},
		{
			name: "missing portal_id",
			publication: APIPublicationResource{
				Ref: "api-pub-1",
			},
			wantErr: true,
			errMsg:  "API publication portal_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.publication.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAPIImplementationResource_Validation(t *testing.T) {
	tests := []struct {
		name           string
		implementation APIImplementationResource
		wantErr        bool
		errMsg         string
	}{
		{
			name: "valid api implementation without service",
			implementation: APIImplementationResource{
				Ref: "api-impl-1",
			},
			wantErr: false,
		},
		{
			name: "valid api implementation with service and reference control_plane_id",
			implementation: APIImplementationResource{
				Ref: "api-impl-1",
				APIImplementation: kkComps.APIImplementation{
					Service: &kkComps.APIImplementationService{
						ID:             "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
						ControlPlaneID: "prod-cp", // Reference to declarative control plane
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid api implementation with service and UUID control_plane_id",
			implementation: APIImplementationResource{
				Ref: "api-impl-1",
				APIImplementation: kkComps.APIImplementation{
					Service: &kkComps.APIImplementationService{
						ID:             "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
						ControlPlaneID: "f9e8d7c6-b5a4-3210-9876-fedcba098765", // External UUID
					},
				},
			},
			wantErr: false,
		},
		{
			name:           "missing ref",
			implementation: APIImplementationResource{
				// No ref field
			},
			wantErr: true,
			errMsg:  "invalid API implementation ref: ref cannot be empty",
		},
		{
			name: "service with missing id",
			implementation: APIImplementationResource{
				Ref: "api-impl-1",
				APIImplementation: kkComps.APIImplementation{
					Service: &kkComps.APIImplementationService{
						ControlPlaneID: "prod-cp",
					},
				},
			},
			wantErr: true,
			errMsg:  "API implementation service.id is required",
		},
		{
			name: "service with id referencing gateway service resource",
			implementation: APIImplementationResource{
				Ref: "api-impl-1",
				APIImplementation: kkComps.APIImplementation{
					Service: &kkComps.APIImplementationService{
						ID:             "not-a-uuid",
						ControlPlaneID: "prod-cp",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "service with missing control_plane_id",
			implementation: APIImplementationResource{
				Ref: "api-impl-1",
				APIImplementation: kkComps.APIImplementation{
					Service: &kkComps.APIImplementationService{
						ID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
					},
				},
			},
			wantErr: true,
			errMsg:  "API implementation service.control_plane_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.implementation.Validate()
			if tt.wantErr {
				if assert.Error(t, err) && tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestReferenceFieldMappings(t *testing.T) {
	t.Run("PortalResource reference mappings", func(t *testing.T) {
		portal := PortalResource{}
		mappings := portal.GetReferenceFieldMappings()

		// Portal references auth strategies
		expectedType, exists := mappings["default_application_auth_strategy_id"]
		assert.True(t, exists, "Should have default_application_auth_strategy_id mapping")
		assert.Equal(t, "application_auth_strategy", expectedType, "Should map to application_auth_strategy type")
	})

	t.Run("APIPublicationResource reference mappings", func(t *testing.T) {
		publication := APIPublicationResource{}
		mappings := publication.GetReferenceFieldMappings()

		// Check expected mappings
		expectedMappings := map[string]string{
			"portal_id":         "portal",
			"auth_strategy_ids": "application_auth_strategy",
		}

		for field, expectedType := range expectedMappings {
			actualType, exists := mappings[field]
			assert.True(t, exists, "Should have %s mapping", field)
			assert.Equal(t, expectedType, actualType, "Field %s should map to %s", field, expectedType)
		}
	})

	t.Run("APIImplementationResource reference mappings", func(t *testing.T) {
		// Test with reference control_plane_id
		t.Run("with reference control_plane_id", func(t *testing.T) {
			implementation := APIImplementationResource{
				APIImplementation: kkComps.APIImplementation{
					Service: &kkComps.APIImplementationService{
						ControlPlaneID: "prod-cp", // Not a UUID
					},
				},
			}
			mappings := implementation.GetReferenceFieldMappings()

			// Should include the mapping
			actualType, exists := mappings["service.control_plane_id"]
			assert.True(t, exists, "Should have service.control_plane_id mapping for reference")
			assert.Equal(t, "control_plane", actualType)
		})

		// Test with UUID control_plane_id
		t.Run("with UUID control_plane_id", func(t *testing.T) {
			implementation := APIImplementationResource{
				APIImplementation: kkComps.APIImplementation{
					Service: &kkComps.APIImplementationService{
						ControlPlaneID: "f9e8d7c6-b5a4-3210-9876-fedcba098765", // UUID
					},
				},
			}
			mappings := implementation.GetReferenceFieldMappings()

			// Should NOT include the mapping
			_, exists := mappings["service.control_plane_id"]
			assert.False(t, exists, "Should not have service.control_plane_id mapping for UUID")
		})

		// Test with no service
		t.Run("with no service", func(t *testing.T) {
			implementation := APIImplementationResource{}
			mappings := implementation.GetReferenceFieldMappings()

			// Should have empty mappings
			assert.Empty(t, mappings, "Should have no mappings without service")
		})

		// Test with empty control_plane_id
		t.Run("with empty control_plane_id", func(t *testing.T) {
			implementation := APIImplementationResource{
				APIImplementation: kkComps.APIImplementation{
					Service: &kkComps.APIImplementationService{
						ControlPlaneID: "", // Empty
					},
				},
			}
			mappings := implementation.GetReferenceFieldMappings()

			// Should have empty mappings
			assert.Empty(t, mappings, "Should have no mappings with empty control_plane_id")
		})
	})

	t.Run("Resources with no outbound references", func(t *testing.T) {
		// Test resources that should have empty mappings
		testCases := []struct {
			name     string
			resource ReferenceMapping
		}{
			{"ApplicationAuthStrategyResource", ApplicationAuthStrategyResource{}},
			{"ControlPlaneResource", ControlPlaneResource{}},
			{"APIResource", APIResource{}},
			{"APIVersionResource", APIVersionResource{}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				mappings := tc.resource.GetReferenceFieldMappings()
				assert.Empty(t, mappings, "%s should have no outbound references", tc.name)
			})
		}
	})
}

func TestKongctlMeta(t *testing.T) {
	t.Run("KongctlMeta structure", func(t *testing.T) {
		trueVal := true
		namespace := "team-a"
		meta := &KongctlMeta{
			Protected: &trueVal,
			Namespace: &namespace,
		}

		assert.NotNil(t, meta.Protected)
		assert.True(t, *meta.Protected, "Protected field should be settable")
		assert.NotNil(t, meta.Namespace)
		assert.Equal(t, "team-a", *meta.Namespace, "Namespace field should be settable")

		// Test zero value
		var zeroMeta KongctlMeta
		assert.Nil(t, zeroMeta.Protected, "Default Protected should be nil")
		assert.Nil(t, zeroMeta.Namespace, "Default Namespace should be nil")
	})

	t.Run("KongctlMeta YAML marshaling", func(t *testing.T) {
		// Test marshaling with values
		trueVal := true
		namespace := "production"
		meta := &KongctlMeta{
			Protected: &trueVal,
			Namespace: &namespace,
		}

		data, err := yaml.Marshal(meta)
		assert.NoError(t, err, "Should marshal without error")

		expected := "namespace: production\nprotected: true\n"
		assert.Equal(t, expected, string(data), "Should marshal to expected YAML")

		// Test unmarshaling
		var unmarshaledMeta KongctlMeta
		err = yaml.Unmarshal(data, &unmarshaledMeta)
		assert.NoError(t, err, "Should unmarshal without error")
		assert.Equal(t, meta.Protected, unmarshaledMeta.Protected, "Protected should match after unmarshaling")
		assert.Equal(t, meta.Namespace, unmarshaledMeta.Namespace, "Namespace should match after unmarshaling")
	})

	t.Run("KongctlMeta omitempty behavior", func(t *testing.T) {
		// Test with zero values (should omit fields)
		meta := &KongctlMeta{}

		data, err := yaml.Marshal(meta)
		assert.NoError(t, err, "Should marshal without error")
		assert.Equal(t, "{}\n", string(data), "Should omit empty fields")

		// Test with only namespace set
		namespace := "team-b"
		meta = &KongctlMeta{
			Namespace: &namespace,
		}

		data, err = yaml.Marshal(meta)
		assert.NoError(t, err, "Should marshal without error")
		assert.Equal(t, "namespace: team-b\n", string(data), "Should only include namespace field")
	})
}

func TestResourceSet(t *testing.T) {
	t.Run("ResourceSet structure", func(t *testing.T) {
		rs := ResourceSet{
			Portals: []PortalResource{
				{Ref: "portal1"},
				{Ref: "portal2"},
			},
			ApplicationAuthStrategies: []ApplicationAuthStrategyResource{
				{Ref: "auth1"},
			},
			ControlPlanes: []ControlPlaneResource{
				{Ref: "cp1"},
			},
			APIs: []APIResource{
				{Ref: "api1"},
			},
		}

		assert.Len(t, rs.Portals, 2, "Should have 2 portals")
		assert.Len(t, rs.ApplicationAuthStrategies, 1, "Should have 1 auth strategy")
		assert.Len(t, rs.ControlPlanes, 1, "Should have 1 control plane")
		assert.Len(t, rs.APIs, 1, "Should have 1 API")

		// Test that we can access refs
		assert.Equal(t, "portal1", rs.Portals[0].GetRef())
		assert.Equal(t, "auth1", rs.ApplicationAuthStrategies[0].GetRef())
		assert.Equal(t, "cp1", rs.ControlPlanes[0].GetRef())
		assert.Equal(t, "api1", rs.APIs[0].GetRef())
	})
}

func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid UUID",
			input:    "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			expected: true,
		},
		{
			name:     "valid UUID with all lowercase",
			input:    "f9e8d7c6-b5a4-3210-9876-fedcba098765",
			expected: true,
		},
		{
			name:     "valid UUID with uppercase letters",
			input:    "A1B2C3D4-E5F6-7890-ABCD-EF1234567890",
			expected: true,
		},
		{
			name:     "invalid UUID - missing hyphens",
			input:    "a1b2c3d4e5f67890abcdef1234567890",
			expected: false,
		},
		{
			name:     "invalid UUID - wrong format",
			input:    "a1b2c3d4-e5f6-7890-abcd",
			expected: false,
		},
		{
			name:     "not a UUID - reference string",
			input:    "prod-cp",
			expected: false,
		},
		{
			name:     "not a UUID - empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "not a UUID - spaces",
			input:    "a1b2c3d4-e5f6-7890-abcd-ef1234567890 ",
			expected: false,
		},
		{
			name:     "invalid UUID - wrong segment lengths",
			input:    "a1b2c3d-e5f6-7890-abcd-ef1234567890",
			expected: false,
		},
		{
			name:     "invalid UUID - contains invalid characters",
			input:    "g1b2c3d4-e5f6-7890-abcd-ef1234567890",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := util.IsValidUUID(tt.input)
			assert.Equal(t, tt.expected, result, "util.IsValidUUID(%q) should return %v", tt.input, tt.expected)
		})
	}
}
