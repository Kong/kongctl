package planner

import (
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestCatalogRootLifecycleProtection(t *testing.T) {
	for _, tt := range []struct {
		name                  string
		oldProtected, protect bool
		changeFields          bool
		wantError             bool
		wantChanges           int
	}{
		{"no-op", false, false, false, false, 0},
		{"update", false, false, true, false, 1},
		{"protected no-op", true, true, false, false, 0},
		{"blocked update", true, true, true, true, 0},
		{"add protection", false, true, false, false, 1},
		{"add protection with fields", false, true, true, false, 1},
		{"remove protection", true, false, false, false, 1},
		{"remove protection with fields", true, false, true, true, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			current := catalogRootObserved("observed-id", "service", tt.oldProtected)
			p := catalogRootPlanner(current)
			desired := catalogRootDesired("different-ref", "service", tt.protect)
			desired.SetKonnectID("earlier-bound-id")
			if tt.changeFields {
				desired.Description = new("changed")
			}
			p.resources = &resources.ResourceSet{CatalogServices: []resources.CatalogServiceResource{desired}}
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
			err := p.planCatalogServiceChanges(t.Context(), NewConfig("default"), p.resources.CatalogServices, plan)
			if tt.wantError {
				require.ErrorContains(t, err, `catalog_service "service" is protected and cannot be updated`)
			} else {
				require.NoError(t, err)
			}
			require.Len(t, plan.Changes, tt.wantChanges)
			// Lifecycle matching must neither rebind the declaration nor enroll a
			// new observed identity for downstream arbitrary payload references.
			require.Equal(t, "earlier-bound-id", p.resources.CatalogServices[0].GetKonnectID())
			require.Empty(t, p.matchedIdentities)
			if tt.wantChanges == 0 {
				return
			}
			change := plan.Changes[0]
			require.Equal(t, ActionUpdate, change.Action)
			require.Equal(t, "observed-id", change.ResourceID)
			require.Equal(t, "different-ref", change.ResourceRef)
			require.Equal(t, "service", change.Fields[FieldName])
			require.Equal(t, "service", change.Fields[FieldDisplayName])
			require.Equal(t, ProtectionChange{Old: tt.oldProtected, New: tt.protect}, change.Protection)
			if tt.changeFields {
				require.Equal(t, "changed", change.Fields[FieldDescription])
				require.Equal(t, FieldChange{Old: "", New: "changed"}, change.ChangedFields[FieldDescription])
			}
		})
	}
}

func TestCatalogRootLifecycleDeleteContinuesAfterProtectionErrors(t *testing.T) {
	p := catalogRootPlanner(
		catalogRootObserved("protected-id", "protected", true),
		catalogRootObserved("first-id", "duplicate", false),
		catalogRootObserved("last-id", "duplicate", false),
	)
	desired := []resources.CatalogServiceResource{
		catalogRootDesired("protected-ref", "protected", false),
		catalogRootDesired("missing-ref", "missing", false),
		catalogRootDesired("different-ref", "duplicate", false),
	}
	desired[2].SetKonnectID("first-id")
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeDelete)
	err := p.planCatalogServiceChanges(t.Context(), NewConfig("default"), desired, plan)
	require.ErrorContains(t, err, `catalog_service "protected" is protected and cannot be deleted`)
	require.Len(t, plan.Warnings, 1)
	require.Equal(t, `catalog_service "missing" not found in Konnect, skipping delete`, plan.Warnings[0].Message)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ActionDelete, plan.Changes[0].Action)
	require.Equal(t, "last-id", plan.Changes[0].ResourceID)
}

func TestCatalogRootLifecycleSyncRetentionAndContinuation(t *testing.T) {
	p := catalogRootPlanner(
		catalogRootObserved("first-id", "kept", false),
		catalogRootObserved("last-id", "kept", false),
		catalogRootObserved("blocked-id", "blocked", true),
		catalogRootObserved("protected-stale-id", "protected-stale", true),
		catalogRootObserved("stale-first-id", "stale", false),
		catalogRootObserved("stale-last-id", "stale", false),
		catalogRootObserved("empty-name-id", "", false),
	)
	desired := []resources.CatalogServiceResource{
		catalogRootDesired("unrelated-ref", "kept", false),
		catalogRootDesired("blocked-ref", "blocked", true),
		catalogRootDesired("new-ref", "new", false),
	}
	desired[0].SetKonnectID("first-id")
	desired[1].Description = new("blocked change")
	// A cached ID and ref matching an observation must not turn this into an update.
	desired[2].Ref = "stale"
	desired[2].SetKonnectID("stale-last-id")
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	err := p.planCatalogServiceChanges(t.Context(), NewConfig("default"), desired, plan)
	require.ErrorContains(t, err, `catalog_service "blocked" is protected and cannot be updated`)
	require.ErrorContains(t, err, `catalog_service "protected-stale" is protected and cannot be deleted`)
	require.Len(t, plan.Changes, 3)
	require.Equal(t, ActionCreate, plan.Changes[0].Action)
	require.Equal(t, "new", plan.Changes[0].Fields[FieldName])
	var deleted []string
	for _, change := range plan.Changes[1:] {
		require.Equal(t, ActionDelete, change.Action)
		deleted = append(deleted, change.ResourceID)
	}
	// Map pruning order is intentionally unspecified; only the last duplicate
	// observation is eligible, and empty names remain in the index.
	require.ElementsMatch(t, []string{"stale-last-id", "empty-name-id"}, deleted)
}

func catalogRootPlanner(observed ...state.CatalogService) *Planner {
	p := NewPlanner(&state.Client{}, slog.Default())
	p.resourceCache.managedCatalogServices.byNamespaces[namespaceCacheKey([]string{"default"})] = observed
	p.matchedIdentities = make(map[string]matchedResourceIdentity)
	return p
}

func catalogRootObserved(id, name string, protected bool) state.CatalogService {
	meta := map[string]string{labels.NamespaceKey: "default"}
	if protected {
		meta[labels.ProtectedKey] = "true"
	}
	return state.CatalogService{
		CatalogService:   kkComps.CatalogService{ID: id, Name: name, DisplayName: name},
		NormalizedLabels: meta,
	}
}

func catalogRootDesired(ref, name string, protected bool) resources.CatalogServiceResource {
	return resources.CatalogServiceResource{
		BaseResource: resources.BaseResource{
			Ref: ref, Kongctl: &resources.KongctlMeta{Protected: new(protected)},
		},
		CreateCatalogService: kkComps.CreateCatalogService{Name: name, DisplayName: name},
	}
}
