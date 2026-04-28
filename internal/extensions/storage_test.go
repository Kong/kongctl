package extensions

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStoreInstallLocalCopiesPackageAndRecordsHashes(t *testing.T) {
	source := writeTestExtension(t)
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
	require.Equal(t, SourceTypeLocalPath, installed.Install.Source.Type)
	require.FileExists(t, filepath.Join(installed.PackageDir, ManifestFileName))

	runtimePath, err := store.ResolveRuntime(installed)
	require.NoError(t, err)
	require.FileExists(t, runtimePath)
}

func TestStoreLinkLocalRefreshesManifestFromSource(t *testing.T) {
	source := writeTestExtension(t)
	store := NewStore(filepath.Join(t.TempDir(), "extensions"))

	linked, err := store.LinkLocal(source, "test-version", time.Unix(100, 0))
	require.NoError(t, err)
	require.Equal(t, InstallTypeLinked, linked.InstallType)

	writeManifest(t, source, "kong", "foo", "list", "foo")
	reloaded, err := store.Get("kong/foo")
	require.NoError(t, err)
	require.Equal(t, "list foo", CommandPathString(reloaded.CommandPaths[0]))
}

func TestStoreInstallGitHubSourceRecordsRemoteProvenance(t *testing.T) {
	source := writeTestExtension(t)
	store := NewStore(filepath.Join(t.TempDir(), "extensions"))
	fetched := FetchedGitHubSource{
		SourceType:     SourceTypeGitHubSource,
		Repository:     "kong/kongctl-ext-foo",
		URL:            "https://github.com/kong/kongctl-ext-foo.git",
		Ref:            "v1.0.0",
		ResolvedCommit: "abc123",
	}

	result, err := store.InstallGitHubSource(source, fetched, "test-version", time.Unix(100, 0), true)

	require.NoError(t, err)
	require.Equal(t, "kong/foo", result.Extension.ID)
	installed, err := store.Get("kong/foo")
	require.NoError(t, err)
	require.Equal(t, SourceTypeGitHubSource, installed.Install.Source.Type)
	require.Empty(t, installed.Install.Source.Path)
	require.Equal(t, fetched.Repository, installed.Install.Source.Repository)
	require.Equal(t, fetched.URL, installed.Install.Source.URL)
	require.Equal(t, fetched.Ref, installed.Install.Source.Ref)
	require.Equal(t, fetched.ResolvedCommit, installed.Install.Source.ResolvedCommit)
	require.True(t, installed.Install.Trust.Confirmed)
	require.Equal(t, "github_source_clone", installed.Install.Trust.Model)
	require.Equal(t, "explicit_ref", installed.Install.Upgrade.Policy)
}

func TestStoreInstallGitHubReleaseAssetRecordsRemoteProvenance(t *testing.T) {
	source := writeTestExtension(t)
	store := NewStore(filepath.Join(t.TempDir(), "extensions"))
	fetched := FetchedGitHubSource{
		SourceType: SourceTypeGitHubReleaseAsset,
		Repository: "kong/kongctl-ext-foo",
		URL:        "https://github.com/kong/kongctl-ext-foo",
		Ref:        "v1.0.0",
		ReleaseTag: "v1.0.0",
		AssetName:  "kongctl-ext-foo-linux-amd64.tar.gz",
		AssetURL:   "https://github.com/kong/kongctl-ext-foo/releases/download/v1.0.0/kongctl-ext-foo-linux-amd64.tar.gz",
	}

	result, err := store.InstallGitHubSource(source, fetched, "test-version", time.Unix(100, 0), true)

	require.NoError(t, err)
	require.Equal(t, "kong/foo", result.Extension.ID)
	installed, err := store.Get("kong/foo")
	require.NoError(t, err)
	require.Equal(t, SourceTypeGitHubReleaseAsset, installed.Install.Source.Type)
	require.Equal(t, fetched.Repository, installed.Install.Source.Repository)
	require.Equal(t, fetched.URL, installed.Install.Source.URL)
	require.Equal(t, fetched.Ref, installed.Install.Source.Ref)
	require.Equal(t, fetched.ReleaseTag, installed.Install.Source.ReleaseTag)
	require.Equal(t, fetched.AssetName, installed.Install.Source.AssetName)
	require.Equal(t, fetched.AssetURL, installed.Install.Source.AssetURL)
	require.Empty(t, installed.Install.Source.ResolvedCommit)
	require.True(t, installed.Install.Trust.Confirmed)
	require.Equal(t, "github_release_asset", installed.Install.Trust.Model)
	require.Equal(t, "github_release", installed.Install.Upgrade.Policy)
}

func TestStoreUninstallPreservesDataByDefault(t *testing.T) {
	source := writeTestExtension(t)
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

func writeTestExtension(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeManifest(t, root, "kong", "foo", "get", "foo")
	runtime := filepath.Join(root, "kongctl-ext-foo")
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
