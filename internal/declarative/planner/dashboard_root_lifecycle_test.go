package planner

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestDashboardRootLifecycleIdentityAndSync(t *testing.T) {
	const first = "11111111-1111-4111-8111-111111111111"
	const second = "22222222-2222-4222-8222-222222222222"
	const missing = "33333333-3333-4333-8333-333333333333"
	for _, tt := range []struct {
		name, ref, boundID, desiredName, matchedID string
	}{
		{"bound ID precedes UUID ref", first, second, "renamed", second},
		{"UUID ref selects observation", first, "", "renamed", first},
		{"unique name", "local-ref", "", "second", second},
		{"missing bound ID has no name fallback", first, missing, "second", ""},
		{"missing UUID has no name fallback", missing, "", "second", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := newDashboardTestPlanner([]kkComps.DashboardResponse{
				dashboardRootObserved(first, "first", false),
				dashboardRootObserved(second, "second", false),
			})
			p.matchedIdentities = make(map[string]matchedResourceIdentity)
			desired := newDashboardResource(tt.ref, tt.desiredName)
			desired.SetKonnectID(tt.boundID)
			// Make even name-matched roots update so every case checks routing.
			desired.Definition = kkComps.Dashboard{}
			p.resources = &resources.ResourceSet{Dashboards: []resources.DashboardResource{desired}}
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
			require.NoError(t, p.planDashboardChanges(t.Context(), NewConfig("analytics"), p.resources.Dashboards, plan))
			require.NotEmpty(t, plan.Changes)
			if tt.matchedID == "" {
				require.Equal(t, ActionCreate, plan.Changes[0].Action)
			} else {
				require.Equal(t, ActionUpdate, plan.Changes[0].Action)
				require.Equal(t, tt.matchedID, plan.Changes[0].ResourceID)
			}
			var deleted, want []string
			for _, id := range []string{first, second} {
				if id != tt.matchedID {
					want = append(want, id)
				}
			}
			for _, change := range plan.Changes[1:] {
				require.Equal(t, ActionDelete, change.Action)
				deleted = append(deleted, change.ResourceID)
			}
			require.Equal(t, want, deleted, "pruning must retain original observation order")
			require.Empty(t, p.matchedIdentities, "lifecycle matching must not register payload identities")
			require.Equal(t, tt.boundID, p.resources.Dashboards[0].GetKonnectID())
		})
	}
}

func TestDashboardRootLifecycleDeleteDiagnostics(t *testing.T) {
	p := newDashboardTestPlanner([]kkComps.DashboardResponse{
		dashboardRootObserved("protected-id", "observed protected", true),
		dashboardRootObserved("delete-id", "observed removable", false),
	})
	desired := []resources.DashboardResource{
		newDashboardResource("protected-ref", "desired protected"),
		newDashboardResource("missing-ref", "missing"),
		newDashboardResource("delete-ref", "desired removable"),
	}
	desired[0].SetKonnectID("protected-id")
	desired[2].SetKonnectID("delete-id")
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeDelete)
	err := p.planDashboardChanges(t.Context(), NewConfig("analytics"), desired, plan)
	require.EqualError(t, err, "Cannot generate plan due to protected resources:\n"+
		"- dashboard \"desired protected\" is protected and cannot be deleted\n"+
		"\nTo proceed, first update these resources to set protected: false")
	require.Len(t, plan.Warnings, 1)
	require.Equal(t, `dashboard "missing" not found in Konnect, skipping delete`, plan.Warnings[0].Message)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, "delete-id", plan.Changes[0].ResourceID)
	require.Equal(t, "observed removable", plan.Changes[0].Fields[FieldName])
}

func TestDashboardRootLifecycleAmbiguityPrecedesProtection(t *testing.T) {
	for _, mode := range []PlanMode{PlanModeApply, PlanModeSync, PlanModeDelete} {
		t.Run(string(mode), func(t *testing.T) {
			p := newDashboardTestPlanner([]kkComps.DashboardResponse{
				dashboardRootObserved("protected-id", "protected", true),
				dashboardRootObserved("first-id", "ambiguous", false),
				dashboardRootObserved("second-id", "ambiguous", false),
				dashboardRootObserved("stale-id", "stale", false),
			})
			desired := []resources.DashboardResource{
				newDashboardResource("protected-ref", "protected"),
				newDashboardResource("ambiguous-ref", "ambiguous"),
				newDashboardResource("later-ref", "later"),
			}
			desired[0].Kongctl.Protected = new(true)
			desired[0].Definition = kkComps.Dashboard{}
			plan := NewPlan(CurrentPlanVersion, "test", mode)
			err := p.planDashboardChanges(t.Context(), NewConfig("analytics"), desired, plan)
			require.EqualError(t, err,
				`multiple managed dashboards named "ambiguous" found in namespace; use a UUID ref or remove duplicates`)
			require.Empty(t, plan.Changes, "ambiguity must abort before later roots and sync pruning")
		})
	}
}

func TestDashboardRootLifecycleProtectionTransitions(t *testing.T) {
	for _, tt := range []struct {
		name                  string
		oldProtected, protect bool
		changeFields, blocked bool
	}{
		{"add protection", false, true, false, false},
		{"add protection with rename", false, true, true, false},
		{"remove protection", true, false, false, false},
		{"remove protection with rename is blocked", true, false, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := newDashboardTestPlanner([]kkComps.DashboardResponse{dashboardRootObserved("id", "original", tt.oldProtected)})
			desired := newDashboardResource("ref", "original")
			desired.SetKonnectID("id")
			desired.Kongctl.Protected = new(tt.protect)
			if tt.changeFields {
				desired.Name = "renamed"
			}
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
			err := p.planDashboardChanges(t.Context(), NewConfig("analytics"), []resources.DashboardResource{desired}, plan)
			if tt.blocked {
				require.ErrorContains(t, err, `dashboard "renamed" is protected and cannot be updated`)
				require.Empty(t, plan.Changes, "a blocked update still retains the matched observation")
				return
			}
			require.NoError(t, err)
			require.Len(t, plan.Changes, 1)
			change := plan.Changes[0]
			require.Equal(t, ActionUpdate, change.Action)
			require.Equal(t, "id", change.ResourceID)
			require.Equal(t, desired.Name, change.Fields[FieldName])
			require.Equal(t, desired.Definition, change.Fields[FieldDefinition])
			require.Equal(t, ProtectionChange{Old: tt.oldProtected, New: tt.protect}, change.Protection)
		})
	}
}

func dashboardRootObserved(id, name string, protected bool) kkComps.DashboardResponse {
	desired := newDashboardResource("unused", name)
	desired.Labels[labels.NamespaceKey] = "analytics"
	if protected {
		desired.Labels[labels.ProtectedKey] = "true"
	}
	return kkComps.DashboardResponse{ID: &id, Name: name, Definition: desired.Definition, Labels: desired.Labels}
}
