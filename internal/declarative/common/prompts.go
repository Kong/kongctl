package common

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/kong/kongctl/internal/declarative/planner"
)

// ConfirmExecution prompts for confirmation.
// Returns true if the user confirms with 'yes', false otherwise.
func ConfirmExecution(plan *planner.Plan, _, stderr io.Writer, stdin io.Reader) bool {
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
	totalChanges := plan.Summary.TotalChanges
	if totalChanges > 0 {
		fmt.Fprintf(out, "Plan Summary (%d changes):\n", totalChanges)
	} else {
		fmt.Fprintln(out, "Plan Summary:")
	}

	if plan.Summary.ByAction == nil || len(plan.Changes) == 0 {
		fmt.Fprintln(out, "  No changes")
		return
	}

	// Group changes by resource type
	changesByResource := make(map[string][]planner.PlannedChange)
	for _, change := range plan.Changes {
		changesByResource[change.ResourceType] = append(
			changesByResource[change.ResourceType], change)
	}

	// Sort resource types by dependency order
	sortedTypes := sortResourceTypesByDependency(changesByResource, plan.Changes)

	// Display changes organized by resource type
	fmt.Fprintln(out, "")
	for _, resourceType := range sortedTypes {
		changes := changesByResource[resourceType]
		fmt.Fprintf(out, "%s (%d):\n", resourceType, len(changes))
		for _, change := range changes {
			resourceName := change.ResourceRef
			if resourceName == "" {
				// Try to get name from fields
				if name, ok := change.Fields["name"].(string); ok {
					resourceName = name
				}
			}
			actionPrefix := getActionPrefix(change.Action)
			
			// Build dependency info
			var depInfo string
			if len(change.DependsOn) > 0 || (change.Parent != nil && change.Parent.Ref != "") {
				// Use a map to track unique dependencies
				depMap := make(map[string]bool)
				
				// Add parent dependency if exists
				if change.Parent != nil && change.Parent.Ref != "" {
					// Find parent resource type
					parentType := getParentResourceType(change.ResourceType)
					if parentType != "" {
						depMap[fmt.Sprintf("%s:%s", parentType, change.Parent.Ref)] = true
					}
				}
				
				// Add explicit dependencies
				for _, depID := range change.DependsOn {
					// Find the dependent change to get its type and ref
					for _, depChange := range plan.Changes {
						if depChange.ID == depID {
							depMap[fmt.Sprintf("%s:%s", depChange.ResourceType, depChange.ResourceRef)] = true
							break
						}
					}
				}
				
				// Convert map to sorted slice for consistent output
				if len(depMap) > 0 {
					deps := make([]string, 0, len(depMap))
					for dep := range depMap {
						deps = append(deps, dep)
					}
					// Sort for consistent output
					sort.Strings(deps)
					depInfo = fmt.Sprintf(" (depends on %s)", strings.Join(deps, ", "))
				}
			}
			
			fmt.Fprintf(out, "  %s %s%s\n", actionPrefix, resourceName, depInfo)
		}
	}

	// Show warnings if any with change IDs
	if len(plan.Warnings) > 0 {
		fmt.Fprintf(out, "\nWarnings (%d):\n", len(plan.Warnings))
		for _, warning := range plan.Warnings {
			// Find the change to get more context
			var change *planner.PlannedChange
			for i := range plan.Changes {
				if plan.Changes[i].ID == warning.ChangeID {
					change = &plan.Changes[i]
					break
				}
			}
			
			if change != nil {
				// Extract position from change ID (format: "N:action:type:ref")
				parts := strings.SplitN(change.ID, ":", 4)
				if len(parts) >= 4 {
					fmt.Fprintf(out, "  ⚠ [%s] %s: %s\n", parts[0], change.ResourceType, change.ResourceRef)
					fmt.Fprintf(out, "    %s\n", warning.Message)
				} else {
					fmt.Fprintf(out, "  ⚠ %s\n", warning.Message)
				}
			} else {
				fmt.Fprintf(out, "  ⚠ %s\n", warning.Message)
			}
		}
	}
}

// getParentResourceType returns the parent resource type for a given child type
func getParentResourceType(childType string) string {
	switch childType {
	case "api_version", "api_publication", "api_implementation", "api_document":
		return "api"
	case "portal_page", "portal_snippet", "portal_customization", "portal_custom_domain":
		return "portal"
	default:
		return ""
	}
}

// sortResourceTypesByDependency sorts resource types so that dependencies appear first
func sortResourceTypesByDependency(
	changesByResource map[string][]planner.PlannedChange,
	allChanges []planner.PlannedChange,
) []string {
	// Build dependency graph between resource types
	typeDependencies := make(map[string]map[string]bool) // resourceType -> set of types it depends on
	resourceTypes := make([]string, 0, len(changesByResource))
	
	for resourceType := range changesByResource {
		resourceTypes = append(resourceTypes, resourceType)
		typeDependencies[resourceType] = make(map[string]bool)
	}
	
	// Analyze dependencies between resource types
	for _, change := range allChanges {
		// Check parent dependencies
		if change.Parent != nil && change.Parent.Ref != "" {
			parentType := getParentResourceType(change.ResourceType)
			if parentType != "" && parentType != change.ResourceType {
				typeDependencies[change.ResourceType][parentType] = true
			}
		}
		
		// Check explicit dependencies
		for _, depID := range change.DependsOn {
			// Find the dependent change
			for _, depChange := range allChanges {
				if depChange.ID == depID && depChange.ResourceType != change.ResourceType {
					typeDependencies[change.ResourceType][depChange.ResourceType] = true
					break
				}
			}
		}
	}
	
	// Perform topological sort (dependencies first)
	visited := make(map[string]bool)
	result := make([]string, 0, len(resourceTypes))
	
	var visit func(resourceType string)
	visit = func(resourceType string) {
		if visited[resourceType] {
			return
		}
		visited[resourceType] = true
		
		// Visit dependencies first
		for depType := range typeDependencies[resourceType] {
			if _, exists := changesByResource[depType]; exists {
				visit(depType)
			}
		}
		
		// Add this type after its dependencies
		result = append(result, resourceType)
	}
	
	// Visit all resource types
	for _, resourceType := range resourceTypes {
		visit(resourceType)
	}
	
	return result
}

// getActionPrefix returns a symbol prefix for the action type
func getActionPrefix(action planner.ActionType) string {
	switch action {
	case planner.ActionCreate:
		return "+"
	case planner.ActionUpdate:
		return "~"
	case planner.ActionDelete:
		return "-"
	default:
		return "?"
	}
}