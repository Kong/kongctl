package attributes

import "fmt"

// NormalizeAPIAttributes converts arbitrary attribute values into the Konnect API
// canonical shape of map[string][]string. It returns the normalized map and true
// when conversion is successful. If the input cannot be converted, the original
// value should be used instead.
func NormalizeAPIAttributes(raw any) (map[string][]string, bool) {
	if raw == nil {
		return nil, false
	}

	switch attrs := raw.(type) {
	case map[string][]string:
		out := make(map[string][]string, len(attrs))
		for k, v := range attrs {
			if v == nil {
				continue
			}
			out[k] = append([]string(nil), v...)
		}
		return out, true
	case map[string][]any:
		out := make(map[string][]string, len(attrs))
		for k, v := range attrs {
			out[k] = toStringSlice(v)
		}
		return out, true
	case map[string]any:
		out := make(map[string][]string, len(attrs))
		for k, v := range attrs {
			out[k] = toStringSlice(v)
		}
		return out, true
	case map[any]any:
		out := make(map[string][]string, len(attrs))
		for k, v := range attrs {
			keyStr, ok := k.(string)
			if !ok {
				continue
			}
			out[keyStr] = toStringSlice(v)
		}
		return out, true
	default:
		return nil, false
	}
}

func toStringSlice(value any) []string {
	switch v := value.(type) {
	case nil:
		return nil
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if item == nil {
				continue
			}
			if str, ok := item.(string); ok {
				out = append(out, str)
				continue
			}
			out = append(out, fmt.Sprint(item))
		}
		return out
	case string:
		return []string{v}
	default:
		return []string{fmt.Sprint(v)}
	}
}
