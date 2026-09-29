package skills

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestAIGatewayWorkflowSecretGuard(t *testing.T) {
	for _, tool := range []string{"bash", "jq", "sha256sum"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("workflow guard requires %s", tool)
		}
	}
	data, err := BundledFS.ReadFile("kongctl-ai-gateway/assets/github-actions/deploy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string
				Run  string
			}
		}
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	var guard string
	for _, step := range workflow.Jobs["deploy"].Steps {
		if step.Name == "Check the reviewed plan" {
			guard = step.Run
		}
	}
	if guard == "" {
		t.Fatal("missing plan guard")
	}
	for _, tt := range []struct {
		name  string
		parts string
		allow bool
	}{
		{"no secret writes", "", true},
		{"provider key", `[{"source":{"kind":"env","reference":"OPENAI_API_KEY"}}]`, true},
		{"public prefix", `[{"literal":"Bearer "},{"source":{"kind":"env","reference":"OPENAI_API_KEY"}}]`, true},
		{"file source", `[{"source":{"kind":"file","reference":"provider-key.txt"}}]`, true},
		{"management token", `[{"source":{"kind":"env","reference":"KONGCTL_DEFAULT_KONNECT_PAT"}}]`, false},
		{"unmapped caller", `[{"source":{"kind":"env","reference":"CALLER_KEY"}}]`, false},
		{"GitHub token", `[{"source":{"kind":"env","reference":"GITHUB_TOKEN"}}]`, false},
		{"unknown source", `[{"source":{"kind":"other","reference":"OPENAI_API_KEY"}}]`, false},
		{"mixed part", `[{"literal":"Bearer ","source":{"kind":"env","reference":"KONGCTL_DEFAULT_KONNECT_PAT"}}]`, false},
		{"second source", `[
			{"source":{"kind":"env","reference":"OPENAI_API_KEY"}},
			{"source":{"kind":"env","reference":"KONGCTL_DEFAULT_KONNECT_PAT"}}
		]`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			change := map[string]any{"namespace": "ai-demo", "action": "CREATE"}
			if tt.parts != "" {
				change["secret_writes"] = []any{map[string]any{
					"expression": map[string]any{"parts": json.RawMessage(tt.parts)},
				}}
			}
			plan, err := json.Marshal(map[string]any{
				"metadata": map[string]string{"mode": "apply", "generator": "kongctl/1.16.0"},
				"changes":  []any{change},
			})
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "ci"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "ci", "plan.json"), plan, 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("GITHUB_REF", "refs/heads/main")
			t.Setenv("DEFAULT_BRANCH", "main")
			t.Setenv("EXPECTED_PLAN_SHA256", fmt.Sprintf("%x", sha256.Sum256(plan)))
			t.Setenv("KONNECT_NAMESPACE", "ai-demo")
			t.Setenv("KONGCTL_VERSION", "1.16.0")
			cmd := exec.CommandContext(t.Context(), "bash", "-c", guard)
			cmd.Dir = dir
			output, err := cmd.CombinedOutput()
			if (err == nil) != tt.allow {
				t.Fatalf("allow = %v, error = %v, output = %s", tt.allow, err, output)
			}
		})
	}
}
