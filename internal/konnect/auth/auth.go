package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	kk "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/ajg/form"
	"github.com/google/uuid"
	"github.com/kong/kong-cli/internal/config"
	"github.com/kong/kong-cli/internal/meta"
)

var (
	DAGGrantType                  = "urn:ietf:params:oauth:grant-type:device_code"
	AuthorizationPendingErrorCode = "authorization_pending"

	defaultCredPath = "$XDG_CONFIG_HOME/" + meta.CLIName
)

func ExpandDefaultCredPath() string {
	return os.ExpandEnv(defaultCredPath)
}

func BuildDefaultCredentialFilePath(profile string) string {
	return fmt.Sprintf("%s/.%s-konnect-token.json", config.ExpandDefaultConfigPath(), profile)
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
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

type AccessToken struct {
	Token      *AccessTokenResponse `json:"token"`
	ReceivedAt time.Time            `json:"received_at"`
}

func (t *AccessToken) IsExpired() bool {
	return time.Now().After(t.ReceivedAt.Add(time.Duration(t.Token.ExpiresIn) * time.Second))
}

func RequestDeviceCode(httpClient *http.Client,
	url string, clientID string,
) (DeviceCodeResponse, error) {
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

	return deviceCodeResponse, nil
}

func RefreshAccessToken(refreshURL string, refreshToken string) (*AccessToken, error) {
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
			TokenType: "Bearer",
			ExpiresIn: 3600,
			Scope:     "",
		},
		ReceivedAt: time.Now(),
	}

	refreshPath := cookieURL.Path
	for _, cookie := range res.Cookies() {
		if cookie.Name == "konnectrefreshtoken" && cookie.Path == refreshPath && cookie.Value != "" {
			rv.Token.RefreshToken = cookie.Value
		} else if cookie.Name == "konnectaccesstoken" && cookie.Value != "" {
			rv.Token.AuthToken = cookie.Value
			rv.Token.ExpiresIn = int(time.Until(cookie.Expires).Seconds())
		}
	}

	return &rv, nil
}

func PollForToken(httpClient *http.Client, url string, clientID string, deviceCode string) (*AccessToken, error) {
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

	request, err := http.NewRequest(
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
	return &rv, nil
}

// For a given profile, load a saved token from disk.
// * If there is no file, return error.
// * If it's not expired, return it.
// * If it's expired, refresh it, then store it, then return it
func LoadAccessToken(profile, refreshURL string) (*AccessToken, error) {
	credsPath := BuildDefaultCredentialFilePath(profile)
	creds, err := LoadAccessTokenFromDisk(credsPath)
	if err != nil {
		return nil, err
	}
	if creds.IsExpired() {
		creds, err = RefreshAccessToken(refreshURL, creds.Token.RefreshToken)
		if err != nil {
			return nil, err
		}
		err = SaveAccessTokenToDisk(credsPath, creds)
		if err != nil {
			return nil, err
		}
	}
	return creds, nil
}

func LoadAccessTokenFromDisk(path string) (*AccessToken, error) {
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

func SaveAccessTokenToDisk(path string, token *AccessToken) error {
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

func GetAuthenticatedClient(token *AccessToken) (*kk.SDK, error) {
	return kk.New(
		kk.WithSecurity(kkComps.Security{
			PersonalAccessToken: kk.String(token.Token.AuthToken),
		}),
	), nil
}
