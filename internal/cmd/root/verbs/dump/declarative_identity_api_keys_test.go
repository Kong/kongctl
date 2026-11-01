package dump

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestGeneratedAPIKeyDumpRoundTripUsesParentAuthorityAndCreationMarkers(t *testing.T) {
	directory := fixture.Directory("default", "", false)
	principal := kkComps.KongPrincipal{
		ID: "5078f559-063b-4f10-9098-9e6553b21d9b", Type: new(kkComps.KongPrincipalTypeUser),
		ManagedBy: resources.BuildPrincipalManagedBy("principal-team", "worker", false, "old"),
	}
	group := fixture.Group("Operators", "group-team", false)
	api := &fixture.APIKeyHTTP{PageSize: 1, Keys: make(map[string][]helpers.IdentityAPIKey)}
	for i, parent := range []struct{ kind, id, ref string }{
		{"principals", principal.ID, "worker-key"}, {"groups", group.ID, "operators-key"},
	} {
		path := "/v2/directories/" + directory.ID + "/" + parent.kind + "/" + parent.id + "/api-keys"
		key := helpers.IdentityAPIKey{
			ID: "6078f559-063b-4f10-9098-9e6553b21d9b", Name: new("Shared display name"),
			ManagedBy: map[string]string{
				resources.ManagedByToolKey: resources.ManagedByTool, resources.ManagedByVersionKey: "creator",
				resources.ManagedByRefKey: parent.ref, resources.IdentityAPIKeyCreateTypeKey: "v1",
			}, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		}
		if i == 1 {
			key.ExpiresAt = new(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
		}
		foreign := key
		foreign.ID, foreign.ManagedBy = "7078f559-063b-4f10-9098-9e6553b21d9b", map[string]string{}
		api.Keys[path] = []helpers.IdentityAPIKey{key, foreign}
	}
	sdk := api.SDK()
	client := state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{directory}},
		IdentityPrincipalsAPI: &fixture.PrincipalAPI{
			Directories: map[string][]kkComps.KongPrincipal{directory.ID: {principal}},
		},
		IdentityGroupsAPI: &fixture.GroupAPI{
			Directories: map[string][]helpers.IdentityGroup{directory.ID: {group}},
		},
		IdentityPrincipalIdentitiesAPI: &fixture.PrincipalIdentitiesAPI{}, IdentityBasicAuthAPI: &fixture.BasicAuthAPI{},
		IdentityPrincipalKeysAPI: sdk.GetIdentityPrincipalKeysAPI(), IdentityGroupKeysAPI: sdk.GetIdentityGroupKeysAPI(),
	})
	rs := &resources.ResourceSet{}
	require.NoError(t, identityDirectoryCollector().collect(t.Context(), &declarativeDumpContext{
		stateClient: client, includeChildren: true,
	}, rs))
	require.Len(t, rs.Identity.Directories, 1)
	declaration := rs.Identity.Directories[0]
	require.True(t, declaration.IsExternal())
	require.Len(t, declaration.Principals[0].APIKeys, 1)
	require.Len(t, declaration.Groups[0].APIKeys, 1)
	require.Equal(t, "worker-key", declaration.Principals[0].APIKeys[0].Ref)
	require.Equal(t, "v1", declaration.Principals[0].APIKeys[0].Type)
	require.True(t, declaration.Principals[0].APIKeys[0].ExpirationDeclared())
	require.Nil(t, declaration.Principals[0].APIKeys[0].ExpiresAt)
	data, err := yaml.Marshal(rs)
	require.NoError(t, err)
	require.NotContains(t, string(data), "managed_by")
	require.NotContains(t, string(data), "secret")
	require.Contains(t, string(data), "expires_at: null")
	path := filepath.Join(t.TempDir(), "dump.yaml")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	loaded, err := loader.New().LoadFile(path)
	require.NoError(t, err)
	plan, err := planner.NewPlanner(client, slog.Default()).
		GeneratePlan(t.Context(), loaded, planner.Options{Mode: planner.PlanModeApply})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
}
