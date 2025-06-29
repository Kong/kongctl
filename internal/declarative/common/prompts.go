package common

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/kong/kongctl/internal/declarative/planner"
)

// ConfirmExecution displays a plan summary and prompts for confirmation.
// Returns true if the user confirms with 'yes', false otherwise.
func ConfirmExecution(plan *planner.Plan, stdout, stderr io.Writer, stdin io.Reader) bool {
	DisplayPlanSummary(plan, stdout)

	// Show DELETE warning if applicable
	deleteCount := 0
	if plan.Summary.ByAction != nil {
		deleteCount = plan.Summary.ByAction[planner.ActionDelete]
	}

	if deleteCount > 0 {
		fmt.Fprintln(stderr, "\nWARNING: This operation will DELETE resources:")
		for _, change := range plan.Changes {
			if change.Action == planner.ActionDelete {
				resourceName := change.ResourceRef
				if resourceName == "" && len(change.Fields) > 0 {
					// Try to get name from fields
					if name, ok := change.Fields["name"].(string); ok {
						resourceName = name
					}
				}
				fmt.Fprintf(stderr, "- %s: %s\n", change.ResourceType, resourceName)
			}
		}
	}

	fmt.Fprint(stderr, "\nDo you want to continue? Type 'yes' to confirm: ")
	
	// Set up interrupt handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)
	
	// Channel for scanner result
	responseChan := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdin)
		if scanner.Scan() {
			responseChan <- scanner.Text()
		} else {
			responseChan <- ""
		}
	}()
	
	// Wait for either response or interrupt
	select {
	case <-sigChan:
		// Interrupted - print newline for clean output
		fmt.Fprintln(stderr)
		return false
	case response := <-responseChan:
		return strings.TrimSpace(response) == "yes"
	case <-ctx.Done():
		return false
	}
}

// DisplayPlanSummary shows a concise summary of the plan.
func DisplayPlanSummary(plan *planner.Plan, out io.Writer) {
	fmt.Fprintln(out, "Plan Summary:")

	if plan.Summary.ByAction == nil || len(plan.Changes) == 0 {
		fmt.Fprintln(out, "- No changes")
		return
	}

	// Group changes by action and resource type
	changesByAction := make(map[planner.ActionType]map[string][]planner.PlannedChange)
	for _, change := range plan.Changes {
		if changesByAction[change.Action] == nil {
			changesByAction[change.Action] = make(map[string][]planner.PlannedChange)
		}
		changesByAction[change.Action][change.ResourceType] = append(
			changesByAction[change.Action][change.ResourceType], change)
	}

	// Display changes organized by action
	actionOrder := []planner.ActionType{planner.ActionCreate, planner.ActionUpdate, planner.ActionDelete}
	for _, action := range actionOrder {
		if resources, ok := changesByAction[action]; ok && len(resources) > 0 {
			fmt.Fprintf(out, "\n%s:\n", getActionHeader(action))
			for resourceType, changes := range resources {
				fmt.Fprintf(out, "  %s (%d):\n", resourceType, len(changes))
				for _, change := range changes {
					resourceName := change.ResourceRef
					if resourceName == "" {
						// Try to get name from fields
						if name, ok := change.Fields["name"].(string); ok {
							resourceName = name
						}
					}
					fmt.Fprintf(out, "    - %s\n", resourceName)
				}
			}
		}
	}

	// Show warnings if any
	if len(plan.Warnings) > 0 {
		fmt.Fprintf(out, "\nWarnings (%d):\n", len(plan.Warnings))
		for _, warning := range plan.Warnings {
			fmt.Fprintf(out, "  ⚠ %s\n", warning.Message)
		}
	}
}

// getActionHeader returns a user-friendly header for the action type
func getActionHeader(action planner.ActionType) string {
	switch action {
	case planner.ActionCreate:
		return "Resources to create"
	case planner.ActionUpdate:
		return "Resources to update"
	case planner.ActionDelete:
		return "Resources to delete"
	default:
		return string(action)
	}
}