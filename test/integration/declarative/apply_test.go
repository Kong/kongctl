//go:build integration
// +build integration

package declarative_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kong/kongctl/internal/cmd/root/verbs/apply"
	kongctlconfig "github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/konnect/helpers"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestApplyCommand_BasicWorkflow(t *testing.T) {
	// Create a config directory and file to satisfy Viper requirements
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "config.yaml")
	configContent := `
profiles:
  default:
    konnect:
      pat: test-token
      base_url: https://global.api.konghq.com
`
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0600))
	
	// Create a test plan file
	tempDir := t.TempDir()
	planFile := filepath.Join(tempDir, "test-plan.json")
	
	plan := planner.Plan{
		Metadata: planner.PlanMetadata{
			Version:     "1.0",
			GeneratedAt: time.Now(),
			Generator:   "kongctl/test",
			Mode:        planner.PlanModeApply,
		},
		Changes: []planner.PlannedChange{
			{
				ID:           "1:c:portal:test",
				ResourceType: "portal",
				ResourceRef:  "test-portal",
				Action:       planner.ActionCreate,
				Fields: map[string]interface{}{
					"name":        "Test Portal",
					"description": "Test portal for integration testing",
				},
			},
		},
		ExecutionOrder: []string{"1:c:portal:test"},
		Summary: planner.PlanSummary{
			TotalChanges: 1,
			ByAction:     map[planner.ActionType]int{planner.ActionCreate: 1},
			ByResource:   map[string]int{"portal": 1},
		},
	}
	
	planData, err := json.Marshal(plan)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(planFile, planData, 0600))
	
	// Set up mock SDK factory
	cleanup := WithMockSDKFactory(t, func(t *testing.T) helpers.SDKAPIFactory {
		return func(_ kongctlconfig.Hook, _ *slog.Logger) (helpers.SDKAPI, error) {
			mockPortalAPI := NewMockPortalAPI(t)
			
			// Mock successful portal creation
			mockPortalAPI.On("CreatePortal", mock.Anything, mock.Anything).
				Return(&kkOps.CreatePortalResponse{
					StatusCode: 201,
					PortalResponse: &kkComps.PortalResponse{
						ID:          "portal-123",
						Name:        "Test Portal",
						Description: stringPtr("Test portal for integration testing"),
						Labels: map[string]string{
							"KONGCTL-managed":      "true",
							"KONGCTL-last-updated": "20240101-120000Z",
						},
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				}, nil)
			
			return &helpers.MockKonnectSDK{
				T: t,
				PortalFactory: func() helpers.PortalAPI {
					return mockPortalAPI
				},
			}, nil
		}
	})
	defer cleanup()
	
	// Create apply command
	cmd, err := apply.NewApplyCmd()
	require.NoError(t, err)
	
	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	
	// Execute with plan file - using konnect subcommand
	cmd.SetArgs([]string{"konnect", "--plan", planFile, "--pat", "test-token"})
	err = cmd.Execute()
	
	// Verify successful execution
	assert.NoError(t, err)
	assert.Contains(t, output.String(), "Execution completed successfully")
	assert.Contains(t, output.String(), "Created: 1")
}

func TestApplyCommand_RejectsDeletes(t *testing.T) {
	// Create a plan with DELETE operation
	tempDir := t.TempDir()
	planFile := filepath.Join(tempDir, "test-plan.json")
	
	plan := planner.Plan{
		Metadata: planner.PlanMetadata{
			Version:     "1.0",
			GeneratedAt: time.Now(),
			Generator:   "kongctl/test",
			Mode:        planner.PlanModeSync, // Sync mode supports deletes
		},
		Changes: []planner.PlannedChange{
			{
				ID:           "1:d:portal:test",
				ResourceType: "portal",
				ResourceRef:  "test-portal",
				Action: planner.ActionDelete,
			},
		},
		ExecutionOrder: []string{"1:d:portal:test"},
		Summary: planner.PlanSummary{
			TotalChanges: 1,
			ByAction:     map[planner.ActionType]int{planner.ActionDelete: 1},
			ByResource:   map[string]int{"portal": 1},
		},
	}
	
	planData, err := json.Marshal(plan)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(planFile, planData, 0600))
	
	// Set up mock SDK factory (shouldn't be called)
	cleanup := WithMockSDKFactory(t, func(t *testing.T) helpers.SDKAPIFactory {
		return func(_ kongctlconfig.Hook, _ *slog.Logger) (helpers.SDKAPI, error) {
			// Should not reach here - apply should reject DELETE operations
			t.Fatal("SDK factory should not be called for plans with DELETE operations")
			return nil, nil
		}
	})
	defer cleanup()
	
	// Create apply command
	cmd, err := apply.NewApplyCmd()
	require.NoError(t, err)
	
	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	
	// Execute with plan file - using konnect subcommand
	cmd.SetArgs([]string{"konnect", "--plan", planFile, "--pat", "test-token"})
	err = cmd.Execute()
	
	// Verify rejection of DELETE operations
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "contains DELETE operations")
	assert.Contains(t, err.Error(), "Use 'sync' command")
}

func TestApplyCommand_DryRun(t *testing.T) {
	// Create a test plan file
	tempDir := t.TempDir()
	planFile := filepath.Join(tempDir, "test-plan.json")
	
	plan := planner.Plan{
		Metadata: planner.PlanMetadata{
			Version:     "1.0",
			GeneratedAt: time.Now(),
			Generator:   "kongctl/test",
			Mode:        planner.PlanModeApply,
		},
		Changes: []planner.PlannedChange{
			{
				ID:           "1:c:portal:test",
				ResourceType: "portal",
				ResourceRef:  "test-portal",
				Action:       planner.ActionCreate,
				Fields: map[string]interface{}{
					"name":        "Test Portal",
					"description": "Test portal for dry-run testing",
				},
			},
		},
		ExecutionOrder: []string{"1:c:portal:test"},
		Summary: planner.PlanSummary{
			TotalChanges: 1,
			ByAction:     map[planner.ActionType]int{planner.ActionCreate: 1},
			ByResource:   map[string]int{"portal": 1},
		},
	}
	
	planData, err := json.Marshal(plan)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(planFile, planData, 0600))
	
	// Set up mock SDK factory (should not make actual API calls in dry-run)
	cleanup := WithMockSDKFactory(t, func(t *testing.T) helpers.SDKAPIFactory {
		return func(_ kongctlconfig.Hook, _ *slog.Logger) (helpers.SDKAPI, error) {
			mockPortalAPI := NewMockPortalAPI(t)
			
			// No API calls should be made in dry-run mode
			
			return &helpers.MockKonnectSDK{
				T: t,
				PortalFactory: func() helpers.PortalAPI {
					return mockPortalAPI
				},
			}, nil
		}
	})
	defer cleanup()
	
	// Create apply command
	cmd, err := apply.NewApplyCmd()
	require.NoError(t, err)
	
	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	
	// Execute with plan file and dry-run flag - using konnect subcommand
	cmd.SetArgs([]string{"konnect", "--plan", planFile, "--dry-run", "--pat", "test-token"})
	err = cmd.Execute()
	
	// Verify dry-run execution
	assert.NoError(t, err)
	assert.Contains(t, output.String(), "DRY-RUN MODE")
	assert.Contains(t, output.String(), "Skipped: 1")
	assert.NotContains(t, output.String(), "Created: 1")
}

