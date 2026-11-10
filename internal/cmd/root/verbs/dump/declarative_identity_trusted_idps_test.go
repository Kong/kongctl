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

func TestTrustedIDPDumpReloadAndParentAuthority(t *testing.T) {
	for _, managed := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			root := fixture.Directory("default", "trust-team", false)
			root.VaultEnabled = &enabled
			if !managed {
				root.ManagedBy = nil
			}
			trusts := &fixture.TrustedIDPsAPI{}
			client := state.NewClient(state.ClientConfig{
				IdentityDirectoriesAPI: &fixture.DirectoryAPI{
					Current:     root,
					Directories: []kkComps.KongDirectory{root},
				},
				IdentityTrustedIDPsAPI: trusts, IdentityVaultProvidersAPI: &fixture.VaultProvidersAPI{},
				IdentityVaultKeysAPI:  &fixture.VaultKeysAPI{},
				IdentityPrincipalsAPI: &fixture.PrincipalAPI{}, IdentityGroupsAPI: &fixture.GroupAPI{},
			})
			_, err := client.CreateIdentityTrustedIDP(t.Context(), root.ID, kkComps.CreateTrustedIdpPayload{
				IssuerURL: "https://a.example.test", DisplayName: new("Shared display"),
				JwksURI:                new("https://a.example.test/keys"),
				JitProvisioningEnabled: new(false), VaultAccessEnabled: new(true),
				Jit: &kkComps.TrustedIdpJit{Mapping: &kkComps.TrustedIdpJitMapping{
					Email: new("email"), Metadata: map[string]string{"team": "department"},
				}},
			})
			require.NoError(t, err)
			_, err = client.CreateIdentityTrustedIDP(
				t.Context(),
				root.ID,
				kkComps.CreateTrustedIdpPayload{IssuerURL: "https://b.example.test"},
			)
			require.NoError(t, err)
			rs := &resources.ResourceSet{}
			dumpContext := &declarativeDumpContext{stateClient: client, includeChildren: true}
			require.NoError(t, identityDirectoryCollector().collect(t.Context(), dumpContext, rs))
			if !managed || !enabled {
				require.Empty(t, rs.Identity.Directories[0].TrustedIDPs)
				require.Zero(t, trusts.Lists)
				continue
			}
			require.Len(t, rs.Identity.Directories[0].TrustedIDPs, 2)
			data, err := yaml.Marshal(rs)
			require.NoError(t, err)
			require.Contains(t, string(data), "jwks_uri: null")
			require.Contains(t, string(data), "display_name: null")
			require.Contains(t, string(data), "jit: {}")
			file := filepath.Join(t.TempDir(), "dump.yaml")
			require.NoError(t, os.WriteFile(file, data, 0o600))
			loaded, err := loader.New().LoadFile(file)
			require.NoError(t, err)
			plan, err := planner.NewPlanner(client, slog.Default()).
				GeneratePlan(t.Context(), loaded, planner.Options{Mode: planner.PlanModeApply})
			require.NoError(t, err)
			require.Empty(t, plan.Changes)
		}
	}
}
