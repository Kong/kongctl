//go:build integration && disabled
// +build integration,disabled

package declarative_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/executor"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestErrorConditionsAndEdgeCases tests comprehensive error handling and edge cases
func TestErrorConditionsAndEdgeCases(t *testing.T) {
	t.Run("malformed YAML handling", func(t *testing.T) {
		tempDir := t.TempDir()

		malformedConfigs := []struct {
			name    string
			content string
			error   string
		}{
			{
				name: "invalid YAML syntax",
				content: `
portals:
  - ref: test-portal
    name: "Test Portal"
    invalid_yaml: [unclosed bracket
`,
				error: "failed to parse YAML",
			},
			{
				name: "invalid indentation",
				content: `
portals:
- ref: test-portal
  name: "Test Portal"
 description: "Invalid indentation"
`,
				error: "found character that cannot start any token",
			},
			{
				name: "missing required fields",
				content: `
portals:
  - name: "Portal without ref"
    description: "Missing ref field"
`,
				error: "portal ref is required",
			},
			{
				name: "duplicate refs",
				content: `
portals:
  - ref: duplicate-portal
    name: "Portal 1"
  - ref: duplicate-portal
    name: "Portal 2"
`,
				error: "duplicate portal ref",
			},
		}

		for _, tc := range malformedConfigs {
			t.Run(tc.name, func(t *testing.T) {
				configFile := filepath.Join(tempDir, "malformed.yaml")
				require.NoError(t, os.WriteFile(configFile, []byte(tc.content), 0o600))

				l := loader.New()
				sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}

				_, err := l.LoadFromSources(sources, false)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.error)
			})
		}
	})

	t.Run("cross-resource validation errors", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test invalid cross-references
		invalidRefs := []struct {
			name    string
			content string
			error   string
		}{
			{
				name: "nonexistent portal reference",
				content: `
api_publications:
  - ref: invalid-pub
    api: test-api
    portal_id: nonexistent-portal
    visibility: public

apis:
  - ref: test-api
    name: "Test API"
    description: "Test"
    version: "1.0.0"
`,
				error: "references unknown portal: nonexistent-portal",
			},
			{
				name: "nonexistent API reference",
				content: `
api_versions:
  - ref: invalid-version
    api: nonexistent-api
    name: "v1"
    gateway_service:
      control_plane_id: "550e8400-e29b-41d4-a716-446655440000"
      id: "550e8400-e29b-41d4-a716-446655440001"
`,
				error: "references unknown api: nonexistent-api",
			},
			{
				name: "nonexistent control plane reference",
				content: `
api_implementations:
  - ref: invalid-impl
    api: test-api
    service:
      control_plane_id: nonexistent-cp
      id: "550e8400-e29b-41d4-a716-446655440001"

apis:
  - ref: test-api
    name: "Test API"
    description: "Test"
    version: "1.0.0"
`,
				error: "references unknown control_plane: nonexistent-cp",
			},
		}

		for _, tc := range invalidRefs {
			t.Run(tc.name, func(t *testing.T) {
				configFile := filepath.Join(tempDir, "invalid.yaml")
				require.NoError(t, os.WriteFile(configFile, []byte(tc.content), 0o600))

				l := loader.New()
				sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}

				_, err := l.LoadFromSources(sources, false)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.error)
			})
		}
	})

	t.Run("SDK error handling during planning", func(t *testing.T) {
		ctx := SetupTestContext(t)
		tempDir := t.TempDir()

		// Create valid configuration
		config := `
portals:
  - ref: error-portal
    name: "Error Portal"
    description: "Portal that will trigger SDK errors"
`
		configFile := filepath.Join(tempDir, "config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0o600))

		// Load configuration
		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}
		resourceSet, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)

		// Set up mock to return various SDK errors
		mockPortalAPI := GetMockPortalAPI(ctx, t)

		// Test different types of SDK errors
		sdkErrors := []struct {
			name          string
			mockSetup     func()
			expectedError string
		}{
			{
				name: "network timeout error",
				mockSetup: func() {
					mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).
						Return(nil, fmt.Errorf("context deadline exceeded"))
				},
				expectedError: "context deadline exceeded",
			},
			{
				name: "authentication error",
				mockSetup: func() {
					mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).
						Return(&kkOps.ListPortalsResponse{
							StatusCode: 401,
						}, nil)
				},
				expectedError: "authentication failed",
			},
			{
				name: "authorization error",
				mockSetup: func() {
					mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).
						Return(&kkOps.ListPortalsResponse{
							StatusCode: 403,
						}, nil)
				},
				expectedError: "authorization failed",
			},
			{
				name: "server error",
				mockSetup: func() {
					mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).
						Return(&kkOps.ListPortalsResponse{
							StatusCode: 500,
						}, nil)
				},
				expectedError: "server error",
			},
		}

		for _, tc := range sdkErrors {
			t.Run(tc.name, func(t *testing.T) {
				// Reset mock for each test
				mockPortalAPI.ExpectedCalls = nil
				tc.mockSetup()

				stateClient := state.NewClientWithAPIs(mockPortalAPI, nil)
				p := planner.NewPlanner(stateClient, slog.Default())

				_, err := p.GeneratePlan(ctx, resourceSet, planner.Options{Mode: planner.PlanModeApply})
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			})
		}
	})

	t.Run("execution error handling", func(t *testing.T) {
		ctx := SetupTestContext(t)
		tempDir := t.TempDir()

		// Create configuration for execution testing
		config := `
portals:
  - ref: exec-error-portal
    name: "Execution Error Portal"
    description: "Portal for testing execution errors"
`
		configFile := filepath.Join(tempDir, "config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0o600))

		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}
		resourceSet, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)

		// Set up mocks for successful planning but failed execution
		mockPortalAPI := GetMockPortalAPI(ctx, t)

		// Mock successful list for planning
		mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).
			Return(&kkOps.ListPortalsResponse{
				StatusCode: 200,
				ListPortalsResponse: &kkComps.ListPortalsResponse{
					Data: []kkComps.Portal{},
				},
			}, nil)

		// Create plan successfully
		stateClient := state.NewClientWithAPIs(mockPortalAPI, nil)
		p := planner.NewPlanner(stateClient, slog.Default())
		plan, err := p.GeneratePlan(ctx, resourceSet, planner.Options{Mode: planner.PlanModeApply})
		require.NoError(t, err)

		// Test execution errors
		executionErrors := []struct {
			name          string
			mockSetup     func()
			expectedError string
		}{
			{
				name: "create operation failure",
				mockSetup: func() {
					mockPortalAPI.On("CreatePortal", mock.Anything, mock.Anything).
						Return(nil, fmt.Errorf("creation failed: validation error"))
				},
				expectedError: "creation failed",
			},
			{
				name: "rate limit error",
				mockSetup: func() {
					mockPortalAPI.On("CreatePortal", mock.Anything, mock.Anything).
						Return(&kkOps.CreatePortalResponse{
							StatusCode: 429,
						}, nil)
				},
				expectedError: "rate limit exceeded",
			},
			{
				name: "conflict error",
				mockSetup: func() {
					mockPortalAPI.On("CreatePortal", mock.Anything, mock.Anything).
						Return(&kkOps.CreatePortalResponse{
							StatusCode: 409,
						}, nil)
				},
				expectedError: "resource conflict",
			},
		}

		for _, tc := range executionErrors {
			t.Run(tc.name, func(t *testing.T) {
				// Reset create mock for each test
				mockPortalAPI.ExpectedCalls = mockPortalAPI.ExpectedCalls[:1] // Keep list mock
				tc.mockSetup()

				exe := executor.New(stateClient, nil, false)
				result, err := exe.Execute(ctx, plan)

				if err != nil {
					assert.Contains(t, err.Error(), tc.expectedError)
				} else {
					require.NotNil(t, result)
					assert.Equal(t, 0, result.SuccessCount)
					assert.Greater(t, result.FailureCount, 0)
				}
			})
		}
	})

	t.Run("resource dependency cycle detection", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create configuration with circular dependencies
		config := `
# This would create a cycle if we allowed portal -> API -> portal references
portals:
  - ref: portal-a
    name: "Portal A"
    description: "Portal A"

apis:
  - ref: api-a
    name: "API A"
    description: "API A"
    publications:
      - ref: pub-a
        portal_id: portal-a
        visibility: public
        
# Note: The current implementation doesn't create cycles because
# API publications reference portals, not the other way around.
# This test verifies the system handles complex dependency graphs correctly.
`
		configFile := filepath.Join(tempDir, "dependency.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0o600))

		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}

		resourceSet, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)

		// Verify dependencies are resolved correctly (no cycles)
		require.Len(t, resourceSet.Portals, 1)
		require.Len(t, resourceSet.APIs, 1)
		require.Len(t, resourceSet.APIPublications, 1)

		// Verify publication correctly references portal
		pub := resourceSet.APIPublications[0]
		assert.Equal(t, "portal-a", pub.PortalID)
		assert.Equal(t, "api-a", pub.API)
	})

	t.Run("large resource configuration limits", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test handling of large numbers of resources
		numResources := 1000

		// Generate large configuration
		configParts := []string{"portals:"}
		for i := 0; i < numResources; i++ {
			portalDef := fmt.Sprintf(`
  - ref: portal-%d
    name: "Portal %d"
    description: "Auto-generated portal number %d"
    labels:
      index: "%d"
      category: "category-%d"`, i, i, i, i, i%10)
			configParts = append(configParts, portalDef)
		}

		largeConfig := strings.Join(configParts, "")
		configFile := filepath.Join(tempDir, "large.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(largeConfig), 0o600))

		// Load large configuration
		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}

		resourceSet, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)
		require.Len(t, resourceSet.Portals, numResources)

		// Verify random sampling of resources
		for i := 0; i < 10; i++ {
			portal := resourceSet.Portals[i]
			expectedName := fmt.Sprintf("Portal %d", i)
			assert.Equal(t, expectedName, portal.Name)
			assert.Equal(t, fmt.Sprintf("%d", i), portal.Labels["index"])
		}
	})

	t.Run("concurrent operation handling", func(t *testing.T) {
		ctx := SetupTestContext(t)
		tempDir := t.TempDir()

		// Create configuration with multiple resource types
		config := `
portals:
  - ref: concurrent-portal
    name: "Concurrent Portal"
    description: "Portal for concurrent testing"

control_planes:
  - ref: concurrent-cp
    name: "Concurrent CP"
    description: "Control plane for concurrent testing"
    cluster_type: "CLUSTER_TYPE_HYBRID"

apis:
  - ref: concurrent-api
    name: "Concurrent API"
    description: "API for concurrent testing"
    version: "1.0.0"
    publications:
      - ref: concurrent-pub
        portal_id: concurrent-portal
        visibility: public
`
		configFile := filepath.Join(tempDir, "concurrent.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0o600))

		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}
		resourceSet, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)

		// Set up mocks for all resource types
		mockPortalAPI := GetMockPortalAPI(ctx, t)
		mockAPIAPI := GetMockAPIAPI(ctx, t)
		mockControlPlaneAPI := GetMockControlPlaneAPI(ctx, t)

		// Mock all list operations
		mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).
			Return(&kkOps.ListPortalsResponse{
				StatusCode:         200,
				ListPortalResponse: &kkComps.ListPortalResponse{Data: []kkComps.Portal{}},
			}, nil)

		mockControlPlaneAPI.On("ListControlPlanes", mock.Anything, mock.Anything).
			Return(&kkOps.ListControlPlanesResponse{
				StatusCode:                200,
				ListControlPlanesResponse: &kkComps.ListControlPlanesResponse{Data: []kkComps.ControlPlane{}},
			}, nil)

		mockAPIAPI.On("ListApis", mock.Anything, mock.Anything).
			Return(&kkOps.ListApisResponse{
				StatusCode:      200,
				ListAPIResponse: &kkComps.ListAPIResponse{Data: []kkComps.APIResponseSchema{}},
			}, nil)

		mockAPIAPI.On("ListAPIPublications", mock.Anything, mock.Anything).
			Return(&kkOps.ListAPIPublicationsResponse{
				StatusCode:                 200,
				ListAPIPublicationResponse: &kkComps.ListAPIPublicationResponse{Data: []kkComps.APIPublication{}},
			}, nil)

		// Mock create operations with delays to test concurrent handling
		mockPortalAPI.On("CreatePortal", mock.Anything, mock.Anything).
			Return(&kkOps.CreatePortalResponse{
				StatusCode: 201,
				Portal: &kkComps.Portal{
					ID:   "portal-123",
					Name: "Concurrent Portal",
					Labels: map[string]string{
						"KONGCTL-managed":      "true",
						"KONGCTL-last-updated": "20240101-120000Z",
					},
				},
			}, nil).After(50 * time.Millisecond)

		mockControlPlaneAPI.On("CreateControlPlane", mock.Anything, mock.Anything).
			Return(&kkOps.CreateControlPlaneResponse{
				StatusCode: 201,
				ControlPlane: &kkComps.ControlPlane{
					ID:   "cp-123",
					Name: "Concurrent CP",
					Labels: map[string]string{
						"KONGCTL-managed":      "true",
						"KONGCTL-last-updated": "20240101-120000Z",
					},
				},
			}, nil).After(30 * time.Millisecond)

		mockAPIAPI.On("CreateAPI", mock.Anything, mock.Anything).
			Return(&kkOps.CreateAPIResponse{
				StatusCode: 201,
				APIResponseSchema: &kkComps.APIResponseSchema{
					ID:   "api-123",
					Name: "Concurrent API",
					Labels: map[string]string{
						"KONGCTL-managed":      "true",
						"KONGCTL-last-updated": "20240101-120000Z",
					},
				},
			}, nil).After(40 * time.Millisecond)

		mockAPIAPI.On("PublishAPIToPortal", mock.Anything, mock.Anything).
			Return(&kkOps.PublishAPIToPortalResponse{
				StatusCode:     200,
				APIPublication: &kkComps.APIPublication{ID: "pub-123"},
			}, nil).After(20 * time.Millisecond)

		// Test planning and execution with concurrent operations
		stateClient := state.NewClientWithAPIs(mockPortalAPI, mockAPIAPI)
		p := planner.NewPlanner(stateClient, slog.Default())

		plan, err := p.GeneratePlan(ctx, resourceSet, planner.Options{Mode: planner.PlanModeApply})
		require.NoError(t, err)
		assert.NotNil(t, plan)
		assert.NotEmpty(t, plan.Changes)

		// Execute plan (operations should be properly ordered despite concurrency)
		exe := executor.New(stateClient, nil, false)
		result, err := exe.Execute(ctx, plan)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Greater(t, result.SuccessCount, 0)
		assert.Equal(t, 0, result.FailureCount)

		// Verify all mocks were called
		mockPortalAPI.AssertExpectations(t)
		mockAPIAPI.AssertExpectations(t)
		mockControlPlaneAPI.AssertExpectations(t)
	})
}

// TestResourceValidationEdgeCases tests edge cases in resource validation
func TestResourceValidationEdgeCases(t *testing.T) {
	t.Run("empty resource sets", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create empty configuration file
		config := `# Empty configuration file`
		configFile := filepath.Join(tempDir, "empty.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0o600))

		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}

		resourceSet, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)
		require.NotNil(t, resourceSet)

		// Verify empty resource set is valid
		assert.Len(t, resourceSet.Portals, 0)
		assert.Len(t, resourceSet.APIs, 0)
		assert.Len(t, resourceSet.ControlPlanes, 0)
	})

	t.Run("resource name and ref edge cases", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test various edge cases in names and refs
		edgeCases := []struct {
			name          string
			config        string
			shouldSucceed bool
			error         string
		}{
			{
				name: "very long ref",
				config: `
portals:
  - ref: this-is-a-very-long-ref-name-that-should-still-be-valid-as-long-as-it-meets-any-system-requirements
    name: "Long Ref Portal"
    description: "Portal with very long ref"
`,
				shouldSucceed: true,
			},
			{
				name: "special characters in ref",
				config: `
portals:
  - ref: portal-with_special.chars123
    name: "Special Chars Portal"
    description: "Portal with special characters in ref"
`,
				shouldSucceed: true,
			},
			{
				name: "unicode in name",
				config: `
portals:
  - ref: unicode-portal
    name: "Portal with Unicode: 世界 🌍 Привет"
    description: "Portal with unicode characters in name"
`,
				shouldSucceed: true,
			},
			{
				name: "empty ref",
				config: `
portals:
  - ref: ""
    name: "Empty Ref Portal"
    description: "Portal with empty ref"
`,
				shouldSucceed: false,
				error:         "portal ref is required",
			},
		}

		for _, tc := range edgeCases {
			t.Run(tc.name, func(t *testing.T) {
				configFile := filepath.Join(tempDir, "edge_case.yaml")
				require.NoError(t, os.WriteFile(configFile, []byte(tc.config), 0o600))

				l := loader.New()
				sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}

				resourceSet, err := l.LoadFromSources(sources, false)

				if tc.shouldSucceed {
					require.NoError(t, err)
					require.NotNil(t, resourceSet)
					if len(resourceSet.Portals) > 0 {
						portal := resourceSet.Portals[0]
						assert.NotEmpty(t, portal.GetRef())
						assert.NotEmpty(t, portal.Name)
					}
				} else {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tc.error)
				}
			})
		}
	})
}
