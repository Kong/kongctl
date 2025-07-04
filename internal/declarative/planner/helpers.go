package planner

import (
	"reflect"
	"github.com/kong/kongctl/internal/declarative/labels"
)

// extractUserLabels returns a map containing only user-defined labels,
// filtering out KONGCTL system labels
func extractUserLabels(allLabels map[string]string) map[string]string {
	userLabels := make(map[string]string)
	for k, v := range allLabels {
		if !labels.IsKongctlLabel(k) {
			userLabels[k] = v
		}
	}
	return userLabels
}

// extractUserLabelsFromPointers returns a map containing only user-defined labels,
// filtering out KONGCTL system labels from a pointer map
func extractUserLabelsFromPointers(allLabels map[string]*string) map[string]string {
	userLabels := make(map[string]string)
	for k, v := range allLabels {
		if !labels.IsKongctlLabel(k) && v != nil {
			userLabels[k] = *v
		}
	}
	return userLabels
}

// compareUserLabels compares two label maps, considering only user-defined labels
// and ignoring KONGCTL system labels. Returns true if user labels differ.
func compareUserLabels(currentLabels, desiredLabels map[string]string) bool {
	currentUser := extractUserLabels(currentLabels)
	desiredUser := extractUserLabels(desiredLabels)
	
	return !reflect.DeepEqual(currentUser, desiredUser)
}

// compareUserLabelsWithPointers compares current labels with desired pointer labels,
// considering only user-defined labels. Returns true if user labels differ.
func compareUserLabelsWithPointers(currentLabels map[string]string, desiredLabels map[string]*string) bool {
	currentUser := extractUserLabels(currentLabels)
	desiredUser := extractUserLabelsFromPointers(desiredLabels)
	
	return !reflect.DeepEqual(currentUser, desiredUser)
}

// compareOptionalString compares two optional string fields
// Returns (differs bool, newValue string)
func compareOptionalString(current *string, desired *string) (bool, string) {
	currentVal := getString(current)
	desiredVal := getString(desired)
	
	if currentVal != desiredVal {
		return true, desiredVal
	}
	return false, ""
}

// compareOptionalBool compares two optional bool fields
// Returns true if they differ
func compareOptionalBool(current *bool, desired *bool) bool {
	// If desired is nil, we don't want to update
	if desired == nil {
		return false
	}
	
	// If current is nil but desired is not, they differ
	if current == nil {
		return true
	}
	
	// Both are non-nil, compare values
	return *current != *desired
}

// mergeLabelsForUpdate prepares labels for an update operation by merging
// user-defined labels with preserved system labels
func mergeLabelsForUpdate(desiredLabels map[string]string, currentLabels map[string]string) map[string]string {
	merged := make(map[string]string)
	
	// Copy all desired labels
	for k, v := range desiredLabels {
		merged[k] = v
	}
	
	// Preserve system labels from current state
	for k, v := range currentLabels {
		if labels.IsKongctlLabel(k) {
			merged[k] = v
		}
	}
	
	return merged
}