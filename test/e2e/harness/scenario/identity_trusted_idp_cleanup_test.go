//go:build e2e

package scenario

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestTrustedIDPDirectoryCleanupRequiresFreshEmptyInventory(t *testing.T) {
	const directory = "c0b37b67-06c8-49fb-b51b-2c8f49b20fb1"
	for _, fault := range []string{"", "nonempty", "trusted-idp-read-error", "trusted-idp-missing"} {
		t.Run(fault, func(t *testing.T) {
			cli, dir := cleanupFixtureCLI(t)
			cli.WithEnv(map[string]string{"CLEANUP_FAULT": fault})
			data := `[{"id":"` + directory + `","name":"generated"}`
			if fault == "nonempty" {
				data += `,{"id":"c0b37b67-06c8-49fb-b51b-2c8f49b20fb2","kind":"trusted-idp","directory_id":"` + directory + `"}`
			}
			data += "]"
			require.NoError(t, os.WriteFile(filepath.Join(dir, "resources.json"), []byte(data), 0o600))
			err := cleanupResource(t.Context(), cli, cleanupTarget{
				kind: resources.ResourceTypeIdentityDirectory, name: "generated", requireEmptyTrustedIDPs: true,
			})
			if fault == "" {
				require.NoError(t, err)
				deleted, err := os.ReadFile(filepath.Join(dir, "deleted"))
				require.NoError(t, err)
				require.Equal(t, "/v2/directories/"+directory+"?force=false\n", string(deleted))
			} else {
				require.Error(t, err)
				_, err = os.Stat(filepath.Join(dir, "deleted"))
				require.True(t, os.IsNotExist(err), "no cleanup writes may follow an incomplete IdP inventory")
			}
		})
	}
}

func TestVaultDependencyCleanupNeverCascadesOrAssumesIncompleteReadsAreEmpty(t *testing.T) {
	const directory = "c0b37b67-06c8-49fb-b51b-2c8f49b20fb1"
	for _, fault := range []string{
		"", "vault-key", "vault-provider", "trusted-idp",
		"vault-read-error", "vault-missing", "vault-nonterminal",
	} {
		t.Run(fault, func(t *testing.T) {
			cli, dir := cleanupFixtureCLI(t)
			cli.WithEnv(map[string]string{"CLEANUP_FAULT": fault})
			values := []map[string]string{{"id": directory, "name": "generated"}}
			if fault == "vault-key" || fault == "vault-provider" || fault == "trusted-idp" {
				values = append(values, map[string]string{
					"id": "c0b37b67-06c8-49fb-b51b-2c8f49b20fb2", "kind": fault, "directory_id": directory,
				})
			}
			data, err := json.Marshal(values)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "resources.json"), data, 0o600))
			err = cleanupResource(t.Context(), cli, cleanupTarget{
				kind: resources.ResourceTypeIdentityDirectory, name: "generated", requireEmptyVaultDependencies: true,
			})
			if fault == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				_, err = os.Stat(filepath.Join(dir, "deleted"))
				require.True(t, os.IsNotExist(err), "no cleanup write may follow an unknown or attached Vault child")
			}
		})
	}
}
