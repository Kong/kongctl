package fieldcontracts

import (
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func extractBool(value any) (bool, bool) {
	if value == nil {
		return false, false
	}
	switch v := value.(type) {
	case bool:
		return v, true
	case *bool:
		if v == nil {
			return false, false
		}
		return *v, true
	default:
		return false, false
	}
}

func convertProxyURLs(value any) ([]kkComps.ProxyURL, error) {
	if value == nil {
		return nil, nil
	}
	if urls, ok := value.([]kkComps.ProxyURL); ok {
		return urls, nil
	}

	switch items := value.(type) {
	case []any:
		return convertProxyInterfaceSlice(items)
	case []map[string]any:
		converted := make([]any, len(items))
		for i := range items {
			converted[i] = items[i]
		}
		return convertProxyInterfaceSlice(converted)
	default:
		return nil, fmt.Errorf("unsupported proxy_urls type %T", value)
	}
}

func convertProxyInterfaceSlice(items []any) ([]kkComps.ProxyURL, error) {
	urls := make([]kkComps.ProxyURL, 0, len(items))
	for _, item := range items {
		data, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("proxy url entry must be object, got %T", item)
		}

		host, _ := data["host"].(string)
		protocol, _ := data["protocol"].(string)

		portVal, hasPort := data["port"]
		var port int64
		if hasPort {
			switch pv := portVal.(type) {
			case int64:
				port = pv
			case int:
				port = int64(pv)
			case float64:
				port = int64(pv)
			default:
				return nil, fmt.Errorf("invalid proxy url port type %T", portVal)
			}
		}

		urls = append(urls, kkComps.ProxyURL{
			Host:     host,
			Port:     port,
			Protocol: protocol,
		})
	}

	return urls, nil
}

func proxyURLsEqual(current, desired []kkComps.ProxyURL) bool {
	if len(current) != len(desired) {
		return false
	}

	for i := range current {
		if current[i].Host != desired[i].Host ||
			current[i].Port != desired[i].Port ||
			current[i].Protocol != desired[i].Protocol {
			return false
		}
	}

	return true
}
