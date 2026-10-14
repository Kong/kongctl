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
	portalVisibilityCreateFixture kkComps.CreatePortal
	portalVisibilityListFixture   kkComps.ListPortalsResponsePortal
)

func TestPortalVisibilityContractPlanning(t *testing.T) {
	visibilityFields := []string{
		FieldDefaultAPIVisibility, FieldDefaultPageVisibility,
	}
	for _, field := range visibilityFields {
		for _, desiredValue := range []any{nil, "", "public", "private", "future-value"} {
			for _, currentValue := range []any{nil, "", "public", "private", "future-value"} {
				t.Run(fmt.Sprintf("%s/desired=%v/current=%v", field, desiredValue, currentValue), func(t *testing.T) {
					var desired resources.PortalResource
					var current state.Portal
					payload, err := json.Marshal(map[string]any{field: desiredValue})
					require.NoError(t, err)
					var desiredCreate portalVisibilityCreateFixture
					require.NoError(t, json.Unmarshal(payload, &desiredCreate))
					desired.CreatePortal = kkComps.CreatePortal(desiredCreate)
					payload, err = json.Marshal(map[string]any{field: currentValue})
					require.NoError(t, err)
					var currentList portalVisibilityListFixture
					require.NoError(t, json.Unmarshal(payload, &currentList))
					current.ListPortalsResponsePortal = kkComps.ListPortalsResponsePortal(currentList)
					fields := extractPortalFields(desired)
					for _, other := range visibilityFields {
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

func TestPortalFieldProjectionComposition(t *testing.T) {
	desiredFields := map[string]any{
		FieldAuthenticationEnabled: false, FieldRBACEnabled: false, FieldSIPREnabled: false,
		FieldAutoApproveDevelopers: false, FieldAutoApproveApplications: false,
		FieldDisplayName: "", FieldDescription: "",
		FieldDefaultAPIVisibility: "public", FieldDefaultPageVisibility: "private",
	}
	currentFields := map[string]any{
		FieldAuthenticationEnabled: true, FieldRBACEnabled: true, FieldSIPREnabled: true,
		FieldAutoApproveDevelopers: true, FieldAutoApproveApplications: true,
		FieldDisplayName: "old", FieldDescription: "old",
		FieldDefaultAPIVisibility: "private", FieldDefaultPageVisibility: "public",
	}
	data, err := json.Marshal(desiredFields)
	require.NoError(t, err)
	var desired portalVisibilityCreateFixture
	require.NoError(t, json.Unmarshal(data, &desired))
	data, err = json.Marshal(currentFields)
	require.NoError(t, err)
	var current portalVisibilityListFixture
	require.NoError(t, json.Unmarshal(data, &current))
	p := &portalPlannerImpl{BasePlanner: &BasePlanner{}}
	declared := resources.PortalResource{CreatePortal: kkComps.CreatePortal(desired)}
	fields := extractPortalFields(declared)
	delete(fields, FieldName)
	require.Equal(t, desiredFields, fields)
	changed, updates, changes := p.shouldUpdatePortal(
		state.Portal{ListPortalsResponsePortal: kkComps.ListPortalsResponsePortal(current)}, declared,
	)
	require.True(t, changed)
	require.Equal(t, desiredFields, updates)
	require.Len(t, changes, len(desiredFields))
	for field, value := range desiredFields {
		require.Equal(t, FieldChange{Old: currentFields[field], New: value}, changes[field])
	}
}
