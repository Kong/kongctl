package eventgateway

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func formatLabelPairs(labels map[string]string) string {
	if len(labels) == 0 {
		return valueNA
	}

	pairs := make([]string, 0, len(labels))
	for k, v := range labels {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(pairs)
	return strings.Join(pairs, ", ")
}

func formatJSONValue(value any) string {
	if value == nil {
		return valueNA
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprint(value)
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" || trimmed == "{}" || trimmed == "[]" {
		return valueNA
	}

	return trimmed
}
