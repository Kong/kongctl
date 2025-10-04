package apiutil

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Doer abstracts the ability to execute HTTP requests.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
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

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	req.Header.Set("Accept", "application/json")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
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
