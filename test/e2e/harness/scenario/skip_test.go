//go:build e2e

package scenario

import (
	"testing"
)

func TestMatchesGlobPattern(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		pattern string
		want    bool
	}{
		// Exact matches
		{
			name:    "exact match",
			text:    "006-delete",
			pattern: "006-delete",
			want:    true,
		},
		{
			name:    "no match",
			text:    "006-delete",
			pattern: "007-delete",
			want:    false,
		},

		// Wildcard matches (note: inputs should be lowercase, case conversion happens in shouldSkipStep)
		{
			name:    "prefix wildcard",
			text:    "006-delete-application",
			pattern: "*delete*",
			want:    true,
		},
		{
			name:    "suffix wildcard",
			text:    "delete-something",
			pattern: "delete*",
			want:    true,
		},
		{
			name:    "prefix match",
			text:    "006-delete-app",
			pattern: "006-*",
			want:    true,
		},
		{
			name:    "middle wildcard",
			text:    "006-delete-application-registration",
			pattern: "006-*-registration",
			want:    true,
		},
		{
			name:    "question mark wildcard",
			text:    "006-delete",
			pattern: "00?-delete",
			want:    true,
		},
		{
			name:    "multiple wildcards",
			text:    "006-delete-portal-application",
			pattern: "*delete*portal*",
			want:    true,
		},

		// Invalid glob patterns (fallback to substring)
		{
			name:    "invalid glob brackets no substring match",
			text:    "006-delete-app",
			pattern: "[invalid",
			want:    false,
		},
		{
			name:    "invalid glob brackets with substring match",
			text:    "test[brackets",
			pattern: "[brackets",
			want:    true, // Invalid pattern, falls back to substring
		},

		// Edge cases
		{
			name:    "empty pattern matches empty text only",
			text:    "",
			pattern: "",
			want:    true,
		},
		{
			name:    "empty text",
			text:    "",
			pattern: "*",
			want:    true,
		},
		{
			name:    "special chars in text",
			text:    "006-delete_app-v2",
			pattern: "*delete_app*",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesGlobPattern(tt.text, tt.pattern)
			if got != tt.want {
				t.Errorf("matchesGlobPattern(%q, %q) = %v, want %v", tt.text, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestShouldSkipStep(t *testing.T) {
	tests := []struct {
		name     string
		stepName string
		patterns string
		want     bool
	}{
		// Empty patterns
		{
			name:     "empty patterns string",
			stepName: "006-delete-application",
			patterns: "",
			want:     false,
		},
		{
			name:     "whitespace only patterns",
			stepName: "006-delete-application",
			patterns: "   ",
			want:     false,
		},

		// Single pattern matches
		{
			name:     "single pattern matches",
			stepName: "006-delete-application-registration",
			patterns: "*delete*",
			want:     true,
		},
		{
			name:     "single pattern no match",
			stepName: "004-verify-portal-applications",
			patterns: "*delete*",
			want:     false,
		},
		{
			name:     "exact match",
			stepName: "006-delete-application-registration",
			patterns: "006-delete-application-registration",
			want:     true,
		},

		// Multiple patterns
		{
			name:     "multiple patterns first matches",
			stepName: "006-delete-application",
			patterns: "*delete*,*cleanup*",
			want:     true,
		},
		{
			name:     "multiple patterns second matches",
			stepName: "009-cleanup-resources",
			patterns: "*delete*,*cleanup*",
			want:     true,
		},
		{
			name:     "multiple patterns none match",
			stepName: "004-verify-portal",
			patterns: "*delete*,*cleanup*",
			want:     false,
		},
		{
			name:     "multiple patterns with spaces",
			stepName: "006-delete-application",
			patterns: " *delete* , *cleanup* ",
			want:     true,
		},
		{
			name:     "multiple patterns empty elements",
			stepName: "006-delete-application",
			patterns: ",*delete*,,",
			want:     true,
		},

		// Numbered step patterns
		{
			name:     "numbered prefix pattern matches",
			stepName: "006-delete-application",
			patterns: "006-*",
			want:     true,
		},
		{
			name:     "numbered prefix pattern no match",
			stepName: "007-delete-application",
			patterns: "006-*",
			want:     false,
		},
		{
			name:     "multiple numbered patterns",
			stepName: "008-verify-post-delete",
			patterns: "006-*,007-*,008-*",
			want:     true,
		},

		// Case insensitivity
		{
			name:     "case insensitive pattern uppercase",
			stepName: "006-delete-application",
			patterns: "*DELETE*",
			want:     true,
		},
		{
			name:     "case insensitive pattern mixed",
			stepName: "006-DELETE-APPLICATION",
			patterns: "*delete*",
			want:     true,
		},

		// Real-world examples from portal/applications scenario
		{
			name:     "portal apps delete registration step",
			stepName: "006-delete-application-registration",
			patterns: "*delete*",
			want:     true,
		},
		{
			name:     "portal apps delete application step",
			stepName: "007-delete-portal-application",
			patterns: "*delete*",
			want:     true,
		},
		{
			name:     "portal apps verify post delete",
			stepName: "008-verify-post-delete",
			patterns: "*delete*",
			want:     true,
		},
		{
			name:     "portal apps verify applications",
			stepName: "004-verify-portal-applications",
			patterns: "*delete*",
			want:     false,
		},
		{
			name:     "portal apps reset org",
			stepName: "001-reset-org",
			patterns: "*delete*",
			want:     false,
		},
		{
			name:     "skip all cleanup steps",
			stepName: "006-delete-application-registration",
			patterns: "006-*,007-*,008-*",
			want:     true,
		},

		// Complex patterns
		{
			name:     "complex pattern with wildcards",
			stepName: "006-delete-portal-application",
			patterns: "*delete*portal*",
			want:     true,
		},
		{
			name:     "question mark wildcard",
			stepName: "006-delete-application",
			patterns: "00?-delete*",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipStep(tt.stepName, tt.patterns)
			if got != tt.want {
				t.Errorf("shouldSkipStep(%q, %q) = %v, want %v", tt.stepName, tt.patterns, got, tt.want)
			}
		})
	}
}

func TestShouldStopAfter(t *testing.T) {
	tests := []struct {
		name            string
		stepName        string
		cmdName         string
		stopAfterSpec   string
		isLastCmdInStep bool
		want            bool
	}{
		// Empty/invalid specs
		{
			name:            "empty spec",
			stepName:        "001-step",
			cmdName:         "000-cmd",
			stopAfterSpec:   "",
			isLastCmdInStep: true,
			want:            false,
		},
		{
			name:            "whitespace only spec",
			stepName:        "001-step",
			cmdName:         "000-cmd",
			stopAfterSpec:   "   ",
			isLastCmdInStep: true,
			want:            false,
		},

		// Step-only format tests
		{
			name:            "step-only match last command",
			stepName:        "001-plan-apply-assets",
			cmdName:         "003-get-favicon-as-json",
			stopAfterSpec:   "001-plan-apply-assets",
			isLastCmdInStep: true,
			want:            true,
		},
		{
			name:            "step-only match NOT last command",
			stepName:        "001-plan-apply-assets",
			cmdName:         "001-apply-assets",
			stopAfterSpec:   "001-plan-apply-assets",
			isLastCmdInStep: false,
			want:            false,
		},
		{
			name:            "step-only wrong step",
			stepName:        "002-different-step",
			cmdName:         "000-cmd",
			stopAfterSpec:   "001-plan-apply-assets",
			isLastCmdInStep: true,
			want:            false,
		},
		{
			name:            "step-only case insensitive",
			stepName:        "001-Plan-Apply-Assets",
			cmdName:         "003-final-cmd",
			stopAfterSpec:   "001-PLAN-APPLY-ASSETS",
			isLastCmdInStep: true,
			want:            true,
		},
		{
			name:            "step-only with whitespace",
			stepName:        "001-step",
			cmdName:         "final",
			stopAfterSpec:   "  001-step  ",
			isLastCmdInStep: true,
			want:            true,
		},

		// Step/command format tests
		{
			name:            "step/command exact match",
			stepName:        "001-plan-apply-assets",
			cmdName:         "001-apply-assets",
			stopAfterSpec:   "001-plan-apply-assets/001-apply-assets",
			isLastCmdInStep: false,
			want:            true,
		},
		{
			name:            "step/command exact match is last",
			stepName:        "001-plan-apply-assets",
			cmdName:         "003-get-favicon",
			stopAfterSpec:   "001-plan-apply-assets/003-get-favicon",
			isLastCmdInStep: true,
			want:            true,
		},
		{
			name:            "step/command wrong step",
			stepName:        "001-plan-apply-assets",
			cmdName:         "001-apply-assets",
			stopAfterSpec:   "002-plan-apply-assets/001-apply-assets",
			isLastCmdInStep: false,
			want:            false,
		},
		{
			name:            "step/command wrong command",
			stepName:        "001-plan-apply-assets",
			cmdName:         "001-apply-assets",
			stopAfterSpec:   "001-plan-apply-assets/002-apply-assets",
			isLastCmdInStep: false,
			want:            false,
		},
		{
			name:            "step/command case insensitive",
			stepName:        "001-plan-apply-assets",
			cmdName:         "001-apply-assets",
			stopAfterSpec:   "001-PLAN-APPLY-ASSETS/001-APPLY-ASSETS",
			isLastCmdInStep: false,
			want:            true,
		},
		{
			name:            "step/command with whitespace",
			stepName:        "001-step",
			cmdName:         "000-cmd",
			stopAfterSpec:   "  001-step / 000-cmd  ",
			isLastCmdInStep: false,
			want:            true,
		},
		{
			name:            "step/command missing step name",
			stepName:        "001-step",
			cmdName:         "000-cmd",
			stopAfterSpec:   "/000-cmd",
			isLastCmdInStep: false,
			want:            false,
		},
		{
			name:            "step/command missing command name",
			stepName:        "001-step",
			cmdName:         "000-cmd",
			stopAfterSpec:   "001-step/",
			isLastCmdInStep: false,
			want:            false,
		},

		// Real-world examples from portal/assets scenario
		{
			name:            "portal assets - stop after step 001",
			stepName:        "001-plan-apply-assets",
			cmdName:         "003-get-favicon-as-json",
			stopAfterSpec:   "001-plan-apply-assets",
			isLastCmdInStep: true,
			want:            true,
		},
		{
			name:            "portal assets - stop after specific command",
			stepName:        "001-plan-apply-assets",
			cmdName:         "001-apply-assets",
			stopAfterSpec:   "001-plan-apply-assets/001-apply-assets",
			isLastCmdInStep: false,
			want:            true,
		},
		{
			name:            "portal assets - plan command",
			stepName:        "001-plan-apply-assets",
			cmdName:         "000-plan-assets",
			stopAfterSpec:   "001-plan-apply-assets/000-plan-assets",
			isLastCmdInStep: false,
			want:            true,
		},
		{
			name:            "portal assets - get logo",
			stepName:        "001-plan-apply-assets",
			cmdName:         "002-get-logo-as-json",
			stopAfterSpec:   "001-plan-apply-assets/002-get-logo-as-json",
			isLastCmdInStep: false,
			want:            true,
		},
		{
			name:            "portal assets - update step",
			stepName:        "002-plan-apply-assets-update",
			cmdName:         "002-verify-updated-logo",
			stopAfterSpec:   "002-plan-apply-assets-update",
			isLastCmdInStep: true,
			want:            true,
		},
		{
			name:            "portal assets - reset org",
			stepName:        "000-reset-org",
			cmdName:         "command-000",
			stopAfterSpec:   "000-reset-org",
			isLastCmdInStep: true,
			want:            true,
		},

		// Edge cases with auto-generated names
		{
			name:            "auto-generated step name",
			stepName:        "step-000",
			cmdName:         "command-001",
			stopAfterSpec:   "step-000/command-001",
			isLastCmdInStep: false,
			want:            true,
		},
		{
			name:            "auto-generated command name",
			stepName:        "001-my-step",
			cmdName:         "command-000",
			stopAfterSpec:   "001-my-step/command-000",
			isLastCmdInStep: false,
			want:            true,
		},
		{
			name:            "auto-generated stop after step",
			stepName:        "step-003",
			cmdName:         "command-005",
			stopAfterSpec:   "step-003",
			isLastCmdInStep: true,
			want:            true,
		},

		// Special characters in names
		{
			name:            "names with underscores",
			stepName:        "001-plan_apply_assets",
			cmdName:         "001-apply_assets",
			stopAfterSpec:   "001-plan_apply_assets/001-apply_assets",
			isLastCmdInStep: false,
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldStopAfter(tt.stepName, tt.cmdName, tt.stopAfterSpec, tt.isLastCmdInStep)
			if got != tt.want {
				t.Errorf("shouldStopAfter(%q, %q, %q, %v) = %v, want %v",
					tt.stepName, tt.cmdName, tt.stopAfterSpec, tt.isLastCmdInStep, got, tt.want)
			}
		})
	}
}
