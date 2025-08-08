package external

import (
	"fmt"
	"sort"
)

// buildDependencyGraph creates a dependency graph for external resources
func (r *ResourceResolver) buildDependencyGraph(
	externalResources []Resource,
) (*DependencyGraph, error) {
	graph := &DependencyGraph{
		Nodes: make(map[string]*DependencyNode),
	}

	// First pass: Create nodes for all resources
	for _, resource := range externalResources {
		node := &DependencyNode{
			Ref:          resource.GetRef(),
			ResourceType: resource.GetResourceType(),
			ChildRefs:    make([]string, 0),
			Resolved:     false,
		}

		// Set parent reference if applicable
		parent := resource.GetParent()
		if parent != nil {
			if parent.GetRef() != "" {
				node.ParentRef = parent.GetRef()
			} else if parent.GetID() != "" {
				// If parent has direct ID, it doesn't create a dependency
				// The parent exists externally and doesn't need resolution
				node.ParentRef = ""
			}
		}

		graph.Nodes[resource.GetRef()] = node
	}

	// Second pass: Build parent-child relationships and validate
	for _, node := range graph.Nodes {
		if node.ParentRef != "" {
			parent, exists := graph.Nodes[node.ParentRef]
			if !exists {
				return nil, fmt.Errorf("parent resource %q not found for child %q",
					node.ParentRef, node.Ref)
			}
			
			// Validate parent-child relationship using registry
			childResource := findResourceByRef(externalResources, node.Ref)
			parentResource := findResourceByRef(externalResources, node.ParentRef)
			
			if childResource != nil && parentResource != nil {
				if !r.registry.IsValidParentChild(parentResource.GetResourceType(), childResource.GetResourceType()) {
					return nil, fmt.Errorf("invalid parent-child relationship: %s (%s) -> %s (%s)",
						parentResource.GetResourceType(), node.ParentRef,
						childResource.GetResourceType(), node.Ref)
				}
			}
			
			parent.ChildRefs = append(parent.ChildRefs, node.Ref)
		}
	}

	// Perform topological sort for resolution order
	order, err := r.topologicalSort(graph)
	if err != nil {
		return nil, fmt.Errorf("failed to determine resolution order: %w", err)
	}

	graph.ResolutionOrder = order
	
	r.logger.Debug("Dependency graph built successfully", 
		"total_resources", len(graph.Nodes),
		"resolution_order", order)
	
	return graph, nil
}

// topologicalSort performs topological sorting of dependency graph using Kahn's algorithm
func (r *ResourceResolver) topologicalSort(graph *DependencyGraph) ([]string, error) {
	// Track in-degree (number of dependencies) for each node
	inDegree := make(map[string]int)
	for ref := range graph.Nodes {
		inDegree[ref] = 0
	}

	// Calculate in-degrees based on parent relationships
	for _, node := range graph.Nodes {
		if node.ParentRef != "" {
			// Check if parent is in our graph (it might be an external ID)
			if _, exists := graph.Nodes[node.ParentRef]; exists {
				inDegree[node.Ref]++
			}
		}
	}

	// Queue of nodes with no dependencies (in-degree 0)
	queue := make([]string, 0)
	for ref, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, ref)
		}
	}

	// Sort queue for deterministic ordering
	sort.Strings(queue)

	result := make([]string, 0, len(graph.Nodes))
	processed := 0

	for len(queue) > 0 {
		// Remove node from queue (FIFO for breadth-first processing)
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)
		processed++

		// Update dependencies for child nodes
		node := graph.Nodes[current]
		for _, childRef := range node.ChildRefs {
			inDegree[childRef]--
			if inDegree[childRef] == 0 {
				queue = append(queue, childRef)
			}
		}
		
		// Keep queue sorted for deterministic ordering
		if len(queue) > 1 {
			sort.Strings(queue)
		}
	}

	// Check for circular dependencies
	if processed != len(graph.Nodes) {
		// Find nodes involved in circular dependency for better error message
		circular := make([]string, 0)
		for ref, degree := range inDegree {
			if degree > 0 {
				circular = append(circular, ref)
			}
		}
		return nil, fmt.Errorf("circular dependency detected among resources: %v", circular)
	}

	return result, nil
}

// ValidateDependencies validates the dependency graph for consistency
func (r *ResourceResolver) ValidateDependencies(graph *DependencyGraph) error {
	if graph == nil {
		return fmt.Errorf("dependency graph is nil")
	}

	for ref, node := range graph.Nodes {
		// Validate parent exists if specified
		if node.ParentRef != "" {
			if _, exists := graph.Nodes[node.ParentRef]; !exists {
				return fmt.Errorf("parent resource %q not found for %q",
					node.ParentRef, ref)
			}
		}

		// Validate children exist
		for _, childRef := range node.ChildRefs {
			if _, exists := graph.Nodes[childRef]; !exists {
				return fmt.Errorf("child resource %q not found for parent %q",
					childRef, ref)
			}
		}
		
		// Check for self-reference
		if node.ParentRef == ref {
			return fmt.Errorf("resource %q has self-reference as parent", ref)
		}
		
		for _, childRef := range node.ChildRefs {
			if childRef == ref {
				return fmt.Errorf("resource %q has self-reference as child", ref)
			}
		}
	}

	// Validate resolution order contains all nodes
	if len(graph.ResolutionOrder) != len(graph.Nodes) {
		return fmt.Errorf("resolution order length (%d) doesn't match node count (%d)",
			len(graph.ResolutionOrder), len(graph.Nodes))
	}

	// Validate all nodes in resolution order exist
	for _, ref := range graph.ResolutionOrder {
		if _, exists := graph.Nodes[ref]; !exists {
			return fmt.Errorf("resource %q in resolution order not found in nodes", ref)
		}
	}

	return nil
}

// GetDependencyInfo returns human-readable dependency information for a resource
func (r *ResourceResolver) GetDependencyInfo(graph *DependencyGraph, ref string) string {
	node, exists := graph.Nodes[ref]
	if !exists {
		return fmt.Sprintf("Resource %q not found in dependency graph", ref)
	}

	info := fmt.Sprintf("Resource: %s (type: %s)\n", ref, node.ResourceType)
	
	if node.ParentRef != "" {
		info += fmt.Sprintf("  Parent: %s\n", node.ParentRef)
	} else {
		info += "  Parent: none (top-level resource)\n"
	}
	
	if len(node.ChildRefs) > 0 {
		info += fmt.Sprintf("  Children: %v\n", node.ChildRefs)
	} else {
		info += "  Children: none\n"
	}
	
	// Find position in resolution order
	for i, orderRef := range graph.ResolutionOrder {
		if orderRef == ref {
			info += fmt.Sprintf("  Resolution order: %d of %d\n", i+1, len(graph.ResolutionOrder))
			break
		}
	}
	
	return info
}

// AnalyzeDependencies provides analysis of the dependency graph
func (r *ResourceResolver) AnalyzeDependencies(graph *DependencyGraph) map[string]interface{} {
	analysis := make(map[string]interface{})
	
	// Count resources by type
	typeCount := make(map[string]int)
	for _, node := range graph.Nodes {
		typeCount[node.ResourceType]++
	}
	analysis["resource_types"] = typeCount
	
	// Find root resources (no parents)
	roots := make([]string, 0)
	for ref, node := range graph.Nodes {
		if node.ParentRef == "" {
			roots = append(roots, ref)
		}
	}
	sort.Strings(roots)
	analysis["root_resources"] = roots
	
	// Find leaf resources (no children)
	leaves := make([]string, 0)
	for ref, node := range graph.Nodes {
		if len(node.ChildRefs) == 0 {
			leaves = append(leaves, ref)
		}
	}
	sort.Strings(leaves)
	analysis["leaf_resources"] = leaves
	
	// Calculate max depth
	maxDepth := r.calculateMaxDepth(graph)
	analysis["max_depth"] = maxDepth
	
	// Count total relationships
	totalRelationships := 0
	for _, node := range graph.Nodes {
		if node.ParentRef != "" {
			totalRelationships++
		}
	}
	analysis["total_relationships"] = totalRelationships
	
	return analysis
}

// calculateMaxDepth calculates the maximum depth of the dependency tree
func (r *ResourceResolver) calculateMaxDepth(graph *DependencyGraph) int {
	if len(graph.Nodes) == 0 {
		return 0
	}
	
	// Use BFS to calculate depth
	depths := make(map[string]int)
	maxDepth := 0
	
	// Initialize root nodes with depth 0
	for ref, node := range graph.Nodes {
		if node.ParentRef == "" {
			depths[ref] = 0
		}
	}
	
	// Process nodes in resolution order (which is topologically sorted)
	for _, ref := range graph.ResolutionOrder {
		node := graph.Nodes[ref]
		
		// Calculate depth based on parent
		if node.ParentRef != "" {
			if parentDepth, exists := depths[node.ParentRef]; exists {
				depths[ref] = parentDepth + 1
				if depths[ref] > maxDepth {
					maxDepth = depths[ref]
				}
			}
		}
	}
	
	return maxDepth
}