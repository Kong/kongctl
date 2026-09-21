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

func TestExternalParentRecoversPartialSync(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skipf("python3 is required for the CLI fixture: %v", err)
	}

	for _, tc := range []struct{ name, failure string }{
		{"recovered", ""},
		{"wrong-initial-action", "000-plan-create"},
		{"missing-version", "002-get-api-versions"},
		{"wrong-publication", "003-get-api-publications"},
		{"missing-role", "004-get-team-roles"},
		{"not-converged", "000-plan-noop"},
		{"wrong-delete-plan", "000-plan-delete"},
		{"not-deleted", "002-get-api-versions"},
		{"missing-parent", "004-get-external-api"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KONGCTL_E2E_UPDATE_EXPECT", "0")
			path := "../../scenarios/external/api-parent/scenario.yaml"
			s, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			// Run the actual lifecycle contracts, replacing only bootstrap/reset and CLI I/O.
			s.Steps = s.Steps[2:5]
			s.BaseInputsPath = ""
			s.Vars["api_id"] = "api-id"
			s.Vars["portal_id"] = "portal-id"
			s.Vars["auth_strategy_id"] = "auth-id"
			s.Defaults.Retry = Retry{Attempts: 2, Interval: "1ms", MaxInterval: "1ms", Jitter: "0s"}
			for i := range s.Steps {
				s.Steps[i].SkipInputs = true
				s.Steps[i].InputOverlayDirs = nil
			}
			dir := t.TempDir()
			script := `#!/usr/bin/env python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ['STATE'])
case = os.environ['CASE']
args = sys.argv[1:]
verb = args[0]
def emit(value):
 print(json.dumps(value))
def count(name):
 path = root / name
 n = int(path.read_text()) + 1 if path.exists() else 1
 path.write_text(str(n))
 return n
if verb == 'plan':
 n = count('plans')
 if n == 1:
  action = 'UPDATE' if case == 'wrong-initial-action' else 'CREATE'
  emit({'changes': [
   {'resource_type': 'api_version', 'action': action, 'parent': {'id': 'api-id'}},
   {'resource_type': 'api_publication', 'action': 'CREATE', 'parent': {'id': 'api-id'},
    'fields': {'auth_strategy_ids': ['auth-id']}},
   {'resource_type': 'organization_team_role', 'action': 'CREATE',
    'fields': {'entity_id': 'api-id', 'entity_type_name': 'APIs'}}]})
 elif n == 2:
  emit({'summary': {'total_changes': 1 if case == 'not-converged' else 0}})
 else:
  action = 'UPDATE' if case == 'wrong-delete-plan' else 'DELETE'
  emit({'changes': [{'resource_type': r, 'action': action} for r in ['api_version', 'api_publication']],
        'summary': {'total_changes': 2, 'by_action': {'DELETE': 2}}})
elif verb == 'sync':
 n = count('syncs')
 if n in (1, 3):
  # Creation leaves a team/role behind; deletion removes one child before failing.
  emit({'summary': {'status': 'partial_success', 'applied': 1, 'failed': 1}})
  print('context deadline exceeded', file=sys.stderr)
  sys.exit(1)
 # The recovered plan omits already-applied work. No fixed change count is valid.
 emit({'summary': {'status': 'success', 'failed': 0, 'applied': 1},
       'plan': {'changes': [{'resource_type': 'api_version', 'action': 'CREATE' if n == 2 else 'DELETE'}]}})
elif verb == 'get':
 deleted = int((root / 'syncs').read_text()) >= 4
 if 'versions' in args:
  emit([] if (deleted and case != 'not-deleted') or case == 'missing-version' else [{'version': 'v1'}])
 elif 'publications' in args:
  emit([] if deleted else [{'portal_id': 'portal-id',
                           'visibility': 'private' if case == 'wrong-publication' else 'public',
                           'auth_strategy_ids': ['auth-id']}])
 elif 'roles' in args:
  emit([] if case == 'missing-role' else [{'role_name': 'Viewer', 'entity_id': 'api-id',
                                         'entity_type_name': 'APIs', 'entity_region': 'us'}])
 elif 'apis' in args:
  emit([] if case == 'missing-parent' else [{'id': 'api-id', 'name': 'External Parent API'}])
 else:
  sys.exit(2)
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
			recovered := 0
			for _, cmd := range d.Commands {
				if cmd.Command == "001-sync" {
					recovered++
					if len(cmd.Attempts) != 2 || cmd.RetryStop != "succeeded" {
						t.Fatalf("missing recovery: %+v", cmd)
					}
				}
			}
			if recovered != 2 {
				t.Fatalf("expected creation and deletion recovery, got %d", recovered)
			}
			for _, step := range []string{"002-sync-version-under-external-api", "004-sync-delete-children-under-external-api"} {
				matches, err := filepath.Glob(filepath.Join(dir, "steps", step, "commands", "*", "attempts", "000", "stdout.txt"))
				if err != nil || len(matches) != 1 {
					t.Fatalf("missing first attempt in %s: %v %v", step, matches, err)
				}
			}
		})
	}
}
