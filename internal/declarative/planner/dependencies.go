package planner

import (
	"fmt"
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

	// Initialize graph
	for _, change := range changes {
		changeID := change.ID
		allChanges[changeID] = true

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
		return nil, fmt.Errorf("circular dependency detected in plan")
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
	case "api_version", "api_publication", "api_implementation":
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