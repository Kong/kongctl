package executor

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/tags"
)

func (e *Executor) preflightSecretWrites(plan *planner.Plan) error {
	e.clearResolvedSecrets()
	if plan == nil {
		return nil
	}
	resolved := make(map[string]map[string]string)
	for _, change := range plan.Changes {
		for _, intent := range change.SecretWrites {
			value, err := tags.ResolveSecretExpression(intent.Expression)
			if err != nil {
				return fmt.Errorf("failed to resolve secret source for %s %q field %s: %w",
					change.ResourceType, change.ResourceRef, intent.Field, err)
			}
			if resolved[change.ID] == nil {
				resolved[change.ID] = make(map[string]string)
			}
			resolved[change.ID][intent.Field] = value
		}
		cloned, err := cloneChangeForExecution(&change)
		if err != nil {
			return err
		}
		for _, intent := range change.SecretWrites {
			if err := setSecretField(cloned.Fields, decodeSecretPointer(intent.Field), "preflight"); err != nil {
				return fmt.Errorf("failed to validate secret field %s: %w", intent.Field, err)
			}
		}
	}
	e.secretMu.Lock()
	e.resolvedSecrets = resolved
	e.secretMu.Unlock()
	return nil
}

func (e *Executor) injectResolvedSecretWrites(change *planner.PlannedChange) error {
	if change == nil || len(change.SecretWrites) == 0 {
		return nil
	}
	e.secretMu.Lock()
	values := e.resolvedSecrets[change.ID]
	delete(e.resolvedSecrets, change.ID)
	e.secretMu.Unlock()
	for _, intent := range change.SecretWrites {
		value, ok := values[intent.Field]
		if !ok {
			return fmt.Errorf("preflighted secret value is unavailable for field %s", intent.Field)
		}
		if err := setSecretField(change.Fields, decodeSecretPointer(intent.Field), value); err != nil {
			return fmt.Errorf("failed to prepare secret field %s: %w", intent.Field, err)
		}
	}
	return nil
}

func (e *Executor) clearResolvedSecrets() {
	e.secretMu.Lock()
	clear(e.resolvedSecrets)
	e.secretMu.Unlock()
}

func cloneChangeForExecution(change *planner.PlannedChange) (*planner.PlannedChange, error) {
	if change == nil {
		return nil, fmt.Errorf("planned change is nil")
	}
	data, err := json.Marshal(change)
	if err != nil {
		return nil, fmt.Errorf("failed to clone planned change: %w", err)
	}
	var cloned planner.PlannedChange
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, fmt.Errorf("failed to clone planned change: %w", err)
	}
	if protection, ok := cloned.Protection.(map[string]any); ok {
		oldValue, oldOK := protection["old"].(bool)
		newValue, newOK := protection["new"].(bool)
		if oldOK && newOK {
			cloned.Protection = planner.ProtectionChange{Old: oldValue, New: newValue}
		}
	}
	return &cloned, nil
}

func decodeSecretPointer(pointer string) []string {
	return planner.DecodeJSONPointer(pointer)
}

func setSecretField(fields map[string]any, segments []string, value string) error {
	if len(segments) == 0 {
		return fmt.Errorf("field path is empty")
	}
	return setSecretValue(fields, segments, value)
}

func setSecretValue(current any, segments []string, value string) error {
	if len(segments) == 0 {
		return fmt.Errorf("field path is empty")
	}
	switch typed := current.(type) {
	case map[string]any:
		if len(segments) == 1 {
			typed[segments[0]] = value
			return nil
		}
		next, ok := typed[segments[0]]
		if !ok || next == nil {
			next = map[string]any{}
			typed[segments[0]] = next
		}
		return setSecretValue(next, segments[1:], value)
	case []any:
		index, err := strconv.Atoi(segments[0])
		if err != nil || index < 0 || index >= len(typed) {
			return fmt.Errorf("array index %q is out of range", segments[0])
		}
		if len(segments) == 1 {
			typed[index] = value
			return nil
		}
		return setSecretValue(typed[index], segments[1:], value)
	default:
		return fmt.Errorf("field path traverses a non-container value")
	}
}
