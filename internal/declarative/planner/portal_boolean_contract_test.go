package planner

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestPortalBooleanContractPlanning(t *testing.T) {
	for _, field := range []string{
		FieldAuthenticationEnabled, FieldRBACEnabled, FieldSIPREnabled,
		FieldAutoApproveDevelopers, FieldAutoApproveApplications,
	} {
		for _, desiredValue := range []any{nil, false, true} {
			for _, currentValue := range []any{nil, false, true} {
				t.Run(fmt.Sprintf("%s/desired=%v/current=%v", field, desiredValue, currentValue), func(t *testing.T) {
					var desired resources.PortalResource
					var current state.Portal
					payload, err := json.Marshal(map[string]any{field: desiredValue})
					require.NoError(t, err)
					require.NoError(t, json.Unmarshal(payload, &desired.CreatePortal))
					payload, err = json.Marshal(map[string]any{field: currentValue})
					require.NoError(t, err)
					require.NoError(t, json.Unmarshal(payload, &current.ListPortalsResponsePortal))
					fields := extractPortalFields(desired)
					if desiredValue == nil {
						require.NotContains(t, fields, field)
					} else {
						require.Equal(t, desiredValue, fields[field])
					}
					p := &portalPlannerImpl{BasePlanner: &BasePlanner{}}
					changed, updates, changes := p.shouldUpdatePortal(current, desired)
					want := desiredValue != nil && desiredValue != currentValue
					require.Equal(t, want, changed)
					if want {
						require.Equal(t, map[string]any{field: desiredValue}, updates)
						require.Equal(t, map[string]FieldChange{field: {Old: currentValue, New: desiredValue}}, changes)
					} else {
						require.Empty(t, updates)
						require.Empty(t, changes)
					}
				})
			}
		}
	}
}
