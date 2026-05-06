package apiutil

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/httpheaders"
)

// Doer abstracts the ability to execute HTTP requests.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

type TokenSource interface {
	Token(ctx context.Context) (string, error)
	Refresh(ctx context.Context, previousToken string) (string, error)
}

type refreshableTokenSource interface {
	Refreshable() bool
}

type StaticTokenSource struct {
	token string
}

func NewStaticTokenSource(token string) StaticTokenSource {
	return StaticTokenSource{token: token}
}

func (s StaticTokenSource) Token(context.Context) (string, error) {
	return s.token, nil
}

func (s StaticTokenSource) Refresh(context.Context, string) (string, error) {
	return "", fmt.Errorf("token refresh is not supported")
}

func (s StaticTokenSource) Refreshable() bool {
	return false
}

// Result represents a simplified HTTP response payload.
type Result struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

// Request issues an HTTP request against the provided endpoint. If the provided
// path is not absolute it is resolved against the base URL. Headers are merged
// with defaults where the caller provided values take precedence.
func Request(
	ctx context.Context,
	client Doer,
	method string,
	baseURL string,
	path string,
	token string,
	headers map[string]string,
	body io.Reader,
) (*Result, error) {
	return request(ctx, client, method, baseURL, path, nil, token, headers, body)
}

func RequestWithTokenSource(
	ctx context.Context,
	client Doer,
	method string,
	baseURL string,
	path string,
	tokenSource TokenSource,
	headers map[string]string,
	body io.Reader,
) (*Result, error) {
	return request(ctx, client, method, baseURL, path, tokenSource, "", headers, body)
}

func request(
	ctx context.Context,
	client Doer,
	method string,
	baseURL string,
	path string,
	tokenSource TokenSource,
	staticToken string,
	headers map[string]string,
	body io.Reader,
) (*Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if client == nil {
		client = http.DefaultClient
	}

	endpoint, err := resolveEndpoint(baseURL, path)
	if err != nil {
		return nil, err
	}

	token := staticToken
	if tokenSource != nil {
		token, err = tokenSource.Token(ctx)
		if err != nil {
			return nil, fmt.Errorf("resolve Konnect access token: %w", err)
		}
	}

	req, err := newRequest(ctx, method, endpoint, token, headers, body)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp != nil && resp.StatusCode == http.StatusUnauthorized && shouldRefreshAndRetry(tokenSource, req) {
		refreshedToken, refreshErr := tokenSource.Refresh(ctx, token)
		if refreshErr != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("refresh Konnect access token after HTTP 401: %w", refreshErr)
		}

		resp.Body.Close()

		retryBody, err := retryBody(req)
		if err != nil {
			return nil, err
		}
		req, err = newRequest(ctx, method, endpoint, refreshedToken, headers, retryBody)
		if err != nil {
			return nil, err
		}
		resp, err = client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("retry request after Konnect token refresh failed: %w", err)
		}
	}
	defer resp.Body.Close()

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &Result{
		StatusCode: resp.StatusCode,
		Body:       bytes,
		Header:     resp.Header.Clone(),
	}, nil
}

func newRequest(
	ctx context.Context,
	method string,
	endpoint string,
	token string,
	headers map[string]string,
	body io.Reader,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	if token != "" {
		httpheaders.SetBearerAuthorization(req, token)
	}
	httpheaders.SetAcceptJSON(req)
	httpheaders.SetUserAgent(req, meta.UserAgent())

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return req, nil
}

func shouldRefreshAndRetry(tokenSource TokenSource, req *http.Request) bool {
	if tokenSource == nil || !requestCanBeReplayed(req) {
		return false
	}
	if refreshable, ok := tokenSource.(refreshableTokenSource); ok && !refreshable.Refreshable() {
		return false
	}
	return true
}

func requestCanBeReplayed(req *http.Request) bool {
	return req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
}

func retryBody(req *http.Request) (io.Reader, error) {
	if req.Body == nil || req.Body == http.NoBody {
		return nil, nil
	}
	if req.GetBody == nil {
		return nil, fmt.Errorf("request body cannot be replayed after Konnect token refresh")
	}
	return req.GetBody()
}

func resolveEndpoint(baseURL, path string) (string, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return "", fmt.Errorf("endpoint path cannot be empty")
	}

	if strings.HasPrefix(trimmedPath, "http://") || strings.HasPrefix(trimmedPath, "https://") {
		return trimmedPath, nil
	}

	if baseURL == "" {
		return "", fmt.Errorf("base URL cannot be empty")
	}

	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(trimmedPath, "/"), nil
}
