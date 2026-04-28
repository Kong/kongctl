package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ajg/form"
	"github.com/google/uuid"

	kk "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkMetadata "github.com/Kong/sdk-konnect-go/pkg/metadata"

	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/httpheaders"
)

var (
	DAGGrantType                  = "urn:ietf:params:oauth:grant-type:device_code"
	AuthorizationPendingErrorCode = "authorization_pending"
)

func getCredentialFileName(profile string) string {
	return fmt.Sprintf(".%s-konnect-token.json", profile)
}

// jwtExpiresIn parses the exp claim from a JWT access token and returns the
// number of seconds until it expires. Returns an error if the token is not a
// valid JWT or does not contain an exp claim. The JWT exp claim is the
// authoritative expiry enforced by the API, and takes precedence over the
// expires_in field returned by the token endpoint.
func jwtExpiresIn(tokenString string) (int, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return 0, fmt.Errorf("not a JWT: expected 3 parts, got %d", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	if claims.Exp == 0 {
		return 0, fmt.Errorf("JWT does not contain an exp claim")
	}

	secs := max(int(time.Until(time.Unix(claims.Exp, 0)).Seconds()), 0)
	return secs, nil
}

type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval,omitempty"`
}

type DAGError struct {
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorURI         string `json:"error_uri,omitempty"`
	ErrorCode        string `json:"error"`
}

func (d *DAGError) Error() string {
	return d.ErrorCode
}

type AccessTokenResponse struct {
	AuthToken    string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresAfter int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

type AccessToken struct {
	Token      *AccessTokenResponse `json:"token"`
	ReceivedAt time.Time            `json:"received_at"`
}

func (t *AccessToken) IsExpired() bool {
	return time.Now().After(t.ReceivedAt.Add(time.Duration(t.Token.ExpiresAfter) * time.Second))
}

// trustedKonnectDomains is the allow list of Kong-owned domains accepted as
// Konnect API endpoints. konghq.tech is used for staging/test environments.
var trustedKonnectDomains = []string{"konghq.com", "konghq.tech"}

// ValidateKonnectURL parses rawURL and ensures it uses HTTPS and targets a
// trusted Kong-owned domains.
func ValidateKonnectURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid Konnect URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("konnect URL must use HTTPS, got %q", u.Scheme)
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	for _, domain := range trustedKonnectDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return nil
		}
	}
	return fmt.Errorf("konnect URL host %q is not a trusted konghq.com domain", host)
}

func RequestDeviceCode(httpClient *http.Client,
	url string, clientID string, logger *slog.Logger,
) (DeviceCodeResponse, error) {
	if err := ValidateKonnectURL(url); err != nil {
		return DeviceCodeResponse{}, err
	}
	logger.Info("Requesting device code", "url", url, "client_id", clientID)
	requestBody := struct {
		ClientID uuid.UUID `form:"client_id"`
	}{
		ClientID: uuid.MustParse(clientID),
	}

	urlValues, err := form.EncodeToValues(requestBody)
	if err != nil {
		return DeviceCodeResponse{}, err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		url,
		strings.NewReader(urlValues.Encode()),
	)
	if err != nil {
		return DeviceCodeResponse{}, err
	}
	httpheaders.SetUserAgent(req, meta.UserAgent())

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Error("Device code request failed", "error", err)
		return DeviceCodeResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return DeviceCodeResponse{}, fmt.Errorf(
			"device code request to %s failed with HTTP %d",
			url, resp.StatusCode,
		)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return DeviceCodeResponse{}, err
	}

	var deviceCodeResponse DeviceCodeResponse
	err = json.Unmarshal(responseBody, &deviceCodeResponse)
	if err != nil {
		return DeviceCodeResponse{}, err
	}

	logger.Info("Device code request successful", "expires_in", deviceCodeResponse.ExpiresIn)
	return deviceCodeResponse, nil
}

func RefreshAccessToken(
	refreshURL string,
	refreshToken string,
	timeout time.Duration,
	transportOptions httpclient.TransportOptions,
	logger *slog.Logger,
) (*AccessToken, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	if err := ValidateKonnectURL(refreshURL); err != nil {
		return nil, err
	}

	cookieURL, err := url.Parse(refreshURL)
	if err != nil {
		return nil, err
	}

	// set the state as a cookie
	jar.SetCookies(cookieURL, []*http.Cookie{
		{
			Name:  "konnectrefreshtoken",
			Value: refreshToken,
		},
	})

	httpClient := httpclient.NewHTTPClientWithConfig(httpclient.ClientConfig{
		Timeout:          timeout,
		Jar:              jar,
		TransportOptions: transportOptions,
	})

	req, err := http.NewRequest(http.MethodPost, refreshURL, nil)
	if err != nil {
		return nil, err
	}
	httpheaders.SetContentTypeJSON(req)
	httpheaders.SetUserAgent(req, meta.UserAgent())

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to refresh token: %s", res.Status)
	}

	rv := AccessToken{
		Token: &AccessTokenResponse{
			TokenType:    "Bearer",
			ExpiresAfter: 3600,
			Scope:        "",
		},
		ReceivedAt: time.Now(),
	}

	refreshPath := cookieURL.Path
	for _, cookie := range res.Cookies() {
		if cookie.Name == "konnectrefreshtoken" && cookie.Path == refreshPath && cookie.Value != "" {
			rv.Token.RefreshToken = cookie.Value
		} else if cookie.Name == "konnectaccesstoken" && cookie.Value != "" {
			rv.Token.AuthToken = cookie.Value
		}
	}

	if secs, err := jwtExpiresIn(rv.Token.AuthToken); err != nil {
		logger.Info("Token refreshed, could not parse JWT exp claim, using default expires_in",
			"expires_after", rv.Token.ExpiresAfter, "error", err)
	} else {
		rv.Token.ExpiresAfter = secs
		logger.Info("Token refreshed, expiry derived from JWT exp claim",
			"expires_after", secs)
	}

	return &rv, nil
}

func PollForToken(ctx context.Context, httpClient *http.Client,
	url string, clientID string, deviceCode string, logger *slog.Logger,
) (*AccessToken, error) {
	if err := ValidateKonnectURL(url); err != nil {
		return nil, err
	}
	logger.Info("Polling for token", "url", url, "client_id", clientID, "device_code", deviceCode)
	requestBody := struct {
		GrantType  string    `form:"grant_type"`
		DeviceCode string    `form:"device_code"`
		ClientID   uuid.UUID `form:"client_id"`
	}{
		GrantType:  DAGGrantType,
		DeviceCode: deviceCode,
		ClientID:   uuid.MustParse(clientID),
	}

	urlsValues, err := form.EncodeToValues(requestBody)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx,
		http.MethodPost,
		url,
		strings.NewReader(urlsValues.Encode()),
	)
	if err != nil {
		return nil, err
	}
	httpheaders.SetUserAgent(request, meta.UserAgent())

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var dagError DAGError
	if response.StatusCode != http.StatusOK {
		err = json.Unmarshal(responseBody, &dagError)
		if err != nil {
			return nil, err
		}
		return nil, &dagError
	}

	var pollForTokenResponse AccessTokenResponse
	err = json.Unmarshal(responseBody, &pollForTokenResponse)
	if err != nil {
		return nil, err
	}

	rv := AccessToken{
		Token:      &pollForTokenResponse,
		ReceivedAt: time.Now(),
	}

	if secs, err := jwtExpiresIn(pollForTokenResponse.AuthToken); err != nil {
		logger.Info("Token received, could not parse JWT exp claim, using expires_in from response",
			"expires_after", pollForTokenResponse.ExpiresAfter, "error", err)
	} else {
		rv.Token.ExpiresAfter = secs
		logger.Info("Token received, expiry derived from JWT exp claim",
			"expires_after", secs)
	}

	return &rv, nil
}

// For a given profile, load a saved token from disk in the same path as the config path.
// * If there is no file, return error.
// * If it's not expired, return it.
// * If it's expired, refresh it, then store it, then return it
func LoadAccessToken(
	cfg config.Hook,
	refreshURL string,
	timeout time.Duration,
	transportOptions httpclient.TransportOptions,
	logger *slog.Logger,
) (*AccessToken, error) {
	profile := cfg.GetProfile()
	cfgPath := filepath.Dir(cfg.GetPath())
	credsPath := filepath.Join(cfgPath, getCredentialFileName(profile))

	creds, err := loadAccessTokenFromDisk(credsPath)
	if err != nil {
		return nil, err
	}

	if creds.IsExpired() {
		logger.Info("Token expired, refreshing", "refresh_url", refreshURL)
		creds, err = RefreshAccessToken(refreshURL, creds.Token.RefreshToken, timeout, transportOptions, logger)
		if err != nil {
			return nil, err
		}
		logger.Info("Token refreshed. Saving to disk",
			"received_at", creds.ReceivedAt,
			"expires_after", creds.Token.ExpiresAfter,
			"creds_path", credsPath)
		err = saveAccessTokenToDisk(credsPath, creds)
		if err != nil {
			return nil, err
		}
	} else {
		logger.Info("Token loaded from disk",
			"expires_after", creds.Token.ExpiresAfter,
			"received_at", creds.ReceivedAt,
			"creds_path", credsPath)
	}
	return creds, nil
}

func loadAccessTokenFromDisk(path string) (*AccessToken, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var token AccessToken
	err = json.Unmarshal(data, &token)
	if err != nil {
		return nil, err
	}

	return &token, nil
}

func SaveAccessToken(cfg config.Hook, token *AccessToken) error {
	profile := cfg.GetProfile()
	cfgPath := filepath.Dir(cfg.GetPath())
	credsPath := filepath.Join(cfgPath, getCredentialFileName(profile))

	return saveAccessTokenToDisk(credsPath, token)
}

func DeleteAccessToken(cfg config.Hook) (bool, error) {
	profile := cfg.GetProfile()
	cfgPath := filepath.Dir(cfg.GetPath())
	credsPath := filepath.Join(cfgPath, getCredentialFileName(profile))

	err := os.Remove(credsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func saveAccessTokenToDisk(path string, token *AccessToken) error {
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}

	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		return err
	}

	return nil
}

func GetAuthenticatedClient(
	baseURL string,
	token string,
	timeout time.Duration,
	transportOptions httpclient.TransportOptions,
	logger *slog.Logger,
) (*kk.SDK, kk.HTTPClient, error) {
	if err := ValidateKonnectURL(baseURL); err != nil {
		return nil, nil, err
	}
	kkMetadata.SetUserAgent(meta.UserAgent())

	opts := []kk.SDKOption{
		kk.WithServerURL(baseURL),
		kk.WithSecurity(kkComps.Security{
			PersonalAccessToken: new(token),
		}),
	}

	loggingClient := httpclient.NewLoggingHTTPClientWithClient(
		httpclient.NewHTTPClientWithConfig(httpclient.ClientConfig{
			Timeout:          timeout,
			TransportOptions: transportOptions,
		}),
		logger,
	)
	opts = append(opts, kk.WithClient(loggingClient))

	return kk.New(opts...), loggingClient, nil
}
