//go:build e2e

package scenario

import (
	"fmt"
	"os"
	"strings"
)

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
