package dump

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestVaultProviderDumpReloadAndUnmanagedDirectoryBoundary(t *testing.T) {
	for _, managed := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			root := fixture.Directory("default", "vault-provider-team", false)
			root.VaultEnabled = &enabled
			if !managed {
				root.ManagedBy = nil
			}
			providers := &fixture.VaultProvidersAPI{}
			client := state.NewClient(state.ClientConfig{
				IdentityTrustedIDPsAPI: &fixture.TrustedIDPsAPI{},
				IdentityDirectoriesAPI: &fixture.DirectoryAPI{
					Current:     root,
					Directories: []kkComps.KongDirectory{root},
				},
				IdentityVaultProvidersAPI: providers, IdentityStaticVaultCredentialsAPI: providers,
				IdentityVaultKeysAPI:  &fixture.VaultKeysAPI{},
				IdentityPrincipalsAPI: &fixture.PrincipalAPI{}, IdentityGroupsAPI: &fixture.GroupAPI{},
			})
			_, err := client.CreateIdentityVaultProvider(t.Context(), root.ID, kkComps.CreateVaultProviderBody{
				Name: "static-provider", TemplateName: kkComps.ProviderTemplateNameStaticSecret, CredentialType: new("shared"),
			})
			require.NoError(t, err)
			_, err = client.CreateIdentityVaultProvider(t.Context(), root.ID, kkComps.CreateVaultProviderBody{
				Name: "dcr-provider", TemplateName: kkComps.ProviderTemplateNameCustomOauth, DcrEnabled: new(true),
				TokenEndpoint: new(
					"https://fixture.example.test/token",
				), AuthorizationEndpoint: new("https://fixture.example.test/authorize"),
				RegistrationEndpoint: new(
					"https://fixture.example.test/register",
				), Scopes: []string{"read"}, PkceEnabled: new(true),
			})
			require.NoError(t, err)
			rs := &resources.ResourceSet{}
			context := &declarativeDumpContext{stateClient: client, includeChildren: true}
			require.NoError(t, identityDirectoryCollector().collect(t.Context(), context, rs))
			if !managed || !enabled {
				require.Empty(t, rs.Identity.Directories[0].VaultProviders)
				require.Zero(t, providers.Lists)
				continue
			}
			require.Len(t, rs.Identity.Directories[0].VaultProviders, 2)
			encoded, err := yaml.Marshal(rs)
			require.NoError(t, err)
			var dumped map[string]any
			require.NoError(t, yaml.Unmarshal(encoded, &dumped))
			directory := dumped["identity"].(map[string]any)["directories"].([]any)[0].(map[string]any)
			require.Len(t, directory["vault"].(map[string]any)["providers"], 2)
			require.NotContains(t, directory, "vault_providers")
			require.Contains(t, string(encoded), "registration_endpoint:")
			require.NotContains(t, string(encoded), "client_secret:")
			require.NotContains(t, string(encoded), "scopes: []")
			file := filepath.Join(t.TempDir(), "dump.yaml")
			require.NoError(t, os.WriteFile(file, encoded, 0o600))
			loaded, err := loader.New().LoadFile(file)
			require.NoError(t, err)
			plan, err := planner.NewPlanner(client, slog.Default()).
				GeneratePlan(t.Context(), loaded, planner.Options{Mode: planner.PlanModeApply})
			require.NoError(t, err)
			require.Empty(t, plan.Changes)
			providers.Providers[0].TokenEndpointEffective = new("")
			err = collectIdentityVaultProviders(
				t.Context(),
				context,
				&resources.IdentityDirectoryResource{BaseResource: resources.BaseResource{Ref: root.ID}},
			)
			require.ErrorContains(t, err, "empty provider endpoints")
			providers.Providers[0].TokenEndpointEffective = nil
			providers.Providers[0].TemplateName = "github"
			err = collectIdentityVaultProviders(
				t.Context(),
				context,
				&resources.IdentityDirectoryResource{BaseResource: resources.BaseResource{Ref: root.ID}},
			)
			require.ErrorContains(t, err, "reproducible declaration")
		}
	}
}
