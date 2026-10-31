//go:build e2e

package scenario

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/test/e2e/harness"
)

var cleanupEnvPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

type cleanupTarget struct {
	kind resources.ResourceType
	name string
}

func registerResourceCleanup(t *testing.T, cli *harness.CLI, specs []CleanupResource, vars map[string]any) error {
	t.Helper()
	seenVars, seenEnv := map[string]bool{}, map[string]bool{}
	for _, spec := range specs {
		// Only families with complete list commands and safe raw deletion participate.
		//nolint:exhaustive
		switch resources.ResourceType(spec.Resource) {
		case resources.ResourceTypeIdentityDirectory, resources.ResourceTypeControlPlane,
			resources.ResourceTypeIdentityAuthServer:
		default:
			return fmt.Errorf("unsupported cleanup resource %q", spec.Resource)
		}
		if strings.TrimSpace(spec.NameVar) == "" || vars[spec.NameVar] != nil || seenVars[spec.NameVar] {
			return fmt.Errorf("cleanup name variable %q is empty or already defined", spec.NameVar)
		}
		if !cleanupEnvPattern.MatchString(spec.NameEnv) || seenEnv[spec.NameEnv] {
			return fmt.Errorf("cleanup name environment variable %q is invalid or already defined", spec.NameEnv)
		}
		seenVars[spec.NameVar], seenEnv[spec.NameEnv] = true, true
	}
	if len(specs) == 0 {
		return nil
	}
	targets := make([]cleanupTarget, 0, len(specs))
	for _, spec := range specs {
		name := "kongctl-e2e-" + strings.ReplaceAll(spec.Resource, "_", "-") + "-" + uuid.NewString()
		vars[spec.NameVar] = name
		cli.WithEnv(map[string]string{spec.NameEnv: name})
		targets = append(targets, cleanupTarget{kind: resources.ResourceType(spec.Resource), name: name})
	}
	t.Cleanup(func() {
		// Preserve teardown evidence separately from finalized scenario diagnostics.
		cli.ObserveHTTP = nil
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		for _, target := range targets {
			var err error
			for attempt := range 3 {
				err = cleanupResource(ctx, cli, target)
				if err == nil || ctx.Err() != nil {
					break
				}
				if attempt < 2 {
					select {
					case <-ctx.Done():
					case <-time.After(time.Second << attempt):
					}
				}
			}
			if err != nil {
				t.Errorf("cleanup %s %q: %v", target.kind, target.name, err)
			}
		}
	})
	return nil
}

func cleanupResource(ctx context.Context, cli *harness.CLI, target cleanupTarget) error {
	var args []string
	var endpoint string
	// Only explicitly supported, independently named resources participate.
	//nolint:exhaustive
	switch target.kind {
	case resources.ResourceTypeIdentityAuthServer:
		args, endpoint = []string{"get", "identity", "auth-servers"}, "/v1/auth-servers/"
	case resources.ResourceTypeIdentityDirectory:
		args, endpoint = []string{"get", "identity", "directories"}, "/v2/directories/"
	case resources.ResourceTypeControlPlane:
		args, endpoint = []string{"get", "gateway", "control-planes"}, "/v2/control-planes/"
	default:
		return fmt.Errorf("unsupported cleanup resource %q", target.kind)
	}
	var observed []struct{ ID, Name string }
	cli.OverrideNextOutput("json")
	if _, err := cli.RunJSON(ctx, &observed, args...); err != nil {
		return err
	}
	var id string
	for _, value := range observed {
		if value.Name != target.name {
			continue
		}
		if id != "" {
			return fmt.Errorf("multiple resources match the generated cleanup name")
		}
		if !util.IsValidUUID(value.ID) {
			return fmt.Errorf("cleanup resource has no valid ID")
		}
		id = value.ID
	}
	if id == "" {
		return nil
	}
	if target.kind == resources.ResourceTypeIdentityDirectory {
		if err := cleanupPrincipalFixtures(ctx, cli, id); err != nil {
			return err
		}
		endpoint += id + "?force=false"
	} else if target.kind == resources.ResourceTypeIdentityAuthServer {
		if err := cleanupAuthServerFixtures(ctx, cli, id, target.name); err != nil {
			return err
		}
		endpoint += id + "?force=false"
	} else {
		endpoint += id
	}
	cli.OverrideNextOutput("json")
	_, err := cli.Run(ctx, "api", "delete", endpoint)
	return err
}

// Scopes and claims have no fixture labels. Only names carrying this generated
// server's UUID or generated name are candidates. Clients must also carry the
// server's ID in a fixture label. Check all collections before deleting anything.
func cleanupAuthServerFixtures(ctx context.Context, cli *harness.CLI, serverID, serverName string) error {
	type fixture struct{ ID, Name string }
	var claims, scopes []fixture
	for _, collection := range []struct {
		command string
		values  *[]fixture
	}{{"auth-server-claims", &claims}, {"auth-server-scopes", &scopes}} {
		cli.OverrideNextOutput("json")
		if _, err := cli.RunJSON(
			ctx, collection.values,
			"get", "identity", collection.command, "--auth-server-id", serverID,
		); err != nil {
			return err
		}
		for _, value := range *collection.values {
			marked := strings.HasPrefix(value.Name, "kongctl-e2e-"+serverID+"-") ||
				strings.HasPrefix(value.Name, serverName+"-")
			if !marked || !util.IsValidUUID(value.ID) {
				return fmt.Errorf("auth server contains an unmarked child; refusing fixture cleanup")
			}
		}
	}
	var clients []struct {
		ID     string            `json:"id"`
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
	}
	cli.OverrideNextOutput("json")
	if _, err := cli.RunJSON(
		ctx, &clients,
		"get", "identity", "auth-server-clients", "--auth-server-id", serverID,
	); err != nil {
		return err
	}
	if clients == nil {
		return fmt.Errorf("missing auth-server client inventory before fixture cleanup")
	}
	clientFixtures := make([]fixture, 0, len(clients))
	for _, value := range clients {
		marked := strings.HasPrefix(value.Name, "kongctl-e2e-"+serverID+"-") ||
			strings.HasPrefix(value.Name, serverName+"-")
		if value.ID == "" || !marked || value.Labels["e2e-fixture"] != serverID {
			return fmt.Errorf("auth server contains an unmarked client; refusing fixture cleanup")
		}
		clientFixtures = append(clientFixtures, fixture{ID: value.ID, Name: value.Name})
	}
	for _, collection := range []struct {
		path   string
		values []fixture
	}{{"clients", clientFixtures}, {"claims", claims}, {"scopes", scopes}} {
		for _, value := range collection.values {
			cli.OverrideNextOutput("json")
			if _, err := cli.Run(
				ctx,
				"api", "delete", "/v1/auth-servers/"+serverID+"/"+collection.path+"/"+url.PathEscape(value.ID),
			); err != nil {
				return err
			}
		}
	}
	return nil
}

// Only principals explicitly marked with this test directory's ID are removed.
// Unmarked descendants cause cleanup to stop before any deletion.
func cleanupPrincipalFixtures(ctx context.Context, cli *harness.CLI, directoryID string) error {
	var values []struct {
		ID     string            `json:"id"`
		Labels map[string]string `json:"labels"`
	}
	cli.OverrideNextOutput("json")
	if _, err := cli.RunJSON(ctx, &values, "get", "identity", "principals", "--directory-id", directoryID); err != nil {
		return err
	}
	for _, value := range values {
		if value.Labels["e2e-fixture"] != directoryID {
			return fmt.Errorf("directory contains an unmarked principal; refusing fixture cleanup")
		}
		if !util.IsValidUUID(value.ID) {
			return fmt.Errorf("principal fixture has no valid ID")
		}
	}
	for _, value := range values {
		cli.OverrideNextOutput("json")
		if _, err := cli.Run(ctx, "api", "delete", "/v2/directories/"+directoryID+"/principals/"+value.ID); err != nil {
			return err
		}
	}
	return nil
}
