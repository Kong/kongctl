package external

import (
	"fmt"
	"sync"
)

// ResolutionRegistry manages resolution metadata for external resource types
type ResolutionRegistry struct {
	mu    sync.RWMutex
	types map[string]*ResolutionMetadata
}

var (
	registry     *ResolutionRegistry
	registryOnce sync.Once
)

// GetResolutionRegistry returns the singleton registry instance
func GetResolutionRegistry() *ResolutionRegistry {
	registryOnce.Do(func() {
		registry = &ResolutionRegistry{
			types: make(map[string]*ResolutionMetadata),
		}
		// Initialize with built-in resource types
		registry.initializeBuiltinTypes()
	})
	return registry
}

// Register adds resolution metadata for a resource type to the registry
func (r *ResolutionRegistry) Register(resourceType string, info *ResolutionMetadata) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.types[resourceType] = info
}

// IsSupported returns true if the resource type is supported for resolution
func (r *ResolutionRegistry) IsSupported(resourceType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.types[resourceType]
	return exists
}

// GetSupportedTypes returns a list of all resource types that can be resolved
func (r *ResolutionRegistry) GetSupportedTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.types))
	for t := range r.types {
		types = append(types, t)
	}
	return types
}

// GetSupportedSelectorFields returns supported fields for selector-based resolution
func (r *ResolutionRegistry) GetSupportedSelectorFields(resourceType string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if info, exists := r.types[resourceType]; exists {
		return info.SelectorFields
	}
	return nil
}

// IsValidParentChild returns true if the parent-child relationship is valid for resolution
func (r *ResolutionRegistry) IsValidParentChild(parentType, childType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	parentInfo, parentExists := r.types[parentType]
	if !parentExists {
		return false
	}

	for _, supportedChild := range parentInfo.SupportedChildren {
		if supportedChild == childType {
			return true
		}
	}

	return false
}

// GetResolutionAdapter returns the resolution adapter for a resource type
func (r *ResolutionRegistry) GetResolutionAdapter(resourceType string) (ResolutionAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.types[resourceType]
	if !exists {
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	if info.ResolutionAdapter == nil {
		return nil, fmt.Errorf("no resolution adapter configured for resource type: %s", resourceType)
	}

	return info.ResolutionAdapter, nil
}

// GetResolutionMetadata returns the full resolution metadata for a resource type
func (r *ResolutionRegistry) GetResolutionMetadata(resourceType string) (*ResolutionMetadata, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.types[resourceType]
	return info, exists
}

// InjectAdapters updates the registry with concrete adapter implementations
func (r *ResolutionRegistry) InjectAdapters(adapters map[string]ResolutionAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	for resourceType, adapter := range adapters {
		if metadata, exists := r.types[resourceType]; exists {
			metadata.ResolutionAdapter = adapter
		}
	}
}

// initializeBuiltinTypes registers the built-in resource types with their resolution metadata
func (r *ResolutionRegistry) initializeBuiltinTypes() {
	// Portal resource type
	r.Register("portal", &ResolutionMetadata{
		Name:              "Portal",
		SelectorFields:    []string{"name", "description"},
		SupportedParents:  nil, // Portals are top-level
		SupportedChildren: []string{"portal_customization", "portal_custom_domain", "portal_page", "portal_snippet"},
		ResolutionAdapter: nil, // Will be set in future steps
	})

	// API resource type
	r.Register("api", &ResolutionMetadata{
		Name:              "API",
		SelectorFields:    []string{"name", "description"},
		SupportedParents:  nil, // APIs are top-level
		SupportedChildren: []string{"api_version", "api_publication", "api_implementation", "api_document"},
		ResolutionAdapter: nil, // Will be set in future steps
	})

	// Control Plane resource type
	r.Register("control_plane", &ResolutionMetadata{
		Name:              "Control Plane",
		SelectorFields:    []string{"name", "description"},
		SupportedParents:  nil,                 // Control planes are top-level
		SupportedChildren: []string{"ce_service"}, // Gateway services as children
		ResolutionAdapter: nil,                 // Will be set in future steps
	})

	// Gateway Service (Core Entity) resource type
	r.Register("ce_service", &ResolutionMetadata{
		Name:              "Gateway Service (Core Entity)",
		SelectorFields:    []string{"name"},
		SupportedParents:  []string{"control_plane"}, // Must have control plane parent
		SupportedChildren: nil,                       // No child resources for now
		ResolutionAdapter: nil,                       // Will be set in future steps
	})

	// API Version resource type (child of API)
	r.Register("api_version", &ResolutionMetadata{
		Name:             "API Version",
		SelectorFields:   []string{"name", "version"},
		SupportedParents: []string{"api"},
		SupportedChildren: nil,
		ResolutionAdapter: nil, // Will be set in future steps
	})

	// API Publication resource type (child of API)
	r.Register("api_publication", &ResolutionMetadata{
		Name:             "API Publication",
		SelectorFields:   []string{"name"},
		SupportedParents: []string{"api"},
		SupportedChildren: nil,
		ResolutionAdapter: nil, // Will be set in future steps
	})

	// API Implementation resource type (child of API)
	r.Register("api_implementation", &ResolutionMetadata{
		Name:             "API Implementation",
		SelectorFields:   []string{"name"},
		SupportedParents: []string{"api"},
		SupportedChildren: nil,
		ResolutionAdapter: nil, // Will be set in future steps
	})

	// API Document resource type (child of API)
	r.Register("api_document", &ResolutionMetadata{
		Name:             "API Document",
		SelectorFields:   []string{"name", "path"},
		SupportedParents: []string{"api"},
		SupportedChildren: nil,
		ResolutionAdapter: nil, // Will be set in future steps
	})

	// Application Auth Strategy resource type
	r.Register("application_auth_strategy", &ResolutionMetadata{
		Name:              "Application Auth Strategy",
		SelectorFields:    []string{"name"},
		SupportedParents:  nil, // Top-level resource
		SupportedChildren: nil,
		ResolutionAdapter: nil, // Will be set in future steps
	})

	// Portal Customization resource type (child of Portal)
	r.Register("portal_customization", &ResolutionMetadata{
		Name:             "Portal Customization",
		SelectorFields:   []string{"name"},
		SupportedParents: []string{"portal"},
		SupportedChildren: nil,
		ResolutionAdapter: nil, // Will be set in future steps
	})

	// Portal Custom Domain resource type (child of Portal)
	r.Register("portal_custom_domain", &ResolutionMetadata{
		Name:             "Portal Custom Domain",
		SelectorFields:   []string{"domain"},
		SupportedParents: []string{"portal"},
		SupportedChildren: nil,
		ResolutionAdapter: nil, // Will be set in future steps
	})

	// Portal Page resource type (child of Portal)
	r.Register("portal_page", &ResolutionMetadata{
		Name:             "Portal Page",
		SelectorFields:   []string{"name", "slug"},
		SupportedParents: []string{"portal"},
		SupportedChildren: nil,
		ResolutionAdapter: nil, // Will be set in future steps
	})

	// Portal Snippet resource type (child of Portal)
	r.Register("portal_snippet", &ResolutionMetadata{
		Name:             "Portal Snippet",
		SelectorFields:   []string{"name"},
		SupportedParents: []string{"portal"},
		SupportedChildren: nil,
		ResolutionAdapter: nil, // Will be set in future steps
	})
}

