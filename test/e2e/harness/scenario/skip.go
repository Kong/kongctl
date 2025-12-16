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

// getStopAfter reads and returns the KONGCTL_E2E_STOP_AFTER environment variable.
// Format: {step-name} or {step-name}/{command-name}
// Examples:
//   - "001-plan-apply-assets" - stops after the last command in step 001-plan-apply-assets
//   - "001-plan-apply-assets/001-apply-assets" - stops after specific command 001-apply-assets
// When set, the scenario will execute up to and including the specified step/command, then stop gracefully.
func getStopAfter() string {
	return os.Getenv("KONGCTL_E2E_STOP_AFTER")
}

// shouldStopAfter checks if the current step and command match the stop-after target.
// Returns true if execution should stop after this command completes.
//
// Supports two formats:
//   - {step-name}/{command-name}: stops after the specific command in the step
//   - {step-name}: stops after the last command in the step (requires isLastCmdInStep=true)
func shouldStopAfter(stepName, cmdName, stopAfterSpec string, isLastCmdInStep bool) bool {
	if strings.TrimSpace(stopAfterSpec) == "" {
		return false
	}

	// Check if spec contains '/' separator
	if strings.Contains(stopAfterSpec, "/") {
		// Format: {step-name}/{command-name}
		parts := strings.SplitN(stopAfterSpec, "/", 2)
		if len(parts) != 2 {
			return false
		}

		targetStep := strings.TrimSpace(parts[0])
		targetCmd := strings.TrimSpace(parts[1])

		if targetStep == "" || targetCmd == "" {
			return false
		}

		// Case-insensitive exact match for both step and command
		stepMatch := strings.EqualFold(stepName, targetStep)
		cmdMatch := strings.EqualFold(cmdName, targetCmd)

		return stepMatch && cmdMatch
	}

	// Format: {step-name} - only stop after last command in step
	targetStep := strings.TrimSpace(stopAfterSpec)
	if targetStep == "" {
		return false
	}

	// Case-insensitive step match, and must be the last command in the step
	stepMatch := strings.EqualFold(stepName, targetStep)
	return stepMatch && isLastCmdInStep
}
