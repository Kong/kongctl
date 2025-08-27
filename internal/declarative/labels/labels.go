package labels

import (
	"fmt"
	"reflect"
	"strings"
)

// Label keys used by kongctl
const (
	// Label prefix
	KongctlPrefix = "KONGCTL-"

	// Label keys (using prefix to avoid repetition)
	NamespaceKey = KongctlPrefix + "namespace"
	ProtectedKey = KongctlPrefix + "protected"

	// Deprecated label keys (kept for backward compatibility)
	// TODO: Remove in future version after migration period
	ManagedKey     = KongctlPrefix + "managed"      // Deprecated: use namespace presence instead
	LastUpdatedKey = KongctlPrefix + "last-updated" // Deprecated: not needed

	// Environment variables
	DebugEnvVar = "KONGCTL_DEBUG"

	// Label values
	TrueValue  = "true"
	FalseValue = "false"
)

// NormalizeLabels converts pointer map to non-pointer map
func NormalizeLabels(labels map[string]*string) map[string]string {
	if labels == nil {
		return nil
	}

	normalized := make(map[string]string)
	for k, v := range labels {
		if v != nil {
			normalized[k] = *v
		}
	}

	// Return nil for empty maps to be consistent
	if len(normalized) == 0 {
		return nil
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
// Deprecated: Use BuildCreateLabels or BuildUpdateLabels instead
func AddManagedLabels(labels map[string]string, namespace string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}

	// Preserve existing labels
	result := make(map[string]string)
	for k, v := range labels {
		result[k] = v
	}

	// Add namespace label (required)
	result[NamespaceKey] = namespace

	// Note: Protected label is handled separately by executors
	// It's only added when explicitly set to true

	return result
}

// IsManagedResource checks if resource has namespace label (new criteria for managed resources)
func IsManagedResource(labels map[string]string) bool {
	if labels == nil {
		return false
	}
	// A resource is considered managed if it has a namespace label
	_, hasNamespace := labels[NamespaceKey]
	return hasNamespace
}

// IsProtectedResource checks if resource has protected label set to true
func IsProtectedResource(labels map[string]string) bool {
	return labels != nil && labels[ProtectedKey] == TrueValue
}

// GetUserLabels returns labels without KONGCTL prefix
func GetUserLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return nil
	}

	user := make(map[string]string)
	for k, v := range labels {
		if !IsKongctlLabel(k) {
			user[k] = v
		}
	}

	// Return nil for empty maps to be consistent
	if len(user) == 0 {
		return nil
	}

	return user
}

// IsKongctlLabel checks if label key is kongctl-managed
func IsKongctlLabel(key string) bool {
	return strings.HasPrefix(key, KongctlPrefix)
}

// CompareUserLabels compares only user-defined labels between current and desired states
// Returns true if user labels differ, ignoring KONGCTL system labels
func CompareUserLabels(current, desired map[string]string) bool {
	// Get user labels from both maps
	currentUser := GetUserLabels(current)
	desiredUser := GetUserLabels(desired)

	// If both are empty or nil, they're equal
	if len(currentUser) == 0 && len(desiredUser) == 0 {
		return false
	}

	// If lengths differ, they're not equal
	if len(currentUser) != len(desiredUser) {
		return true
	}

	// Compare each user label
	for k, v := range desiredUser {
		if currentVal, exists := currentUser[k]; !exists || currentVal != v {
			return true
		}
	}

	return false
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
// Deprecated: Use BuildCreateLabels or BuildUpdateLabels instead
func AddManagedLabelsToPointerMap(labels map[string]*string, namespace string) map[string]*string {
	if labels == nil {
		labels = make(map[string]*string)
	}

	// Create result map preserving all existing entries including nil values
	result := make(map[string]*string)
	for k, v := range labels {
		result[k] = v
	}

	// Add namespace label as pointer
	result[NamespaceKey] = &namespace

	// Note: Protected label is handled separately by executors
	// It's only added when explicitly set to true

	return result
}

// ExtractLabelsFromField extracts labels from a planner field that could be various types
// Handles type assertions for map[string]any, map[string]string, etc.
func ExtractLabelsFromField(field any) map[string]string {
	if field == nil {
		return nil
	}

	result := make(map[string]string)

	switch labels := field.(type) {
	case map[string]any:
		// Handle map[string]any case from planner
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

	// Return nil for empty maps to be consistent
	if len(result) == 0 {
		return nil
	}

	return result
}

// BuildCreateLabels prepares labels for resource creation
// Adds management labels and handles protection status
func BuildCreateLabels(userLabels map[string]string, namespace string, protection any) map[string]string {
	result := make(map[string]string)

	// Copy user-defined labels (excluding KONGCTL labels)
	for k, v := range userLabels {
		if !IsKongctlLabel(k) {
			result[k] = v
		}
	}

	// Add namespace label (required)
	result[NamespaceKey] = namespace

	// Only add protection label if explicitly true
	if prot, ok := protection.(bool); ok && prot {
		result[ProtectedKey] = TrueValue
	}

	return result
}

// BuildUpdateLabels prepares labels for resource update with removal support
// Returns a pointer map to support nil values for label removal
func BuildUpdateLabels(
	desiredLabels, currentLabels map[string]string,
	namespace string,
	protection any,
) map[string]*string {
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

	// Add namespace label (required)
	result[NamespaceKey] = &namespace

	// Handle protection label
	if protection != nil {
		// Check if this is a protection change
		// We check for a struct with Old and New bool fields using reflection
		// to avoid circular dependency with planner package
		v := fmt.Sprintf("%T", protection)
		if v == "planner.ProtectionChange" {
			// Use reflection to get the New field value
			// This avoids importing planner package which would create circular dependency
			if newVal := getProtectionNewValue(protection); newVal {
				val := TrueValue
				result[ProtectedKey] = &val
			} else {
				// Remove the label when changing from true to false
				result[ProtectedKey] = nil
			}
		} else if prot, ok := protection.(bool); ok {
			if prot {
				val := TrueValue
				result[ProtectedKey] = &val
			} else {
				// Remove the label if false
				result[ProtectedKey] = nil
			}
		}
	}
	// If protection is nil, don't add or modify the label

	return result
}

// getProtectionNewValue uses reflection to get the New field from a ProtectionChange
// This avoids circular dependency with the planner package
func getProtectionNewValue(protection any) bool {
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
	if len(labels) == 0 {
		return nil
	}

	result := make(map[string]*string)
	for k, v := range labels {
		val := v
		result[k] = &val
	}
	return result
}
