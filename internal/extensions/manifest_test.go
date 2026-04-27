package extensions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseManifestMinimal(t *testing.T) {
	manifest, err := ParseManifest([]byte(`
schema_version: 1
publisher: Kong
name: Foo
runtime:
  command: bin/kongctl-ext-foo
command_paths:
  - path:
      - name: get
      - name: foo
        aliases: [foos]
`))

	require.NoError(t, err)
	require.Equal(t, "kong", manifest.Publisher)
	require.Equal(t, "foo", manifest.Name)
	require.Equal(t, "bin/kongctl-ext-foo", manifest.Runtime.Command)
	require.Equal(t, "kong_foo_get_foo", manifest.CommandPaths[0].ID)
	require.Equal(t, "kongctl get foo [args] [flags]", manifest.CommandPaths[0].Usage)
	require.Equal(t, "Run kong/foo extension command", manifest.CommandPaths[0].Summary)
}

func TestParseManifestRejectsUnknownTopLevelKey(t *testing.T) {
	_, err := ParseManifest([]byte(`
schema_version: 1
publisher: kong
name: foo
runtime:
  command: kongctl-ext-foo
unexpected: true
command_paths:
  - path:
      - name: get
      - name: foo
`))

	require.ErrorContains(t, err, "unknown top-level key")
}

func TestParseManifestRejectsYAMLAlias(t *testing.T) {
	_, err := ParseManifest([]byte(`
schema_version: 1
publisher: kong
name: foo
runtime:
  command: kongctl-ext-foo
command_paths:
  - path:
      - &verb
        name: get
      - *verb
`))

	require.ErrorContains(t, err, "aliases or anchors")
}

func TestParseManifestRejectsClosedBuiltInRoot(t *testing.T) {
	_, err := ParseManifest([]byte(`
schema_version: 1
publisher: kong
name: foo
runtime:
  command: kongctl-ext-foo
command_paths:
  - path:
      - name: install
      - name: foo
`))

	require.ErrorContains(t, err, "closed to extension contributions")
}

func TestParseManifestRejectsBuiltInRootAliases(t *testing.T) {
	_, err := ParseManifest([]byte(`
schema_version: 1
publisher: kong
name: foo
runtime:
  command: kongctl-ext-foo
command_paths:
  - path:
      - name: get
        aliases: [g]
      - name: foo
`))

	require.ErrorContains(t, err, "cannot declare aliases")
}

func TestParseManifestRejectsOversizedManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), ManifestFileName)
	data := []byte("schema_version: 1\n" + strings.Repeat("#", MaxManifestBytes))
	require.NoError(t, os.WriteFile(path, data, 0o600))

	_, _, err := LoadManifestFile(path)

	require.Error(t, err)
}
