package executor

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/kong/kongctl/internal/declarative/planner"
)

// ConsoleReporter provides console output for plan execution progress
type ConsoleReporter struct {
	writer       io.Writer
	dryRun       bool
	totalChanges int
	currentIndex int
	namespaceStats map[string]*namespaceStats
}

// namespaceStats tracks execution statistics per namespace
type namespaceStats struct {
	successCount int
	failureCount int
	skippedCount int
}

// NewConsoleReporter creates a new console reporter that writes to the provided writer
func NewConsoleReporter(w io.Writer) *ConsoleReporter {
	return &ConsoleReporter{
		writer:         w,
		dryRun:         false,
		totalChanges:   0,
		currentIndex:   0,
		namespaceStats: make(map[string]*namespaceStats),
	}
}

// NewConsoleReporterWithOptions creates a new console reporter with options
func NewConsoleReporterWithOptions(w io.Writer, dryRun bool) *ConsoleReporter {
	return &ConsoleReporter{
		writer:         w,
		dryRun:         dryRun,
		totalChanges:   0,
		currentIndex:   0,
		namespaceStats: make(map[string]*namespaceStats),
	}
}

// StartExecution is called at the beginning of plan execution
func (r *ConsoleReporter) StartExecution(plan *planner.Plan) {
	if r.writer == nil {
		return
	}
	
	// Store total changes and reset current index
	r.totalChanges = plan.Summary.TotalChanges
	r.currentIndex = 0
	
	if plan.Summary.TotalChanges == 0 {
		fmt.Fprintln(r.writer, "No changes to execute.")
		return
	}

	// In dry-run mode, show a simple header
	if r.dryRun {
		fmt.Fprintln(r.writer, "Validating changes:")
	} else {
		fmt.Fprintln(r.writer, "Applying changes:")
	}
}

// StartChange is called before executing a change
func (r *ConsoleReporter) StartChange(change planner.PlannedChange) {
	if r.writer == nil {
		return
	}
	
	// Increment current index for this change
	r.currentIndex++
	
	// Initialize namespace stats if needed
	namespace := change.Namespace
	if namespace == "" {
		namespace = "default"
	}
	if r.namespaceStats[namespace] == nil {
		r.namespaceStats[namespace] = &namespaceStats{}
	}
	
	action := getActionVerb(change.Action)
	resourceName := formatResourceNameForProgress(change)
	
	// Show progress counter with namespace if we have total changes
	if r.totalChanges > 0 {
		fmt.Fprintf(r.writer, "[%d/%d] [namespace: %s] %s %s: %s... ", 
			r.currentIndex, r.totalChanges, namespace, action, change.ResourceType, resourceName)
	} else {
		fmt.Fprintf(r.writer, "• [namespace: %s] %s %s: %s... ", 
			namespace, action, change.ResourceType, resourceName)
	}
}

// CompleteChange is called after a change is executed (success or failure)
func (r *ConsoleReporter) CompleteChange(change planner.PlannedChange, err error) {
	if r.writer == nil {
		return
	}
	
	// Update namespace stats
	namespace := change.Namespace
	if namespace == "" {
		namespace = "default"
	}
	
	if err != nil {
		fmt.Fprintf(r.writer, "✗ Error: %s\n", err.Error())
		if stats := r.namespaceStats[namespace]; stats != nil {
			stats.failureCount++
		}
	} else {
		fmt.Fprintln(r.writer, "✓")
		if stats := r.namespaceStats[namespace]; stats != nil {
			stats.successCount++
		}
	}
}

// SkipChange is called when a change is skipped
func (r *ConsoleReporter) SkipChange(change planner.PlannedChange, reason string) {
	if r.writer == nil {
		return
	}
	
	// Update namespace stats
	namespace := change.Namespace
	if namespace == "" {
		namespace = "default"
	}
	if stats := r.namespaceStats[namespace]; stats != nil {
		stats.skippedCount++
	}
	
	fmt.Fprintf(r.writer, "⚠ Skipped: %s\n", reason)
}

// FinishExecution is called at the end of plan execution
func (r *ConsoleReporter) FinishExecution(result *ExecutionResult) {
	if r.writer == nil {
		return
	}
	fmt.Fprintln(r.writer, "")
	
	// Show namespace breakdown if we have multiple namespaces
	if len(r.namespaceStats) > 1 {
		fmt.Fprintln(r.writer, "\nNamespace Summary:")
		
		// Sort namespaces for consistent output
		namespaces := make([]string, 0, len(r.namespaceStats))
		for ns := range r.namespaceStats {
			namespaces = append(namespaces, ns)
		}
		sort.Strings(namespaces)
		
		for _, ns := range namespaces {
			stats := r.namespaceStats[ns]
			total := stats.successCount + stats.failureCount + stats.skippedCount
			if total > 0 {
				fmt.Fprintf(r.writer, "  %s: ", ns)
				
				parts := []string{}
				if stats.successCount > 0 {
					parts = append(parts, fmt.Sprintf("%d succeeded", stats.successCount))
				}
				if stats.failureCount > 0 {
					parts = append(parts, fmt.Sprintf("%d failed", stats.failureCount))
				}
				if stats.skippedCount > 0 && r.dryRun {
					parts = append(parts, fmt.Sprintf("%d validated", stats.skippedCount))
				}
				
				fmt.Fprintln(r.writer, strings.Join(parts, ", "))
			}
		}
		fmt.Fprintln(r.writer, "")
	}
	
	if result.DryRun {
		// For dry-run, show what would happen
		fmt.Fprintln(r.writer, "Dry run complete.")
		if result.SkippedCount > 0 {
			fmt.Fprintf(r.writer, "%d changes would be applied.\n", result.SkippedCount)
		}
		
		if result.FailureCount > 0 {
			fmt.Fprintln(r.writer, "\nValidation errors:")
			for _, err := range result.Errors {
				fmt.Fprintf(r.writer, "  • %s %s: %s\n", err.ResourceType, err.ResourceName, err.Error)
			}
		}
	} else {
		// For actual execution, show results
		fmt.Fprintln(r.writer, "Complete.")
		if result.SuccessCount > 0 {
			fmt.Fprintf(r.writer, "Applied %d changes.\n", result.SuccessCount)
		}
		
		if result.FailureCount > 0 && len(result.Errors) > 0 {
			fmt.Fprintln(r.writer, "\nErrors:")
			for _, err := range result.Errors {
				fmt.Fprintf(r.writer, "  • %s %s: %s\n", 
					err.ResourceType, err.ResourceName, err.Error)
			}
		}
	}
}

// getActionVerb converts an ActionType to a present-tense verb for display
func getActionVerb(action planner.ActionType) string {
	switch action {
	case planner.ActionCreate:
		return "Creating"
	case planner.ActionUpdate:
		return "Updating"
	case planner.ActionDelete:
		return "Deleting"
	default:
		return string(action) + "ing"
	}
}

// Ensure ConsoleReporter implements ProgressReporter
var _ ProgressReporter = (*ConsoleReporter)(nil)

// formatResourceNameForProgress formats a resource name for display, using monikers when ref is unknown
func formatResourceNameForProgress(change planner.PlannedChange) string {
	resourceName := change.ResourceRef
	
	// If resource ref is unknown, try to build a meaningful name from monikers
	if resourceName == "[unknown]" && len(change.ResourceMonikers) > 0 {
		switch change.ResourceType {
		case "portal_page":
			if slug, ok := change.ResourceMonikers["slug"]; ok {
				if parent, ok := change.ResourceMonikers["parent_portal"]; ok {
					return fmt.Sprintf("page '%s' in portal:%s", slug, parent)
				}
				return fmt.Sprintf("page '%s'", slug)
			}
		case "portal_snippet":
			if name, ok := change.ResourceMonikers["name"]; ok {
				if parent, ok := change.ResourceMonikers["parent_portal"]; ok {
					return fmt.Sprintf("snippet '%s' in portal:%s", name, parent)
				}
				return fmt.Sprintf("snippet '%s'", name)
			}
		case "api_document":
			if slug, ok := change.ResourceMonikers["slug"]; ok {
				if parent, ok := change.ResourceMonikers["parent_api"]; ok {
					return fmt.Sprintf("document '%s' in api:%s", slug, parent)
				}
				return fmt.Sprintf("document '%s'", slug)
			}
		case "api_publication":
			if portal, ok := change.ResourceMonikers["portal_name"]; ok {
				if api, ok := change.ResourceMonikers["api_ref"]; ok {
					return fmt.Sprintf("api:%s published to portal:%s", api, portal)
				}
				return fmt.Sprintf("published to portal:%s", portal)
			}
		}
		// Fallback: show available monikers in a generic format
		var parts []string
		for key, value := range change.ResourceMonikers {
			parts = append(parts, fmt.Sprintf("%s=%s", key, value))
		}
		if len(parts) > 0 {
			sort.Strings(parts) // Consistent ordering
			return strings.Join(parts, ", ")
		}
	}
	
	// Fallback to normal behavior
	if resourceName == "" {
		resourceName = fmt.Sprintf("%s/%s", change.ResourceType, change.ID)
	}
	
	return resourceName
}