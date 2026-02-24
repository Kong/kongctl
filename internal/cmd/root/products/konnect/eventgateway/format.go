package eventgateway

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
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

func formatListenerPorts(ports []kkComps.EventGatewayListenerPort) string {
	if len(ports) == 0 {
		return valueNA
	}

	portStrs := make([]string, 0, len(ports))
	for _, p := range ports {
		switch p.Type {
		case kkComps.EventGatewayListenerPortTypeStr:
			if p.Str != nil {
				portStrs = append(portStrs, *p.Str)
			}
		case kkComps.EventGatewayListenerPortTypeInteger:
			if p.Integer != nil {
				portStrs = append(portStrs, fmt.Sprintf("%d", *p.Integer))
			}
		}
	}

	if len(portStrs) == 0 {
		return valueNA
	}

	return strings.Join(portStrs, ", ")
}
