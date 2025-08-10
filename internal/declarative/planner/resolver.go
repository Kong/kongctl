package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/state"
)

// ReferenceResolver resolves declarative refs to Konnect IDs
type ReferenceResolver struct {
	client *state.Client
}

// NewReferenceResolver creates a new resolver
func NewReferenceResolver(client *state.Client) *ReferenceResolver {
	return &ReferenceResolver{
		client: client,
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

	// Build maps of what's being created in this plan
	// Global map for ref lookups (since refs are globally unique)
	createdResourcesByRef := make(map[string]string) // ref -> change_id
	// Map to track resource types (for validation)
	refToResourceType := make(map[string]string) // ref -> resource_type
	
	for _, change := range changes {
		if change.Action == ActionCreate {
			createdResourcesByRef[change.ResourceRef] = change.ID
			refToResourceType[change.ResourceRef] = change.ResourceType
		}
	}

	// Resolve references for each change
	for _, change := range changes {
		changeRefs := make(map[string]ResolvedReference)

		// Check fields that might contain references
		for fieldName, fieldValue := range change.Fields {
			if ref, isRef := r.extractReference(fieldName, fieldValue); isRef {
				// Determine expected resource type from field name
				expectedResourceType := r.getResourceTypeForField(fieldName)

				// Check if this references something being created in this plan
				if _, inPlan := createdResourcesByRef[ref]; inPlan {
					// Validate the resource type matches expectation
					actualResourceType := refToResourceType[ref]
					if actualResourceType != expectedResourceType {
						result.Errors = append(result.Errors, fmt.Errorf(
							"change %s: field %s expects %s resource, but ref %q is a %s resource",
							change.ID, fieldName, expectedResourceType, ref, actualResourceType))
						continue
					}
					changeRefs[fieldName] = ResolvedReference{
						Ref: ref,
						ID:  "[unknown]", // Will be resolved at execution
					}
				} else {
					// Resolve from existing resources
					// Since refs are globally unique, we can resolve by ref alone
					// But we still pass expectedResourceType for validation
					id, err := r.resolveReference(ctx, expectedResourceType, ref)
					if err != nil {
						result.Errors = append(result.Errors, fmt.Errorf(
							"change %s: failed to resolve %s reference %q: %w",
							change.ID, expectedResourceType, ref, err))
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