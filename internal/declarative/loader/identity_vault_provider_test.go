package loader

import (
	"encoding/json"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestVaultProviderDeclarationsRequireExplicitFieldsAndManagedParent(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		for _, invalid := range []string{
			"", "name", "template_name", "credential_type", "external", "kongctl",
			"managed_by", "duplicate", "scopes", "endpoint", "dcr",
		} {
			t.Run(invalid+map[bool]string{true: "/grouped", false: "/nested"}[grouped], func(t *testing.T) {
				parent := map[string]any{
					"ref":                      "people",
					"name":                     "default",
					"allow_all_control_planes": true,
					"vault_enabled":            true,
				}
				provider := map[string]any{
					"ref":             "billing",
					"name":            "billing-provider",
					"template_name":   "static_secret",
					"credential_type": "shared",
				}
				switch invalid {
				case "name", "template_name", "credential_type":
					delete(provider, invalid)
				case "external":
					parent["_external"] = map[string]any{"id": "731559d3-39bd-48c7-b33c-eb192847eb20"}
				case "kongctl":
					provider["kongctl"] = map[string]any{"namespace": "independent"}
				case "managed_by":
					provider["managed_by"] = map[string]any{"tool": "kongctl"}
				case "scopes":
					provider["scopes"] = []string{}
				case "endpoint":
					provider["token_endpoint"] = ""
				case "dcr":
					provider["dcr_enabled"] = true
				}
				providers := []any{provider}
				if invalid == "duplicate" {
					providers = append(
						providers,
						map[string]any{
							"ref":             "other-ref",
							"name":            "billing-provider",
							"template_name":   "static_secret",
							"credential_type": "shared",
						},
					)
				}
				identity := map[string]any{"directories": []any{parent}}
				if grouped {
					for _, value := range providers {
						value.(map[string]any)["directory"] = "people"
					}
					identity["vault_providers"] = providers
				} else {
					parent["vault_providers"] = providers
				}
				data, err := json.Marshal(map[string]any{"identity": identity})
				require.NoError(t, err)
				rs, err := New().LoadFile(writeLoaderTestFile(t, string(data)))
				if invalid != "" {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
				require.Len(t, rs.Identity.VaultProviders, 1)
				require.Equal(t, "people", rs.Identity.VaultProviders[0].Directory)
				require.Equal(t, "billing-provider", rs.Identity.VaultProviders[0].Name)
				require.True(
					t,
					rs.SyncScope.ChildInScope(
						resources.ResourceTypeIdentityDirectory,
						"people",
						resources.ResourceTypeIdentityVaultProvider,
					),
				)
			})
		}
	}
}

func TestVaultProviderCollectionPresenceAndDeferredSecret(t *testing.T) {
	for _, presence := range []string{"", ",\"vault_providers\":[]"} {
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
				resources.ResourceTypeIdentityVaultProvider,
			),
		)
	}
	input := `identity:
  directories:
    - ref: people
      name: default
      allow_all_control_planes: true
      vault_providers:
        - ref: github
          name: github-provider
          template_name: github
          client_id: upstream-client
          client_secret: !secret {source: !env UNREAD_IDENTITY_PROVIDER_SECRET}
`
	rs, err := New().LoadFile(writeLoaderTestFile(t, input))
	require.NoError(t, err)
	require.Contains(t, rs.SecretSources, "github")
	require.Contains(t, rs.SecretSources["github"], "/client_secret")
}

func TestVaultProviderPublishedExampleLoadsWithoutReadingSecret(t *testing.T) {
	t.Setenv("VAULT_GITHUB_CLIENT_ID", "public-upstream-client")
	t.Setenv("VAULT_GITHUB_CLIENT_SECRET", "")
	rs, err := New().LoadFile("../../../docs/examples/declarative/identity/vault-providers.yaml")
	require.NoError(t, err)
	require.Len(t, rs.Identity.VaultProviders, 3)
	require.Contains(t, rs.SecretSources["github"], "/client_secret")
	require.Equal(t, "identity-directory", resources.GetNamespace(rs.Identity.Directories[0].Kongctl))
}
