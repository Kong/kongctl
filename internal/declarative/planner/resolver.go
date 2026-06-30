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

type extractedReference struct {
	Field string
	Ref   string
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
		for _, fieldRef := range r.extractReferencesFromFields(change.Fields) {
			// Determine resource type from field name and role entity metadata.
			if aiGatewayPolicyReferenceField(change.ResourceType, fieldRef.Field) {
				fieldRef.Ref = aiGatewayPolicyNameReference(fieldRef.Ref)
			}
			resourceType := r.getResourceTypeForChangeField(change, fieldRef.Field)
			targetRef := referenceTargetRef(fieldRef.Ref)

			// Check if this references something being created
			if referenceCreatedInPlan(createdResources, resourceType, targetRef) {
				changeRefs[fieldRef.Field] = ResolvedReference{
					Ref: fieldRef.Ref,
					ID:  resources.UnknownReferenceID, // Will be resolved at execution
				}
			} else {
				// Resolve from existing resources
				id, err := r.resolveReference(ctx, resourceType, fieldRef.Ref)
				if err != nil {
					result.Errors = append(result.Errors, fmt.Errorf(
						"change %s: failed to resolve %s reference %q: %w",
						change.ID, resourceType, fieldRef.Ref, err,
					))
					continue
				}
				changeRefs[fieldRef.Field] = ResolvedReference{
					Ref: fieldRef.Ref,
					ID:  id,
				}
			}
		}

		if len(changeRefs) > 0 {
			result.ChangeReferences[change.ID] = changeRefs
		}
	}

	return result, nil
}

func referenceCreatedInPlan(createdResources map[string]map[string]string, resourceType, targetRef string) bool {
	if resourceType == "" || targetRef == "" {
		return false
	}
	_, ok := createdResources[resourceType][targetRef]
	return ok
}

func (r *ReferenceResolver) extractReferencesFromFields(fields map[string]any) []extractedReference {
	references := []extractedReference{}
	for fieldName, fieldValue := range fields {
		references = append(references, r.extractReferences(fieldName, fieldValue)...)
	}
	return references
}

func (r *ReferenceResolver) extractReferences(fieldName string, value any) []extractedReference {
	if ref, isRef := r.extractReference(fieldName, value); isRef {
		return []extractedReference{{Field: fieldName, Ref: ref}}
	}

	switch v := value.(type) {
	case FieldChange:
		return r.extractReferences(fieldName, v.New)
	case map[string]any:
		references := []extractedReference{}
		for childField, childValue := range v {
			references = append(references, r.extractReferences(fieldName+"."+childField, childValue)...)
		}
		return references
	case []any:
		references := []extractedReference{}
		for i, childValue := range v {
			references = append(references, r.extractReferences(fmt.Sprintf("%s.%d", fieldName, i), childValue)...)
		}
		return references
	default:
		return nil
	}
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
		FieldAuditLogDestinationID,
		FieldDCRProviderID,
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
	case FieldDefaultApplicationStrategyID, FieldAuthStrategyIDs:
		return ResourceTypeApplicationAuthStrategy
	case FieldAuditLogDestinationID:
		return ResourceTypeAuditLogWebhookDestination
	case FieldDCRProviderID:
		return ResourceTypeDCRProvider
	case "control_plane_id", "gateway_service.control_plane_id", "service.control_plane_id":
		return ResourceTypeControlPlane
	case FieldPortalID:
		return ResourceTypePortal
	case FieldEntityID:
		return ResourceTypeAPI
	default:
		if strings.HasSuffix(fieldName, ".schema_registry.id") {
			return ResourceTypeEventGatewaySchemaRegistry
		}
		if strings.HasSuffix(fieldName, ".encryption_key.key.id") {
			return ResourceTypeEventGatewayStaticKey
		}
		return ""
	}
}

func (r *ReferenceResolver) getResourceTypeForChangeField(change PlannedChange, fieldName string) string {
	if fieldName == FieldEntityID {
		entityTypeName, _ := change.Fields[FieldEntityTypeName].(string)
		if resourceType, ok := resources.RoleEntityResourceType(entityTypeName); ok {
			return string(resourceType)
		}
	}
	if aiGatewayPolicyReferenceField(change.ResourceType, fieldName) {
		return ResourceTypeAIGatewayPolicy
	}
	return r.getResourceTypeForField(fieldName)
}

func aiGatewayPolicyReferenceField(resourceType, fieldName string) bool {
	switch resourceType {
	case ResourceTypeAIGatewayAgent,
		ResourceTypeAIGatewayConsumer,
		ResourceTypeAIGatewayConsumerGroup,
		ResourceTypeAIGatewayModel,
		ResourceTypeAIGatewayMCPServer:
	default:
		return false
	}
	return fieldName == FieldPolicies || strings.HasPrefix(fieldName, FieldPolicies+".")
}

func aiGatewayPolicyNameReference(ref string) string {
	targetRef := ref
	if parsedRef, _, ok := tags.ParseRefPlaceholder(ref); ok {
		targetRef = parsedRef
	}
	if targetRef == "" {
		return ref
	}
	return fmt.Sprintf("%s%s#%s", tags.RefPlaceholderPrefix, targetRef, FieldName)
}

func referenceTargetRef(ref string) string {
	targetRef, _, ok := tags.ParseRefPlaceholder(ref)
	if ok {
		return targetRef
	}
	return ref
}

// resolveReference looks up a reference in existing resources
func (r *ReferenceResolver) resolveReference(ctx context.Context, resourceType, ref string) (string, error) {
	var targetRef string
	fieldName := FieldID // Default field

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
		if resource, exists := r.getResourceByTypeAndRef(resourceType, targetRef); exists {
			// Special handling for "id" field - return konnectID
			if fieldName == FieldID || fieldName == "ID" {
				konnectID := resource.GetKonnectID()
				if konnectID == "" {
					if id, resolved, err := r.resolveEventGatewayChildResource(ctx, resource); err != nil {
						return "", err
					} else if resolved {
						return id, nil
					}
					// Resource exists but no Konnect ID (will be created)
					return resources.UnknownReferenceID, nil // Trigger forward reference
				}
				return konnectID, nil
			}
			if fieldName == FieldName {
				if name := resource.GetMoniker(); name != "" {
					return name, nil
				}
			}

			// For other fields, use reflection to extract value
			return r.extractFieldFromResource(resource, fieldName)
		}
	}

	// Fallback to original resolution for backward compatibility
	switch resourceType {
	case ResourceTypeApplicationAuthStrategy:
		return r.resolveAuthStrategyRef(ctx, targetRef)
	case ResourceTypeDCRProvider:
		return r.resolveDCRProviderRef(ctx, targetRef)
	case ResourceTypeControlPlane:
		return r.resolveControlPlaneRef(ctx, targetRef)
	case ResourceTypePortal:
		return r.resolvePortalRef(ctx, targetRef)
	default:
		return "", fmt.Errorf("unknown resource type: %s", resourceType)
	}
}

func (r *ReferenceResolver) getResourceByTypeAndRef(resourceType string, ref string) (resources.Resource, bool) {
	if r.resources == nil || resourceType == "" {
		return nil, false
	}
	for _, resource := range r.resources.AllResourcesByType(resources.ResourceType(resourceType)) {
		if resource.GetRef() == ref {
			return resource, true
		}
	}
	if resource, exists := r.getEventGatewayChildResourceByTypeAndRef(resourceType, ref); exists {
		return resource, true
	}
	if resource, exists := r.resources.GetResourceByRef(ref); exists && string(resource.GetType()) == resourceType {
		return resource, true
	}
	return nil, false
}

func (r *ReferenceResolver) getEventGatewayChildResourceByTypeAndRef(
	resourceType string,
	ref string,
) (resources.Resource, bool) {
	for _, gateway := range r.resources.EventGatewayControlPlanes {
		switch resourceType {
		case ResourceTypeEventGatewaySchemaRegistry:
			for _, registry := range gateway.SchemaRegistries {
				if registry.Ref == ref {
					registryCopy := registry
					registryCopy.EventGateway = gateway.Ref
					return &registryCopy, true
				}
			}
		case ResourceTypeEventGatewayStaticKey:
			for _, staticKey := range gateway.StaticKeys {
				if staticKey.Ref == ref {
					staticKeyCopy := staticKey
					staticKeyCopy.EventGateway = gateway.Ref
					return &staticKeyCopy, true
				}
			}
		}
	}
	return nil, false
}

func (r *ReferenceResolver) resolveEventGatewayChildResource(
	ctx context.Context,
	resource resources.Resource,
) (string, bool, error) {
	switch typed := resource.(type) {
	case *resources.EventGatewaySchemaRegistryResource:
		if r.client == nil || typed == nil {
			return "", false, nil
		}
		gatewayID, err := r.resolveEventGatewayParentRef(ctx, typed.EventGateway)
		if err != nil {
			return "", true, err
		}
		registry, err := r.client.GetEventGatewaySchemaRegistryByName(ctx, gatewayID, typed.GetMoniker())
		if err != nil {
			return "", true, fmt.Errorf("failed to resolve event gateway schema registry ref %q: %w", typed.Ref, err)
		}
		if registry == nil {
			return "", true, fmt.Errorf("event gateway schema registry not found: ref=%s", typed.Ref)
		}
		return registry.ID, true, nil
	case *resources.EventGatewayStaticKeyResource:
		if r.client == nil || typed == nil {
			return "", false, nil
		}
		gatewayID, err := r.resolveEventGatewayParentRef(ctx, typed.EventGateway)
		if err != nil {
			return "", true, err
		}
		keys, err := r.client.ListEventGatewayStaticKeys(ctx, gatewayID)
		if err != nil {
			return "", true, fmt.Errorf("failed to resolve event gateway static key ref %q: %w", typed.Ref, err)
		}
		for _, key := range keys {
			if key.Name == typed.GetMoniker() {
				return key.ID, true, nil
			}
		}
		return "", true, fmt.Errorf("event gateway static key not found: ref=%s", typed.Ref)
	default:
		return "", false, nil
	}
}

func (r *ReferenceResolver) resolveEventGatewayParentRef(ctx context.Context, eventGatewayRef string) (string, error) {
	if eventGatewayRef == "" {
		return "", fmt.Errorf("event gateway parent reference is required")
	}
	if util.IsValidUUID(eventGatewayRef) {
		return eventGatewayRef, nil
	}
	if resource, exists := r.getResourceByTypeAndRef(ResourceTypeEventGatewayControlPlane, eventGatewayRef); exists {
		if id := resource.GetKonnectID(); id != "" {
			return id, nil
		}
		if moniker := resource.GetMoniker(); moniker != "" {
			return r.resolveControlPlaneRef(ctx, moniker)
		}
	}
	return r.resolveControlPlaneRef(ctx, eventGatewayRef)
}

func (r *ReferenceResolver) resolveDCRProviderRef(ctx context.Context, ref string) (string, error) {
	provider, err := r.client.GetDCRProviderByName(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("failed to resolve DCR provider ref '%s': %w", ref, err)
	}
	if provider == nil {
		return "", fmt.Errorf("dcr provider with ref '%s' not found", ref)
	}
	return provider.ID, nil
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
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	// Support dot notation for nested fields
	parts := strings.Split(fieldName, ".")
	current := v

	for _, part := range parts {
		// Dereference pointers
		for current.Kind() == reflect.Pointer && !current.IsNil() {
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
	for val.Kind() == reflect.Pointer && !val.IsNil() {
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
	case reflect.Array, reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer,
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
