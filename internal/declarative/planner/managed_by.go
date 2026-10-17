package planner

import (
	"context"
	"fmt"
	"maps"
	"reflect"

	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/declarative/resources"
)

func stampPlanManagedBy(ctx context.Context, plan *Plan) error {
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.Action == ActionDelete || !resources.SupportsManagedBy(resources.ResourceType(change.ResourceType)) {
			continue
		}
		metadata := make(map[string]string)
		switch raw := change.Fields[FieldManagedBy].(type) {
		case map[string]string:
			maps.Copy(metadata, raw)
		case map[string]any:
			for key, value := range raw {
				text, ok := value.(string)
				if !ok {
					return fmt.Errorf("%s %q managed_by must contain string values", change.ResourceType, change.ResourceRef)
				}
				metadata[key] = text
			}
		case nil:
		default:
			return fmt.Errorf("%s %q managed_by must be an object", change.ResourceType, change.ResourceRef)
		}
		prepared, err := resources.PrepareManagedBy(resources.ResourceType(change.ResourceType), metadata, build.Version(ctx))
		if err != nil {
			return fmt.Errorf("%s %q: %w", change.ResourceType, change.ResourceRef, err)
		}
		metadata = prepared
		var stamped any = metadata
		if _, generic := change.Fields[FieldManagedBy].(map[string]any); generic {
			values := make(map[string]any, len(metadata))
			for key, value := range metadata {
				values[key] = value
			}
			stamped = values
		}
		change.Fields[FieldManagedBy] = stamped
		if field, ok := change.ChangedFields[FieldManagedBy]; ok {
			field.New = stamped
			change.ChangedFields[FieldManagedBy] = field
		}
	}
	return nil
}

// Provenance describes the last write. Changing binaries alone must not cause
// an update, and generated entries must not look like user metadata removal.
func managedByValuesEqual(current, desired any) bool {
	comparableMetadata := func(value any) map[string]any {
		result := make(map[string]any)
		switch metadata := value.(type) {
		case map[string]string:
			for key, value := range metadata {
				result[key] = value
			}
		case map[string]any:
			maps.Copy(result, metadata)
		}
		delete(result, resources.ManagedByToolKey)
		delete(result, resources.ManagedByVersionKey)
		return result
	}
	return reflect.DeepEqual(comparableMetadata(current), comparableMetadata(desired))
}
