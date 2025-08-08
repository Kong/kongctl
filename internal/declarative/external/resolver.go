package external

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/kong/kongctl/internal/declarative/state"
)

// ResourceResolver resolves external resource references to Konnect IDs
type ResourceResolver struct {
	registry *ResolutionRegistry
	client   *state.Client
	logger   *slog.Logger
	resolved map[string]*ResolvedResource
}

// NewResourceResolver creates a new resolver instance
func NewResourceResolver(
	registry *ResolutionRegistry,
	client *state.Client,
	logger *slog.Logger,
) *ResourceResolver {
	return &ResourceResolver{
		registry: registry,
		client:   client,
		logger:   logger,
		resolved: make(map[string]*ResolvedResource),
	}
}

// ResolveExternalResources resolves all external resources in dependency order
func (r *ResourceResolver) ResolveExternalResources(
	ctx context.Context,
	externalResources []Resource,
) error {
	if len(externalResources) == 0 {
		return nil
	}

	r.logger.Debug("Starting external resource resolution", "count", len(externalResources))

	// Build dependency graph for resolution ordering
	graph, err := r.buildDependencyGraph(externalResources)
	if err != nil {
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}

	// Resolve resources in dependency order
	for _, ref := range graph.ResolutionOrder {
		resource := findResourceByRef(externalResources, ref)
		if resource == nil {
			return fmt.Errorf("external resource %q not found in dependency graph", ref)
		}
		if err := r.resolveResource(ctx, resource); err != nil {
			return fmt.Errorf("failed to resolve external resource %q: %w", ref, err)
		}
	}

	r.logger.Info("External resource resolution completed", "resolved_count", len(r.resolved))
	return nil
}

// resolveResource resolves a single external resource
func (r *ResourceResolver) resolveResource(ctx context.Context, resource Resource) error {
	// Skip if already resolved
	if _, exists := r.resolved[resource.GetRef()]; exists {
		return nil
	}

	r.logger.Debug("Resolving external resource", "ref", resource.GetRef(), "type", resource.GetResourceType())

	// Get appropriate adapter from registry
	adapter, err := r.registry.GetResolutionAdapter(resource.GetResourceType())
	if err != nil {
		return fmt.Errorf("failed to get adapter for resource type %q: %w", resource.GetResourceType(), err)
	}

	// Prepare parent context if needed
	var parentResource *ResolvedParent
	parent := resource.GetParent()
	if parent != nil {
		// Resolve parent reference
		var parentRef string
		if parent.GetRef() != "" {
			parentRef = parent.GetRef()
		} else if parent.GetID() != "" {
			// For direct ID parent, we still need to check if we've resolved it
			// This handles cases where parent is referenced by ID but might be external too
			parentRef = parent.GetID()
		}

		if parentRef != "" {
			if parentResolved, exists := r.resolved[parentRef]; exists {
				parentResource = &ResolvedParent{
					ResourceType: parentResolved.ResourceType,
					ID:           parentResolved.ID,
					Resource:     parentResolved.Resource,
				}
			} else if parent.GetID() != "" {
				// Direct ID parent that's not in our resolved map
				// This is okay - it might be a resource that exists but isn't external
				parentResource = &ResolvedParent{
					ResourceType: parent.GetResourceType(),
					ID:           parent.GetID(),
					Resource:     nil, // We don't have the full resource object
				}
			} else {
				return fmt.Errorf("parent resource %q not resolved yet", parentRef)
			}
		}
	}

	// Execute resolution via adapter
	var resolved interface{}
	var resolvedID string

	id := resource.GetID()
	selector := resource.GetSelector()
	
	if id != nil && *id != "" {
		// Direct ID resolution
		resolved, err = adapter.GetByID(ctx, *id, parentResource)
		if err != nil {
			return fmt.Errorf("failed to resolve by ID: %w", err)
		}
		resolvedID = *id
	} else if selector != nil {
		// Selector-based resolution
		results, err := adapter.GetBySelector(ctx, selector.GetMatchFields(), parentResource)
		if err != nil {
			return fmt.Errorf("failed to resolve by selector: %w", err)
		}

		// Validate exactly one match
		if len(results) == 0 {
			return r.createZeroMatchError(resource)
		}
		if len(results) > 1 {
			return r.createMultipleMatchError(resource, len(results))
		}

		resolved = results[0]
		// Extract ID from resolved resource - adapter should provide a way to do this
		// For now, we'll need to type assert based on resource type
		resolvedID = r.extractIDFromResource(resource.GetResourceType(), resolved)
	} else {
		return fmt.Errorf("external resource %q has neither ID nor selector specified", resource.GetRef())
	}

	// Store resolved resource
	resolvedResource := &ResolvedResource{
		ID:           resolvedID,
		Resource:     resolved,
		ResourceType: resource.GetResourceType(),
		Ref:          resource.GetRef(),
		ResolvedAt:   time.Now(),
	}

	// Set parent reference if applicable
	if parentResource != nil && parent != nil && parent.GetRef() != "" {
		if parentResolved, exists := r.resolved[parent.GetRef()]; exists {
			resolvedResource.Parent = parentResolved
		}
	}

	r.resolved[resource.GetRef()] = resolvedResource

	// Update original resource with resolved ID
	resource.SetResolvedID(resolvedID)
	resource.SetResolvedResource(resolved)

	r.logger.Debug("External resource resolved", "ref", resource.GetRef(), "id", resolvedID)
	return nil
}

// extractIDFromResource extracts the ID from a resolved resource based on its type
func (r *ResourceResolver) extractIDFromResource(resourceType string, resource interface{}) string {
	// This is a temporary implementation - ideally the adapter should provide an ExtractID method
	// For now, we'll use type assertions based on known SDK types
	// This will be updated when we have actual SDK integration
	
	// Generic approach: try to extract ID field using reflection
	// For actual implementation, we'll need proper type assertions based on SDK types
	if m, ok := resource.(map[string]interface{}); ok {
		if id, exists := m["id"]; exists {
			if idStr, ok := id.(string); ok {
				return idStr
			}
		}
	}
	
	r.logger.Warn("Could not extract ID from resolved resource, using empty string", 
		"resource_type", resourceType)
	return ""
}

// GetResolvedResource retrieves a resolved resource by reference
func (r *ResourceResolver) GetResolvedResource(ref string) (*ResolvedResource, bool) {
	resolved, exists := r.resolved[ref]
	return resolved, exists
}

// HasResolvedResource checks if a resource reference has been resolved
func (r *ResourceResolver) HasResolvedResource(ref string) bool {
	_, exists := r.resolved[ref]
	return exists
}

// GetResolvedID returns just the resolved ID for a reference
func (r *ResourceResolver) GetResolvedID(ref string) (string, bool) {
	if resolved, exists := r.resolved[ref]; exists {
		return resolved.ID, true
	}
	return "", false
}

// GetAllResolvedResources returns all resolved resources (useful for debugging/testing)
func (r *ResourceResolver) GetAllResolvedResources() map[string]*ResolvedResource {
	// Return a copy to prevent external modification
	result := make(map[string]*ResolvedResource, len(r.resolved))
	for k, v := range r.resolved {
		result[k] = v
	}
	return result
}

// ClearCache clears the resolved resource cache (useful for testing)
func (r *ResourceResolver) ClearCache() {
	r.resolved = make(map[string]*ResolvedResource)
}

// Helper functions for error creation
func (r *ResourceResolver) createZeroMatchError(resource Resource) error {
	selectorStr := ""
	selector := resource.GetSelector()
	if selector != nil {
		selectorStr = fmt.Sprintf("%+v", selector.GetMatchFields())
	}
	
	return fmt.Errorf("external resource %q selector matched 0 resources\n"+
		"  Resource type: %s\n"+
		"  Selector: %s\n"+
		"  Suggestion: Verify the resource exists in Konnect and the selector fields are correct",
		resource.GetRef(), resource.GetResourceType(), selectorStr)
}

func (r *ResourceResolver) createMultipleMatchError(resource Resource, count int) error {
	selectorStr := ""
	selector := resource.GetSelector()
	if selector != nil {
		selectorStr = fmt.Sprintf("%+v", selector.GetMatchFields())
	}
	
	return fmt.Errorf("external resource %q selector matched %d resources\n"+
		"  Resource type: %s\n"+
		"  Selector: %s\n"+
		"  Suggestion: Use more specific selector fields to match exactly one resource",
		resource.GetRef(), count, resource.GetResourceType(), selectorStr)
}

// findResourceByRef finds an external resource by reference
func findResourceByRef(resources []Resource, ref string) Resource {
	for _, resource := range resources {
		if resource.GetRef() == ref {
			return resource
		}
	}
	return nil
}

