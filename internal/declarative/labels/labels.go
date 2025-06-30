package labels

import (
	"fmt"
	"strings"
	"time"
)

// Label keys used by kongctl
const (
	ManagedKey     = "KONGCTL-managed"
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
func AddManagedLabels(labels map[string]string) map[string]string {
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
	// Use a timestamp format that only contains allowed characters for labels
	// Format: YYYYMMDD-HHMMSSZ (no colons allowed in label values)
	result[LastUpdatedKey] = time.Now().UTC().Format("20060102-150405Z")
	
	// Always include protected label, default to false if not already set
	if _, exists := result[ProtectedKey]; !exists {
		result[ProtectedKey] = "false"
	}
	
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

