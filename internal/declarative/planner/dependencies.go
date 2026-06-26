package planner

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
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
	graph := make(map[string][]string)       // change_id -> list of dependencies
	inDegree := make(map[string]int)         // change_id -> number of incoming edges
	allChanges := make(map[string]bool)      // set of all change IDs
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
		if change.Parent != nil && unresolvedReferenceID(change.Parent.ID) {
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

// DependencyResolutionResult holds the results of a full dependency resolution run.
type DependencyResolutionResult struct {
	// ExecutionOrder is a flat topological ordering of all change IDs (same semantics as ResolveDependencies).
	ExecutionOrder []string
	// ExecutionGroups groups change IDs by Kahn level; changes within a group are safe
	// to execute concurrently. Groups must be executed sequentially in order.
	ExecutionGroups [][]string
	// FullDepsMap maps each change ID to the complete set of direct dependency IDs,
	// including both explicit DependsOn entries and all implicit edges discovered during
	// graph construction. The planner uses this to persist implicit edges back to
	// PlannedChange.DependsOn so the plan is the single source of truth for ordering.
	FullDepsMap map[string][]string
}

// ResolveDependenciesWithGroups builds the dependency graph, computes a flat
// topological execution order, and additionally computes Kahn-level concurrency
// groups. Changes within the same group are guaranteed to have no dependencies
// on each other and are safe to execute concurrently.
func (d *DependencyResolver) ResolveDependenciesWithGroups(
	changes []PlannedChange,
) (*DependencyResolutionResult, error) {
	graph := make(map[string][]string)                 // dep -> list of dependents
	inDegree := make(map[string]int)                   // change_id -> incoming edge count
	allChanges := make(map[string]bool)                // set of all change IDs
	changeDetails := make(map[string]string)           // change_id -> description for errors
	allDepsPerNode := make(map[string]map[string]bool) // change_id -> set of its dependencies
	previousAIGatewayChildByParent := make(map[string]string)

	for _, change := range changes {
		id := change.ID
		allChanges[id] = true
		changeDetails[id] = fmt.Sprintf("%s:%s:%s", change.Action, change.ResourceType, change.ResourceRef)

		if _, ok := graph[id]; !ok {
			graph[id] = []string{}
		}
		if _, ok := inDegree[id]; !ok {
			inDegree[id] = 0
		}
		if allDepsPerNode[id] == nil {
			allDepsPerNode[id] = make(map[string]bool)
		}

		addEdge := func(dep string) {
			if dep == "" || allDepsPerNode[id][dep] {
				return
			}
			allDepsPerNode[id][dep] = true
			graph[dep] = append(graph[dep], id)
			inDegree[id]++
		}

		for _, dep := range change.DependsOn {
			addEdge(dep)
		}
		for _, dep := range d.findImplicitDependencies(change, changes) {
			addEdge(dep)
		}
		if change.Parent != nil && unresolvedReferenceID(change.Parent.ID) {
			if parentDep := d.findParentChange(change.Parent.Ref, change.ResourceType, changes); parentDep != "" {
				addEdge(parentDep)
			}
		}
		if parentKey := aiGatewayChildSerializationParentKey(change); parentKey != "" {
			if previousID := previousAIGatewayChildByParent[parentKey]; previousID != "" {
				addEdge(previousID)
			}
			previousAIGatewayChildByParent[parentKey] = id
		}
	}

	// Kahn's algorithm — process one level at a time to produce both a flat order
	// and level-grouped concurrency buckets.
	var (
		executionOrder  []string
		executionGroups [][]string
	)

	// Seed with level-0 nodes (no dependencies), sorted for deterministic output.
	var level []string
	for id := range allChanges {
		if inDegree[id] == 0 {
			level = append(level, id)
		}
	}
	slices.Sort(level)

	for len(level) > 0 {
		executionGroups = append(executionGroups, level)
		executionOrder = append(executionOrder, level...)

		var next []string
		for _, id := range level {
			for _, dependent := range graph[id] {
				inDegree[dependent]--
				if inDegree[dependent] == 0 {
					next = append(next, dependent)
				}
			}
		}
		slices.Sort(next)
		level = next
	}

	if len(executionOrder) != len(allChanges) {
		cycleInfo := d.findCycleDetails(graph, inDegree, allChanges, changeDetails)
		return nil, fmt.Errorf("circular dependency detected in plan: %s", cycleInfo)
	}

	// Build FullDepsMap with sorted slices for stable output.
	fullDepsMap := make(map[string][]string, len(allDepsPerNode))
	for id, depsSet := range allDepsPerNode {
		if len(depsSet) == 0 {
			continue
		}
		deps := make([]string, 0, len(depsSet))
		for dep := range depsSet {
			deps = append(deps, dep)
		}
		slices.Sort(deps)
		fullDepsMap[id] = deps
	}

	return &DependencyResolutionResult{
		ExecutionOrder:  executionOrder,
		ExecutionGroups: executionGroups,
		FullDepsMap:     fullDepsMap,
	}, nil
}

func aiGatewayChildSerializationParentKey(change PlannedChange) string {
	switch change.ResourceType {
	case ResourceTypeAIGatewayProvider,
		ResourceTypeAIGatewayPolicy,
		ResourceTypeAIGatewayModel,
		ResourceTypeAIGatewayMCPServer:
	default:
		return ""
	}

	if change.Parent != nil {
		if change.Parent.Ref != "" {
			return "ref:" + change.Parent.Ref
		}
		if !unresolvedReferenceID(change.Parent.ID) {
			return "id:" + change.Parent.ID
		}
	}

	refInfo, ok := change.References[FieldAIGatewayID]
	if !ok {
		return ""
	}
	if refInfo.Ref != "" {
		return "ref:" + refInfo.Ref
	}
	if !unresolvedReferenceID(refInfo.ID) {
		return "id:" + refInfo.ID
	}
	return ""
}

// findImplicitDependencies finds dependencies based on references
func (d *DependencyResolver) findImplicitDependencies(change PlannedChange, allChanges []PlannedChange) []string {
	var dependencies []string

	// Check references field
	for _, refInfo := range change.References {
		if refInfo.IsArray {
			for _, ref := range refInfo.Refs {
				if !tags.IsRefPlaceholder(ref) {
					continue
				}
				parsedRef, _, ok := tags.ParseRefPlaceholder(ref)
				if !ok {
					continue
				}
				for _, other := range allChanges {
					if other.ResourceRef == parsedRef && other.Action == ActionCreate {
						dependencies = append(dependencies, other.ID)
						break
					}
				}
			}
			continue
		}
		ref := refInfo.Ref
		isPlaceholder := tags.IsRefPlaceholder(ref)
		if isPlaceholder {
			parsedRef, _, ok := tags.ParseRefPlaceholder(ref)
			if !ok {
				continue
			}
			ref = parsedRef
		}

		if !isPlaceholder && !unresolvedReferenceID(refInfo.ID) {
			continue
		}

		// Find the change that creates this resource
		for _, other := range allChanges {
			if other.ResourceRef == ref && other.Action == ActionCreate {
				dependencies = append(dependencies, other.ID)
				break
			}
		}
	}

	return dependencies
}

func unresolvedReferenceID(id string) bool {
	trimmed := strings.TrimSpace(id)
	return trimmed == "" || trimmed == resources.UnknownReferenceID
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
	case ResourceTypeAPIVersion, ResourceTypeAPIPublication, ResourceTypeAPIImplementation, ResourceTypeAPIDocument:
		return ResourceTypeAPI
	case ResourceTypePortalPage:
		return ResourceTypePortal
	case ResourceTypeAIGatewayProvider,
		ResourceTypeAIGatewayPolicy,
		ResourceTypeAIGatewayModel,
		ResourceTypeAIGatewayMCPServer:
		return ResourceTypeAIGateway
	default:
		return ""
	}
}

// contains checks if string is in slice
func contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
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
	var details strings.Builder
	fmt.Fprintf(&details, "The following resources form a circular dependency (%d resources):\n", len(cycleNodes))
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
			fmt.Fprintf(&details, "  - %s (%s) is waiting for: %v\n", node, resourceInfo, waitingFor)
		} else if len(deps) > 0 {
			var depDetails []string
			for _, dep := range deps {
				if depInfo, ok := changeDetails[dep]; ok {
					depDetails = append(depDetails, depInfo)
				} else {
					depDetails = append(depDetails, dep)
				}
			}
			fmt.Fprintf(&details, "  - %s (%s) has dependents: %v\n", node, resourceInfo, depDetails)
		} else {
			fmt.Fprintf(&details, "  - %s (%s) has %d unresolved incoming dependencies\n", node, resourceInfo, inDegree[node])
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
		fmt.Fprintf(&details, "\nDetected cycle: %s", strings.Join(pathDetails, " → "))
	}

	return details.String()
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
