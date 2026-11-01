package loader

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func principalCSVBytes(t *testing.T, records ...[]string) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	require.NoError(t, writer.WriteAll(records))
	return output.Bytes()
}

func TestPrincipalCSVLoadsTypedNestedResourcesAndLiteralValues(t *testing.T) {
	base := t.TempDir()
	cells := principalCSVBytes(
		t,
		[]string{
			"ref",
			"display_name",
			"description",
			"type",
			"active",
			"metadata",
			"labels",
			"kongctl",
			"identities",
			"api_keys",
			"basic_auths",
		},
		[]string{
			"worker", "__REF__:other#id", "A CSV worker", "agent", "false",
			`{"large":9007199254740994,"enabled":true,"roles":["a","b"],"__REF__:key":"literal","literal":"__REF__:other#id"}`,
			`{"application":"orders"}`, `{"namespace":"csv-parents"}`,
			`[{"ref":"worker-external","type":"custom","key":"external","value":"csv-worker"}]`,
			`[{"ref":"worker-key","type":"v1","name":"Worker primary","expires_at":null}]`,
			`[{"ref":"worker-login","username":"csv-worker","kongctl":{"namespace":"csv-logins"}}]`,
		},
	)
	require.NoError(t, os.WriteFile(filepath.Join(base, "principals.csv"), cells, 0o600))
	manifest := `_defaults:
  kongctl: {namespace: csv-demo}
identity:
  directories:
    - ref: people
      _external: {id: 4078f559-063b-4f10-9098-9e6553b21d9b}
      principals: !csv {path: principals.csv, metadata_mode: merge}
`
	path := filepath.Join(base, "manifest.yaml")
	require.NoError(t, os.WriteFile(path, []byte(manifest), 0o600))
	rs, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.Identity.Principals, 1)
	principal := rs.Identity.Principals[0]
	require.Equal(t, "people", principal.Directory)
	require.Equal(t, "__REF__:other#id", *principal.DisplayName)
	require.Equal(t, "csv-parents", resources.GetNamespace(principal.Kongctl))
	require.Equal(t, resources.IdentityMetadataModeMerge, principal.MetadataMode)
	require.NotNil(t, principal.Active)
	require.False(t, *principal.Active)
	require.Equal(t, json.Number("9007199254740994"), principal.Metadata["large"])
	require.Equal(t, "literal", principal.Metadata["__REF__:key"])
	require.Equal(t, "__REF__:other#id", principal.Metadata["literal"])
	require.Len(t, rs.Identity.Identities, 1)
	require.Equal(t, "worker", rs.Identity.Identities[0].Principal)
	require.Len(t, rs.Identity.PrincipalAPIKeys, 1)
	require.True(t, rs.Identity.PrincipalAPIKeys[0].ExpirationDeclared())
	require.Nil(t, rs.Identity.PrincipalAPIKeys[0].ExpiresAt)
	require.Len(t, rs.Identity.BasicAuths, 1)
	require.Equal(t, "csv-logins", resources.GetNamespace(rs.Identity.BasicAuths[0].Kongctl))
	require.True(
		t,
		rs.SyncScope.ChildInScope(
			resources.ResourceTypeIdentityPrincipal,
			"worker",
			resources.ResourceTypeIdentityPrincipalAPIKey,
		),
	)
}

func TestPrincipalCSVScopesDistinguishBlankFromExplicitEmptyCells(t *testing.T) {
	for _, cell := range []string{"", "[]"} {
		base := t.TempDir()
		data := principalCSVBytes(t, []string{"ref", "directory", "api_keys"}, []string{"worker", "people", cell})
		require.NoError(t, os.WriteFile(filepath.Join(base, "rows.csv"), data, 0o600))
		manifest := `identity:
  directories:
    - ref: people
      _external: {id: 4078f559-063b-4f10-9098-9e6553b21d9b}
  principals: !csv {path: rows.csv, metadata_mode: sync}
`
		path := filepath.Join(base, "manifest.yaml")
		require.NoError(t, os.WriteFile(path, []byte(manifest), 0o600))
		rs, err := New().LoadFile(path)
		require.NoError(t, err)
		require.Equal(
			t,
			cell != "",
			rs.SyncScope.ChildInScope(
				resources.ResourceTypeIdentityPrincipal,
				"worker",
				resources.ResourceTypeIdentityPrincipalAPIKey,
			),
		)
	}
	rows, err := parsePrincipalCSV([]byte("ref,metadata\n"), resources.IdentityMetadataModeMerge)
	require.NoError(t, err)
	require.NotNil(t, rows)
	require.Empty(t, rows)
}

func TestPrincipalCSVRejectsInvalidInputWithoutEchoingValues(t *testing.T) {
	const private = "private-input-value-withheld"
	for _, sample := range []struct {
		name    string
		records [][]string
	}{
		{"header", [][]string{{"ref", private}}},
		{"repeated-header", [][]string{{"ref", "ref"}}},
		{"missing-ref", [][]string{{"display_name"}, {private}}},
		{"bad-ref", [][]string{{"ref"}, {"bad ref " + private}}},
		{"duplicate-ref", [][]string{{"ref"}, {"worker"}, {"worker"}}},
		{"duplicate-id", [][]string{
			{"ref", "id"},
			{"worker", "5078f559-063b-4f10-9098-9e6553b21d9b"},
			{"reader", "5078f559-063b-4f10-9098-9e6553b21d9b"},
		}},
		{"bad-id", [][]string{{"ref", "id"}, {"worker", private}}},
		{"bad-active", [][]string{{"ref", "active"}, {"worker", private}}},
		{"bad-type", [][]string{{"ref", "type"}, {"worker", private}}},
		{"bad-json", [][]string{{"ref", "labels"}, {"worker", private}}},
		{"metadata-null", [][]string{{"ref", "metadata"}, {"worker", "null"}}},
		{"metadata-object-value", [][]string{{"ref", "metadata"}, {"worker", `{"bad":{"value":"` + private + `"}}`}}},
		{"duplicate-json-key", [][]string{{"ref", "metadata"}, {"worker", `{"team":"a","team":"` + private + `"}`}}},
		{"bad-expiration", [][]string{
			{"ref", "api_keys"},
			{"worker", `[{"ref":"primary","type":"v1","expires_at":"` + private + `"}]`},
		}},
		{"imported-key", [][]string{
			{"ref", "api_keys"},
			{"worker", `[{"ref":"primary","type":"imported","secret":"` + private + `"}]`},
		}},
		{"password", [][]string{
			{"ref", "basic_auths"},
			{"worker", `[{"ref":"login","username":"worker","password":"` + private + `"}]`},
		}},
		{"environment-encoding", [][]string{{"ref", "display_name"}, {"worker", "__ENV__:" + private}}},
		{"secret-encoding", [][]string{{"ref", "metadata"}, {"worker", `{"public":"__SECRET__:` + private + `"}`}}},
		{"template-control", [][]string{{"ref", "metadata"}, {"worker", `{"_extends":"` + private + `"}`}}},
		{"child-parent", [][]string{
			{"ref", "api_keys"},
			{"worker", `[{"ref":"primary","type":"v1","principal":"other"}]`},
		}},
	} {
		t.Run(sample.name, func(t *testing.T) {
			_, err := parsePrincipalCSV(principalCSVBytes(t, sample.records...), resources.IdentityMetadataModeMerge)
			require.Error(t, err)
			require.NotContains(t, err.Error(), private)
		})
	}
	for _, data := range [][]byte{
		[]byte("ref\n\"unclosed"), []byte("ref,active\nworker,false,extra"), []byte("ref\nworker\x00"), {0xff, 0xfe},
	} {
		_, err := parsePrincipalCSV(data, resources.IdentityMetadataModeMerge)
		require.Error(t, err)
	}
}

func TestPrincipalCSVRejectsConflictingNestedDirectory(t *testing.T) {
	base := t.TempDir()
	data := principalCSVBytes(t, []string{"ref", "directory"}, []string{"worker", "other"})
	require.NoError(t, os.WriteFile(filepath.Join(base, "rows.csv"), data, 0o600))
	manifest := `identity:
  directories:
    - ref: people
      _external: {id: 4078f559-063b-4f10-9098-9e6553b21d9b}
      principals: !csv {path: rows.csv, metadata_mode: merge}
`
	path := filepath.Join(base, "manifest.yaml")
	require.NoError(t, os.WriteFile(path, []byte(manifest), 0o600))
	_, err := New().LoadFile(path)
	require.ErrorContains(t, err, "selector conflicts with the enclosing directory")
	require.NotContains(t, err.Error(), `"other"`)
	_, err = InspectPrincipalCSV(base, resources.IdentityMetadataModeMerge)
	require.ErrorContains(t, err, "regular file")
}

func TestPrincipalCSVPreservesFileBoundaryAndExplicitOptions(t *testing.T) {
	base := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(base, "rows.csv"), []byte("ref\nworker\n"), 0o600))
	for _, source := range []string{
		`!csv rows.csv`, `!csv {path: rows.csv}`, `!csv {path: rows.csv, metadata_mode: invalid}`,
		`!csv {path: rows.csv, metadata_mode: merge, extra: value}`,
		`!csv {path: https://example.test/rows.csv, metadata_mode: merge}`,
		`!csv {path: ../rows.csv, metadata_mode: merge}`,
	} {
		manifest := "identity:\n  principals: " + source + "\n"
		path := filepath.Join(base, "manifest.yaml")
		require.NoError(t, os.WriteFile(path, []byte(manifest), 0o600))
		_, err := New().LoadFile(path)
		require.Error(t, err)
	}
	outside := filepath.Join(t.TempDir(), "outside.csv")
	require.NoError(t, os.WriteFile(outside, []byte("ref\nworker\n"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(base, "escape.csv")))
	manifest := "identity:\n  principals: !csv {path: escape.csv, metadata_mode: merge}\n"
	path := filepath.Join(base, "manifest.yaml")
	require.NoError(t, os.WriteFile(path, []byte(manifest), 0o600))
	_, err := New().LoadFile(path)
	require.ErrorContains(t, err, "outside")
	manifest = strings.Replace(manifest, "principals:", "groups:", 1)
	require.NoError(t, os.WriteFile(path, []byte(manifest), 0o600))
	_, err = New().LoadFile(path)
	require.ErrorContains(t, err, "only on an identity principal collection")
}
