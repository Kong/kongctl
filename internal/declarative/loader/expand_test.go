package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeExpandFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestExpandSourcesExpandsTemplatesAcrossFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	templates := writeExpandFile(t, dir, "templates.yaml", `
_templates:
  standard-portal:
    authentication_enabled: true
    labels:
      managed-by: kongctl
`)
	team := writeExpandFile(t, dir, "team.yaml", `
portals:
  - ref: team-portal
    name: Team Portal
    _extends: standard-portal
    labels:
      team: a
`)

	docs, err := New().ExpandSources(t.Context(), []Source{
		{Path: templates, Type: SourceTypeFile},
		{Path: team, Type: SourceTypeFile},
	}, false)
	require.NoError(t, err)
	require.Len(t, docs, 2)

	assert.Equal(t, templates, docs[0].SourcePath)
	assert.True(t, docs[0].Empty)

	assert.Equal(t, team, docs[1].SourcePath)
	assert.False(t, docs[1].Empty)
	content := string(docs[1].Content)
	assert.Contains(t, content, "authentication_enabled: true")
	assert.Contains(t, content, "managed-by: kongctl")
	assert.Contains(t, content, "team: a")
	assert.NotContains(t, content, "_extends")
	assert.NotContains(t, content, "_templates")
}

func TestExpandSourcesIncludesFileTags(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeExpandFile(t, dir, "fragments/labels.yaml", "owner: platform\n")
	file := writeExpandFile(t, dir, "config.yaml", `
portals:
  - ref: team-portal
    name: Team Portal
    labels: !file ./fragments/labels.yaml
`)

	docs, err := New().ExpandSources(t.Context(), []Source{{Path: file, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Contains(t, string(docs[0].Content), "owner: platform")
}

func TestExpandSourcesDoesNotReadEnvironmentValues(t *testing.T) {
	const value = "value-that-must-not-appear"
	t.Setenv("KONGCTL_EXPAND_TEST_VALUE", value)

	dir := t.TempDir()
	file := writeExpandFile(t, dir, "config.yaml", `
portals:
  - ref: team-portal
    name: !env KONGCTL_EXPAND_TEST_VALUE
`)

	docs, err := New().ExpandSources(t.Context(), []Source{{Path: file, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.NotContains(t, string(docs[0].Content), value)
}
