//go:build e2e

package harness

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// HTTPAttempt contains metadata only, never request paths, headers, or bodies.
type HTTPAttempt struct {
	Method     string
	Host       string
	Status     int
	Duration   time.Duration
	Timeout    time.Duration
	TimedOut   bool
	ErrorClass string
}

func (c *CLI) observeHTTP(req *http.Request, resp *http.Response, err error, start time.Time, timeout time.Duration) {
	if c.ObserveHTTP == nil {
		return
	}
	a := HTTPAttempt{
		Method: req.Method, Host: req.URL.Hostname(), Duration: time.Since(start), Timeout: timeout,
	}
	if resp != nil {
		a.Status = resp.StatusCode
	}
	if err != nil {
		a.ErrorClass = "unknown"
		var netErr net.Error
		if errors.As(err, &netErr) {
			a.ErrorClass = "network"
			a.TimedOut = netErr.Timeout()
		}
		if errors.Is(err, context.DeadlineExceeded) {
			a.TimedOut = true
		}
	}
	c.ObserveHTTP(a)
}
