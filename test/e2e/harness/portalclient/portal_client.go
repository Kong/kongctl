package portalclient

import (
	"net/http"
	"net/http/cookiejar"
	"time"
)

// PortalOption configures the thin portal API helper.
type PortalOption func(*portalOptions)

type portalOptions struct {
	httpClient     *http.Client
	requestEditors []RequestEditorFn
}

// WithPortalHTTPClient injects a pre-configured HTTP client.
func WithPortalHTTPClient(client *http.Client) PortalOption {
	return func(opts *portalOptions) {
		opts.httpClient = client
	}
}

// WithPortalRequestEditor registers a default request mutation hook.
func WithPortalRequestEditor(editor RequestEditorFn) PortalOption {
	return func(opts *portalOptions) {
		if editor == nil {
			return
		}
		opts.requestEditors = append(opts.requestEditors, editor)
	}
}

// PortalAPI wraps the generated client with sane defaults for e2e tests.
type PortalAPI struct {
	client *ClientWithResponses
}

// NewPortalAPI creates the generated client with default transport behavior
// (cookie jar, timeout, etc.) so e2e tests can call the portal endpoints.
func NewPortalAPI(baseURL string, opts ...PortalOption) (*PortalAPI, error) {
	cfg := portalOptions{}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.httpClient == nil {
		jar, _ := cookiejar.New(nil)
		cfg.httpClient = &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
		}
	}

	clientOpts := []ClientOption{WithHTTPClient(cfg.httpClient)}
	for _, editor := range cfg.requestEditors {
		clientOpts = append(clientOpts, WithRequestEditorFn(editor))
	}

	genClient, err := NewClientWithResponses(baseURL, clientOpts...)
	if err != nil {
		return nil, err
	}

	return &PortalAPI{
		client: genClient,
	}, nil
}

// Raw exposes the underlying generated client for direct method calls.
func (p *PortalAPI) Raw() *ClientWithResponses {
	return p.client
}
