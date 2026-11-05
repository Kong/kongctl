package executor

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func directorySavedChange(t *testing.T, fields map[string]any) *planner.PlannedChange {
	t.Helper()
	change := planner.PlannedChange{
		ResourceType: planner.ResourceTypeIdentityDirectory,
		ResourceID:   "7a5e1b58-48a4-4586-b02d-0d91eccca152", Namespace: "identity-team", Fields: fields, Protection: false,
	}
	encoded, err := json.Marshal(change)
	require.NoError(t, err)
	var saved planner.PlannedChange
	require.NoError(t, json.Unmarshal(encoded, &saved))
	return &saved
}

func TestIdentityDirectorySavedUpdatesPreserveFreshOmittedFields(t *testing.T) {
	for _, clearCollections := range []bool{false, true} {
		api := &fixture.DirectoryAPI{Current: fixture.Directory("people", "identity-team", false)}
		api.Current.AllowedControlPlanes = []string{"gateway-id"}
		adapter := &IdentityDirectoryAdapter{client: state.NewClient(state.ClientConfig{IdentityDirectoriesAPI: api})}
		fields := map[string]any{
			planner.FieldName: "people", planner.FieldDescription: "new description",
			planner.FieldManagedBy: resources.BuildChildManagedBy("identity-team", false, "stale-version"),
		}
		if clearCollections {
			fields[planner.FieldLabels] = map[string]string{}
			fields[planner.FieldAllowedControlPlanes] = []any{}
		}
		change := directorySavedChange(t, fields)
		ctx := NewExecutionContext(change)
		var request kkComps.ReplaceDirectoryBody
		require.NoError(t, adapter.MapUpdateFields(t.Context(), ctx, change.Fields, &request, nil))
		_, err := adapter.Update(t.Context(), change.ResourceID, request, "identity-team", ctx)
		require.NoError(t, err)
		require.Equal(t, 1, api.Replaces)
		require.Equal(t, "new description", *api.Replaced.Description)
		require.Equal(t, int64(900), *api.Replaced.TTLSecs)
		require.Equal(t, int64(450), *api.Replaced.NegativeTTLSecs)
		require.True(t, *api.Replaced.AllowAllControlPlanes)
		require.False(t, *api.Replaced.VaultEnabled)
		if clearCollections {
			require.Empty(t, api.Replaced.Labels)
			require.Empty(t, api.Replaced.AllowedControlPlanes)
		} else {
			require.Equal(t, api.Current.Labels, api.Replaced.Labels)
			require.Equal(t, api.Current.AllowedControlPlanes, api.Replaced.AllowedControlPlanes)
		}
		require.Equal(t, resources.BuildChildManagedBy("identity-team", false, "dev"), api.Replaced.ManagedBy)
	}
}

func TestIdentityDirectoryRejectsChangedOwnershipAndProtection(t *testing.T) {
	for _, namespace := range []string{"identity-team", "foreign", ""} {
		for _, protected := range []bool{false, true} {
			api := &fixture.DirectoryAPI{Current: fixture.Directory("people", namespace, protected)}
			api.Current.VaultEnabled = new(true)
			adapter := &IdentityDirectoryAdapter{client: state.NewClient(state.ClientConfig{
				IdentityDirectoriesAPI: api, IdentityDirectoryDependenciesAPI: &fixture.DirectoryDependenciesAPI{},
			})}
			change := directorySavedChange(t, map[string]any{
				planner.FieldName: "people", planner.FieldDescription: "changed",
				planner.FieldManagedBy: resources.BuildChildManagedBy("identity-team", false, "dev"),
			})
			ctx := NewExecutionContext(change)
			var request kkComps.ReplaceDirectoryBody
			require.NoError(t, adapter.MapUpdateFields(t.Context(), ctx, change.Fields, &request, nil))
			_, err := adapter.Update(t.Context(), change.ResourceID, request, "identity-team", ctx)
			deleteErr := adapter.Delete(t.Context(), change.ResourceID, ctx)
			if namespace == "identity-team" && !protected {
				require.NoError(t, err)
				require.NoError(t, deleteErr)
				require.Nil(t, api.Force, "directory deletion must never request a cascade")
			} else {
				require.Error(t, err)
				require.Error(t, deleteErr)
				require.Zero(t, api.Replaces)
				require.Zero(t, api.Deletes)
			}
		}
	}
}

func TestIdentityDirectorySavedDeletionReobservesVaultChildren(t *testing.T) {
	for _, attached := range []string{"", "Vault keys", "Vault providers", "Trusted IdPs"} {
		t.Run(attached, func(t *testing.T) {
			api := &fixture.DirectoryAPI{Current: fixture.Directory("people", "identity-team", false)}
			api.Current.VaultEnabled = new(true)
			dependencies := &fixture.DirectoryDependenciesAPI{Attached: attached}
			adapter := &IdentityDirectoryAdapter{client: state.NewClient(state.ClientConfig{
				IdentityDirectoriesAPI: api, IdentityDirectoryDependenciesAPI: dependencies,
			})}
			change := directorySavedChange(t, map[string]any{planner.FieldName: "people"})
			err := adapter.Delete(t.Context(), change.ResourceID, NewExecutionContext(change))
			if attached == "" {
				require.NoError(t, err)
				require.Equal(t, 1, api.Deletes)
				require.Len(t, dependencies.Calls, 3)
			} else {
				require.ErrorContains(t, err, attached)
				require.Zero(t, api.Deletes)
			}
		})
	}
	for _, value := range []*bool{nil, new(false)} {
		api := &fixture.DirectoryAPI{Current: fixture.Directory("people", "identity-team", false)}
		api.Current.VaultEnabled = value
		dependencies := &fixture.DirectoryDependenciesAPI{}
		adapter := &IdentityDirectoryAdapter{client: state.NewClient(state.ClientConfig{
			IdentityDirectoriesAPI: api, IdentityDirectoryDependenciesAPI: dependencies,
		})}
		change := directorySavedChange(t, map[string]any{planner.FieldName: "people"})
		err := adapter.Delete(t.Context(), change.ResourceID, NewExecutionContext(change))
		require.ErrorContains(t, err, "explicitly enable Vault")
		require.Empty(t, dependencies.Calls)
		require.Zero(t, api.Deletes)
		require.Zero(t, api.Replaces, "observations must not implicitly enable Vault")
	}
}

func TestIdentityDirectoryProtectedUnprotectCannotHidePayloadChanges(t *testing.T) {
	for _, injected := range []bool{false, true} {
		api := &fixture.DirectoryAPI{Current: fixture.Directory("people", "identity-team", true)}
		api.Current.ManagedBy["outside"] = "removed-on-write"
		adapter := &IdentityDirectoryAdapter{client: state.NewClient(state.ClientConfig{IdentityDirectoriesAPI: api})}
		metadata := resources.BuildChildManagedBy("identity-team", false, "dev")
		change := directorySavedChange(t, map[string]any{planner.FieldName: "people", planner.FieldManagedBy: metadata})
		change.Protection = planner.ProtectionChange{Old: true, New: false}
		change.ChangedFields = map[string]planner.FieldChange{
			planner.FieldManagedBy: {Old: api.Current.ManagedBy, New: metadata},
		}
		if injected {
			change.Fields[planner.FieldDescription] = "injected"
		}
		ctx := NewExecutionContext(change)
		var request kkComps.ReplaceDirectoryBody
		require.NoError(t, adapter.MapUpdateFields(t.Context(), ctx, change.Fields, &request, nil))
		_, err := adapter.Update(t.Context(), change.ResourceID, request, "identity-team", ctx)
		if injected {
			require.ErrorContains(t, err, "protected")
			require.Zero(t, api.Replaces)
		} else {
			require.NoError(t, err)
			require.Equal(t, api.Current.Description, *api.Replaced.Description)
			require.NotContains(t, api.Replaced.ManagedBy, "outside")
		}
	}
}
