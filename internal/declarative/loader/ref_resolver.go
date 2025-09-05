package loader

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/log"
)

// FieldResolver interface for field resolution
type FieldResolver interface {
	// ResolveField extracts a field value from a resource
	ResolveField(resource resources.Resource, field string) (string, error)

	// CanResolve checks if this resolver can handle the given resource type
	CanResolve(resourceType string) bool
}

// LocalFieldResolver resolves fields from local ResourceSet
type LocalFieldResolver struct {
	logger *slog.Logger
}

// NewLocalFieldResolver creates a new local field resolver
func NewLocalFieldResolver(logger *slog.Logger) *LocalFieldResolver {
	if logger == nil {
		logger = slog.Default()
	}
	return &LocalFieldResolver{logger: logger}
}

// ResolveField extracts field value using reflection
func (r *LocalFieldResolver) ResolveField(resource resources.Resource, field string) (string, error) {
	r.logger.LogAttrs(context.Background(), log.LevelTrace, "Resolving field from resource",
		slog.String("resource_ref", resource.GetRef()),
		slog.String("resource_type", string(resource.GetType())),
		slog.String("field", field),
	)

	if field == "" {
		return "", fmt.Errorf("field name cannot be empty")
	}

	// Support dot notation for nested fields
	parts := strings.Split(field, ".")
	current := reflect.ValueOf(resource)

	for i, part := range parts {
		r.logger.LogAttrs(context.Background(), log.LevelTrace, "Walking field path",
			slog.String("part", part),
			slog.Int("index", i),
			slog.String("current_type", current.Type().String()),
		)

		// Dereference pointers
		for current.Kind() == reflect.Ptr && !current.IsNil() {
			current = current.Elem()
		}

		// Handle struct fields
		if current.Kind() == reflect.Struct {
			fieldVal := findFieldByJSONTag(current, part)
			if !fieldVal.IsValid() {
				fieldVal = current.FieldByName(part)
			}

			if !fieldVal.IsValid() {
				r.logger.LogAttrs(context.Background(), slog.LevelDebug, "Field not found",
					slog.String("resource_ref", resource.GetRef()),
					slog.String("field_path", strings.Join(parts[:i+1], ".")),
					slog.String("struct_type", current.Type().String()),
				)
				return "", fmt.Errorf("field %s not found in %s", part, current.Type())
			}
			current = fieldVal
		} else {
			r.logger.LogAttrs(context.Background(), slog.LevelWarn, "Cannot navigate field on non-struct",
				slog.String("resource_ref", resource.GetRef()),
				slog.String("field_path", strings.Join(parts[:i], ".")),
				slog.String("kind", current.Kind().String()),
			)
			return "", fmt.Errorf("cannot access field %s on %s", part, current.Kind())
		}
	}

	// Convert to string
	result := convertToString(current)

	r.logger.LogAttrs(context.Background(), slog.LevelDebug, "Field resolved successfully",
		slog.String("resource_ref", resource.GetRef()),
		slog.String("field", field),
		slog.String("value", result),
	)

	return result, nil
}

// CanResolve checks if this resolver can handle the resource type
func (r *LocalFieldResolver) CanResolve(_ string) bool {
	// Local resolver handles all types in ResourceSet
	return true
}

// ResolveReferences resolves all ref placeholders in the ResourceSet
func ResolveReferences(ctx context.Context, rs *resources.ResourceSet) error {
	// Extract logger from context or use default
	var logger *slog.Logger
	if loggerVal := ctx.Value(log.LoggerKey); loggerVal != nil {
		logger = loggerVal.(*slog.Logger)
	} else {
		logger = slog.Default()
	}

	logger.LogAttrs(ctx, slog.LevelInfo, "Starting reference resolution",
		slog.Int("portals", len(rs.Portals)),
		slog.Int("apis", len(rs.APIs)),
		slog.Int("auth_strategies", len(rs.ApplicationAuthStrategies)),
	)

	resolver := NewLocalFieldResolver(logger)
	resolutionPath := make([]string, 0)

	// Process all resource types
	processCount := 0

	// Process Portals
	for i := range rs.Portals {
		if err := resolveResourceFields(ctx, &rs.Portals[i], rs, resolver, resolutionPath, logger); err != nil {
			logger.LogAttrs(ctx, slog.LevelError, "Failed to resolve portal references",
				slog.String("portal_ref", rs.Portals[i].GetRef()),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("resolving portal %s: %w", rs.Portals[i].GetRef(), err)
		}
		processCount++
	}

	// Process APIs
	for i := range rs.APIs {
		if err := resolveResourceFields(ctx, &rs.APIs[i], rs, resolver, resolutionPath, logger); err != nil {
			logger.LogAttrs(ctx, slog.LevelError, "Failed to resolve API references",
				slog.String("api_ref", rs.APIs[i].GetRef()),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("resolving API %s: %w", rs.APIs[i].GetRef(), err)
		}
		processCount++
	}

	// Process ApplicationAuthStrategies
	for i := range rs.ApplicationAuthStrategies {
		if err := resolveResourceFields(ctx, &rs.ApplicationAuthStrategies[i], rs,
			resolver, resolutionPath, logger); err != nil {
			logger.LogAttrs(ctx, slog.LevelError, "Failed to resolve auth strategy references",
				slog.String("auth_ref", rs.ApplicationAuthStrategies[i].GetRef()),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("resolving auth strategy %s: %w", rs.ApplicationAuthStrategies[i].GetRef(), err)
		}
		processCount++
	}

	// Process child resources (PortalPages, PortalSnippets, etc.)
	for i := range rs.PortalPages {
		if err := resolveResourceFields(ctx, &rs.PortalPages[i], rs, resolver, resolutionPath, logger); err != nil {
			return fmt.Errorf("resolving portal page %s: %w", rs.PortalPages[i].GetRef(), err)
		}
		processCount++
	}

	for i := range rs.PortalSnippets {
		if err := resolveResourceFields(ctx, &rs.PortalSnippets[i], rs, resolver, resolutionPath, logger); err != nil {
			return fmt.Errorf("resolving portal snippet %s: %w", rs.PortalSnippets[i].GetRef(), err)
		}
		processCount++
	}

	for i := range rs.PortalCustomizations {
		if err := resolveResourceFields(ctx, &rs.PortalCustomizations[i], rs, resolver, resolutionPath, logger); err != nil {
			return fmt.Errorf("resolving portal customization %s: %w", rs.PortalCustomizations[i].GetRef(), err)
		}
		processCount++
	}

	for i := range rs.PortalCustomDomains {
		if err := resolveResourceFields(ctx, &rs.PortalCustomDomains[i], rs, resolver, resolutionPath, logger); err != nil {
			return fmt.Errorf("resolving portal custom domain %s: %w", rs.PortalCustomDomains[i].GetRef(), err)
		}
		processCount++
	}

	logger.LogAttrs(ctx, slog.LevelInfo, "Reference resolution completed",
		slog.Int("resources_processed", processCount),
	)

	return nil
}

// resolveResourceFields walks a resource struct and resolves placeholder strings
func resolveResourceFields(ctx context.Context, resource interface{}, rs *resources.ResourceSet,
	resolver FieldResolver, resolutionPath []string, logger *slog.Logger,
) error {
	val := reflect.ValueOf(resource)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	// Get resource ref for logging
	resourceRef := ""
	// Try to get ref via Resource interface first
	if resourceIface, ok := resource.(resources.Resource); ok {
		resourceRef = resourceIface.GetRef()
	} else {
		// Fallback to struct field access
		refField := val.FieldByName("Ref")
		if refField.IsValid() && refField.Kind() == reflect.String {
			resourceRef = refField.String()
		}
	}

	logger.LogAttrs(ctx, log.LevelTrace, "Processing resource for references",
		slog.String("resource_ref", resourceRef),
		slog.String("type", val.Type().String()),
	)

	return walkAndResolve(ctx, val, rs, resolver, resolutionPath, resourceRef, logger)
}

// walkAndResolve recursively walks struct fields and resolves placeholders
func walkAndResolve(ctx context.Context, val reflect.Value, rs *resources.ResourceSet,
	resolver FieldResolver, resolutionPath []string, currentResourceRef string, logger *slog.Logger,
) error {
	// Dereference pointers
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.String:
		str := val.String()
		if tags.IsRefPlaceholder(str) {
			refStr, field, ok := tags.ParseRefPlaceholder(str)
			if !ok {
				logger.LogAttrs(ctx, slog.LevelWarn, "Invalid placeholder format",
					slog.String("placeholder", str),
					slog.String("resource", currentResourceRef),
				)
				return fmt.Errorf("invalid placeholder: %s", str)
			}

			logger.LogAttrs(ctx, slog.LevelDebug, "Found reference placeholder",
				slog.String("placeholder", str),
				slog.String("target_ref", refStr),
				slog.String("target_field", field),
				slog.String("source_resource", currentResourceRef),
			)

			// Check for circular dependency
			pathKey := fmt.Sprintf("%s->%s#%s", currentResourceRef, refStr, field)
			for _, p := range resolutionPath {
				if p == pathKey {
					logger.LogAttrs(ctx, slog.LevelError, "Circular reference detected",
						slog.String("path", strings.Join(append(resolutionPath, pathKey), " -> ")),
					)
					return fmt.Errorf("circular reference: %s", strings.Join(append(resolutionPath, pathKey), " -> "))
				}
			}

			// Resolve the reference
			target, exists := rs.GetResourceByRef(refStr)
			if !exists {
				logger.LogAttrs(ctx, slog.LevelWarn, "Referenced resource not found",
					slog.String("ref", refStr),
					slog.String("source_resource", currentResourceRef),
				)
				return fmt.Errorf("resource not found: %s", refStr)
			}

			logger.LogAttrs(ctx, log.LevelTrace, "Found target resource",
				slog.String("target_ref", refStr),
				slog.String("target_type", string(target.GetType())),
				slog.String("requesting_field", field),
			)

			// Extract field value
			value, err := resolver.ResolveField(target, field)
			if err != nil {
				logger.LogAttrs(ctx, slog.LevelError, "Failed to extract field",
					slog.String("resource_ref", refStr),
					slog.String("field", field),
					slog.String("error", err.Error()),
				)
				return fmt.Errorf("extracting field %s from %s: %w", field, refStr, err)
			}

			// Set the resolved value
			if val.CanSet() {
				val.SetString(value)
				logger.LogAttrs(ctx, slog.LevelDebug, "Reference resolved",
					slog.String("source_resource", currentResourceRef),
					slog.String("target_ref", refStr),
					slog.String("field", field),
					slog.String("resolved_value", value),
				)
			}
		}

	case reflect.Struct:
		for i := 0; i < val.NumField(); i++ {
			fieldVal := val.Field(i)
			if fieldVal.CanSet() {
				if err := walkAndResolve(ctx, fieldVal, rs, resolver, resolutionPath, currentResourceRef, logger); err != nil {
					return err
				}
			}
		}

	case reflect.Slice, reflect.Array:
		for i := 0; i < val.Len(); i++ {
			if err := walkAndResolve(ctx, val.Index(i), rs, resolver, resolutionPath, currentResourceRef, logger); err != nil {
				return err
			}
		}

	case reflect.Map:
		for _, key := range val.MapKeys() {
			// Maps typically contain interface{} values which we need to handle specially
			_ = val.MapIndex(key)
			// For now, skip map resolution as it requires special handling
			// This can be enhanced in Phase 2
		}
	case reflect.Ptr:
		// Handle pointer types by dereferencing and processing
		if !val.IsNil() {
			if err := walkAndResolve(ctx, val.Elem(), rs, resolver, resolutionPath, currentResourceRef, logger); err != nil {
				return err
			}
		}
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128,
		reflect.Chan, reflect.Func, reflect.Interface, reflect.UnsafePointer:
		// For primitives and other types, no action needed
		// They cannot contain reference placeholders
	case reflect.Invalid:
		// Handle invalid reflect values
		// No action needed
	default:
		// This should never be reached as we've covered all reflect.Kind values
	}

	return nil
}

// findFieldByJSONTag finds a struct field by its JSON tag
func findFieldByJSONTag(val reflect.Value, jsonTag string) reflect.Value {
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
func convertToString(val reflect.Value) string {
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
