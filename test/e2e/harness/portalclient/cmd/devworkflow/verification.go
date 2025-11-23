package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/quotedprintable"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kong/kongctl/test/e2e/harness/portalclient"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

const (
	gmailTokenEndpoint = "https://oauth2.googleapis.com/token"
	gmailMessagesURL   = "https://gmail.googleapis.com/gmail/v1/users/me/messages"
	defaultSubject     = "Please confirm your email address"
)

var (
	errMessageNotFound    = errors.New("gmail message not found yet")
	uuidPattern           = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	verificationTokenExpr = regexp.MustCompile(`token=([A-Za-z0-9\-_%]+)`)
)

type gmailClient struct {
	httpClient   *http.Client
	clientID     string
	clientSecret string
	refreshToken string

	cachedAccessToken string
	tokenExpiry       time.Time
}

func newGmailClientFromEnv() (*gmailClient, error) {
	clientID := strings.TrimSpace(os.Getenv("KONGCTL_E2E_GMAIL_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("KONGCTL_E2E_GMAIL_CLIENT_SECRET"))
	refreshToken := strings.TrimSpace(os.Getenv("KONGCTL_E2E_GMAIL_REFRESH_TOKEN"))

	switch {
	case clientID == "":
		return nil, fmt.Errorf("KONGCTL_E2E_GMAIL_CLIENT_ID not set")
	case clientSecret == "":
		return nil, fmt.Errorf("KONGCTL_E2E_GMAIL_CLIENT_SECRET not set")
	case refreshToken == "":
		return nil, fmt.Errorf("KONGCTL_E2E_GMAIL_REFRESH_TOKEN not set")
	}

	return &gmailClient{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		clientID:     clientID,
		clientSecret: clientSecret,
		refreshToken: refreshToken,
	}, nil
}

func (g *gmailClient) getAccessToken(ctx context.Context) (string, error) {
	if g.cachedAccessToken != "" && time.Until(g.tokenExpiry) > 15*time.Second {
		return g.cachedAccessToken, nil
	}

	form := url.Values{
		"client_id":     {g.clientID},
		"client_secret": {g.clientSecret},
		"refresh_token": {g.refreshToken},
		"grant_type":    {"refresh_token"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gmailTokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return "", fmt.Errorf("gmail token exchange failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("token response missing access_token")
	}

	g.cachedAccessToken = tokenResp.AccessToken
	g.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return g.cachedAccessToken, nil
}

func (g *gmailClient) waitForVerificationToken(ctx context.Context, to string) (string, error) {
	start := time.Now().Add(-1 * time.Minute)
	deadline := time.Now().Add(2 * time.Minute)
	for {
		token, err := g.fetchVerificationToken(ctx, to, start)
		if err == nil {
			return token, nil
		}
		if !errors.Is(err, errMessageNotFound) {
			return "", err
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("verification email for %s did not arrive in time", to)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func (g *gmailClient) fetchVerificationToken(ctx context.Context, to string, after time.Time) (string, error) {
	accessToken, err := g.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("gmail access token: %w", err)
	}

	query := buildGmailQuery(to, after)
	listURL := fmt.Sprintf("%s?maxResults=5&q=%s", gmailMessagesURL, url.QueryEscape(query))
	log.Printf("gmail query %s", query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return "", fmt.Errorf("gmail list failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var listResp struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return "", fmt.Errorf("decode gmail list: %w", err)
	}
	if len(listResp.Messages) == 0 {
		return "", errMessageNotFound
	}

	msgID := listResp.Messages[0].ID
	getURL := fmt.Sprintf("%s/%s?format=raw", gmailMessagesURL, msgID)
	msgReq, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	if err != nil {
		return "", err
	}
	msgReq.Header.Set("Authorization", "Bearer "+accessToken)

	msgResp, err := g.httpClient.Do(msgReq)
	if err != nil {
		return "", err
	}
	defer msgResp.Body.Close()

	if msgResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(msgResp.Body, 8<<10))
		return "", fmt.Errorf("gmail message fetch failed: status=%d body=%s", msgResp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rawResp struct {
		Raw string `json:"raw"`
	}
	if err := json.NewDecoder(msgResp.Body).Decode(&rawResp); err != nil {
		return "", fmt.Errorf("decode gmail message: %w", err)
	}
	if rawResp.Raw == "" {
		return "", fmt.Errorf("gmail message missing raw payload")
	}

	bodyBytes, err := decodeGmailBody(rawResp.Raw)
	if err != nil {
		return "", fmt.Errorf("decode gmail payload: %w", err)
	}

	token, err := extractVerificationToken(string(bodyBytes))
	if err != nil {
		log.Printf("verification email body:\n%s", string(bodyBytes))
		return "", err
	}
	return token, nil
}

func extractVerificationToken(body string) (string, error) {
	body = decodeQuotedPrintable(body)
	if match := verificationTokenExpr.FindStringSubmatch(body); len(match) == 2 {
		value, err := url.QueryUnescape(match[1])
		if err == nil {
			value = trimEncodedPrefix(value)
			if uuidPattern.MatchString(value) {
				return value, nil
			}
		}
		candidate := trimEncodedPrefix(match[1])
		if uuidPattern.MatchString(candidate) {
			return candidate, nil
		}
	}

	if match := uuidPattern.FindString(body); match != "" {
		return match, nil
	}

	snippet := body
	if len(snippet) > 256 {
		snippet = snippet[:256] + "…"
	}
	return "", fmt.Errorf("verification token not found in email body snippet=%q", snippet)
}

func decodeGmailBody(raw string) ([]byte, error) {
	clean := strings.TrimSpace(raw)
	clean = strings.ReplaceAll(clean, "\n", "")
	switch {
	case len(clean)%4 == 0:
	case len(clean)%4 == 2:
		clean += "=="
	case len(clean)%4 == 3:
		clean += "="
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(clean); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.URLEncoding.DecodeString(clean); err == nil {
		return decoded, nil
	}
	return base64.StdEncoding.DecodeString(clean)
}

func buildGmailQuery(to string, after time.Time) string {
	subject := strings.TrimSpace(os.Getenv("KONGCTL_E2E_GMAIL_SUBJECT"))
	if subject == "" {
		subject = defaultSubject
	}
	return fmt.Sprintf(`to:%q after:%d subject:%q`, to, after.Unix(), subject)
}

func decodeQuotedPrintable(body string) string {
	reader := quotedprintable.NewReader(strings.NewReader(body))
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return body
	}
	return string(decoded)
}

func trimEncodedPrefix(value string) string {
	for strings.HasPrefix(value, "3D") {
		value = value[2:]
	}
	return value
}

func completeDeveloperVerification(ctx context.Context, api *portalclient.PortalAPI, gmail *gmailClient, email, password string) error {
	token, err := gmail.waitForVerificationToken(ctx, email)
	if err != nil {
		return err
	}
	log.Printf("gmail verification token=%s", token)

	resetToken, err := verifyDeveloperEmail(ctx, api, token)
	if err != nil {
		return err
	}
	return resetDeveloperPassword(ctx, api, resetToken, password)
}

func verifyDeveloperEmail(ctx context.Context, api *portalclient.PortalAPI, token string) (string, error) {
	parsed, err := uuid.Parse(token)
	if err != nil {
		return "", fmt.Errorf("invalid verification token: %w", err)
	}
	tokenUUID := openapi_types.UUID(parsed)
	resp, err := api.Raw().VerifyEmailWithResponse(ctx, portalclient.VerifyEmailRequest{
		Token: &tokenUUID,
	})
	if err != nil {
		return "", err
	}
	if resp.StatusCode() != http.StatusAccepted {
		body := ""
		if resp.Body != nil {
			body = strings.TrimSpace(string(resp.Body))
		}
		return "", fmt.Errorf("verify email unexpected status %d body=%s", resp.StatusCode(), body)
	}
	if resp.JSON202 == nil || resp.JSON202.Token == nil {
		return "", fmt.Errorf("verify email response missing token")
	}
	return resp.JSON202.Token.String(), nil
}

func resetDeveloperPassword(ctx context.Context, api *portalclient.PortalAPI, token, password string) error {
	parsed, err := uuid.Parse(token)
	if err != nil {
		return fmt.Errorf("invalid reset token: %w", err)
	}
	tokenUUID := openapi_types.UUID(parsed)
	payload := portalclient.ResetPasswordRequest{
		Token:    &tokenUUID,
		Password: &password,
	}
	resp, err := api.Raw().ResetPasswordWithResponse(ctx, payload)
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusNoContent {
		return fmt.Errorf("reset password unexpected status %d", resp.StatusCode())
	}
	return nil
}
