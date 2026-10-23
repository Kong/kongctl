package executor

import (
	"encoding/json"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/declarative/values"
)

// Inspect the mapped SDK payload so management labels count toward the limit,
// SDK unions are flattened, and nil update values remain label removals.
func validateRequestLabels(request any, change *planner.PlannedChange) error {
	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to inspect request labels: %w", err)
	}
	var payload struct {
		Labels map[string]*string `json:"labels"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("failed to decode request labels: %w", err)
	}
	labelValues := labels.NormalizeLabels(payload.Labels)
	deferred := make(map[string]bool)
	if change != nil {
		for key, value := range labelValues {
			for _, binding := range change.PayloadReferences {
				if binding.Path == values.Child("/labels", key) &&
					value == tags.RefPlaceholderPrefix+binding.Ref+"#"+binding.Selector {
					deferred[key] = true
				}
			}
		}
	}
	return labels.ValidateLabelsWithDeferredValues(labelValues, labels.MaxCount, deferred)
}
