package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
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

func (s stubConfig) Save() error                           { return nil }
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
