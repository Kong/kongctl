package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	kk "github.com/Kong/sdk-konnect-go"

	"github.com/kong/kongctl/internal/util/httpheaders"
)

// RefreshingHTTPClient retries one replayable request after a recoverable 401.
type RefreshingHTTPClient struct {
	base        kk.HTTPClient
	tokenSource *TokenSource
}

func NewRefreshingHTTPClient(base kk.HTTPClient, tokenSource *TokenSource) *RefreshingHTTPClient {
	return &RefreshingHTTPClient{
		base:        base,
		tokenSource: tokenSource,
	}
}

func (c *RefreshingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if c == nil || c.base == nil {
		return nil, fmt.Errorf("http client is not configured")
	}
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	resp, err := c.base.Do(req)
	if err != nil || resp == nil || resp.StatusCode != http.StatusUnauthorized || c.tokenSource == nil {
		return resp, err
	}

	if !requestCanBeReplayed(req) {
		return resp, nil
	}

	previousToken := bearerToken(req.Header.Get(httpheaders.HeaderAuthorization))
	refreshedToken, refreshErr := c.tokenSource.Refresh(req.Context(), previousToken)
	if refreshErr != nil {
		if errors.Is(refreshErr, ErrTokenRefreshUnsupported) {
			return resp, nil
		}
		resp.Body.Close()
		return nil, fmt.Errorf("refresh Konnect access token after HTTP 401: %w", refreshErr)
	}

	resp.Body.Close()

	retryReq := req.Clone(req.Context())
	if req.Body != nil && req.Body != http.NoBody {
		body, err := req.GetBody()
		if err != nil {
			return nil, fmt.Errorf("replay request after Konnect token refresh: %w", err)
		}
		retryReq.Body = body
	}
	httpheaders.SetBearerAuthorization(retryReq, refreshedToken)

	return c.base.Do(retryReq)
}

func requestCanBeReplayed(req *http.Request) bool {
	return req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	prefix := httpheaders.BearerAuthorizationPrefix
	if len(header) >= len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
		return strings.TrimSpace(header[len(prefix):])
	}
	return ""
}
