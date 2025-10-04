package apiutil

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
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
