package executor

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
)

// Full-PUT mapped child adapters supply the observed mutable payload after a
// fresh read. Identity, namespace, protection and metadata capacity are checked
// here; adapters retain their SDK mapping and resource-specific config rules.
func prepareOwnedMappedChildUpdate(
	kind string, current, desired map[string]any, execCtx *ExecutionContext,
) (map[string]any, error) {
	metadata := childPayloadManagedBy(current)
	if err := resources.ValidateChildOwnership(kind, metadata, execCtx.Namespace); err != nil {
		return nil, err
	}
	if err := validateChildPlannedProtection(kind, metadata, execCtx.PlannedChange); err != nil {
		return nil, err
	}
	if current[planner.FieldType] != desired[planner.FieldType] ||
		current[planner.FieldName] != desired[planner.FieldName] {
		return nil, fmt.Errorf("%s name and type are immutable", kind)
	}
	if _, present := desired[planner.FieldLabels]; !present {
		if labels, explicit := execCtx.PlannedChange.Fields[planner.FieldLabels]; explicit {
			desired[planner.FieldLabels] = labels
		} else {
			desired[planner.FieldLabels] = current[planner.FieldLabels]
		}
	}
	requested := childPayloadManagedBy(desired)
	if metadata[resources.ChildProtectedKey] == "true" {
		before, err := ownedChildPayload(current)
		if err != nil {
			return nil, err
		}
		after, err := ownedChildPayload(desired)
		if err != nil {
			return nil, err
		}
		delete(before, planner.FieldManagedBy)
		delete(after, planner.FieldManagedBy)
		if !childUnprotectOnly(execCtx.PlannedChange, metadata, requested) || !reflect.DeepEqual(before, after) {
			return nil, fmt.Errorf("%s is protected; unprotect it in a separate update", kind)
		}
	}
	merged := maps.Clone(metadata)
	if merged == nil {
		merged = make(map[string]string)
	}
	maps.Copy(merged, requested)
	if err := resources.ValidateChildManagedBy(merged, false); err != nil {
		return nil, err
	}
	desired[planner.FieldManagedBy] = merged
	return desired, nil
}

func ownedChildPayload(request any) (map[string]any, error) {
	encoded, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("cannot encode AI Gateway child payload")
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return nil, fmt.Errorf("cannot decode AI Gateway child payload")
	}
	return payload, nil
}

func childPayloadManagedBy(payload map[string]any) map[string]string {
	if values, ok := payload[planner.FieldManagedBy].(map[string]string); ok {
		return maps.Clone(values)
	}
	metadata := make(map[string]string)
	if values, ok := payload[planner.FieldManagedBy].(map[string]any); ok {
		for key, value := range values {
			if text, ok := value.(string); ok {
				metadata[key] = text
			}
		}
	}
	return metadata
}

func validateChildWriteContext(kind string, execCtx *ExecutionContext, payload map[string]any, namespace string) error {
	if execCtx == nil || execCtx.PlannedChange == nil || namespace != execCtx.Namespace {
		return fmt.Errorf("%s execution namespace mismatch", kind)
	}
	return validateChildPayloadOwnership(kind, childPayloadManagedBy(payload), namespace)
}
