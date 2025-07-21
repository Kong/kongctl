//go:build integration
// +build integration

package declarative_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kong/kongctl/internal/cmd/root/verbs/plan"
	"github.com/kong/kongctl/internal/cmd/root/verbs/diff"
	kongctlconfig "github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/konnect/helpers"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// TODO: Fix mock injection for command-level tests
// Issue: When executing commands via cobra.Command.Execute(), the command creates its own
// context and SDK factory, overriding test mocks. This causes "unexpected method call" errors.
// 
// Recommended fix: Mock at the SDK factory level by overriding helpers.DefaultSDKFactory
// See docs/plan/004-dec-cfg-multi-resource/test-refactoring-todo.md for detailed proposal
func TestPlanGeneration_CreatePortal(t *testing.T) {
	t.Skip("Temporarily disabled - mock injection issue with command execution")
	// Create test configuration
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "portal.yaml")
	
	config := `
portals:
  - ref: test-portal
    name: "Test Portal"
    description: "Integration test portal"
    display_name: "Test Display"
`
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
	
	// Create plan command
	planCmd, err := plan.NewPlanCmd()
	require.NoError(t, err)
	
	// Set up test context with all necessary values
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
	
	// Override the PersistentPreRunE to preserve our test SDK factory
	originalPreRun := planCmd.PersistentPreRunE
	planCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Run original pre-run first
		if originalPreRun != nil {
			if err := originalPreRun(cmd, args); err != nil {
				return err
			}
		}
		// Now restore our test SDK factory
		cmd.SetContext(ctx)
		return nil
	}
	
	planCmd.SetContext(ctx)
	
	// Capture output
	var output bytes.Buffer
	planCmd.SetOut(&output)
	planCmd.SetErr(&output)
	
	// Generate plan to JSON
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
	assert.Contains(t, plan.Metadata.Generator, "kongctl")
	
	// Verify changes
	assert.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	
	assert.Equal(t, "1:c:portal:test-portal", change.ID)
	assert.Equal(t, planner.ActionCreate, change.Action)
	assert.Equal(t, "portal", change.ResourceType)
	assert.Equal(t, "test-portal", change.ResourceRef)
	assert.Equal(t, "Test Portal", change.Fields["name"])
	assert.Equal(t, "Integration test portal", change.Fields["description"])
	assert.Equal(t, "Test Display", change.Fields["display_name"])
	
	// Verify summary
	assert.Equal(t, 1, plan.Summary.TotalChanges)
	assert.Equal(t, 1, plan.Summary.ByAction[planner.ActionCreate])
	assert.Equal(t, 1, plan.Summary.ByResource["portal"])
}

func TestPlanGeneration_UpdatePortal(t *testing.T) {
	// Create test configuration
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "portal.yaml")
	
	config := `
portals:
  - ref: existing-portal
    name: "Existing Portal"
    description: "Updated description"
    display_name: "Updated Display"
`
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
	
	// Create plan command
	planCmd, err := plan.NewPlanCmd()
	require.NoError(t, err)
	
	// Set up SDK factory with existing portal
	sdkFactory := helpers.SDKAPIFactory(func(_ kongctlconfig.Hook, _ *slog.Logger) (helpers.SDKAPI, error) {
		mockPortal := NewMockPortalAPI(t)
		
		// Mock existing portal with different values
		existingID := "portal-123"
		existingName := "Existing Portal"
		oldDesc := "Old description"
		oldDisplay := "Old Display"
		
		mockPortal.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
			ListPortalsResponse: &kkComps.ListPortalsResponse{
				Data: []kkComps.Portal{
					{
						ID:          existingID,
						Name:        existingName,
						Description: &oldDesc,
						DisplayName: oldDisplay,
						Labels: map[string]string{
							labels.ManagedKey:     "true",
							labels.LastUpdatedKey: "20240101-120000Z",
						},
					},
				},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{
						Total: 1,
					},
				},
			},
		}, nil)
		
		return &helpers.MockKonnectSDK{
			T:             t,
			PortalFactory: func() helpers.PortalAPI { return mockPortal },
		}, nil
	})
	
	// Set up test context
	ctx := SetupTestContext(t)
	// Override SDK factory
	ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, sdkFactory)
	
	// Override the PersistentPreRunE to preserve our test SDK factory
	originalPreRun := planCmd.PersistentPreRunE
	planCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Run original pre-run first
		if originalPreRun != nil {
			if err := originalPreRun(cmd, args); err != nil {
				return err
			}
		}
		// Now restore our test SDK factory
		cmd.SetContext(ctx)
		return nil
	}
	
	planCmd.SetContext(ctx)
	
	// Capture output
	var output bytes.Buffer
	planCmd.SetOut(&output)
	planCmd.SetErr(&output)
	
	// Generate plan
	planFile := filepath.Join(t.TempDir(), "plan.json")
	planCmd.SetArgs([]string{"-f", configFile, "--output-file", planFile})
	
	// Execute command
	err = planCmd.Execute()
	require.NoError(t, err)
	
	// Verify plan
	planData, err := os.ReadFile(planFile)
	require.NoError(t, err)
	
	var plan planner.Plan
	require.NoError(t, json.Unmarshal(planData, &plan))
	
	// Verify UPDATE change
	assert.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	
	assert.Equal(t, planner.ActionUpdate, change.Action)
	assert.Equal(t, "portal", change.ResourceType)
	assert.Equal(t, "existing-portal", change.ResourceRef)
	assert.Equal(t, "portal-123", change.ResourceID)
	
	// Verify field changes - we now store the new values directly
	assert.Equal(t, "Updated description", change.Fields["description"])
	assert.Equal(t, "Updated Display", change.Fields["display_name"])
}

func TestPlanGeneration_ProtectionChange(t *testing.T) {
	// Create test configuration with protection enabled
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "portal.yaml")
	
	config := `
portals:
  - ref: protected-portal
    name: "Protected Portal"
    description: "Portal with protection"
    kongctl:
      protected: true
`
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
	
	// Create plan command
	planCmd, err := plan.NewPlanCmd()
	require.NoError(t, err)
	
	// Set up SDK factory with unprotected portal
	sdkFactory := helpers.SDKAPIFactory(func(_ kongctlconfig.Hook, _ *slog.Logger) (helpers.SDKAPI, error) {
		mockPortal := NewMockPortalAPI(t)
		
		// Mock existing unprotected portal
		existingID := "portal-456"
		existingName := "Protected Portal"
		desc := "Portal with protection"
		
		mockPortal.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
			ListPortalsResponse: &kkComps.ListPortalsResponse{
				Data: []kkComps.Portal{
					{
						ID:          existingID,
						Name:        existingName,
						Description: &desc,
						Labels: map[string]string{
							labels.ManagedKey:     "true",
							labels.LastUpdatedKey: "20240101-120000Z",
							// No protected label = unprotected
						},
					},
				},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{
						Total: 1,
					},
				},
			},
		}, nil)
		
		return &helpers.MockKonnectSDK{
			T:             t,
			PortalFactory: func() helpers.PortalAPI { return mockPortal },
		}, nil
	})
	
	// Set up test context
	ctx := SetupTestContext(t)
	// Override SDK factory
	ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, sdkFactory)
	
	// Override the PersistentPreRunE to preserve our test SDK factory
	originalPreRun := planCmd.PersistentPreRunE
	planCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Run original pre-run first
		if originalPreRun != nil {
			if err := originalPreRun(cmd, args); err != nil {
				return err
			}
		}
		// Now restore our test SDK factory
		cmd.SetContext(ctx)
		return nil
	}
	
	planCmd.SetContext(ctx)
	
	// Generate plan
	planFile := filepath.Join(t.TempDir(), "plan.json")
	planCmd.SetArgs([]string{"-f", configFile, "--output-file", planFile})
	
	err = planCmd.Execute()
	require.NoError(t, err)
	
	// Verify plan has protection change
	planData, err := os.ReadFile(planFile)
	require.NoError(t, err)
	
	var plan planner.Plan
	require.NoError(t, json.Unmarshal(planData, &plan))
	
	// Should have one UPDATE for protection change
	assert.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	
	assert.Equal(t, planner.ActionUpdate, change.Action)
	assert.Equal(t, "1:u:portal:protected-portal", change.ID)
	
	// Verify protection change (marshaled as map from JSON)
	protChange, ok := change.Protection.(map[string]interface{})
	require.True(t, ok, "Protection should be a map after JSON unmarshaling")
	assert.False(t, protChange["old"].(bool))
	assert.True(t, protChange["new"].(bool))
	
	// Protection change includes name and labels fields
	assert.NotEmpty(t, change.Fields)
	assert.Equal(t, "Protected Portal", change.Fields["name"])
	
	// Protection status is tracked in change.Protection, not in labels field
	// The labels field should be updated but without KONGCTL-protected label
}

func TestPlanGeneration_EmptyPlan(t *testing.T) {
	// Create test configuration matching existing state
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "portal.yaml")
	
	config := `
portals:
  - ref: existing-portal
    name: "Existing Portal"
    description: "Same description"
`
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
	
	// Create plan command
	planCmd, err := plan.NewPlanCmd()
	require.NoError(t, err)
	
	// Set up SDK factory with matching portal
	sdkFactory := helpers.SDKAPIFactory(func(_ kongctlconfig.Hook, _ *slog.Logger) (helpers.SDKAPI, error) {
		mockPortal := NewMockPortalAPI(t)
		
		// Mock portal that matches desired state
		portal := CreateManagedPortal("Existing Portal", "portal-789", "Same description")
		
		mockPortal.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
			ListPortalsResponse: &kkComps.ListPortalsResponse{
				Data: []kkComps.Portal{portal},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{
						Total: 1,
					},
				},
			},
		}, nil)
		
		return &helpers.MockKonnectSDK{
			T:             t,
			PortalFactory: func() helpers.PortalAPI { return mockPortal },
		}, nil
	})
	
	// Set up test context
	ctx := SetupTestContext(t)
	// Override SDK factory
	ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, sdkFactory)
	
	// Override the PersistentPreRunE to preserve our test SDK factory
	originalPreRun := planCmd.PersistentPreRunE
	planCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Run original pre-run first
		if originalPreRun != nil {
			if err := originalPreRun(cmd, args); err != nil {
				return err
			}
		}
		// Now restore our test SDK factory
		cmd.SetContext(ctx)
		return nil
	}
	
	planCmd.SetContext(ctx)
	
	// Generate plan
	planFile := filepath.Join(t.TempDir(), "plan.json")
	planCmd.SetArgs([]string{"-f", configFile, "--output-file", planFile})
	
	err = planCmd.Execute()
	require.NoError(t, err)
	
	// Verify empty plan
	planData, err := os.ReadFile(planFile)
	require.NoError(t, err)
	
	var plan planner.Plan
	require.NoError(t, json.Unmarshal(planData, &plan))
	
	assert.True(t, plan.IsEmpty())
	assert.Len(t, plan.Changes, 0)
	assert.Equal(t, 0, plan.Summary.TotalChanges)
}

func TestDiffCommand_TextOutput(t *testing.T) {
	// Create a plan file
	plan := planner.Plan{
		Metadata: planner.PlanMetadata{
			Version:     "1.0",
			GeneratedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			Generator:   "kongctl/test",
			Mode:        planner.PlanModeSync,
		},
		Changes: []planner.PlannedChange{
			{
				ID:           "1:c:portal:new-portal",
				ResourceType: "portal",
				ResourceRef:  "new-portal",
				Action:       planner.ActionCreate,
				Fields: map[string]interface{}{
					"name":        "New Portal",
					"description": "A new portal",
				},
			},
		},
		ExecutionOrder: []string{"1:c:portal:new-portal"},
		Summary: planner.PlanSummary{
			TotalChanges: 1,
			ByAction:     map[planner.ActionType]int{planner.ActionCreate: 1},
			ByResource:   map[string]int{"portal": 1},
		},
	}
	
	planData, err := json.MarshalIndent(plan, "", "  ")
	require.NoError(t, err)
	
	planFile := filepath.Join(t.TempDir(), "test-plan.json")
	require.NoError(t, os.WriteFile(planFile, planData, 0600))
	
	// Create diff command
	diffCmd, err := diff.NewDiffCmd()
	require.NoError(t, err)
	
	// Set up test context
	ctx := SetupTestContext(t)
	diffCmd.SetContext(ctx)
	
	// Capture output
	var output bytes.Buffer
	diffCmd.SetOut(&output)
	diffCmd.SetErr(&output)
	
	// Run diff with text output
	diffCmd.SetArgs([]string{"--plan", planFile})
	
	err = diffCmd.Execute()
	require.NoError(t, err)
	
	// Verify text output
	outputStr := output.String()
	assert.Contains(t, outputStr, "Plan: 1 to add, 0 to change")
	assert.Contains(t, outputStr, `+ [1:c:portal:new-portal] portal "new-portal" will be created`)
	assert.Contains(t, outputStr, `name: "New Portal"`)
	assert.Contains(t, outputStr, `description: "A new portal"`)
}

func TestDiffCommand_JSONOutput(t *testing.T) {
	// Create a plan file
	plan := planner.Plan{
		Metadata: planner.PlanMetadata{
			Version: "1.0",
			Mode: planner.PlanModeSync,
		},
		Changes: []planner.PlannedChange{
			{
				ID:           "1:u:portal:portal",
				ResourceType: "portal",
				ResourceRef:  "portal-1",
				ResourceID:   "portal-123",
				Action:       planner.ActionUpdate,
				Fields: map[string]interface{}{
					"description": planner.FieldChange{
						Old: "Old desc",
						New: "New desc",
					},
				},
			},
		},
		Summary: planner.PlanSummary{
			TotalChanges: 1,
			ByAction:     map[planner.ActionType]int{planner.ActionUpdate: 1},
			ByResource:   map[string]int{"portal": 1},
		},
	}
	
	planData, err := json.Marshal(plan)
	require.NoError(t, err)
	
	planFile := filepath.Join(t.TempDir(), "update-plan.json")
	require.NoError(t, os.WriteFile(planFile, planData, 0600))
	
	// Create diff command
	diffCmd, err := diff.NewDiffCmd()
	require.NoError(t, err)
	
	// Set up test context
	ctx := SetupTestContext(t)
	diffCmd.SetContext(ctx)
	
	// Capture output
	var output bytes.Buffer
	diffCmd.SetOut(&output)
	diffCmd.SetErr(&output)
	
	// Run diff with JSON output
	diffCmd.SetArgs([]string{"--plan", planFile, "-o", "json"})
	
	err = diffCmd.Execute()
	require.NoError(t, err)
	
	// Verify JSON output is valid
	var outputPlan planner.Plan
	require.NoError(t, json.Unmarshal(output.Bytes(), &outputPlan))
	
	// Should be pass-through of the plan
	assert.Equal(t, plan.Metadata.Version, outputPlan.Metadata.Version)
	assert.Len(t, outputPlan.Changes, 1)
	assert.Equal(t, planner.ActionUpdate, outputPlan.Changes[0].Action)
}

func TestDiffCommand_YAMLOutput(t *testing.T) {
	// Create a simple plan
	plan := planner.Plan{
		Metadata: planner.PlanMetadata{
			Version: "1.0",
			Mode: planner.PlanModeSync,
		},
		Changes:        []planner.PlannedChange{},
		ExecutionOrder: []string{},
		Summary: planner.PlanSummary{
			TotalChanges: 0,
		},
	}
	
	planData, err := json.Marshal(plan)
	require.NoError(t, err)
	
	planFile := filepath.Join(t.TempDir(), "empty-plan.json")
	require.NoError(t, os.WriteFile(planFile, planData, 0600))
	
	// Create diff command
	diffCmd, err := diff.NewDiffCmd()
	require.NoError(t, err)
	
	// Set up test context
	ctx := SetupTestContext(t)
	diffCmd.SetContext(ctx)
	
	// Capture output
	var output bytes.Buffer
	diffCmd.SetOut(&output)
	diffCmd.SetErr(&output)
	
	// Run diff with YAML output
	diffCmd.SetArgs([]string{"--plan", planFile, "-o", "yaml"})
	
	err = diffCmd.Execute()
	require.NoError(t, err)
	
	// Verify YAML output is valid
	var outputPlan planner.Plan
	require.NoError(t, yaml.Unmarshal(output.Bytes(), &outputPlan))
	
	assert.Equal(t, "1.0", outputPlan.Metadata.Version)
	assert.True(t, outputPlan.IsEmpty())
}

// TODO: Fix mock injection for command-level tests (same issue as TestPlanGeneration_CreatePortal)
// See docs/plan/004-dec-cfg-multi-resource/test-refactoring-todo.md for detailed proposal
func TestPlanDiffPipeline(t *testing.T) {
	t.Skip("Temporarily disabled - mock injection issue with command execution")
	// Create test configuration
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "portal.yaml")
	
	config := `
portals:
  - ref: pipeline-portal
    name: "Pipeline Portal"
    description: "Test piping plan to diff"
`
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
	
	// Create plan command
	planCmd, err := plan.NewPlanCmd()
	require.NoError(t, err)
	
	// Set up test context with all necessary values
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
	
	// Override the PersistentPreRunE to preserve our test SDK factory
	originalPreRun := planCmd.PersistentPreRunE
	planCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Run original pre-run first
		if originalPreRun != nil {
			if err := originalPreRun(cmd, args); err != nil {
				return err
			}
		}
		// Now restore our test SDK factory
		cmd.SetContext(ctx)
		return nil
	}
	
	planCmd.SetContext(ctx)
	
	// Capture plan output
	var planOutput bytes.Buffer
	planCmd.SetOut(&planOutput)
	planCmd.SetErr(&planOutput)
	
	// Generate plan to stdout
	planCmd.SetArgs([]string{"-f", configFile})
	
	err = planCmd.Execute()
	require.NoError(t, err)
	
	// Create diff command
	diffCmd, err := diff.NewDiffCmd()
	require.NoError(t, err)
	
	// Set context for diff command
	diffCmd.SetContext(ctx)
	
	// Capture diff output
	var diffOutput bytes.Buffer
	diffCmd.SetOut(&diffOutput)
	diffCmd.SetErr(&diffOutput)
	
	// Simulate piping by using plan output as stdin
	diffCmd.SetIn(strings.NewReader(planOutput.String()))
	
	// Run diff reading from stdin
	diffCmd.SetArgs([]string{"--plan", "-"})
	
	err = diffCmd.Execute()
	require.NoError(t, err)
	
	// Verify diff output
	outputStr := diffOutput.String()
	assert.Contains(t, outputStr, "Plan: 1 to add, 0 to change")
	assert.Contains(t, outputStr, "pipeline-portal")
	assert.Contains(t, outputStr, "Pipeline Portal")
}

