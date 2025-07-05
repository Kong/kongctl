package labels

import (
	"fmt"
	"reflect"
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

// ExtractLabelsFromField extracts labels from a planner field that could be various types
// Handles type assertions for map[string]interface{}, map[string]string, etc.
func ExtractLabelsFromField(field interface{}) map[string]string {
	if field == nil {
		return nil
	}

	result := make(map[string]string)

	switch labels := field.(type) {
	case map[string]interface{}:
		// Handle map[string]interface{} case from planner
		for k, v := range labels {
			if strVal, ok := v.(string); ok {
				result[k] = strVal
			}
		}
	case map[string]string:
		// Handle map[string]string case
		for k, v := range labels {
			result[k] = v
		}
	}

	return result
}

// BuildCreateLabels prepares labels for resource creation
// Adds management labels and handles protection status
func BuildCreateLabels(userLabels map[string]string, protection interface{}) map[string]string {
	result := make(map[string]string)

	// Copy user-defined labels (excluding KONGCTL labels)
	for k, v := range userLabels {
		if !IsKongctlLabel(k) {
			result[k] = v
		}
	}

	// Add protection label based on protection field
	protectionValue := FalseValue
	if prot, ok := protection.(bool); ok && prot {
		protectionValue = TrueValue
	}
	result[ProtectedKey] = protectionValue

	// Note: Managed and last-updated labels will be added by the client
	// This is to ensure they're added after any normalization

	return result
}

// BuildUpdateLabels prepares labels for resource update with removal support
// Returns a pointer map to support nil values for label removal
func BuildUpdateLabels(desiredLabels, currentLabels map[string]string, protection interface{}) map[string]*string {
	result := make(map[string]*string)

	// First, add all desired user labels
	for k, v := range desiredLabels {
		if !IsKongctlLabel(k) {
			val := v
			result[k] = &val
		}
	}

	// Then, add nil values for current user labels that should be removed
	for k := range currentLabels {
		if !IsKongctlLabel(k) {
			if _, exists := desiredLabels[k]; !exists {
				result[k] = nil
			}
		}
	}

	// Handle protection label
	protectionValue := FalseValue
	
	// Check if this is a protection change
	// We check for a struct with Old and New bool fields using reflection
	// to avoid circular dependency with planner package
	if protection != nil {
		v := fmt.Sprintf("%T", protection)
		if v == "planner.ProtectionChange" {
			// Use reflection to get the New field value
			// This avoids importing planner package which would create circular dependency
			if newVal := getProtectionNewValue(protection); newVal {
				protectionValue = TrueValue
			}
		} else if prot, ok := protection.(bool); ok && prot {
			protectionValue = TrueValue
		}
	}
	
	result[ProtectedKey] = &protectionValue

	// Note: Managed and last-updated labels will be added by the client
	// using AddManagedLabelsToPointerMap to preserve nil values

	return result
}

// getProtectionNewValue uses reflection to get the New field from a ProtectionChange
// This avoids circular dependency with the planner package
func getProtectionNewValue(protection interface{}) bool {
	// Try direct field access via reflection
	v := reflect.ValueOf(protection)
	if v.Kind() == reflect.Struct {
		if newField := v.FieldByName("New"); newField.IsValid() && newField.Kind() == reflect.Bool {
			return newField.Bool()
		}
	}
	
	return false
}

// ConvertStringMapToPointerMap converts map[string]string to map[string]*string
func ConvertStringMapToPointerMap(labels map[string]string) map[string]*string {
	if labels == nil {
		return nil
	}

	result := make(map[string]*string)
	for k, v := range labels {
		val := v
		result[k] = &val
	}
	return result
}

