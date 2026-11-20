//go:build e2e

package scenario

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kong/kongctl/test/e2e/harness"
)

type RecoveryMode string

const (
	RecoveryReport RecoveryMode = "report"
	RecoveryActive RecoveryMode = "active"
	RecoveryOff    RecoveryMode = "off"
)

func RecoveryModeFromEnv() (RecoveryMode, error) {
	mode := RecoveryMode(strings.TrimSpace(os.Getenv("KONGCTL_E2E_SCENARIO_RETRY")))
	if mode == "" {
		mode = RecoveryReport
	}
	switch mode {
	case RecoveryReport, RecoveryActive, RecoveryOff:
		return mode, nil
	default:
		return "", fmt.Errorf("KONGCTL_E2E_SCENARIO_RETRY must be report, active or off, got %q", mode)
	}
}

type recoveryResult struct {
	Scenario   string       `json:"scenario"`
	Mode       RecoveryMode `json:"mode"`
	Outcome    string       `json:"outcome"`
	Eligible   bool         `json:"eligible"`
	Class      string       `json:"class,omitempty"`
	FirstError string       `json:"first_error,omitempty"`
	RetryError string       `json:"retry_error,omitempty"`
	ResetError string       `json:"reset_error,omitempty"`
}

// RecoveryBlockedError is always fatal, including for advisory beta scenarios:
// proceeding after a failed reset would reuse an unclean organization.
type RecoveryBlockedError struct{ Cause error }

func (e *RecoveryBlockedError) Error() string { return e.Cause.Error() }
func (e *RecoveryBlockedError) Unwrap() error { return e.Cause }

func IsRecoveryBlocked(err error) bool {
	var blocked *RecoveryBlockedError
	return errors.As(err, &blocked)
}

func recoverScenario(mode RecoveryMode, run func(int) error, reset func() error) (recoveryResult, error) {
	r := recoveryResult{Mode: mode, Outcome: "first_pass"}
	first := run(1)
	if first == nil {
		return r, nil
	}
	r.Outcome, r.FirstError = "failed", first.Error()
	var failure *harness.ScenarioFailure
	if !IsRecoveryBlocked(first) && errors.As(first, &failure) {
		r.Eligible, r.Class = true, failure.Class
	}
	if !r.Eligible || mode != RecoveryActive {
		return r, first
	}
	if err := reset(); err != nil {
		r.Outcome, r.ResetError = "reset_failed", err.Error()
		return r, &RecoveryBlockedError{Cause: fmt.Errorf("recovery reset failed: %w (first failure: %w)", err, first)}
	}
	if err := run(2); err != nil {
		r.Outcome, r.RetryError = "exhausted", err.Error()
		return r, fmt.Errorf("scenario recovery exhausted: %w (first failure: %w)", err, first)
	}
	r.Outcome = "recovered"
	return r, nil
}

// RunWithRecovery keeps attempt errors out of testing.T until recovery finishes.
// Each attempt has separate artifacts and completes its registered cleanups
// before another reset or attempt is allowed to start.
func RunWithRecovery(t *testing.T, path string, beta BetaMode, mode RecoveryMode) (Maturity, error) {
	t.Helper()
	var maturity Maturity
	r, runErr := recoverScenario(mode, func(attempt int) error {
		var err error
		skipped := false
		ok := t.Run(fmt.Sprintf("attempt-%d", attempt), func(t *testing.T) {
			defer func() { skipped = t.Skipped() }()
			maturity, err = Run(t, path, beta)
		})
		if !ok {
			return &RecoveryBlockedError{Cause: errors.Join(
				fmt.Errorf("scenario attempt %d reported a failure to testing.T", attempt), err,
			)}
		}
		if skipped {
			t.Skip("scenario attempt skipped")
		}
		return err
	}, func() error { return resetForRecovery(t, path) })
	r.Scenario = filepath.ToSlash(path)
	cli, err := harness.NewCLIT(t)
	if err != nil {
		return maturity, &RecoveryBlockedError{Cause: errors.Join(fmt.Errorf("initialize recovery report: %w", err), runErr)}
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err == nil {
		err = os.WriteFile(filepath.Join(cli.TestDir, "scenario-recovery.json"), append(data, '\n'), 0o600)
	}
	if err != nil {
		return maturity, &RecoveryBlockedError{Cause: errors.Join(fmt.Errorf("write recovery report: %w", err), runErr)}
	}
	t.Logf("scenario outcome=%s retry_mode=%s eligible=%t class=%s", r.Outcome, mode, r.Eligible, r.Class)
	if r.FirstError != "" {
		t.Logf("first failure (preserved in attempt-1 artifacts): %s", r.FirstError)
	}
	return maturity, runErr
}

func resetForRecovery(t *testing.T, path string) error {
	// A skipped reset is not a fresh org and cannot authorize recovery.
	if value := strings.ToLower(strings.TrimSpace(os.Getenv("KONGCTL_E2E_RESET"))); value != "" &&
		!slices.Contains([]string{"1", "true", "yes", "on", "y"}, value) {
		return fmt.Errorf("scenario recovery requires resetOrg; KONGCTL_E2E_RESET=%q", value)
	}
	if os.Getenv("KONGCTL_E2E_KONNECT_PAT") == "" {
		return fmt.Errorf("scenario recovery requires KONGCTL_E2E_KONNECT_PAT for resetOrg")
	}
	s, err := Load(path)
	if err != nil {
		return err
	}
	var regions []string
	// Reset every region in the scenario's resetOrg contract. Include the
	// default region when any reset has no explicit regions, or none exists.
	hasReset, defaultRegion := false, false
	for _, step := range s.Steps {
		for _, command := range step.Commands {
			if command.ResetOrg {
				if len(command.ResetRegions) == 0 {
					defaultRegion = true
				}
				regions = append(regions, command.ResetRegions...)
				hasReset = true
			}
		}
	}
	defaultRegion = defaultRegion || !hasReset
	slices.Sort(regions)
	regions = slices.Compact(regions)
	cli, err := harness.NewCLIForArtifacts(t.Name()+"/recovery-reset", "tests")
	if err != nil {
		return err
	}
	step, err := harness.NewStep(t, cli, "reset")
	if err != nil {
		return err
	}
	if defaultRegion {
		if err := step.ResetOrg("scenario_recovery"); err != nil {
			return err
		}
	}
	if len(regions) > 0 {
		return step.ResetOrgForRegions("scenario_recovery", regions)
	}
	return nil
}
