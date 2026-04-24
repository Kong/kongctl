package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalIdentityProviderAPI defines the interface for portal identity provider operations.
type PortalIdentityProviderAPI interface {
	ListPortalIdentityProviders(
		ctx context.Context,
		request kkOps.GetPortalIdentityProvidersRequest,
		opts ...kkOps.Option,
	) (*kkOps.GetPortalIdentityProvidersResponse, error)
	GetPortalIdentityProvider(
		ctx context.Context,
		portalID string,
		id string,
		opts ...kkOps.Option,
	) (*kkOps.GetPortalIdentityProviderResponse, error)
	CreatePortalIdentityProvider(
		ctx context.Context,
		portalID string,
		request kkComps.CreateIdentityProvider,
		opts ...kkOps.Option,
	) (*kkOps.CreatePortalIdentityProviderResponse, error)
	UpdatePortalIdentityProvider(
		ctx context.Context,
		request kkOps.UpdatePortalIdentityProviderRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdatePortalIdentityProviderResponse, error)
	DeletePortalIdentityProvider(
		ctx context.Context,
		portalID string,
		id string,
		opts ...kkOps.Option,
	) (*kkOps.DeletePortalIdentityProviderResponse, error)
}

// PortalIdentityProviderAPIImpl provides an implementation backed by the SDK.
type PortalIdentityProviderAPIImpl struct {
	SDK        *kkSDK.SDK
	BaseURL    string
	Token      string
	HTTPClient kkSDK.HTTPClient
}

// ListPortalIdentityProviders lists identity providers for a portal.
func (p *PortalIdentityProviderAPIImpl) ListPortalIdentityProviders(
	ctx context.Context,
	request kkOps.GetPortalIdentityProvidersRequest,
	opts ...kkOps.Option,
) (*kkOps.GetPortalIdentityProvidersResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalAuthSettings.GetPortalIdentityProviders(ctx, request.PortalID, request.Filter, opts...)
}

// GetPortalIdentityProvider fetches a single identity provider for a portal.
func (p *PortalIdentityProviderAPIImpl) GetPortalIdentityProvider(
	ctx context.Context,
	portalID string,
	id string,
	opts ...kkOps.Option,
) (*kkOps.GetPortalIdentityProviderResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalAuthSettings.GetPortalIdentityProvider(ctx, portalID, id, opts...)
}

// CreatePortalIdentityProvider creates a new identity provider for a portal.
func (p *PortalIdentityProviderAPIImpl) CreatePortalIdentityProvider(
	ctx context.Context,
	portalID string,
	request kkComps.CreateIdentityProvider,
	opts ...kkOps.Option,
) (*kkOps.CreatePortalIdentityProviderResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	if p.BaseURL == "" || p.HTTPClient == nil {
		return p.SDK.PortalAuthSettings.CreatePortalIdentityProvider(ctx, portalID, request, opts...)
	}

	sdk := kkSDK.New(
		kkSDK.WithServerURL(p.BaseURL),
		kkSDK.WithSecurity(kkComps.Security{
			PersonalAccessToken: &p.Token,
		}),
		kkSDK.WithClient(&portalIdentityProviderCreateHTTPClient{base: p.HTTPClient}),
	)
	if sdk == nil || sdk.PortalAuthSettings == nil {
		return nil, fmt.Errorf("failed to initialize SDK for portal identity provider create")
	}
	return sdk.PortalAuthSettings.CreatePortalIdentityProvider(ctx, portalID, request, opts...)
}

// UpdatePortalIdentityProvider updates an identity provider for a portal.
func (p *PortalIdentityProviderAPIImpl) UpdatePortalIdentityProvider(
	ctx context.Context,
	request kkOps.UpdatePortalIdentityProviderRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdatePortalIdentityProviderResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalAuthSettings.UpdatePortalIdentityProvider(ctx, request, opts...)
}

// DeletePortalIdentityProvider deletes an identity provider from a portal.
func (p *PortalIdentityProviderAPIImpl) DeletePortalIdentityProvider(
	ctx context.Context,
	portalID string,
	id string,
	opts ...kkOps.Option,
) (*kkOps.DeletePortalIdentityProviderResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalAuthSettings.DeletePortalIdentityProvider(ctx, portalID, id, opts...)
}

type portalIdentityProviderCreateHTTPClient struct {
	base kkSDK.HTTPClient
}

func (c *portalIdentityProviderCreateHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if c == nil || c.base == nil {
		return nil, fmt.Errorf("http client is not configured")
	}

	if err := stripEnabledFromPortalIdentityProviderCreateRequest(req); err != nil {
		return nil, err
	}

	return c.base.Do(req)
}

func stripEnabledFromPortalIdentityProviderCreateRequest(req *http.Request) error {
	if !isPortalIdentityProviderCreateRequest(req) {
		return nil
	}

	body, err := readAndRestoreRequestBody(req)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to decode portal identity provider create request: %w", err)
	}
	if _, ok := payload["enabled"]; !ok {
		return nil
	}

	delete(payload, "enabled")

	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode portal identity provider create request: %w", err)
	}
	restoreRequestBody(req, encoded)
	return nil
}

func isPortalIdentityProviderCreateRequest(req *http.Request) bool {
	if req == nil || req.URL == nil {
		return false
	}
	if req.Method != http.MethodPost {
		return false
	}

	path := strings.Trim(req.URL.Path, "/")
	segments := strings.Split(path, "/")
	if len(segments) != 4 {
		return false
	}

	return segments[0] == "v3" &&
		segments[1] == "portals" &&
		segments[2] != "" &&
		segments[3] == "identity-providers"
}

func readAndRestoreRequestBody(req *http.Request) ([]byte, error) {
	if req == nil || req.Body == nil {
		return nil, nil
	}

	originalBody := req.Body
	defer originalBody.Close()

	body, err := io.ReadAll(originalBody)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	restoreRequestBody(req, body)
	return body, nil
}

func restoreRequestBody(req *http.Request, body []byte) {
	if req == nil {
		return
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))
}

var _ PortalIdentityProviderAPI = (*PortalIdentityProviderAPIImpl)(nil)
