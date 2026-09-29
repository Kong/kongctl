package tags

import (
	"bytes"
	"encoding/json"
	"slices"
)

const DeferredEnvRedactedDisplay = "[redacted from !env]"

// RedactEnvOldValue hides remote values at paths supplied by deferred !env in
// the desired value. Old values are display metadata, never execution inputs.
// Neither input is mutated. JSON normalization supports typed SDK containers
// and preserves numbers without converting them to float64.
func RedactEnvOldValue(oldValue, newValue any) any {
	desired, err := envRedactionJSONValue(newValue)
	if err != nil {
		return DeferredEnvRedactedDisplay
	}
	if !containsDeferredEnv(desired) {
		return oldValue
	}
	current, err := envRedactionJSONValue(oldValue)
	if err != nil {
		return DeferredEnvRedactedDisplay
	}
	return redactEnvOldValue(current, desired)
}

func envRedactionJSONValue(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var normalized any
	err = decoder.Decode(&normalized)
	return normalized, err
}

func containsDeferredEnv(value any) bool {
	switch value := value.(type) {
	case string:
		return IsEnvPlaceholder(value)
	case map[string]any:
		for _, child := range value {
			if containsDeferredEnv(child) {
				return true
			}
		}
	case []any:
		if slices.ContainsFunc(value, containsDeferredEnv) {
			return true
		}
	}
	return false
}

func redactEnvOldValue(current, desired any) any {
	if current == nil || !containsDeferredEnv(desired) {
		return current
	}
	switch desired := desired.(type) {
	case map[string]any:
		if current, ok := current.(map[string]any); ok {
			for key, child := range desired {
				if old, exists := current[key]; exists {
					current[key] = redactEnvOldValue(old, child)
				}
			}
			return current
		}
	case []any:
		if current, ok := current.([]any); ok {
			for i := range min(len(current), len(desired)) {
				current[i] = redactEnvOldValue(current[i], desired[i])
			}
			return current
		}
	}
	// A deferred scalar or a changed container shape may replace any remote
	// value. Do not print that value, even if its type differs from the source.
	return DeferredEnvRedactedDisplay
}
