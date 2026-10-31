package dump

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
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

func TestIdentityPrincipalDumpPreservesRefsTypesAndIntegerMetadata(t *testing.T) {
	directory := fixture.Directory("people", "directory-team", false)
	api := &fixture.PrincipalAPI{Directories: map[string][]kkComps.KongPrincipal{directory.ID: {
		{
			ID: "1248f559-063b-4f10-9098-9e6553b21d9b", Type: new(kkComps.KongPrincipalTypeUser),
			DisplayName: new("Shared name"),
			ManagedBy:   resources.BuildPrincipalManagedBy("principal-team", "alice", true, "old"),
			Metadata: map[string]kkComps.KongIdentityMetadata{
				"large": kkComps.CreateKongIdentityMetadataInteger(9007199254740993),
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
		IdentityPrincipalIdentitiesAPI: &fixture.PrincipalIdentitiesAPI{},
	})}
	rs := &resources.ResourceSet{}
	require.NoError(t, identityDirectoryCollector().collect(t.Context(), d, rs))
	encoded, err := yaml.Marshal(rs)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "9007199254740993")
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

func TestIdentityDumpKeepsUnownedAndWorkspaceAssociationsExternal(t *testing.T) {
	directory := fixture.Directory("people", "directory-team", false)
	const principalID = "1248f559-063b-4f10-9098-9e6553b21d9b"
	const ownedID = "2248f559-063b-4f10-9098-9e6553b21d9b"
	const unownedID = "3248f559-063b-4f10-9098-9e6553b21d9b"
	const workspaceID = "4248f559-063b-4f10-9098-9e6553b21d9b"
	identities := []kkComps.KongPrincipalIdentity{
		kkComps.CreateKongPrincipalIdentityCustom(kkComps.KongPrincipalIdentityCustomResponse{
			ID: ownedID, Key: "employee", Value: "123",
			ManagedBy: resources.BuildPrincipalManagedBy("identity-team", "employee", true, "old"),
		}),
		kkComps.CreateKongPrincipalIdentityCustom(kkComps.KongPrincipalIdentityCustomResponse{
			ID: unownedID, Key: "external", Value: "456",
		}),
		kkComps.CreateKongPrincipalIdentityControlPlaneConsumer(kkComps.KongPrincipalIdentityCPConsumerResponse{
			ID: workspaceID, ControlPlaneID: directory.ID, ConsumerID: unownedID, WorkspaceID: new("inspected-workspace"),
			ManagedBy: resources.BuildPrincipalManagedBy("identity-team", "workspace-link", false, "old"),
		}),
	}
	d := &declarativeDumpContext{includeChildren: true, stateClient: state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{directory}},
		IdentityPrincipalsAPI: &fixture.PrincipalAPI{Directories: map[string][]kkComps.KongPrincipal{directory.ID: {
			{ID: principalID, Type: new(kkComps.KongPrincipalTypeUser)},
		}}},
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
