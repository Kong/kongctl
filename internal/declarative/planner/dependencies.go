package planner

import (
	"fmt"
	"strings"
)

// DependencyResolver calculates execution order for plan changes
type DependencyResolver struct{}

// NewDependencyResolver creates a new resolver
func NewDependencyResolver() *DependencyResolver {
	return &DependencyResolver{}
}

// ResolveDependencies builds dependency graph and calculates execution order
func (d *DependencyResolver) ResolveDependencies(changes []PlannedChange) ([]string, error) {
	// Build dependency graph
	graph := make(map[string][]string)     // change_id -> list of dependencies
	inDegree := make(map[string]int)       // change_id -> number of incoming edges
	allChanges := make(map[string]bool)    // set of all change IDs
	changeDetails := make(map[string]string) // change_id -> resource details for error reporting

	// Initialize graph
	for _, change := range changes {
		changeID := change.ID
		allChanges[changeID] = true
		changeDetails[changeID] = fmt.Sprintf("%s:%s:%s", change.Action, change.ResourceType, change.ResourceRef)

		if _, exists := graph[changeID]; !exists {
			graph[changeID] = []string{}
		}
		if _, exists := inDegree[changeID]; !exists {
			inDegree[changeID] = 0
		}

		// Add explicit dependencies
		for _, dep := range change.DependsOn {
			graph[dep] = append(graph[dep], changeID)
			inDegree[changeID]++
		}

		// Add implicit dependencies based on references
		deps := d.findImplicitDependencies(change, changes)
		for _, dep := range deps {
			if !contains(change.DependsOn, dep) { // Avoid duplicates
				graph[dep] = append(graph[dep], changeID)
				inDegree[changeID]++
			}
		}

		// Parent dependencies
		if change.Parent != nil && change.Parent.ID == "<unknown>" {
			parentDep := d.findParentChange(change.Parent.Ref, change.ResourceType, changes)
			if parentDep != "" && !contains(change.DependsOn, parentDep) {
				graph[parentDep] = append(graph[parentDep], changeID)
				inDegree[changeID]++
				// Added parent dependency
			} else if parentDep == "" && change.Parent.Ref != "" {
				// Parent not found in changes - this might indicate a problem
				// TODO: Consider adding a warning or error for missing parent
				_ = parentDep // Suppress empty block warning
			}
		}
	}

	// Topological sort using Kahn's algorithm
	queue := []string{}
	for changeID := range allChanges {
		if inDegree[changeID] == 0 {
			queue = append(queue, changeID)
		}
	}

	executionOrder := []string{}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		executionOrder = append(executionOrder, current)

		for _, dependent := range graph[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Check for cycles
	if len(executionOrder) != len(allChanges) {
		// Find which resources are part of the cycle or have unresolved dependencies
		cycleInfo := d.findCycleDetails(graph, inDegree, allChanges, changeDetails)
		return nil, fmt.Errorf("circular dependency detected in plan: %s", cycleInfo)
	}

	return executionOrder, nil
}

// findImplicitDependencies finds dependencies based on references
func (d *DependencyResolver) findImplicitDependencies(change PlannedChange, allChanges []PlannedChange) []string {
	var dependencies []string

	// Check references field
	for _, refInfo := range change.References {
		if refInfo.ID == "<unknown>" {
			// Find the change that creates this resource
			for _, other := range allChanges {
				if other.ResourceRef == refInfo.Ref && other.Action == ActionCreate {
					dependencies = append(dependencies, other.ID)
					break
				}
			}
		}
	}

	return dependencies
}

// findParentChange finds the change that creates the parent resource
func (d *DependencyResolver) findParentChange(parentRef, childResourceType string, changes []PlannedChange) string {
	parentType := d.getParentType(childResourceType)

	for _, change := range changes {
		if change.ResourceRef == parentRef &&
			change.ResourceType == parentType &&
			change.Action == ActionCreate {
			return change.ID
		}
	}

	return ""
}

// getParentType determines parent resource type from child type
func (d *DependencyResolver) getParentType(childType string) string {
	switch childType {
	case "api_version", "api_publication", "api_implementation", "api_document":
		return "api"
	case "portal_page":
		return "portal"
	default:
		return ""
	}
}

// contains checks if string is in slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// findCycleDetails finds and returns detailed information about circular dependencies
func (d *DependencyResolver) findCycleDetails(
	graph map[string][]string, 
	inDegree map[string]int, 
	allChanges map[string]bool, 
	changeDetails map[string]string,
) string {
	// Find nodes that still have dependencies (part of cycle)
	var cycleNodes []string
	for changeID := range allChanges {
		if inDegree[changeID] > 0 {
			cycleNodes = append(cycleNodes, changeID)
		}
	}
	
	if len(cycleNodes) == 0 {
		return "unable to determine cycle participants"
	}
	
	// Build detailed message
	details := fmt.Sprintf("The following resources form a circular dependency (%d resources):\n", len(cycleNodes))
	for _, node := range cycleNodes {
		resourceInfo := changeDetails[node]
		
		// Find what this node is waiting for (incoming edges)
		var waitingFor []string
		for dep, dependents := range graph {
			for _, dependent := range dependents {
				if dependent == node {
					if depInfo, ok := changeDetails[dep]; ok {
						waitingFor = append(waitingFor, fmt.Sprintf("%s (%s)", dep, depInfo))
					} else {
						waitingFor = append(waitingFor, dep)
					}
				}
			}
		}
		
		// Find what depends on this node (outgoing edges)
		deps := graph[node]
		if len(waitingFor) > 0 {
			details += fmt.Sprintf("  - %s (%s) is waiting for: %v\n", node, resourceInfo, waitingFor)
		} else if len(deps) > 0 {
			var depDetails []string
			for _, dep := range deps {
				if depInfo, ok := changeDetails[dep]; ok {
					depDetails = append(depDetails, depInfo)
				} else {
					depDetails = append(depDetails, dep)
				}
			}
			details += fmt.Sprintf("  - %s (%s) has dependents: %v\n", node, resourceInfo, depDetails)
		} else {
			details += fmt.Sprintf("  - %s (%s) has %d unresolved incoming dependencies\n", node, resourceInfo, inDegree[node])
		}
	}
	
	// Try to find a specific cycle path using DFS
	cyclePath := d.findCyclePath(graph, cycleNodes[0], make(map[string]bool), []string{})
	if len(cyclePath) > 0 {
		var pathDetails []string
		for _, node := range cyclePath {
			if info, ok := changeDetails[node]; ok {
				pathDetails = append(pathDetails, fmt.Sprintf("%s (%s)", node, info))
			} else {
				pathDetails = append(pathDetails, node)
			}
		}
		details += fmt.Sprintf("\nDetected cycle: %s", strings.Join(pathDetails, " → "))
	}
	
	return details
}

// findCyclePath uses DFS to find a cycle path starting from a given node
func (d *DependencyResolver) findCyclePath(
	graph map[string][]string, 
	start string, 
	visited map[string]bool, 
	path []string,
) []string {
	// Check if we've found a cycle
	for i, node := range path {
		if node == start && len(path) > 1 {
			// Found cycle, return the cycle portion
			return path[i:]
		}
	}
	
	// Mark as visited
	visited[start] = true
	path = append(path, start)
	
	// DFS on dependencies
	for _, dep := range graph[start] {
		if visited[dep] && contains(path, dep) {
			// Found a cycle
			cycleStart := -1
			for i, node := range path {
				if node == dep {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				cyclePath := append(path[cycleStart:], dep)
				return cyclePath
			}
		}
		
		if !visited[dep] {
			if cyclePath := d.findCyclePath(graph, dep, visited, path); len(cyclePath) > 0 {
				return cyclePath
			}
		}
	}
	
	return nil
}

