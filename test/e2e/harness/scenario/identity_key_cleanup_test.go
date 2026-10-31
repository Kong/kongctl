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

func TestPrincipalKeyFixtureCleanupRechecksAllCollectionsAndMetadata(t *testing.T) {
	for _, fault := range []string{"", "unmarked-key", "key-read-error", "key-metadata-change", "key-protection-change"} {
		t.Run(fault, func(t *testing.T) {
			cli, dir := cleanupFixtureCLI(t)
			cli.WithEnv(map[string]string{"CLEANUP_FAULT": fault})
			const directory = "c0b37b67-06c8-49fb-b51b-2c8f49b20fb1"
			const principal = "c0b37b67-06c8-49fb-b51b-2c8f49b20fb2"
			const key = "c0b37b67-06c8-49fb-b51b-2c8f49b20fb3"
			metadata := map[string]string{
				"tool": "kongctl", "version": "e2e-fixture", "kongctl-namespace": "identity-key-read-e2e",
				"kongctl-ref": "fixture-" + directory + "-1", "kongctl-protected": "false",
			}
			values := []map[string]any{
				{"id": directory, "name": "generated"},
				{
					"id": principal, "kind": "principal", "directory_id": directory,
					"labels": map[string]string{"e2e-fixture": directory},
				},
				{
					"id": key, "kind": "api-key", "directory_id": directory, "principal_id": principal,
					"managed_by": metadata,
				},
			}
			if fault == "unmarked-key" {
				const other = "c0b37b67-06c8-49fb-b51b-2c8f49b20fb4"
				values = append(
					values,
					map[string]any{
						"id": other, "kind": "principal", "directory_id": directory,
						"labels": map[string]string{"e2e-fixture": directory},
					},
					map[string]any{
						"id": "c0b37b67-06c8-49fb-b51b-2c8f49b20fb5", "kind": "api-key",
						"directory_id": directory, "principal_id": other, "managed_by": map[string]string{},
					},
				)
			}
			data, err := json.Marshal(values)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "resources.json"), data, 0o600))
			err = cleanupResource(t.Context(), cli, cleanupTarget{
				kind: resources.ResourceTypeIdentityDirectory, name: "generated", principalAPIKeys: true,
			})
			if fault == "" {
				require.NoError(t, err)
				deleted, err := os.ReadFile(filepath.Join(dir, "deleted"))
				require.NoError(t, err)
				require.Equal(t, "/v2/directories/"+directory+"/principals/"+principal+"/api-keys/"+key+"\n"+
					"/v2/directories/"+directory+"/principals/"+principal+"\n/v2/directories/"+directory+"?force=false\n",
					string(deleted))
			} else {
				require.Error(t, err)
				_, err := os.Stat(filepath.Join(dir, "deleted"))
				require.True(t, os.IsNotExist(err))
			}
		})
	}
}
