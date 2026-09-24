package loader

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/declarative/values"
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
	r.logger.LogAttrs(
		context.Background(), log.LevelTrace, "Resolving field from resource",
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
		r.logger.LogAttrs(
			context.Background(), log.LevelTrace, "Walking field path",
			slog.String("part", part),
			slog.Int("index", i),
			slog.String("current_type", current.Type().String()),
		)

		// Dereference pointers
		for current.Kind() == reflect.Pointer && !current.IsNil() {
			current = current.Elem()
		}

		// Handle struct fields
		if current.Kind() == reflect.Struct {
			fieldVal := findFieldByJSONTag(current, part)
			if !fieldVal.IsValid() {
				fieldVal = current.FieldByName(part)
			}

			if !fieldVal.IsValid() {
				r.logger.LogAttrs(
					context.Background(), slog.LevelDebug, "Field not found",
					slog.String("resource_ref", resource.GetRef()),
					slog.String("field_path", strings.Join(parts[:i+1], ".")),
					slog.String("struct_type", current.Type().String()),
				)
				return "", fmt.Errorf("field %s not found in %s", part, current.Type())
			}
			current = fieldVal
		} else {
			r.logger.LogAttrs(
				context.Background(), slog.LevelWarn, "Cannot navigate field on non-struct",
				slog.String("resource_ref", resource.GetRef()),
				slog.String("field_path", strings.Join(parts[:i], ".")),
				slog.String("kind", current.Kind().String()),
			)
			return "", fmt.Errorf("cannot access field %s on %s", part, current.Kind())
		}
	}

	// Convert to string
	result := convertToString(current)

	r.logger.LogAttrs(
		context.Background(), slog.LevelDebug, "Field resolved successfully",
		slog.String("resource_ref", resource.GetRef()),
		slog.String("field", field),
	)

	return result, nil
}

// CanResolve checks if this resolver can handle the resource type
func (r *LocalFieldResolver) CanResolve(_ string) bool {
	// Local resolver handles all types in ResourceSet
	return true
}

// ResolveReferences resolves explicit scalar references throughout registered declarations.
func ResolveReferences(ctx context.Context, rs *resources.ResourceSet) error {
	logger := slog.Default()
	if value, ok := ctx.Value(log.LoggerKey).(*slog.Logger); ok {
		logger = value
	}
	logger.Debug("Starting reference resolution")
	for _, resource := range rs.AllResources() {
		err := values.Transform(resource, func(path string, value string) (any, error) {
			if !tags.IsRefPlaceholder(value) ||
				rs.GetEnvSources(resource.GetRef())[path] != "" || rs.LiteralSources[resource.GetRef()][path] != "" {
				return value, nil
			}
			if resources.IsRelationshipPath(resource, path) {
				// Preserve relationship-specific resolution, but validate the explicit
				// target and selector before deferring its value to the planner.
				_, err := rs.ResolvePayloadReference(value)
				return value, err
			}
			logger.Debug("Found reference placeholder", "resource_ref", resource.GetRef())
			if source := rs.PayloadReferenceLiteralSource(value); source != "" {
				rs.AddLiteralSource(resource.GetRef(), path, source)
			}
			if source := rs.PayloadReferenceEnvSource(value); source != "" {
				rs.AddEnvSource(resource.GetRef(), path, source)
			}
			resolved, err := rs.ResolvePayloadReference(value)
			if err == nil {
				logger.Debug("Reference resolved", "resource_ref", resource.GetRef())
			}
			return resolved, err
		})
		if err != nil {
			return fmt.Errorf("resolving %s %q: %w", resource.GetType(), resource.GetRef(), err)
		}
	}
	logger.Debug("Reference resolution completed")
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
