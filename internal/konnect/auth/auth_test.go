package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

// buildTestJWT creates a minimal JWT with the given exp claim for testing.
// The signature segment is a placeholder — jwtExpiresIn only decodes the payload.
func buildTestJWT(exp int64) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload, _ := json.Marshal(map[string]int64{"exp": exp, "iat": exp - 900})
	payloadEnc := base64.RawURLEncoding.EncodeToString(payload)
	return strings.Join([]string{header, payloadEnc, "signature"}, ".")
}

type stubConfig struct {
	profile string
	path    string
}

func (s stubConfig) GetString(string) string               { return "" }
func (s stubConfig) GetBool(string) bool                   { return false }
func (s stubConfig) GetInt(string) int                     { return 0 }
func (s stubConfig) GetIntOrElse(_ string, orElse int) int { return orElse }
func (s stubConfig) GetStringSlice(string) []string        { return nil }
func (s stubConfig) SetString(string, string)              {}
func (s stubConfig) Set(string, any)                       {}
func (s stubConfig) Get(string) any                        { return nil }
func (s stubConfig) BindFlag(string, *pflag.Flag) error    { return nil }
func (s stubConfig) GetProfile() string                    { return s.profile }
func (s stubConfig) GetPath() string                       { return s.path }

func TestDeleteAccessTokenRemovesFile(t *testing.T) {
	dir := t.TempDir()

	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte{}, 0o600))

	profile := "default"
	tokenPath := filepath.Join(dir, fmt.Sprintf(".%s-konnect-token.json", profile))
	require.NoError(t, os.WriteFile(tokenPath, []byte(`{"token":"value"}`), 0o600))

	cfg := stubConfig{
		profile: profile,
		path:    configPath,
	}

	removed, err := DeleteAccessToken(cfg)
	require.NoError(t, err)
	require.True(t, removed)

	_, err = os.Stat(tokenPath)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestDeleteAccessTokenNoFile(t *testing.T) {
	dir := t.TempDir()

	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte{}, 0o600))

	cfg := stubConfig{
		profile: "default",
		path:    configPath,
	}

	removed, err := DeleteAccessToken(cfg)
	require.NoError(t, err)
	require.False(t, removed)
}

func TestJWTExpiresIn_FutureExpiry(t *testing.T) {
	// Token that expires 900 seconds from now
	exp := time.Now().Add(900 * time.Second).Unix()
	token := buildTestJWT(exp)

	secs, err := jwtExpiresIn(token)
	require.NoError(t, err)
	// Allow a small window for test execution time
	require.InDelta(t, 900, secs, 5)
}

func TestJWTExpiresIn_AlreadyExpired(t *testing.T) {
	// Token whose exp is in the past
	exp := time.Now().Add(-60 * time.Second).Unix()
	token := buildTestJWT(exp)

	secs, err := jwtExpiresIn(token)
	require.NoError(t, err)
	require.Equal(t, 0, secs) // clamped to zero, not negative
}

func TestJWTExpiresIn_NotAJWT(t *testing.T) {
	_, err := jwtExpiresIn("not-a-jwt")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a JWT")
}

func TestJWTExpiresIn_InvalidBase64Payload(t *testing.T) {
	_, err := jwtExpiresIn("header.!!!invalid!!!.signature")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to decode JWT payload")
}

func TestJWTExpiresIn_MissingExpClaim(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"user123"}`))
	token := strings.Join([]string{header, payload, "sig"}, ".")

	_, err := jwtExpiresIn(token)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exp claim")
}

func TestJWTExpiresIn_APIResponseMismatch(t *testing.T) {
	// Simulates the Konnect bug: API returns expires_in=3600 but JWT exp is 900s
	// Verify jwtExpiresIn returns the JWT value, not the API-provided one
	apiExpiresIn := 3600
	jwtLifetime := 900 * time.Second

	exp := time.Now().Add(jwtLifetime).Unix()
	token := buildTestJWT(exp)

	secs, err := jwtExpiresIn(token)
	require.NoError(t, err)
	require.NotEqual(t, apiExpiresIn, secs, "jwtExpiresIn should return JWT exp, not API expires_in")
	require.InDelta(t, int(jwtLifetime.Seconds()), secs, 5)
}

func TestValidateKonnectURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		// valid URLs
		{name: "production base URL", url: "https://us.api.konghq.com"},
		{name: "global API URL", url: "https://global.api.konghq.com"},
		{name: "refresh endpoint", url: "https://global.api.konghq.com/kauth/api/v1/refresh"},
		{name: "apex domain", url: "https://konghq.com"},
		{name: "staging base URL", url: "https://api.konghq.tech"},
		{name: "staging subdomain", url: "https://us.api.konghq.tech"},
		{name: "uppercase host normalized", url: "https://US.API.KONGHQ.COM"},
		{name: "trailing dot normalized", url: "https://us.api.konghq.com."},

		// scheme errors
		{name: "http not allowed", url: "http://us.api.konghq.com",
			wantErr: "must use HTTPS"},
		{name: "empty scheme", url: "//us.api.konghq.com",
			wantErr: "must use HTTPS"},

		// untrusted hosts
		{name: "arbitrary host", url: "https://evil.com",
			wantErr: "not a trusted konghq.com domain"},
		{name: "subdomain typosquat", url: "https://us.api.konghq.com.evil.com",
			wantErr: "not a trusted konghq.com domain"},
		{name: "konghq.com prefix only", url: "https://konghq.com.evil.com",
			wantErr: "not a trusted konghq.com domain"},
		{name: "localhost", url: "https://localhost",
			wantErr: "not a trusted konghq.com domain"},
		{name: "link-local", url: "https://169.254.169.254",
			wantErr: "not a trusted konghq.com domain"},

		// parse errors
		{name: "invalid URL", url: "://bad-url",
			wantErr: "invalid Konnect URL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKonnectURL(tt.url)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}
func TestRequestDeviceCode_NonOKStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "404 HTML page",
			statusCode: http.StatusNotFound,
			body:       "<html><body>Not Found</body></html>",
		},
		{
			name:       "500 server error",
			statusCode: http.StatusInternalServerError,
			body:       "Internal Server Error",
		},
		{
			name:       "403 forbidden",
			statusCode: http.StatusForbidden,
			body:       `{"error":"forbidden"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{
				Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: tt.statusCode,
						Body:       io.NopCloser(strings.NewReader(tt.body)),
					}, nil
				}),
			}

			_, err := RequestDeviceCode(
				client,
				"https://us.konghq.com/some/endpoint",
				"344f59db-f401-4ce7-9407-00a0823fbacf",
				slog.New(slog.NewTextHandler(io.Discard, nil)),
			)

			require.Error(t, err)
			require.NotContains(t, err.Error(), "invalid character",
				"error should not be a JSON parse failure")
			require.Contains(t, err.Error(), fmt.Sprintf("HTTP %d", tt.statusCode))
		})
	}
}

// roundTripFunc allows an inline function to be used as an http.RoundTripper,
// intercepting HTTP requests without making real network calls.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
