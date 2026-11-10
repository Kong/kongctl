//go:build e2e

package scenario

import (
	"context"
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/test/e2e/harness"
)

type principalKeyFixture struct {
	ID        string            `json:"id"`
	ManagedBy map[string]string `json:"managed_by"`
}

func markedPrincipalKey(value principalKeyFixture, directoryID string) bool {
	return util.IsValidUUID(value.ID) && value.ManagedBy[resources.ManagedByToolKey] == resources.ManagedByTool &&
		value.ManagedBy[resources.ChildNamespaceKey] == "identity-key-read-e2e" &&
		value.ManagedBy[resources.ManagedByVersionKey] == "e2e-fixture" &&
		value.ManagedBy[resources.ChildProtectedKey] == "false" &&
		strings.HasPrefix(value.ManagedBy[resources.ManagedByRefKey], "fixture-"+directoryID+"-")
}

func principalAPIKeyFixtures(ctx context.Context, cli *harness.CLI,
	directoryID, principalID string,
) ([]principalKeyFixture, error) {
	var values []principalKeyFixture
	cli.OverrideNextOutput("json")
	if _, err := cli.RunJSON(ctx, &values, "get", "identity", "principal-api-keys",
		"--directory-id", directoryID, "--principal-id", principalID); err != nil {
		return nil, err
	}
	if values == nil {
		return nil, fmt.Errorf("missing principal API-key inventory before fixture cleanup")
	}
	for _, value := range values {
		if !markedPrincipalKey(value, directoryID) {
			return nil, fmt.Errorf("principal contains an unmarked API key; refusing fixture cleanup")
		}
	}
	return values, nil
}

func deletePrincipalAPIKeyFixtures(ctx context.Context, cli *harness.CLI,
	directoryID, principalID string, values []principalKeyFixture,
) error {
	for _, value := range values {
		var current principalKeyFixture
		cli.OverrideNextOutput("json")
		if _, err := cli.RunJSON(ctx, &current, "get", "identity", "principal-api-key", value.ID,
			"--directory-id", directoryID, "--principal-id", principalID); err != nil {
			return err
		}
		if current.ID != value.ID || !markedPrincipalKey(current, directoryID) {
			return fmt.Errorf("API-key fixture identity or metadata changed before cleanup")
		}
		cli.OverrideNextOutput("json")
		if _, err := cli.Run(ctx, "api", "delete",
			"/v2/directories/"+directoryID+"/principals/"+principalID+"/api-keys/"+value.ID); err != nil {
			return err
		}
	}
	remaining, err := principalAPIKeyFixtures(ctx, cli, directoryID, principalID)
	if err != nil {
		return err
	}
	if len(remaining) != 0 {
		return fmt.Errorf("principal still contains API keys; refusing parent cleanup")
	}
	return nil
}
