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

	"github.com/kong/kongctl/test/e2e/harness"
)

func TestAdoptPlanConvergence(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skipf("python3 required: %v", err)
	}
	for _, mode := range []string{"ready", "stale", "persistent", "read-error"} {
		t.Run(mode, func(t *testing.T) {
			path := "../../scenarios/adopt/full/scenario.yaml"
			sc, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			cmd := sc.Steps[5].Commands[0]
			cmd.Assertions = cmd.Assertions[len(cmd.Assertions)-1:]
			cmd.Assertions[0].Retry = Retry{Attempts: 3, Interval: "1ms", MaxInterval: "1ms", Jitter: "0s"}
			dir := t.TempDir()
			bin := filepath.Join(dir, "fixture")
			//nolint:gosec // Executable test fixture.
			if err := os.WriteFile(bin, []byte(planConvergenceFixture), 0o700); err != nil {
				t.Fatal(err)
			}
			parent := filepath.Join(dir, "parent")
			if err := os.Mkdir(parent, 0o755); err != nil {
				t.Fatal(err)
			}
			cli := &harness.CLI{
				BinPath: bin, TestDir: dir, LastCommandDir: parent, Timeout: time.Second,
				Env: append(os.Environ(), "STATE="+dir, "MODE="+mode),
			}
			err = executeAssertions(cli, path, sc, Step{}, cmd, nil, dir, "dump-and-plan", "dump", nil)
			if mode == "persistent" {
				if err == nil || !strings.Contains(err.Error(), "assertion mismatch") {
					t.Fatalf("persistent difference accepted: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			attempts := 2
			if mode == "ready" {
				attempts = 1
			}
			if mode == "persistent" {
				attempts = 3
			}
			count, err := os.ReadFile(filepath.Join(dir, "count"))
			if err != nil || string(count) != fmt.Sprint(attempts) {
				t.Fatalf("wrong attempt count: %s %v", count, err)
			}
			if cli.LastCommandDir != parent {
				t.Fatal("lost parent directory")
			}
			for i := range attempts {
				pattern := filepath.Join(parent, "assertions", "plan-converges", "retries",
					fmt.Sprintf("%03d", i), "*", "stdout.txt")
				files, err := filepath.Glob(pattern)
				if err != nil || len(files) != 1 {
					t.Fatalf("attempt %d artifacts missing: %v %v", i, files, err)
				}
			}
			data, err := os.ReadFile(filepath.Join(parent, "assertions", "plan-converges", "source.json"))
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
			if source.Kind != "plan" || source.Attempt != attempts {
				t.Fatalf("wrong source: %s", data)
			}
		})
	}
}

const planConvergenceFixture = `#!/usr/bin/env python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ['STATE'])
assert sys.argv[1:] == ['plan', '-f', str(root / 'dump.yaml'), '--mode', 'apply'], sys.argv
p = root / 'count'
n = int(p.read_text()) + 1 if p.exists() else 1
p.write_text(str(n))
mode = os.environ['MODE']
if mode == 'read-error' and n == 1:
 print('connection reset by peer', file=sys.stderr)
 sys.exit(1)
stale = mode == 'persistent' or (mode == 'stale' and n == 1)
print(json.dumps({'summary': {'total_changes': 1 if stale else 0},
 'changes': [{'action': 'CREATE', 'resource_type': 'control_plane'}] if stale else []}))
`
