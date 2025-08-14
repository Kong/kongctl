//go:build integration
// +build integration

package declarative_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kong/kongctl/internal/cmd/root/products/konnect/declarative"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/konnect/helpers"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestApplyCommand_BasicWorkflow(t *testing.T) {
	// Create a test plan file
	planDir := t.TempDir()
	planFile := filepath.Join(planDir, "test-plan.json")
	
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
				Fields: map[string]any{
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
	
	// Set up test context with mocks
	ctx := SetupTestContext(t)
	
	// Get the mock SDK and set up expectations
	sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
	konnectSDK, _ := sdkFactory(GetTestConfig(), nil)
	mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
	mockPortalAPI := mockSDK.GetPortalAPI().(*MockPortalAPI)
	
	// Mock checking for existing portal (not found)
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).
		Return(&kkOps.ListPortalsResponse{
			StatusCode: 200,
			ListPortalsResponse: &kkComps.ListPortalsResponse{
				Data: []kkComps.Portal{},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{
						Total: 0,
					},
				},
			},
		}, nil).Maybe()
	
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
	
	// Create apply command using declarative command
	cmd, err := declarative.NewDeclarativeCmd("apply")
	require.NoError(t, err)
	
	// Set context
	cmd.SetContext(ctx)
	
	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	
	// Execute with plan file
	cmd.SetArgs([]string{"--plan", planFile, "--auto-approve"})
	err = cmd.Execute()
	
	// Verify successful execution
	assert.NoError(t, err)
	assert.Contains(t, output.String(), "Complete.")
	assert.Contains(t, output.String(), "Applied 1 changes.")
}

func TestApplyCommand_RejectsDeletes(t *testing.T) {
	// Create a plan with DELETE operation
	planDir := t.TempDir()
	planFile := filepath.Join(planDir, "test-plan.json")
	
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
	
	// Set up test context with mocks (shouldn't be called)
	ctx := SetupTestContext(t)
	
	// Create apply command using declarative command
	cmd, err := declarative.NewDeclarativeCmd("apply")
	require.NoError(t, err)
	
	// Set context
	cmd.SetContext(ctx)
	
	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	
	// Execute with plan file
	cmd.SetArgs([]string{"--plan", planFile, "--auto-approve"})
	err = cmd.Execute()
	
	// Verify rejection of DELETE operations
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "apply command cannot execute plans with DELETE operations")
	assert.Contains(t, err.Error(), "Use 'sync' command")
}

func TestApplyCommand_DryRun(t *testing.T) {
	// Create a test plan file
	planDir := t.TempDir()
	planFile := filepath.Join(planDir, "test-plan.json")
	
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
				Fields: map[string]any{
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
	
	// Set up test context with mocks (should not make actual API calls in dry-run)
	ctx := SetupTestContext(t)
	
	// Get the mock SDK and set up expectations
	sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
	konnectSDK, _ := sdkFactory(GetTestConfig(), nil)
	mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
	mockPortalAPI := mockSDK.GetPortalAPI().(*MockPortalAPI)
	
	// Mock checking for existing portal (not found) - needed even in dry-run
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).
		Return(&kkOps.ListPortalsResponse{
			StatusCode: 200,
			ListPortalsResponse: &kkComps.ListPortalsResponse{
				Data: []kkComps.Portal{},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{
						Total: 0,
					},
				},
			},
		}, nil).Maybe()
	
	// Create apply command using declarative command
	cmd, err := declarative.NewDeclarativeCmd("apply")
	require.NoError(t, err)
	
	// Set context
	cmd.SetContext(ctx)
	
	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	
	// Execute with plan file and dry-run flag
	cmd.SetArgs([]string{"--plan", planFile, "--dry-run"})
	err = cmd.Execute()
	
	// Verify dry-run execution
	assert.NoError(t, err)
	assert.Contains(t, output.String(), "Validating changes:")
	assert.Contains(t, output.String(), "Dry run complete.")
	assert.NotContains(t, output.String(), "Applied")
}

