//go:build e2e

package scenario

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kong/kongctl/test/e2e/harness"
)

// Diagnostics deliberately excludes arguments, output, error text, and URL paths:
// these can contain credentials or user data. Unknown causes stay unknown.
type scenarioDiagnostics struct {
	ProfileHTTPTimeoutMS int64               `json:"profile_http_timeout_ms"`
	SchemaVersion        int                 `json:"schema_version"`
	Scenario             string              `json:"scenario"`
	RunID                string              `json:"run_id,omitempty"`
	RunAttempt           string              `json:"run_attempt,omitempty"`
	SHA                  string              `json:"checkout_sha,omitempty"`
	WorkflowSHA          string              `json:"workflow_sha,omitempty"`
	Org                  string              `json:"org,omitempty"`
	Outcome              string              `json:"outcome"`
	Commands             []commandDiagnostic `json:"commands"`
	Failure              *terminalFailure    `json:"failure,omitempty"`

	path    string
	step    string
	phase   string
	current *commandDiagnostic
	started time.Time
}

type terminalFailure struct {
	Step    string `json:"step,omitempty"`
	Command string `json:"command,omitempty"`
	Phase   string `json:"phase"`
	Cause   string `json:"cause"`
}

type commandDiagnostic struct {
	Step         string              `json:"step"`
	Command      string              `json:"command"`
	DurationMS   int64               `json:"duration_ms"`
	Outcome      string              `json:"outcome"`
	AttemptLimit int                 `json:"attempt_limit,omitempty"`
	RetryStop    string              `json:"retry_stop,omitempty"`
	Attempts     []attemptDiagnostic `json:"attempts"`
}

type attemptDiagnostic struct {
	Kind       string `json:"kind"`
	DurationMS int64  `json:"duration_ms"`
	TimeoutMS  int64  `json:"timeout_ms"`
	TimedOut   bool   `json:"timed_out"`
	ExitCode   int    `json:"exit_code,omitempty"`
	Status     int    `json:"http_status,omitempty"`
	Method     string `json:"http_method,omitempty"`
	Host       string `json:"http_host,omitempty"`
	ErrorClass string `json:"error_class,omitempty"`
}

func newScenarioDiagnostics(path, scenario string) *scenarioDiagnostics {
	return &scenarioDiagnostics{
		ProfileHTTPTimeoutMS: harness.HTTPRequestTimeout().Milliseconds(),
		SchemaVersion:        1, Scenario: filepath.ToSlash(scenario),
		RunID: os.Getenv("GITHUB_RUN_ID"), RunAttempt: os.Getenv("GITHUB_RUN_ATTEMPT"),
		SHA: os.Getenv("KONGCTL_E2E_CHECKOUT_SHA"), WorkflowSHA: os.Getenv("GITHUB_SHA"),
		Org:     os.Getenv("KONGCTL_E2E_MATRIX_ORG"),
		Outcome: "in_progress", Commands: []commandDiagnostic{}, path: path, phase: "setup",
	}
}

func (d *scenarioDiagnostics) begin(step, command string) {
	d.finishCommand(nil)
	d.step, d.phase = step, "setup"
	d.current = &commandDiagnostic{Step: step, Command: command, Attempts: []attemptDiagnostic{}}
	d.started = time.Now()
}

func (d *scenarioDiagnostics) subprocess(res harness.Result, timeout time.Duration) {
	d.current.Attempts = append(d.current.Attempts, attemptDiagnostic{
		Kind: "subprocess", DurationMS: res.Duration.Milliseconds(), TimeoutMS: timeout.Milliseconds(),
		TimedOut: res.TimedOut, ExitCode: res.ExitCode,
	})
}

func (d *scenarioDiagnostics) http(a harness.HTTPAttempt) {
	d.current.Attempts = append(d.current.Attempts, attemptDiagnostic{
		Kind: "http", DurationMS: a.Duration.Milliseconds(), TimeoutMS: a.Timeout.Milliseconds(),
		TimedOut: a.TimedOut, Status: a.Status, Method: a.Method, Host: a.Host, ErrorClass: a.ErrorClass,
	})
}

func (d *scenarioDiagnostics) finishCommand(err error) {
	if d.current == nil {
		return
	}
	d.current.DurationMS = time.Since(d.started).Milliseconds()
	d.current.Outcome = "passed"
	if err != nil {
		d.current.Outcome = "failed"
	}
	d.Commands = append(d.Commands, *d.current)
	d.current = nil
}

func (d *scenarioDiagnostics) finish(err error) error {
	d.Outcome = "passed"
	if err != nil {
		d.Outcome = "failed"
		f := &terminalFailure{Step: d.step, Phase: d.phase, Cause: "unknown"}
		if d.current != nil {
			f.Command = d.current.Command
			attempts := d.current.Attempts
			if len(attempts) > 0 {
				last := attempts[len(attempts)-1]
				switch {
				case d.phase == "execution" && last.TimedOut:
					f.Cause = "subprocess_deadline"
				case d.phase == "execution" && last.ExitCode != 0:
					f.Cause = "subprocess_exit"
				case d.phase == "http" && last.TimedOut:
					f.Cause = "http_timeout"
				case d.phase == "http" && last.ErrorClass == "network":
					f.Cause = "transport_error"
				case d.phase == "http" && last.Status != 0 && last.ErrorClass == "":
					f.Cause = "http_status"
				}
			}
		}
		// Assertion failures can include reads as well as comparisons. Do not
		// attribute them to an earlier command's recovered transport error.
		d.Failure = f
	}
	d.finishCommand(err)
	b, marshalErr := json.MarshalIndent(d, "", "  ")
	if marshalErr != nil {
		return fmt.Errorf("marshal scenario diagnostics: %w", marshalErr)
	}
	if writeErr := os.WriteFile(d.path, append(b, '\n'), 0o600); writeErr != nil {
		return fmt.Errorf("write scenario diagnostics: %w", writeErr)
	}
	return nil
}
