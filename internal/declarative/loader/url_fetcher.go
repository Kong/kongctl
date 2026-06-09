package loader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kong/kongctl/internal/konnect/httpclient"
	applog "github.com/kong/kongctl/internal/log"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/httpheaders"
)

const (
	defaultURLFetchTimeout     = 30 * time.Second
	defaultURLFetchMaxBytes    = 50 * 1024 * 1024
	defaultURLFetchMaxAttempts = 3
	defaultURLFetchMaxRedirect = 10
	defaultURLFetchBackoff     = 100 * time.Millisecond
)

// URLFetchAuthPolicy controls when remote declarative URL fetches may include authentication.
type URLFetchAuthPolicy string

const (
	// URLFetchAuthAuto sends the configured bearer token only to HTTPS URLs whose host is explicitly allowed.
	URLFetchAuthAuto URLFetchAuthPolicy = "auto"
	// URLFetchAuthNone disables authentication for remote declarative URL fetches.
	URLFetchAuthNone URLFetchAuthPolicy = "none"
)

// URLFetchTokenSource provides bearer tokens for authenticated remote declarative URL fetches.
type URLFetchTokenSource interface {
	Token(context.Context) (string, error)
}

// URLFetchOptions controls remote declarative URL fetch behavior that should be selected by the command layer.
type URLFetchOptions struct {
	AuthPolicy       URLFetchAuthPolicy
	AuthAllowedHosts []string
	TokenSource      URLFetchTokenSource
}

type urlFetchConfig struct {
	client       *http.Client
	maxBytes     int64
	maxAttempts  int
	maxRedirects int
	backoff      time.Duration
	options      URLFetchOptions
}

// FetchURL retrieves a declarative configuration source from an HTTP(S) URL.
func FetchURL(ctx context.Context, rawURL string) ([]byte, error) {
	return fetchURL(ctx, rawURL, urlFetchConfig{})
}

// FetchURLWithOptions retrieves a declarative configuration source from an HTTP(S) URL using the provided options.
func FetchURLWithOptions(ctx context.Context, rawURL string, options URLFetchOptions) ([]byte, error) {
	return fetchURL(ctx, rawURL, urlFetchConfig{options: options})
}

func fetchURL(ctx context.Context, rawURL string, cfg urlFetchConfig) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := parseFetchURL(rawURL); err != nil {
		return nil, err
	}

	client := configuredFetchClient(cfg)
	maxAttempts := cfg.maxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultURLFetchMaxAttempts
	}
	backoff := cfg.backoff
	if backoff <= 0 {
		backoff = defaultURLFetchBackoff
	}

	var lastErr error
	for attempt := range maxAttempts {
		body, retryable, err := fetchURLOnce(ctx, client, rawURL, maxResponseBytes(cfg), cfg.options, attempt+1)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retryable || attempt == maxAttempts-1 {
			return nil, err
		}
		if err := waitBeforeURLFetchRetry(ctx, backoff, attempt); err != nil {
			return nil, fmt.Errorf("failed to fetch URL %s: %w", rawURL, err)
		}
	}

	return nil, lastErr
}

func parseFetchURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("URL must include a host")
	}
	return parsed, nil
}

func configuredFetchClient(cfg urlFetchConfig) *http.Client {
	var client http.Client
	if cfg.client != nil {
		client = *cfg.client
	} else {
		client = *httpclient.NewHTTPClient(defaultURLFetchTimeout)
	}
	client.CheckRedirect = checkURLFetchRedirect(maxRedirects(cfg), cfg.options)
	return &client
}

func maxResponseBytes(cfg urlFetchConfig) int64 {
	if cfg.maxBytes > 0 {
		return cfg.maxBytes
	}
	return defaultURLFetchMaxBytes
}

func maxRedirects(cfg urlFetchConfig) int {
	if cfg.maxRedirects > 0 {
		return cfg.maxRedirects
	}
	return defaultURLFetchMaxRedirect
}

func checkURLFetchRedirect(maxRedirects int, options URLFetchOptions) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		if len(via) > 0 && via[len(via)-1].URL.Scheme == "https" && req.URL.Scheme == "http" {
			return fmt.Errorf("refusing to follow HTTPS to HTTP redirect")
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			return fmt.Errorf("unsupported redirect URL scheme %q", req.URL.Scheme)
		}
		return configureURLFetchAuth(req, options)
	}
}

func fetchURLOnce(
	ctx context.Context,
	client *http.Client,
	rawURL string,
	maxBytes int64,
	options URLFetchOptions,
	attempt int,
) ([]byte, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create request for URL %s: %w", rawURL, err)
	}
	httpheaders.SetUserAgent(req, meta.UserAgent())
	httpheaders.SetAccept(req, "application/yaml, text/yaml, text/plain, */*")
	if err := configureURLFetchAuth(req, options); err != nil {
		return nil, false, err
	}

	if logger := loggerFromContext(ctx); logger != nil {
		logger.Debug("fetching remote declarative configuration",
			slog.String("url", safeURLForLog(rawURL)),
			slog.Int("attempt", attempt),
			slog.Bool("authenticated", req.Header.Get(httpheaders.HeaderAuthorization) != ""))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, isRetryableURLFetchError(ctx, err), fmt.Errorf("failed to fetch URL %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		statusErr := unexpectedURLFetchStatusError(rawURL, resp)
		return nil, isRetryableURLFetchStatus(resp.StatusCode), statusErr
	}

	if resp.ContentLength > maxBytes {
		return nil, false, fmt.Errorf(
			"failed to fetch URL %s: response is too large (%d bytes, limit %d bytes)",
			rawURL, resp.ContentLength, maxBytes,
		)
	}

	if logger := loggerFromContext(ctx); logger != nil {
		contentType := resp.Header.Get(httpheaders.HeaderContentType)
		if contentType != "" && !isExpectedURLFetchContentType(contentType) {
			logger.Debug("remote declarative configuration has unexpected content type",
				slog.String("url", safeURLForLog(rawURL)),
				slog.String("content_type", contentType))
		}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, isRetryableURLFetchError(ctx, err), fmt.Errorf("failed to read URL %s: %w", rawURL, err)
	}
	if int64(len(body)) > maxBytes {
		return nil, false, fmt.Errorf(
			"failed to fetch URL %s: response is too large (limit %d bytes)",
			rawURL, maxBytes,
		)
	}

	return body, false, nil
}

// AllowsAuthenticationForURL reports whether the URL is within the configured authentication boundary.
func (o URLFetchOptions) AllowsAuthenticationForURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return o.allowsAuthenticationForParsedURL(parsed)
}

func (o URLFetchOptions) shouldAuthenticateURL(u *url.URL) bool {
	return o.TokenSource != nil && o.allowsAuthenticationForParsedURL(u)
}

func (o URLFetchOptions) allowsAuthenticationForParsedURL(u *url.URL) bool {
	if remoteURLFetchAuthPolicyOrDefault(o.AuthPolicy) != URLFetchAuthAuto {
		return false
	}
	if u == nil || u.Scheme != "https" {
		return false
	}
	return urlFetchAuthHostAllowed(u.Hostname(), o.AuthAllowedHosts)
}

func remoteURLFetchAuthPolicyOrDefault(policy URLFetchAuthPolicy) URLFetchAuthPolicy {
	if policy == "" {
		return URLFetchAuthAuto
	}
	return policy
}

func configureURLFetchAuth(req *http.Request, options URLFetchOptions) error {
	if req == nil {
		return nil
	}
	if !options.shouldAuthenticateURL(req.URL) {
		req.Header.Del(httpheaders.HeaderAuthorization)
		return nil
	}

	token, err := options.TokenSource.Token(req.Context())
	if err != nil {
		return fmt.Errorf("failed to resolve authentication token for URL %s: %w", req.URL.String(), err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("failed to resolve authentication token for URL %s: token is empty", req.URL.String())
	}
	httpheaders.SetBearerAuthorization(req, token)
	return nil
}

func urlFetchAuthHostAllowed(host string, allowedHosts []string) bool {
	host = normalizeURLFetchAuthHost(host)
	if host == "" {
		return false
	}
	for _, allowedHost := range allowedHosts {
		allowedHost = normalizeURLFetchAuthHost(allowedHost)
		if allowedHost == "" {
			continue
		}
		if host == allowedHost || strings.HasSuffix(host, "."+allowedHost) {
			return true
		}
	}
	return false
}

func normalizeURLFetchAuthHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return ""
	}
	if parsed, err := url.Parse(host); err == nil && parsed.Hostname() != "" {
		host = parsed.Hostname()
	} else if splitHost, _, err := net.SplitHostPort(host); err == nil {
		host = splitHost
	}
	host = strings.Trim(host, "[]")
	return strings.TrimSuffix(host, ".")
}

func unexpectedURLFetchStatusError(rawURL string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	snippet := strings.TrimSpace(string(body))
	if snippet == "" {
		return fmt.Errorf("failed to fetch URL %s: unexpected HTTP status %s", rawURL, resp.Status)
	}
	return fmt.Errorf("failed to fetch URL %s: unexpected HTTP status %s: %s", rawURL, resp.Status, snippet)
}

func isRetryableURLFetchStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

func isRetryableURLFetchError(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func waitBeforeURLFetchRetry(ctx context.Context, base time.Duration, attempt int) error {
	delay := min(base<<attempt, time.Second)
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isExpectedURLFetchContentType(contentType string) bool {
	mediaType, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(contentType)), ";")
	switch mediaType {
	case "", "application/yaml", "application/x-yaml", "text/yaml", "text/x-yaml", "text/plain",
		"application/octet-stream":
		return true
	default:
		return strings.HasSuffix(mediaType, "+yaml")
	}
}

func loggerFromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return nil
	}
	logger, _ := ctx.Value(applog.LoggerKey).(*slog.Logger)
	return logger
}

func safeURLForLog(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}
