package executor

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestMeterSavedPlanRejectsImmutableUpdatePayload(t *testing.T) {
	adapter := NewMeterAdapter(nil)
	ctx := NewExecutionContext(&planner.PlannedChange{Namespace: "billing"})
	for _, field := range []string{
		planner.FieldKey, planner.FieldAggregation, planner.FieldEventType,
		planner.FieldValueProperty, planner.FieldEventsFrom,
	} {
		var req kkComps.UpdateMeterRequest
		require.ErrorContains(
			t,
			adapter.MapUpdateFields(t.Context(), ctx, map[string]any{field: "changed"}, &req, nil),
			"unknown field",
		)
	}
	var create kkComps.CreateMeterRequest
	require.Error(
		t,
		adapter.MapCreateFields(
			t.Context(),
			ctx,
			map[string]any{planner.FieldName: "Usage", planner.FieldKey: "usage", planner.FieldAggregation: "count"},
			&create,
		),
	)
}

func TestMeterExecutionRevalidatesProtection(t *testing.T) {
	server := fixture.NewMeterServer(
		t,
		kkComps.Meter{
			ID:     fixture.MeterID,
			Key:    "usage",
			Labels: map[string]string{labels.NamespaceKey: "billing", labels.ProtectedKey: "true"},
		},
	)
	adapter := NewMeterAdapter(state.NewClient(state.ClientConfig{MetersAPI: server.SDK.GetMetersAPI()}))
	tests := []struct {
		name    string
		change  planner.PlannedChange
		allowed bool
	}{
		{name: "delete", change: planner.PlannedChange{Action: planner.ActionDelete}},
		{
			name: "stale ordinary update",
			change: planner.PlannedChange{
				Action:     planner.ActionUpdate,
				Protection: planner.ProtectionChange{Old: false, New: false},
				Fields:     map[string]any{planner.FieldName: "Renamed"},
			},
		},
		{
			name: "unprotect and edit",
			change: planner.PlannedChange{
				Action:     planner.ActionUpdate,
				Protection: planner.ProtectionChange{Old: true, New: false},
				Fields:     map[string]any{planner.FieldName: "Renamed"},
			},
		},
		{
			name: "unprotect only",
			change: planner.PlannedChange{
				Action:     planner.ActionUpdate,
				Protection: planner.ProtectionChange{Old: true, New: false},
			},
			allowed: true,
		},
		{
			name: "saved unprotect only",
			change: planner.PlannedChange{
				Action:     planner.ActionUpdate,
				Protection: map[string]any{"old": true, "new": false},
			},
			allowed: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.change.Namespace = "billing"
			_, err := adapter.GetByID(t.Context(), fixture.MeterID, NewExecutionContext(&tt.change))
			if tt.allowed {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "protected")
			}
		})
	}
}
