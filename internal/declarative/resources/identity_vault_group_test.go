package resources_test

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestVaultGroupingExplainAndScaffoldAgreeOnDeclarationPaths(t *testing.T) {
	for _, tc := range []struct {
		path       string
		kind       resources.ResourceType
		collection string
	}{
		{"identity.vault.keys", resources.ResourceTypeIdentityVaultKey, "keys"},
		{"identity.directories.vault.keys", resources.ResourceTypeIdentityVaultKey, "keys"},
		{"identity.vault.providers", resources.ResourceTypeIdentityVaultProvider, "providers"},
		{"identity.directories.vault.providers", resources.ResourceTypeIdentityVaultProvider, "providers"},
		{
			"identity.vault.provider_credentials",
			resources.ResourceTypeIdentityVaultProviderCredential,
			"provider_credentials",
		},
		{
			"identity.directories.vault.providers.shared_credentials",
			resources.ResourceTypeIdentityVaultProviderCredential,
			"providers",
		},
	} {
		t.Run(tc.path, func(t *testing.T) {
			subject, err := resources.ResolveExplainSubject(tc.path)
			require.NoError(t, err)
			require.Equal(t, tc.kind, subject.Doc.ResourceType)
			text, err := resources.RenderScaffoldYAML(subject)
			require.NoError(t, err)
			var doc map[string]any
			require.NoError(t, yaml.Unmarshal([]byte(text), &doc))
			identity := doc["identity"].(map[string]any)
			parent := identity
			if directories, ok := identity["directories"].([]any); ok {
				parent = directories[0].(map[string]any)
			}
			vault, ok := parent["vault"].(map[string]any)
			require.True(t, ok, text)
			require.NotEmpty(t, vault[tc.collection], text)
			indent := "    "
			if _, nested := identity["directories"]; nested {
				indent = "        "
			}
			require.Contains(t, text, "\n"+indent+tc.collection+":\n")
			require.NotContains(t, text, "vault_keys:")
			require.NotContains(t, text, "vault_providers:")
		})
	}
	group, err := resources.ResolveExplainSubject("identity.directories.vault")
	require.NoError(t, err)
	require.False(t, group.ResourceTarget)
	require.Equal(t, resources.ResourceTypeIdentityDirectory, group.Doc.ResourceType)
}
