//go:build e2e

package scenario

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kong/kongctl/test/e2e/harness"
)

// Provision the variables declared by test.meshControlPlanes with isolated hosted
// control planes. Cleanup runs even when a scenario fails before its last step.
func provisionMeshControlPlanes(t *testing.T, cli *harness.CLI, names []string, vars map[string]any) error {
	t.Helper()
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if strings.TrimSpace(name) == "" || seen[name] || vars[name] != nil {
			return fmt.Errorf("invalid or already defined Mesh control-plane variable %q", name)
		}
		seen[name] = true
	}
	for _, name := range names {
		step, err := harness.NewStep(t, cli, "setup-"+name)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(map[string]any{
			"name": "kongctl-e2e-mesh-" + uuid.NewString(), "version": "v3",
			"description": "Temporary kongctl Mesh E2E control plane",
		})
		if err != nil {
			return err
		}
		result, err := step.CreateResource("mesh-control-plane", payload, harness.CreateResourceOptions{})
		if err != nil {
			return err
		}
		resource, ok := result.Parsed.(map[string]any)
		if !ok {
			return fmt.Errorf("mesh control-plane creation returned an invalid resource")
		}
		id, _ := resource["id"].(string)
		if id == "" {
			return fmt.Errorf("mesh control-plane creation returned no ID")
		}
		vars[name] = id
		t.Cleanup(func() {
			// Scenario diagnostics are already finalized when teardown runs.
			cli.ObserveHTTP = nil
			cleanup, err := harness.NewStep(t, cli, "cleanup-"+name)
			if err != nil {
				t.Errorf("initialize Mesh control-plane cleanup: %v", err)
				return
			}
			for attempt := range 3 {
				result, err := cleanup.DeleteResource("mesh-control-plane", harness.DeleteResourceOptions{
					Slug:       fmt.Sprintf("delete-%d", attempt),
					PathParams: map[string]string{"meshControlPlaneId": id},
				})
				if err == nil || result.Status == http.StatusNotFound {
					return
				}
				if attempt == 2 {
					t.Errorf("delete test Mesh control plane %s: %v", id, err)
					return
				}
				time.Sleep(time.Second << attempt)
			}
		})
		if resource["version"] != "v3" {
			return fmt.Errorf("mesh control plane %s has API version %v, expected v3", id, resource["version"])
		}
		// Creation can precede API readiness. Exercise the CLI discovery path
		// while waiting, using the same profile and target as scenario commands.
		for attempt := range 30 {
			_, err := cli.Run(t.Context(), "get", "mesh", "resource-types", "--control-plane-id", id)
			if err == nil {
				break
			}
			if attempt == 29 || t.Context().Err() != nil {
				return fmt.Errorf("mesh control plane %s did not become ready: %w", id, err)
			}
			time.Sleep(2 * time.Second)
		}
	}
	return nil
}
