package planner

import (
	"context"
	"fmt"
	"sync"

	"github.com/kong/kongctl/internal/declarative/external"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// ReferenceResolver resolves declarative refs to Konnect IDs
type ReferenceResolver struct {
	client           *state.Client
	externalResolver *external.ResourceResolver
	mappingCache     map[string]map[string]string // Cache for resource mappings
	cacheMutex       sync.RWMutex                 // Mutex for cache synchronization
}

// NewReferenceResolver creates a new resolver
func NewReferenceResolver(
	client *state.Client,
	externalResolver *external.ResourceResolver,
) *ReferenceResolver {
	return &ReferenceResolver{
		client:           client,
		externalResolver: externalResolver,
		mappingCache:     make(map[string]map[string]string),
	}
}

// ResolvedReference contains ref and resolved ID
type ResolvedReference struct {
	Ref string
	ID  string
}

// ResolveResult contains resolved reference information
type ResolveResult struct {
	// Map of change_id -> field -> resolved reference
	ChangeReferences map[string]map[string]ResolvedReference
	// Errors encountered during resolution
	Errors []error
}

// ResolveReferences resolves all references in planned changes
func (r *ReferenceResolver) ResolveReferences(ctx context.Context, changes []PlannedChange) (*ResolveResult, error) {
	result := &ResolveResult{
		ChangeReferences: make(map[string]map[string]ResolvedReference),
		Errors:           []error{},
	}

	// Build a map of what's being created in this plan
	createdResources := make(map[string]map[string]string) // resource_type -> ref -> change_id
	for _, change := range changes {
		if change.Action == ActionCreate {
			if createdResources[change.ResourceType] == nil {
				createdResources[change.ResourceType] = make(map[string]string)
			}
			createdResources[change.ResourceType][change.ResourceRef] = change.ID
		}
	}

	// Resolve references for each change
	for _, change := range changes {
		changeRefs := make(map[string]ResolvedReference)

		// Check fields that might contain references
		for fieldName, fieldValue := range change.Fields {
			// First try dynamic resolution based on resource type
			if r.isReferenceFieldDynamic(change.ResourceType, fieldName) {
				if ref, isRef := r.extractReferenceValue(fieldValue); isRef {
					// Use dynamic resource type lookup
					resourceType := r.getResourceTypeForFieldDynamic(change.ResourceType, fieldName)
					
					// Check if this references something being created
					if _, inPlan := createdResources[resourceType][ref]; inPlan {
						changeRefs[fieldName] = ResolvedReference{
							Ref: ref,
							ID:  "[unknown]", // Will be resolved at execution
						}
					} else {
						// Resolve from existing resources
						id, err := r.resolveReference(ctx, resourceType, ref)
						if err != nil {
							result.Errors = append(result.Errors, fmt.Errorf(
								"change %s: failed to resolve %s reference %q: %w",
								change.ID, resourceType, ref, err))
							continue
						}
						changeRefs[fieldName] = ResolvedReference{
							Ref: ref,
							ID:  id,
						}
					}
				}
			} else if ref, isRef := r.extractReference(fieldName, fieldValue); isRef {
				// Fallback to hardcoded approach for backward compatibility
				resourceType := r.getResourceTypeForField(fieldName)

				// Check if this references something being created
				if _, inPlan := createdResources[resourceType][ref]; inPlan {
					changeRefs[fieldName] = ResolvedReference{
						Ref: ref,
						ID:  "[unknown]", // Will be resolved at execution
					}
				} else {
					// Resolve from existing resources
					id, err := r.resolveReference(ctx, resourceType, ref)
					if err != nil {
						result.Errors = append(result.Errors, fmt.Errorf(
							"change %s: failed to resolve %s reference %q: %w",
							change.ID, resourceType, ref, err))
						continue
					}
					changeRefs[fieldName] = ResolvedReference{
						Ref: ref,
						ID:  id,
					}
				}
			}
		}

		if len(changeRefs) > 0 {
			result.ChangeReferences[change.ID] = changeRefs
		}
	}

	return result, nil
}

// extractReference checks if a field value is a reference
func (r *ReferenceResolver) extractReference(fieldName string, value interface{}) (string, bool) {
	// Check if field name suggests a reference
	if !r.isReferenceField(fieldName) {
		return "", false
	}

	// Extract string value
	switch v := value.(type) {
	case string:
		if !isUUID(v) {
			return v, true
		}
	case FieldChange:
		if newVal, ok := v.New.(string); ok && !isUUID(newVal) {
			return newVal, true
		}
	}

	return "", false
}


// isReferenceField checks if field name indicates a reference
func (r *ReferenceResolver) isReferenceField(fieldName string) bool {
	// Fields that contain references to other resources
	referenceFields := []string{
		"default_application_auth_strategy_id",
		"control_plane_id",
		"portal_id",
		"auth_strategy_ids",
		// Add more as needed
	}

	for _, rf := range referenceFields {
		if fieldName == rf ||
			fieldName == "gateway_service."+rf ||
			fieldName == "gateway_service.service_id" ||
			fieldName == "service."+rf {
			return true
		}
	}
	return false
}

// getResourceTypeForField maps field names to resource types
func (r *ReferenceResolver) getResourceTypeForField(fieldName string) string {
	switch fieldName {
	case "default_application_auth_strategy_id", "auth_strategy_ids":
		return "application_auth_strategy"
	case "control_plane_id", "gateway_service.control_plane_id", "service.control_plane_id":
		return "control_plane"
	case "portal_id":
		return ResourceTypePortal
	default:
		return ""
	}
}

// resolveReference looks up a reference in existing resources
func (r *ReferenceResolver) resolveReference(ctx context.Context, resourceType, ref string) (string, error) {
	// Check external resources first
	if r.externalResolver != nil {
		if resolvedID, found := r.externalResolver.GetResolvedID(ref); found {
			return resolvedID, nil
		}
	}
	
	// Fall back to internal resolution
	switch resourceType {
	case "application_auth_strategy":
		return r.resolveAuthStrategyRef(ctx, ref)
	case "control_plane":
		return r.resolveControlPlaneRef(ctx, ref)
	case ResourceTypePortal:
		return r.resolvePortalRef(ctx, ref)
	default:
		return "", fmt.Errorf("unknown resource type: %s", resourceType)
	}
}

// resolveAuthStrategyRef resolves auth strategy ref to ID
func (r *ReferenceResolver) resolveAuthStrategyRef(_ context.Context, _ string) (string, error) {
	// TODO: Implement when auth strategy API is available
	return "", fmt.Errorf("auth strategy resolution not yet implemented")
}

// resolveControlPlaneRef resolves control plane ref to ID
func (r *ReferenceResolver) resolveControlPlaneRef(_ context.Context, _ string) (string, error) {
	// TODO: Implement when control plane state client is available
	return "", fmt.Errorf("control plane resolution not yet implemented")
}

// resolvePortalRef resolves portal ref to ID
func (r *ReferenceResolver) resolvePortalRef(ctx context.Context, ref string) (string, error) {
	portal, err := r.client.GetPortalByName(ctx, ref)
	if err != nil {
		return "", err
	}
	if portal == nil {
		return "", fmt.Errorf("portal not found")
	}
	return portal.ID, nil
}

// isUUID checks if string is already a UUID
func isUUID(s string) bool {
	// Simple check - actual implementation would use regex or uuid library
	return len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-'
}

// getResourceMappings retrieves reference field mappings for a resource type
func (r *ReferenceResolver) getResourceMappings(resourceType string) map[string]string {
	// Check cache first
	r.cacheMutex.RLock()
	if mappings, exists := r.mappingCache[resourceType]; exists {
		r.cacheMutex.RUnlock()
		return mappings
	}
	r.cacheMutex.RUnlock()

	// Create a resource instance to get its mappings
	mappings := r.createResourceAndGetMappings(resourceType)
	
	// Cache the result
	r.cacheMutex.Lock()
	r.mappingCache[resourceType] = mappings
	r.cacheMutex.Unlock()
	
	return mappings
}

// createResourceAndGetMappings creates a resource instance and retrieves its field mappings
func (r *ReferenceResolver) createResourceAndGetMappings(resourceType string) map[string]string {
	// Create an instance of the resource type to query its mappings
	var resource interface{}
	
	switch resourceType {
	case ResourceTypePortal:
		resource = resources.PortalResource{}
	case ResourceTypeAPI:
		resource = resources.APIResource{}
	case ResourceTypeAPIVersion:
		resource = resources.APIVersionResource{}
	case ResourceTypeAPIPublication:
		resource = resources.APIPublicationResource{}
	case ResourceTypeAPIImplementation:
		resource = resources.APIImplementationResource{}
	case ResourceTypeAPIDocument:
		resource = resources.APIDocumentResource{}
	case "application_auth_strategy":
		resource = resources.ApplicationAuthStrategyResource{}
	case "control_plane":
		resource = resources.ControlPlaneResource{}
	case "portal_customization":
		resource = resources.PortalCustomizationResource{}
	case "portal_custom_domain":
		resource = resources.PortalCustomDomainResource{}
	case "portal_page":
		resource = resources.PortalPageResource{}
	case "portal_snippet":
		resource = resources.PortalSnippetResource{}
	default:
		return make(map[string]string)
	}
	
	// Check if resource implements reference mappings interface
	if mapper, ok := resource.(interface{ GetReferenceFieldMappings() map[string]string }); ok {
		return mapper.GetReferenceFieldMappings()
	}
	
	return make(map[string]string)
}

// isReferenceFieldDynamic checks if a field is a reference field using resource mappings
func (r *ReferenceResolver) isReferenceFieldDynamic(resourceType, fieldName string) bool {
	mappings := r.getResourceMappings(resourceType)
	_, exists := mappings[fieldName]
	return exists
}

// getResourceTypeForFieldDynamic gets resource type using dynamic mappings
func (r *ReferenceResolver) getResourceTypeForFieldDynamic(resourceType, fieldName string) string {
	mappings := r.getResourceMappings(resourceType)
	if targetType, exists := mappings[fieldName]; exists {
		return targetType
	}
	return ""
}

// extractReferenceValue extracts reference value without checking field name
func (r *ReferenceResolver) extractReferenceValue(value interface{}) (string, bool) {
	// Extract string value
	switch v := value.(type) {
	case string:
		if !isUUID(v) && v != "" {
			return v, true
		}
	case FieldChange:
		if newVal, ok := v.New.(string); ok && !isUUID(newVal) && newVal != "" {
			return newVal, true
		}
	}
	return "", false
}