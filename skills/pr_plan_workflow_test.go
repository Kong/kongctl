package skills

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestPRPlanWorkflowRetainsCommittedBytes(t *testing.T) {
	for _, tool := range []string{"bash", "git", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("PR workflow requires %s", tool)
		}
	}
	data, err := BundledFS.ReadFile("kongctl-ai-gateway/assets/github-actions/pr-plan-deploy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct{ Name, Run string }
		}
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	var generate string
	for _, step := range workflow.Jobs["plan"].Steps {
		if step.Name == "Generate plan and diff" {
			generate = step.Run
		}
	}
	if generate == "" {
		t.Fatal("missing plan generation step")
	}
	const committed = `{"metadata":{"generator":"kongctl/1.19.0","generated_at":"2026-09-30T00:00:00Z"},
"changes":[{"fields":{"description":"first"},"secret_writes":[{"source":"FIRST_KEY"}]}]}`
	timestampOnly := strings.ReplaceAll(committed, "00:00:00Z", "00:01:00Z")
	for _, tt := range []struct {
		name      string
		generated string
		existing  bool
		retain    bool
	}{
		{"first plan", timestampOnly, false, false},
		{"identical plan", committed, true, true},
		{"timestamp only", timestampOnly, true, true},
		{"timestamp and whitespace", timestampOnly + "\n", true, true},
		{"resource change", strings.ReplaceAll(timestampOnly, "first", "second"), true, false},
		{"generator change", strings.ReplaceAll(timestampOnly, "1.19.0", "1.20.0"), true, false},
		{"secret source change", strings.ReplaceAll(timestampOnly, "FIRST_KEY", "SECOND_KEY"), true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			write := func(name, content string, mode os.FileMode) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), mode); err != nil {
					t.Fatal(err)
				}
			}
			git := func(args ...string) {
				t.Helper()
				cmd := exec.CommandContext(t.Context(), "git", args...)
				cmd.Dir = dir
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v\n%s", args, err, output)
				}
			}
			for _, name := range []string{"ci", "bin"} {
				if err := os.Mkdir(filepath.Join(dir, name), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			git("init", "-q")
			git("config", "user.name", "Workflow fixture")
			git("config", "user.email", "fixture@example.invalid")
			if tt.existing {
				write("ci/plan.json", committed, 0o600)
				git("add", "ci/plan.json")
			}
			git("-c", "commit.gpgsign=false", "commit", "--allow-empty", "-qm", "Fixture")
			write("generated.json", tt.generated, 0o600)
			write("bin/kongctl", `#!/usr/bin/env bash
set -euo pipefail
case "$1" in
  version) echo '1.19.0 (fixture)' ;;
  get) echo fixture-org ;;
  plan) cp "$GENERATED_PLAN" ci/plan.json ;;
  diff) printf 'diff:\n'; cat "$3" ;;
  *) exit 1 ;;
esac
`, 0o700)
			if _, err := exec.LookPath("sha256sum"); err != nil {
				if _, err := exec.LookPath("shasum"); err != nil {
					t.Skip("workflow requires sha256sum or shasum")
				}
				write("bin/sha256sum", "#!/bin/sh\nexec shasum -a 256 \"$@\"\n", 0o700)
			}
			t.Setenv("PATH", filepath.Join(dir, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("GENERATED_PLAN", filepath.Join(dir, "generated.json"))
			t.Setenv("KONGCTL_VERSION", "1.19.0")
			t.Setenv("KONNECT_REGION", "us")
			t.Setenv("KONNECT_ORG_ID", "fixture-org")
			t.Setenv("KONNECT_NAMESPACE", "fixture-ns")
			cmd := exec.CommandContext(t.Context(), "bash", "-c", generate)
			cmd.Dir = dir
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("generate plan: %v\n%s", err, output)
			}
			expected := []byte(tt.generated)
			if tt.retain {
				expected = []byte(committed)
			}
			for name, want := range map[string][]byte{
				"ci/plan.json":             expected,
				"evidence/diff.txt":        append([]byte("diff:\n"), expected...),
				"evidence/plan-sha256.txt": fmt.Appendf(nil, "%x  ci/plan.json\n", sha256.Sum256(expected)),
			} {
				actual, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(actual, want) {
					t.Fatalf("%s: got %q, want %q", name, actual, want)
				}
			}
			git("add", "ci/plan.json")
			cmd = exec.CommandContext(t.Context(), "git", "diff", "--cached", "--quiet")
			cmd.Dir = dir
			if output, err := cmd.CombinedOutput(); (err == nil) != tt.retain {
				t.Fatalf("unchanged index = %v, error = %v, output = %s", tt.retain, err, output)
			}
		})
	}
}
