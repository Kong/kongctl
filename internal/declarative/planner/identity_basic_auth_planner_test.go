package planner

import (
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

const basicPrincipal = "4078f559-063b-4f10-9098-9e6553b21d9b"

func basicAuthPlan(t *testing.T, api *fixture.BasicAuthAPI, desired []resources.IdentityBasicAuthResource,
	mode PlanMode, scoped bool,
) (*Plan, error) {
	t.Helper()
	directory := fixture.Directory("people", "directories", true)
	rs := &resources.ResourceSet{DefaultNamespace: "credentials", Identity: &resources.IdentityResource{
		Directories: []resources.IdentityDirectoryResource{{
			BaseResource: resources.BaseResource{Ref: "people"},
			External:     &resources.ExternalBlock{ID: directory.ID},
		}},
		Principals: []resources.IdentityPrincipalResource{{
			BaseResource: resources.BaseResource{Ref: "worker"},
			Directory:    "people", External: &resources.ExternalBlock{ID: basicPrincipal},
		}}, BasicAuths: desired,
	}, SyncScope: resources.NewSyncScope()}
	rs.SyncScope.AddRoot(resources.ResourceTypeIdentityDirectory)
	if scoped {
		rs.SyncScope.AddChildNamespace(resources.ResourceTypeIdentityBasicAuth, "worker", "credentials")
	}
	return NewPlanner(state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{directory}},
		IdentityPrincipalsAPI: &fixture.PrincipalAPI{Directories: map[string][]kkComps.KongPrincipal{
			directory.ID: {{ID: basicPrincipal}},
		}}, IdentityBasicAuthAPI: api,
	}), slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: mode})
}

func TestBasicAuthNameIdentityAndScopedPruning(t *testing.T) {
	key := fixture.Directory("people", "directories", true).ID + "/" + basicPrincipal
	basic := fixture.BasicAuth("worker@example.test", "credentials", false)
	foreign := fixture.BasicAuth("foreign", "other", false)
	api := &fixture.BasicAuthAPI{Parents: map[string][]helpers.IdentityBasicAuth{key: {basic, foreign}}}
	desired := resources.IdentityBasicAuthResource{BaseResource: resources.BaseResource{
		Ref: "local-login", Kongctl: &resources.KongctlMeta{Namespace: new("credentials")},
	}, Principal: "worker", Username: basic.Username}
	plan, err := basicAuthPlan(t, api, []resources.IdentityBasicAuthResource{desired}, PlanModeApply, true)
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	for _, scoped := range []bool{false, true} {
		plan, err = basicAuthPlan(t, api, nil, PlanModeSync, scoped)
		require.NoError(t, err)
		if scoped {
			require.Len(t, plan.Changes, 1)
			require.Equal(t, basic.ID, plan.Changes[0].ResourceID)
		} else {
			require.Empty(t, plan.Changes)
		}
	}
	desired.Kongctl.Protected = new(true)
	plan, err = basicAuthPlan(t, api, []resources.IdentityBasicAuthResource{desired}, PlanModeApply, true)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, basic.ID, plan.Changes[0].ResourceID)
	require.Len(t, plan.Changes[0].Fields[FieldManagedBy], 4)
	api.Parents[key][0].ManagedBy = nil
	_, err = basicAuthPlan(t, api, []resources.IdentityBasicAuthResource{desired}, PlanModeApply, true)
	require.ErrorContains(t, err, "adopt")
}

func TestBasicAuthDeletionBlocksPasswordsAndIncompleteObservations(t *testing.T) {
	key := fixture.Directory("people", "directories", true).ID + "/" + basicPrincipal
	for _, fault := range []string{"password", "password-list", "list", "duplicate", "protected"} {
		t.Run(fault, func(t *testing.T) {
			basic := fixture.BasicAuth("worker", "credentials", fault == "protected")
			api := &fixture.BasicAuthAPI{Parents: map[string][]helpers.IdentityBasicAuth{key: {basic}}}
			switch fault {
			case "password":
				api.Passwords = map[string][]helpers.IdentityPassword{key + "/" + basic.ID: {{ID: basicPrincipal}}}
			case "duplicate":
				other := basic
				other.ID = basicPrincipal
				api.Parents[key] = append(api.Parents[key], other)
			default:
				api.Failure = fault
			}
			_, err := basicAuthPlan(t, api, nil, PlanModeSync, true)
			require.Error(t, err)
			require.Zero(t, api.Deletes)
		})
	}
}
