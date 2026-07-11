package token

import (
	"errors"
	"strings"
	"testing"
	"time"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
)

func TestParseExpirationSupportsDurationsAndDays(t *testing.T) {
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

func TestParseCreateTokenExpirationAcceptsDurationUnitsWithinBounds(t *testing.T) {
	tests := []struct {
		name      string
		expiresIn string
		wantTTL   int64
	}{
		{name: "hours at minimum", expiresIn: "24h", wantTTL: minTokenTTLSeconds},
		{name: "hours above minimum", expiresIn: "36h", wantTTL: 36 * 60 * 60},
		{name: "minutes at minimum", expiresIn: "1440m", wantTTL: minTokenTTLSeconds},
		{name: "days", expiresIn: "30d", wantTTL: 30 * secondsPerDay},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCreateTokenExpiration(tt.expiresIn, "")
			if err != nil {
				t.Fatalf("parseCreateTokenExpiration returned error: %v", err)
			}
			if got.TTLSeconds == nil || *got.TTLSeconds != tt.wantTTL {
				t.Fatalf("expected ttl %d, got %#v", tt.wantTTL, got.TTLSeconds)
			}
		})
	}
}

func TestParseCreateTokenExpirationRejectsBelowMinDuration(t *testing.T) {
	_, err := parseCreateTokenExpiration("12h", "")
	var cfgErr *cmdpkg.ConfigurationError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigurationError, got %T: %v", err, err)
	}
	for _, want := range []string{
		"minimum token lifetime is 1 day",
		"--expires-in must be at least 1d",
	} {
		if !strings.Contains(cfgErr.Error(), want) {
			t.Fatalf("expected error to contain %q, got %q", want, cfgErr.Error())
		}
	}
}

func TestParseCreateTokenExpirationRejectsOverMaxDuration(t *testing.T) {
	_, err := parseCreateTokenExpiration("366d", "")
	var cfgErr *cmdpkg.ConfigurationError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigurationError, got %T: %v", err, err)
	}
	for _, want := range []string{
		"maximum token lifetime is 365 days (12 months)",
		"--expires-in must be at most 365d",
	} {
		if !strings.Contains(cfgErr.Error(), want) {
			t.Fatalf("expected error to contain %q, got %q", want, cfgErr.Error())
		}
	}
}

func TestParseCreateTokenExpirationRejectsExpiresAtOutsideBounds(t *testing.T) {
	now := time.Date(2026, time.May, 27, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		expiresAt string
		want      []string
	}{
		{
			name:      "too soon",
			expiresAt: now.Add(12 * time.Hour).Format(time.RFC3339),
			want: []string{
				"minimum token lifetime is 1 day",
				"--expires-at must be at least 1 day from now",
			},
		},
		{
			name:      "too far",
			expiresAt: now.Add(366 * 24 * time.Hour).Format(time.RFC3339),
			want: []string{
				"maximum token lifetime is 365 days (12 months)",
				"--expires-at must be at most 365 days from now",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseCreateTokenExpirationAt("", tt.expiresAt, now)
			var cfgErr *cmdpkg.ConfigurationError
			if !errors.As(err, &cfgErr) {
				t.Fatalf("expected ConfigurationError, got %T: %v", err, err)
			}
			for _, want := range tt.want {
				if !strings.Contains(cfgErr.Error(), want) {
					t.Fatalf("expected error to contain %q, got %q", want, cfgErr.Error())
				}
			}
		})
	}
}

func TestParseCreateTokenExpirationAcceptsBoundaryDurations(t *testing.T) {
	tests := []struct {
		name      string
		expiresIn string
		wantTTL   int64
	}{
		{name: "minimum", expiresIn: "1d", wantTTL: minTokenTTLSeconds},
		{name: "maximum", expiresIn: "365d", wantTTL: maxTokenTTLSeconds},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCreateTokenExpiration(tt.expiresIn, "")
			if err != nil {
				t.Fatalf("parseCreateTokenExpiration returned error: %v", err)
			}
			if got.TTLSeconds == nil || *got.TTLSeconds != tt.wantTTL {
				t.Fatalf("expected ttl %d, got %#v", tt.wantTTL, got.TTLSeconds)
			}
		})
	}
}

func TestParseCreateTokenExpirationAcceptsExpiresAtAtMinBoundaryWithSubsecondNow(t *testing.T) {
	now := time.Date(2026, time.May, 27, 12, 0, 0, 900_000_000, time.UTC)
	expiresAt := now.Truncate(time.Second).Add(24 * time.Hour).Format(time.RFC3339)
	got, err := parseCreateTokenExpirationAt("", expiresAt, now)
	if err != nil {
		t.Fatalf("parseCreateTokenExpiration returned error: %v", err)
	}
	if got.ExpiresAt == nil || got.ExpiresAt.Format(time.RFC3339) != expiresAt {
		t.Fatalf("expected expires_at %s, got %#v", expiresAt, got.ExpiresAt)
	}
}

func TestParseCreateTokenExpirationSupportsRFC3339(t *testing.T) {
	now := time.Date(2026, time.May, 27, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		expiresAt string
		want      time.Time
	}{
		{
			name:      "utc",
			expiresAt: "2026-06-24T12:00:00Z",
			want:      time.Date(2026, time.June, 24, 12, 0, 0, 0, time.UTC),
		},
		{
			name:      "offset",
			expiresAt: "2026-06-24T14:00:00+02:00",
			want:      time.Date(2026, time.June, 24, 12, 0, 0, 0, time.UTC),
		},
		{
			name:      "fractional seconds",
			expiresAt: "2026-06-24T12:00:00.123Z",
			want:      time.Date(2026, time.June, 24, 12, 0, 0, 123_000_000, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCreateTokenExpirationAt("", tt.expiresAt, now)
			if err != nil {
				t.Fatalf("parseCreateTokenExpiration returned error: %v", err)
			}
			if got.ExpiresAt == nil || !got.ExpiresAt.Equal(tt.want) {
				t.Fatalf("expected expires_at %s, got %#v", tt.want.Format(time.RFC3339Nano), got.ExpiresAt)
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
		"use a valid duration with a unit suffix",
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

func TestValidateCreatePATFlagsBundlesMissingRequired(t *testing.T) {
	err := validateCreatePATFlags(&patOptions{})
	var cfgErr *cmdpkg.ConfigurationError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigurationError, got %T: %v", err, err)
	}
	for _, want := range []string{
		"--name is required",
		"exactly one of --expires-in or --expires-at is required",
	} {
		if !strings.Contains(cfgErr.Error(), want) {
			t.Fatalf("expected error to contain %q, got %q", want, cfgErr.Error())
		}
	}
	if lines := strings.Count(cfgErr.Error(), "\n") + 1; lines != 2 {
		t.Fatalf("expected 2 bundled errors, got %d: %q", lines, cfgErr.Error())
	}
}

func TestValidateCreatePATFlagsReportsOnlyMissing(t *testing.T) {
	err := validateCreatePATFlags(&patOptions{name: "ci"})
	var cfgErr *cmdpkg.ConfigurationError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigurationError, got %T: %v", err, err)
	}
	if strings.Contains(cfgErr.Error(), "--name is required") {
		t.Fatalf("did not expect name error, got %q", cfgErr.Error())
	}
	if !strings.Contains(cfgErr.Error(), "exactly one of --expires-in or --expires-at is required") {
		t.Fatalf("expected expiry error, got %q", cfgErr.Error())
	}

	if err := validateCreatePATFlags(&patOptions{name: "ci", expiresIn: "30d"}); err != nil {
		t.Fatalf("expected no error when required flags are set, got %v", err)
	}
}

func TestValidateCreateSPATFlagsBundlesAllMissing(t *testing.T) {
	err := validateCreateSPATFlags(&spatOptions{})
	var cfgErr *cmdpkg.ConfigurationError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected ConfigurationError, got %T: %v", err, err)
	}
	for _, want := range []string{
		"--name is required",
		"exactly one of --system-account-id or --system-account-name is required",
		"exactly one of --expires-in or --expires-at is required",
	} {
		if !strings.Contains(cfgErr.Error(), want) {
			t.Fatalf("expected error to contain %q, got %q", want, cfgErr.Error())
		}
	}
}
