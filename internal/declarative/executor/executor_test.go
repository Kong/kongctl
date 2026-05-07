package executor

import (
	"context"
	"fmt"
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockProgressReporter for testing
type MockProgressReporter struct {
	mock.Mock
	StartExecutionCalled  int
	FinishExecutionCalled int
	StartChangeCalls      []planner.PlannedChange
	CompleteChangeCalls   []planner.PlannedChange
	SkipChangeCalls       []planner.PlannedChange
	SkipReasons           []string
}

func (m *MockProgressReporter) StartExecution(plan *planner.Plan) {
	m.StartExecutionCalled++
	m.Called(plan)
}

func (m *MockProgressReporter) StartChange(change planner.PlannedChange) {
	m.StartChangeCalls = append(m.StartChangeCalls, change)
	m.Called(change)
}

func (m *MockProgressReporter) CompleteChange(change planner.PlannedChange, err error) {
	m.CompleteChangeCalls = append(m.CompleteChangeCalls, change)
	m.Called(change, err)
}

func (m *MockProgressReporter) SkipChange(change planner.PlannedChange, reason string) {
	m.SkipChangeCalls = append(m.SkipChangeCalls, change)
	m.SkipReasons = append(m.SkipReasons, reason)
	m.Called(change, reason)
}

func (m *MockProgressReporter) FinishExecution(result *ExecutionResult) {
	m.FinishExecutionCalled++
	m.Called(result)
}

func TestExecutor_New(t *testing.T) {
	reporter := &MockProgressReporter{}

	exec := New(nil, reporter, false)

	assert.NotNil(t, exec)
	assert.Nil(t, exec.client)
	assert.Equal(t, reporter, exec.reporter)
	assert.False(t, exec.dryRun)

	// Test with dry-run
	execDryRun := New(nil, reporter, true)
	assert.True(t, execDryRun.dryRun)
}

func TestExecutor_Execute_EmptyPlan(t *testing.T) {
	reporter := &MockProgressReporter{}

	// Set up expectations
	reporter.On("StartExecution", mock.Anything).Return()
	reporter.On("FinishExecution", mock.Anything).Return()

	exec := New(nil, reporter, false)
	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)

	result := exec.Execute(context.Background(), plan)

	assert.NotNil(t, result)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 0, result.FailureCount)
	assert.Equal(t, 0, result.SkippedCount)
	assert.Empty(t, result.Errors)
	assert.False(t, result.DryRun)

	// Verify reporter was called
	assert.Equal(t, 1, reporter.StartExecutionCalled)
	assert.Equal(t, 1, reporter.FinishExecutionCalled)
}

func TestExecutor_Execute_DryRun(t *testing.T) {
	reporter := &MockProgressReporter{}

	// Set up expectations
	reporter.On("StartExecution", mock.Anything).Return()
	reporter.On("StartChange", mock.Anything).Return()
	reporter.On("SkipChange", mock.Anything, "dry-run mode").Return()
	reporter.On("FinishExecution", mock.Anything).Return()

	exec := New(nil, reporter, true) // dry-run enabled

	// Create a plan with a CREATE change
	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)
	change := planner.PlannedChange{
		ID:           "1-c-portal",
		ResourceType: "portal",
		ResourceRef:  "dev-portal",
		Action:       planner.ActionCreate,
		Fields: map[string]any{
			"name":        "Developer Portal",
			"description": "Main developer portal",
		},
	}
	plan.AddChange(change)
	plan.SetExecutionOrder([]string{"1-c-portal"})

	result := exec.Execute(context.Background(), plan)

	assert.NotNil(t, result)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 0, result.FailureCount)
	assert.Equal(t, 1, result.SkippedCount)
	assert.Empty(t, result.Errors)
	assert.True(t, result.DryRun)

	// Check validation results
	require.Len(t, result.ValidationResults, 1)
	assert.Equal(t, "1-c-portal", result.ValidationResults[0].ChangeID)
	assert.Equal(t, "would_succeed", result.ValidationResults[0].Status)
	assert.Equal(t, "passed", result.ValidationResults[0].Validation)

	// Verify reporter was called correctly
	assert.Equal(t, 1, len(reporter.SkipChangeCalls))
	assert.Equal(t, "dry-run mode", reporter.SkipReasons[0])
}

func TestExecutor_Execute_WithErrors(t *testing.T) {
	reporter := &MockProgressReporter{}

	// Set up expectations
	reporter.On("StartExecution", mock.Anything).Return()
	reporter.On("StartChange", mock.Anything).Return()
	reporter.On("CompleteChange", mock.Anything, mock.Anything).Return()
	reporter.On("FinishExecution", mock.Anything).Return()

	exec := New(nil, reporter, false)

	// Create a plan with a CREATE change for an unimplemented resource type
	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)
	change := planner.PlannedChange{
		ID:           "1-c-service",
		ResourceType: "service", // Not yet implemented
		ResourceRef:  "test-service",
		Action:       planner.ActionCreate,
		Fields: map[string]any{
			"name": "Test Service",
		},
	}
	plan.AddChange(change)
	plan.SetExecutionOrder([]string{"1-c-service"})

	result := exec.Execute(context.Background(), plan)

	assert.NotNil(t, result)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 1, result.FailureCount)
	assert.Equal(t, 0, result.SkippedCount)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error, "not yet implemented")

	// Verify error details
	assert.Equal(t, "1-c-service", result.Errors[0].ChangeID)
	assert.Equal(t, "service", result.Errors[0].ResourceType)
	assert.Equal(t, "Test Service", result.Errors[0].ResourceName)
}

func TestExecutor_Execute_NilReporter(t *testing.T) {
	// Execute with nil reporter should not panic
	exec := New(nil, nil, false)
	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)

	result := exec.Execute(context.Background(), plan)

	assert.NotNil(t, result)
}

func TestExecutor_resolveAuthStrategyRef_WithPlaceholder(t *testing.T) {
	exec := New(nil, nil, false)
	exec.refToID["application_auth_strategy"] = map[string]string{
		"key-auth": "abc-123",
	}

	id, err := exec.resolveAuthStrategyRef(context.Background(), planner.ReferenceInfo{Ref: "__REF__:key-auth#id"})
	require.NoError(t, err)
	assert.Equal(t, "abc-123", id)
}

func TestExecutor_syncResolvedDCRProviderID_UpdatesFieldsFromResolvedReference(t *testing.T) {
	exec := New(nil, nil, false)
	exec.refToID["dcr_provider"] = map[string]string{
		"okta-dcr": "6f211020-9ffb-4f64-b351-9ca7282fe451",
	}

	change := &planner.PlannedChange{
		Fields: map[string]any{
			planner.FieldDCRProviderID: "__REF__:okta-dcr#id",
		},
		References: map[string]planner.ReferenceInfo{
			planner.FieldDCRProviderID: {
				Ref: "__REF__:okta-dcr#id",
				ID:  "[unknown]",
			},
		},
	}

	err := exec.syncResolvedDCRProviderID(context.Background(), change)
	require.NoError(t, err)

	assert.Equal(
		t,
		"6f211020-9ffb-4f64-b351-9ca7282fe451",
		change.Fields[planner.FieldDCRProviderID],
	)
	assert.Equal(
		t,
		"6f211020-9ffb-4f64-b351-9ca7282fe451",
		change.References[planner.FieldDCRProviderID].ID,
	)
}

func TestExecutor_syncResolvedPortalDefaultAuthStrategyID_UpdatesFieldsFromResolvedReference(t *testing.T) {
	exec := New(nil, nil, false)
	exec.refToID[planner.ResourceTypeApplicationAuthStrategy] = map[string]string{
		"portal-default-strategy": "11111111-1111-4111-8111-111111111111",
	}

	change := &planner.PlannedChange{
		Fields: map[string]any{
			planner.FieldDefaultApplicationStrategyID: "__REF__:portal-default-strategy#id",
		},
		References: map[string]planner.ReferenceInfo{
			planner.FieldDefaultApplicationStrategyID: {
				Ref: "__REF__:portal-default-strategy#id",
				ID:  "[unknown]",
			},
		},
	}

	err := exec.syncResolvedPortalDefaultAuthStrategyID(context.Background(), change)
	require.NoError(t, err)

	assert.Equal(
		t,
		"11111111-1111-4111-8111-111111111111",
		change.Fields[planner.FieldDefaultApplicationStrategyID],
	)
	assert.Equal(
		t,
		"11111111-1111-4111-8111-111111111111",
		change.References[planner.FieldDefaultApplicationStrategyID].ID,
	)
}

func TestExecutor_ValidateChangePreExecution_Basic(t *testing.T) {
	tests := []struct {
		name          string
		change        planner.PlannedChange
		expectError   bool
		errorContains string
	}{
		{
			name: "create action - no validation",
			change: planner.PlannedChange{
				Action:       planner.ActionCreate,
				ResourceType: "portal",
			},
			expectError: false,
		},
		{
			name: "update without resource ID",
			change: planner.PlannedChange{
				Action:       planner.ActionUpdate,
				ResourceType: "portal",
			},
			expectError:   true,
			errorContains: "resource ID required",
		},
		{
			name: "delete without resource ID",
			change: planner.PlannedChange{
				Action:       planner.ActionDelete,
				ResourceType: "portal",
			},
			expectError:   true,
			errorContains: "resource ID required",
		},
		{
			name: "update non-portal resource - TODO",
			change: planner.PlannedChange{
				Action:       planner.ActionUpdate,
				ResourceType: "auth_strategy",
				ResourceID:   "auth-123",
			},
			expectError: false, // No validation for non-portal resources yet
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := New(nil, nil, false)

			err := exec.validateChangePreExecution(context.Background(), tt.change)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestExecutionResult_Methods(t *testing.T) {
	// Test Message() method
	t.Run("dry-run messages", func(t *testing.T) {
		result := &ExecutionResult{DryRun: true}
		assert.Equal(t, "Dry-run complete. No changes were made.", result.Message())

		result.FailureCount = 1
		assert.Equal(t, "Dry-run complete with errors. No changes were made.", result.Message())
	})

	t.Run("real execution messages", func(t *testing.T) {
		result := &ExecutionResult{DryRun: false}
		assert.Equal(t, "Execution completed successfully.", result.Message())

		result.FailureCount = 1
		assert.Equal(t, "Execution completed with errors.", result.Message())
	})

	// Test HasErrors() method
	t.Run("has errors", func(t *testing.T) {
		result := &ExecutionResult{}
		assert.False(t, result.HasErrors())

		result.FailureCount = 1
		assert.True(t, result.HasErrors())

		result.FailureCount = 0
		result.Errors = []ExecutionError{{Error: "test"}}
		assert.True(t, result.HasErrors())
	})

	// Test TotalChanges() method
	t.Run("total changes", func(t *testing.T) {
		result := &ExecutionResult{
			SuccessCount: 2,
			FailureCount: 1,
			SkippedCount: 3,
		}
		assert.Equal(t, 6, result.TotalChanges())
	})
}

func TestExecutor_ExecutionOrder(t *testing.T) {
	reporter := &MockProgressReporter{}

	// Track the order of StartChange calls
	var executionOrder []string
	reporter.On("StartExecution", mock.Anything).Return()
	reporter.On("StartChange", mock.Anything).Run(func(args mock.Arguments) {
		change := args.Get(0).(planner.PlannedChange)
		executionOrder = append(executionOrder, change.ID)
	}).Return()
	reporter.On("SkipChange", mock.Anything, mock.Anything).Return()
	reporter.On("FinishExecution", mock.Anything).Return()

	exec := New(nil, reporter, true) // dry-run

	// Create a plan with multiple changes
	plan := planner.NewPlan("1.0", "test", planner.PlanModeSync)

	// Add changes in one order
	changes := []planner.PlannedChange{
		{
			ID:           "3-d-portal",
			Action:       planner.ActionDelete,
			ResourceType: "portal",
			ResourceID:   "portal-3", // Required for DELETE
			Fields:       map[string]any{"name": "Portal 3-d-portal"},
		},
		{
			ID:           "1-c-portal",
			Action:       planner.ActionCreate,
			ResourceType: "portal",
			Fields:       map[string]any{"name": "Portal 1-c-portal"},
		},
		{
			ID:           "2-u-portal",
			Action:       planner.ActionUpdate,
			ResourceType: "portal",
			ResourceID:   "portal-2", // Required for UPDATE
			Fields:       map[string]any{"name": "Portal 2-u-portal"},
		},
	}

	for _, change := range changes {
		plan.AddChange(change)
	}

	// Set specific execution order
	plan.SetExecutionOrder([]string{"1-c-portal", "2-u-portal", "3-d-portal"})

	_ = exec.Execute(context.Background(), plan)

	// Verify execution followed the specified order
	assert.Equal(t, []string{"1-c-portal", "2-u-portal", "3-d-portal"}, executionOrder)
}

func TestExecutor_ContinuesOnError(t *testing.T) {
	reporter := &MockProgressReporter{}

	// Set up expectations
	reporter.On("StartExecution", mock.Anything).Return()
	reporter.On("StartChange", mock.Anything).Return()
	reporter.On("CompleteChange", mock.Anything, mock.Anything).Return()
	reporter.On("FinishExecution", mock.Anything).Return()

	exec := New(nil, reporter, false)

	// Create a plan with multiple changes (all will fail due to not implemented)
	plan := planner.NewPlan("1.0", "test", planner.PlanModeSync)

	for i := 1; i <= 3; i++ {
		change := planner.PlannedChange{
			ID:           fmt.Sprintf("%d-c-route", i),
			ResourceType: "route", // Not yet implemented
			ResourceRef:  fmt.Sprintf("route-%d", i),
			Action:       planner.ActionCreate,
			Fields: map[string]any{
				"name": fmt.Sprintf("Route %d", i),
			},
		}
		plan.AddChange(change)
	}
	plan.SetExecutionOrder([]string{"1-c-route", "2-c-route", "3-c-route"})

	result := exec.Execute(context.Background(), plan)

	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 3, result.FailureCount) // All failed
	assert.Equal(t, 0, result.SkippedCount)
	assert.Len(t, result.Errors, 3) // All errors collected

	// Verify all changes were attempted
	assert.Len(t, reporter.CompleteChangeCalls, 3)
}

// // --- effectiveDependsOn tests ---

// func TestEffectiveDependsOn_ExplicitOnly(t *testing.T) {
// 	change := &planner.PlannedChange{
// 		ID:         "2:c:portal:my-portal",
// 		DependsOn:  []string{"1:c:api:my-api"},
// 		References: map[string]planner.ReferenceInfo{},
// 	}
// 	refToChangeID := map[string]string{
// 		"my-api": "1:c:api:my-api",
// 	}

// 	deps := effectiveDependsOn(change, refToChangeID)
// 	assert.Equal(t, []string{"1:c:api:my-api"}, deps)
// }

// func TestEffectiveDependsOn_RefPlaceholderAddsImplicitDep(t *testing.T) {
// 	// Virtual cluster references a backend cluster via __REF__ placeholder.
// 	change := &planner.PlannedChange{
// 		ID:        "3:c:event_gateway_virtual_cluster:default-virtual-cluster",
// 		DependsOn: []string{"1:c:event_gateway:egw"},
// 		References: map[string]planner.ReferenceInfo{
// 			planner.FieldEventGatewayBackendClusterID: {
// 				Ref: "__REF__:default-backend-cluster#id",
// 			},
// 		},
// 	}
// 	refToChangeID := map[string]string{
// 		"egw":                     "1:c:event_gateway:egw",
// 		"default-backend-cluster": "2:c:event_gateway_backend_cluster:default-backend-cluster",
// 	}

// 	deps := effectiveDependsOn(change, refToChangeID)

// 	assert.Contains(t, deps, "1:c:event_gateway:egw")
// 	assert.Contains(t, deps, "2:c:event_gateway_backend_cluster:default-backend-cluster")
// 	assert.Len(t, deps, 2)
// }

// func TestEffectiveDependsOn_ArrayRefsAddImplicitDeps(t *testing.T) {
// 	change := &planner.PlannedChange{
// 		ID:        "5:c:cp:group-cp",
// 		DependsOn: []string{},
// 		References: map[string]planner.ReferenceInfo{
// 			planner.FieldMembers: {
// 				Refs: []string{
// 					"__REF__:member-cp-1#id",
// 					"__REF__:member-cp-2#id",
// 				},
// 			},
// 		},
// 	}
// 	refToChangeID := map[string]string{
// 		"member-cp-1": "3:c:control_plane:member-cp-1",
// 		"member-cp-2": "4:c:control_plane:member-cp-2",
// 	}

// 	deps := effectiveDependsOn(change, refToChangeID)

// 	assert.Contains(t, deps, "3:c:control_plane:member-cp-1")
// 	assert.Contains(t, deps, "4:c:control_plane:member-cp-2")
// 	assert.Len(t, deps, 2)
// }

// func TestEffectiveDependsOn_UnresolvedParentAddsImplicitDep(t *testing.T) {
// 	change := &planner.PlannedChange{
// 		ID:         "2:c:child:my-child",
// 		DependsOn:  []string{},
// 		References: map[string]planner.ReferenceInfo{},
// 		Parent: &planner.ParentInfo{
// 			Ref: "my-gateway",
// 			ID:  "[unknown]",
// 		},
// 	}
// 	refToChangeID := map[string]string{
// 		"my-gateway": "1:c:event_gateway:my-gateway",
// 	}

// 	deps := effectiveDependsOn(change, refToChangeID)

// 	assert.Equal(t, []string{"1:c:event_gateway:my-gateway"}, deps)
// }

// func TestEffectiveDependsOn_UnresolvedParentEmptyIDAddsImplicitDep(t *testing.T) {
// 	change := &planner.PlannedChange{
// 		ID:         "2:c:child:my-child",
// 		DependsOn:  []string{},
// 		References: map[string]planner.ReferenceInfo{},
// 		Parent: &planner.ParentInfo{
// 			Ref: "my-gateway",
// 			ID:  "", // empty also counts as unresolved
// 		},
// 	}
// 	refToChangeID := map[string]string{
// 		"my-gateway": "1:c:event_gateway:my-gateway",
// 	}

// 	deps := effectiveDependsOn(change, refToChangeID)

// 	assert.Equal(t, []string{"1:c:event_gateway:my-gateway"}, deps)
// }

// func TestEffectiveDependsOn_ResolvedParentNotAddedAsDep(t *testing.T) {
// 	change := &planner.PlannedChange{
// 		ID:         "2:c:child:my-child",
// 		DependsOn:  []string{},
// 		References: map[string]planner.ReferenceInfo{},
// 		Parent: &planner.ParentInfo{
// 			Ref: "existing-gateway",
// 			ID:  "550e8400-e29b-41d4-a716-446655440000", // already resolved UUID
// 		},
// 	}
// 	refToChangeID := map[string]string{
// 		"existing-gateway": "1:c:event_gateway:existing-gateway",
// 	}

// 	deps := effectiveDependsOn(change, refToChangeID)

// 	assert.Empty(t, deps)
// }

// func TestEffectiveDependsOn_NoDuplicates(t *testing.T) {
// 	// Explicit DependsOn and a ref placeholder both point to the same change.
// 	change := &planner.PlannedChange{
// 		ID:        "3:c:virtual_cluster:vc",
// 		DependsOn: []string{"2:c:backend_cluster:bc"},
// 		References: map[string]planner.ReferenceInfo{
// 			planner.FieldEventGatewayBackendClusterID: {
// 				Ref: "__REF__:bc#id",
// 			},
// 		},
// 	}
// 	refToChangeID := map[string]string{
// 		"bc": "2:c:backend_cluster:bc",
// 	}

// 	deps := effectiveDependsOn(change, refToChangeID)

// 	assert.Equal(t, []string{"2:c:backend_cluster:bc"}, deps, "should not contain duplicate")
// }

// func TestEffectiveDependsOn_RefNotInPlanIsIgnored(t *testing.T) {
// 	// A placeholder that references something outside the plan — no implicit dep should be added.
// 	change := &planner.PlannedChange{
// 		ID:        "2:c:child:x",
// 		DependsOn: []string{},
// 		References: map[string]planner.ReferenceInfo{
// 			"some_ref": {Ref: "__REF__:external-resource#id"},
// 		},
// 	}
// 	refToChangeID := map[string]string{} // nothing in the plan

// 	deps := effectiveDependsOn(change, refToChangeID)

// 	assert.Empty(t, deps)
// }

// // --- parallel execution ordering tests ---

// // parallelOrderTracker records the order in which changes start, protected by a mutex.
// type parallelOrderTracker struct {
// 	mu    sync.Mutex
// 	order []string
// }

// func (t *parallelOrderTracker) record(id string) {
// 	t.mu.Lock()
// 	defer t.mu.Unlock()
// 	t.order = append(t.order, id)
// }

// func (t *parallelOrderTracker) snapshot() []string {
// 	t.mu.Lock()
// 	defer t.mu.Unlock()
// 	cp := make([]string, len(t.order))
// 	copy(cp, t.order)
// 	return cp
// }

// // TestParallelExecution_RefPlaceholderEnforcesOrdering verifies that in parallel
// // mode a change whose References contain a __REF__ placeholder waits for the
// // change that creates the referenced resource, even when DependsOn is absent.
// func TestParallelExecution_RefPlaceholderEnforcesOrdering(t *testing.T) {
// 	tracker := &parallelOrderTracker{}

// 	reporter := &MockProgressReporter{}
// 	reporter.On("StartExecution", mock.Anything).Return()
// 	reporter.On("StartChange", mock.Anything).Run(func(args mock.Arguments) {
// 		change := args.Get(0).(planner.PlannedChange)
// 		tracker.record(change.ID)
// 	}).Return()
// 	reporter.On("SkipChange", mock.Anything, mock.Anything).Return()
// 	reporter.On("FinishExecution", mock.Anything).Return()

// 	exec := NewWithOptions(nil, reporter, true /* dry-run */, Options{MaxConcurrency: 5})

// 	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)

// 	gateway := planner.PlannedChange{
// 		ID:           "1:c:event_gateway:egw",
// 		ResourceType: planner.ResourceTypeEventGatewayControlPlane,
// 		ResourceRef:  "egw",
// 		Action:       planner.ActionCreate,
// 		Fields:       map[string]any{"name": "egw"},
// 	}
// 	backendCluster := planner.PlannedChange{
// 		ID:           "2:c:event_gateway_backend_cluster:bc",
// 		ResourceType: planner.ResourceTypeEventGatewayBackendCluster,
// 		ResourceRef:  "bc",
// 		Action:       planner.ActionCreate,
// 		Fields:       map[string]any{"name": "bc"},
// 		DependsOn:    []string{"1:c:event_gateway:egw"},
// 		References: map[string]planner.ReferenceInfo{
// 			planner.FieldEventGatewayID: {Ref: "egw"},
// 		},
// 	}
// 	// virtualCluster references bc via a __REF__ placeholder but has NO explicit
// 	// DependsOn on the backend cluster change — the executor must infer it.
// 	virtualCluster := planner.PlannedChange{
// 		ID:           "3:c:event_gateway_virtual_cluster:vc",
// 		ResourceType: planner.ResourceTypeEventGatewayVirtualCluster,
// 		ResourceRef:  "vc",
// 		Action:       planner.ActionCreate,
// 		Fields:       map[string]any{"name": "vc"},
// 		DependsOn:    []string{"1:c:event_gateway:egw"},
// 		References: map[string]planner.ReferenceInfo{
// 			planner.FieldEventGatewayID: {Ref: "egw"},
// 			planner.FieldEventGatewayBackendClusterID: {
// 				Ref: "__REF__:bc#id",
// 			},
// 		},
// 	}

// 	plan.AddChange(gateway)
// 	plan.AddChange(backendCluster)
// 	plan.AddChange(virtualCluster)
// 	plan.SetExecutionOrder([]string{
// 		"1:c:event_gateway:egw",
// 		"2:c:event_gateway_backend_cluster:bc",
// 		"3:c:event_gateway_virtual_cluster:vc",
// 	})

// 	_ = exec.Execute(context.Background(), plan)

// 	order := tracker.snapshot()
// 	assert.Len(t, order, 3)

// 	bcIdx := slices.Index(order, "2:c:event_gateway_backend_cluster:bc")
// 	vcIdx := slices.Index(order, "3:c:event_gateway_virtual_cluster:vc")
// 	assert.GreaterOrEqual(t, vcIdx, 0, "virtual cluster should have been started")
// 	assert.GreaterOrEqual(t, bcIdx, 0, "backend cluster should have been started")
// 	// The virtual cluster must not start before the backend cluster.
// 	assert.Less(t, bcIdx, vcIdx, "backend cluster must start before virtual cluster")
// }

// // TestParallelExecution_UnresolvedParentEnforcesOrdering verifies that when a
// // change has an unresolved Parent (ID == "[unknown]"), the parallel executor
// // waits for the change that creates the parent before proceeding.
// func TestParallelExecution_UnresolvedParentEnforcesOrdering(t *testing.T) {
// 	tracker := &parallelOrderTracker{}

// 	reporter := &MockProgressReporter{}
// 	reporter.On("StartExecution", mock.Anything).Return()
// 	reporter.On("StartChange", mock.Anything).Run(func(args mock.Arguments) {
// 		change := args.Get(0).(planner.PlannedChange)
// 		tracker.record(change.ID)
// 	}).Return()
// 	reporter.On("SkipChange", mock.Anything, mock.Anything).Return()
// 	reporter.On("FinishExecution", mock.Anything).Return()

// 	exec := NewWithOptions(nil, reporter, true /* dry-run */, Options{MaxConcurrency: 5})

// 	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)

// 	parent := planner.PlannedChange{
// 		ID:           "1:c:portal:my-portal",
// 		ResourceType: "portal",
// 		ResourceRef:  "my-portal",
// 		Action:       planner.ActionCreate,
// 		Fields:       map[string]any{"name": "my-portal"},
// 	}
// 	// child has no explicit DependsOn — the unresolved Parent must imply it.
// 	child := planner.PlannedChange{
// 		ID:           "2:c:portal_page:home",
// 		ResourceType: "portal_page",
// 		ResourceRef:  "home",
// 		Action:       planner.ActionCreate,
// 		Fields:       map[string]any{"name": "home"},
// 		Parent: &planner.ParentInfo{
// 			Ref: "my-portal",
// 			ID:  "[unknown]",
// 		},
// 	}

// 	plan.AddChange(parent)
// 	plan.AddChange(child)
// 	plan.SetExecutionOrder([]string{
// 		"1:c:portal:my-portal",
// 		"2:c:portal_page:home",
// 	})

// 	_ = exec.Execute(context.Background(), plan)

// 	order := tracker.snapshot()
// 	assert.Len(t, order, 2)
// 	assert.Equal(t, "1:c:portal:my-portal", order[0], "parent must execute before child")
// 	assert.Equal(t, "2:c:portal_page:home", order[1])
// }
