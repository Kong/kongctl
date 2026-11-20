//go:build e2e

package harness

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"syscall"
	"testing"
)

func TestScenarioOperationEligibility(t *testing.T) {
	for _, tc := range []struct {
		name  string
		err   error
		class string
	}{
		{"500", &httpError{status: 500}, "http_5xx"},
		{"599", &httpError{status: 599}, "http_5xx"},
		{"429", &httpError{status: 429}, ""},
		{"400", &httpError{status: 400}, ""},
		{"timeout", &url.Error{Op: "Post", Err: context.DeadlineExceeded}, "request_timeout"},
		{"reset", &url.Error{Op: "Post", Err: syscall.ECONNRESET}, "connection_reset"},
		{"unknown-post", &CreateOutcomeUnknownError{Cause: &httpError{status: 500}}, "http_5xx"},
		{"refused", syscall.ECONNREFUSED, ""},
		{"unknown", errors.New("500 context deadline exceeded"), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := fmt.Errorf("operation: %w", tc.err)
			got := ScenarioOperationFailure(original)
			var failure *ScenarioFailure
			eligible := errors.As(got, &failure)
			if eligible != (tc.class != "") || eligible && failure.Class != tc.class {
				t.Fatalf("incorrect classification: %#v", got)
			}
			if !errors.Is(got, tc.err) || got.Error() != original.Error() {
				t.Fatalf("lost original failure: %v", got)
			}
		})
	}
}

func TestScenarioCommandEligibility(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result Result
		class  string
	}{
		{"sdk", Result{Stderr: "Error: API error occurred: Status 500\n{\"instance\":\"kong:trace:123\"}"}, "http_5xx"},
		{"status", Result{Stderr: "Error: unexpected status 503 (expected 201)"}, "http_5xx"},
		{"problem", Result{Stderr: `Error: request failed: {"status":500,"instance":"kong:trace:123"}`}, "http_5xx"},
		{"problem-detail", Result{Stderr: `Error: {"status":400,"detail":"unexpected status 500"}`}, ""},
		{"timeout", Result{Stderr: `Error: Post "https://example.test": context deadline exceeded`}, "request_timeout"},
		{"unknown-deadline", Result{Stderr: "Error: context deadline exceeded"}, ""},
		{"reset", Result{Stderr: "Error: read tcp: connection reset by peer"}, "connection_reset"},
		{"process-deadline", Result{Stderr: "context deadline exceeded", TimedOut: true}, ""},
		{"config", Result{Stderr: "unknown flag: 500"}, ""},
		{"stdout", Result{Stdout: "unexpected status 500"}, ""},
		{"other-status", Result{Stderr: "Error: API error occurred: Status 401"}, ""},
		{"historical", Result{Stderr: "WARN unexpected status 500\nError: invalid configuration"}, ""},
		{"body", Result{Stderr: "Error: API error occurred: Status 400\ncontext deadline exceeded"}, ""},
		{"unknown", Result{Stderr: "command failed"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := errors.New(tc.result.Stderr)
			got := ScenarioCommandFailure(original, tc.result)
			var failure *ScenarioFailure
			eligible := errors.As(got, &failure)
			if eligible != (tc.class != "") || eligible && failure.Class != tc.class {
				t.Fatalf("incorrect classification: %#v", got)
			}
			if !errors.Is(got, original) {
				t.Fatal("lost original command error")
			}
		})
	}
}
