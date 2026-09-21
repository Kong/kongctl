//go:build e2e

package scenario

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/kong/kongctl/test/e2e/harness"
)

func TestSyntheticCreateDoesNotReplayUnknownOutcome(t *testing.T) {
	var posts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	defer server.Close()
	t.Setenv("KONGCTL_E2E_KONNECT_BASE_URL", server.URL)
	t.Setenv("KONGCTL_E2E_KONNECT_PAT", "secret")
	t.Setenv("KONGCTL_E2E_HTTP_TIMEOUT", "20ms")
	dir := t.TempDir()
	cli := &harness.CLI{TestDir: dir}
	s := Scenario{Defaults: Defaults{Retry: Retry{Attempts: 3, Interval: "1ms"}}, Steps: []Step{{
		Name: "create", SkipInputs: true, Commands: []Command{{Name: "create-portal", Create: &CreateSpec{
			Resource: "portal", Payload: CreatePayload{Inline: map[string]any{"name": "portal"}},
		}}},
	}}}
	d := newScenarioDiagnostics(filepath.Join(dir, "scenario-diagnostics.json"), "fixture")
	err := executeScenario(t, "fixture", s, cli, d)
	if !harness.IsCreateOutcomeUnknown(err) {
		t.Fatalf("expected unknown outcome: %v", err)
	}
	if posts.Load() != 1 {
		t.Fatalf("replayed ambiguous POST %d times", posts.Load())
	}
	if d.current.RetryStop != "outcome_unknown" {
		t.Fatalf("incorrect retry stop: %+v", d.current)
	}
	if finishErr := d.finish(err); finishErr != nil {
		t.Fatal(finishErr)
	}
	if d.Failure == nil || d.Failure.Cause != "create_outcome_unknown" {
		t.Fatalf("incorrect failure cause: %+v", d.Failure)
	}
}
