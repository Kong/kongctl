//go:build e2e

package scenario

import (
	"os"
	"path/filepath"
	"strings"
)

// shouldSkipStep checks if stepName matches any pattern in the provided skip patterns.
// Patterns are comma-separated glob patterns (e.g., "*delete*,*cleanup*").
// Matching is case-insensitive. If a glob pattern is invalid, falls back to substring matching.
// Returns true if the step should be skipped, false otherwise.
func shouldSkipStep(stepName, patterns string) bool {
	if strings.TrimSpace(patterns) == "" {
		return false // default: skip nothing
	}

	stepLower := strings.ToLower(stepName)
	for _, pattern := range strings.Split(patterns, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if matchesGlobPattern(stepLower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// matchesGlobPattern performs case-insensitive glob matching with substring fallback.
// Uses filepath.Match for glob patterns. If the pattern is invalid, falls back to
// simple substring matching to avoid confusing users with pattern syntax errors.
func matchesGlobPattern(text, pattern string) bool {
	matched, err := filepath.Match(pattern, text)
	if err != nil {
		// Invalid glob pattern, fallback to substring match
		return strings.Contains(text, pattern)
	}
	return matched
}

// getSkipPatterns reads and returns the KONGCTL_E2E_SKIP_STEPS environment variable.
// This is a helper function to centralize env var access and make testing easier.
func getSkipPatterns() string {
	return os.Getenv("KONGCTL_E2E_SKIP_STEPS")
}
