package identity

import (
	"bytes"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/segmentio/cli"
	"github.com/stretchr/testify/require"
)

func TestVaultProviderReadSelectorsAndTemplateCatalogScope(t *testing.T) {
	const id = "731559d3-39bd-48c7-b33c-eb192847eb20"
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		providers := []string{
			"vault-provider", "vault-providers", "vault_providers", "credential-provider", "credential-providers",
		}
		for _, alias := range providers {
			root, err := NewIdentityCmd(verb, nil, nil)
			require.NoError(t, err)
			command, _, err := root.Find([]string{alias})
			require.NoError(t, err)
			require.NotNil(t, command.RunE)
			require.ErrorContains(t, command.Args(command, nil), "directory")
			require.NoError(t, command.Flags().Set("directory-name", "default"))
			require.NoError(t, command.Args(command, nil))
			require.NoError(t, command.Args(command, []string{"github-app"}))
			require.NoError(t, command.Args(command, []string{id}))
			require.Error(t, command.Args(command, []string{""}))
			require.Error(t, command.Args(command, []string{"one", "two"}))
			require.NoError(t, command.Flags().Set("provider-name", id))
			require.NoError(t, command.Args(command, nil))
			require.ErrorContains(t, command.Args(command, []string{id}), "not both")
			require.NoError(t, command.Flags().Set("provider-name", ""))
			require.ErrorContains(t, command.Args(command, nil), "must not be empty")
		}
		templates := []string{
			"credential-provider-template", "credential-provider-templates", "provider-template", "provider-templates",
		}
		for _, alias := range templates {
			root, err := NewIdentityCmd(verb, nil, nil)
			require.NoError(t, err)
			command, _, err := root.Find([]string{alias})
			require.NoError(t, err)
			require.NoError(t, command.Args(command, nil))
			require.NoError(t, command.Args(command, []string{"custom-oauth"}))
			require.Error(t, command.Args(command, []string{""}))
			require.Error(t, command.Args(command, []string{"one", "two"}))
			require.Nil(t, command.Flags().Lookup("directory-id"))
			require.Nil(t, command.Flags().Lookup("directory-name"))
		}
	}
}

func TestProviderPublicOutputFormatsPreserveFalseAndUnknown(t *testing.T) {
	provider := helpers.IdentityVaultProvider{
		ID: "id", Name: "static-shared", TemplateName: "static_secret",
		CredentialType: "shared", Enabled: new(false), DCREnabled: new(false), PKCEEnabled: new(false),
	}
	record := vaultProviderDisplay(provider)
	require.Equal(t, "false", record.Enabled)
	require.Equal(t, "false", record.DCR)
	require.Equal(t, "false", record.PKCE)
	var out bytes.Buffer
	require.NoError(t, printVaultProviderRows(&out, []helpers.IdentityVaultProvider{provider}))
	require.Contains(t, out.String(), "TEMPLATE_NAME")
	require.Contains(t, out.String(), "static-shared")
	require.NotContains(t, out.String(), "{")
	require.Equal(t, "unknown", vaultProviderDisplay(helpers.IdentityVaultProvider{}).Enabled)
	template := helpers.IdentityCredentialProviderTemplate{
		Name: "custom-oauth", DisplayName: "Custom OAuth",
		SecretType: "authorization_code", BearerMethodsSupported: []string{"header"}, PKCERequired: new(false),
	}
	out.Reset()
	require.NoError(t, printProviderTemplateRows(&out, []helpers.IdentityCredentialProviderTemplate{template}))
	require.Contains(t, out.String(), "PKCE_REQUIRED")
	require.Contains(t, out.String(), "false")
	require.Equal(t, "unknown", providerTemplateDisplay(helpers.IdentityCredentialProviderTemplate{}).PKCERequired)
	for _, format := range []string{"json", "yaml"} {
		for _, value := range []any{provider, template} {
			out.Reset()
			printer, err := cli.Format(format, &out)
			require.NoError(t, err)
			printer.Print(value)
			printer.Flush()
			require.Contains(t, out.String(), "false")
			require.NotContains(t, out.String(), "client_secret")
			require.NotContains(t, out.String(), "enrollment_url")
		}
	}
}
