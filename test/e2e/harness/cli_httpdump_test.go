//go:build e2e

package harness

import (
	"strings"
	"testing"
)

func TestExtractHTTPDumps(t *testing.T) {
	stdout := "before\nrequest:\nGET /v1 HTTP/1.1\r\nHost: example\r\n\r\n\n\nresponse:\nHTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n\n\nafter\n"
	cleaned, dumps := extractHTTPDumps(stdout)
	if strings.Contains(cleaned, "request:") || strings.Contains(cleaned, "response:") {
		t.Fatalf("expected http dumps to be removed: %q", cleaned)
	}
	if !strings.Contains(cleaned, "before") || !strings.Contains(cleaned, "after") {
		t.Fatalf("expected surrounding output to remain: %q", cleaned)
	}
	if len(dumps) != 2 {
		t.Fatalf("expected 2 dumps, got %d", len(dumps))
	}
	if dumps[0].Kind != "request" {
		t.Fatalf("unexpected first dump kind: %s", dumps[0].Kind)
	}
	if dumps[1].Kind != "response" {
		t.Fatalf("unexpected second dump kind: %s", dumps[1].Kind)
	}
	if dumps[0].Body == "" || dumps[1].Body == "" {
		t.Fatalf("expected payloads to be captured: %#v", dumps)
	}
}

func TestExtractHTTPDumpsIgnoresNonHTTP(t *testing.T) {
	stdout := "request:\nthis is not http\n\npayload\n"
	cleaned, dumps := extractHTTPDumps(stdout)
	if cleaned != stdout {
		t.Fatalf("expected original output when block not recognized")
	}
	if len(dumps) != 0 {
		t.Fatalf("expected no dumps, got %d", len(dumps))
	}
}

func TestSanitizeHTTPDumpBody(t *testing.T) {
	body := "GET /foo HTTP/1.1\r\nAuthorization: Bearer secret-token\r\nHost: example\r\n"
	s := sanitizeHTTPDumpBody(body)
	if strings.Contains(s, "secret-token") {
		t.Fatalf("expected token to be redacted: %q", s)
	}
	if !strings.Contains(s, "Authorization: ***") {
		t.Fatalf("expected Authorization header to be replaced: %q", s)
	}
}
