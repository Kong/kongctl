//go:build e2e

package harness

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultHTTPTimeout = 15 * time.Second

	DefaultResetTimeout              = 3 * time.Minute
	DefaultResetRetryAttempts        = 3
	DefaultResetRetryInterval        = 1 * time.Second
	DefaultResetRetryMaxInterval     = 5 * time.Second
	DefaultResetRetryBackoffFactor   = 2.0
	DefaultResetRetryJitter          = 250 * time.Millisecond
	DefaultRawHTTPRetryAttempts      = 4
	DefaultRawHTTPRetryInterval      = 1 * time.Second
	DefaultRawHTTPRetryMaxInterval   = 5 * time.Second
	DefaultRawHTTPRetryBackoffFactor = 2.0
	DefaultRawHTTPRetryJitter        = 250 * time.Millisecond
)

type HTTPRetryPolicy struct {
	RequestTimeout time.Duration
	TotalTimeout   time.Duration
	Backoff        BackoffConfig
}

func defaultResetHTTPPolicy() HTTPRetryPolicy {
	return HTTPRetryPolicy{
		RequestTimeout: DefaultHTTPTimeout,
		TotalTimeout:   DefaultResetTimeout,
		Backoff: BackoffConfig{
			Attempts: DefaultResetRetryAttempts,
			Base:     DefaultResetRetryInterval,
			Max:      DefaultResetRetryMaxInterval,
			Factor:   DefaultResetRetryBackoffFactor,
			Jitter:   DefaultResetRetryJitter,
		},
	}
}

func resetHTTPPolicyFromEnv() HTTPRetryPolicy {
	policy := defaultResetHTTPPolicy()
	policy.RequestTimeout = timeoutEnv("KONGCTL_E2E_RESET_HTTP_TIMEOUT", policy.RequestTimeout)
	policy.TotalTimeout = timeoutEnv("KONGCTL_E2E_RESET_TIMEOUT", policy.TotalTimeout)
	policy.Backoff = BackoffConfig{
		Attempts: intEnv("KONGCTL_E2E_RESET_RETRY_ATTEMPTS", policy.Backoff.Attempts),
		Base:     durationEnv("KONGCTL_E2E_RESET_RETRY_INTERVAL", policy.Backoff.Base),
		Max:      durationEnv("KONGCTL_E2E_RESET_RETRY_MAX_INTERVAL", policy.Backoff.Max),
		Factor:   floatEnv("KONGCTL_E2E_RESET_RETRY_BACKOFF_FACTOR", policy.Backoff.Factor),
		Jitter:   durationEnv("KONGCTL_E2E_RESET_RETRY_JITTER", policy.Backoff.Jitter),
	}
	return policy
}

func HTTPRequestTimeout() time.Duration {
	return timeoutEnv("KONGCTL_E2E_HTTP_TIMEOUT", DefaultHTTPTimeout)
}

func RawHTTPRetryDefaults() BackoffConfig {
	return BackoffConfig{
		Attempts: intEnv("KONGCTL_E2E_HTTP_RETRY_ATTEMPTS", DefaultRawHTTPRetryAttempts),
		Base:     durationEnv("KONGCTL_E2E_HTTP_RETRY_INTERVAL", DefaultRawHTTPRetryInterval),
		Max:      durationEnv("KONGCTL_E2E_HTTP_RETRY_MAX_INTERVAL", DefaultRawHTTPRetryMaxInterval),
		Factor:   floatEnv("KONGCTL_E2E_HTTP_RETRY_BACKOFF_FACTOR", DefaultRawHTTPRetryBackoffFactor),
		Jitter:   durationEnv("KONGCTL_E2E_HTTP_RETRY_JITTER", DefaultRawHTTPRetryJitter),
	}
}

func HTTPTransportOptionsFromEnv() HTTPTransportOptions {
	return HTTPTransportOptions{
		TCPUserTimeout:            timeoutEnv("KONGCTL_E2E_HTTP_TCP_USER_TIMEOUT", 0),
		DisableKeepAlives:         boolEnv("KONGCTL_E2E_HTTP_DISABLE_KEEPALIVES", false),
		RecycleConnectionsOnError: boolEnv("KONGCTL_E2E_HTTP_RECYCLE_CONNECTIONS_ON_ERROR", false),
	}
}

func timeoutEnv(name string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	if timeoutDisabled(raw) {
		return 0
	}

	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		Warnf("invalid %s=%q; using %s", name, raw, fallback)
		return fallback
	}
	return d
}

func durationEnv(name string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		Warnf("invalid %s=%q; using %s", name, raw, fallback)
		return fallback
	}
	return d
}

func timeoutDisabled(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "0", "default", "defaults", "disable", "disabled", "none", "off", "platform", "system":
		return true
	default:
		return false
	}
}

func intEnv(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		Warnf("invalid %s=%q; using %d", name, raw, fallback)
		return fallback
	}
	return n
}

func floatEnv(name string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || n <= 0 {
		Warnf("invalid %s=%q; using %.2f", name, raw, fallback)
		return fallback
	}
	return n
}

func boolEnv(name string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on", "y":
		return true
	case "0", "false", "no", "off", "n":
		return false
	default:
		Warnf("invalid %s=%q; using %t", name, raw, fallback)
		return fallback
	}
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return newHTTPClientWithOptions(timeout, HTTPTransportOptionsFromEnv())
}

func newHTTPClientWithOptions(timeout time.Duration, options HTTPTransportOptions) *http.Client {
	if timeout <= 0 {
		return &http.Client{Transport: newHTTPTransport(options)}
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: newHTTPTransport(options),
	}
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
