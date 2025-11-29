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
