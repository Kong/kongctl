package dump

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestIdentityDirectoryDumpRoundTrip(t *testing.T) {
	current := fixture.Directory(strings.Repeat("directory", 20), "identity-team", true)
	current.Labels["KONGCTL-namespace"] = "stale-label-must-not-own-directory"
	current.AllowedControlPlanes = []string{"4078f559-063b-4f10-9098-9e6553b21d9b"}
	api := &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{current}}
	dumpContext := &declarativeDumpContext{
		stateClient: state.NewClient(state.ClientConfig{IdentityDirectoriesAPI: api}),
	}
	rs := &resources.ResourceSet{}
	require.NoError(t, identityDirectoryCollector().collect(t.Context(), dumpContext, rs))
	require.Len(t, rs.Identity.Directories, 1)
	directory := rs.Identity.Directories[0]
	require.Equal(t, current.ID, directory.Ref, "valid long API names must not produce invalid local refs")
	require.True(t, directory.Protected())
	require.Equal(t, "identity-team", resources.GetNamespace(directory.Kongctl))
	require.NotContains(t, directory.Labels, "KONGCTL-namespace")
	require.Equal(t, current.Description, *directory.Description)
	require.Equal(t, current.AllowedControlPlanes, directory.AllowedControlPlanes)
	encoded, err := yaml.Marshal(rs)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "managed_by")
	path := filepath.Join(t.TempDir(), "dump.yaml")
	require.NoError(t, os.WriteFile(path, encoded, 0o600))
	loaded, err := loader.New().LoadFile(path)
	require.NoError(t, err)
	directory.Kongctl.NamespaceOrigin = loaded.Identity.Directories[0].Kongctl.NamespaceOrigin
	require.Equal(t, directory, loaded.Identity.Directories[0])
}

func TestUnmanagedDefaultDirectoryDumpRetainsManagedBasicAuthChildren(t *testing.T) {
	directory := fixture.Directory("default", "", false)
	const principalID = "4078f559-063b-4f10-9098-9e6553b21d9b"
	principal := kkComps.KongPrincipal{
		ID: principalID, DisplayName: new("Worker"),
		ManagedBy: resources.BuildPrincipalManagedBy("principals", "worker", true, "old"),
	}
	basic := fixture.BasicAuth("worker", "credentials", true)
	client := state.NewClient(state.ClientConfig{
		IdentityPrincipalKeysReader: &fixture.BasicAuthAPI{},
		IdentityDirectoriesAPI:      &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{directory}},
		IdentityPrincipalsAPI: &fixture.PrincipalAPI{
			Directories: map[string][]kkComps.KongPrincipal{directory.ID: {principal}},
		},
		IdentityBasicAuthAPI: &fixture.BasicAuthAPI{Parents: map[string][]helpers.IdentityBasicAuth{
			directory.ID + "/" + principalID: {basic},
		}}, IdentityGroupsAPI: &fixture.GroupAPI{}, IdentityPrincipalIdentitiesAPI: &fixture.PrincipalIdentitiesAPI{},
	})
	rs := &resources.ResourceSet{}
	require.NoError(t, identityDirectoryCollector().collect(t.Context(), &declarativeDumpContext{
		stateClient: client, includeChildren: true,
	}, rs))
	declaration := rs.Identity.Directories[0]
	require.True(t, declaration.IsExternal())
	require.Nil(t, declaration.Kongctl)
	require.Equal(t, "default", declaration.External.Selector.MatchFields[resources.SchemaFieldName])
	require.Equal(t, "credentials", resources.GetNamespace(declaration.Principals[0].BasicAuths[0].Kongctl))
	encoded, err := yaml.Marshal(rs)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "default.yaml")
	require.NoError(t, os.WriteFile(path, encoded, 0o600))
	loaded, err := loader.New().LoadFile(path)
	require.NoError(t, err)
	plan, err := planner.NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), loaded,
		planner.Options{Mode: planner.PlanModeApply})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
}

func TestIdentityPrincipalDumpPreservesRefsTypesAndIntegerMetadata(t *testing.T) {
	directory := fixture.Directory("people", "directory-team", false)
	api := &fixture.PrincipalAPI{Directories: map[string][]kkComps.KongPrincipal{directory.ID: {
		{
			ID: "1248f559-063b-4f10-9098-9e6553b21d9b", Type: new(kkComps.KongPrincipalTypeUser),
			DisplayName: new("Shared name"),
			ManagedBy:   resources.BuildPrincipalManagedBy("principal-team", "alice", true, "old"),
			Metadata: map[string]kkComps.KongIdentityMetadata{
				"large": kkComps.CreateKongIdentityMetadataInteger(9007199254740994),
			},
		},
		{
			ID: "2248f559-063b-4f10-9098-9e6553b21d9b", DisplayName: new("Shared name"),
			ManagedBy: resources.BuildPrincipalManagedBy("principal-team", "unclassified", false, "old"),
		},
	}}}
	d := &declarativeDumpContext{includeChildren: true, stateClient: state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI:         &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{directory}},
		IdentityPrincipalsAPI:          api,
		IdentityBasicAuthAPI:           &fixture.BasicAuthAPI{},
		IdentityPrincipalKeysReader:    &fixture.BasicAuthAPI{},
		IdentityGroupsAPI:              &fixture.GroupAPI{},
		IdentityPrincipalIdentitiesAPI: &fixture.PrincipalIdentitiesAPI{},
	})}
	rs := &resources.ResourceSet{}
	require.NoError(t, identityDirectoryCollector().collect(t.Context(), d, rs))
	encoded, err := yaml.Marshal(rs)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "9007199254740994")
	require.Contains(t, string(encoded), "type: null")
	require.NotContains(t, string(encoded), "active:")
	require.NotContains(t, string(encoded), "managed_by:")
	path := filepath.Join(t.TempDir(), "principals.yaml")
	require.NoError(t, os.WriteFile(path, encoded, 0o600))
	loaded, err := loader.New().LoadFile(path)
	require.NoError(t, err)
	require.Len(t, loaded.Identity.Principals, 2)
	require.Equal(t, "alice", loaded.Identity.Principals[0].Ref)
	require.True(t, loaded.Identity.Principals[0].Protected())
	require.Equal(t, "principal-team", resources.GetNamespace(loaded.Identity.Principals[0].Kongctl))
	require.True(t, loaded.Identity.Principals[1].TypeDeclared())
	require.Nil(t, loaded.Identity.Principals[1].Type)
}

func TestIdentityDumpKeepsImporterAdoptedPrincipalExternal(t *testing.T) {
	directory := fixture.Directory("people", "directory-team", false)
	const id = "1248f559-063b-4f10-9098-9e6553b21d9b"
	api := &fixture.PrincipalAPI{Directories: map[string][]kkComps.KongPrincipal{directory.ID: {{
		ID: id, ProvisionedBy: new(kkComps.ProvisionedBy("imported")),
		ManagedBy: resources.BuildPrincipalManagedBy("old-team", "former-ref", true, "dev"),
	}}}}
	keys := (&fixture.APIKeyHTTP{}).SDK()
	d := &declarativeDumpContext{includeChildren: true, stateClient: state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI:         &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{directory}},
		IdentityPrincipalsAPI:          api,
		IdentityPrincipalKeysAPI:       keys.GetIdentityPrincipalKeysAPI(),
		IdentityBasicAuthAPI:           &fixture.BasicAuthAPI{},
		IdentityGroupsAPI:              &fixture.GroupAPI{},
		IdentityPrincipalIdentitiesAPI: &fixture.PrincipalIdentitiesAPI{},
	})}
	rs := &resources.ResourceSet{}
	require.NoError(t, identityDirectoryCollector().collect(t.Context(), d, rs))
	principal := rs.Identity.Directories[0].Principals[0]
	require.Equal(t, id, principal.Ref)
	require.Equal(t, id, principal.External.ID)
	require.Nil(t, principal.Kongctl)
	encoded, err := yaml.Marshal(rs)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "imported.yaml")
	require.NoError(t, os.WriteFile(path, encoded, 0o600))
	loaded, err := loader.New().LoadFile(path)
	require.NoError(t, err)
	require.True(t, loaded.Identity.Principals[0].IsExternal())
}

func TestIdentityDumpKeepsUnownedAndWorkspaceAssociationsExternal(t *testing.T) {
	directory := fixture.Directory("people", "directory-team", false)
	const principalID = "1248f559-063b-4f10-9098-9e6553b21d9b"
	const ownedID = "2248f559-063b-4f10-9098-9e6553b21d9b"
	const unownedID = "3248f559-063b-4f10-9098-9e6553b21d9b"
	const workspaceID = "4248f559-063b-4f10-9098-9e6553b21d9b"
	identities := []kkComps.KongPrincipalIdentity{
		kkComps.CreateKongPrincipalIdentityCustom(kkComps.KongPrincipalIdentityCustomResponse{
			Type: kkComps.KongPrincipalIdentityCustomResponseTypeCustom,
			ID:   ownedID, Key: "employee", Value: "123",
			ManagedBy: resources.BuildPrincipalManagedBy("identity-team", "employee", true, "old"),
		}),
		kkComps.CreateKongPrincipalIdentityCustom(kkComps.KongPrincipalIdentityCustomResponse{
			Type: kkComps.KongPrincipalIdentityCustomResponseTypeCustom,
			ID:   unownedID, Key: "external", Value: "456",
		}),
		kkComps.CreateKongPrincipalIdentityControlPlaneConsumer(kkComps.KongPrincipalIdentityCPConsumerResponse{
			Type: kkComps.KongPrincipalIdentityCPConsumerResponseTypeControlPlaneConsumer,
			ID:   workspaceID, ControlPlaneID: directory.ID, ConsumerID: unownedID, WorkspaceID: new("inspected-workspace"),
			ManagedBy: resources.BuildPrincipalManagedBy("identity-team", "workspace-link", false, "old"),
		}),
	}
	d := &declarativeDumpContext{includeChildren: true, stateClient: state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{directory}},
		IdentityPrincipalsAPI: &fixture.PrincipalAPI{Directories: map[string][]kkComps.KongPrincipal{directory.ID: {
			{ID: principalID, Type: new(kkComps.KongPrincipalTypeUser)},
		}}},
		IdentityBasicAuthAPI:        &fixture.BasicAuthAPI{},
		IdentityPrincipalKeysReader: &fixture.BasicAuthAPI{},
		IdentityGroupsAPI:           &fixture.GroupAPI{},
		IdentityPrincipalIdentitiesAPI: &fixture.PrincipalIdentitiesAPI{
			Parents: map[string][]kkComps.KongPrincipalIdentity{directory.ID + "/" + principalID: identities},
		},
	})}
	rs := &resources.ResourceSet{}
	require.NoError(t, identityDirectoryCollector().collect(t.Context(), d, rs))
	p := rs.Identity.Directories[0].Principals[0]
	require.Equal(t, principalID, p.External.ID)
	require.Nil(t, p.Kongctl)
	require.Len(t, p.Identities, 3)
	byRef := make(map[string]resources.IdentityPrincipalIdentityResource)
	for _, identity := range p.Identities {
		byRef[identity.Ref] = identity
	}
	require.Nil(t, byRef["employee"].External)
	require.True(t, byRef["employee"].Protected())
	require.Equal(t, "identity-team", resources.GetNamespace(byRef["employee"].Kongctl))
	require.Equal(t, unownedID, byRef[unownedID].External.ID)
	require.Nil(t, byRef[unownedID].Kongctl)
	require.Equal(t, workspaceID, byRef["workspace-link"].External.ID)
	require.Nil(t, byRef["workspace-link"].Kongctl)
	require.Equal(t, "inspected-workspace", *byRef["workspace-link"].WorkspaceID)
	encoded, err := yaml.Marshal(rs)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "managed_by:")
	path := filepath.Join(t.TempDir(), "identities.yaml")
	require.NoError(t, os.WriteFile(path, encoded, 0o600))
	loaded, err := loader.New().LoadFile(path)
	require.NoError(t, err)
	require.True(t, loaded.Identity.Principals[0].IsExternal())
	require.Len(t, loaded.Identity.Identities, 3)
}

func TestPrincipalDumpPreservesExplicitFalseAndTrue(t *testing.T) {
	for _, active := range []bool{false, true} {
		directory := fixture.Directory("people", "directory-team", false)
		api := &fixture.PrincipalAPI{
			ExposeActive: true,
			Directories: map[string][]kkComps.KongPrincipal{directory.ID: {{
				ID: "1248f559-063b-4f10-9098-9e6553b21d9b", Active: active,
				ManagedBy: resources.BuildPrincipalManagedBy("principal-team", "alice", false, "old"),
			}}},
		}
		d := &declarativeDumpContext{includeChildren: true, stateClient: state.NewClient(state.ClientConfig{
			IdentityDirectoriesAPI:      &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{directory}},
			IdentityBasicAuthAPI:        &fixture.BasicAuthAPI{},
			IdentityPrincipalKeysReader: &fixture.BasicAuthAPI{},
			IdentityGroupsAPI:           &fixture.GroupAPI{},
			IdentityPrincipalsAPI:       api, IdentityPrincipalIdentitiesAPI: &fixture.PrincipalIdentitiesAPI{},
		})}
		rs := &resources.ResourceSet{}
		require.NoError(t, identityDirectoryCollector().collect(t.Context(), d, rs))
		require.Equal(t, new(active), rs.Identity.Directories[0].Principals[0].Active)
		encoded, err := yaml.Marshal(rs)
		require.NoError(t, err)
		path := filepath.Join(t.TempDir(), "principals.yaml")
		require.NoError(t, os.WriteFile(path, encoded, 0o600))
		loaded, err := loader.New().LoadFile(path)
		require.NoError(t, err)
		require.Equal(t, new(active), loaded.Identity.Principals[0].Active)
	}
}

func TestGroupDumpUsesSafeRefsAndKeepsUnsupportedGroupsExternal(t *testing.T) {
	directory := fixture.Directory("people", "directory-team", false)
	owned := fixture.Group("Fulfillment", "group-team", true)
	owned.Metadata["large"] = kkComps.CreateKongIdentityMetadataInteger(9007199254740994)
	team := fixture.Group("OrgTeam", "importer", false)
	team.Type = new(kkComps.KongGroupTypeTeam)
	api := &fixture.GroupAPI{Directories: map[string][]helpers.IdentityGroup{directory.ID: {owned, team}}}
	const principalID = "1248f559-063b-4f10-9098-9e6553b21d9b"
	api.Memberships = map[string][]kkComps.KongGroupMembershipEntry{owned.ID: {{
		ID: "2248f559-063b-4f10-9098-9e6553b21d9b",
		Member: kkComps.KongGroupMember{
			Type: kkComps.KongGroupMembershipTypePrincipal, ID: principalID,
		},
	}}}
	client := state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{directory}},
		IdentityPrincipalsAPI: &fixture.PrincipalAPI{Directories: map[string][]kkComps.KongPrincipal{directory.ID: {{
			ID: principalID, ManagedBy: resources.BuildPrincipalManagedBy("principal-team", "alice", false, "old"),
		}}}}, IdentityPrincipalIdentitiesAPI: &fixture.PrincipalIdentitiesAPI{},
		IdentityBasicAuthAPI:        &fixture.BasicAuthAPI{},
		IdentityPrincipalKeysReader: &fixture.BasicAuthAPI{},
		IdentityGroupsAPI:           api,
	})
	d := &declarativeDumpContext{includeChildren: true, stateClient: client}
	rs := &resources.ResourceSet{}
	require.NoError(t, identityDirectoryCollector().collect(t.Context(), d, rs))
	require.Len(t, rs.Identity.Directories[0].Groups, 2)
	encoded, err := yaml.Marshal(rs)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "managed_by:")
	require.Contains(t, string(encoded), "9007199254740994")
	path := filepath.Join(t.TempDir(), "groups.yaml")
	require.NoError(t, os.WriteFile(path, encoded, 0o600))
	loaded, err := loader.New().LoadFile(path)
	require.NoError(t, err)
	for _, group := range loaded.Identity.Groups {
		if group.Ref == owned.ID {
			require.Len(t, loaded.Identity.Memberships, 1)
			require.Equal(t, "__REF__:alice#id", loaded.Identity.Memberships[0].Member.ID)
			require.True(t, group.Protected())
			require.False(t, group.IsExternal())
			require.Equal(t, "group-team", resources.GetNamespace(group.Kongctl))
		} else {
			require.True(t, group.IsExternal())
			require.Nil(t, group.Kongctl)
		}
	}
	plan, err := planner.NewPlanner(client, slog.Default()).
		GeneratePlan(t.Context(), loaded, planner.Options{Mode: planner.PlanModeApply})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	// Managed principals must be re-creatable with fresh IDs. A membership
	// cannot keep the exported principal's old UUID in a new directory.
	fresh, err := loader.New().LoadFile(path)
	require.NoError(t, err)
	var managed []resources.IdentityGroupResource
	for _, group := range fresh.Identity.Groups {
		if !group.IsExternal() {
			managed = append(managed, group)
		}
	}
	fresh.Identity.Groups = managed
	empty := state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI:         &fixture.DirectoryAPI{},
		IdentityBasicAuthAPI:           &fixture.BasicAuthAPI{},
		IdentityPrincipalKeysReader:    &fixture.BasicAuthAPI{},
		IdentityGroupsAPI:              &fixture.GroupAPI{},
		IdentityPrincipalsAPI:          &fixture.PrincipalAPI{},
		IdentityPrincipalIdentitiesAPI: &fixture.PrincipalIdentitiesAPI{},
	})
	restored, err := planner.NewPlanner(empty, slog.Default()).
		GeneratePlan(t.Context(), fresh, planner.Options{Mode: planner.PlanModeApply})
	require.NoError(t, err)
	require.Len(t, restored.Changes, 4)
}
