//go:build integration
// +build integration

package declarative_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/products/konnect/declarative"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/konnect/helpers"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPlanCommand_FileTagLoading(t *testing.T) {
	// Create test configuration directory
	configDir := t.TempDir()
	
	// Create external content file
	externalContent := `
description: "This content was loaded from an external file"
version: "1.0.0"
`
	externalFile := filepath.Join(configDir, "external.yaml")
	require.NoError(t, os.WriteFile(externalFile, []byte(externalContent), 0600))
	
	// Create main configuration file with file tags
	config := `
portals:
  - ref: test-portal
    name: "Test Portal"
    display_name: !file ./external.yaml#version
`
	configFile := filepath.Join(configDir, "portal.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
	
	// Set up test context with mocks
	ctx := SetupTestContext(t)
	
	// Get the mock portal API and set up expectations
	sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
	konnectSDK, _ := sdkFactory(GetTestConfig(), nil)
	mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
	mockPortalAPI := mockSDK.GetPortalAPI().(*MockPortalAPI)
	
	// Mock empty portals list (no existing portals)
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).
		Return(&kkOps.ListPortalsResponse{
			ListPortalsResponse: &kkComps.ListPortalsResponse{
				Data: []kkComps.Portal{},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{
						Total: 0,
					},
				},
			},
		}, nil)
	
	// Get the mock auth strategies API and set up expectations
	mockAuthAPI := mockSDK.GetAppAuthStrategiesAPI().(*MockAppAuthStrategiesAPI)
	// Mock empty auth strategies list
	mockAuthAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).
		Return(&kkOps.ListAppAuthStrategiesResponse{
			StatusCode: 200,
			ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
				Data: []kkComps.AppAuthStrategy{},
			},
		}, nil).Maybe()
	
	// Get the mock API API and set up expectations
	mockAPIAPI := mockSDK.GetAPIAPI().(*MockAPIAPI)
	// Mock empty APIs list
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
		}, nil).Maybe()
	
	// Create plan command
	planCmd, err := declarative.NewDeclarativeCmd("plan")
	require.NoError(t, err)
	
	planCmd.SetContext(ctx)
	
	// Capture output
	planFile := filepath.Join(t.TempDir(), "plan.json")
	planCmd.SetArgs([]string{"-f", configFile, "--output-file", planFile})
	
	// Execute command
	err = planCmd.Execute()
	require.NoError(t, err)
	
	// Verify plan file exists and parse it
	planData, err := os.ReadFile(planFile)
	require.NoError(t, err)
	
	var plan planner.Plan
	require.NoError(t, json.Unmarshal(planData, &plan))
	
	// Verify plan structure
	assert.Equal(t, "1.0", plan.Metadata.Version)
	assert.NotEmpty(t, plan.Metadata.GeneratedAt)
	
	// Verify changes - the file tag should have been processed
	assert.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	
	assert.Equal(t, planner.ActionCreate, change.Action)
	assert.Equal(t, "portal", change.ResourceType)
	assert.Equal(t, "test-portal", change.ResourceRef)
	assert.Equal(t, "Test Portal", change.Fields["name"])
	
	// The display_name should have been loaded from the external file
	assert.Equal(t, "1.0.0", change.Fields["display_name"])
}

func TestPlanCommand_FileTagLoadingInSubdirectory(t *testing.T) {
	// Create test configuration directory structure
	configDir := t.TempDir()
	subDir := filepath.Join(configDir, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	
	// Create external content file in subdirectory
	externalContent := `
api_spec: |
  openapi: 3.0.0
  info:
    title: Test API
    version: 1.0.0
  paths: {}
metadata:
  environment: production
`
	externalFile := filepath.Join(subDir, "api-spec.yaml")
	require.NoError(t, os.WriteFile(externalFile, []byte(externalContent), 0600))
	
	// Create API configuration file in subdirectory with relative file reference
	config := `
apis:
  - ref: test-api
    name: "Test API"
    description: !file ./api-spec.yaml#metadata.environment
`
	configFile := filepath.Join(subDir, "api.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
	
	// Set up test context with mocks
	ctx := SetupTestContext(t)
	
	// Get the mock SDK and set up expectations
	sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
	konnectSDK, _ := sdkFactory(GetTestConfig(), nil)
	mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
	mockPortalAPI := mockSDK.GetPortalAPI().(*MockPortalAPI)
	mockAPIAPI := mockSDK.GetAPIAPI().(*MockAPIAPI)
	
	// Mock empty portals list
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).
		Return(&kkOps.ListPortalsResponse{
			ListPortalsResponse: &kkComps.ListPortalsResponse{
				Data: []kkComps.Portal{},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{
						Total: 0,
					},
				},
			},
		}, nil)
	
	// Mock empty APIs list
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
		}, nil).Maybe()
	
	// Get the mock auth strategies API and set up expectations
	mockAuthAPI := mockSDK.GetAppAuthStrategiesAPI().(*MockAppAuthStrategiesAPI)
	// Mock empty auth strategies list
	mockAuthAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).
		Return(&kkOps.ListAppAuthStrategiesResponse{
			StatusCode: 200,
			ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
				Data: []kkComps.AppAuthStrategy{},
			},
		}, nil).Maybe()
	
	// Create plan command
	planCmd, err := declarative.NewDeclarativeCmd("plan")
	require.NoError(t, err)
	
	planCmd.SetContext(ctx)
	
	// Capture output
	planFile := filepath.Join(t.TempDir(), "plan.json")
	planCmd.SetArgs([]string{"-f", configFile, "--output-file", planFile})
	
	// Execute command
	err = planCmd.Execute()
	require.NoError(t, err)
	
	// Verify plan file exists and parse it
	planData, err := os.ReadFile(planFile)
	require.NoError(t, err)
	
	var plan planner.Plan
	require.NoError(t, json.Unmarshal(planData, &plan))
	
	// Verify plan structure
	assert.Equal(t, "1.0", plan.Metadata.Version)
	
	// Verify changes - the file tag should have been processed with correct relative path
	assert.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	
	assert.Equal(t, planner.ActionCreate, change.Action)
	assert.Equal(t, "api", change.ResourceType)
	assert.Equal(t, "test-api", change.ResourceRef)
	assert.Equal(t, "Test API", change.Fields["name"])
	
	// The description should have been loaded from the external file using extraction
	assert.Equal(t, "production", change.Fields["description"])
}

func TestPlanCommand_FileTagLoadingWithDirectoryRecursive(t *testing.T) {
	// Create test configuration directory structure
	configDir := t.TempDir()
	subDir := filepath.Join(configDir, "apis")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	
	// Create external data file with spec info
	specContent := `
spec_info:
  title: "External API Spec"
  version: "2.1.0"
  description: "This is loaded from an external specification file"
`
	specFile := filepath.Join(subDir, "spec_data.yaml")
	require.NoError(t, os.WriteFile(specFile, []byte(specContent), 0600))
	
	// Create portal config in root
	portalConfig := `
portals:
  - ref: main-portal
    name: "Main Portal"
    description: "Main portal for APIs"
`
	portalFile := filepath.Join(configDir, "portal.yaml")
	require.NoError(t, os.WriteFile(portalFile, []byte(portalConfig), 0600))
	
	// Create API config in subdirectory with file tag
	apiConfig := `
apis:
  - ref: external-api
    name: !file ./spec_data.yaml#spec_info.title
    description: !file ./spec_data.yaml#spec_info.description
`
	apiFile := filepath.Join(subDir, "api.yaml")
	require.NoError(t, os.WriteFile(apiFile, []byte(apiConfig), 0600))
	
	// Set up test context with mocks
	ctx := SetupTestContext(t)
	
	// Get the mock SDK and set up expectations
	sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
	konnectSDK, _ := sdkFactory(GetTestConfig(), nil)
	mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
	mockPortalAPI := mockSDK.GetPortalAPI().(*MockPortalAPI)
	mockAPIAPI := mockSDK.GetAPIAPI().(*MockAPIAPI)
	
	// Mock empty portals list
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).
		Return(&kkOps.ListPortalsResponse{
			ListPortalsResponse: &kkComps.ListPortalsResponse{
				Data: []kkComps.Portal{},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{
						Total: 0,
					},
				},
			},
		}, nil)
	
	// Mock empty APIs list
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
		}, nil).Maybe()
	
	// Get the mock auth strategies API and set up expectations
	mockAuthAPI := mockSDK.GetAppAuthStrategiesAPI().(*MockAppAuthStrategiesAPI)
	// Mock empty auth strategies list
	mockAuthAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).
		Return(&kkOps.ListAppAuthStrategiesResponse{
			StatusCode: 200,
			ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
				Data: []kkComps.AppAuthStrategy{},
			},
		}, nil).Maybe()
	
	// Create plan command
	planCmd, err := declarative.NewDeclarativeCmd("plan")
	require.NoError(t, err)
	
	planCmd.SetContext(ctx)
	
	// Capture output - load specific files
	planFile := filepath.Join(t.TempDir(), "plan.json")
	planCmd.SetArgs([]string{"-f", portalFile, "-f", apiFile, "--output-file", planFile})
	
	// Execute command
	err = planCmd.Execute()
	require.NoError(t, err)
	
	// Verify plan file exists and parse it
	planData, err := os.ReadFile(planFile)
	require.NoError(t, err)
	
	var plan planner.Plan
	require.NoError(t, json.Unmarshal(planData, &plan))
	
	// Verify plan structure
	assert.Equal(t, "1.0", plan.Metadata.Version)
	
	// Should have 2 changes: 1 portal + 1 API
	assert.Len(t, plan.Changes, 2)
	
	// Find the API change
	var apiChange *planner.PlannedChange
	for i := range plan.Changes {
		if plan.Changes[i].ResourceType == "api" {
			apiChange = &plan.Changes[i]
			break
		}
	}
	require.NotNil(t, apiChange, "API change should be present")
	
	// Verify API fields were loaded from external file
	assert.Equal(t, planner.ActionCreate, apiChange.Action)
	assert.Equal(t, "external-api", apiChange.ResourceRef)
	assert.Equal(t, "External API Spec", apiChange.Fields["name"])
	assert.Equal(t, "This is loaded from an external specification file", apiChange.Fields["description"])
}