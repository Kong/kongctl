//go:build e2e

package harness

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestObserveHTTPPreservesFailedResourceEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	t.Setenv("KONGCTL_E2E_KONNECT_BASE_URL", server.URL)
	t.Setenv("KONGCTL_E2E_KONNECT_PAT", "secret-token")
	var observations []HTTPAttempt
	cli := &CLI{TestDir: t.TempDir(), ObserveHTTP: func(a HTTPAttempt) { observations = append(observations, a) }}
	step := &Step{cli: cli}
	// The public helper intentionally still discards failure results. Observing
	// the request must not change inputs to the existing retry policy.
	result, err := step.CreateResource("portal", nil, CreateResourceOptions{})
	if err == nil || result.Status != 0 {
		t.Fatalf("resource helper behavior changed: %+v, %v", result, err)
	}
	if len(observations) != 1 {
		t.Fatalf("expected one request observation: %+v", observations)
	}
	a := observations[0]
	if a.Status != 503 || a.Method != "POST" || a.Host != "127.0.0.1" || a.Duration <= 0 {
		t.Fatalf("incomplete failure evidence: %+v", a)
	}
}
