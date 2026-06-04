package executor

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
)

// ConsoleReporter provides console output for plan execution progress
type ConsoleReporter struct {
	mu             sync.Mutex
	writer         io.Writer
	dryRun         bool
	totalChanges   int
	currentIndex   int
	namespaceStats map[string]*namespaceStats
	pendingByID    map[string]changeProgress
	pendingByKey   map[string][]changeProgress
}

type changeProgress struct {
	Index        int
	Namespace    string
	Action       string
	ResourceType string
	ResourceName string
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
		pendingByID:    make(map[string]changeProgress),
		pendingByKey:   make(map[string][]changeProgress),
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
		pendingByID:    make(map[string]changeProgress),
		pendingByKey:   make(map[string][]changeProgress),
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
	r.pendingByID = make(map[string]changeProgress)
	r.pendingByKey = make(map[string][]changeProgress)

	if plan.Summary.TotalChanges == 0 {
		fmt.Fprintln(r.writer, "No changes to execute.")
		return
	}

	// In dry-run mode, show a simple header
	if r.dryRun {
		fmt.Fprintln(r.writer, "Validating changes:")
	} else {
		fmt.Fprintln(r.writer, "Executing changes:")
	}
}

// StartChange is called before executing a change
func (r *ConsoleReporter) StartChange(change planner.PlannedChange) {
	if r.writer == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Increment current index for this change
	r.currentIndex++

	progress := buildChangeProgress(change, r.currentIndex)

	if change.ID != "" {
		r.pendingByID[change.ID] = progress
		return
	}

	key := changeProgressFallbackKey(change)
	r.pendingByKey[key] = append(r.pendingByKey[key], progress)
}

// CompleteChange is called after a change is executed (success or failure)
func (r *ConsoleReporter) CompleteChange(change planner.PlannedChange, err error) {
	if r.writer == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	progress := r.takeOrCreateProgress(change)
	r.ensureNamespaceStats(progress.Namespace)

	if err != nil {
		r.namespaceStats[progress.Namespace].failureCount++
		fmt.Fprintf(r.writer, "%s ✗ Error: %s\n", r.changePrefix(progress), err.Error())
	} else {
		r.namespaceStats[progress.Namespace].successCount++
		fmt.Fprintf(r.writer, "%s ✓\n", r.changePrefix(progress))
	}
}

// SkipChange is called when a change is skipped
func (r *ConsoleReporter) SkipChange(change planner.PlannedChange, reason string) {
	if r.writer == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	progress := r.takeOrCreateProgress(change)
	r.ensureNamespaceStats(progress.Namespace)
	r.namespaceStats[progress.Namespace].skippedCount++

	fmt.Fprintf(r.writer, "%s ⚠ Skipped: %s\n", r.changePrefix(progress), reason)
}

func buildChangeProgress(change planner.PlannedChange, index int) changeProgress {
	namespace := change.Namespace
	if namespace == "" {
		namespace = planner.DefaultNamespace
	}

	return changeProgress{
		Index:        index,
		Namespace:    namespace,
		Action:       getActionVerb(change.Action),
		ResourceType: change.ResourceType,
		ResourceName: formatResourceNameForProgress(change),
	}
}

func changeProgressFallbackKey(change planner.PlannedChange) string {
	return change.Namespace + "|" + string(change.Action) + "|" + change.ResourceType + "|" + change.ResourceRef
}

func (r *ConsoleReporter) takeOrCreateProgress(change planner.PlannedChange) changeProgress {
	if change.ID != "" {
		if progress, ok := r.pendingByID[change.ID]; ok {
			delete(r.pendingByID, change.ID)
			return progress
		}
	}

	key := changeProgressFallbackKey(change)
	if queue := r.pendingByKey[key]; len(queue) > 0 {
		progress := queue[0]
		rest := queue[1:]
		if len(rest) == 0 {
			delete(r.pendingByKey, key)
		} else {
			r.pendingByKey[key] = rest
		}
		return progress
	}

	// Fall back gracefully when completion arrives without a recorded start.
	r.currentIndex++
	return buildChangeProgress(change, r.currentIndex)
}

func (r *ConsoleReporter) ensureNamespaceStats(namespace string) {
	if r.namespaceStats[namespace] == nil {
		r.namespaceStats[namespace] = &namespaceStats{}
	}
}

func (r *ConsoleReporter) changePrefix(progress changeProgress) string {
	if r.totalChanges > 0 {
		return fmt.Sprintf(
			"[%d/%d] [namespace: %s] %s %s: %s...",
			progress.Index,
			r.totalChanges,
			progress.Namespace,
			progress.Action,
			progress.ResourceType,
			progress.ResourceName,
		)
	}

	return fmt.Sprintf(
		"• [namespace: %s] %s %s: %s...",
		progress.Namespace,
		progress.Action,
		progress.ResourceType,
		progress.ResourceName,
	)
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
			fmt.Fprintf(r.writer, "Executed %d changes.\n", result.SuccessCount)
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
	case planner.ActionExternalTool:
		return "Running"
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
	if resourceName == resources.UnknownReferenceID && len(change.ResourceMonikers) > 0 {
		switch change.ResourceType {
		case planner.ResourceTypePortalPage:
			if slug, ok := change.ResourceMonikers[planner.FieldSlug]; ok {
				if parent, ok := change.ResourceMonikers["parent_portal"]; ok {
					return fmt.Sprintf("page '%s' in portal:%s", slug, parent)
				}
				return fmt.Sprintf("page '%s'", slug)
			}
		case planner.ResourceTypePortalSnippet:
			if name, ok := change.ResourceMonikers[planner.FieldName]; ok {
				if parent, ok := change.ResourceMonikers["parent_portal"]; ok {
					return fmt.Sprintf("snippet '%s' in portal:%s", name, parent)
				}
				return fmt.Sprintf("snippet '%s'", name)
			}
		case planner.ResourceTypeAPIDocument:
			if slug, ok := change.ResourceMonikers[planner.FieldSlug]; ok {
				if parent, ok := change.ResourceMonikers["parent_api"]; ok {
					return fmt.Sprintf("document '%s' in api:%s", slug, parent)
				}
				return fmt.Sprintf("document '%s'", slug)
			}
		case planner.ResourceTypeAPIPublication:
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
