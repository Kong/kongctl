package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImportedPrincipalAPIKeysRequireDeferredDeclarationsWithoutReadingSources(t *testing.T) {
	for _, source := range []string{
		"!secret {source: !env UNAVAILABLE_IMPORTED_KEY}",
		`!secret {parts: ["prefix-", !env UNAVAILABLE_IMPORTED_KEY]}`,
	} {
		input := strings.Replace(generatedKeyManifest, "              type: v1",
			"              type: imported\n              secret: "+source, 1)
		rs, err := New().LoadFile(writeLoaderTestFile(t, input))
		require.NoError(t, err)
		require.Len(t, rs.GetSecretSources("worker-primary"), 1)
		require.False(t, rs.GetSecretSources("worker-primary")["/secret"].DeprecatedBareEnv)
	}
	for _, source := range []string{
		"private-material", `""`, "null", "!env UNAVAILABLE_IMPORTED_KEY", "{vault://store/key}",
	} {
		input := strings.Replace(generatedKeyManifest, "              type: v1",
			"              type: imported\n              secret: "+source, 1)
		_, err := New().LoadFile(writeLoaderTestFile(t, input))
		require.Error(t, err)
		require.NotContains(t, err.Error(), "private-material")
	}
	for _, source := range []string{"null", "!secret {source: !env UNAVAILABLE_IMPORTED_KEY}"} {
		input := strings.Replace(generatedKeyManifest, "name: Worker primary", "secret: "+source, 1)
		_, err := New().LoadFile(writeLoaderTestFile(t, input))
		require.ErrorContains(t, err, "generated API keys cannot declare secret")
	}
}

func TestImportedPrincipalAPIKeyFileAndTopLevelSourcesStayDeferred(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "key"), []byte("private-file-material"), 0o600))
	input := strings.Replace(generatedKeyManifest, "      type: v1\n  group_api_keys:",
		"      type: imported\n      secret: !secret {source: !file ./key}\n  group_api_keys:", 1)
	file := filepath.Join(dir, "manifest.yaml")
	require.NoError(t, os.WriteFile(file, []byte(input), 0o600))
	rs, err := New().LoadFile(file)
	require.NoError(t, err)
	source := rs.GetSecretSources("worker-secondary")["/secret"].Expression.Parts[0].Source
	require.Equal(t, "file", source.Kind)
	data, err := json.Marshal(rs)
	require.NoError(t, err)
	require.NotContains(t, string(data), "private-file-material")
}
