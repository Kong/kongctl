package loader

import (
	"reflect"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
					BaseResource: resources.BaseResource{
						Ref: "portal1",
					},
					CreatePortal: kkComps.CreatePortal{
						Name: "Portal One",
					},
				},
				{
					BaseResource: resources.BaseResource{
						Ref: "portal2",
					},
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
					BaseResource: resources.BaseResource{
						Ref: "portal1",
					},
					CreatePortal: kkComps.CreatePortal{
						Name: "Portal One",
					},
				},
				{
					BaseResource: resources.BaseResource{
						Ref: "portal1",
					},
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
				{
					BaseResource: resources.BaseResource{
						Ref: "",
					},
				},
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
					BaseResource: resources.BaseResource{Ref: "oauth1"},
					CreateAppAuthStrategyRequest: kkComps.CreateAppAuthStrategyRequest{
						Type: kkComps.CreateAppAuthStrategyRequestTypeKeyAuth,
						AppAuthStrategyKeyAuthRequest: &kkComps.AppAuthStrategyKeyAuthRequest{
							Name: "Key Auth One",
						},
					},
				},
				{
					BaseResource: resources.BaseResource{Ref: "oauth2"},
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
					BaseResource: resources.BaseResource{Ref: "oauth1"},
					CreateAppAuthStrategyRequest: kkComps.CreateAppAuthStrategyRequest{
						Type: kkComps.CreateAppAuthStrategyRequestTypeKeyAuth,
						AppAuthStrategyKeyAuthRequest: &kkComps.AppAuthStrategyKeyAuthRequest{
							Name: "Key Auth One",
						},
					},
				},
				{
					BaseResource: resources.BaseResource{Ref: "oauth1"},
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
					BaseResource: resources.BaseResource{
						Ref: "cp1",
					},
					CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{
						Name: "Control Plane One",
					},
				},
				{
					BaseResource: resources.BaseResource{
						Ref: "cp2",
					},
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
					BaseResource: resources.BaseResource{
						Ref: "cp1",
					},
					CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{
						Name: "Control Plane One",
					},
				},
				{
					BaseResource: resources.BaseResource{
						Ref: "cp1",
					},
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
					BaseResource: resources.BaseResource{Ref: "api1"},
					CreateAPIRequest: kkComps.CreateAPIRequest{
						Name: "API One",
					},
				},
				{
					BaseResource: resources.BaseResource{Ref: "api2"},
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
					BaseResource: resources.BaseResource{Ref: "api1"},
					CreateAPIRequest: kkComps.CreateAPIRequest{
						Name: "API One",
					},
				},
				{
					BaseResource: resources.BaseResource{Ref: "api1"},
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
					BaseResource: resources.BaseResource{Ref: "api1"},
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
					BaseResource: resources.BaseResource{Ref: "api1"},
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
					BaseResource: resources.BaseResource{Ref: "api1"},
					Implementations: []resources.APIImplementationResource{
						{
							Ref: "impl1",
							APIImplementation: kkComps.APIImplementation{
								ServiceReferenceInput: &kkComps.ServiceReferenceInput{
									Service: &kkComps.APIImplementationService{
										ID:             "12345678-1234-1234-1234-123456789012",
										ControlPlaneID: "dummy-cp", // Use dummy value for required field
									},
								},
								Type: kkComps.APIImplementationTypeServiceReferenceInput,
							},
						},
						{
							Ref: "impl1",
							APIImplementation: kkComps.APIImplementation{
								ServiceReferenceInput: &kkComps.ServiceReferenceInput{
									Service: &kkComps.APIImplementationService{
										ID:             "12345678-1234-1234-1234-123456789012",
										ControlPlaneID: "dummy-cp",
									},
								},
								Type: kkComps.APIImplementationTypeServiceReferenceInput,
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
					BaseResource: resources.BaseResource{
						Ref: "api1",
					},
					CreateAPIRequest: kkComps.CreateAPIRequest{
						Name: "API One",
					},
					Versions: []resources.APIVersionResource{
						{
							Ref: "v1",
							CreateAPIVersionRequest: kkComps.CreateAPIVersionRequest{
								Version: new("v1.0"),
							},
						},
						{
							Ref: "v2",
							CreateAPIVersionRequest: kkComps.CreateAPIVersionRequest{
								Version: new("v2.0"),
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
					BaseResource: resources.BaseResource{
						Ref: "api1",
					},
					CreateAPIRequest: kkComps.CreateAPIRequest{
						Name: "API One",
					},
					Versions: []resources.APIVersionResource{
						{
							Ref: "v1",
							CreateAPIVersionRequest: kkComps.CreateAPIVersionRequest{
								Version: new("v1.0"),
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
				{BaseResource: resources.BaseResource{
					Ref: "dummy-portal",
				}},
			}
			rs.ControlPlanes = []resources.ControlPlaneResource{
				{BaseResource: resources.BaseResource{
					Ref: "dummy-cp",
				}},
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

func TestLoaderValidateAIGatewayProvidersRequiresParent(t *testing.T) {
	loader := New()
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{Ref: "ai-gateway"},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				Name:        "ai-gateway",
				DisplayName: "AI Gateway",
			},
		}},
		AIGatewayProviders: []resources.AIGatewayProviderResource{{
			BaseResource: resources.BaseResource{Ref: "openai-provider"},
			Name:         "openai-provider",
			Type:         "openai",
			DisplayName:  "OpenAI Provider",
			Config:       map[string]any{"auth": map[string]any{"type": "basic"}},
		}},
	}

	err := loader.validateResourceSet(rs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must specify ai_gateway")
}

func TestLoaderValidateAIGatewayProvidersRejectsDuplicateNamesPerGateway(t *testing.T) {
	loader := New()
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{Ref: "ai-gateway"},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				Name:        "ai-gateway",
				DisplayName: "AI Gateway",
			},
		}},
		AIGatewayProviders: []resources.AIGatewayProviderResource{
			{
				BaseResource: resources.BaseResource{Ref: "openai-provider-1"},
				AIGateway:    "ai-gateway",
				Name:         "openai-provider",
				Type:         "openai",
				DisplayName:  "OpenAI Provider",
				Config:       map[string]any{"auth": map[string]any{"type": "basic"}},
			},
			{
				BaseResource: resources.BaseResource{Ref: "openai-provider-2"},
				AIGateway:    "ai-gateway",
				Name:         "openai-provider",
				Type:         "openai",
				DisplayName:  "OpenAI Provider Duplicate",
				Config:       map[string]any{"auth": map[string]any{"type": "basic"}},
			},
		},
	}

	err := loader.validateResourceSet(rs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate ai_gateway_model_provider name")
}

func TestLoaderValidateAIGatewayIdentityProvidersRequiresParent(t *testing.T) {
	loader := New()
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{Ref: "ai-gateway"},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				Name:        "ai-gateway",
				DisplayName: "AI Gateway",
			},
		}},
		AIGatewayIdentityProviders: []resources.AIGatewayIdentityProviderResource{{
			BaseResource: resources.BaseResource{Ref: "support-key-auth"},
			Name:         "support-key-auth",
			Type:         "key-auth",
			DisplayName:  "Support Key Auth",
			Config:       map[string]any{"key_names": []any{"apikey"}},
		}},
	}

	err := loader.validateResourceSet(rs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must specify ai_gateway")
}

func TestLoaderValidateAIGatewayIdentityProvidersRejectsDuplicateNamesPerGateway(t *testing.T) {
	loader := New()
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{Ref: "ai-gateway"},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				Name:        "ai-gateway",
				DisplayName: "AI Gateway",
			},
		}},
		AIGatewayIdentityProviders: []resources.AIGatewayIdentityProviderResource{
			{
				BaseResource: resources.BaseResource{Ref: "support-key-auth-1"},
				AIGateway:    "ai-gateway",
				Name:         "support-key-auth",
				Type:         "key-auth",
				DisplayName:  "Support Key Auth",
				Config:       map[string]any{"key_names": []any{"apikey"}},
			},
			{
				BaseResource: resources.BaseResource{Ref: "support-key-auth-2"},
				AIGateway:    "ai-gateway",
				Name:         "support-key-auth",
				Type:         "key-auth",
				DisplayName:  "Support Key Auth Duplicate",
				Config:       map[string]any{"key_names": []any{"apikey"}},
			},
		},
	}

	err := loader.validateResourceSet(rs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate ai_gateway_identity_provider name")
}

func TestLoader_validateCrossReferences(t *testing.T) {
	loader := New()

	// Create a base ResourceSet with some resources for reference validation
	baseResources := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				BaseResource: resources.BaseResource{
					Ref: "portal1",
				},
			},
		},
		ApplicationAuthStrategies: []resources.ApplicationAuthStrategyResource{
			{
				BaseResource: resources.BaseResource{
					Ref: "oauth1",
				},
			},
		},
		ControlPlanes: []resources.ControlPlaneResource{
			{
				BaseResource: resources.BaseResource{
					Ref: "cp1",
				},
			},
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
						BaseResource: resources.BaseResource{
							Ref: "portal1",
						},
						CreatePortal: kkComps.CreatePortal{
							DefaultApplicationAuthStrategyID: new("oauth1"),
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
						BaseResource: resources.BaseResource{
							Ref: "portal1",
						},
						CreatePortal: kkComps.CreatePortal{
							DefaultApplicationAuthStrategyID: new("nonexistent"),
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
						BaseResource: resources.BaseResource{
							Ref: "portal1",
						},
						// No default auth strategy - should be fine
					},
				},
			},
			wantErr: false,
		},
		{
			name: "raw UUID reference should be accepted without local ref match",
			rs: &resources.ResourceSet{
				Portals: []resources.PortalResource{
					{
						BaseResource: resources.BaseResource{
							Ref: "portal1",
						},
						CreatePortal: kkComps.CreatePortal{
							// A raw UUID is an already-resolved Konnect resource ID;
							// it cannot be matched against local refs and must be allowed.
							DefaultApplicationAuthStrategyID: new("5cb6138c-fcf4-4db6-8972-756262d743ac"),
						},
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

func TestLoader_validateOrganizationTeamRoles_ValidatesReferences(t *testing.T) {
	loader := New()

	tests := []struct {
		name        string
		rs          *resources.ResourceSet
		expectError string
	}{
		{
			name: "valid team and api refs",
			rs: &resources.ResourceSet{
				OrganizationTeams: []resources.OrganizationTeamResource{
					{
						BaseResource: resources.BaseResource{Ref: "platform-team"},
						CreateTeam:   kkComps.CreateTeam{Name: "Platform"},
					},
				},
				APIs: []resources.APIResource{
					{BaseResource: resources.BaseResource{Ref: "products-api"}},
				},
				OrganizationTeamRoles: []resources.OrganizationTeamRoleResource{
					{
						Ref:            "platform-admin",
						Team:           "platform-team",
						RoleName:       "Admin",
						EntityID:       tags.RefPlaceholderPrefix + "products-api#id",
						EntityTypeName: "APIs",
						EntityRegion:   "us",
					},
				},
			},
		},
		{
			name: "valid team and portal refs",
			rs: &resources.ResourceSet{
				OrganizationTeams: []resources.OrganizationTeamResource{
					{
						BaseResource: resources.BaseResource{Ref: "platform-team"},
						CreateTeam:   kkComps.CreateTeam{Name: "Platform"},
					},
				},
				Portals: []resources.PortalResource{
					{
						BaseResource: resources.BaseResource{Ref: "developer-portal"},
						CreatePortal: kkComps.CreatePortal{Name: "Developer Portal"},
					},
				},
				OrganizationTeamRoles: []resources.OrganizationTeamRoleResource{
					{
						Ref:            "platform-portal-viewer",
						Team:           "platform-team",
						RoleName:       "Viewer",
						EntityID:       tags.RefPlaceholderPrefix + "developer-portal#id",
						EntityTypeName: "Portals",
						EntityRegion:   "us",
					},
				},
			},
		},
		{
			name: "unknown team ref",
			rs: &resources.ResourceSet{
				OrganizationTeamRoles: []resources.OrganizationTeamRoleResource{
					{
						Ref:            "platform-admin",
						Team:           "missing-team",
						RoleName:       "Admin",
						EntityID:       "*",
						EntityTypeName: "APIs",
						EntityRegion:   "us",
					},
				},
			},
			expectError: `organization_team_role "platform-admin" references unknown organization_team: missing-team`,
		},
		{
			name: "unknown entity api ref",
			rs: &resources.ResourceSet{
				OrganizationTeams: []resources.OrganizationTeamResource{
					{
						BaseResource: resources.BaseResource{Ref: "platform-team"},
						CreateTeam:   kkComps.CreateTeam{Name: "Platform"},
					},
				},
				OrganizationTeamRoles: []resources.OrganizationTeamRoleResource{
					{
						Ref:            "platform-admin",
						Team:           "platform-team",
						RoleName:       "Admin",
						EntityID:       tags.RefPlaceholderPrefix + "missing-api#id",
						EntityTypeName: "APIs",
						EntityRegion:   "us",
					},
				},
			},
			expectError: `organization_team_role "platform-admin" references unknown api: missing-api (field: entity_id)`,
		},
		{
			name: "entity type mismatch",
			rs: &resources.ResourceSet{
				OrganizationTeams: []resources.OrganizationTeamResource{
					{
						BaseResource: resources.BaseResource{Ref: "platform-team"},
						CreateTeam:   kkComps.CreateTeam{Name: "Platform"},
					},
				},
				APIs: []resources.APIResource{
					{BaseResource: resources.BaseResource{Ref: "products-api"}},
				},
				OrganizationTeamRoles: []resources.OrganizationTeamRoleResource{
					{
						Ref:            "platform-admin",
						Team:           "platform-team",
						RoleName:       "Admin",
						EntityID:       tags.RefPlaceholderPrefix + "products-api#id",
						EntityTypeName: "Portals",
						EntityRegion:   "us",
					},
				},
			},
			expectError: `organization_team_role "platform-admin" references api but expected portal: ` +
				`products-api (field: entity_id)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := loader.validateOrganizationTeamRoles(tt.rs.OrganizationTeamRoles, tt.rs)
			if tt.expectError == "" {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestLoader_validateOrganizationUserRole_AllowsPortalEntityRef(t *testing.T) {
	loader := New()
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				BaseResource: resources.BaseResource{Ref: "developer-portal"},
				CreatePortal: kkComps.CreatePortal{Name: "Developer Portal"},
			},
		},
	}
	role := &resources.OrganizationUserRoleResource{
		Ref:            "alice-portal-viewer",
		User:           "alice",
		RoleName:       "Viewer",
		EntityID:       tags.RefPlaceholderPrefix + "developer-portal#id",
		EntityTypeName: "Portals",
		EntityRegion:   "us",
	}

	require.NoError(t, loader.validateUserRoleEntityReference(role, rs))
}

func TestLoader_validateOrganizationSystemAccountRole_AllowsPortalEntityRef(t *testing.T) {
	loader := New()
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				BaseResource: resources.BaseResource{Ref: "developer-portal"},
				CreatePortal: kkComps.CreatePortal{Name: "Developer Portal"},
			},
		},
	}
	role := &resources.OrganizationSystemAccountRoleResource{
		Ref:            "ci-bot-portal-viewer",
		SystemAccount:  "ci-bot",
		RoleName:       "Viewer",
		EntityID:       tags.RefPlaceholderPrefix + "developer-portal#id",
		EntityTypeName: "Portals",
		EntityRegion:   "us",
	}

	require.NoError(t, loader.validateSystemAccountRoleEntityReference(role, rs))
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

func TestLoader_validateResourceSet_RejectsDeprecatedPortalAuthSettingsFields(t *testing.T) {
	loader := New()
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{{
			BaseResource: resources.BaseResource{Ref: "portal-1"},
			CreatePortal: kkComps.CreatePortal{Name: "portal-one"},
		}},
		PortalAuthSettings: []resources.PortalAuthSettingsResource{{
			Ref:    "portal-auth-settings",
			Portal: "portal-1",
			PortalAuthenticationSettingsUpdateRequest: kkComps.PortalAuthenticationSettingsUpdateRequest{
				OidcAuthEnabled: new(true),
			},
		}},
	}

	err := loader.validateResourceSet(rs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "uses deprecated field \"oidc_auth_enabled\"")
	assert.Contains(t, err.Error(), "move identity provider configuration to identity_providers")
}

func TestLoader_validateResourceSet_RejectsDuplicatePortalIdentityProviderTypesPerPortal(t *testing.T) {
	loader := New()
	configA := kkComps.CreateCreateIdentityProviderConfigOIDCIdentityProviderConfig(kkComps.OIDCIdentityProviderConfig{
		IssuerURL: "https://accounts.google.com",
		ClientID:  "client-id-a",
	})
	configB := kkComps.CreateCreateIdentityProviderConfigOIDCIdentityProviderConfig(kkComps.OIDCIdentityProviderConfig{
		IssuerURL: "https://login.microsoftonline.com/common/v2.0",
		ClientID:  "client-id-b",
	})
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{{
			BaseResource: resources.BaseResource{Ref: "portal-1"},
			CreatePortal: kkComps.CreatePortal{Name: "portal-one"},
		}},
		PortalIdentityProviders: []resources.PortalIdentityProviderResource{
			{
				Ref:    "portal-oidc-a",
				Portal: "portal-1",
				CreateIdentityProvider: kkComps.CreateIdentityProvider{
					Type:   kkComps.IdentityProviderTypeOidc.ToPointer(),
					Config: &configA,
				},
			},
			{
				Ref:    "portal-oidc-b",
				Portal: "portal-1",
				CreateIdentityProvider: kkComps.CreateIdentityProvider{
					Type:   kkComps.IdentityProviderTypeOidc.ToPointer(),
					Config: &configB,
				},
			},
		},
	}

	err := loader.validateResourceSet(rs)
	assert.Error(t, err)
	assert.Contains(
		t,
		err.Error(),
		"multiple portal_identity_provider entries target portal \"portal-1\" and type \"oidc\"",
	)
}

func TestLoader_validateResourceSet_RejectsDuplicatePortalIdentityProviderSAMLTypesPerPortal(t *testing.T) {
	loader := New()
	configA := kkComps.CreateCreateIdentityProviderConfigSAMLIdentityProviderConfigInput(
		kkComps.SAMLIdentityProviderConfigInput{
			IdpMetadataURL: new("https://example-a.test/saml.xml"),
		},
	)
	configB := kkComps.CreateCreateIdentityProviderConfigSAMLIdentityProviderConfigInput(
		kkComps.SAMLIdentityProviderConfigInput{
			IdpMetadataURL: new("https://example-b.test/saml.xml"),
		},
	)
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{{
			BaseResource: resources.BaseResource{Ref: "portal-1"},
			CreatePortal: kkComps.CreatePortal{Name: "portal-one"},
		}},
		PortalIdentityProviders: []resources.PortalIdentityProviderResource{
			{
				Ref:    "portal-saml-a",
				Portal: "portal-1",
				CreateIdentityProvider: kkComps.CreateIdentityProvider{
					Type:   kkComps.IdentityProviderTypeSaml.ToPointer(),
					Config: &configA,
				},
			},
			{
				Ref:    "portal-saml-b",
				Portal: "portal-1",
				CreateIdentityProvider: kkComps.CreateIdentityProvider{
					Type:   kkComps.IdentityProviderTypeSaml.ToPointer(),
					Config: &configB,
				},
			},
		},
	}

	err := loader.validateResourceSet(rs)
	assert.Error(t, err)
	assert.Contains(
		t,
		err.Error(),
		"multiple portal_identity_provider entries target portal \"portal-1\" and type \"saml\"",
	)
}

func TestLoader_validateResourceSet_AllowsMixedPortalIdentityProviderTypesPerPortal(t *testing.T) {
	loader := New()
	oidcConfig := kkComps.CreateCreateIdentityProviderConfigOIDCIdentityProviderConfig(
		kkComps.OIDCIdentityProviderConfig{
			IssuerURL: "https://accounts.google.com",
			ClientID:  "client-id-a",
		},
	)
	samlConfig := kkComps.CreateCreateIdentityProviderConfigSAMLIdentityProviderConfigInput(
		kkComps.SAMLIdentityProviderConfigInput{
			IdpMetadataURL: new("https://example.test/saml.xml"),
		},
	)
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{{
			BaseResource: resources.BaseResource{Ref: "portal-1"},
			CreatePortal: kkComps.CreatePortal{Name: "portal-one"},
		}},
		PortalIdentityProviders: []resources.PortalIdentityProviderResource{
			{
				Ref:    "portal-oidc",
				Portal: "portal-1",
				CreateIdentityProvider: kkComps.CreateIdentityProvider{
					Type:   kkComps.IdentityProviderTypeOidc.ToPointer(),
					Config: &oidcConfig,
				},
			},
			{
				Ref:    "portal-saml",
				Portal: "portal-1",
				CreateIdentityProvider: kkComps.CreateIdentityProvider{
					Type:   kkComps.IdentityProviderTypeSaml.ToPointer(),
					Config: &samlConfig,
				},
			},
		},
	}

	err := loader.validateResourceSet(rs)
	assert.NoError(t, err)
}

func TestLoader_validateResourceSet_RejectsDuplicatePortalIPAllowListsPerPortal(t *testing.T) {
	loader := New()
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{{
			BaseResource: resources.BaseResource{Ref: "portal-1"},
			CreatePortal: kkComps.CreatePortal{Name: "portal-one"},
		}},
		PortalIPAllowLists: []resources.PortalIPAllowListResource{
			{
				Ref:        "portal-allow-list-a",
				Portal:     "portal-1",
				AllowedIPs: []string{"192.0.2.10"},
			},
			{
				Ref:        "portal-allow-list-b",
				Portal:     "portal-1",
				AllowedIPs: []string{"198.51.100.0/24"},
			},
		},
	}

	err := loader.validateResourceSet(rs)
	assert.Error(t, err)
	assert.Contains(
		t,
		err.Error(),
		"multiple portal_ip_allow_list entries target portal \"portal-1\"",
	)
}
