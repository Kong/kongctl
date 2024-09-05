package auth

import (
	"context"
	"encoding/json"
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

	kk "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/ajg/form"
	"github.com/google/uuid"
	"github.com/kong/kongctl/internal/config"
)

var (
	DAGGrantType                  = "urn:ietf:params:oauth:grant-type:device_code"
	AuthorizationPendingErrorCode = "authorization_pending"
)

func getCredentialFileName(profile string) string {
	return fmt.Sprintf(".%s-konnect-token.json", profile)
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

func RequestDeviceCode(httpClient *http.Client,
	url string, clientID string, logger *slog.Logger,
) (DeviceCodeResponse, error) {
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

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Error("Device code request failed", "error", err)
		return DeviceCodeResponse{}, err
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

func RefreshAccessToken(refreshURL string, refreshToken string, _ *slog.Logger) (*AccessToken, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
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

	httpClient := &http.Client{
		Jar: jar,
	}

	res, err := httpClient.Post(refreshURL, "application/json", nil)
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
			rv.Token.ExpiresAfter = int(time.Until(cookie.Expires).Seconds())
		}
	}

	return &rv, nil
}

func PollForToken(ctx context.Context, httpClient *http.Client,
	url string, clientID string, deviceCode string, logger *slog.Logger,
) (*AccessToken, error) {
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

	logger.Info("Token received", "expires_after", pollForTokenResponse.ExpiresAfter)
	return &rv, nil
}

// For a given profile, load a saved token from disk in the same path as the config path.
// * If there is no file, return error.
// * If it's not expired, return it.
// * If it's expired, refresh it, then store it, then return it
func LoadAccessToken(cfg config.Hook, refreshURL string, logger *slog.Logger) (*AccessToken, error) {
	profile := cfg.GetProfile()
	cfgPath := filepath.Dir(cfg.GetPath())
	credsPath := filepath.Join(cfgPath, getCredentialFileName(profile))

	creds, err := loadAccessTokenFromDisk(credsPath)
	if err != nil {
		return nil, err
	}

	if creds.IsExpired() {
		logger.Info("Token expired, refreshing", "refresh_url", refreshURL)
		creds, err = RefreshAccessToken(refreshURL, creds.Token.RefreshToken, logger)
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

func GetAuthenticatedClient(baseURL string, token string) (*kk.SDK, error) {
	return kk.New(
		kk.WithServerURL(baseURL),
		kk.WithSecurity(kkComps.Security{
			PersonalAccessToken: kk.String(token),
		}),
	), nil
}
