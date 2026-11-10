package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestExpandSourcesAcceptsFragmentInputs(t *testing.T) {
	for _, fragment := range []string{"false\n", "- first\n- second\n", "[]\n"} {
		t.Run(strings.TrimSpace(fragment), func(t *testing.T) {
			dir := t.TempDir()
			writeExpandFile(t, dir, "fragments/value.yaml", fragment)
			writeExpandFile(t, dir, "config.yaml", "value: !file ./fragments/value.yaml\n")
			docs, err := New().ExpandSources(t.Context(), []Source{{Path: dir, Type: SourceTypeDirectory}}, true)
			require.NoError(t, err)
			require.Len(t, docs, 2)
			assert.False(t, docs[1].Empty, "a selected fragment must be linted, even an empty list or false")
			assert.Equal(t, filepath.Join(dir, "fragments/value.yaml"), docs[1].SourcePath)
		})
	}
}

func TestExpandSourcesReportsPreprocessingFailures(t *testing.T) {
	for _, tc := range []struct{ name, content, message string }{
		{"cycle", "_templates: {a: {_extends: b}, b: {_extends: a}}\nportals: [{_extends: a}]\n", "cycle"},
		{"invalid override", "portals: [{_extends: [one, two]}]\n", "must name one template"},
		{"missing file", "description: !file ./absent.md\n", "absent.md"},
		{"malformed YAML", "portals: [\n", "parse YAML"},
		{"unknown tag", "value: !unrecognized value\n", "unsupported YAML tag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			file := writeExpandFile(t, dir, "config.yaml", tc.content)
			_, err := New().ExpandSources(t.Context(), []Source{{Path: file, Type: SourceTypeFile}}, false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), file)
			assert.Contains(t, err.Error(), tc.message)
		})
	}
}

func TestExpandSourcesFileBoundariesAndTemplateOrigins(t *testing.T) {
	dir := t.TempDir()
	templates := writeExpandFile(t, dir, "templates/base.yaml", `
_templates:
  base:
    labels: !file ./labels.yaml
`)
	writeExpandFile(t, dir, "templates/labels.yaml", "owner: template-team\n")
	consumer := writeExpandFile(t, dir, "team/config.yaml", "portals: [{_extends: base}]\n")
	docs, err := New().ExpandSources(t.Context(), []Source{
		{Path: templates, Type: SourceTypeFile}, {Path: consumer, Type: SourceTypeFile},
	}, false)
	require.NoError(t, err)
	assert.Contains(t, string(docs[1].Content), "owner: template-team", "relative paths use the template's source")

	for _, tc := range []struct{ name, path string }{
		{"traversal", "../templates/labels.yaml"},
		{"absolute", filepath.Join(dir, "templates/labels.yaml")},
		{"symlink", "./outside.yaml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "symlink" {
				require.NoError(t, os.Symlink(filepath.Join(dir, "templates/labels.yaml"),
					filepath.Join(dir, "team/outside.yaml")))
			}
			file := writeExpandFile(t, dir, "team/boundary.yaml", "labels: !file "+tc.path+"\n")
			_, err := New().ExpandSources(t.Context(), []Source{{Path: file, Type: SourceTypeFile}}, false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), file)
			if tc.name == "absolute" {
				assert.Contains(t, err.Error(), "absolute paths are not allowed")
			} else {
				assert.Contains(t, err.Error(), "outside base dir")
			}
		})
	}

	// A directory input grants that directory as the root, including its children.
	directoryRoot := t.TempDir()
	writeExpandFile(t, directoryRoot, "templates/labels.yaml", "owner: directory-team\n")
	writeExpandFile(t, directoryRoot, "team/config.yaml", "labels: !file ../templates/labels.yaml\n")
	docs, err = New().ExpandSources(t.Context(), []Source{{Path: directoryRoot, Type: SourceTypeDirectory}}, true)
	require.NoError(t, err)
	require.Len(t, docs, 2)
	assert.Contains(t, string(docs[0].Content), "owner: directory-team")
}

func TestExpandSourcesPreservesDeferredTagsWithoutResourceValidation(t *testing.T) {
	const sensitive = "must-not-be-read-during-expansion"
	t.Setenv("KONGCTL_EXPAND_DEFERRED", sensitive)
	dir := t.TempDir()
	writeExpandFile(t, dir, "secret.txt", sensitive)
	file := writeExpandFile(t, dir, "config.yaml", `
_templates:
  deferred:
    authentication_enabled: !env KONGCTL_EXPAND_DEFERRED
    reference: !ref absent-resource
    lookup: !lookup {name: !env KONGCTL_EXPAND_DEFERRED}
    external: !external name:unavailable
    secret: !secret {source: !env KONGCTL_EXPAND_DEFERRED}
    file_secret: !secret {source: !file ./secret.txt}
    stored: !env {var: KONGCTL_EXPAND_DEFERRED, store: true, type: boolean}
portals:
  - _extends: deferred
  - authentication_enabled: !env KONGCTL_EXPAND_DEFERRED
`)
	docs, err := New().ExpandSources(t.Context(), []Source{{Path: file, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.NotContains(t, string(docs[0].Content), sensitive)
	assert.Contains(t, string(docs[0].Content), "!env", "stored env declarations remain deferred too")
	var root struct{ Portals []map[string]any }
	require.NoError(t, yaml.Unmarshal(docs[0].Content, &root))
	require.Len(t, root.Portals, 2)
	portal := root.Portals[0]
	assert.True(t, tags.IsEnvPlaceholder(portal["authentication_enabled"].(string)))
	assert.True(t, tags.IsRefPlaceholder(portal["reference"].(string)))
	assert.True(t, tags.IsSecretPlaceholder(portal["secret"].(string)))
	assert.True(t, tags.IsSecretPlaceholder(portal["file_secret"].(string)))
	_, ok := tags.ParseExternalPlaceholder(portal["lookup"].(string))
	assert.True(t, ok)
	_, ok = tags.ParseExternalPlaceholder(portal["external"].(string))
	assert.True(t, ok)
}

func TestExpandSourcesRejectsMultipleYAMLDocuments(t *testing.T) {
	dir := t.TempDir()
	file := writeExpandFile(t, dir, "multiple.yaml", "first: valid\n---\nsecond: must-not-be-ignored\n")
	_, err := New().ExpandSources(t.Context(), []Source{{Path: file, Type: SourceTypeFile}}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), file)
	assert.Contains(t, err.Error(), "one YAML document")
}

func TestExpandSourcesDoesNotApplyDefaultsOrValidateResources(t *testing.T) {
	dir := t.TempDir()
	file := writeExpandFile(t, dir, "config.yaml", "portals:\n  - ref: missing-required-name\n")
	docs, err := New().ExpandSources(t.Context(), []Source{{Path: file, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	var expanded map[string]any
	require.NoError(t, yaml.Unmarshal(docs[0].Content, &expanded))
	assert.Equal(t, map[string]any{"portals": []any{map[string]any{"ref": "missing-required-name"}}}, expanded)
	loaded, err := New().LoadFromSources([]Source{{Path: file, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	assert.Equal(t, "missing-required-name", loaded.Portals[0].Name, "full loading still applies legacy defaults")
	invalid := writeExpandFile(t, dir, "invalid.yaml", "portals:\n  - unrecognized_field: true\n")
	_, err = New().ExpandSources(t.Context(), []Source{{Path: invalid, Type: SourceTypeFile}}, false)
	require.NoError(t, err, "lint must allow rules to check resources that do not pass schema validation")
	_, err = New().LoadFromSources([]Source{{Path: invalid, Type: SourceTypeFile}}, false)
	require.Error(t, err, "full loading must still validate resources")
}
