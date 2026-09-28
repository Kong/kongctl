//go:build e2e

package scenario

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kong/kongctl/test/e2e/harness"
	"github.com/stretchr/testify/require"
)

func TestControlPlaneNamespaceReadbackPolling(t *testing.T) {
	for _, converges := range []bool{true, false} {
		name := "exhausted"
		if converges {
			name = "converges"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "fake-kongctl")
			const script = `#!/bin/sh
test "$1" = get && test "$2" = gateway && test "$3" = control-planes || exit 2
n=0
if test -f "$POLL_STATE"; then read -r n < "$POLL_STATE"; fi
n=$((n + 1))
printf '%s' "$n" > "$POLL_STATE"
namespace=old
if test "$CONVERGES" = yes && test "$n" -ge 2; then namespace=new; fi
printf '[{"id":"cp-id","labels":{"KONGCTL-namespace":"%s"}}]' "$namespace"
`
			// Only the owner executes this local subprocess fixture.
			err := os.WriteFile(bin, []byte(script), 0o700) //nolint:gosec
			require.NoError(t, err)
			state := filepath.Join(dir, "reads")
			env := []string{"POLL_STATE=" + state, "CONVERGES=no"}
			if converges {
				env[1] = "CONVERGES=yes"
			}
			cli := &harness.CLI{BinPath: bin, Env: env, LastCommandDir: dir, Timeout: time.Second * 10}
			command := Command{Assertions: []Assertion{{
				Name: "namespace-visible", Source: AssertionSrc{Get: "gateway control-planes"},
				Retry:  Retry{Attempts: 3, Interval: "1ms", MaxInterval: "1ms"},
				Select: "[?id=='cp-id'] | [0]",
				Expect: Expect{Fields: map[string]any{`labels."KONGCTL-namespace"`: "new"}},
			}}}
			err = executeAssertions(cli, "scenario.yaml", Scenario{}, Step{}, command, nil, dir, "overwrite", "adopt", nil)
			wantReads := "3"
			if converges {
				require.NoError(t, err)
				wantReads = "2"
			} else {
				require.ErrorContains(t, err, "assertion mismatch")
			}
			reads, err := os.ReadFile(state)
			require.NoError(t, err)
			require.Equal(t, wantReads, string(reads))
		})
	}
}
