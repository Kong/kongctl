package labels

import (
	"fmt"
	"time"
)

// Label keys used by kongctl
const (
	ManagedKey     = "KONGCTL/managed"
	ConfigHashKey  = "KONGCTL/config-hash"
	LastUpdatedKey = "KONGCTL/last-updated"
	ProtectedKey   = "KONGCTL/protected"
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
	if labels == nil {
		return make(map[string]*string)
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
	result[ConfigHashKey] = configHash
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
	return len(key) >= 8 && key[:8] == "KONGCTL/"
}

// ValidateLabel ensures label key follows Konnect rules
func ValidateLabel(key string) error {
	if len(key) < 1 || len(key) > 63 {
		return fmt.Errorf("label key must be 1-63 characters: %s", key)
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