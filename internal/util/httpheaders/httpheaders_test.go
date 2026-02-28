package httpheaders

import (
	"net/http"
	"testing"
)

func TestHeaderHelpers(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	SetUserAgent(req, "kongctl/0.5.0")
	SetBearerAuthorization(req, "token")
	SetAcceptJSON(req)
	SetContentTypeJSON(req)

	if got := req.Header.Get(HeaderUserAgent); got != "kongctl/0.5.0" {
		t.Fatalf("User-Agent = %q, want %q", got, "kongctl/0.5.0")
	}
	if got := req.Header.Get(HeaderAuthorization); got != "Bearer token" {
		t.Fatalf("Authorization = %q, want %q", got, "Bearer token")
	}
	if got := req.Header.Get(HeaderAccept); got != MediaTypeJSON {
		t.Fatalf("Accept = %q, want %q", got, MediaTypeJSON)
	}
	if got := req.Header.Get(HeaderContentType); got != MediaTypeJSON {
		t.Fatalf("Content-Type = %q, want %q", got, MediaTypeJSON)
	}
}
