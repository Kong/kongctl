package apiutil

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/meta"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestResolveEndpoint(t *testing.T) {
	endpoint, err := resolveEndpoint("https://example.com/api", "/v1/resources")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if endpoint != "https://example.com/api/v1/resources" {
		t.Fatalf("unexpected endpoint: %s", endpoint)
	}

	endpoint, err = resolveEndpoint("https://example.com/api/", "v1/resources")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if endpoint != "https://example.com/api/v1/resources" {
		t.Fatalf("unexpected endpoint: %s", endpoint)
	}

	endpoint, err = resolveEndpoint("https://example.com/api", "https://another.test/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if endpoint != "https://another.test/path" {
		t.Fatalf("unexpected endpoint: %s", endpoint)
	}
}

func TestRequestSuccess(t *testing.T) {
	var receivedURL string
	client := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		receivedURL = req.URL.String()
		if req.Header.Get("Authorization") != "Bearer tok" {
			t.Fatalf("unexpected authorization header: %s", req.Header.Get("Authorization"))
		}
		if req.Body != nil {
			reqBody, _ := io.ReadAll(req.Body)
			if len(reqBody) != 0 {
				t.Fatalf("expected empty body, got %q", string(reqBody))
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"hello":"world"}`)),
			Header:     make(http.Header),
		}, nil
	})

	res, err := Request(context.Background(), client, http.MethodGet, "https://example.com", "/v1/foo", "tok", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedURL != "https://example.com/v1/foo" {
		t.Fatalf("unexpected request URL: %s", receivedURL)
	}
	if string(res.Body) != `{"hello":"world"}` {
		t.Fatalf("unexpected body: %s", string(res.Body))
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", res.StatusCode)
	}
}

func TestRequestError(t *testing.T) {
	client := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("boom")
	})

	_, err := Request(context.Background(), client, http.MethodGet, "https://example.com", "/foo", "tok", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error containing 'boom', got %v", err)
	}
}

func TestRequestNilResponse(t *testing.T) {
	client := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, nil
	})

	_, err := Request(context.Background(), client, http.MethodGet, "https://example.com", "/foo", "tok", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "request returned nil response") {
		t.Fatalf("expected nil response error, got %v", err)
	}
}

func TestRequestNilBody(t *testing.T) {
	client := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Header:     make(http.Header),
		}, nil
	})

	result, err := Request(
		context.Background(),
		client,
		http.MethodDelete,
		"https://example.com",
		"/foo",
		"tok",
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StatusCode != http.StatusNoContent {
		t.Fatalf("StatusCode = %d, want %d", result.StatusCode, http.StatusNoContent)
	}
	if len(result.Body) != 0 {
		t.Fatalf("Body = %q, want empty", string(result.Body))
	}
}

func TestRequestSetsUserAgentHeader(t *testing.T) {
	original := meta.CLIVersion()
	t.Cleanup(func() {
		meta.SetCLIVersion(original)
	})
	meta.SetCLIVersion("0.5.0")

	var gotUserAgent string
	client := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotUserAgent = req.Header.Get("User-Agent")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
		}, nil
	})

	_, err := Request(context.Background(), client, http.MethodGet, "https://example.com", "/v1/test", "", nil, nil)
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}

	if gotUserAgent != "kongctl/v0.5.0" {
		t.Fatalf("User-Agent = %q, want %q", gotUserAgent, "kongctl/v0.5.0")
	}
}

func TestRequestWithTokenSourceRefreshesAndRetries401(t *testing.T) {
	source := &testTokenSource{
		token:          "old-token",
		refreshedToken: "new-token",
	}

	var attempts int
	client := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if string(body) != `{"ok":true}` {
			t.Fatalf("unexpected body: %q", string(body))
		}

		switch attempts {
		case 1:
			if req.Header.Get("Authorization") != "Bearer old-token" {
				t.Fatalf("first authorization = %q", req.Header.Get("Authorization"))
			}
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":"expired"}`)),
			}, nil
		case 2:
			if req.Header.Get("Authorization") != "Bearer new-token" {
				t.Fatalf("retry authorization = %q", req.Header.Get("Authorization"))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"done":true}`)),
			}, nil
		default:
			t.Fatalf("unexpected attempt %d", attempts)
			return nil, nil
		}
	})

	result, err := RequestWithTokenSource(
		context.Background(),
		client,
		http.MethodPost,
		"https://example.com",
		"/v1/test",
		source,
		map[string]string{"Content-Type": "application/json"},
		bytes.NewReader([]byte(`{"ok":true}`)),
	)
	if err != nil {
		t.Fatalf("RequestWithTokenSource() error = %v", err)
	}
	if result.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", result.StatusCode, http.StatusOK)
	}
	if string(result.Body) != `{"done":true}` {
		t.Fatalf("Body = %q", string(result.Body))
	}
	if source.refreshCalls != 1 {
		t.Fatalf("refreshCalls = %d, want 1", source.refreshCalls)
	}
}

func TestRequestWithTokenSourceNilRetryResponse(t *testing.T) {
	source := &testTokenSource{
		token:          "old-token",
		refreshedToken: "new-token",
	}

	attempts := 0
	client := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":"expired"}`)),
			}, nil
		}
		return nil, nil
	})

	_, err := RequestWithTokenSource(
		context.Background(),
		client,
		http.MethodGet,
		"https://example.com",
		"/v1/test",
		source,
		nil,
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "retry request after Konnect token refresh returned nil response") {
		t.Fatalf("expected nil retry response error, got %v", err)
	}
}

type testTokenSource struct {
	token          string
	refreshedToken string
	refreshCalls   int
}

func (s *testTokenSource) Token(context.Context) (string, error) {
	return s.token, nil
}

func (s *testTokenSource) Refresh(_ context.Context, previousToken string) (string, error) {
	if previousToken != s.token {
		return "", errors.New("unexpected previous token")
	}
	s.refreshCalls++
	return s.refreshedToken, nil
}
