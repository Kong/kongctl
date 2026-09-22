//go:build e2e

package scenario

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kong/kongctl/test/e2e/harness"
)

func TestSystemAccountSyncRetryContracts(t *testing.T) {
	t.Setenv("KONGCTL_E2E_UPDATE_EXPECT", "0")
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skipf("python3 required: %v", err)
	}
	for phase := 2; phase <= 4; phase++ {
		// Both zero and nonzero final-attempt counts must satisfy the same state contract.
		faults := []string{"", "plan", "roles", "teams", "convergence", "remaining"}
		if phase == 4 {
			faults = append(faults, "api-survives")
		}
		for _, fault := range faults {
			t.Run(strconv.Itoa(phase)+"/"+fault, func(t *testing.T) {
				path := "../../scenarios/org/system-accounts/sync/scenario.yaml"
				s, err := Load(path)
				if err != nil {
					t.Fatal(err)
				}
				s.Steps = s.Steps[phase : phase+1]
				s.BaseInputsPath = ""
				s.Steps[0].SkipInputs = true
				s.Steps[0].InputOverlayDirs = nil
				s.Defaults.Retry = Retry{Attempts: 2, Interval: "1ms", MaxInterval: "1ms", Jitter: "0s"}
				dir := t.TempDir()
				bin := filepath.Join(dir, "kongctl-fixture")
				//nolint:gosec // Executable test fixture.
				if err := os.WriteFile(bin, []byte(systemAccountFixture), 0o700); err != nil {
					t.Fatal(err)
				}
				cli := &harness.CLI{
					BinPath: bin, TestDir: dir, Timeout: 5 * time.Second,
					Env: append(os.Environ(), "STATE="+dir, "PHASE="+strconv.Itoa(phase), "FAULT="+fault),
				}
				d := newScenarioDiagnostics(filepath.Join(dir, "diagnostics.json"), path)
				err = executeScenario(t, path, s, cli, d)
				if fault != "" && fault != "remaining" {
					if err == nil || !strings.Contains(err.Error(), "assertion mismatch") {
						t.Fatalf("fault %s accepted: %v", fault, err)
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
					if len(cmd.Attempts) == 2 && cmd.RetryStop == "succeeded" {
						recovered++
					}
				}
				want := 1
				if phase == 4 {
					want = 2
				}
				if recovered != want {
					t.Fatalf("recovered=%d want %d", recovered, want)
				}
			})
		}
	}
}

const systemAccountFixture = `#!/usr/bin/env python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ['STATE'])
phase = int(os.environ['PHASE'])
fault = os.environ['FAULT']
args = sys.argv[1:]
def emit(value): print(json.dumps(value))
def change(kind, action, ref='', role=None):
 c = {'resource_type': 'organization_'+kind, 'action': action,
      'namespace': 'org-sa-sync-e2e', 'resource_ref': ref}
 if role:
  c['fields'] = {'role_name': role, 'entity_type_name': 'APIs',
                 'entity_region': 'us' if role == 'Viewer' else '*', 'entity_id': '*'}
 return c
if args[0] == 'plan':
 if phase == 2:
  changes = [change('system_account_team_membership', 'CREATE'),
             change('system_account_role', 'CREATE', 'org-sa-sync-sa-api-viewer', 'Viewer')]
  summary = {'total_changes': 5, 'by_action': {'CREATE': 5}}
 elif phase == 3:
  changes = [change('system_account_team_membership', 'DELETE'),
             change('system_account_role', 'DELETE'),
             change('team_role', 'CREATE', 'org-sa-sync-team-wildcard-admin', 'Admin'),
             change('system_account_role', 'CREATE', role='Admin')]
  summary = {'total_changes': 4, 'by_action': {'CREATE': 2, 'DELETE': 2}}
 else:
  changes = [change('system_account_role', 'DELETE')]
  summary = {}
 emit({} if fault == 'plan' else {'summary': summary, 'changes': changes})
elif args[0] in ('apply', 'sync', 'delete'):
 marker = root / args[0]
 if not marker.exists():
  marker.touch()
  print('context deadline exceeded after partial mutation', file=sys.stderr)
  sys.exit(1)
 emit({'summary': {'applied': 3 if fault == 'remaining' else 0, 'failed': 0, 'status': 'success'},
       'plan': {'changes': [], 'summary': {'total_changes': 0}}})
elif args[0] == 'diff':
 emit({'summary': {'total_changes': 1 if fault == 'convergence' else 0}})
elif args[0] == 'get':
 if 'roles' in args:
  role = 'Viewer' if phase == 2 else 'Admin'
  values = [] if phase == 4 else [{'role_name': role, 'entity_type_name': 'APIs',
                                  'entity_region': 'us' if phase == 2 else '*', 'entity_id': '*'}]
  if fault == 'roles':
   values = [{'role_name': 'Viewer'}] if phase == 4 else []
 elif 'teams' in args:
  values = [{'name': 'Organization SA Sync Team'}] if phase == 2 else []
  if fault == 'teams': values = [] if phase == 2 else [{'name': 'Organization SA Sync Team'}]
 else:
  values = [{'name': 'org-sa-sync-api'}] if fault == 'api-survives' else []
 emit(values)
else: sys.exit(2)
`
