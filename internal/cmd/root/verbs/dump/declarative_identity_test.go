package dump

import (
	"os"
	"path/filepath"
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
	current := fixture.Directory("default", "identity-team", true)
	current.Labels["KONGCTL-namespace"] = "stale-label-must-not-own-directory"
	current.AllowedControlPlanes = []string{"4078f559-063b-4f10-9098-9e6553b21d9b"}
	api := &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{current}}
	dumpContext := &declarativeDumpContext{stateClient: state.NewClient(state.ClientConfig{IdentityDirectoriesAPI: api})}
	rs := &resources.ResourceSet{}
	require.NoError(t, identityDirectoryCollector().collect(t.Context(), dumpContext, rs))
	require.Len(t, rs.Identity.Directories, 1)
	directory := rs.Identity.Directories[0]
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
