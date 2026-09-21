//go:build e2e

package scenario

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kong/kongctl/test/e2e/harness"
)

func TestTeamCleanupRecoversCompletedDelete(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skipf("python3 is required for the CLI fixture: %v", err)
	}
	for _, tc := range []struct{ name, failure string }{
		{"recovered", ""},
		{"wrong-target", "000-plan-delete"},
		{"still-present", "002-get-teams"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KONGCTL_E2E_UPDATE_EXPECT", "0")
			path := "../../scenarios/adopt/team-create-adopt-dump/scenario.yaml"
			s, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			s.Steps = s.Steps[len(s.Steps)-1:]
			s.Vars["team_id"] = "team-id"
			s.Defaults.Retry = Retry{Attempts: 2, Interval: "1ms", MaxInterval: "1ms", Jitter: "0s"}
			dir := t.TempDir()
			script := `#!/usr/bin/env python3
import json, os, pathlib, sys
state = pathlib.Path(os.environ['STATE']) / 'deleted'
case = os.environ['CASE']
verb = sys.argv[1]
def emit(value):
 print(json.dumps(value))
if verb == 'plan':
 emit({'metadata': {'mode': 'delete'},
       'summary': {'total_changes': 1, 'by_action': {'DELETE': 1}},
       'changes': [{'action': 'DELETE', 'resource_type': 'organization_team',
                    'resource_id': 'wrong-id' if case == 'wrong-target' else 'team-id'}]})
elif verb == 'delete':
 if not state.exists():
  state.write_text('deleted')
  print('context deadline exceeded', file=sys.stderr)
  sys.exit(1)
 emit({'plan': {'metadata': {'mode': 'delete'}, 'changes': [],
                'summary': {'total_changes': 0, 'by_action': {}}},
       'summary': {'status': 'success', 'failed': 0, 'applied': 0}})
elif verb == 'get':
 emit([{'id': 'unrelated-team'}] + ([{'id': 'team-id'}] if case == 'still-present' else []))
else:
 sys.exit(2)
`
			bin := filepath.Join(dir, "kongctl-fixture")
			if err := os.WriteFile(bin, []byte(script), 0o700); err != nil { //nolint:gosec // Executable test fixture.
				t.Fatal(err)
			}
			cli := &harness.CLI{
				BinPath: bin, TestDir: dir, Timeout: 5 * time.Second,
				Env: append(os.Environ(), "STATE="+dir, "CASE="+tc.name),
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
			for _, cmd := range d.Commands {
				if cmd.Command == "001-delete" {
					if len(cmd.Attempts) != 2 || cmd.RetryStop != "succeeded" {
						t.Fatalf("missing delete recovery: %+v", cmd)
					}
					return
				}
			}
			t.Fatal("delete command was not executed")
		})
	}
}
