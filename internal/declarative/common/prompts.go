package common

import (
	"bufio"
	"fmt"
	"io"
	"strings"

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
	
	scanner := bufio.NewScanner(stdin)
	if scanner.Scan() {
		response := strings.TrimSpace(scanner.Text())
		return response == "yes"
	}
	
	return false
}

// DisplayPlanSummary shows a concise summary of the plan.
func DisplayPlanSummary(plan *planner.Plan, out io.Writer) {
	fmt.Fprintln(out, "Plan Summary:")

	if plan.Summary.ByAction == nil {
		fmt.Fprintln(out, "- No changes")
		return
	}

	createCount := plan.Summary.ByAction[planner.ActionCreate]
	updateCount := plan.Summary.ByAction[planner.ActionUpdate]
	deleteCount := plan.Summary.ByAction[planner.ActionDelete]

	if createCount > 0 {
		fmt.Fprintf(out, "- Create: %d resources\n", createCount)
	}
	if updateCount > 0 {
		fmt.Fprintf(out, "- Update: %d resources\n", updateCount)
	}
	if deleteCount > 0 {
		fmt.Fprintf(out, "- Delete: %d resources\n", deleteCount)
	}

	// Show warnings if any
	if len(plan.Warnings) > 0 {
		fmt.Fprintf(out, "\nWarnings: %d\n", len(plan.Warnings))
		for _, warning := range plan.Warnings {
			fmt.Fprintf(out, "  ⚠ %s\n", warning.Message)
		}
	}
}