package executor

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestSavedIdentityDeletesRecheckLateRegistryAssociationsAndObservationFailures(t *testing.T) {
	const principal = "5078f559-063b-4f10-9098-9e6553b21d9b"
	const agent = "6078f559-063b-4f10-9098-9e6553b21d9b"
	const association = "7078f559-063b-4f10-9098-9e6553b21d9b"
	for _, kind := range []string{planner.ResourceTypeIdentityDirectory, planner.ResourceTypeIdentityPrincipal} {
		for _, scenario := range []string{"empty", "linked", "failed-agents", "failed-links", "unconfigured"} {
			t.Run(kind+"/"+scenario, func(t *testing.T) {
				root := fixture.Directory("people", "identity-team", false)
				root.VaultEnabled = new(true)
				roots := &fixture.DirectoryAPI{Current: root}
				_, principals, execCtx := principalExecutorFixture()
				current := principals.Directories["directory-id"][0]
				current.ID = principal
				principals.Directories = map[string][]kkComps.KongPrincipal{root.ID: {current}}
				api := &fixture.RegistryReferencesAPI{}
				if scenario != "empty" {
					api.Agents = []string{agent}
					api.Links = map[string][]helpers.RegistryAgentPrincipalReference{agent: {{
						ID: association, DirectoryID: root.ID, PrincipalID: principal,
					}}}
				}
				if scenario == "failed-agents" {
					api.Failure = "agents"
				}
				if scenario == "failed-links" {
					api.Failure = "links"
				}
				config := state.ClientConfig{
					IdentityDirectoriesAPI: roots, IdentityPrincipalsAPI: principals,
					IdentityDirectoryDependenciesAPI: &fixture.DirectoryDependenciesAPI{},
					IdentityPrincipalIdentitiesAPI:   &fixture.PrincipalIdentitiesAPI{},
					IdentityBasicAuthAPI:             &fixture.BasicAuthAPI{}, IdentityPrincipalKeysReader: &fixture.BasicAuthAPI{},
					IdentityGroupsAPI: &fixture.GroupAPI{}, RegistryAgentReferencesReader: api,
				}
				if scenario == "unconfigured" {
					config.RegistryAgentReferencesReader = nil
				}
				client := state.NewClient(config)
				var err error
				if kind == planner.ResourceTypeIdentityDirectory {
					// The referenced principal need not still exist for its directory
					// to be a target of the stored association.
					change := directorySavedChange(t, map[string]any{planner.FieldName: "people"})
					err = (&IdentityDirectoryAdapter{client: client}).Delete(t.Context(), root.ID, NewExecutionContext(change))
				} else {
					execCtx.PlannedChange.Action = planner.ActionDelete
					execCtx.PlannedChange.ResourceID = principal
					execCtx.PlannedChange.Parent.ID = root.ID
					err = (&IdentityPrincipalAdapter{client: client}).Delete(t.Context(), principal, execCtx)
				}
				if scenario == "empty" {
					require.NoError(t, err)
					require.Equal(t, 1, roots.Deletes+principals.Deletes)
				} else {
					require.Error(t, err)
					require.Zero(t, roots.Deletes+principals.Deletes)
				}
			})
		}
	}
}
