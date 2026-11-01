package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestVaultCredentialNestedGroupedValidationAndDeferredSources(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		for _, invalid := range []string{
			"", "priority-only", "selector-only", "negative", "duplicate", "oauth", "external",
			"literal", "bare-file", "managed_by", "kongctl",
		} {
			t.Run(invalid+map[bool]string{true: "/grouped", false: "/nested"}[grouped], func(t *testing.T) {
				credential := map[string]any{ // #nosec G101 -- deferred source marker, not credential material.
					"ref":      "shared",
					"selector": "true",
					"priority": int64(0),
					"secret":   "DEFER_STATIC_SECRET",
				}
				provider := map[string]any{
					"ref":             "billing",
					"name":            "billing",
					"template_name":   "static_secret",
					"credential_type": "shared",
					"directory":       "people",
				}
				parent := map[string]any{"ref": "people", "name": "default", "allow_all_control_planes": true}
				switch invalid {
				case "priority-only":
					delete(credential, "selector")
				case "selector-only":
					delete(credential, "priority")
				case "negative":
					credential["priority"] = -1
				case "oauth":
					provider["template_name"] = "github"
				case "external":
					parent["_external"] = map[string]any{"id": "731559d3-39bd-48c7-b33c-eb192847eb20"}
				case "literal":
					credential["secret"] = "literal-material-is-forbidden"
				case "bare-file":
					credential["secret"] = "BARE_FILE_SECRET"
				case "managed_by":
					credential["managed_by"] = map[string]any{"tool": "kongctl"}
				case "kongctl":
					credential["kongctl"] = map[string]any{"namespace": "other"}
				}
				values := []any{credential}
				if invalid == "duplicate" {
					values = append(values, map[string]any{"ref": "another", "selector": "false", "priority": 0})
				}
				identity := map[string]any{"directories": []any{parent}, "vault_providers": []any{provider}}
				if grouped {
					for _, value := range values {
						value.(map[string]any)["provider"] = "billing"
					}
					identity["vault_provider_credentials"] = values
				} else {
					provider["shared_credentials"] = values
				}
				data, err := json.Marshal(map[string]any{"identity": identity})
				require.NoError(t, err)
				text := strings.ReplaceAll(
					string(data),
					`"DEFER_STATIC_SECRET"`,
					`!secret {source: !env UNREAD_STATIC_SECRET}`,
				)
				text = strings.ReplaceAll(text, `"BARE_FILE_SECRET"`, `!file ./private-material`)
				rs, err := New().LoadFile(writeLoaderTestFile(t, text))
				if invalid != "" {
					require.Error(t, err)
					require.NotContains(t, err.Error(), "literal-material-is-forbidden")
					return
				}
				require.NoError(t, err)
				require.Len(t, rs.Identity.VaultProviderCredentials, 1)
				value := rs.Identity.VaultProviderCredentials[0]
				require.Equal(t, "billing", value.Provider)
				require.EqualValues(t, 0, *value.Priority)
				require.Contains(t, rs.SecretSources["shared"], "/secret")
				require.True(
					t,
					rs.SyncScope.ChildInScope(
						resources.ResourceTypeIdentityVaultProvider,
						"billing",
						resources.ResourceTypeIdentityVaultProviderCredential,
					),
				)
			})
		}
	}
}

func TestVaultCredentialCollectionPresenceAndLocalRefIdentity(t *testing.T) {
	for _, presence := range []string{"", `,"shared_credentials":[]`} {
		input := `{"identity":{"directories":[{"ref":"people","name":"default","allow_all_control_planes":true}],` +
			`"vault_providers":[{"ref":"billing","directory":"people","name":"billing",` +
			`"template_name":"static_secret","credential_type":"shared"` + presence + `}]}}`
		rs, err := New().LoadFile(writeLoaderTestFile(t, input))
		require.NoError(t, err)
		require.Equal(
			t,
			presence != "",
			rs.SyncScope.ChildInScope(
				resources.ResourceTypeIdentityVaultProvider,
				"billing",
				resources.ResourceTypeIdentityVaultProviderCredential,
			),
		)
	}
}

func TestVaultCredentialPublishedExampleDefersMaterial(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"vault-shared-credentials.yaml", "vault-public-key.pem"} {
		data, err := os.ReadFile(filepath.Join("../../../docs/examples/declarative/identity", name))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(directory, name), data, 0o600))
	}
	require.NoError(t, os.Mkdir(filepath.Join(directory, "secrets"), 0o700))
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(directory, "secrets/billing-priority-token.txt"),
			[]byte("private-example-test-fixture"),
			0o600,
		),
	)
	rs, err := New().LoadFile(filepath.Join(directory, "vault-shared-credentials.yaml"))
	require.NoError(t, err)
	require.Len(t, rs.Identity.VaultProviderCredentials, 2)
	require.Contains(t, rs.SecretSources["default-token"], "/secret")
	require.Contains(t, rs.SecretSources["preferred-token"], "/secret")
	data, err := json.Marshal(rs)
	require.NoError(t, err)
	require.NotContains(t, string(data), "private-example-test-fixture")
}
