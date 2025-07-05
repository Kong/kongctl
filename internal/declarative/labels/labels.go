package labels

import (
	"fmt"
	"strings"
	"time"
)

// Label keys used by kongctl
const (
	// Label prefix
	KongctlPrefix = "KONGCTL-"
	
	// Label keys (using prefix to avoid repetition)
	ManagedKey     = KongctlPrefix + "managed"
	LastUpdatedKey = KongctlPrefix + "last-updated"
	ProtectedKey   = KongctlPrefix + "protected"
	
	// Environment variables
	DebugEnvVar = "KONGCTL_DEBUG"
	
	// Label values
	TrueValue  = "true"
	FalseValue = "false"
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
	result[ManagedKey] = TrueValue
	// Use a timestamp format that only contains allowed characters for labels
	// Format: YYYYMMDD-HHMMSSZ (no colons allowed in label values)
	result[LastUpdatedKey] = time.Now().UTC().Format("20060102-150405Z")
	
	// If protected label is not already set by the executor, default to false
	if _, exists := result[ProtectedKey]; !exists {
		result[ProtectedKey] = FalseValue
	}
	
	return result
}

// IsManagedResource checks if resource has managed label
func IsManagedResource(labels map[string]string) bool {
	return labels != nil && labels[ManagedKey] == TrueValue
}

// IsProtectedResource checks if resource has protected label set to true
func IsProtectedResource(labels map[string]string) bool {
	return labels != nil && labels[ProtectedKey] == TrueValue
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
	return strings.HasPrefix(key, KongctlPrefix)
}

// ValidateLabel ensures label key follows Konnect rules
func ValidateLabel(key string) error {
	if len(key) < 1 || len(key) > 63 {
		return fmt.Errorf("label key must be 1-63 characters: %s", key)
	}
	
	// Allow our KONGCTL labels
	if strings.HasPrefix(key, KongctlPrefix) || strings.HasPrefix(key, "kongctl-") {
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

// AddManagedLabelsToPointerMap adds kongctl management labels to a pointer map
// This function preserves nil values (for label removal) while adding KONGCTL labels
func AddManagedLabelsToPointerMap(labels map[string]*string) map[string]*string {
	if labels == nil {
		labels = make(map[string]*string)
	}
	
	// Create result map preserving all existing entries including nil values
	result := make(map[string]*string)
	for k, v := range labels {
		result[k] = v
	}
	
	// Add management labels as pointers
	managedValue := TrueValue
	result[ManagedKey] = &managedValue
	
	// Add timestamp
	timestamp := time.Now().UTC().Format("20060102-150405Z")
	result[LastUpdatedKey] = &timestamp
	
	// If protected label is not already set, default to false
	if _, exists := result[ProtectedKey]; !exists {
		protectedValue := FalseValue
		result[ProtectedKey] = &protectedValue
	}
	
	return result
}

