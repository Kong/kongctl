package planner

import (
	"encoding/json"
	"reflect"
)

const aiGatewayPolicyTypeAISanitizer = "ai-sanitizer"

var aiGatewayAISanitizerConfigDefaults = map[string]any{
	"allow_all_conversation_history": true,
	"block_if_detected":              false,
	"custom_patterns":                nil,
	"host":                           "localhost",
	"keepalive_timeout":              float64(60000),
	"port":                           float64(8080),
	"proxy_config": map[string]any{
		"auth_password":    nil,
		"auth_username":    nil,
		"http_proxy_host":  nil,
		"http_proxy_port":  nil,
		"https_proxy_host": nil,
		"https_proxy_port": nil,
		"no_proxy":         nil,
		"proxy_scheme":     "http",
	},
	"recover_redacted":             true,
	"redact_type":                  "placeholder",
	"sanitization_mode":            "INPUT",
	"scheme":                       "http",
	"skip_logging_sanitized_items": false,
	"stop_on_error":                true,
	"timeout":                      float64(10000),
}

var aiGatewayRouteConfigDefaults = map[string]any{
	"protocols": []any{"http", "https"},
}

func normalizeAIGatewayPolicyDefaultsForComparison(
	currentPayload map[string]any,
	desiredPayload map[string]any,
) (map[string]any, map[string]any) {
	currentCompare := deepClonePayloadMap(currentPayload)
	desiredCompare := deepClonePayloadMap(desiredPayload)

	if currentCompare == nil || desiredCompare == nil {
		return currentCompare, desiredCompare
	}

	policyType, _ := desiredCompare[FieldType].(string)
	if policyType == "" {
		policyType, _ = currentCompare[FieldType].(string)
	}
	if policyType == aiGatewayPolicyTypeAISanitizer {
		pruneCurrentDefaultsAtPath(
			currentCompare,
			desiredCompare,
			[]string{FieldConfig},
			aiGatewayAISanitizerConfigDefaults,
		)
	}

	return currentCompare, desiredCompare
}

func normalizeAIGatewayRouteDefaultsForComparison(
	currentPayload map[string]any,
	desiredPayload map[string]any,
) (map[string]any, map[string]any) {
	currentCompare := deepClonePayloadMap(currentPayload)
	desiredCompare := deepClonePayloadMap(desiredPayload)

	pruneCurrentDefaultsAtPath(
		currentCompare,
		desiredCompare,
		[]string{FieldConfig, "route"},
		aiGatewayRouteConfigDefaults,
	)

	return currentCompare, desiredCompare
}

func deepClonePayloadMap(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return clonePayloadMap(payload)
	}
	var clone map[string]any
	if err := json.Unmarshal(data, &clone); err != nil {
		return clonePayloadMap(payload)
	}
	return clone
}

func pruneCurrentDefaultsAtPath(
	current map[string]any,
	desired map[string]any,
	path []string,
	defaults map[string]any,
) {
	if current == nil || len(path) == 0 {
		pruneCurrentDefaults(current, desired, defaults)
		return
	}

	key := path[0]
	currentChild, ok := current[key].(map[string]any)
	if !ok {
		return
	}

	desiredChild, desiredHasMap := desired[key].(map[string]any)
	if !desiredHasMap {
		desiredChild = map[string]any{}
	}

	pruneCurrentDefaultsAtPath(currentChild, desiredChild, path[1:], defaults)
	if !desiredHasMap && len(currentChild) == 0 {
		delete(current, key)
	}
}

func pruneCurrentDefaults(current map[string]any, desired map[string]any, defaults map[string]any) {
	if current == nil {
		return
	}
	if desired == nil {
		desired = map[string]any{}
	}

	for key, defaultValue := range defaults {
		currentValue, currentHasValue := current[key]
		if !currentHasValue {
			continue
		}

		defaultMap, defaultIsMap := defaultValue.(map[string]any)
		if defaultIsMap {
			currentMap, currentIsMap := currentValue.(map[string]any)
			if !currentIsMap {
				continue
			}
			desiredMap, desiredIsMap := desired[key].(map[string]any)
			if !desiredIsMap {
				desiredMap = map[string]any{}
			}
			pruneCurrentDefaults(currentMap, desiredMap, defaultMap)
			if _, desiredHasValue := desired[key]; !desiredHasValue && len(currentMap) == 0 {
				delete(current, key)
			}
			continue
		}

		if _, desiredHasValue := desired[key]; desiredHasValue {
			continue
		}
		if reflect.DeepEqual(currentValue, defaultValue) {
			delete(current, key)
		}
	}
}
