//go:build e2e

package scenario

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/test/e2e/harness"
)

func TestProducePolicyTranscodeRegistryAssertion(t *testing.T) {
	t.Setenv("KONGCTL_E2E_UPDATE_EXPECT", "0")
	path := "../../scenarios/event-gateway/produce-policy/scenario.yaml"
	sc, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	sc.Vars["schemaValidationPolicyID"] = "schema-policy-id"
	sc.Vars["transcodeRegistryID"] = "registry-id"
	var verify Command
	for _, step := range sc.Steps {
		if step.Name == "015-transcode-create" {
			for _, cmd := range step.Commands {
				if cmd.Name == "verify" {
					verify = cmd
				}
			}
		}
	}
	if len(verify.Assertions) == 0 {
		t.Fatal("transcode verification command missing")
	}
	for _, registryID := range []string{"registry-id", "different-registry-id"} {
		t.Run(registryID, func(t *testing.T) {
			parent := []any{
				map[string]any{"id": "schema-policy-id", "name": sc.Vars["svPolicyName"]},
				map[string]any{
					"id": "transcode-policy-id", "name": "transcode-policy", "type": "transcode",
					"enabled": true, "parent_policy_id": "schema-policy-id",
					"config": map[string]any{
						"failure_mode": "reject", "output_format": "avro",
						"schema_source": map[string]any{
							"type": "reference", "subject": "'orders-value'", "version": "'latest'",
							"schema_registry": map[string]any{"id": registryID},
						},
					},
				},
			}
			cli := &harness.CLI{LastCommandDir: t.TempDir()}
			err := executeAssertions(cli, path, sc, Step{}, verify, parent, t.TempDir(), "transcode", "verify", nil)
			if registryID == "registry-id" {
				if err != nil {
					t.Fatalf("matching registry rejected: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "assertion mismatch") {
				t.Fatalf("mismatched registry must fail comparison, got %v", err)
			}
		})
	}
}
