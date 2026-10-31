//go:build e2e

package scenario

import (
	"context"
	"fmt"
	"maps"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/test/e2e/harness"
)

func markedGroupFixture(value helpers.IdentityGroup, directoryID string) bool {
	expected := map[string]string{
		resources.ManagedByToolKey: resources.ManagedByTool, resources.ManagedByVersionKey: "e2e-fixture",
		resources.ChildNamespaceKey: "identity-group-read-e2e", resources.ChildProtectedKey: "false",
	}
	return util.IsValidUUID(value.ID) && value.Labels["e2e-fixture"] == directoryID &&
		strings.HasPrefix(value.Name, "Kongctl"+strings.ReplaceAll(directoryID, "-", "")) &&
		maps.Equal(value.ManagedBy, expected) && value.ProvisionedBy != nil &&
		*value.ProvisionedBy == kkComps.ProvisionedByKonnectUser && value.TeamManager == nil
}

func groupMembershipFixtures(ctx context.Context, cli *harness.CLI, directoryID, groupID string,
	principals map[string]bool,
) ([]kkComps.KongGroupMembershipEntry, error) {
	var values []kkComps.KongGroupMembershipEntry
	cli.OverrideNextOutput("json")
	if _, err := cli.RunJSON(ctx, &values, "get", "identity", "group-memberships",
		"--directory-id", directoryID, "--group-id", groupID); err != nil {
		return nil, err
	}
	if values == nil {
		return nil, fmt.Errorf("missing group membership fixture inventory")
	}
	for _, value := range values {
		if !util.IsValidUUID(value.ID) || value.Member.Type != kkComps.KongGroupMembershipTypePrincipal ||
			!principals[value.Member.ID] {
			return nil, fmt.Errorf("group contains an unmarked member; refusing fixture cleanup")
		}
	}
	return values, nil
}

// This opt-in policy covers manually created, credential-free inspection
// fixtures only. It does not establish production authority over group keys
// (Q33) or importer-managed memberships (Q32).
func cleanupGroupFixtures(ctx context.Context, cli *harness.CLI, directoryID string) error {
	var groups []helpers.IdentityGroup
	cli.OverrideNextOutput("json")
	if _, err := cli.RunJSON(ctx, &groups, "get", "identity", "groups", "--directory-id", directoryID); err != nil {
		return err
	}
	if groups == nil {
		return fmt.Errorf("missing group fixture inventory")
	}
	var values []struct {
		ID     string            `json:"id"`
		Labels map[string]string `json:"labels"`
	}
	cli.OverrideNextOutput("json")
	if _, err := cli.RunJSON(ctx, &values, "get", "identity", "principals", "--directory-id", directoryID); err != nil {
		return err
	}
	if values == nil {
		return fmt.Errorf("missing group member principal fixture inventory")
	}
	principals := make(map[string]bool)
	for _, value := range values {
		if !util.IsValidUUID(value.ID) || value.Labels["e2e-fixture"] != directoryID {
			return fmt.Errorf("directory contains an unmarked principal; refusing group fixture cleanup")
		}
		principals[value.ID] = true
	}
	entries := make(map[string][]kkComps.KongGroupMembershipEntry)
	for _, value := range groups {
		if !markedGroupFixture(value, directoryID) {
			return fmt.Errorf("directory contains an unmarked group; refusing fixture cleanup")
		}
		members, err := groupMembershipFixtures(ctx, cli, directoryID, value.ID, principals)
		if err != nil {
			return err
		}
		entries[value.ID] = members
	}
	for _, value := range groups {
		var current helpers.IdentityGroup
		cli.OverrideNextOutput("json")
		if _, err := cli.RunJSON(ctx, &current, "get", "identity", "group", value.ID,
			"--directory-id", directoryID); err != nil {
			return err
		}
		if current.ID != value.ID || current.Name != value.Name || !markedGroupFixture(current, directoryID) {
			return fmt.Errorf("group fixture identity or ownership changed before cleanup")
		}
		members, err := groupMembershipFixtures(ctx, cli, directoryID, value.ID, principals)
		if err != nil {
			return err
		}
		if len(members) != len(entries[value.ID]) {
			return fmt.Errorf("group fixture membership changed before cleanup")
		}
		expected := make(map[string]kkComps.KongGroupMember)
		for _, member := range entries[value.ID] {
			expected[member.ID] = member.Member
		}
		for _, member := range members {
			initial, ok := expected[member.ID]
			if !ok || initial.ID != member.Member.ID || initial.Type != member.Member.Type {
				return fmt.Errorf("group fixture member identity changed before cleanup")
			}
			var observed kkComps.KongGroupMembershipEntry
			cli.OverrideNextOutput("json")
			if _, err := cli.RunJSON(ctx, &observed, "get", "identity", "group-membership", member.ID,
				"--directory-id", directoryID, "--group-id", value.ID); err != nil {
				return err
			}
			if observed.ID != member.ID || observed.Member.ID != member.Member.ID ||
				observed.Member.Type != member.Member.Type {
				return fmt.Errorf("group fixture membership changed before deletion")
			}
			cli.OverrideNextOutput("json")
			if _, err := cli.Run(ctx, "api", "delete",
				"/v2/directories/"+directoryID+"/groups/"+value.ID+"/membership/"+member.ID); err != nil {
				return err
			}
		}
		remaining, err := groupMembershipFixtures(ctx, cli, directoryID, value.ID, principals)
		if err != nil {
			return err
		}
		if len(remaining) != 0 {
			return fmt.Errorf("group still contains members; refusing fixture cleanup")
		}
		cli.OverrideNextOutput("json")
		if _, err := cli.Run(ctx, "api", "delete", "/v2/directories/"+directoryID+"/groups/"+value.ID); err != nil {
			return err
		}
	}
	cli.OverrideNextOutput("json")
	if _, err := cli.RunJSON(ctx, &groups, "get", "identity", "groups", "--directory-id", directoryID); err != nil {
		return err
	}
	if groups == nil || len(groups) != 0 {
		return fmt.Errorf("directory still contains groups; refusing fixture cleanup")
	}
	return nil
}
