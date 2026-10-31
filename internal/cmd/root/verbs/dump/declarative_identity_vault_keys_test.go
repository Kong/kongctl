package dump

import (
	"encoding/json"
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

func TestVaultPublicKeyDumpReloadAndExternalDirectoryBoundary(t *testing.T) {
	data, err := os.ReadFile("../../../../konnect/identitykeys/testdata/public-keys.json")
	require.NoError(t, err)
	var public []struct {
		PublicKey string `json:"public_key"`
	}
	require.NoError(t, json.Unmarshal(data, &public))
	for _, managed := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			root := fixture.Directory("default", "vault-team", false)
			if !managed {
				root.ManagedBy = nil
			}
			root.VaultEnabled = &enabled
			roots := &fixture.DirectoryAPI{Current: root, Directories: []kkComps.KongDirectory{root}}
			keys := &fixture.VaultKeysAPI{}
			client := state.NewClient(state.ClientConfig{
				IdentityDirectoriesAPI: roots, IdentityVaultKeysAPI: keys,
				IdentityPrincipalsAPI: &fixture.PrincipalAPI{}, IdentityGroupsAPI: &fixture.GroupAPI{},
			})
			_, err := client.CreateIdentityVaultKey(
				t.Context(),
				root.ID,
				kkComps.CreateVaultKeyBody{Name: "same-name", PublicKey: public[0].PublicKey},
			)
			require.NoError(t, err)
			_, err = client.CreateIdentityVaultKey(
				t.Context(),
				root.ID,
				kkComps.CreateVaultKeyBody{Name: "same-name", PublicKey: public[1].PublicKey},
			)
			require.NoError(t, err)
			rs := &resources.ResourceSet{}
			dumpContext := &declarativeDumpContext{stateClient: client, includeChildren: true}
			require.NoError(t, identityDirectoryCollector().collect(t.Context(), dumpContext, rs))
			if !managed || !enabled {
				require.Empty(t, rs.Identity.Directories[0].VaultKeys)
				require.Zero(t, keys.Lists, "hidden/external keys are not assigned implicit ownership")
				continue
			}
			require.Len(t, rs.Identity.Directories[0].VaultKeys, 2)
			encoded, err := yaml.Marshal(rs)
			require.NoError(t, err)
			require.Contains(t, string(encoded), "BEGIN PUBLIC KEY")
			require.NotContains(t, string(encoded), "PRIVATE KEY")
			file := filepath.Join(t.TempDir(), "dump.yaml")
			require.NoError(t, os.WriteFile(file, encoded, 0o600))
			loaded, err := loader.New().LoadFile(file)
			require.NoError(t, err)
			plan, err := planner.NewPlanner(client, slog.Default()).
				GeneratePlan(t.Context(), loaded, planner.Options{Mode: planner.PlanModeApply})
			require.NoError(t, err)
			require.Empty(t, plan.Changes)
		}
	}
}
