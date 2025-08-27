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

		// Group deletes by namespace
		deletesByNamespace := make(map[string][]planner.PlannedChange)
		for _, change := range plan.Changes {
			if change.Action == planner.ActionDelete {
				namespace := change.Namespace
				if namespace == "" {
					namespace = "default"
				}
				deletesByNamespace[namespace] = append(deletesByNamespace[namespace], change)
			}
		}

		// Sort namespaces
		namespaces := make([]string, 0, len(deletesByNamespace))
		for ns := range deletesByNamespace {
			namespaces = append(namespaces, ns)
		}
		sort.Strings(namespaces)

		// Display deletions by namespace
		for _, namespace := range namespaces {
			if len(namespaces) > 1 {
				fmt.Fprintf(stderr, "  Namespace %s:\n", namespace)
			}
			for _, change := range deletesByNamespace[namespace] {
				resourceName := formatResourceName(change)
				prefix := "- "
				if len(namespaces) > 1 {
					prefix = "    - "
				}
				fmt.Fprintf(stderr, "%s%s: %s\n", prefix, change.ResourceType, resourceName)
			}
		}
	}

	// Add CONFIRM? section
	fmt.Fprintln(stderr, "\nCONFIRM?")
	fmt.Fprintln(stderr, strings.Repeat("-", 70))
	fmt.Fprint(stderr, "Do you want to continue? Type 'yes' to confirm: ")

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

// DisplayPlanSummary shows an enhanced summary of the plan with better formatting,
// field-level changes, protected resource warnings, and comprehensive statistics.
func DisplayPlanSummary(plan *planner.Plan, out io.Writer) {
	if plan.Summary.ByAction == nil || len(plan.Changes) == 0 {
		fmt.Fprintln(out, "No changes detected. Configuration matches current state.")
		return
	}

	// Group changes by namespace first, then by resource type
	changesByNamespace := make(map[string]map[string][]planner.PlannedChange)
	namespaces := make([]string, 0)
	namespaceSeen := make(map[string]bool)

	// Count different types of protected resource changes
	protectedBeingCreated := 0
	protectedBeingModified := 0
	protectedBeingRemoved := 0

	for _, change := range plan.Changes {
		namespace := change.Namespace
		if namespace == "" {
			namespace = "default"
		}

		if !namespaceSeen[namespace] {
			namespaceSeen[namespace] = true
			namespaces = append(namespaces, namespace)
		}

		if changesByNamespace[namespace] == nil {
			changesByNamespace[namespace] = make(map[string][]planner.PlannedChange)
		}

		changesByNamespace[namespace][change.ResourceType] = append(
			changesByNamespace[namespace][change.ResourceType], change)

		// Count protected resource changes by type
		if change.Action == planner.ActionCreate && willBeProtected(change) {
			protectedBeingCreated++
		} else if change.Action == planner.ActionUpdate {
			if pc, ok := change.Protection.(planner.ProtectionChange); ok {
				if pc.Old && !pc.New {
					protectedBeingRemoved++
				} else if pc.Old {
					protectedBeingModified++
				}
			} else if isProtectedResource(change) {
				protectedBeingModified++
			}
		}
	}

	// Sort namespaces for consistent output
	sort.Strings(namespaces)

	// Display changes organized by namespace, then by resource type (FIRST)
	fmt.Fprintln(out, "\nRESOURCE CHANGES")
	fmt.Fprintln(out, strings.Repeat("-", 70))
	for nsIdx, namespace := range namespaces {
		changesByResource := changesByNamespace[namespace]

		// Count total changes in this namespace
		namespaceTotal := 0
		createCount, updateCount, deleteCount := 0, 0, 0
		for _, changes := range changesByResource {
			namespaceTotal += len(changes)
			for _, change := range changes {
				switch change.Action {
				case planner.ActionCreate:
					createCount++
				case planner.ActionUpdate:
					updateCount++
				case planner.ActionDelete:
					deleteCount++
				}
			}
		}

		// Add spacing between namespaces (but not before the first one)
		if nsIdx > 0 {
			fmt.Fprintln(out, "")
		}

		// Display namespace header with statistics
		fmt.Fprintf(out, "Namespace: %s (%d changes: ", namespace, namespaceTotal)
		actionSummary := []string{}
		if createCount > 0 {
			actionSummary = append(actionSummary, fmt.Sprintf("%d create", createCount))
		}
		if updateCount > 0 {
			actionSummary = append(actionSummary, fmt.Sprintf("%d update", updateCount))
		}
		if deleteCount > 0 {
			actionSummary = append(actionSummary, fmt.Sprintf("%d delete", deleteCount))
		}
		fmt.Fprintf(out, "%s)\n", strings.Join(actionSummary, ", "))

		// Sort resource types by dependency order
		sortedTypes := sortResourceTypesByDependency(changesByResource, plan.Changes)

		// Display resources within namespace
		for resIdx, resourceType := range sortedTypes {
			changes := changesByResource[resourceType]
			if resIdx > 0 {
				fmt.Fprintln(out, "") // Add blank line between resource types
			}
			fmt.Fprintf(out, "  %s (%d resources):\n", resourceType, len(changes))

			for _, change := range changes {
				resourceName := formatResourceName(change)
				actionPrefix := getActionPrefix(change.Action)

				// Check protection status and create appropriate indicator
				protectedIndicator := ""
				if pc, ok := change.Protection.(planner.ProtectionChange); ok {
					if pc.Old && !pc.New {
						protectedIndicator = " [protected → unprotected]"
					} else if !pc.Old && pc.New {
						protectedIndicator = " [unprotected → protected]"
					} else if pc.Old && pc.New {
						protectedIndicator = " [protected]"
					}
				} else if pcMap, ok := change.Protection.(map[string]any); ok {
					// Handle JSON deserialization
					oldVal, hasOld := pcMap["old"].(bool)
					newVal, hasNew := pcMap["new"].(bool)
					if hasOld && hasNew {
						if oldVal && !newVal {
							protectedIndicator = " [protected → unprotected]"
						} else if !oldVal && newVal {
							protectedIndicator = " [unprotected → protected]"
						} else if oldVal && newVal {
							protectedIndicator = " [protected]"
						}
					}
				} else if prot, ok := change.Protection.(bool); ok && prot {
					if change.Action == planner.ActionCreate {
						protectedIndicator = " [will be protected]"
					} else {
						protectedIndicator = " [protected]"
					}
				}

				// Display the resource change with enhanced formatting
				fmt.Fprintf(out, "    %s %s%s\n", actionPrefix, resourceName, protectedIndicator)

				// Show field-level changes for updates
				if change.Action == planner.ActionUpdate {
					displayFieldChanges(out, change, "      ")
				}

				// Show dependencies if any
				displayDependencies(out, change, plan.Changes, "      ")
			}
		}
	}

	// Display protected resource warnings if any (SECOND)
	if protectedBeingCreated > 0 || protectedBeingModified > 0 || protectedBeingRemoved > 0 {
		fmt.Fprintf(out, "\nPROTECTED RESOURCES\n")
		fmt.Fprintln(out, strings.Repeat("-", 70))

		if protectedBeingCreated > 0 {
			fmt.Fprintf(
				out,
				"  Adding protection to %d resource(s) - these changes will succeed\n",
				protectedBeingCreated,
			)
		}

		if protectedBeingRemoved > 0 {
			fmt.Fprintf(
				out,
				"  Removing protection from %d resource(s) - these changes will succeed\n",
				protectedBeingRemoved,
			)
		}

		if protectedBeingModified > 0 {
			fmt.Fprintf(out, "  Attempting to modify %d protected resource(s) - these changes will fail\n",
				protectedBeingModified)
			fmt.Fprintln(out, "  To modify protected resources, first update them to set protected: false")
		}
	}

	// Show warnings if any with enhanced formatting
	if len(plan.Warnings) > 0 {
		fmt.Fprintln(out, "\nWARNINGS")
		fmt.Fprintln(out, strings.Repeat("-", 70))
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
					fmt.Fprintf(out, "  [%s] %s: %s\n", parts[0], change.ResourceType, change.ResourceRef)
					fmt.Fprintf(out, "      %s\n", warning.Message)
				} else {
					fmt.Fprintf(out, "  %s\n", warning.Message)
				}
			} else {
				fmt.Fprintf(out, "  %s\n", warning.Message)
			}
		}
		fmt.Fprintln(out, strings.Repeat("-", 70))
	}

	// Display summary statistics at the end (THIRD)
	displaySummary(plan, out)
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
		// Ensure the resource type has an entry in typeDependencies
		if _, exists := typeDependencies[change.ResourceType]; !exists {
			typeDependencies[change.ResourceType] = make(map[string]bool)
		}

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

// formatResourceName formats a resource name for display, using monikers when ref is unknown
func formatResourceName(change planner.PlannedChange) string {
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
		// Try to get name from fields
		if name, ok := change.Fields["name"].(string); ok {
			resourceName = name
		}
	}

	return resourceName
}

// displaySummary shows comprehensive plan summary at the end
func displaySummary(plan *planner.Plan, out io.Writer) {
	fmt.Fprintln(out, "\nSUMMARY")
	fmt.Fprintln(out, strings.Repeat("-", 70))

	// Action breakdown
	createCount := plan.Summary.ByAction[planner.ActionCreate]
	updateCount := plan.Summary.ByAction[planner.ActionUpdate]
	deleteCount := plan.Summary.ByAction[planner.ActionDelete]

	fmt.Fprintf(out, "  Total changes: %d\n", plan.Summary.TotalChanges)

	// Namespace count
	namespaces := make(map[string]bool)
	for _, change := range plan.Changes {
		namespace := change.Namespace
		if namespace == "" {
			namespace = "default"
		}
		namespaces[namespace] = true
	}
	fmt.Fprintf(out, "  Namespaces affected: %d\n", len(namespaces))

	if createCount > 0 {
		fmt.Fprintf(out, "  Resources to create: %d\n", createCount)
	}
	if updateCount > 0 {
		fmt.Fprintf(out, "  Resources to update: %d\n", updateCount)
	}
	if deleteCount > 0 {
		fmt.Fprintf(out, "  Resources to delete: %d\n", deleteCount)
	}

	// Resource type breakdown
	if len(plan.Summary.ByResource) > 0 {
		fmt.Fprintln(out, "\n  Resource breakdown:")
		// Sort resource types for consistent output
		resourceTypes := make([]string, 0, len(plan.Summary.ByResource))
		for resourceType := range plan.Summary.ByResource {
			resourceTypes = append(resourceTypes, resourceType)
		}
		sort.Strings(resourceTypes)

		for _, resourceType := range resourceTypes {
			count := plan.Summary.ByResource[resourceType]
			fmt.Fprintf(out, "    %s: %d\n", resourceType, count)
		}
	}

	// Protection changes
	if plan.Summary.ProtectionChanges != nil {
		if plan.Summary.ProtectionChanges.Protecting > 0 {
			fmt.Fprintf(out, "  Resources being protected: %d\n", plan.Summary.ProtectionChanges.Protecting)
		}
		if plan.Summary.ProtectionChanges.Unprotecting > 0 {
			fmt.Fprintf(out, "  Resources being unprotected: %d\n", plan.Summary.ProtectionChanges.Unprotecting)
		}
	}
}

// isProtectedResource checks if a resource is currently protected
func isProtectedResource(change planner.PlannedChange) bool {
	// Check for protection status
	switch p := change.Protection.(type) {
	case bool:
		// For CREATE actions, this indicates the resource will be created with protection
		return change.Action != planner.ActionCreate && p
	case planner.ProtectionChange:
		// Resource is protected if it's currently protected (old value)
		return p.Old
	case map[string]any:
		// Handle JSON deserialization
		if oldVal, hasOld := p["old"].(bool); hasOld {
			return oldVal
		}
	}
	return false
}

// willBeProtected checks if a resource will be protected after the plan executes
func willBeProtected(change planner.PlannedChange) bool {
	switch p := change.Protection.(type) {
	case bool:
		// For CREATE actions, this is the protection status
		return p
	case planner.ProtectionChange:
		// For UPDATE actions, use the new value
		return p.New
	case map[string]any:
		// Handle JSON deserialization
		if newVal, hasNew := p["new"].(bool); hasNew {
			return newVal
		}
		// Fallback to old format
		if val, ok := p["protected"].(bool); ok {
			return val
		}
	}
	return false
}

// displayFieldChanges shows detailed field-level changes for update operations
func displayFieldChanges(out io.Writer, change planner.PlannedChange, indent string) {
	hasFieldChanges := false

	// Collect and sort field names for consistent output
	fieldNames := make([]string, 0, len(change.Fields))
	for field := range change.Fields {
		// Skip internal/special fields
		if field == "current_labels" || strings.HasPrefix(field, "_") {
			continue
		}
		fieldNames = append(fieldNames, field)
	}
	sort.Strings(fieldNames)

	for _, field := range fieldNames {
		value := change.Fields[field]

		// Handle different field change formats
		if fc, ok := value.(planner.FieldChange); ok {
			hasFieldChanges = true
			fmt.Fprintf(out, "%s%s: %v → %v\n", indent, field, formatFieldValue(fc.Old), formatFieldValue(fc.New))
		} else if fc, ok := value.(map[string]any); ok {
			// Handle FieldChange that was unmarshaled from JSON
			if oldVal, hasOld := fc["old"]; hasOld {
				if newVal, hasNew := fc["new"]; hasNew {
					hasFieldChanges = true
					fmt.Fprintf(out, "%s%s: %v → %v\n", indent, field, formatFieldValue(oldVal), formatFieldValue(newVal))
				}
			}
		}
	}

	// Check protection changes
	if pc, ok := change.Protection.(planner.ProtectionChange); ok {
		if pc.Old != pc.New {
			hasFieldChanges = true
			if pc.Old && !pc.New {
				fmt.Fprintf(out, "%sprotection: enabled → disabled\n", indent)
			} else if !pc.Old && pc.New {
				fmt.Fprintf(out, "%sprotection: disabled → enabled\n", indent)
			}
		}
	} else if pc, ok := change.Protection.(map[string]any); ok {
		// Handle JSON deserialization
		if oldVal, hasOld := pc["old"].(bool); hasOld {
			if newVal, hasNew := pc["new"].(bool); hasNew {
				if oldVal != newVal {
					hasFieldChanges = true
					if oldVal && !newVal {
						fmt.Fprintf(out, "%sprotection: enabled → disabled\n", indent)
					} else if !oldVal && newVal {
						fmt.Fprintf(out, "%sprotection: disabled → enabled\n", indent)
					}
				}
			}
		}
	}

	if !hasFieldChanges {
		fmt.Fprintf(out, "%s<configuration changes detected>\n", indent)
	}
}

// displayDependencies shows resource dependencies with better formatting
func displayDependencies(out io.Writer, change planner.PlannedChange,
	allChanges []planner.PlannedChange, indent string,
) {
	if len(change.DependsOn) == 0 && (change.Parent == nil || change.Parent.Ref == "") {
		return
	}

	// Use a map to track unique dependencies
	depMap := make(map[string]bool)

	// Add parent dependency if exists
	if change.Parent != nil && change.Parent.Ref != "" {
		parentType := getParentResourceType(change.ResourceType)
		if parentType != "" {
			depMap[fmt.Sprintf("%s:%s", parentType, change.Parent.Ref)] = true
		}
	}

	// Add explicit dependencies
	for _, depID := range change.DependsOn {
		// Find the dependent change to get its type and ref
		for _, depChange := range allChanges {
			if depChange.ID == depID {
				depMap[fmt.Sprintf("%s:%s", depChange.ResourceType, depChange.ResourceRef)] = true
				break
			}
		}
	}

	// Display dependencies if any
	if len(depMap) > 0 {
		deps := make([]string, 0, len(depMap))
		for dep := range depMap {
			deps = append(deps, dep)
		}
		sort.Strings(deps) // Consistent ordering
		fmt.Fprintf(out, "%sdepends on: %s\n", indent, strings.Join(deps, ", "))
	}
}

// formatFieldValue formats a field value for display, truncating long strings
func formatFieldValue(value any) string {
	switch v := value.(type) {
	case string:
		if len(v) > 50 {
			return fmt.Sprintf("\"%.47s...\"", v)
		}
		return fmt.Sprintf("\"%s\"", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case nil:
		return "<nil>"
	default:
		str := fmt.Sprintf("%v", v)
		if len(str) > 50 {
			return fmt.Sprintf("%.47s...", str)
		}
		return str
	}
}
