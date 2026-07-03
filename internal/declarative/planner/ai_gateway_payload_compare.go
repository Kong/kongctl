package planner

import (
	"encoding/json"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

func normalizeAIGatewayPayloadsForComparison(
	currentPayload map[string]any,
	desiredPayload map[string]any,
) (map[string]any, map[string]any) {
	currentCompare := normalizeAIGatewayJSONMap(currentPayload)
	desiredCompare := normalizeAIGatewayJSONMap(desiredPayload)

	pruneNilValues(currentCompare)
	pruneNilValues(desiredCompare)
	normalizeAIGatewayUnorderedCollections(currentCompare)
	normalizeAIGatewayUnorderedCollections(desiredCompare)
	pruneAIGatewayDefaultsMissingFromPeer(currentCompare, desiredCompare)
	pruneAIGatewayDefaultsMissingFromPeer(desiredCompare, currentCompare)
	pruneEmptyContainersMissingFromPeer(currentCompare, desiredCompare)
	pruneEmptyContainersMissingFromPeer(desiredCompare, currentCompare)

	return currentCompare, desiredCompare
}

func normalizeAIGatewayPolicyPayloadsForComparison(
	currentPayload map[string]any,
	desiredPayload map[string]any,
) (map[string]any, map[string]any) {
	currentCompare, desiredCompare := normalizeAIGatewayPayloadsForComparison(currentPayload, desiredPayload)

	pruneAIGatewayPolicyConfigDefaultsMissingFromPeer(currentCompare, desiredCompare)
	pruneAIGatewayPolicyConfigDefaultsMissingFromPeer(desiredCompare, currentCompare)
	pruneEmptyContainersMissingFromPeer(currentCompare, desiredCompare)
	pruneEmptyContainersMissingFromPeer(desiredCompare, currentCompare)

	return currentCompare, desiredCompare
}

func diffAIGatewayPayloads(
	currentPayload map[string]any,
	desiredPayload map[string]any,
	currentCompare map[string]any,
	desiredCompare map[string]any,
) map[string]FieldChange {
	changedFields := make(map[string]FieldChange)
	keys := make(map[string]struct{})
	for key := range currentCompare {
		keys[key] = struct{}{}
	}
	for key := range desiredCompare {
		keys[key] = struct{}{}
	}
	for key := range keys {
		if !reflect.DeepEqual(currentCompare[key], desiredCompare[key]) {
			changedFields[key] = FieldChange{Old: currentPayload[key], New: desiredPayload[key]}
		}
	}
	return changedFields
}

func normalizeAIGatewayJSONMap(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return clonePayloadMap(payload)
	}
	var normalized map[string]any
	if err := json.Unmarshal(data, &normalized); err != nil {
		return clonePayloadMap(payload)
	}
	return normalized
}

func pruneNilValues(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if child == nil {
				delete(typed, key)
				continue
			}
			typed[key] = pruneNilValues(child)
		}
		return typed
	case []any:
		for i, child := range typed {
			typed[i] = pruneNilValues(child)
		}
		return typed
	default:
		return value
	}
}

func normalizeAIGatewayUnorderedCollections(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			typed[key] = normalizeAIGatewayUnorderedCollectionValue(key, child)
		}
		return typed
	case []any:
		for i, child := range typed {
			typed[i] = normalizeAIGatewayUnorderedCollections(child)
		}
		return typed
	default:
		return value
	}
}

func normalizeAIGatewayUnorderedCollectionValue(key string, value any) any {
	normalized := normalizeAIGatewayUnorderedCollections(value)
	switch key {
	case FieldTargets:
		return sortAIGatewayObjectSlice(normalized, FieldName, FieldProvider, FieldType)
	case FieldFormats:
		return sortAIGatewayObjectSlice(normalized, FieldType)
	case "capabilities", "failover_criteria", "protocols":
		return sortAIGatewayStringSlice(normalized)
	default:
		return normalized
	}
}

func sortAIGatewayObjectSlice(value any, keys ...string) any {
	items, ok := value.([]any)
	if !ok {
		return value
	}
	for _, item := range items {
		if _, ok := item.(map[string]any); !ok {
			return value
		}
	}
	slices.SortStableFunc(items, func(a, b any) int {
		return strings.Compare(aiGatewayObjectSortKey(a, keys), aiGatewayObjectSortKey(b, keys))
	})
	return items
}

func aiGatewayObjectSortKey(value any, keys []string) string {
	payload, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		if part, ok := aiGatewayScalarSortValue(payload[key]); ok {
			parts = append(parts, key+"="+part)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\x00")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func aiGatewayScalarSortValue(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, true
	case bool:
		if typed {
			return "true", true
		}
		return "false", true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case nil:
		return "", false
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return "", false
		}
		return string(data), true
	}
}

func sortAIGatewayStringSlice(value any) any {
	items, ok := stringSliceFromValue(value)
	if !ok {
		return value
	}
	slices.Sort(items)
	normalized := make([]any, len(items))
	for i, item := range items {
		normalized[i] = item
	}
	return normalized
}

func pruneAIGatewayDefaultsMissingFromPeer(payload map[string]any, peer map[string]any) {
	for key, value := range payload {
		var peerValue any
		if peer != nil {
			peerValue = peer[key]
		}

		if payloadMap, ok := value.(map[string]any); ok {
			var peerMap map[string]any
			if typed, ok := peerValue.(map[string]any); ok {
				peerMap = typed
			}
			pruneAIGatewayDefaultsMissingFromPeer(payloadMap, peerMap)
		}
		if payloadSlice, ok := value.([]any); ok {
			if peerSlice, ok := peerValue.([]any); ok {
				pruneAIGatewaySliceDefaultsMissingFromPeer(payloadSlice, peerSlice)
			}
		}

		if _, ok := peer[key]; !ok && isAIGatewayDefaultValue(key, value) {
			delete(payload, key)
		}
	}
}

func pruneAIGatewaySliceDefaultsMissingFromPeer(payload []any, peer []any) {
	for i, value := range payload {
		if i >= len(peer) {
			return
		}
		payloadMap, payloadIsMap := value.(map[string]any)
		peerMap, peerIsMap := peer[i].(map[string]any)
		if payloadIsMap && peerIsMap {
			pruneAIGatewayDefaultsMissingFromPeer(payloadMap, peerMap)
			pruneEmptyContainersMissingFromPeer(payloadMap, peerMap)
		}
	}
}

func pruneEmptyContainersMissingFromPeer(payload map[string]any, peer map[string]any) {
	for key, value := range payload {
		peerValue, peerHasKey := peer[key]

		payloadMap, payloadIsMap := value.(map[string]any)
		peerMap, peerIsMap := peerValue.(map[string]any)
		if payloadIsMap {
			if peerIsMap {
				pruneEmptyContainersMissingFromPeer(payloadMap, peerMap)
			}
			if !peerHasKey && len(payloadMap) == 0 {
				delete(payload, key)
				continue
			}
		}

		payloadSlice, payloadIsSlice := value.([]any)
		peerSlice, peerIsSlice := peerValue.([]any)
		if payloadIsSlice {
			if peerIsSlice {
				for i, child := range payloadSlice {
					if i >= len(peerSlice) {
						break
					}
					childMap, childIsMap := child.(map[string]any)
					peerChildMap, peerChildIsMap := peerSlice[i].(map[string]any)
					if childIsMap && peerChildIsMap {
						pruneEmptyContainersMissingFromPeer(childMap, peerChildMap)
					}
				}
			}
			if !peerHasKey && len(payloadSlice) == 0 {
				delete(payload, key)
			}
		}
	}
}

func pruneAIGatewayPolicyConfigDefaultsMissingFromPeer(payload map[string]any, peer map[string]any) {
	policyType, ok := payload[FieldType].(string)
	if !ok {
		return
	}
	if peerType, ok := peer[FieldType].(string); ok && peerType != policyType {
		return
	}

	config, ok := payload[FieldConfig].(map[string]any)
	if !ok {
		return
	}
	peerConfig, _ := peer[FieldConfig].(map[string]any)

	switch policyType {
	case "ai-prompt-guard":
		pruneAIGatewayPolicyConfigDefaults(config, peerConfig, isAIGatewayPromptGuardConfigDefaultValue)
	case "ai-sanitizer":
		pruneAIGatewayPolicyConfigDefaults(config, peerConfig, isAIGatewaySanitizerConfigDefaultValue)
	}
}

func pruneAIGatewayPolicyConfigDefaults(
	config map[string]any,
	peerConfig map[string]any,
	isDefault func(string, any) bool,
) {
	for key, value := range config {
		var peerValue any
		if peerConfig != nil {
			peerValue = peerConfig[key]
		}

		if configMap, ok := value.(map[string]any); ok {
			var peerMap map[string]any
			if typed, ok := peerValue.(map[string]any); ok {
				peerMap = typed
			}
			pruneAIGatewayPolicyConfigDefaults(configMap, peerMap, isDefault)
		}

		if _, ok := peerConfig[key]; !ok && isDefault(key, value) {
			delete(config, key)
		}
	}
}

func isAIGatewayPromptGuardConfigDefaultValue(key string, value any) bool {
	switch key {
	case "allow_all_conversation_history", "match_all_roles":
		return boolValueEqual(value, false)
	case "allow_patterns":
		return value == nil
	case "genai_category":
		return stringValueEqual(value, "text/generation")
	case "llm_format":
		return stringValueEqual(value, "openai")
	case "max_request_body_size":
		return numberValueEqual(value, 8388608)
	default:
		return false
	}
}

func isAIGatewaySanitizerConfigDefaultValue(key string, value any) bool {
	switch key {
	case "allow_all_conversation_history", "recover_redacted", "stop_on_error":
		return boolValueEqual(value, true)
	case "block_if_detected", "skip_logging_sanitized_items":
		return boolValueEqual(value, false)
	case "host":
		return stringValueEqual(value, "localhost")
	case "keepalive_timeout":
		return numberValueEqual(value, 60000)
	case "max_request_body_size":
		return numberValueEqual(value, 8388608)
	case "port":
		return numberValueEqual(value, 8080)
	case "proxy_scheme", "scheme":
		return stringValueEqual(value, "http")
	case "redact_type":
		return stringValueEqual(value, "placeholder")
	case "sanitization_mode":
		return stringValueEqual(value, "INPUT")
	case "timeout":
		return numberValueEqual(value, 10000)
	case "dictionary_name", "genai_category", "llm_format", "redis":
		return true
	default:
		return false
	}
}

func isAIGatewayDefaultValue(key string, value any) bool {
	switch key {
	case "allow_auth_override", "audits", "payloads", "preserve_host":
		return boolValueEqual(value, false)
	case "name_header", "request_buffering", "response_buffering", "statistics", "strip_path":
		return boolValueEqual(value, true)
	case "https_redirect_status_code":
		return numberValueEqual(value, 426)
	case "max_payload_size":
		return numberValueEqual(value, 1048576)
	case "max_request_body_size":
		return numberValueEqual(value, 8388608)
	case "regex_priority":
		return numberValueEqual(value, 0)
	case "response_streaming":
		return stringValueEqual(value, "allow")
	case "failover_criteria":
		return stringSliceValueEqual(value, []string{"error", "timeout"})
	case "protocols":
		return stringSliceValueEqual(value, []string{"http", "https"})
	case "connect_timeout", "read_timeout", "write_timeout":
		return numberValueEqual(value, 60000)
	case "fail_timeout":
		return numberValueEqual(value, 10000)
	case "max_fails":
		return numberValueEqual(value, 0)
	case "retries":
		return numberValueEqual(value, 5)
	case "slots":
		return numberValueEqual(value, 10000)
	case "weight":
		return numberValueEqual(value, 100)
	default:
		return false
	}
}

func boolValueEqual(value any, want bool) bool {
	got, ok := value.(bool)
	return ok && got == want
}

func numberValueEqual(value any, want float64) bool {
	switch typed := value.(type) {
	case float64:
		return typed == want
	case float32:
		return float64(typed) == want
	case int:
		return float64(typed) == want
	case int64:
		return float64(typed) == want
	case int32:
		return float64(typed) == want
	default:
		return false
	}
}

func stringValueEqual(value any, want string) bool {
	got, ok := value.(string)
	return ok && got == want
}

func stringSliceValueEqual(value any, want []string) bool {
	got, ok := stringSliceFromValue(value)
	if !ok || len(got) != len(want) {
		return false
	}
	got = slices.Clone(got)
	want = slices.Clone(want)
	slices.Sort(got)
	slices.Sort(want)
	return slices.Equal(got, want)
}

func stringSliceFromValue(value any) ([]string, bool) {
	switch typed := value.(type) {
	case []any:
		items := make([]string, len(typed))
		for i, item := range typed {
			stringItem, ok := item.(string)
			if !ok {
				return nil, false
			}
			items[i] = stringItem
		}
		return items, true
	case []string:
		return slices.Clone(typed), true
	default:
		return nil, false
	}
}
