//go:build e2e

package scenario

import (
	"fmt"
	"os"
	"strings"
)

const BetaModeEnvName = "KONGCTL_E2E_BETA_MODE"

type Maturity string

const (
	MaturityStable Maturity = "stable"
	MaturityBeta   Maturity = "beta"
)

type BetaMode string

const (
	BetaModeFail BetaMode = "fail"
	BetaModeWarn BetaMode = "warn"
	BetaModeSkip BetaMode = "skip"
)

type scenarioPreflight struct {
	Maturity   Maturity
	SkipReason string
}

func BetaModeFromEnv() (BetaMode, error) {
	return parseBetaMode(os.Getenv(BetaModeEnvName))
}

func parseBetaMode(value string) (BetaMode, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return BetaModeFail, nil
	}

	mode := BetaMode(value)
	switch mode {
	case BetaModeFail, BetaModeWarn, BetaModeSkip:
		return mode, nil
	default:
		return "", fmt.Errorf(
			"invalid %s value %q: supported values are %q, %q, and %q",
			BetaModeEnvName,
			value,
			BetaModeFail,
			BetaModeWarn,
			BetaModeSkip,
		)
	}
}

func scenarioMaturity(s Scenario) (Maturity, error) {
	value := strings.TrimSpace(s.Test.Maturity)
	if value == "" {
		return MaturityStable, nil
	}

	maturity := Maturity(value)
	switch maturity {
	case MaturityStable, MaturityBeta:
		return maturity, nil
	default:
		return "", fmt.Errorf(
			"invalid scenario maturity %q: supported values are %q and %q",
			value,
			MaturityStable,
			MaturityBeta,
		)
	}
}

func IsAdvisoryFailure(maturity Maturity, mode BetaMode) bool {
	return maturity == MaturityBeta && mode == BetaModeWarn
}

func preflightScenario(s Scenario, mode BetaMode) (scenarioPreflight, error) {
	maturity, err := scenarioMaturity(s)
	if err != nil {
		return scenarioPreflight{}, err
	}
	if _, err := parseBetaMode(string(mode)); err != nil {
		return scenarioPreflight{}, err
	}
	if maturity == MaturityBeta && mode == BetaModeSkip {
		return scenarioPreflight{
			Maturity:   maturity,
			SkipReason: "skipping: beta scenario disabled by KONGCTL_E2E_BETA_MODE=skip",
		}, nil
	}

	return scenarioPreflight{
		Maturity:   maturity,
		SkipReason: skipScenarioReason(s),
	}, nil
}

// skipScenarioReason applies scenario-level preflight checks declared in scenario.yaml.
// This keeps optional, external-dependency scenarios (like Gmail-backed flows) from
// running by default unless explicitly opted in and configured.
func skipScenarioReason(s Scenario) string {
	if s.Test.Enabled != nil && !*s.Test.Enabled {
		return formatSkipReason(s.Test.Info, "scenario disabled via scenario.yaml")
	}

	if env := strings.TrimSpace(s.Test.EnabledByEnvVar); env != "" {
		if !truthyEnvValue(os.Getenv(env)) {
			return formatSkipReason(s.Test.Info, fmt.Sprintf("%s not enabled", env))
		}
	}

	if missing := missingEnvVars(s.Test.RequiredEnvVars); len(missing) > 0 {
		return formatSkipReason(s.Test.Info, fmt.Sprintf("missing required env %s", strings.Join(missing, ", ")))
	}

	return ""
}

func missingEnvVars(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	missing := make([]string, 0, len(names))
	for _, name := range names {
		n := strings.TrimSpace(name)
		if n == "" {
			continue
		}
		if strings.TrimSpace(os.Getenv(n)) == "" {
			missing = append(missing, n)
		}
	}
	return missing
}

func truthyEnvValue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on", "y":
		return true
	default:
		return false
	}
}

func formatSkipReason(info, fallback string) string {
	fallback = strings.TrimSpace(fallback)
	if fallback != "" && !strings.HasPrefix(strings.ToLower(fallback), "skipping:") {
		fallback = "skipping: " + fallback
	}

	info = strings.TrimSpace(info)
	if info == "" {
		return fallback
	}
	if fallback == "" {
		return info
	}
	return fmt.Sprintf("%s (%s)", info, fallback)
}
