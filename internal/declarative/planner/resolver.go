package planner

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/util"
)

// ReferenceResolver resolves declarative refs to Konnect IDs
type ReferenceResolver struct {
	client    *state.Client
	resources *resources.ResourceSet
}

// NewReferenceResolver creates a new resolver
func NewReferenceResolver(client *state.Client, rs *resources.ResourceSet) *ReferenceResolver {
	return &ReferenceResolver{
		client:    client,
		resources: rs,
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
			if ref, isRef := r.extractReference(fieldName, fieldValue); isRef {
				// Determine resource type from field name
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
func (r *ReferenceResolver) extractReference(fieldName string, value any) (string, bool) {
	// Check for __REF__ placeholder format
	if str, ok := value.(string); ok && strings.HasPrefix(str, "__REF__:") {
		return str, true // Return full placeholder for resolution
	}

	// Check if field name suggests a reference
	if !r.isReferenceField(fieldName) {
		return "", false
	}

	// Extract string value
	switch v := value.(type) {
	case string:
		if !util.IsValidUUID(v) {
			return v, true
		}
	case FieldChange:
		if newVal, ok := v.New.(string); ok && !util.IsValidUUID(newVal) {
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
		"entity_id",
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
	case "entity_id":
		return "api"
	default:
		return ""
	}
}

// resolveReference looks up a reference in existing resources
func (r *ReferenceResolver) resolveReference(ctx context.Context, resourceType, ref string) (string, error) {
	var targetRef string
	fieldName := "id" // Default field

	// Parse __REF__ placeholder format
	if strings.HasPrefix(ref, "__REF__:") {
		parsedRef, field, ok := tags.ParseRefPlaceholder(ref)
		if !ok {
			return "", fmt.Errorf("invalid reference format: %s", ref)
		}
		targetRef = parsedRef
		fieldName = field
	} else {
		// Traditional ref (backward compatibility)
		targetRef = ref
	}

	// Find resource in ResourceSet by ref
	if r.resources != nil {
		resource, exists := r.resources.GetResourceByRef(targetRef)
		if exists {
			// Special handling for "id" field - return konnectID
			if fieldName == "id" || fieldName == "ID" {
				konnectID := resource.GetKonnectID()
				if konnectID == "" {
					// Resource exists but no Konnect ID (will be created)
					return "[unknown]", nil // Trigger forward reference
				}
				return konnectID, nil
			}

			// For other fields, use reflection to extract value
			return r.extractFieldFromResource(resource, fieldName)
		}
	}

	// Fallback to original resolution for backward compatibility
	switch resourceType {
	case "application_auth_strategy":
		return r.resolveAuthStrategyRef(ctx, targetRef)
	case "control_plane":
		return r.resolveControlPlaneRef(ctx, targetRef)
	case ResourceTypePortal:
		return r.resolvePortalRef(ctx, targetRef)
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
func (r *ReferenceResolver) resolveControlPlaneRef(ctx context.Context, ref string) (string, error) {
	cp, err := r.client.GetControlPlaneByName(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("failed to resolve control plane ref '%s': %w", ref, err)
	}
	if cp == nil {
		return "", fmt.Errorf("control plane with ref '%s' not found", ref)
	}
	return cp.ID, nil
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

// extractFieldFromResource extracts a field value from a resource using reflection
func (r *ReferenceResolver) extractFieldFromResource(resource resources.Resource, fieldName string) (string, error) {
	v := reflect.ValueOf(resource)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Support dot notation for nested fields
	parts := strings.Split(fieldName, ".")
	current := v

	for _, part := range parts {
		// Dereference pointers
		for current.Kind() == reflect.Ptr && !current.IsNil() {
			current = current.Elem()
		}

		// Handle struct fields
		if current.Kind() == reflect.Struct {
			// First try to find by JSON tag
			fieldVal := r.findFieldByJSONTag(current, part)
			if !fieldVal.IsValid() {
				// Fallback to field name
				fieldVal = current.FieldByName(part)
			}

			if !fieldVal.IsValid() {
				return "", fmt.Errorf("field %s not found in %s", part, current.Type())
			}
			current = fieldVal
		} else {
			return "", fmt.Errorf("cannot access field %s on %s", part, current.Kind())
		}
	}

	// Convert to string
	return r.convertToString(current), nil
}

// findFieldByJSONTag finds a struct field by its JSON tag
func (r *ReferenceResolver) findFieldByJSONTag(val reflect.Value, jsonTag string) reflect.Value {
	t := val.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		if idx := strings.Index(tag, ","); idx != -1 {
			tag = tag[:idx]
		}
		if tag == jsonTag {
			return val.Field(i)
		}
	}
	return reflect.Value{}
}

// convertToString converts a reflect.Value to string
func (r *ReferenceResolver) convertToString(val reflect.Value) string {
	// Dereference pointers
	for val.Kind() == reflect.Ptr && !val.IsNil() {
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.String:
		return val.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", val.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return fmt.Sprintf("%d", val.Uint())
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%f", val.Float())
	case reflect.Bool:
		return fmt.Sprintf("%t", val.Bool())
	case reflect.Complex64, reflect.Complex128:
		return fmt.Sprintf("%v", val.Complex())
	case reflect.Array, reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr,
		reflect.Slice, reflect.Struct, reflect.UnsafePointer:
		// For composite types, use general interface conversion
		return fmt.Sprintf("%v", val.Interface())
	case reflect.Invalid:
		// Handle invalid reflect values
		return "<invalid>"
	default:
		// This should never be reached as we've covered all reflect.Kind values
		return fmt.Sprintf("%v", val.Interface())
	}
}
