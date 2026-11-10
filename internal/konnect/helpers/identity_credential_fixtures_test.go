package helpers_test

import (
	"io"
	"net/http"
	"strings"
)

const (
	writeFixtureDirectory = "731559d3-39bd-48c7-b33c-eb192847eb20"
	writeFixturePrincipal = "731559d3-39bd-48c7-b33c-eb192847eb21"
	writeFixtureBasicAuth = "731559d3-39bd-48c7-b33c-eb192847eb22"
	writeFixtureID        = "731559d3-39bd-48c7-b33c-eb192847eb23"
	writeFixtureExpiry    = "2031-01-01T00:00:00Z"
)

func passwordPublicBody(expiration any) map[string]any {
	return map[string]any{
		"id": writeFixtureID, "expires_at": expiration,
		"managed_by": map[string]string{}, "created_at": "2026-10-31T00:00:00Z", "updated_at": "2026-10-31T00:00:00Z",
	}
}

type secretHTTPFunc func(*http.Request) (*http.Response, error)

func (f secretHTTPFunc) Do(request *http.Request) (*http.Response, error) { return f(request) }

func secretResponse(request *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(body)), Request: request,
	}
}

const (
	fixtureServerID = "731559d3-39bd-48c7-b33c-eb192847eb20"
)
