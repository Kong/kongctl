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

func TestDeckMultiFilePartialApplyRecovery(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skipf("python3 required: %v", err)
	}
	for _, fault := range []string{
		"", "replan-transient", "replan-terminal", "replan-exhausted", "binding", "missing-service", "stale-plan",
	} {
		t.Run(fault, func(t *testing.T) {
			path := "../../scenarios/deck/multi-file/scenario.yaml"
			s, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			s.Steps = s.Steps[1:2]
			s.Defaults.Retry = Retry{Attempts: 3, Interval: "1ms", MaxInterval: "1ms", Jitter: "0s"}
			if fault == "stale-plan" {
				s.Steps[0].Commands[2].ReplanOnRetry = nil
			}
			dir := t.TempDir()
			bin := filepath.Join(dir, "kongctl-fixture")
			//nolint:gosec // Executable test fixture.
			if err := os.WriteFile(bin, []byte(savedPlanFixture), 0o700); err != nil {
				t.Fatal(err)
			}
			cli := &harness.CLI{
				BinPath: bin, TestDir: dir, Timeout: 5 * time.Second,
				Env: append(os.Environ(), "STATE="+dir, "FAULT="+fault),
			}
			d := newScenarioDiagnostics(filepath.Join(dir, "scenario-diagnostics.json"), path)
			err = executeScenario(t, path, s, cli, d)
			want := map[string]string{
				"replan-terminal": "invalid manifest", "replan-exhausted": "context deadline exceeded",
				"binding": "assertion mismatch", "missing-service": "recordVar failed", "stale-plan": "409 Conflict",
			}[fault]
			if want == "" && err != nil {
				t.Fatal(err)
			}
			if want != "" && (err == nil || !strings.Contains(err.Error(), want)) {
				t.Fatalf("want %q, got %v", want, err)
			}
			b, err := os.ReadFile(filepath.Join(dir, "applies"))
			if err != nil {
				t.Fatal(err)
			}
			expected := "2"
			if strings.HasPrefix(fault, "replan-") && fault != "replan-transient" {
				expected = "1"
			}
			if string(b) != expected {
				t.Fatalf("apply count = %s, want %s", b, expected)
			}
			plans, err := os.ReadFile(filepath.Join(dir, "plans"))
			if err != nil {
				t.Fatal(err)
			}
			wantPlans := "2"
			if fault == "replan-transient" || fault == "replan-exhausted" {
				wantPlans = "3"
			}
			if fault == "stale-plan" {
				wantPlans = "1"
			}
			if string(plans) != wantPlans {
				t.Fatalf("plan count = %s, want %s", plans, wantPlans)
			}
		})
	}
}

const savedPlanFixture = `#!/usr/bin/env python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ['STATE'])
fault = os.environ['FAULT']
args = sys.argv[1:]
def count(key):
 p = root / key
 n = int(p.read_text()) + 1 if p.exists() else 1
 p.write_text(str(n))
 return n
def fail(message):
 print(message, file=sys.stderr)
 sys.exit(1)
def emit(value):
 print(json.dumps(value))
if args[0] == 'plan':
 n = count('plans')
 inputs = [args[i+1] for i, arg in enumerate(args) if arg == '-f']
 if len(inputs) != 4 or any(not pathlib.Path(p).exists() for p in inputs):
  fail('missing plan inputs')
 if n > 1:
  if fault == 'replan-terminal': fail('invalid manifest')
  if fault == 'replan-exhausted' or (fault == 'replan-transient' and n == 2):
   fail('context deadline exceeded')
 p = pathlib.Path(args[args.index('--output-file')+1])
 p.write_text(json.dumps({'metadata': {'mode': 'apply'},
  'changes': [{'resource_type': '_deck'}], 'remaining': n > 1}))
 emit({})
elif args[0] == 'apply':
 n = count('applies')
 plan = json.loads(pathlib.Path(args[args.index('--plan')+1]).read_text())
 if n == 1:
  # Control plane, APIs and decK services committed before ID lookup timed out.
  fail('context deadline exceeded; read: connection reset by peer')
 if not plan['remaining']: fail('409 Conflict: resource already exists')
 emit({'plan': {'metadata': {'mode': 'apply'}},
       'summary': {'failed': 0, 'status': 'success', 'applied': 2}})
elif args[0] == 'get':
 if 'services' in args:
  emit([] if fault == 'missing-service' else [
   {'name': 'e2e-gw-svc-one', 'id': 'svc-one'},
   {'name': 'e2e-gw-svc-two', 'id': 'svc-two'}])
 elif 'control-planes' in args:
  emit([{'name': 'e2e-cp', 'id': 'cp-id'}])
 else:
  name = args[args.index('--api-name')+1]
  suffix = name.rsplit('-', 1)[1]
  emit([{'service': {'id': 'wrong' if fault == 'binding' else 'svc-'+suffix,
                    'control_plane_id': 'cp-id'}}])
else:
 fail('unexpected command')
`

func TestCommandRetryPlanValidation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		cmd       Command
		args      []string
		wantError bool
	}{
		{"disabled", Command{}, []string{"get", "apis"}, false},
		{"wrong-verb", Command{ReplanOnRetry: []string{"a.yaml"}}, []string{"sync"}, true},
		{"missing-plan", Command{ReplanOnRetry: []string{"a.yaml"}}, []string{"apply"}, true},
		{"empty-input", Command{ReplanOnRetry: []string{" "}}, []string{"apply", "--plan", "p.json"}, true},
		{
			"expected-failure",
			Command{ReplanOnRetry: []string{"a.yaml"}, ExpectFail: &ExpectedFailure{}},
			[]string{"apply", "--plan", "p.json"},
			true,
		},
		{"valid", Command{ReplanOnRetry: []string{"{{ .workdir }}/a.yaml"}}, []string{"apply", "--plan=p.json"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := commandRetryPlan(tc.cmd, tc.args, map[string]any{"workdir": "inputs"})
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.name == "valid" && strings.Join(plan.args, " ") != "plan --mode apply --output-file p.json -f inputs/a.yaml" {
				t.Fatalf("unexpected replan args: %v", plan.args)
			}
		})
	}
}
