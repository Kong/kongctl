//go:build e2e

package harness

import (
	"slices"
	"testing"
)

func TestOnlyE2EEmailDomains(t *testing.T) {
	tests := []struct {
		name     string
		resource map[string]any
		want     bool
	}{
		{
			name:     "e2e mail domain",
			resource: map[string]any{"domain": "abc123.mail.kongctl-e2e.io"},
			want:     true,
		},
		{
			name:     "non e2e domain",
			resource: map[string]any{"domain": "example.com"},
			want:     false,
		},
		{
			name:     "missing domain",
			resource: map[string]any{"id": "abc123"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := onlyE2EEmailDomains(tt.resource); got != tt.want {
				t.Fatalf("onlyE2EEmailDomains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfiguredE2EUserEmails(t *testing.T) {
	t.Setenv("KONGCTL_E2E_ORG_USER_EMAIL_2", "user-2@example.com")
	t.Setenv("KONGCTL_E2E_ORG_USER_EMAIL_1", "user-1@example.com")
	t.Setenv("KONGCTL_E2E_ORG_USER_EMAIL_3", "user-2@example.com")
	t.Setenv("KONGCTL_E2E_ORG_USER_EMAIL_EMPTY", " ")
	t.Setenv("KONGCTL_E2E_OTHER_EMAIL", "ignored@example.com")

	got := configuredE2EUserEmails()
	want := []string{"user-1@example.com", "user-2@example.com"}
	if !slices.Equal(got, want) {
		t.Fatalf("configuredE2EUserEmails() = %#v, want %#v", got, want)
	}
}
