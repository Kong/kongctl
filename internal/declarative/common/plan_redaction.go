package common

import (
	"maps"
	"slices"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/konnect/httpclient"
)

// RedactPlanForDisplay copies plan payloads for structured diffs and reports.
// The executable plan retains the actual desired values.
func RedactPlanForDisplay(plan *planner.Plan) *planner.Plan {
	if plan == nil {
		return nil
	}
	redacted := *plan
	redacted.Changes = slices.Clone(plan.Changes)
	for i := range redacted.Changes {
		change := &redacted.Changes[i]
		if len(change.Fields) > 0 {
			change.Fields = httpclient.RedactSensitiveFields(change.Fields).(map[string]any)
		}
		change.ChangedFields = maps.Clone(change.ChangedFields)
		for field, fc := range change.ChangedFields {
			fc.Old = redactDisplayField(field, fc.Old)
			fc.New = redactDisplayField(field, fc.New)
			change.ChangedFields[field] = fc
		}
	}
	redactPlanEnvOldValues(&redacted)
	return &redacted
}

func redactDisplayField(field string, value any) any {
	redacted := httpclient.RedactSensitiveFields(map[string]any{field: value}).(map[string]any)
	return redacted[field]
}
