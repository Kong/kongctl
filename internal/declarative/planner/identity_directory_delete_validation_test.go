package planner

import (
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestDirectoryDeleteAndSyncRequireObservableEmptyVaultDependencies(t *testing.T) {
	for _, mode := range []PlanMode{PlanModeDelete, PlanModeSync} {
		for _, attached := range []string{"", "Vault keys", "Vault providers", "Trusted IdPs"} {
			for _, enabled := range []bool{false, true} {
				api := &fixture.DirectoryAPI{Current: fixture.Directory("people", "identity-team", false)}
				api.Current.VaultEnabled = &enabled
				api.Directories = []kkComps.KongDirectory{api.Current}
				dependencies := &fixture.DirectoryDependenciesAPI{Attached: attached}
				client := state.NewClient(state.ClientConfig{
					IdentityDirectoriesAPI: api, IdentityDirectoryDependenciesAPI: dependencies,
				})
				rs := &resources.ResourceSet{DefaultNamespace: "identity-team"}
				if mode == PlanModeSync {
					rs.SyncScope = resources.NewSyncScope()
					rs.SyncScope.AddRoot(resources.ResourceTypeIdentityDirectory)
				}
				if mode == PlanModeDelete {
					rs.Identity = &resources.IdentityResource{Directories: []resources.IdentityDirectoryResource{directoryResource()}}
				}
				plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: mode})
				if enabled && attached == "" {
					require.NoError(t, err)
					require.Len(t, plan.Changes, 1)
					require.Equal(t, ActionDelete, plan.Changes[0].Action)
				} else {
					require.Error(t, err)
					require.Nil(t, plan, "do not return a usable deletion plan for unknown/nonempty dependencies")
				}
				require.Zero(t, api.Deletes)
				require.Zero(t, api.Replaces, "planning cannot enable Vault")
			}
		}
	}
}

func TestDirectoryObservationGuardDoesNotAffectNonDeletingPlans(t *testing.T) {
	api := &fixture.DirectoryAPI{Current: fixture.Directory("people", "identity-team", false)}
	api.Directories = []kkComps.KongDirectory{api.Current}
	dependencies := &fixture.DirectoryDependenciesAPI{Attached: "Vault keys"}
	rs := &resources.ResourceSet{Identity: &resources.IdentityResource{
		Directories: []resources.IdentityDirectoryResource{directoryResource()},
	}}
	client := state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: api, IdentityDirectoryDependenciesAPI: dependencies,
	})
	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	require.Empty(t, dependencies.Calls)
}
