package labels

import (
	"fmt"
	"strings"
	"time"
)

// Label keys used by kongctl
const (
	ManagedKey     = "KONGCTL-managed"
	ConfigHashKey  = "KONGCTL-config-hash"
	LastUpdatedKey = "KONGCTL-last-updated"
	ProtectedKey   = "KONGCTL-protected"
)

// NormalizeLabels converts pointer map to non-pointer map
func NormalizeLabels(labels map[string]*string) map[string]string {
	if labels == nil {
		return make(map[string]string)
	}
	
	normalized := make(map[string]string)
	for k, v := range labels {
		if v != nil {
			normalized[k] = *v
		}
	}
	return normalized
}

// DenormalizeLabels converts non-pointer map to pointer map for SDK
func DenormalizeLabels(labels map[string]string) map[string]*string {
	if len(labels) == 0 {
		return nil
	}
	
	denormalized := make(map[string]*string)
	for k, v := range labels {
		denormalized[k] = &v
	}
	return denormalized
}

// AddManagedLabels adds kongctl management labels
func AddManagedLabels(labels map[string]string, configHash string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	
	// Preserve existing labels
	result := make(map[string]string)
	for k, v := range labels {
		result[k] = v
	}
	
	// Add management labels
	result[ManagedKey] = "true"
	result[ConfigHashKey] = sanitizeLabelValue(configHash)
	result[LastUpdatedKey] = time.Now().UTC().Format(time.RFC3339)
	
	return result
}

// IsManagedResource checks if resource has managed label
func IsManagedResource(labels map[string]string) bool {
	return labels != nil && labels[ManagedKey] == "true"
}

// GetUserLabels returns labels without KONGCTL prefix
func GetUserLabels(labels map[string]string) map[string]string {
	user := make(map[string]string)
	for k, v := range labels {
		if !IsKongctlLabel(k) {
			user[k] = v
		}
	}
	return user
}

// IsKongctlLabel checks if label key is kongctl-managed
func IsKongctlLabel(key string) bool {
	return len(key) >= 8 && key[:8] == "KONGCTL-"
}

// ValidateLabel ensures label key follows Konnect rules
func ValidateLabel(key string) error {
	if len(key) < 1 || len(key) > 63 {
		return fmt.Errorf("label key must be 1-63 characters: %s", key)
	}
	
	// Allow our KONGCTL labels
	if strings.HasPrefix(key, "KONGCTL-") || strings.HasPrefix(key, "kongctl-") {
		return nil
	}
	
	// Check forbidden prefixes
	forbidden := []string{"kong", "konnect", "mesh", "kic", "_"}
	for _, prefix := range forbidden {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			return fmt.Errorf("label key cannot start with %s: %s", prefix, key)
		}
	}
	
	return nil
}

// sanitizeLabelValue converts a value to be compatible with Konnect label requirements
// Pattern: ^[a-z0-9A-Z]{1}([a-z0-9A-Z-._]*[a-z0-9A-Z]+)?$
func sanitizeLabelValue(value string) string {
	// Replace invalid characters with hyphens
	var result strings.Builder
	for i, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		} else if i > 0 && i < len(value)-1 && (r == '-' || r == '.' || r == '_') {
			result.WriteRune(r)
		} else {
			result.WriteRune('-')
		}
	}
	
	// Ensure it doesn't end with special characters
	s := result.String()
	s = strings.TrimRight(s, "-._")
	
	// Ensure it's not empty
	if s == "" {
		return "empty"
	}
	
	return s
}