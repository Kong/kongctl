//go:build e2e

package harness

import "testing"

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
