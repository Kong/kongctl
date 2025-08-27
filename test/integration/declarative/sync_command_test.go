//go:build integration
// +build integration

package declarative_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/declarative"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSyncCommand_WithDeletes(t *testing.T) {
	// Create a test configuration file that will cause deletions
	syncDir := t.TempDir()
	syncConfigFile := filepath.Join(syncDir, "sync-config.yaml")

	// Empty config will delete all managed resources
	syncConfigContent := `# Empty configuration - will delete all managed resources
`

	err := os.WriteFile(syncConfigFile, []byte(syncConfigContent), 0o600)
	require.NoError(t, err)

	// Set up test context with mocks
	ctx := SetupTestContext(t)

	// Get the mock SDK and set up expectations
	sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
	konnectSDK, _ := sdkFactory(GetTestConfig(), nil)
	mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
	mockPortalAPI := mockSDK.GetPortalAPI().(*MockPortalAPI)
	mockAPIAPI := mockSDK.GetAPIAPI().(*MockAPIAPI)
	mockAppAuthAPI := mockSDK.GetAppAuthStrategiesAPI().(*MockAppAuthStrategiesAPI)

	// Mock listing existing managed portals
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).
		Return(&kkOps.ListPortalsResponse{
			StatusCode: 200,
			ListPortalsResponse: &kkComps.ListPortalsResponse{
				Data: []kkComps.Portal{
					{
						ID:          "portal-to-delete",
						Name:        "Old Portal",
						Description: stringPtr("This portal should be deleted"),
						Labels: map[string]string{
							"KONGCTL-managed":   "true",
							"KONGCTL-namespace": "default",
						},
						CreatedAt: time.Now().Add(-24 * time.Hour),
						UpdatedAt: time.Now().Add(-24 * time.Hour),
					},
				},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{
						Total: 1,
					},
				},
			},
		}, nil)

	// Mock successful portal deletion
	mockPortalAPI.On("DeletePortal", mock.Anything, "portal-to-delete", true).
		Return(&kkOps.DeletePortalResponse{
			StatusCode: 204,
		}, nil).Once()

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
		}, nil)

	// Mock empty auth strategies list
	mockAppAuthAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).
		Return(&kkOps.ListAppAuthStrategiesResponse{
			ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
				Data: []kkComps.AppAuthStrategy{},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{
						Total: 0,
					},
				},
			},
		}, nil)

	// Create sync command using declarative command
	cmd, err := declarative.NewDeclarativeCmd("sync")
	require.NoError(t, err)

	// Set context
	cmd.SetContext(ctx)

	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	// Mock stdin for confirmation (simulate user typing "yes")
	cmd.SetIn(strings.NewReader("yes\n"))

	// Execute with config file
	cmd.SetArgs([]string{"-f", syncConfigFile, "--auto-approve"})
	err = cmd.Execute()

	// Verify successful execution with deletions
	assert.NoError(t, err)
	outputStr := output.String()

	// Verify plan was generated with new enhanced format
	assert.Contains(t, outputStr, "SUMMARY")
	assert.Contains(t, outputStr, "- Old Portal")
	assert.Contains(t, outputStr, "portal (1 resources):")

	// Verify no confirmation prompt since we used --auto-approve
	assert.NotContains(t, outputStr, "Do you want to proceed")

	// Verify execution completed
	assert.Contains(t, outputStr, "Complete.")
	assert.Contains(t, outputStr, "Applied 1 changes.")
}
