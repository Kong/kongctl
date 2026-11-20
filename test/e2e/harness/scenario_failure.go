//go:build e2e

package harness

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"regexp"
	"strings"
	"syscall"
)

// ScenarioFailure records eligibility independently of command retry policy.
// Callers must only attach it to an unexpected operation failure.
type ScenarioFailure struct {
	Cause error
	Class string
}

func (e *ScenarioFailure) Error() string { return e.Cause.Error() }
func (e *ScenarioFailure) Unwrap() error { return e.Cause }

func ScenarioOperationFailure(err error) error {
	if err == nil {
		return nil
	}
	class := ""
	var status *httpError
	var network net.Error
	switch {
	case errors.As(err, &status) && status.status >= 500 && status.status <= 599:
		class = "http_5xx"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &network) && network.Timeout():
		class = "request_timeout"
	case errors.Is(err, syscall.ECONNRESET):
		class = "connection_reset"
	}
	if class == "" {
		return err
	}
	return &ScenarioFailure{Cause: err, Class: class}
}

// The subprocess loses Go error types. Match explicit SDK/HTTP error syntax
// in the failed command's stderr, never scenario names or historical attempts.
var scenarioStatusPattern = regexp.MustCompile(
	`(?i)(?:status code[:=]?\s*|unexpected status\s+|status=|: status\s+)(5[0-9]{2})\b`,
)

var scenarioRequestPattern = regexp.MustCompile(`\b(?:get|head|post|put|patch|delete|options) "https?://`)

func ScenarioCommandFailure(err error, result Result) error {
	if err == nil || result.TimedOut {
		// A subprocess deadline does not prove an HTTP request timed out.
		return err
	}
	detail := ""
	for line := range strings.SplitSeq(result.Stderr, "\n") {
		if message, ok := strings.CutPrefix(strings.TrimSpace(line), "Error: "); ok {
			detail = strings.ToLower(message)
			break
		}
	}
	// Typed SDK problem responses render as JSON rather than "Status 500".
	// Inspect only their top-level status, excluding free-text detail fields.
	if header, body, ok := strings.Cut(detail, "{"); ok {
		var problem struct {
			Status int `json:"status"`
		}
		if json.Unmarshal([]byte("{"+body), &problem) == nil && problem.Status >= 400 {
			if problem.Status >= 500 && problem.Status <= 599 {
				return &ScenarioFailure{Cause: err, Class: "http_5xx"}
			}
			return err
		}
		detail = header
	}
	class := ""
	switch {
	case scenarioStatusPattern.MatchString(detail):
		class = "http_5xx"
	case strings.Contains(detail, "client.timeout exceeded"), strings.Contains(detail, "tls handshake timeout"),
		scenarioRequestPattern.MatchString(detail) && (strings.Contains(detail, "i/o timeout") ||
			strings.Contains(detail, "context deadline exceeded")):
		class = "request_timeout"
	case strings.Contains(detail, "connection reset by peer"):
		class = "connection_reset"
	}
	if class == "" {
		return err
	}
	return &ScenarioFailure{Cause: err, Class: class}
}
