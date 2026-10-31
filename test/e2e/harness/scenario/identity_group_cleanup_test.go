//go:build e2e

package scenario

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestGroupFixtureCleanupRequiresManualMarkedGroupsAndMembers(t *testing.T) {
	for _, fault := range []string{
		"", "unmarked-group", "unmarked-member", "imported-group", "group-read-error",
		"membership-read-error", "group-protection-change", "membership-member-change",
	} {
		t.Run(fault, func(t *testing.T) {
			cli, dir := cleanupFixtureCLI(t)
			cli.WithEnv(map[string]string{"CLEANUP_FAULT": fault})
			const directory = "c0b37b67-06c8-49fb-b51b-2c8f49b20fb1"
			const principal = "c0b37b67-06c8-49fb-b51b-2c8f49b20fb2"
			const group = "c0b37b67-06c8-49fb-b51b-2c8f49b20fb3"
			const other = "c0b37b67-06c8-49fb-b51b-2c8f49b20fb4"
			const member = "c0b37b67-06c8-49fb-b51b-2c8f49b20fb5"
			makeGroup := func(id string) map[string]any {
				return map[string]any{
					"id": id, "kind": "group", "directory_id": directory,
					"name":   "Kongctl" + strings.ReplaceAll(directory, "-", "") + strings.ReplaceAll(id, "-", ""),
					"labels": map[string]string{"e2e-fixture": directory}, "provisioned_by": "konnect_user",
					"managed_by": map[string]string{
						"tool": "kongctl", "version": "e2e-fixture", "kongctl-namespace": "identity-group-read-e2e",
						"kongctl-protected": "false",
					},
				}
			}
			values := []map[string]any{
				{"id": directory, "name": "generated"},
				{
					"id": principal, "kind": "principal", "directory_id": directory,
					"labels": map[string]string{"e2e-fixture": directory},
				},
				makeGroup(group), makeGroup(other),
				{
					"id": member, "kind": "membership", "directory_id": directory, "group_id": group,
					"member": map[string]string{"type": "principal", "id": principal},
				},
			}
			if fault == "unmarked-group" {
				values[3]["labels"] = map[string]string{}
			} else if fault == "unmarked-member" {
				values[4]["member"] = map[string]string{"type": "principal", "id": other}
			} else if fault == "imported-group" {
				values[2]["provisioned_by"] = "trusted_idp"
			}
			data, err := json.Marshal(values)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "resources.json"), data, 0o600))
			err = cleanupResource(t.Context(), cli, cleanupTarget{
				kind: resources.ResourceTypeIdentityDirectory, name: "generated", principalGroups: true,
			})
			if fault != "" {
				require.Error(t, err)
				_, err := os.Stat(filepath.Join(dir, "deleted"))
				require.True(t, os.IsNotExist(err), "check all fixture scopes before deletion")
				return
			}
			require.NoError(t, err)
			deleted, err := os.ReadFile(filepath.Join(dir, "deleted"))
			require.NoError(t, err)
			require.Equal(t, "/v2/directories/"+directory+"/groups/"+group+"/membership/"+member+"\n"+
				"/v2/directories/"+directory+"/groups/"+group+"\n"+
				"/v2/directories/"+directory+"/groups/"+other+"\n"+
				"/v2/directories/"+directory+"/principals/"+principal+"\n"+
				"/v2/directories/"+directory+"?force=false\n", string(deleted))
		})
	}
}
