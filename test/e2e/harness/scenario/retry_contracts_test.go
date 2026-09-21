//go:build e2e

package scenario

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kong/kongctl/test/e2e/harness"
)

func TestScenarioContractsAfterPartialMutation(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skipf("python3 is required for the CLI fixture: %v", err)
	}
	for _, tc := range []struct {
		name, scenario string
		start, end     int
		plan           string
		reads          []string
	}{
		{
			"team-create", "org/teams/sync", 1, 2,
			`{
  "metadata": {
    "mode": "sync"
  },
  "summary": {
    "total_changes": 3,
    "by_action": {
      "CREATE": 3
    }
  }
}`,
			[]string{`[
  {
    "name": "Platform Team",
    "description": "Platform engineering and infrastructure team",
    "labels": {
      "department": "engineering",
      "tier": "core",
      "focus": "platform"
    }
  },
  {
    "name": "Security Team",
    "description": "Security and compliance team",
    "labels": {
      "tier": "critical"
    }
  },
  {
    "name": "Legacy Systems Team",
    "labels": {
      "lifecycle": "sunset"
    }
  }
]`},
		},
		{
			"team-update", "org/teams/sync", 2, 3,
			`{
  "metadata": {
    "mode": "sync"
  },
  "summary": {
    "total_changes": 4,
    "by_action": {
      "CREATE": 1,
      "UPDATE": 2,
      "DELETE": 1
    }
  }
}`,
			[]string{`[
  {
    "name": "Platform Team",
    "description": "Updated: Cloud platform engineering and DevOps infrastructure team",
    "labels": {
      "specialty": "cloud-native",
      "KONGCTL-namespace": "teams-sync-e2e"
    }
  },
  {
    "name": "Security Team",
    "description": "Updated: Application security, compliance, and risk management team",
    "labels": {
      "specialty": "appsec"
    }
  },
  {
    "name": "Data Engineering Team",
    "description": "Team focused on data pipelines, analytics, and ML infrastructure",
    "labels": {
      "focus": "data"
    }
  }
]`},
		},
		{
			"team-delete", "org/teams/sync", 3, 4,
			`{
  "metadata": {
    "mode": "delete"
  },
  "summary": {
    "total_changes": 3,
    "by_action": {
      "DELETE": 3
    }
  }
}`,
			[]string{`[]`},
		},
		{
			"declarative-delete", "delete/declarative", 2, 4,
			`{
  "metadata": {
    "mode": "delete"
  },
  "summary": {
    "total_changes": 7,
    "by_action": {
      "DELETE": 7
    }
  },
  "changes": [
    {
      "resource_type": "application_auth_strategy",
      "action": "DELETE"
    }
  ]
}`,
			[]string{`[]`, `[]`, `[]`, `[]`, `[]`, `[]`},
		},
		{
			"deck-create", "deck/sync", 1, 2,
			`{
  "metadata": {
    "mode": "sync"
  },
  "changes": [
    {
      "resource_type": "_deck"
    },
    {
      "resource_type": "api_implementation",
      "resource_ref": "code-breakers-api-impl",
      "action": "CREATE"
    }
  ]
}`,
			[]string{`[
  {
    "id": "service-id",
    "name": "code-breakers"
  }
]`, `[
  {
    "id": "cp-id",
    "name": "code-breakers"
  }
]`, `[
  {
    "service": {
      "id": "service-id",
      "control_plane_id": "cp-id"
    }
  }
]`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			faults := []string{"", "plan", "readback"}
			if tc.name == "team-update" {
				faults = append(faults, "convergence")
			}
			for _, fault := range faults {
				t.Run("fault="+fault, func(t *testing.T) {
					t.Setenv("KONGCTL_E2E_UPDATE_EXPECT", "0")
					path := "../../scenarios/" + tc.scenario + "/scenario.yaml"
					s, err := Load(path)
					if err != nil {
						t.Fatal(err)
					}
					s.Steps = s.Steps[tc.start:tc.end]
					s.BaseInputsPath = ""
					s.Defaults.Retry = Retry{Attempts: 2, Interval: "1ms", MaxInterval: "1ms", Jitter: "0s"}
					for i := range s.Steps {
						s.Steps[i].SkipInputs = true
						s.Steps[i].InputOverlayDirs = nil
					}
					dir := t.TempDir()
					reads := make([]json.RawMessage, len(tc.reads))
					for i, r := range tc.reads {
						reads[i] = json.RawMessage(r)
					}
					faultRead := 1
					if tc.name == "deck-create" {
						faultRead = 3
					}
					fixture, err := json.Marshal(map[string]any{
						"plan": json.RawMessage(tc.plan), "reads": reads, "fault_read": faultRead,
					})
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, "fixture.json"), fixture, 0o600); err != nil {
						t.Fatal(err)
					}
					bin := filepath.Join(dir, "kongctl-fixture")
					//nolint:gosec // Executable test fixture.
					if err := os.WriteFile(bin, []byte(partialMutationFixture), 0o700); err != nil {
						t.Fatal(err)
					}
					cli := &harness.CLI{
						BinPath: bin, TestDir: dir, Timeout: 5 * time.Second,
						Env: append(os.Environ(), "STATE="+dir, "FAULT="+fault),
					}
					d := newScenarioDiagnostics(filepath.Join(dir, "scenario-diagnostics.json"), path)
					err = executeScenario(t, path, s, cli, d)
					if fault != "" {
						if err == nil || !strings.Contains(err.Error(), "assertion mismatch") {
							t.Fatalf("%s fault accepted: %v", fault, err)
						}
						if fault == "plan" && !strings.Contains(err.Error(), "000-plan-") {
							t.Fatalf("wrong failure location: %v", err)
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
						t.Fatalf("expected one recovered mutation, got %d", recovered)
					}
				})
			}
		})
	}
}

const partialMutationFixture = `#!/usr/bin/env python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ['STATE'])
data = json.loads((root / 'fixture.json').read_text())
fault = os.environ['FAULT']
verb = sys.argv[1]
def count(key):
 path = root / key
 n = int(path.read_text()) + 1 if path.exists() else 1
 path.write_text(str(n))
 return n
def emit(value):
 print(json.dumps(value))
if verb == 'plan':
 n = count('plans')
 noop = {'summary': {'total_changes': 1 if fault == 'convergence' else 0}}
 emit({} if fault == 'plan' else data['plan'] if n == 1 else noop)
elif verb in ('sync', 'delete'):
 if count('mutations') == 1:
  # Model a committed mutation whose response was lost. Retry has no work left.
  print('context deadline exceeded', file=sys.stderr)
  sys.exit(1)
 emit({'summary': {'failed': 0, 'status': 'success', 'applied': 0},
       'plan': {'changes': [], 'summary': {'total_changes': 0, 'by_action': {}}}})
elif verb == 'get':
 n = count('reads')
 value = data['reads'][n-1]
 if fault == 'readback' and n == data['fault_read']:
  # Reject missing created resources, incorrect bindings, and surviving deletions.
  if value and 'service' in value[0]:
   value[0]['service']['id'] = 'wrong-service'
  else:
   value = [{'name': 'Platform Team'}, {'name': 'delete-test-portal'}] if not value else []
 emit(value)
else:
 sys.exit(2)
`
