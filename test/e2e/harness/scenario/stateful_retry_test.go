//go:build e2e

package scenario

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kong/kongctl/test/e2e/harness"
)

func TestAllRecoversPartialSync(t *testing.T) {
	for _, tc := range []struct{ name, failure string }{
		{"recovered", ""},
		{"wrong-initial-action", "000-plan-all"},
		{"missing-resource", "001-get-portals"},
		{"wrong-field", "001-get-portals"},
		{"not-converged", "000-sync-no-op"},
		{"not-deleted", "000-get-portals"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KONGCTL_E2E_UPDATE_EXPECT", "0")
			dir := t.TempDir()
			path := "../../scenarios/all/scenario.yaml"
			s, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			// Exercise the actual scenario contracts with a small remote-state fixture.
			// The other resource types follow the same initial-plan/readback pattern.
			initial := s.Steps[2]
			plan := initial.Commands[0]
			var assertions []Assertion
			for _, as := range plan.Assertions {
				if as.Select == "metadata" || strings.Contains(as.Select, "all-key-auth") ||
					strings.Contains(as.Select, "all-api-implementation") {
					assertions = append(assertions, as)
				}
			}
			if len(assertions) != 3 {
				t.Fatalf("initial plan coverage missing: %+v", assertions)
			}
			plan.Assertions = assertions
			initial.Commands = []Command{plan, initial.Commands[1], initial.Commands[2]}
			deleted := s.Steps[5]
			deleted.Commands = deleted.Commands[:1]
			s.Steps = []Step{initial, s.Steps[3], s.Steps[4], deleted}
			s.BaseInputsPath = ""
			s.Defaults.Retry = Retry{Attempts: 2, Interval: "1ms", MaxInterval: "1ms", Jitter: "0s"}
			for i := range s.Steps {
				s.Steps[i].SkipInputs = true
				s.Steps[i].InputOverlayOpsFiles = nil
			}
			script := `#!/bin/sh
set -eu
case "$1" in
plan)
 action=CREATE
 [ "$CASE" != wrong-initial-action ] || action=UPDATE
 printf '{"metadata":{"mode":"sync"},"changes":[
 {"resource_type":"application_auth_strategy","resource_ref":"all-key-auth","action":"%s"},
 {"resource_type":"api_implementation","resource_ref":"all-api-implementation","action":"CREATE"}]}\n' "$action"
 ;;
sync)
 if [ ! -f "$STATE/created" ]; then
  touch "$STATE/created"
  echo '{"summary":{"failed":1},"plan":{"changes":[{"resource_ref":"all-key-auth","action":"CREATE"}]}}'
  echo 'connection reset by peer' >&2
  exit 1
 fi
 if [ ! -f "$STATE/completed" ]; then
  touch "$STATE/completed"
  echo '{"plan":{"metadata":{"mode":"sync"},"changes":[
  {"resource_type":"api_implementation","resource_ref":"all-api-implementation","action":"CREATE"}]},
  "summary":{"failed":0,"status":"success"}}'
 else
  count=0
  [ "$CASE" != not-converged ] || count=1
  printf '{"plan":{"metadata":{"mode":"sync"},"summary":{"total_changes":%s},"changes":[]},
  "summary":{"total_changes":0,"failed":0,"status":"success"}}\n' "$count"
 fi
 ;;
get)
 if [ "$CASE" = missing-resource ] || { [ -f "$STATE/deleted" ] && [ "$CASE" != not-deleted ]; }; then
  echo '[]'
 else
  display='All Resources Portal'
  [ "$CASE" != wrong-field ] || display=wrong
  printf '[{"name":"all-portal","display_name":"%s"}]\n' "$display"
 fi
 ;;
delete)
 touch "$STATE/deleted"
 echo '{"plan":{"metadata":{"mode":"delete"}},"summary":{"failed":0,"status":"success"}}'
 ;;
*) exit 2 ;;
esac
`
			bin := filepath.Join(dir, "kongctl-fixture")
			if err := os.WriteFile(bin, []byte(script), 0o700); err != nil { //nolint:gosec // Executable test fixture.
				t.Fatal(err)
			}
			cli := &harness.CLI{
				BinPath: bin,
				TestDir: dir,
				Timeout: time.Second,
				Env:     append(os.Environ(), "STATE="+dir, "CASE="+tc.name),
			}
			d := newScenarioDiagnostics(filepath.Join(dir, "scenario-diagnostics.json"), path)
			err = executeScenario(t, path, s, cli, d)
			if tc.failure != "" {
				if err == nil || !strings.Contains(err.Error(), tc.failure) {
					t.Fatalf("want failure at %s, got %v", tc.failure, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := d.finish(nil); err != nil {
				t.Fatal(err)
			}
			sync := d.Commands[1]
			if len(sync.Attempts) != 2 || sync.RetryStop != "succeeded" {
				t.Fatalf("missing partial recovery: %+v", sync)
			}
			for _, artifact := range []string{
				"steps/002-sync-all/commands/000-sync-all/attempts/000/stdout.txt",
			} {
				data, err := os.ReadFile(filepath.Join(dir, artifact))
				if err != nil || !strings.Contains(string(data), "all-key-auth") {
					t.Fatalf("partial execution artifact missing: %s: %v", artifact, err)
				}
			}
			// The old CREATE assertion must fail on the recovered plan: the first
			// resource is absent, not merely changed to UPDATE.
			recovered, err := parseCommandOutput(
				"json",
				`{"plan":{"changes":[{
                    "resource_type":"api_implementation","resource_ref":"all-api-implementation","action":"CREATE"
                }]}}`,
			)
			if err != nil {
				t.Fatal(err)
			}
			old := assertions[1]
			old.Select = "plan." + old.Select
			oldCommand := Command{Assertions: []Assertion{old}}
			err = executeAssertions(cli, path, s, Step{}, oldCommand, recovered.Value(), dir, "old", "sync", nil)
			if err == nil {
				t.Fatal("old CREATE assertion unexpectedly passed after partial application")
			}
		})
	}
}

func TestImmutableAssertionsDoNotBackoff(t *testing.T) {
	for _, artifact := range []bool{false, true} {
		t.Run(map[bool]string{false: "stdout", true: "artifact"}[artifact], func(t *testing.T) {
			t.Setenv("KONGCTL_E2E_UPDATE_EXPECT", "0")
			dir := t.TempDir()
			cli := &harness.CLI{LastCommandDir: dir}
			as := Assertion{Expect: Expect{Fields: map[string]any{"name": "expected"}}}
			if artifact {
				if err := os.WriteFile(filepath.Join(dir, "output.json"), []byte(`{"name":"wrong"}`), 0o600); err != nil {
					t.Fatal(err)
				}
				as.Source.Artifact = &AssertionArtifactSource{Path: "output.json"}
			}
			cmd := Command{Retry: Retry{Attempts: 3, Interval: "2s", MaxInterval: "2s"}, Assertions: []Assertion{as}}
			started := time.Now()
			err := executeAssertions(
				cli,
				"scenario.yaml",
				Scenario{},
				Step{},
				cmd,
				map[string]any{"name": "wrong"},
				dir,
				"step",
				"cmd",
				nil,
			)
			if err == nil {
				t.Fatal("expected immutable mismatch")
			}
			if elapsed := time.Since(started); elapsed >= time.Second {
				t.Fatalf("immutable mismatch backed off: %s", elapsed)
			}
		})
	}
}

func TestFreshReadAssertionsPollAndPreserveParent(t *testing.T) {
	for _, first := range []string{"echo '[]'", "echo 'connection reset' >&2; exit 1"} {
		t.Run(first, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "read-fixture")
			script := "#!/bin/sh\nif [ ! -f \"$STATE/read\" ]; then touch \"$STATE/read\"; " +
				first + "; else echo '[{\"name\":\"ready\"}]'; fi\n"
			if err := os.WriteFile(bin, []byte(script), 0o700); err != nil { //nolint:gosec // Executable test fixture.
				t.Fatal(err)
			}
			parent := filepath.Join(dir, "parent")
			if err := os.Mkdir(parent, 0o755); err != nil {
				t.Fatal(err)
			}
			cli := &harness.CLI{
				BinPath:        bin,
				TestDir:        dir,
				LastCommandDir: parent,
				Timeout:        time.Second,
				Env:            append(os.Environ(), "STATE="+dir),
			}
			cmd := Command{
				Retry: Retry{Attempts: 2, Interval: "1ms", MaxInterval: "1ms", Jitter: "0s"},
				Assertions: []Assertion{
					{
						Source: AssertionSrc{Get: "portals"},
						Select: "[0]",
						Expect: Expect{Fields: map[string]any{"name": "ready"}},
					},
				},
			}
			err := executeAssertions(cli, "scenario.yaml", Scenario{}, Step{}, cmd, nil, dir, "step", "cmd", nil)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(parent, "assertions", "assert-000", "source.json"))
			if err != nil {
				t.Fatal(err)
			}
			var source struct {
				Kind    string
				Attempt int
			}
			if err := json.Unmarshal(data, &source); err != nil {
				t.Fatal(err)
			}
			if source.Kind != "get" || source.Attempt != 2 {
				t.Fatalf("wrong final assertion source: %s", data)
			}
			if cli.LastCommandDir != parent {
				t.Fatalf("lost assertion parent: %s", cli.LastCommandDir)
			}
			for _, attempt := range []string{"000", "001"} {
				files, err := filepath.Glob(
					filepath.Join(parent, "assertions", "assert-000", "retries", attempt, "*", "stdout.txt"),
				)
				if err != nil || len(files) != 1 {
					t.Fatalf("read attempt %s missing: %v %v", attempt, files, err)
				}
			}
		})
	}
}
