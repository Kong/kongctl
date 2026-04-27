package extensions

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStoreInstallLocalCopiesPackageAndRecordsHashes(t *testing.T) {
	source := writeTestExtension(t, "kong", "foo", "get", "foo")
	store := NewStore(filepath.Join(t.TempDir(), "extensions"))

	result, err := store.InstallLocal(source, "test-version", time.Unix(100, 0))

	require.NoError(t, err)
	require.Equal(t, "kong/foo", result.Extension.ID)
	require.NotEmpty(t, result.ManifestHash)
	require.NotEmpty(t, result.RuntimeHash)
	require.NotEmpty(t, result.PackageHash)

	installed, err := store.Get("kong/foo")
	require.NoError(t, err)
	require.Equal(t, InstallTypeInstalled, installed.InstallType)
	require.Equal(t, "test-version", installed.Install.CLIVersion)
	require.Equal(t, "local_path", installed.Install.Source.Type)
	require.FileExists(t, filepath.Join(installed.PackageDir, ManifestFileName))

	runtimePath, err := store.ResolveRuntime(installed)
	require.NoError(t, err)
	require.FileExists(t, runtimePath)
}

func TestStoreLinkLocalRefreshesManifestFromSource(t *testing.T) {
	source := writeTestExtension(t, "kong", "foo", "get", "foo")
	store := NewStore(filepath.Join(t.TempDir(), "extensions"))

	linked, err := store.LinkLocal(source, "test-version", time.Unix(100, 0))
	require.NoError(t, err)
	require.Equal(t, InstallTypeLinked, linked.InstallType)

	writeManifest(t, source, "kong", "foo", "list", "foo")
	reloaded, err := store.Get("kong/foo")
	require.NoError(t, err)
	require.Equal(t, "list foo", CommandPathString(reloaded.CommandPaths[0]))
}

func TestStoreUninstallPreservesDataByDefault(t *testing.T) {
	source := writeTestExtension(t, "kong", "foo", "get", "foo")
	store := NewStore(filepath.Join(t.TempDir(), "extensions"))
	_, err := store.InstallLocal(source, "test-version", time.Unix(100, 0))
	require.NoError(t, err)
	dataDir, err := store.DataDir("kong/foo")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dataDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "state.json"), []byte("{}"), 0o600))

	result, err := store.Uninstall("kong/foo", false)

	require.NoError(t, err)
	require.True(t, result.RemovedInstall)
	require.False(t, result.RemovedData)
	require.FileExists(t, filepath.Join(dataDir, "state.json"))
}

func writeTestExtension(t *testing.T, publisher, name, verb, resource string) string {
	t.Helper()
	root := t.TempDir()
	writeManifest(t, root, publisher, name, verb, resource)
	runtime := filepath.Join(root, "kongctl-ext-"+name)
	require.NoError(t, os.WriteFile(runtime, []byte("#!/bin/sh\necho ok\n"), 0o600))
	require.NoError(t, os.Chmod(runtime, 0o755))
	return root
}

func writeManifest(t *testing.T, root, publisher, name, verb, resource string) {
	t.Helper()
	manifest := []byte(`schema_version: 1
publisher: ` + publisher + `
name: ` + name + `
version: 0.1.0
runtime:
  command: kongctl-ext-` + name + `
command_paths:
  - path:
      - name: ` + verb + `
      - name: ` + resource + `
`)
	require.NoError(t, os.WriteFile(filepath.Join(root, ManifestFileName), manifest, 0o600))
}
