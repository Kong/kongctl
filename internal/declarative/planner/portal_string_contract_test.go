package planner

import (
	"encoding/json"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

// Plain fixtures preserve pointer presence without SDK decoding defaults.
type (
	portalStringCreateFixture kkComps.CreatePortal
	portalStringListFixture   kkComps.ListPortalsResponsePortal
)

func TestPortalStringContractPlanning(t *testing.T) {
	stringFields := []string{
		FieldDisplayName, FieldDescription,
	}
	for _, field := range stringFields {
		for _, desiredValue := range []any{nil, "", "old", "new"} {
			for _, currentValue := range []any{nil, "", "old", "new"} {
				t.Run(fmt.Sprintf("%s/desired=%v/current=%v", field, desiredValue, currentValue), func(t *testing.T) {
					var desired resources.PortalResource
					var current state.Portal
					payload, err := json.Marshal(map[string]any{field: desiredValue})
					require.NoError(t, err)
					var desiredCreate portalStringCreateFixture
					require.NoError(t, json.Unmarshal(payload, &desiredCreate))
					desired.CreatePortal = kkComps.CreatePortal(desiredCreate)
					payload, err = json.Marshal(map[string]any{field: currentValue})
					require.NoError(t, err)
					var currentList portalStringListFixture
					require.NoError(t, json.Unmarshal(payload, &currentList))
					current.ListPortalsResponsePortal = kkComps.ListPortalsResponsePortal(currentList)
					fields := extractPortalFields(desired)
					for _, other := range stringFields {
						if other != field {
							require.NotContains(t, fields, other)
						}
					}
					if desiredValue == nil {
						require.NotContains(t, fields, field)
					} else {
						require.Equal(t, desiredValue, fields[field])
					}
					p := &portalPlannerImpl{BasePlanner: &BasePlanner{}}
					changed, updates, changes := p.shouldUpdatePortal(current, desired)
					old := currentValue
					if old == nil {
						old = ""
					}
					want := desiredValue != nil && desiredValue != old
					require.Equal(t, want, changed)
					if want {
						require.Equal(t, map[string]any{field: desiredValue}, updates)
						require.Equal(t, map[string]FieldChange{field: {Old: old, New: desiredValue}}, changes)
					} else {
						require.Empty(t, updates)
						require.Empty(t, changes)
					}
				})
			}
		}
	}
}
