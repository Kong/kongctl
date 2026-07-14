package executor

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kong/kongctl/internal/declarative/planner"
)

func mapAIGatewaySDKRequest(resource string, payload any, destination any) error {
	payload = aiGatewaySDKBody(payload)

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode %s payload: %w", resource, err)
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("failed to decode %s payload: %w", resource, &aiGatewaySDKDecodeError{cause: err})
	}

	mappedData, err := json.Marshal(destination)
	if err != nil {
		return fmt.Errorf("failed to verify %s SDK payload: %w", resource, err)
	}

	var sourceValue any
	if err := json.Unmarshal(data, &sourceValue); err != nil {
		return fmt.Errorf("failed to inspect %s source payload: %w", resource, err)
	}
	var mappedValue any
	if err := json.Unmarshal(mappedData, &mappedValue); err != nil {
		return fmt.Errorf("failed to inspect %s SDK payload: %w", resource, err)
	}

	dropped := droppedAIGatewayPayloadPaths(sourceValue, mappedValue, "")
	if len(dropped) > 0 {
		slices.Sort(dropped)
		return fmt.Errorf(
			"%s contains fields not supported by the current SDK: %s",
			resource,
			strings.Join(dropped, ", "),
		)
	}

	return nil
}

func aiGatewaySDKBody(payload any) any {
	fields, ok := payload.(map[string]any)
	if !ok {
		return payload
	}

	body := maps.Clone(fields)
	delete(body, planner.FieldAIGatewayID)
	delete(body, planner.FieldAIGatewayConsumerID)
	return body
}

type aiGatewaySDKDecodeError struct {
	cause error
}

func (e *aiGatewaySDKDecodeError) Error() string {
	return "the current SDK rejected the payload; verify the resource fields with kongctl explain"
}

func (e *aiGatewaySDKDecodeError) Unwrap() error {
	return e.cause
}

func droppedAIGatewayPayloadPaths(source, mapped any, path string) []string {
	switch sourceValue := source.(type) {
	case map[string]any:
		mappedValue, ok := mapped.(map[string]any)
		if !ok {
			return declaredAIGatewayPayloadPaths(sourceValue, path)
		}

		var dropped []string
		for key, value := range sourceValue {
			childPath := joinAIGatewayPayloadPath(path, key)
			mappedChild, found := mappedValue[key]
			if !found {
				dropped = append(dropped, declaredAIGatewayPayloadPaths(value, childPath)...)
				continue
			}
			dropped = append(dropped, droppedAIGatewayPayloadPaths(value, mappedChild, childPath)...)
		}
		return dropped
	case []any:
		mappedValue, ok := mapped.([]any)
		if !ok {
			return declaredAIGatewayPayloadPaths(sourceValue, path)
		}

		var dropped []string
		for i, value := range sourceValue {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			if i >= len(mappedValue) {
				dropped = append(dropped, declaredAIGatewayPayloadPaths(value, childPath)...)
				continue
			}
			dropped = append(dropped, droppedAIGatewayPayloadPaths(value, mappedValue[i], childPath)...)
		}
		return dropped
	default:
		return nil
	}
}

func declaredAIGatewayPayloadPaths(value any, path string) []string {
	switch typed := value.(type) {
	case nil:
		return nil
	case map[string]any:
		var paths []string
		for key, child := range typed {
			paths = append(paths, declaredAIGatewayPayloadPaths(child, joinAIGatewayPayloadPath(path, key))...)
		}
		return paths
	case []any:
		var paths []string
		for i, child := range typed {
			paths = append(paths, declaredAIGatewayPayloadPaths(child, fmt.Sprintf("%s[%d]", path, i))...)
		}
		return paths
	default:
		if path == "" {
			return nil
		}
		return []string{path}
	}
}

func joinAIGatewayPayloadPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}
