package extensions

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseGitHubSource(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		ref       string
		wantOK    bool
		wantOwner string
		wantRepo  string
		wantRef   string
		wantErr   string
	}{
		{
			name:      "owner repo",
			source:    "kong/kongctl-ext-debug",
			ref:       "v1.0.0",
			wantOK:    true,
			wantOwner: "kong",
			wantRepo:  "kongctl-ext-debug",
			wantRef:   "v1.0.0",
		},
		{
			name:      "inline ref",
			source:    "kong/kongctl-ext-debug@v1.2.3",
			wantOK:    true,
			wantOwner: "kong",
			wantRepo:  "kongctl-ext-debug",
			wantRef:   "v1.2.3",
		},
		{
			name:      "https URL",
			source:    "https://github.com/Kong/kongctl-ext-debug.git",
			wantOK:    true,
			wantOwner: "Kong",
			wantRepo:  "kongctl-ext-debug",
		},
		{
			name:      "ssh URL",
			source:    "git@github.com:kong/kongctl-ext-debug.git",
			wantOK:    true,
			wantOwner: "kong",
			wantRepo:  "kongctl-ext-debug",
		},
		{
			name:   "local path",
			source: "./extensions/debug",
			wantOK: false,
		},
		{
			name:    "invalid owner",
			source:  "bad_owner/repo",
			wantOK:  true,
			wantErr: "invalid GitHub source",
		},
		{
			name:    "inline and flag ref",
			source:  "kong/kongctl-ext-debug@v1.2.3",
			ref:     "v1.0.0",
			wantOK:  true,
			wantErr: "use either --ref or @ref",
		},
		{
			name:   "too many path segments",
			source: "kong/team/repo",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, ok, err := ParseGitHubSource(tt.source, tt.ref)

			require.Equal(t, tt.wantOK, ok)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantOwner, source.Owner)
			require.Equal(t, tt.wantRepo, source.Repo)
			require.Equal(t, tt.wantRef, source.Ref)
		})
	}
}

func TestFetchGitHubSourcePrefersReleaseAsset(t *testing.T) {
	archive := testReleaseTarGzip(t)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/kong/kongctl-ext-foo/releases/latest":
			require.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
			require.NotEmpty(t, r.Header.Get("User-Agent"))
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(githubRelease{
				TagName: "v1.2.3",
				Assets: []githubReleaseAsset{
					{
						Name:        "kongctl-ext-foo.tar.gz",
						DownloadURL: server.URL + "/downloads/kongctl-ext-foo.tar.gz",
					},
				},
			}))
		case "/downloads/kongctl-ext-foo.tar.gz":
			require.Equal(t, "application/octet-stream", r.Header.Get("Accept"))
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(archive)
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	previousAPIBaseURL := githubAPIBaseURL
	previousHTTPClient := githubHTTPClient
	githubAPIBaseURL = server.URL
	githubHTTPClient = server.Client()
	t.Cleanup(func() {
		githubAPIBaseURL = previousAPIBaseURL
		githubHTTPClient = previousHTTPClient
	})

	fetched, err := FetchGitHubSource(context.Background(), GitHubSource{
		Owner: "kong",
		Repo:  "kongctl-ext-foo",
	}, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(fetched.Cleanup)

	require.Equal(t, SourceTypeGitHubReleaseAsset, fetched.SourceType)
	require.Equal(t, "kong/kongctl-ext-foo", fetched.Repository)
	require.Equal(t, "v1.2.3", fetched.Ref)
	require.Equal(t, "v1.2.3", fetched.ReleaseTag)
	require.Equal(t, "kongctl-ext-foo.tar.gz", fetched.AssetName)
	require.FileExists(t, filepath.Join(fetched.Dir, ManifestFileName))

	runtimeInfo, err := os.Stat(filepath.Join(fetched.Dir, "bin", "kongctl-ext-foo"))
	require.NoError(t, err)
	require.NotZero(t, runtimeInfo.Mode().Perm()&0o111)
}

func TestSelectGitHubReleaseAssetRequiresUnambiguousArchive(t *testing.T) {
	_, err := selectGitHubReleaseAsset([]githubReleaseAsset{
		{Name: "one.tar.gz", DownloadURL: "https://example.test/one.tar.gz"},
		{Name: "two.zip", DownloadURL: "https://example.test/two.zip"},
	})
	require.ErrorContains(t, err, "multiple release archive assets found")
}

func TestSelectGitHubReleaseAssetPrefersCurrentPlatform(t *testing.T) {
	otherOS := "linux"
	if runtime.GOOS == otherOS {
		otherOS = "darwin"
	}
	otherArch := "amd64"
	if runtime.GOARCH == otherArch {
		otherArch = "arm64"
	}

	asset, err := selectGitHubReleaseAsset([]githubReleaseAsset{
		{
			Name:        "kongctl-ext-foo-" + otherOS + "-" + otherArch + ".tar.gz",
			DownloadURL: "https://example.test/other.tar.gz",
		},
		{
			Name:        "kongctl-ext-foo-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz",
			DownloadURL: "https://example.test/current.tar.gz",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://example.test/current.tar.gz", asset.DownloadURL)
}

func TestExtractGitHubReleaseArchiveRejectsUnsafeZipPath(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "bad.zip")
	writeTestZipArchive(t, archivePath, []testZipEntry{
		{Name: "../extension.yaml", Body: "nope", Mode: 0o644},
	})

	err := extractGitHubReleaseArchive(archivePath, "bad.zip", t.TempDir())

	require.ErrorContains(t, err, "parent-directory marker")
}

func TestExtractGitHubReleaseArchiveRejectsUnsafeTarPath(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "bad.tar.gz")
	writeTestTarGzipArchive(t, archivePath, []testTarEntry{
		{Name: "nested/../extension.yaml", Body: "nope", Mode: 0o644},
	})

	err := extractGitHubReleaseArchive(archivePath, "bad.tar.gz", t.TempDir())

	require.ErrorContains(t, err, "parent-directory marker")
}

func testReleaseTarGzip(t *testing.T) []byte {
	t.Helper()

	var archive bytes.Buffer
	gzipWriter := gzip.NewWriter(&archive)
	tarWriter := tar.NewWriter(gzipWriter)
	entries := []struct {
		name string
		body string
		mode int64
	}{
		{
			name: ManifestFileName,
			mode: 0o644,
			body: `schema_version: 1
publisher: kong
name: foo
version: 1.2.3
runtime:
  command: bin/kongctl-ext-foo
command_paths:
  - path:
      - name: get
      - name: foo
`,
		},
		{
			name: "bin/kongctl-ext-foo",
			mode: 0o755,
			body: "#!/bin/sh\necho ok\n",
		},
	}
	for _, entry := range entries {
		require.NoError(t, tarWriter.WriteHeader(&tar.Header{
			Name: entry.name,
			Mode: entry.mode,
			Size: int64(len(entry.body)),
		}))
		_, err := tarWriter.Write([]byte(entry.body))
		require.NoError(t, err)
	}
	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())
	return archive.Bytes()
}

type testZipEntry struct {
	Name string
	Body string
	Mode os.FileMode
}

func writeTestZipArchive(t *testing.T, archivePath string, entries []testZipEntry) {
	t.Helper()

	file, err := os.Create(archivePath)
	require.NoError(t, err)
	defer file.Close()

	writer := zip.NewWriter(file)
	for _, entry := range entries {
		header := &zip.FileHeader{
			Name:   entry.Name,
			Method: zip.Deflate,
		}
		header.SetMode(entry.Mode)
		zipEntry, err := writer.CreateHeader(header)
		require.NoError(t, err)
		_, err = zipEntry.Write([]byte(entry.Body))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
}

type testTarEntry struct {
	Name string
	Body string
	Mode int64
}

func writeTestTarGzipArchive(t *testing.T, archivePath string, entries []testTarEntry) {
	t.Helper()

	file, err := os.Create(archivePath)
	require.NoError(t, err)
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		require.NoError(t, tarWriter.WriteHeader(&tar.Header{
			Name: entry.Name,
			Mode: entry.Mode,
			Size: int64(len(entry.Body)),
		}))
		_, err := tarWriter.Write([]byte(entry.Body))
		require.NoError(t, err)
	}
	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())
}
