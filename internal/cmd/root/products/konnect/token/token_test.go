package token

import (
	"errors"
	"strings"
	"testing"
	"time"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
)

func TestParseExpirationSupportsGoDurationsAndDays(t *testing.T) {
	tests := []struct {
		name      string
		expiresIn string
		wantTTL   int64
	}{
		{name: "hours", expiresIn: "12h", wantTTL: 43200},
		{name: "days", expiresIn: "30d", wantTTL: 2592000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseExpiration(tt.expiresIn, "")
			if err != nil {
				t.Fatalf("parseExpiration returned error: %v", err)
			}
			if got.TTLSeconds == nil || *got.TTLSeconds != tt.wantTTL {
				t.Fatalf("expected ttl %d, got %#v", tt.wantTTL, got.TTLSeconds)
			}
		})
	}
}

func TestParseExpirationSupportsRFC3339(t *testing.T) {
	got, err := parseExpiration("", "2026-06-24T12:00:00Z")
	if err != nil {
		t.Fatalf("parseExpiration returned error: %v", err)
	}
	want := time.Date(2026, time.June, 24, 12, 0, 0, 0, time.UTC)
	if got.ExpiresAt == nil || !got.ExpiresAt.Equal(want) {
		t.Fatalf("expected expires_at %s, got %#v", want.Format(time.RFC3339), got.ExpiresAt)
	}
}

func TestParseExpirationRequiresExactlyOneExpirationFlag(t *testing.T) {
	tests := []struct {
		name      string
		expiresIn string
		expiresAt string
	}{
		{name: "neither"},
		{name: "both", expiresIn: "12h", expiresAt: "2026-06-24T12:00:00Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseExpiration(tt.expiresIn, tt.expiresAt)
			var cfgErr *cmdpkg.ConfigurationError
			if !errors.As(err, &cfgErr) {
				t.Fatalf("expected ConfigurationError, got %T: %v", err, err)
			}
		})
	}
}

func TestParseExpirationInvalidDurationExplainsAcceptedUnits(t *testing.T) {
	_, err := parseExpiration("30days", "")
	var cfgErr *cmdpkg.ConfigurationError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigurationError, got %T: %v", err, err)
	}
	for _, want := range []string{
		`invalid --expires-in value "30days"`,
		"supported units: ns, us, ms, s, m, h, d",
		"90m, 12h, or 30d",
	} {
		if !strings.Contains(cfgErr.Error(), want) {
			t.Fatalf("expected error to contain %q, got %q", want, cfgErr.Error())
		}
	}
}

func TestValidateSystemAccountSelectorRequiresExactlyOneSelector(t *testing.T) {
	tests := []struct {
		name string
		id   string
		sa   string
		err  bool
	}{
		{name: "id", id: "sa-id"},
		{name: "name", sa: "ci-bot"},
		{name: "neither", err: true},
		{name: "both", id: "sa-id", sa: "ci-bot", err: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSystemAccountSelector(tt.id, tt.sa)
			if tt.err && err == nil {
				t.Fatal("expected error")
			}
			if !tt.err && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
