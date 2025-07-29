//go:build integration
// +build integration

package declarative_test

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kong/kongctl/internal/cmd/root/verbs/sync"
	kongctlconfig "github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSyncCommand_WithDeletes(t *testing.T) {
	// Create a test configuration file that will cause deletions
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "sync-config.yaml")
	
	// Empty config will delete all managed resources
	configContent := `# Empty configuration - will delete all managed resources
`
	
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	require.NoError(t, err)
	
	// Set up mock SDK factory
	cleanup := WithMockSDKFactory(t, func(t *testing.T) helpers.SDKAPIFactory {
		return func(_ kongctlconfig.Hook, _ *slog.Logger) (helpers.SDKAPI, error) {
			mockPortalAPI := NewMockPortalAPI(t)
			mockAPIAPI := NewMockAPIAPI(t)
			mockAppAuthAPI := NewMockAppAuthStrategiesAPI(t)
			
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
									"KONGCTL-managed": "true",
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
				}, nil).Once()
			
			// Mock successful portal deletion
			mockPortalAPI.On("DeletePortal", mock.Anything, 
				kkOps.DeletePortalRequest{
					PortalID: "portal-to-delete",
				}).
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
			
			return &helpers.MockKonnectSDK{
				T: t,
				PortalFactory: func() helpers.PortalAPI {
					return mockPortalAPI
				},
				APIFactory: func() helpers.APIFullAPI {
					return mockAPIAPI
				},
				AppAuthStrategiesFactory: func() helpers.AppAuthStrategiesAPI {
					return mockAppAuthAPI
				},
			}, nil
		}
	})
	defer cleanup()
	
	// Create sync command
	cmd, err := sync.NewSyncCmd()
	require.NoError(t, err)
	
	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	
	// Mock stdin for confirmation (simulate user typing "yes")
	cmd.SetIn(strings.NewReader("yes\n"))
	
	// Execute with config file - using konnect subcommand
	cmd.SetArgs([]string{"konnect", "-f", configFile, "--pat", "test-token"})
	err = cmd.Execute()
	
	// Verify successful execution with deletions
	assert.NoError(t, err)
	outputStr := output.String()
	
	// Verify plan was generated
	assert.Contains(t, outputStr, "Plan Summary")
	assert.Contains(t, outputStr, "Delete: 1")
	assert.Contains(t, outputStr, "portal: Old Portal")
	
	// Verify confirmation prompt
	assert.Contains(t, outputStr, "Do you want to proceed")
	
	// Verify execution completed
	assert.Contains(t, outputStr, "Execution completed successfully")
	assert.Contains(t, outputStr, "Deleted: 1")
}