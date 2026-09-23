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

func TestSavedPlanBatchTwoRecovery(t *testing.T) {
	t.Setenv("KONGCTL_E2E_UPDATE_EXPECT", "0")
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skipf("python3 required: %v", err)
	}
	// Explicit scope prevents a removed replan opt-in from silently dropping coverage.
	scenarios := map[string]int{
		"ai-gateway/agent":                     2,
		"ai-gateway/auth-strategy":             2,
		"ai-gateway/consumer-group":            2,
		"ai-gateway/data-plane-certificate":    1,
		"ai-gateway/mcp-server":                2,
		"ai-gateway/model-matrix":              1,
		"ai-gateway/policy":                    1,
		"ai-gateway/policy-matrix":             1,
		"ai-gateway/root":                      3,
		"ai-gateway/vault":                     2,
		"analytics/dashboard":                  1,
		"catalog/service":                      3,
		"control-plane/data-plane-certificate": 2,
		"dcr-providers/workflow":               2,
		"event-gateway/backend-cluster":        5,
		"event-gateway/cluster-policy":         3,
		"event-gateway/dataplane-certificate":  3,
		"event-gateway/listener":               3,
		"event-gateway/listener-policy":        3,
		"event-gateway/plan/apply-workflow":    12,
		"event-gateway/principal-metadata":     2,
		"event-gateway/produce-policy":         7,
		"event-gateway/schema-registry":        4,
		"event-gateway/static-key":             2,
		"event-gateway/tls-trust-bundle":       3,
		"event-gateway/topic-aliases":          2,
		"event-gateway/virtual-cluster":        4,
		"plan/apply-workflow":                  1,
		"portal/api_with_attributes":           1,
		"portal/app-auth-strategy":             1,
		"portal/applications":                  1,
		"portal/assets":                        2,
		"portal/audit-log-webhook":             2,
		"portal/auth_settings":                 1,
		"portal/custom-domain":                 3,
		"portal/page-frontmatter-conflict":     3,
		"portal/snippets":                      2,
	}
	for name, wantCount := range scenarios {
		t.Run(name, func(t *testing.T) {
			path := "../../scenarios/" + name + "/scenario.yaml"
			original, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, step := range original.Steps {
				for _, command := range step.Commands {
					if len(command.ReplanOnRetry) == 0 {
						continue
					}
					count++
					for _, outcome := range []string{"zero", "remaining", "replan-failure"} {
						t.Run(step.Name+"/"+command.Name+"/"+outcome, func(t *testing.T) {
							dir := t.TempDir()
							bin := filepath.Join(dir, "fixture")
							//nolint:gosec // Executable test fixture.
							if err := os.WriteFile(bin, []byte(batchTwoFixture), 0o700); err != nil {
								t.Fatal(err)
							}
							sc := original
							sc.BaseInputsPath = ""
							sc.Steps = []Step{{Name: step.Name, SkipInputs: true, Commands: []Command{command}}}
							sc.Defaults.Retry = Retry{Attempts: 2, Interval: "1ms", MaxInterval: "1ms", Jitter: "0s"}
							cli := &harness.CLI{
								BinPath: bin, TestDir: dir, Timeout: 5 * time.Second,
								Env: append(os.Environ(), "STATE="+dir, "OUTCOME="+outcome),
							}
							d := newScenarioDiagnostics(filepath.Join(dir, "diagnostics.json"), path)
							err := executeScenario(t, path, sc, cli, d)
							if outcome == "replan-failure" {
								if err == nil || !strings.Contains(err.Error(), "invalid manifest") {
									t.Fatalf("replan failure lost: %v", err)
								}
								data, err := os.ReadFile(filepath.Join(dir, "applies"))
								if err != nil || string(data) != "1" {
									t.Fatalf("stale plan executed: %s %v", data, err)
								}
								return
							}
							if err != nil {
								t.Fatal(err)
							}
							if err := d.finish(nil); err != nil {
								t.Fatal(err)
							}
							recovered := false
							for _, c := range d.Commands {
								if c.Command == command.Name && len(c.Attempts) == 2 && c.RetryStop == "succeeded" {
									recovered = true
								}
							}
							if !recovered {
								t.Fatal("apply did not recover")
							}
						})
					}
				}
			}
			if count != wantCount {
				t.Fatalf("replan coverage=%d want %d", count, wantCount)
			}
		})
	}
}

const batchTwoFixture = `#!/usr/bin/env python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ['STATE'])
args = sys.argv[1:]
mode = os.environ['OUTCOME']
def fail(message):
 print(message, file=sys.stderr)
 sys.exit(1)
if args[0] == 'apply':
 count = root / 'applies'
 n = int(count.read_text()) + 1 if count.exists() else 1
 count.write_text(str(n))
 path = pathlib.Path(args[args.index('--plan')+1])
 if n == 1:
  (root / 'plan-path').write_text(str(path))
  fail('context deadline exceeded after partial apply')
 if not path.exists() or json.loads(path.read_text()) != {'remaining': True}:
  fail('409 Conflict: stale CREATE plan replayed')
 print(json.dumps({'plan': {'metadata': {'mode': 'apply'}, 'changes': []},
                   'summary': {'applied': 0 if mode == 'zero' else 1,
                               'failed': 0, 'status': 'success'}}))
elif args[0] == 'plan':
 if mode == 'replan-failure': fail('invalid manifest')
 assert args[args.index('--mode')+1] == 'apply'
 assert '-f' in args
 path = pathlib.Path(args[args.index('--output-file')+1])
 assert str(path) == (root / 'plan-path').read_text()
 path.write_text(json.dumps({'remaining': True}))
 print('{}')
else: fail('unexpected command')
`
