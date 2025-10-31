//go:build integration
// +build integration

package declarative_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/executor"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestNamespace_SingleNamespaceOperations tests basic namespace functionality with a single namespace
func TestNamespace_SingleNamespaceOperations(t *testing.T) {
	ctx := SetupTestContext(t)

	// Create test YAML with namespace
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "namespace.yaml")

	configContent := `
apis:
  - ref: team-api
    name: "Team API"
    description: "API for team operations"
    kongctl:
      namespace: team-alpha
portals:
  - ref: team-portal
    name: "Team Portal"
    description: "Portal for team"
    kongctl:
      namespace: team-alpha
`

	err := os.WriteFile(configFile, []byte(configContent), 0o600)
	require.NoError(t, err)

	// Load configuration
	l := loader.New()
	resourceSet, err := l.LoadFromSources([]loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, resourceSet.APIs, 1)
	require.Len(t, resourceSet.Portals, 1)

	// Verify namespace was parsed correctly
	require.NotNil(t, resourceSet.APIs[0].Kongctl)
	require.NotNil(t, resourceSet.APIs[0].Kongctl.Namespace)
	assert.Equal(t, "team-alpha", *resourceSet.APIs[0].Kongctl.Namespace)

	require.NotNil(t, resourceSet.Portals[0].Kongctl)
	require.NotNil(t, resourceSet.Portals[0].Kongctl.Namespace)
	assert.Equal(t, "team-alpha", *resourceSet.Portals[0].Kongctl.Namespace)

	// Debug: print loaded resources
	t.Logf("Loaded %d APIs, %d Portals", len(resourceSet.APIs), len(resourceSet.Portals))
	t.Logf("API[0]: ref=%s, namespace=%s", resourceSet.APIs[0].Ref, *resourceSet.APIs[0].Kongctl.Namespace)
	t.Logf("Portal[0]: ref=%s, namespace=%s", resourceSet.Portals[0].Ref, *resourceSet.Portals[0].Kongctl.Namespace)

	// Set up mocks
	mockAPIAPI := GetMockAPIAPI(ctx, t)
	mockPortalAPI := GetMockPortalAPI(ctx, t)

	// Mock empty lists (no existing resources)
	mockAPIAPI.On("ListApis", mock.Anything, mock.Anything).
		Return(&kkOps.ListApisResponse{
			StatusCode: 200,
			ListAPIResponse: &kkComps.ListAPIResponse{
				Data: []kkComps.APIResponseSchema{},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{
						Total: 0,
					},
				},
			},
		}, nil).Maybe() // Use Maybe() for flexible call count

	// Mock portal list - called multiple times: once for main portal plan and 4 times for child resources
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).
		Return(&kkOps.ListPortalsResponse{
			StatusCode: 200,
			ListPortalsResponse: &kkComps.ListPortalsResponse{
				Data: []kkComps.ListPortalsResponsePortal{},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{
						Total: 0,
					},
				},
			},
		}, nil).Maybe() // Use Maybe() for flexible call count

	// Mock CREATE operations - verify namespace label is applied
	mockAPIAPI.On("CreateAPI", mock.Anything, mock.MatchedBy(func(api kkComps.CreateAPIRequest) bool {
		// Verify namespace label is present
		if api.Labels == nil {
			return false
		}
		return api.Labels[labels.NamespaceKey] == "team-alpha"
	})).Return(&kkOps.CreateAPIResponse{
		StatusCode: 201,
		APIResponseSchema: &kkComps.APIResponseSchema{
			ID:          "api-123",
			Name:        "Team API",
			Description: stringPtr("API for team operations"),
			Labels: map[string]string{
				labels.NamespaceKey: "team-alpha",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}, nil)

	portalTime := time.Now()
	mockPortalAPI.On("CreatePortal", mock.Anything, mock.MatchedBy(func(portal kkComps.CreatePortal) bool {
		// Verify namespace label is present
		if portal.Labels == nil {
			return false
		}
		// Portal.Labels is a map[string]*string, need to dereference
		if nsLabel, ok := portal.Labels[labels.NamespaceKey]; ok && nsLabel != nil {
			return *nsLabel == "team-alpha"
		}
		return false
	})).Return(&kkOps.CreatePortalResponse{
		StatusCode: 201,
		PortalResponse: &kkComps.PortalResponse{
			ID:        "portal-123",
			Name:      "Team Portal",
			CreatedAt: portalTime,
			UpdatedAt: portalTime,
		},
	}, nil)

	// Mock empty child resources
	mockAPIAPI.On("ListAPIVersions", mock.Anything, mock.Anything).
		Return(&kkOps.ListAPIVersionsResponse{
			StatusCode: 200,
			ListAPIVersionResponse: &kkComps.ListAPIVersionResponse{
				Data: []kkComps.ListAPIVersionResponseAPIVersionSummary{},
			},
		}, nil).Maybe()

	mockAPIAPI.On("ListAPIPublications", mock.Anything, mock.Anything).
		Return(&kkOps.ListAPIPublicationsResponse{
			StatusCode: 200,
			ListAPIPublicationResponse: &kkComps.ListAPIPublicationResponse{
				Data: nil,
			},
		}, nil).Maybe()

	mockAPIAPI.On("ListAPIImplementations", mock.Anything, mock.Anything).
		Return(&kkOps.ListAPIImplementationsResponse{
			StatusCode: 200,
			ListAPIImplementationsResponse: &kkComps.ListAPIImplementationsResponse{
				Data: []kkComps.APIImplementationListItem{},
			},
		}, nil).Maybe()

	// Create state client and planner
	mockAuthStrategyAPI := GetMockAppAuthStrategiesAPI(ctx, t)
	// Mock empty auth strategies list - add Maybe() to be flexible with call count
	mockAuthStrategyAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).
		Return(&kkOps.ListAppAuthStrategiesResponse{
			StatusCode: 200,
			ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
				Data: []kkComps.AppAuthStrategy{},
			},
		}, nil).Maybe().Maybe()

	stateClient := state.NewClient(state.ClientConfig{
		PortalAPI:  mockPortalAPI,
		APIAPI:     mockAPIAPI,
		AppAuthAPI: mockAuthStrategyAPI,
	})
	p := planner.NewPlanner(stateClient, slog.Default())

	// Generate plan
	plan, err := p.GeneratePlan(ctx, resourceSet, planner.Options{Mode: planner.PlanModeApply})
	if err != nil {
		t.Logf("Plan generation error: %v", err)
	}
	require.NoError(t, err)

	// Debug: print plan summary
	t.Logf("Plan generated: %d changes", len(plan.Changes))
	for i, change := range plan.Changes {
		t.Logf("Change %d: %s %s (namespace: %s)", i, change.Action, change.ResourceType, change.Namespace)
	}

	// Verify plan contains CREATE operations for both resources
	assert.Len(t, plan.Changes, 2)

	// Verify both changes have namespace set
	for _, change := range plan.Changes {
		assert.Equal(t, "team-alpha", change.Namespace)
		assert.Equal(t, planner.ActionCreate, change.Action)
	}

	// Execute the plan
	exec := executor.New(stateClient, nil, false)
	report := exec.Execute(ctx, plan)
	require.NoError(t, err)
	assert.Equal(t, 2, report.SuccessCount)
	assert.Equal(t, 0, report.FailureCount)

	// Verify all mocks were called
	mockAPIAPI.AssertExpectations(t)
	mockPortalAPI.AssertExpectations(t)
}

// TestNamespace_MultiNamespaceOperations tests operations across multiple namespaces
func TestNamespace_MultiNamespaceOperations(t *testing.T) {
	ctx := SetupTestContext(t)

	// Create test YAML with multiple namespaces
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "multi-namespace.yaml")

	configContent := `
apis:
  - ref: alpha-api
    name: "Alpha API"
    kongctl:
      namespace: team-alpha
  - ref: beta-api
    name: "Beta API"
    kongctl:
      namespace: team-beta
  - ref: default-api
    name: "Default API"
    kongctl:
      namespace: default
`

	err := os.WriteFile(configFile, []byte(configContent), 0o600)
	require.NoError(t, err)

	// Load configuration
	l := loader.New()
	resourceSet, err := l.LoadFromSources([]loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, resourceSet.APIs, 3)

	// Set up mocks
	mockAPIAPI := GetMockAPIAPI(ctx, t)

	// Mock separate ListApis calls for each namespace
	// The planner groups by namespace and makes separate calls
	mockAPIAPI.On("ListApis", mock.Anything, mock.Anything).
		Return(&kkOps.ListApisResponse{
			StatusCode: 200,
			ListAPIResponse: &kkComps.ListAPIResponse{
				Data: []kkComps.APIResponseSchema{},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{
						Total: 0,
					},
				},
			},
		}, nil).Maybe() // Multiple calls for 3 namespaces during planning and execution

	// Mock CREATE operations for each namespace
	// Mock Alpha API creation
	mockAPIAPI.On("CreateAPI", mock.Anything, mock.MatchedBy(func(api kkComps.CreateAPIRequest) bool {
		return api.Name == "Alpha API" && api.Labels != nil && api.Labels[labels.NamespaceKey] == "team-alpha"
	})).Return(&kkOps.CreateAPIResponse{
		StatusCode: 201,
		APIResponseSchema: &kkComps.APIResponseSchema{
			ID:   "api-alpha",
			Name: "Alpha API",
			Labels: map[string]string{
				labels.NamespaceKey: "team-alpha",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}, nil).Once()

	// Mock Beta API creation
	mockAPIAPI.On("CreateAPI", mock.Anything, mock.MatchedBy(func(api kkComps.CreateAPIRequest) bool {
		return api.Name == "Beta API" && api.Labels != nil && api.Labels[labels.NamespaceKey] == "team-beta"
	})).Return(&kkOps.CreateAPIResponse{
		StatusCode: 201,
		APIResponseSchema: &kkComps.APIResponseSchema{
			ID:   "api-beta",
			Name: "Beta API",
			Labels: map[string]string{
				labels.NamespaceKey: "team-beta",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}, nil).Once()

	// Mock Default API creation
	mockAPIAPI.On("CreateAPI", mock.Anything, mock.MatchedBy(func(api kkComps.CreateAPIRequest) bool {
		return api.Name == "Default API" && api.Labels != nil && api.Labels[labels.NamespaceKey] == "default"
	})).Return(&kkOps.CreateAPIResponse{
		StatusCode: 201,
		APIResponseSchema: &kkComps.APIResponseSchema{
			ID:   "api-default",
			Name: "Default API",
			Labels: map[string]string{
				labels.NamespaceKey: "default",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}, nil).Once()

	// Mock empty child resources
	mockAPIAPI.On("ListAPIVersions", mock.Anything, mock.Anything).
		Return(&kkOps.ListAPIVersionsResponse{
			StatusCode: 200,
			ListAPIVersionResponse: &kkComps.ListAPIVersionResponse{
				Data: []kkComps.ListAPIVersionResponseAPIVersionSummary{},
			},
		}, nil).Maybe()

	mockAPIAPI.On("ListAPIPublications", mock.Anything, mock.Anything).
		Return(&kkOps.ListAPIPublicationsResponse{
			StatusCode: 200,
			ListAPIPublicationResponse: &kkComps.ListAPIPublicationResponse{
				Data: nil,
			},
		}, nil).Maybe()

	mockAPIAPI.On("ListAPIImplementations", mock.Anything, mock.Anything).
		Return(&kkOps.ListAPIImplementationsResponse{
			StatusCode: 200,
			ListAPIImplementationsResponse: &kkComps.ListAPIImplementationsResponse{
				Data: []kkComps.APIImplementationListItem{},
			},
		}, nil).Maybe()

	// Create state client and planner
	mockAuthStrategyAPI := GetMockAppAuthStrategiesAPI(ctx, t)
	// Mock empty auth strategies list
	mockAuthStrategyAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).
		Return(&kkOps.ListAppAuthStrategiesResponse{
			StatusCode: 200,
			ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
				Data: []kkComps.AppAuthStrategy{},
			},
		}, nil).Maybe()

	// Get the portal API mock
	mockPortalAPI2 := GetMockPortalAPI(ctx, t)
	// Mock empty portals list - add Times(6) to handle all calls (3 namespaces x 2 calls each)
	mockPortalAPI2.On("ListPortals", mock.Anything, mock.Anything).
		Return(&kkOps.ListPortalsResponse{
			StatusCode: 200,
			ListPortalsResponse: &kkComps.ListPortalsResponse{
				Data: []kkComps.ListPortalsResponsePortal{},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{
						Total: 0,
					},
				},
			},
		}, nil).Maybe()

	stateClient := state.NewClient(state.ClientConfig{
		PortalAPI:  mockPortalAPI2,
		APIAPI:     mockAPIAPI,
		AppAuthAPI: mockAuthStrategyAPI,
	})
	p := planner.NewPlanner(stateClient, slog.Default())

	// Generate plan
	plan, err := p.GeneratePlan(ctx, resourceSet, planner.Options{Mode: planner.PlanModeApply})
	require.NoError(t, err)

	// Verify plan contains CREATE operations for all 3 APIs
	assert.Len(t, plan.Changes, 3)

	// Group changes by namespace
	namespaceChanges := make(map[string]int)
	for _, change := range plan.Changes {
		namespaceChanges[change.Namespace]++
	}

	// Verify we have one change per namespace
	assert.Equal(t, 1, namespaceChanges["team-alpha"])
	assert.Equal(t, 1, namespaceChanges["team-beta"])
	assert.Equal(t, 1, namespaceChanges["default"])

	// Execute the plan
	exec := executor.New(stateClient, nil, false)
	report := exec.Execute(ctx, plan)
	require.NoError(t, err)

	// Verify all APIs were created
	assert.Equal(t, 3, report.SuccessCount)
	assert.Equal(t, 0, report.FailureCount)
	mockAPIAPI.AssertExpectations(t)
}

// TestNamespace_DefaultsInheritance tests _defaults.kongctl.namespace inheritance
func TestNamespace_DefaultsInheritance(t *testing.T) {
	_ = SetupTestContext(t)

	// Create test YAML with namespace defaults
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "defaults.yaml")

	configContent := `
_defaults:
  kongctl:
    namespace: team-gamma
    
apis:
  - ref: inherited-api
    name: "Inherited API"
    description: "Should inherit team-gamma namespace"
  - ref: explicit-api
    name: "Explicit API"
    description: "Should use explicit namespace"
    kongctl:
      namespace: team-delta
`

	err := os.WriteFile(configFile, []byte(configContent), 0o600)
	require.NoError(t, err)

	// Load configuration
	l := loader.New()
	resourceSet, err := l.LoadFromSources([]loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, resourceSet.APIs, 2)

	// Verify namespace inheritance
	// First API should inherit from defaults
	require.NotNil(t, resourceSet.APIs[0].Kongctl)
	require.NotNil(t, resourceSet.APIs[0].Kongctl.Namespace)
	assert.Equal(t, "team-gamma", *resourceSet.APIs[0].Kongctl.Namespace,
		"First API should inherit namespace from defaults")

	// Second API should use explicit namespace
	require.NotNil(t, resourceSet.APIs[1].Kongctl)
	require.NotNil(t, resourceSet.APIs[1].Kongctl.Namespace)
	assert.Equal(t, "team-delta", *resourceSet.APIs[1].Kongctl.Namespace, "Second API should use explicit namespace")
}

// TestNamespace_IsolationBetweenNamespaces tests that sync operations respect namespace boundaries
func TestNamespace_IsolationBetweenNamespaces(t *testing.T) {
	ctx := SetupTestContext(t)

	// Create test YAML with only team-alpha resources
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "isolation.yaml")

	configContent := `
apis:
  - ref: alpha-api
    name: "Alpha API"
    kongctl:
      namespace: team-alpha
`

	err := os.WriteFile(configFile, []byte(configContent), 0o600)
	require.NoError(t, err)

	// Load configuration
	l := loader.New()
	resourceSet, err := l.LoadFromSources([]loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}, false)
	require.NoError(t, err)

	// Set up mocks
	mockAPIAPI := GetMockAPIAPI(ctx, t)

	// Mock ListApis to return APIs from multiple namespaces
	// This simulates existing resources in Konnect
	existingAPIs := []kkComps.APIResponseSchema{
		{
			ID:   "alpha-1",
			Name: "Alpha API 1",
			Labels: map[string]string{
				labels.NamespaceKey: "team-alpha",
			},
		},
		{
			ID:   "beta-1",
			Name: "Beta API 1",
			Labels: map[string]string{
				labels.NamespaceKey: "team-beta",
			},
		},
		{
			ID:   "alpha-2",
			Name: "Alpha API 2",
			Labels: map[string]string{
				labels.NamespaceKey: "team-alpha",
			},
		},
	}

	mockAPIAPI.On("ListApis", mock.Anything, mock.Anything).
		Return(&kkOps.ListApisResponse{
			StatusCode: 200,
			ListAPIResponse: &kkComps.ListAPIResponse{
				Data: existingAPIs,
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{
						Total: float64(len(existingAPIs)),
					},
				},
			},
		}, nil).Maybe() // Use Maybe() for flexible call count

	// Mock child resources for existing APIs
	for _, api := range existingAPIs {
		mockAPIAPI.On("ListAPIVersions", mock.Anything, mock.MatchedBy(func(req kkOps.ListAPIVersionsRequest) bool {
			return req.APIID == api.ID
		})).Return(&kkOps.ListAPIVersionsResponse{
			StatusCode: 200,
			ListAPIVersionResponse: &kkComps.ListAPIVersionResponse{
				Data: []kkComps.ListAPIVersionResponseAPIVersionSummary{},
			},
		}, nil).Maybe().Maybe()

		mockAPIAPI.On("ListAPIPublications", mock.Anything, mock.MatchedBy(func(req kkOps.ListAPIPublicationsRequest) bool {
			return req.Filter != nil && req.Filter.APIID != nil && req.Filter.APIID.Eq != nil &&
				*req.Filter.APIID.Eq == api.ID
		})).
			Return(&kkOps.ListAPIPublicationsResponse{
				StatusCode: 200,
				ListAPIPublicationResponse: &kkComps.ListAPIPublicationResponse{
					Data: nil,
				},
			}, nil).
			Maybe().
			Maybe()

		mockAPIAPI.On("ListAPIImplementations", mock.Anything,
			mock.MatchedBy(func(req kkOps.ListAPIImplementationsRequest) bool {
				return req.Filter != nil && req.Filter.APIID != nil && req.Filter.APIID.Eq != nil &&
					*req.Filter.APIID.Eq == api.ID
			})).Return(&kkOps.ListAPIImplementationsResponse{
			StatusCode: 200,
			ListAPIImplementationsResponse: &kkComps.ListAPIImplementationsResponse{
				Data: []kkComps.APIImplementationListItem{},
			},
		}, nil).Maybe().Maybe()
	}

	// Mock CREATE for new Alpha API
	mockAPIAPI.On("CreateAPI", mock.Anything, mock.MatchedBy(func(api kkComps.CreateAPIRequest) bool {
		return api.Name == "Alpha API" && api.Labels[labels.NamespaceKey] == "team-alpha"
	})).Return(&kkOps.CreateAPIResponse{
		StatusCode: 201,
		APIResponseSchema: &kkComps.APIResponseSchema{
			ID:   "alpha-new",
			Name: "Alpha API",
			Labels: map[string]string{
				labels.NamespaceKey: "team-alpha",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}, nil)

	// Mock DELETE operations - should only delete team-alpha resources
	mockAPIAPI.On("DeleteAPI", mock.Anything, "alpha-1").
		Return(&kkOps.DeleteAPIResponse{StatusCode: 204}, nil)
	mockAPIAPI.On("DeleteAPI", mock.Anything, "alpha-2").
		Return(&kkOps.DeleteAPIResponse{StatusCode: 204}, nil)

	// Create state client and planner
	mockAuthStrategyAPI := GetMockAppAuthStrategiesAPI(ctx, t)
	// Mock empty auth strategies list
	mockAuthStrategyAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).
		Return(&kkOps.ListAppAuthStrategiesResponse{
			StatusCode: 200,
			ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
				Data: []kkComps.AppAuthStrategy{},
			},
		}, nil).Maybe() // Use Maybe() for flexible call count // For plan and execute

	// Get mock portal API and set up expectations
	mockPortalAPI2 := GetMockPortalAPI(ctx, t)
	// Mock empty portals list
	mockPortalAPI2.On("ListPortals", mock.Anything, mock.Anything).
		Return(&kkOps.ListPortalsResponse{
			StatusCode: 200,
			ListPortalsResponse: &kkComps.ListPortalsResponse{
				Data: []kkComps.ListPortalsResponsePortal{},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{
						Total: 0,
					},
				},
			},
		}, nil).Maybe() // Use Maybe() for flexible call count // For plan and execute

	stateClient := state.NewClient(state.ClientConfig{
		PortalAPI:  mockPortalAPI2,
		APIAPI:     mockAPIAPI,
		AppAuthAPI: mockAuthStrategyAPI,
	})
	p := planner.NewPlanner(stateClient, slog.Default())

	// Generate plan in sync mode
	plan, err := p.GeneratePlan(ctx, resourceSet, planner.Options{Mode: planner.PlanModeSync})
	require.NoError(t, err)

	// Verify plan:
	// - 1 CREATE for new Alpha API
	// - 2 DELETEs for existing team-alpha APIs (alpha-1, alpha-2)
	// - 0 operations for team-beta (should be isolated)
	assert.Len(t, plan.Changes, 3)

	createCount := 0
	deleteCount := 0
	for _, change := range plan.Changes {
		// All changes should be in team-alpha namespace
		assert.Equal(t, "team-alpha", change.Namespace)

		switch change.Action {
		case planner.ActionCreate:
			createCount++
		case planner.ActionDelete:
			deleteCount++
			// Verify we're only deleting team-alpha resources
			assert.Contains(t, []string{"alpha-1", "alpha-2"}, change.ResourceID)
		case planner.ActionUpdate:
			// Should not have any updates in this test
			t.Errorf("Unexpected UPDATE action for %s", change.ResourceType)
		}
	}

	assert.Equal(t, 1, createCount, "Should create 1 new API")
	assert.Equal(t, 2, deleteCount, "Should delete 2 existing team-alpha APIs")

	// Execute the plan
	exec := executor.New(stateClient, nil, false)
	_ = exec.Execute(ctx, plan)

	// Verify beta-1 was NOT deleted (namespace isolation)
	mockAPIAPI.AssertNotCalled(t, "DeleteAPI", mock.Anything, "beta-1")
	mockAPIAPI.AssertExpectations(t)
}

// TestNamespace_ValidationErrors tests namespace validation error cases
func TestNamespace_ValidationErrors(t *testing.T) {
	testCases := []struct {
		name        string
		yaml        string
		expectError string
	}{
		{
			name: "invalid namespace with uppercase",
			yaml: `
apis:
  - ref: api1
    name: "API 1"
    kongctl:
      namespace: TeamAlpha
`,
			expectError: "namespace 'TeamAlpha' is invalid: must consist of lowercase alphanumeric",
		},
		{
			name: "invalid namespace with underscore",
			yaml: `
apis:
  - ref: api1
    name: "API 1"
    kongctl:
      namespace: team_alpha
`,
			expectError: "namespace 'team_alpha' is invalid: must consist of lowercase alphanumeric",
		},
		{
			name: "invalid namespace too long",
			yaml: `
apis:
  - ref: api1
    name: "API 1"
    kongctl:
      namespace: this-is-a-very-long-namespace-name-that-exceeds-the-maximum-allowed
`,
			expectError: "exceeds maximum length of 63 characters",
		},
		{
			name: "invalid namespace with double hyphen",
			yaml: `
apis:
  - ref: api1
    name: "API 1"
    kongctl:
      namespace: team--alpha
`,
			expectError: "cannot contain consecutive hyphens",
		},
		{
			name: "empty namespace explicitly set",
			yaml: `
apis:
  - ref: api1
    name: "API 1"
    kongctl:
      namespace: ""
`,
			expectError: "cannot have an empty namespace",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test YAML
			tempDir := t.TempDir()
			configFile := filepath.Join(tempDir, "invalid.yaml")

			err := os.WriteFile(configFile, []byte(tc.yaml), 0o600)
			require.NoError(t, err)

			// Attempt to load configuration
			l := loader.New()
			_, err = l.LoadFromSources([]loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}, false)

			// Should get validation error
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectError)
		})
	}
}

// TestNamespace_ProtectedResourcesWithNamespaces tests protected flag behavior with namespaces
func TestNamespace_ProtectedResourcesWithNamespaces(t *testing.T) {
	ctx := SetupTestContext(t)

	// Create test YAML with protected resources in different namespaces
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "protected.yaml")

	configContent := `
apis:
  - ref: protected-alpha
    name: "Protected Alpha API"
    kongctl:
      namespace: team-alpha
      protected: true
  - ref: unprotected-alpha
    name: "Unprotected Alpha API"
    kongctl:
      namespace: team-alpha
      protected: false
`

	err := os.WriteFile(configFile, []byte(configContent), 0o600)
	require.NoError(t, err)

	// Load configuration
	l := loader.New()
	resourceSet, err := l.LoadFromSources([]loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, resourceSet.APIs, 2)

	// Set up mocks
	mockAPIAPI := GetMockAPIAPI(ctx, t)

	// Mock empty lists
	mockAPIAPI.On("ListApis", mock.Anything, mock.Anything).
		Return(&kkOps.ListApisResponse{
			StatusCode: 200,
			ListAPIResponse: &kkComps.ListAPIResponse{
				Data: []kkComps.APIResponseSchema{},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{
						Total: 0,
					},
				},
			},
		}, nil).Maybe() // Use Maybe() for flexible call count

	// Mock CREATE operations - verify both namespace and protected labels
	// Mock Protected Alpha API creation
	mockAPIAPI.On("CreateAPI", mock.Anything, mock.MatchedBy(func(api kkComps.CreateAPIRequest) bool {
		return api.Name == "Protected Alpha API" &&
			api.Labels != nil &&
			api.Labels[labels.NamespaceKey] == "team-alpha" &&
			api.Labels[labels.ProtectedKey] == "true"
	})).Return(&kkOps.CreateAPIResponse{
		StatusCode: 201,
		APIResponseSchema: &kkComps.APIResponseSchema{
			ID:   "api-protected",
			Name: "Protected Alpha API",
			Labels: map[string]string{
				labels.NamespaceKey: "team-alpha",
				labels.ProtectedKey: "true",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}, nil).Once()

	// Mock Unprotected Alpha API creation
	mockAPIAPI.On("CreateAPI", mock.Anything, mock.MatchedBy(func(api kkComps.CreateAPIRequest) bool {
		// When protected is false, the label is not added (only namespace label is present)
		_, hasProtected := api.Labels[labels.ProtectedKey]
		return api.Name == "Unprotected Alpha API" &&
			api.Labels != nil &&
			api.Labels[labels.NamespaceKey] == "team-alpha" &&
			!hasProtected
	})).Return(&kkOps.CreateAPIResponse{
		StatusCode: 201,
		APIResponseSchema: &kkComps.APIResponseSchema{
			ID:   "api-unprotected",
			Name: "Unprotected Alpha API",
			Labels: map[string]string{
				labels.NamespaceKey: "team-alpha",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}, nil).Once()

	// Mock empty child resources
	mockAPIAPI.On("ListAPIVersions", mock.Anything, mock.Anything).
		Return(&kkOps.ListAPIVersionsResponse{
			StatusCode: 200,
			ListAPIVersionResponse: &kkComps.ListAPIVersionResponse{
				Data: []kkComps.ListAPIVersionResponseAPIVersionSummary{},
			},
		}, nil).Maybe() // Use Maybe() for flexible call count

	mockAPIAPI.On("ListAPIPublications", mock.Anything, mock.Anything).
		Return(&kkOps.ListAPIPublicationsResponse{
			StatusCode: 200,
			ListAPIPublicationResponse: &kkComps.ListAPIPublicationResponse{
				Data: nil,
			},
		}, nil).Maybe() // Use Maybe() for flexible call count

	mockAPIAPI.On("ListAPIImplementations", mock.Anything, mock.Anything).
		Return(&kkOps.ListAPIImplementationsResponse{
			StatusCode: 200,
			ListAPIImplementationsResponse: &kkComps.ListAPIImplementationsResponse{
				Data: []kkComps.APIImplementationListItem{},
			},
		}, nil).Maybe() // Use Maybe() for flexible call count

	// Create state client and planner
	mockAuthStrategyAPI := GetMockAppAuthStrategiesAPI(ctx, t)
	// Mock empty auth strategies list
	mockAuthStrategyAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).
		Return(&kkOps.ListAppAuthStrategiesResponse{
			StatusCode: 200,
			ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
				Data: []kkComps.AppAuthStrategy{},
			},
		}, nil).Maybe() // Use Maybe() for flexible call count

	// Get mock portal API and set up expectations
	mockPortalAPI3 := GetMockPortalAPI(ctx, t)
	// Mock empty portals list
	mockPortalAPI3.On("ListPortals", mock.Anything, mock.Anything).
		Return(&kkOps.ListPortalsResponse{
			StatusCode: 200,
			ListPortalsResponse: &kkComps.ListPortalsResponse{
				Data: []kkComps.ListPortalsResponsePortal{},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{
						Total: 0,
					},
				},
			},
		}, nil).Maybe() // Use Maybe() for flexible call count

	stateClient := state.NewClient(state.ClientConfig{
		PortalAPI:  mockPortalAPI3,
		APIAPI:     mockAPIAPI,
		AppAuthAPI: mockAuthStrategyAPI,
	})
	p := planner.NewPlanner(stateClient, slog.Default())

	// Generate plan
	plan, err := p.GeneratePlan(ctx, resourceSet, planner.Options{Mode: planner.PlanModeApply})
	require.NoError(t, err)

	// Execute the plan
	exec := executor.New(stateClient, nil, false)
	report := exec.Execute(ctx, plan)
	assert.Equal(t, 2, report.SuccessCount)
	assert.Equal(t, 0, report.FailureCount)

	mockAPIAPI.AssertExpectations(t)
}

// Note: GetMockPortalAPI, GetMockAPIAPI, and stringPtr are already defined in api_test.go
// We only need to define GetMockAppAuthStrategiesAPI here

func GetMockAppAuthStrategiesAPI(ctx context.Context, _ *testing.T) *MockAppAuthStrategiesAPI {
	sdk := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
	konnectSDK, _ := sdk(GetTestConfig(), nil)
	mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
	return mockSDK.GetAppAuthStrategiesAPI().(*MockAppAuthStrategiesAPI)
}
