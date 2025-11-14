//go:build integration && disabled

package declarative_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestSDKModeVerification tests that both mock and real SDK modes are properly supported
func TestSDKModeVerification(t *testing.T) {
	t.Run("mock SDK mode functionality", func(t *testing.T) {
		// Ensure we're using mock SDK by unsetting any real token
		originalToken := os.Getenv("KONNECT_INTEGRATION_TOKEN")
		if originalToken != "" {
			os.Unsetenv("KONNECT_INTEGRATION_TOKEN")
			defer os.Setenv("KONNECT_INTEGRATION_TOKEN", originalToken)
		}

		ctx := SetupTestContext(t)

		// Verify SDK factory returns mock SDK
		sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
		require.NotNil(t, sdkFactory)

		sdk, err := sdkFactory(GetTestConfig(), nil)
		require.NoError(t, err)
		require.NotNil(t, sdk)

		// Verify it's a mock SDK
		mockSDK, ok := sdk.(*helpers.MockKonnectSDK)
		require.True(t, ok, "Expected MockKonnectSDK but got %T", sdk)
		require.NotNil(t, mockSDK)

		// Test that mock SDK provides working API interfaces
		portalAPI := mockSDK.GetPortalAPI()
		require.NotNil(t, portalAPI)
		_, ok = portalAPI.(*MockPortalAPI)
		assert.True(t, ok, "Expected MockPortalAPI")

		apiAPI := mockSDK.GetAPIAPI()
		require.NotNil(t, apiAPI)
		_, ok = apiAPI.(*MockAPIAPI)
		assert.True(t, ok, "Expected MockAPIAPI")

		// Test mock functionality with a simple operation
		tempDir := t.TempDir()
		config := `
portals:
  - ref: mock-test-portal
    name: "Mock Test Portal"
    description: "Portal for testing mock SDK"
`
		configFile := filepath.Join(tempDir, "config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0o600))

		// Load configuration
		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}
		resourceSet, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)

		// Set up mock expectations
		mockPortalAPI := GetMockPortalAPI(ctx, t)
		mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).
			Return(&kkOps.ListPortalsResponse{
				StatusCode: 200,
				ListPortalsResponse: &kkComps.ListPortalsResponse{
					Data: []kkComps.ListPortalsResponsePortal{},
				},
			}, nil)

		// Test planning with mock SDK
		stateClient := state.NewClientWithAPIs(mockPortalAPI, nil)
		p := planner.NewPlanner(stateClient, slog.Default())

		plan, err := p.GeneratePlan(ctx, resourceSet, planner.Options{Mode: planner.PlanModeApply})
		require.NoError(t, err)
		require.NotNil(t, plan)
		assert.NotNil(t, plan)
		assert.NotEmpty(t, plan.Changes)

		// Verify mock was called
		mockPortalAPI.AssertExpectations(t)
	})

	t.Run("real SDK mode detection", func(t *testing.T) {
		// Test that setting KONNECT_INTEGRATION_TOKEN changes SDK mode
		originalToken := os.Getenv("KONNECT_INTEGRATION_TOKEN")
		defer func() {
			if originalToken != "" {
				os.Setenv("KONNECT_INTEGRATION_TOKEN", originalToken)
			} else {
				os.Unsetenv("KONNECT_INTEGRATION_TOKEN")
			}
		}()

		// Set fake token to trigger real SDK mode
		os.Setenv("KONNECT_INTEGRATION_TOKEN", "fake-token-for-testing")

		ctx := SetupTestContext(t)

		// Verify SDK factory detects real token
		sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
		require.NotNil(t, sdkFactory)

		// Attempt to create SDK with real token (should fail with our test implementation)
		sdk, err := sdkFactory(GetTestConfig(), nil)

		// Since we don't have real SDK implementation yet, it should fail with specific error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "real Konnect SDK integration not yet implemented")
		assert.Nil(t, sdk)
	})

	t.Run("SDK mode environment detection", func(t *testing.T) {
		// Test helper function respects environment variable
		originalToken := os.Getenv("KONNECT_INTEGRATION_TOKEN")
		defer func() {
			if originalToken != "" {
				os.Setenv("KONNECT_INTEGRATION_TOKEN", originalToken)
			} else {
				os.Unsetenv("KONNECT_INTEGRATION_TOKEN")
			}
		}()

		// Test with no token - should get mock
		os.Unsetenv("KONNECT_INTEGRATION_TOKEN")
		factory1 := GetSDKFactory(t)
		sdk1, err1 := factory1(GetTestConfig(), nil)
		require.NoError(t, err1)
		_, ok := sdk1.(*helpers.MockKonnectSDK)
		assert.True(t, ok, "Should get mock SDK when no token set")

		// Test with token - should attempt real SDK
		os.Setenv("KONNECT_INTEGRATION_TOKEN", "test-token")
		factory2 := GetSDKFactory(t)
		sdk2, err2 := factory2(GetTestConfig(), nil)
		require.Error(t, err2)
		assert.Contains(t, err2.Error(), "real Konnect SDK integration not yet implemented")
		assert.Nil(t, sdk2)
	})

	t.Run("mock API coverage verification", func(t *testing.T) {
		ctx := SetupTestContext(t)

		// Get all mock APIs and verify they implement expected interfaces
		mockPortalAPI := GetMockPortalAPI(ctx, t)
		require.NotNil(t, mockPortalAPI)

		// Verify MockPortalAPI implements helpers.PortalAPI interface
		var _ helpers.PortalAPI = mockPortalAPI

		mockAPIAPI := GetMockAPIAPI(ctx, t)
		require.NotNil(t, mockAPIAPI)

		// Verify MockAPIAPI implements helpers.APIAPI interface
		var _ helpers.APIAPI = mockAPIAPI

		// Test that all expected methods are available and mockable
		mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).
			Return(&kkOps.ListPortalsResponse{StatusCode: 200}, nil)
		mockPortalAPI.On("CreatePortal", mock.Anything, mock.Anything).
			Return(&kkOps.CreatePortalResponse{StatusCode: 201}, nil)
		mockPortalAPI.On("UpdatePortal", mock.Anything, mock.Anything).
			Return(&kkOps.UpdatePortalResponse{StatusCode: 200}, nil)
		mockPortalAPI.On("DeletePortal", mock.Anything, mock.Anything, mock.Anything).
			Return(&kkOps.DeletePortalResponse{StatusCode: 200}, nil)

		mockAPIAPI.On("ListApis", mock.Anything, mock.Anything).
			Return(&kkOps.ListApisResponse{StatusCode: 200}, nil)
		mockAPIAPI.On("CreateAPI", mock.Anything, mock.Anything).
			Return(&kkOps.CreateAPIResponse{StatusCode: 201}, nil)
		mockAPIAPI.On("UpdateAPI", mock.Anything, mock.Anything, mock.Anything).
			Return(&kkOps.UpdateAPIResponse{StatusCode: 200}, nil)
		mockAPIAPI.On("DeleteAPI", mock.Anything, mock.Anything).
			Return(&kkOps.DeleteAPIResponse{StatusCode: 200}, nil)

		// Verify child resource operations are available
		mockAPIAPI.On("ListAPIVersions", mock.Anything, mock.Anything).
			Return(&kkOps.ListAPIVersionsResponse{StatusCode: 200}, nil)
		mockAPIAPI.On("CreateAPIVersion", mock.Anything, mock.Anything).
			Return(&kkOps.CreateAPIVersionResponse{StatusCode: 201}, nil)
		mockAPIAPI.On("ListAPIPublications", mock.Anything, mock.Anything).
			Return(&kkOps.ListAPIPublicationsResponse{StatusCode: 200}, nil)
		mockAPIAPI.On("PublishAPIToPortal", mock.Anything, mock.Anything).
			Return(&kkOps.PublishAPIToPortalResponse{StatusCode: 200}, nil)
		mockAPIAPI.On("ListAPIImplementations", mock.Anything, mock.Anything).
			Return(&kkOps.ListAPIImplementationsResponse{StatusCode: 200}, nil)
		mockAPIAPI.On("ListAPIDocuments", mock.Anything, mock.Anything).
			Return(&kkOps.ListAPIDocumentsResponse{StatusCode: 200}, nil)

		// All mocks set up successfully indicates good interface coverage
		t.Log("All mock APIs successfully implement expected interfaces")
	})

	t.Run("integration test dual mode support", func(t *testing.T) {
		// Verify that all our integration tests can work in both modes
		tempDir := t.TempDir()

		// Create a simple test configuration
		config := `
portals:
  - ref: dual-mode-portal
    name: "Dual Mode Portal"
    description: "Portal for dual-mode testing"
    
apis:
  - ref: dual-mode-api
    name: "Dual Mode API"
    description: "API for dual-mode testing"
    version: "1.0.0"
`
		configFile := filepath.Join(tempDir, "dual-mode.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0o600))

		// Test with mock mode (current default)
		originalToken := os.Getenv("KONNECT_INTEGRATION_TOKEN")
		if originalToken != "" {
			os.Unsetenv("KONNECT_INTEGRATION_TOKEN")
			defer os.Setenv("KONNECT_INTEGRATION_TOKEN", originalToken)
		}

		ctx := SetupTestContext(t)

		// Load configuration (should work in both modes)
		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}
		resourceSet, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)
		require.Len(t, resourceSet.Portals, 1)
		require.Len(t, resourceSet.APIs, 1)

		// Set up mocks for planning test
		mockPortalAPI := GetMockPortalAPI(ctx, t)
		mockAPIAPI := GetMockAPIAPI(ctx, t)

		mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).
			Return(&kkOps.ListPortalsResponse{
				StatusCode:          200,
				ListPortalsResponse: &kkComps.ListPortalsResponse{Data: []kkComps.ListPortalsResponsePortal{}},
			}, nil)

		mockAPIAPI.On("ListApis", mock.Anything, mock.Anything).
			Return(&kkOps.ListApisResponse{
				StatusCode:      200,
				ListAPIResponse: &kkComps.ListAPIResponse{Data: []kkComps.APIResponseSchema{}},
			}, nil)

		mockAPIAPI.On("ListAPIVersions", mock.Anything, mock.Anything).
			Return(&kkOps.ListAPIVersionsResponse{
				StatusCode: 200,
				ListAPIVersionResponse: &kkComps.ListAPIVersionResponse{
					Data: []kkComps.ListAPIVersionResponseAPIVersionSummary{},
				},
			}, nil)

		mockAPIAPI.On("ListAPIPublications", mock.Anything, mock.Anything).
			Return(&kkOps.ListAPIPublicationsResponse{
				StatusCode:                 200,
				ListAPIPublicationResponse: &kkComps.ListAPIPublicationResponse{Data: []kkComps.APIPublication{}},
			}, nil)

		mockAPIAPI.On("ListAPIImplementations", mock.Anything, mock.Anything).
			Return(&kkOps.ListAPIImplementationsResponse{
				StatusCode: 200,
				ListAPIImplementationResponse: &kkComps.ListAPIImplementationResponse{
					Data: []kkComps.APIImplementation{},
				},
			}, nil)

		mockAPIAPI.On("ListAPIDocuments", mock.Anything, mock.Anything).
			Return(&kkOps.ListAPIDocumentsResponse{
				StatusCode: 200,
				ListAPIDocumentResponse: &kkComps.ListAPIDocumentResponse{
					Data: []kkComps.APIDocumentSummaryWithChildren{},
				},
			}, nil)

		// Test planning (core functionality should work regardless of SDK mode)
		stateClient := state.NewClientWithAPIs(mockPortalAPI, mockAPIAPI)
		p := planner.NewPlanner(stateClient, slog.Default())

		plan, err := p.GeneratePlan(ctx, resourceSet, planner.Options{Mode: planner.PlanModeApply})
		require.NoError(t, err)
		require.NotNil(t, plan)
		assert.NotNil(t, plan)
		assert.NotEmpty(t, plan.Changes)

		// Verify plan contains expected operations
		assert.Len(t, plan.Operations, 2) // Portal + API

		mockPortalAPI.AssertExpectations(t)
		mockAPIAPI.AssertExpectations(t)
	})
}

// GetMockControlPlaneAPI helper function (missing from previous tests)
func GetMockControlPlaneAPI(ctx context.Context, _ *testing.T) *MockControlPlaneAPI {
	sdk := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
	konnectSDK, _ := sdk(GetTestConfig(), nil)
	mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
	return mockSDK.GetControlPlaneAPI().(*MockControlPlaneAPI)
}

// MockControlPlaneAPI for testing (if not already defined)
type MockControlPlaneAPI struct {
	mock.Mock
	t *testing.T
}

func NewMockControlPlaneAPI(t *testing.T) *MockControlPlaneAPI {
	return &MockControlPlaneAPI{t: t}
}

func (m *MockControlPlaneAPI) ListControlPlanes(
	ctx context.Context,
	request kkOps.ListControlPlanesRequest,
) (*kkOps.ListControlPlanesResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.ListControlPlanesResponse), args.Error(1)
}

func (m *MockControlPlaneAPI) CreateControlPlane(
	ctx context.Context,
	request kkComps.CreateControlPlaneRequest,
) (*kkOps.CreateControlPlaneResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.CreateControlPlaneResponse), args.Error(1)
}

func (m *MockControlPlaneAPI) UpdateControlPlane(
	ctx context.Context,
	id string,
	request kkComps.UpdateControlPlaneRequest,
) (*kkOps.UpdateControlPlaneResponse, error) {
	args := m.Called(ctx, id, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.UpdateControlPlaneResponse), args.Error(1)
}

func (m *MockControlPlaneAPI) DeleteControlPlane(
	ctx context.Context,
	id string,
) (*kkOps.DeleteControlPlaneResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.DeleteControlPlaneResponse), args.Error(1)
}
