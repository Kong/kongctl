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

