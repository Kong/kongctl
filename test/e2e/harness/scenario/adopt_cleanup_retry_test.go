//go:build e2e

package scenario

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/test/e2e/harness"
)

func TestAdoptCleanupRecoversPartialDelete(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skipf("python3 is required for the CLI fixture: %v", err)
	}
	type target struct {
		Variable     string                 `json:"variable"`
		ResourceType resources.ResourceType `json:"resource_type"`
		Collection   []string               `json:"collection"`
	}
	type cleanupCase struct {
		name, failure string
		target        int
	}
	for _, scenario := range []struct {
		name    string
		targets []target
	}{
		{"create-portal-adopt-dump-plan", []target{{"portal_id", resources.ResourceTypePortal, []string{"portals"}}}},
		{"auth-strategy-adopt", []target{
			{"auth_strategy_id", resources.ResourceTypeApplicationAuthStrategy, []string{"auth-strategies"}},
			{"auth_strategy_id_2", resources.ResourceTypeApplicationAuthStrategy, []string{"auth-strategies"}},
		}},
		{"event-gateway-adopt", []target{
			{"egw1_id", resources.ResourceTypeEventGatewayControlPlane, []string{"event-gateways"}},
			{"egw2_id", resources.ResourceTypeEventGatewayControlPlane, []string{"event-gateways"}},
		}},
		{"full", []target{
			{"cp_id", resources.ResourceTypeControlPlane, []string{"gateway", "control-planes"}},
			{"portal_id", resources.ResourceTypePortal, []string{"portals"}},
			{"api_id", resources.ResourceTypeAPI, []string{"apis"}},
			{"ai_gateway_id", resources.ResourceTypeAIGateway, []string{"ai-gateways"}},
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			cases := []cleanupCase{{"zero-change-retry", "", 0}}
			if len(scenario.targets) > 1 {
				cases = append(cases, cleanupCase{"remaining-change-retry", "", 0})
			}
			for i := range scenario.targets {
				cases = append(cases, cleanupCase{"wrong-target", "000-plan-delete", i},
					cleanupCase{"still-present", "-get-", i})
			}
			for _, tc := range cases {
				t.Run(fmt.Sprintf("%s/%d", tc.name, tc.target), func(t *testing.T) {
					t.Setenv("KONGCTL_E2E_UPDATE_EXPECT", "0")
					path := "../../scenarios/adopt/" + scenario.name + "/scenario.yaml"
					s, err := Load(path)
					if err != nil {
						t.Fatal(err)
					}
					s.Steps = s.Steps[len(s.Steps)-1:]
					for _, target := range scenario.targets {
						s.Vars[target.Variable] = target.Variable + "-value"
					}
					s.Defaults.Retry = Retry{Attempts: 2, Interval: "1ms", MaxInterval: "1ms", Jitter: "0s"}
					dir := t.TempDir()
					data, err := json.Marshal(scenario.targets)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, "targets.json"), data, 0o600); err != nil {
						t.Fatal(err)
					}
					bin := filepath.Join(dir, "kongctl-fixture")
					//nolint:gosec // Executable test fixture.
					if err := os.WriteFile(bin, []byte(adoptCleanupFixture), 0o700); err != nil {
						t.Fatal(err)
					}
					cli := &harness.CLI{
						BinPath: bin, TestDir: dir, Timeout: 5 * time.Second,
						Env: append(os.Environ(), "STATE="+dir, "CASE="+tc.name, fmt.Sprintf("TARGET=%d", tc.target)),
					}
					d := newScenarioDiagnostics(filepath.Join(dir, "scenario-diagnostics.json"), path)
					err = executeScenario(t, path, s, cli, d)
					if tc.failure != "" {
						if err == nil || !strings.Contains(err.Error(), tc.failure) ||
							!strings.Contains(err.Error(), "assertion mismatch") {
							t.Fatalf("want assertion failure at %s, got %v", tc.failure, err)
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
								t.Fatalf("missing recovery: %+v", cmd)
							}
							return
						}
					}
					t.Fatal("delete command was not executed")
				})
			}
		})
	}
}

const adoptCleanupFixture = `#!/usr/bin/env python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ['STATE'])
targets = json.loads((root / 'targets.json').read_text())
case = os.environ['CASE']
selected = int(os.environ['TARGET'])
args = sys.argv[1:]
changes = [{'resource_id': t['variable'] + '-value', 'resource_type': t['resource_type'],
            'action': 'DELETE'} for t in targets]
def emit(value):
 print(json.dumps(value))
if args[0] == 'plan':
 if case == 'wrong-target':
  changes[selected]['resource_id'] = 'unrelated-id'
 emit({'metadata': {'mode': 'delete'}, 'changes': changes,
       'summary': {'total_changes': len(changes), 'by_action': {'DELETE': len(changes)}}})
elif args[0] == 'delete':
 state = root / 'deleted'
 if not state.exists():
  state.write_text('partial-or-complete')
  print('context deadline exceeded', file=sys.stderr)
  sys.exit(1)
 remaining = changes[-1:] if case == 'remaining-change-retry' else []
 emit({'plan': {'metadata': {'mode': 'delete'}, 'changes': remaining,
                'summary': {'total_changes': len(remaining), 'by_action': {'DELETE': len(remaining)}}},
       'summary': {'status': 'success', 'failed': 0, 'applied': len(remaining)}})
elif args[0] == 'get':
 target = targets[selected]
 data = [{'id': 'unrelated-resource'}]
 if case == 'still-present' and args[1:1+len(target['collection'])] == target['collection']:
  data.append({'id': target['variable'] + '-value'})
 emit(data)
else:
 sys.exit(2)
`
