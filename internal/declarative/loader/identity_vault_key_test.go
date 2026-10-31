package loader

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestVaultKeysRequireExplicitPublicConfigurationAndManagedDirectory(t *testing.T) {
	data, err := os.ReadFile("../../konnect/identitykeys/testdata/public-keys.json")
	require.NoError(t, err)
	var fixtures []map[string]any
	require.NoError(t, json.Unmarshal(data, &fixtures))
	for _, grouped := range []bool{false, true} {
		for _, invalid := range []string{"", "name", "public_key", "private", "duplicate", "external", "kongctl"} {
			t.Run(invalid+map[bool]string{false: "/nested", true: "/grouped"}[grouped], func(t *testing.T) {
				parent := map[string]any{
					"ref": "people", "name": "default", "allow_all_control_planes": true,
					"vault_enabled": true,
				}
				key := map[string]any{
					"ref":        "encryption",
					"name":       "explicit-name",
					"public_key": fixtures[0]["public_key"],
				}
				switch invalid {
				case "name", "public_key":
					delete(key, invalid)
				case "private":
					key["public_key"] = "-----BEGIN PRIVATE KEY-----\nsentinel\n-----END PRIVATE KEY-----"
				case "external":
					parent["_external"] = map[string]any{
						"selector": map[string]any{"matchFields": map[string]any{"name": "default"}},
					}
				case "kongctl":
					key["kongctl"] = map[string]any{"namespace": "independent"}
				}
				keys := []any{key}
				if invalid == "duplicate" {
					keys = append(
						keys,
						map[string]any{
							"ref":        "same-material",
							"name":       "different-name",
							"public_key": fixtures[0]["public_key"],
						},
					)
				}
				identity := map[string]any{"directories": []any{parent}}
				if grouped {
					for _, value := range keys {
						value.(map[string]any)["directory"] = "people"
					}
					identity["vault_keys"] = keys
				} else {
					parent["vault_keys"] = keys
				}
				document, err := json.Marshal(map[string]any{"identity": identity})
				require.NoError(t, err)
				rs, err := New().LoadFile(writeLoaderTestFile(t, string(document)))
				if invalid != "" {
					require.Error(t, err)
					require.NotContains(t, err.Error(), "sentinel")
					return
				}
				require.NoError(t, err)
				require.Len(t, rs.Identity.VaultKeys, 1)
				require.Equal(t, "people", rs.Identity.VaultKeys[0].Directory)
				require.Equal(t, fixtures[0]["kid"], rs.Identity.VaultKeys[0].GetMoniker())
				require.True(
					t,
					rs.SyncScope.ChildInScope(
						resources.ResourceTypeIdentityDirectory,
						"people",
						resources.ResourceTypeIdentityVaultKey,
					),
				)
			})
		}
	}
	for _, presence := range []string{"", ",\"vault_keys\":[]"} {
		input := `{"identity":{"directories":[{"ref":"people","name":"default",` +
			`"allow_all_control_planes":true` + presence + `}]}}`
		rs, err := New().LoadFile(writeLoaderTestFile(t, input))
		require.NoError(t, err)
		require.Equal(
			t,
			presence != "",
			rs.SyncScope.ChildInScope(
				resources.ResourceTypeIdentityDirectory,
				"people",
				resources.ResourceTypeIdentityVaultKey,
			),
		)
	}
}
