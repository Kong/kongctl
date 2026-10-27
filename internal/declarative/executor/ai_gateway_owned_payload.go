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
// Names are immutable; adapters specify any additional immutable scalar fields.
func prepareOwnedMappedChildUpdate(
	kind string, current, desired map[string]any, execCtx *ExecutionContext, immutableFields ...string,
) (map[string]any, error) {
	metadata := childPayloadManagedBy(current)
	if err := resources.ValidateChildOwnership(kind, metadata, execCtx.Namespace); err != nil {
		return nil, err
	}
	if err := validateChildPlannedProtection(kind, metadata, execCtx.PlannedChange); err != nil {
		return nil, err
	}
	if current[planner.FieldName] != desired[planner.FieldName] {
		return nil, fmt.Errorf("%s name is immutable", kind)
	}
	for _, field := range immutableFields {
		if current[field] != desired[field] {
			return nil, fmt.Errorf("%s %s is immutable", kind, field)
		}
	}
	if _, present := desired[planner.FieldLabels]; !present {
		if labels, explicit := execCtx.PlannedChange.Fields[planner.FieldLabels]; explicit {
			desired[planner.FieldLabels] = labels
		} else if labels, present := current[planner.FieldLabels]; present {
			desired[planner.FieldLabels] = labels
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

// Keep serialization errors sanitized: custom marshalers can include resolved
// credentials in their error text. Do not wrap the underlying error here.
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

// A metadata-only full PUT retains fresh public API defaults and configuration.
// Compare against the mapped saved fields first so injected payload or credentials
// cannot be disguised as unprotection. Resolved secret writes use the normal path.
func preserveOwnedMappedChildMetadataUpdate[Request any](kind string,
	current, requested map[string]any, execCtx *ExecutionContext, excludedFields ...string,
) (map[string]any, error) {
	change := execCtx.PlannedChange
	if len(change.ChangedFields) != 1 || change.ChangedFields[planner.FieldManagedBy].New == nil ||
		len(change.SecretWrites) != 0 {
		return requested, nil
	}
	if current[planner.FieldName] != requested[planner.FieldName] ||
		current[planner.FieldType] != requested[planner.FieldType] {
		return nil, fmt.Errorf("%s identity changed during metadata-only update; regenerate the plan", kind)
	}
	var planned Request
	plannedFields := maps.Clone(change.Fields)
	for _, field := range excludedFields {
		delete(plannedFields, field)
	}
	if err := mapAIGatewaySDKRequest(kind+" update", plannedFields, &planned); err != nil {
		return nil, err
	}
	expected, err := ownedChildPayload(planned)
	if err != nil {
		return nil, err
	}
	actual := maps.Clone(requested)
	delete(expected, planner.FieldManagedBy)
	delete(actual, planner.FieldManagedBy)
	if !reflect.DeepEqual(actual, expected) {
		return nil, fmt.Errorf("%s metadata-only payload differs from planned fields", kind)
	}
	preserved := maps.Clone(current)
	preserved[planner.FieldManagedBy] = requested[planner.FieldManagedBy]
	return preserved, nil
}
