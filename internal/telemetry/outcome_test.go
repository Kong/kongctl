package telemetry

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/kong/kongctl/internal/cmd"
)

func TestCategorize(t *testing.T) {
	cancelledCtx, cancel := context.WithCancel(t.Context())
	cancel()

	deadlineErr := context.DeadlineExceeded

	cfgErr := &cmd.ConfigurationError{Err: errors.New("bad flag combination")}
	execErr := &cmd.ExecutionError{Msg: "boom", Err: errors.New("boom")}

	sdk := func(status int) *sdkerrors.SDKError {
		return &sdkerrors.SDKError{StatusCode: status, Message: "x"}
	}

	cases := []struct {
		name string
		err  error
		want Outcome
	}{
		{"nil", nil, OutcomeSuccess},
		{"context.Canceled direct", context.Canceled, OutcomeInterrupted},
		{"context.Canceled wrapped", fmt.Errorf("op: %w", cancelledCtx.Err()), OutcomeInterrupted},
		{"deadline exceeded", deadlineErr, OutcomeNetworkError},
		{"configuration error", cfgErr, OutcomeUserError},
		{"sdk 401", sdk(401), OutcomeUserError},
		{"sdk 403", sdk(403), OutcomeUserError},
		{"sdk 400", sdk(400), OutcomeAPIError},
		{"sdk 422", sdk(422), OutcomeAPIError},
		{"sdk 404", sdk(404), OutcomeAPIError},
		{"sdk 500", sdk(500), OutcomeAPIError},
		{"sdk 503 wrapped", fmt.Errorf("api call: %w", sdk(503)), OutcomeAPIError},
		{
			"url.Error",
			&url.Error{Op: "Get", URL: "https://x", Err: errors.New("dial tcp: refused")},
			OutcomeNetworkError,
		},
		{"net.Error timeout", &net.OpError{Op: "dial", Err: &timeoutNetErr{}}, OutcomeNetworkError},
		{"execution error", execErr, OutcomeInternalError},
		{"plain error", errors.New("oops"), OutcomeInternalError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Categorize(tc.err); got != tc.want {
				t.Fatalf("Categorize(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

type timeoutNetErr struct{}

func (timeoutNetErr) Error() string               { return "i/o timeout" }
func (timeoutNetErr) Timeout() bool               { return true }
func (timeoutNetErr) Temporary() bool             { return true }
func (timeoutNetErr) Unwrap() error               { return nil }
func (timeoutNetErr) Deadline() (time.Time, bool) { return time.Time{}, false }
