//go:build integration
// +build integration

package declarative_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kong/kongctl/internal/cmd/root/products/konnect/declarative"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSyncFullReconciliation(t *testing.T) {
	// Create test configuration with multiple resources
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "sync.yaml")
	
	config := `
portals:
  - ref: portal-1
    name: "Portal One"
    description: "First portal"
  - ref: portal-2
    name: "Portal Two"
    description: "Second portal"
`
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
	
	// Set up test context with mocks
	ctx := SetupTestContext(t)
	
	// Get the mock SDK and set up expectations
	sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
	konnectSDK, _ := sdkFactory(GetTestConfig(), nil)
	mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
	mockPortalAPI := mockSDK.GetPortalAPI().(*MockPortalAPI)
	
	// Mock current state: one existing managed portal, one unmanaged
	existingPortal := CreateManagedPortal("Portal One", "portal-existing", "First portal")
	unmanagedPortal := CreateUnmanagedPortal("Unmanaged Portal", "unmanaged-id", "Not managed by kongctl")
	
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{existingPortal, unmanagedPortal},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 2,
				},
			},
		},
	}, nil)
	
	// Mock creation of new portal
	mockPortalAPI.On("CreatePortal", mock.Anything, mock.Anything).Return(&kkOps.CreatePortalResponse{
		StatusCode: 201,
		PortalResponse: &kkComps.PortalResponse{
			ID:          "portal-two-id",
			Name:        "Portal Two",
			Description: stringPtr("Second portal"),
			DisplayName: "",
			Labels:      make(map[string]string),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}, nil)
	
	// Set up auth and API mocks (required for state fetching)
	setupEmptyAPIMocks(mockSDK)
	
	// Create sync command
	syncCmd, err := declarative.NewDeclarativeCmd("sync")
	require.NoError(t, err)
	
	// Set context
	syncCmd.SetContext(ctx)
	
	// Capture output
	var output bytes.Buffer
	syncCmd.SetOut(&output)
	syncCmd.SetErr(&output)
	
	// Run sync with auto-approve flag (no interactive prompts)
	syncCmd.SetArgs([]string{"-f", configFile, "--auto-approve"})
	
	// Execute command
	err = syncCmd.Execute()
	require.NoError(t, err)
	
	// Verify output indicates sync completion
	outputStr := output.String()
	assert.Contains(t, outputStr, "Creating portal: portal-2")
	
	// Verify mocks were called as expected
	mockPortalAPI.AssertExpectations(t)
}

func TestSyncDeletesUnmanagedResources(t *testing.T) {
	// Create empty configuration (should delete all managed resources)
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "empty.yaml")
	
	config := `
portals: []
`
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
	
	// Set up test context with mocks
	ctx := SetupTestContext(t)
	
	// Get the mock SDK and set up expectations
	sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
	konnectSDK, _ := sdkFactory(GetTestConfig(), nil)
	mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
	mockPortalAPI := mockSDK.GetPortalAPI().(*MockPortalAPI)
	
	// Mock current state: one managed portal that should be deleted
	managedPortal := CreateManagedPortal("Managed Portal", "managed-id", "Should be deleted")
	
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{managedPortal},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)
	
	// Mock deletion
	mockPortalAPI.On("DeletePortal", mock.Anything, "managed-id", true).Return(&kkOps.DeletePortalResponse{
		StatusCode: 204,
	}, nil)
	
	// Set up auth and API mocks
	setupEmptyAPIMocks(mockSDK)
	
	// Create sync command
	syncCmd, err := declarative.NewDeclarativeCmd("sync")
	require.NoError(t, err)
	
	// Set context
	syncCmd.SetContext(ctx)
	
	// Capture output
	var output bytes.Buffer
	syncCmd.SetOut(&output)
	syncCmd.SetErr(&output)
	
	// Run sync with auto-approve flag
	syncCmd.SetArgs([]string{"-f", configFile, "--auto-approve"})
	
	// Execute command
	err = syncCmd.Execute()
	require.NoError(t, err)
	
	// Verify output indicates deletion
	outputStr := output.String()
	assert.Contains(t, outputStr, "Deleting portal: Managed Portal")
	
	// Verify mocks were called
	mockPortalAPI.AssertExpectations(t)
}

func TestSyncProtectedResourceHandling(t *testing.T) {
	// Create configuration that would modify a protected resource
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "protected.yaml")
	
	config := `
portals:
  - ref: protected-portal
    name: "Protected Portal"
    description: "Modified description"
    kongctl:
      protected: false  # Try to unprotect
`
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
	
	// Set up test context with mocks
	ctx := SetupTestContext(t)
	
	// Get the mock SDK and set up expectations
	sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
	konnectSDK, _ := sdkFactory(GetTestConfig(), nil)
	mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
	mockPortalAPI := mockSDK.GetPortalAPI().(*MockPortalAPI)
	
	// Mock current state: protected portal
	protectedPortal := CreateManagedPortal("Protected Portal", "protected-id", "Original description")
	protectedPortal.Labels[labels.ProtectedKey] = "true"
	
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{protectedPortal},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)
	
	// Set up auth and API mocks
	setupEmptyAPIMocks(mockSDK)
	
	// Create sync command
	syncCmd, err := declarative.NewDeclarativeCmd("sync")
	require.NoError(t, err)
	
	// Set context
	syncCmd.SetContext(ctx)
	
	// Capture output
	var output bytes.Buffer
	syncCmd.SetOut(&output)
	syncCmd.SetErr(&output)
	
	// Run sync - should fail due to protected resource modification
	syncCmd.SetArgs([]string{"-f", configFile})
	
	// Execute command - expect failure
	err = syncCmd.Execute()
	require.Error(t, err)
	
	// Verify sync was cancelled (this happens when protected resources would be modified)
	assert.Contains(t, err.Error(), "Cannot generate plan due to protected resources")
	
	// Verify no API calls were made for updates (fail-fast behavior)
	mockPortalAPI.AssertExpectations(t)
}

func TestSyncConfirmationFlow(t *testing.T) {
	// Create configuration that will require deletion
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "confirm.yaml")
	
	config := `
portals:
  - ref: keep-portal
    name: "Keep This"
    description: "Portal to keep"
`
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
	
	// Set up test context with mocks
	ctx := SetupTestContext(t)
	
	// Get the mock SDK and set up expectations
	sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
	konnectSDK, _ := sdkFactory(GetTestConfig(), nil)
	mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
	mockPortalAPI := mockSDK.GetPortalAPI().(*MockPortalAPI)
	
	// Mock current state: two managed portals, one to keep, one to delete
	keepPortal := CreateManagedPortal("Keep This", "keep-id", "Portal to keep")
	deletePortal := CreateManagedPortal("Delete This", "delete-id", "Portal to delete")
	
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{keepPortal, deletePortal},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 2,
				},
			},
		},
	}, nil)
	
	// Set up auth and API mocks
	setupEmptyAPIMocks(mockSDK)
	
	// Create sync command
	syncCmd, err := declarative.NewDeclarativeCmd("sync")
	require.NoError(t, err)
	
	// Set context
	syncCmd.SetContext(ctx)
	
	// Capture output
	var output bytes.Buffer
	syncCmd.SetOut(&output)
	syncCmd.SetErr(&output)
	
	// Simulate user declining confirmation by providing "n" as input
	syncCmd.SetIn(strings.NewReader("n\n"))
	
	// Run sync without auto-approve (should prompt for confirmation)
	syncCmd.SetArgs([]string{"-f", configFile})
	
	// Execute command - should abort due to declined confirmation
	err = syncCmd.Execute()
	require.Error(t, err)
	
	// Verify output shows confirmation prompt
	outputStr := output.String()
	assert.Contains(t, outputStr, "Delete This")
	
	// Verify no deletion calls were made (user declined)
	mockPortalAPI.AssertExpectations(t)
}

func TestSyncAutoApprove(t *testing.T) {
	// Create configuration that will require deletion
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "auto-approve.yaml")
	
	config := `
portals:
  - ref: new-portal
    name: "New Portal"
    description: "Newly created"
`
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
	
	// Set up test context with mocks
	ctx := SetupTestContext(t)
	
	// Get the mock SDK and set up expectations
	sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
	konnectSDK, _ := sdkFactory(GetTestConfig(), nil)
	mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
	mockPortalAPI := mockSDK.GetPortalAPI().(*MockPortalAPI)
	
	// Mock current state: one managed portal to delete
	deletePortal := CreateManagedPortal("Old Portal", "old-id", "To be deleted")
	
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{deletePortal},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)
	
	// Mock creation of new portal
	mockPortalAPI.On("CreatePortal", mock.Anything, mock.Anything).Return(&kkOps.CreatePortalResponse{
		StatusCode: 201,
		PortalResponse: &kkComps.PortalResponse{
			ID:          "new-id",
			Name:        "New Portal",
			Description: stringPtr("Newly created"),
			DisplayName: "",
			Labels:      make(map[string]string),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}, nil)
	
	// Mock deletion of old portal
	mockPortalAPI.On("DeletePortal", mock.Anything, "old-id", true).Return(&kkOps.DeletePortalResponse{
		StatusCode: 204,
	}, nil)
	
	// Set up auth and API mocks
	setupEmptyAPIMocks(mockSDK)
	
	// Create sync command
	syncCmd, err := declarative.NewDeclarativeCmd("sync")
	require.NoError(t, err)
	
	// Set context
	syncCmd.SetContext(ctx)
	
	// Capture output
	var output bytes.Buffer
	syncCmd.SetOut(&output)
	syncCmd.SetErr(&output)
	
	// Run sync with auto-approve (should not prompt)
	syncCmd.SetArgs([]string{"-f", configFile, "--auto-approve"})
	
	// Execute command
	err = syncCmd.Execute()
	require.NoError(t, err)
	
	// Verify both create and delete occurred
	outputStr := output.String()
	assert.Contains(t, outputStr, "Creating portal: new-portal")
	assert.Contains(t, outputStr, "Deleting portal: Old Portal")
	
	// Verify all mocks were called
	mockPortalAPI.AssertExpectations(t)
}

func TestSyncOutputFormats(t *testing.T) {
	testCases := []struct {
		name   string
		format string
	}{
		{"Text Output", "text"},
		{"JSON Output", "json"},
		{"YAML Output", "yaml"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test configuration
			configDir := t.TempDir()
			configFile := filepath.Join(configDir, "output.yaml")
			
			config := `
portals:
  - ref: output-portal
    name: "Output Test"
    description: "Test output formats"
`
			require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
			
			// Set up test context with mocks
			ctx := SetupTestContext(t)
			
			// Get the mock SDK and set up expectations
			sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
			konnectSDK, _ := sdkFactory(GetTestConfig(), nil)
			mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
			mockPortalAPI := mockSDK.GetPortalAPI().(*MockPortalAPI)
			
			// Mock empty current state
			mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
				ListPortalsResponse: &kkComps.ListPortalsResponse{
					Data: []kkComps.Portal{},
					Meta: kkComps.PaginatedMeta{
						Page: kkComps.PageMeta{
							Total: 0,
						},
					},
				},
			}, nil)
			
			// Mock creation
			mockPortalAPI.On("CreatePortal", mock.Anything, mock.Anything).Return(&kkOps.CreatePortalResponse{
				StatusCode: 201,
				PortalResponse: &kkComps.PortalResponse{
					ID:          "output-id",
					Name:        "Output Test",
					Description: stringPtr("Test output formats"),
					DisplayName: "",
					Labels:      make(map[string]string),
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				},
			}, nil)
			
			// Set up auth and API mocks
			setupEmptyAPIMocks(mockSDK)
			
			// Create sync command
			syncCmd, err := declarative.NewDeclarativeCmd("sync")
			require.NoError(t, err)
			
			// Set context
			syncCmd.SetContext(ctx)
			
			// Capture output
			var output bytes.Buffer
			syncCmd.SetOut(&output)
			syncCmd.SetErr(&output)
			
			// Run sync with specific output format
			args := []string{"-f", configFile, "--auto-approve"}
			if tc.format != "text" {
				args = append(args, "-o", tc.format)
			}
			syncCmd.SetArgs(args)
			
			// Execute command
			err = syncCmd.Execute()
			require.NoError(t, err)
			
			// Verify output format
			outputStr := output.String()
			switch tc.format {
			case "json":
				// Should be valid JSON
				var jsonData map[string]any
				assert.NoError(t, json.Unmarshal(output.Bytes(), &jsonData))
			case "yaml":
				// Should contain YAML-style output
				assert.Contains(t, outputStr, ":")  // YAML uses colons
			default: // text
				// Should contain human-readable text
				assert.Contains(t, outputStr, "Creating portal: output-portal")
			}
			
			mockPortalAPI.AssertExpectations(t)
		})
	}
}

func TestSyncDryRun(t *testing.T) {
	// Create test configuration
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "dry-run.yaml")
	
	config := `
portals:
  - ref: dry-run-portal
    name: "Dry Run Test"
    description: "Should not be created"
`
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
	
	// Set up test context with mocks
	ctx := SetupTestContext(t)
	
	// Get the mock SDK and set up expectations
	sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
	konnectSDK, _ := sdkFactory(GetTestConfig(), nil)
	mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
	mockPortalAPI := mockSDK.GetPortalAPI().(*MockPortalAPI)
	
	// Mock empty current state
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 0,
				},
			},
		},
	}, nil)
	
	// DO NOT mock CreatePortal - dry run should not call it
	
	// Set up auth and API mocks
	setupEmptyAPIMocks(mockSDK)
	
	// Create sync command
	syncCmd, err := declarative.NewDeclarativeCmd("sync")
	require.NoError(t, err)
	
	// Set context
	syncCmd.SetContext(ctx)
	
	// Capture output
	var output bytes.Buffer
	syncCmd.SetOut(&output)
	syncCmd.SetErr(&output)
	
	// Run sync with dry-run flag
	syncCmd.SetArgs([]string{"-f", configFile, "--dry-run"})
	
	// Execute command
	err = syncCmd.Execute()
	require.NoError(t, err)
	
	// Verify output shows what would happen but no actual changes made
	outputStr := output.String()
	assert.Contains(t, outputStr, "Creating portal: dry-run-portal")
	assert.Contains(t, outputStr, "Dry run complete") // Should indicate it's a dry run
	
	// Verify only ListPortals was called, not CreatePortal
	mockPortalAPI.AssertExpectations(t)
}

// Helper functions

func CreateUnmanagedPortal(name, id, description string) kkComps.Portal {
	return kkComps.Portal{
		ID:          id,
		Name:        name,
		Description: stringPtr(description),
		DisplayName: "",
		Labels: map[string]string{
			// No managed labels - this is unmanaged
		},
	}
}

func setupEmptyAPIMocks(mockSDK *helpers.MockKonnectSDK) {
	// Mock auth strategies API
	mockAuthAPI := mockSDK.GetAppAuthStrategiesAPI().(*MockAppAuthStrategiesAPI)
	mockAuthAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).
		Return(&kkOps.ListAppAuthStrategiesResponse{
			StatusCode: 200,
			ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
				Data: []kkComps.AppAuthStrategy{},
			},
		}, nil).Maybe()
	
	// Mock API API
	mockAPIAPI := mockSDK.GetAPIAPI().(*MockAPIAPI)
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
}

