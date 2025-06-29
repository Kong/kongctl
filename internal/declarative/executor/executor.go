package executor

import (
	"context"
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// Executor handles the execution of declarative configuration plans
type Executor struct {
	client   *state.Client
	reporter ProgressReporter
	dryRun   bool
}

// New creates a new Executor instance
func New(client *state.Client, reporter ProgressReporter, dryRun bool) *Executor {
	return &Executor{
		client:   client,
		reporter: reporter,
		dryRun:   dryRun,
	}
}

// Execute runs the plan and returns the execution result
func (e *Executor) Execute(ctx context.Context, plan *planner.Plan) (*ExecutionResult, error) {
	result := &ExecutionResult{
		DryRun: e.dryRun,
	}
	
	// Notify reporter of execution start
	if e.reporter != nil {
		e.reporter.StartExecution(plan)
	}
	
	// Execute changes in order
	for _, changeID := range plan.ExecutionOrder {
		// Find the change by ID
		var change *planner.PlannedChange
		for i := range plan.Changes {
			if plan.Changes[i].ID == changeID {
				change = &plan.Changes[i]
				break
			}
		}
		
		if change == nil {
			// This shouldn't happen, but handle gracefully
			err := fmt.Errorf("change with ID %s not found in plan", changeID)
			result.Errors = append(result.Errors, ExecutionError{
				ChangeID: changeID,
				Error:    err.Error(),
			})
			result.FailureCount++
			continue
		}
		
		// Execute the change
		if err := e.executeChange(ctx, result, *change); err != nil {
			// Error already recorded in executeChange
			continue
		}
	}
	
	// Notify reporter of execution completion
	if e.reporter != nil {
		e.reporter.FinishExecution(result)
	}
	
	return result, nil
}

// executeChange executes a single change from the plan
func (e *Executor) executeChange(ctx context.Context, result *ExecutionResult, change planner.PlannedChange) error {
	// Notify reporter of change start
	if e.reporter != nil {
		e.reporter.StartChange(change)
	}
	
	// Extract resource name from fields
	resourceName := getResourceName(change.Fields)
	
	// Pre-execution validation (always performed, even in dry-run)
	if err := e.validateChangePreExecution(ctx, change); err != nil {
		// Record error
		execError := ExecutionError{
			ChangeID:     change.ID,
			ResourceType: change.ResourceType,
			ResourceName: resourceName,
			ResourceRef:  change.ResourceRef,
			Action:       string(change.Action),
			Error:        err.Error(),
		}
		result.Errors = append(result.Errors, execError)
		result.FailureCount++
		
		// In dry-run, also record validation result
		if e.dryRun {
			result.ValidationResults = append(result.ValidationResults, ValidationResult{
				ChangeID:     change.ID,
				ResourceType: change.ResourceType,
				ResourceName: resourceName,
				ResourceRef:  change.ResourceRef,
				Action:       string(change.Action),
				Status:       "would_fail",
				Validation:   "failed",
				Message:      err.Error(),
			})
		}
		
		// Notify reporter
		if e.reporter != nil {
			e.reporter.CompleteChange(change, err)
		}
		
		return err
	}
	
	// If dry-run, skip actual execution
	if e.dryRun {
		result.SkippedCount++
		result.ValidationResults = append(result.ValidationResults, ValidationResult{
			ChangeID:     change.ID,
			ResourceType: change.ResourceType,
			ResourceName: resourceName,
			ResourceRef:  change.ResourceRef,
			Action:       string(change.Action),
			Status:       "would_succeed",
			Validation:   "passed",
		})
		
		if e.reporter != nil {
			e.reporter.SkipChange(change, "dry-run mode")
		}
		
		return nil
	}
	
	// Execute the actual change
	var err error
	var resourceID string
	
	switch change.Action {
	case planner.ActionCreate:
		resourceID, err = e.createResource(ctx, change)
	case planner.ActionUpdate:
		resourceID, err = e.updateResource(ctx, change)
	case planner.ActionDelete:
		err = e.deleteResource(ctx, change)
		resourceID = change.ResourceID
	default:
		err = fmt.Errorf("unknown action: %s", change.Action)
	}
	
	// Record result
	if err != nil {
		execError := ExecutionError{
			ChangeID:     change.ID,
			ResourceType: change.ResourceType,
			ResourceName: resourceName,
			ResourceRef:  change.ResourceRef,
			Action:       string(change.Action),
			Error:        err.Error(),
		}
		result.Errors = append(result.Errors, execError)
		result.FailureCount++
	} else {
		result.SuccessCount++
		result.ChangesApplied = append(result.ChangesApplied, AppliedChange{
			ChangeID:     change.ID,
			ResourceType: change.ResourceType,
			ResourceName: resourceName,
			ResourceRef:  change.ResourceRef,
			Action:       string(change.Action),
			ResourceID:   resourceID,
		})
	}
	
	// Notify reporter
	if e.reporter != nil {
		e.reporter.CompleteChange(change, err)
	}
	
	return err
}

// validateChangePreExecution performs validation before executing a change
func (e *Executor) validateChangePreExecution(ctx context.Context, change planner.PlannedChange) error {
	switch change.Action {
	case planner.ActionUpdate, planner.ActionDelete:
		// For update/delete, verify resource still exists and check protection
		if change.ResourceID == "" {
			return fmt.Errorf("resource ID required for %s operation", change.Action)
		}
		
		// TODO: In Step 4, implement resource-specific validation
		// For now, perform basic validation for portal resources
		if change.ResourceType == "portal" && e.client != nil {
			portal, err := e.client.GetPortalByName(ctx, getResourceName(change.Fields))
			if err != nil {
				return fmt.Errorf("failed to fetch portal: %w", err)
			}
			if portal == nil {
				return fmt.Errorf("portal no longer exists")
			}
			
			// Check protection status
			isProtected := portal.NormalizedLabels[labels.ProtectedKey] == "true"
			if isProtected && (change.Action == planner.ActionUpdate || change.Action == planner.ActionDelete) {
				return fmt.Errorf("resource is protected and cannot be %s", 
					actionToVerb(change.Action))
			}
		}
		
	case planner.ActionCreate:
		// For create, verify resource doesn't already exist
		if change.ResourceType == "portal" && e.client != nil {
			resourceName := getResourceName(change.Fields)
			portal, err := e.client.GetPortalByName(ctx, resourceName)
			if err != nil {
				// API error is acceptable here - might mean not found
				// Only fail if it's a real API error (not 404)
				if !strings.Contains(err.Error(), "not found") {
					return fmt.Errorf("failed to check existing portal: %w", err)
				}
			}
			if portal != nil {
				// Portal already exists - check if configuration matches
				existingHash := portal.NormalizedLabels[labels.ConfigHashKey]
				if change.ConfigHash != "" && existingHash == change.ConfigHash {
					// Configuration matches - this is idempotent
					return fmt.Errorf("portal '%s' already exists with matching configuration (idempotent)", resourceName)
				}
				// Configuration differs
				return fmt.Errorf("portal '%s' already exists with different configuration", resourceName)
			}
		}
	}
	
	return nil
}

// Resource operations

func (e *Executor) createResource(ctx context.Context, change planner.PlannedChange) (string, error) {
	switch change.ResourceType {
	case "portal":
		return e.createPortal(ctx, change)
	default:
		return "", fmt.Errorf("create operation not yet implemented for %s", change.ResourceType)
	}
}

func (e *Executor) updateResource(ctx context.Context, change planner.PlannedChange) (string, error) {
	switch change.ResourceType {
	case "portal":
		return e.updatePortal(ctx, change)
	default:
		return "", fmt.Errorf("update operation not yet implemented for %s", change.ResourceType)
	}
}

func (e *Executor) deleteResource(ctx context.Context, change planner.PlannedChange) error {
	switch change.ResourceType {
	case "portal":
		return e.deletePortal(ctx, change)
	default:
		return fmt.Errorf("delete operation not yet implemented for %s", change.ResourceType)
	}
}

// Helper functions

func getResourceName(fields map[string]interface{}) string {
	if name, ok := fields["name"].(string); ok {
		return name
	}
	return "<unknown>"
}

func actionToVerb(action planner.ActionType) string {
	switch action {
	case planner.ActionCreate:
		return "created"
	case planner.ActionUpdate:
		return "updated"
	case planner.ActionDelete:
		return "deleted"
	default:
		return string(action)
	}
}