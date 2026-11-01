package planner

import (
	"log/slog"
	"testing"

	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestIdentityDeletionRequiresGlobalRegistryReferenceInventory(t *testing.T) {
	const directory = "4078f559-063b-4f10-9098-9e6553b21d9b"
	const principal = "5078f559-063b-4f10-9098-9e6553b21d9b"
	const agent = "6078f559-063b-4f10-9098-9e6553b21d9b"
	const association = "7078f559-063b-4f10-9098-9e6553b21d9b"
	for _, target := range []string{ResourceTypeIdentityPrincipal, ResourceTypeIdentityDirectory} {
		for _, scenario := range []string{"empty", "linked", "other-directory", "other-principal", "failed", "unconfigured"} {
			t.Run(target+"/"+scenario, func(t *testing.T) {
				api := &fixture.RegistryReferencesAPI{}
				link := helpers.RegistryAgentPrincipalReference{ID: association, DirectoryID: directory, PrincipalID: principal}
				if scenario == "other-directory" {
					link.DirectoryID = association
				}
				if scenario == "other-principal" {
					link.PrincipalID = association
				}
				if scenario != "empty" {
					api.Agents, api.Links = []string{agent}, map[string][]helpers.RegistryAgentPrincipalReference{agent: {link}}
				}
				if scenario == "failed" {
					api.Failure = "links"
				}
				config := state.ClientConfig{RegistryAgentReferencesReader: api}
				if scenario == "unconfigured" {
					config.RegistryAgentReferencesReader = nil
				}
				p := NewPlanner(state.NewClient(config), slog.Default())
				change := PlannedChange{
					ResourceType: target, ResourceRef: "retiring", Action: ActionDelete,
					ResourceID: directory, Namespace: "selected-scope",
				}
				if target == ResourceTypeIdentityPrincipal {
					change.ResourceID, change.Parent = principal, &ParentInfo{ID: directory}
				}
				// Neither Registry ownership nor a Registry declaration is present.
				err := p.validateIdentityRegistryReferences(t.Context(), &Plan{Changes: []PlannedChange{change, change}})
				allowed := scenario == "empty" || scenario == "other-directory" ||
					scenario == "other-principal" && target == ResourceTypeIdentityPrincipal
				if allowed {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
				if scenario == "empty" {
					require.Equal(t, 1, api.Calls, "planning shares one complete inventory")
				}
			})
		}
	}
}

func TestRegistryReferenceRequirementDoesNotAffectNonDeletingIdentityOrOtherResources(t *testing.T) {
	for _, change := range []PlannedChange{
		{ResourceType: ResourceTypeIdentityPrincipal, Action: ActionCreate},
		{ResourceType: ResourceTypeIdentityDirectory, Action: ActionUpdate},
		{ResourceType: ResourceTypeIdentityGroup, Action: ActionDelete},
		{ResourceType: ResourceTypeIdentityAuthServer, Action: ActionDelete},
	} {
		api := &fixture.RegistryReferencesAPI{Failure: "agents"}
		p := NewPlanner(state.NewClient(state.ClientConfig{RegistryAgentReferencesReader: api}), slog.Default())
		require.NoError(t, p.validateIdentityRegistryReferences(t.Context(), &Plan{Changes: []PlannedChange{change}}))
		require.Zero(t, api.Calls)
	}
}
