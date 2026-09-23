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

func TestSavedPlanBatchRetryContracts(t *testing.T) {
	t.Setenv("KONGCTL_E2E_UPDATE_EXPECT", "0")
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skipf("python3 required: %v", err)
	}
	for _, tc := range []struct {
		name, path  string
		step, start int
	}{
		{"teams-create", "org/teams/plan/apply-workflow", 1, 2},
		{"teams-update", "org/teams/plan/apply-workflow", 2, 2},
		{"system-account", "org/system-accounts/plan/apply-workflow", 2, 2},
		{"file-tags", "yaml-tags/file", 1, 1},
		{"teams-cleanup", "org/teams/plan/apply-workflow", 3, 0},
	} {
		faults := []string{"", "remaining", "readback"}
		if tc.name == "teams-create" || tc.name == "teams-update" {
			faults = append(faults, "duplicate")
		}
		if tc.name == "file-tags" {
			faults = append(faults, "convergence")
		}
		if tc.name != "teams-cleanup" {
			faults = append(faults, "stale-plan", "replan-error")
		}
		for _, fault := range faults {
			t.Run(tc.name+"/"+fault, func(t *testing.T) {
				path := "../../scenarios/" + tc.path + "/scenario.yaml"
				sc, err := Load(path)
				if err != nil {
					t.Fatal(err)
				}
				sc.Steps = sc.Steps[tc.step : tc.step+1]
				sc.Steps[0].Commands = sc.Steps[0].Commands[tc.start:]
				sc.Steps[0].SkipInputs = true
				sc.Steps[0].InputOverlayDirs = nil
				sc.BaseInputsPath = ""
				sc.Defaults.Retry = Retry{Attempts: 2, Interval: "1ms", MaxInterval: "1ms", Jitter: "0s"}
				if fault == "stale-plan" {
					sc.Steps[0].Commands[0].ReplanOnRetry = nil
				}
				dir := t.TempDir()
				bin := filepath.Join(dir, "fixture")
				//nolint:gosec // Executable test fixture.
				if err := os.WriteFile(bin, []byte(savedPlanBatchFixture), 0o700); err != nil {
					t.Fatal(err)
				}
				cli := &harness.CLI{
					BinPath: bin, TestDir: dir, Timeout: 5 * time.Second,
					Env: append(os.Environ(), "STATE="+dir, "CASE="+tc.name, "FAULT="+fault),
				}
				d := newScenarioDiagnostics(filepath.Join(dir, "diagnostics.json"), path)
				err = executeScenario(t, path, sc, cli, d)
				want := map[string]string{
					"readback": "assertion mismatch", "duplicate": "assertion mismatch",
					"convergence": "assertion mismatch", "stale-plan": "409 Conflict",
					"replan-error": "invalid manifest",
				}[fault]
				if want != "" {
					if err == nil || !strings.Contains(err.Error(), want) {
						t.Fatalf("want %q, got %v", want, err)
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
				if recovered != 1 {
					t.Fatalf("recovered=%d want 1", recovered)
				}
			})
		}
	}
}

const savedPlanBatchFixture = `#!/usr/bin/env python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ['STATE'])
case, fault = os.environ['CASE'], os.environ['FAULT']
args = sys.argv[1:]
def emit(value):
 if fault == 'duplicate' and isinstance(value, list) and value: value = value + [value[0]]
 print(json.dumps(value))
def fail(message):
 print(message, file=sys.stderr)
 sys.exit(1)
if args[0] == 'plan':
 if '--output-file' in args:
  if fault == 'replan-error': fail('invalid manifest')
  assert args[args.index('--mode')+1] == 'apply'
  expected = 'teams.yaml' if case.startswith('teams') else 'config.yaml'
  assert pathlib.Path(args[args.index('-f')+1]).name == expected
  pathlib.Path(args[args.index('--output-file')+1]).write_text('{"remaining":true}')
  emit({})
 elif case == 'teams-cleanup':
  emit({'summary': {'total_changes': 3, 'by_action': {'DELETE': 3}}})
 else: emit({'summary': {'total_changes': 1 if fault in ('readback', 'convergence') else 0}})
elif args[0] in ('apply', 'delete'):
 marker = root / 'mutated'
 if not marker.exists():
  marker.touch()
  fail('context deadline exceeded after partial mutation')
 if args[0] == 'apply':
  path = pathlib.Path(args[args.index('--plan')+1])
  if not path.exists() or not json.loads(path.read_text()).get('remaining'):
   fail('409 Conflict: original CREATE plan replayed')
 emit({'plan': {'metadata': {'mode': 'delete' if args[0] == 'delete' else 'apply'}, 'changes': []},
       'summary': {'applied': 1 if fault == 'remaining' else 0, 'failed': 0, 'status': 'success'}})
elif args[0] == 'diff':
 emit({'summary': {'total_changes': 1 if fault in ('readback', 'convergence') else 0}})
elif args[0] == 'get':
 if case == 'teams-cleanup':
  emit([{'name': 'Backend Team'}] if fault == 'readback' else [])
 elif fault == 'readback': emit([])
 elif case == 'teams-create':
  emit([{'name':'Backend Team','description':'Backend engineering team for plan apply workflow',
         'labels':{'department':'engineering','focus':'backend','tier':'core'}},
        {'name':'Frontend Team','description':'Frontend engineering team for plan apply workflow',
         'labels':{'focus':'frontend'}}])
 elif case == 'teams-update':
  emit([{'name':'Mobile Team','description':'Mobile app development team introduced during plan workflow',
         'labels':{'focus':'mobile','tier':'growth'}},
        {'name':'Backend Team','description':'Backend engineering team with updated scope and responsibilities',
         'labels':{'specialty':'microservices'}},
        {'name':'Frontend Team','description':'Frontend engineering team with expanded capabilities',
         'labels':{'tier':'core'}}])
 elif 'versions' in args: emit([{'version':'1.2.3','spec':{'type':'oas3'}}])
 elif 'portals' in args:
  emit([{'name':'portal-1','display_name':'Portal 1 Display',
         'description':'Portal 1 introduction loaded from an external Markdown file.', 'labels':{'team':'docs'}}])
 else:
  emit([{'name':'api-1','description':'API 1 summary sourced from a Markdown file via !file tag.',
         'labels':{'owner':'api-team'}}])
else: fail('unexpected command')
`
