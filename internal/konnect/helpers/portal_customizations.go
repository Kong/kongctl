package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComponents "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"

	"github.com/kong/kongctl/internal/konnect/apiutil"
)

// PortalCustomizationAPI defines the interface for operations on Portal Customizations
type PortalCustomizationAPI interface {
	// Portal Customization operations (singleton resource - no create/delete)
	UpdatePortalCustomization(ctx context.Context, portalID string, request *kkComponents.PortalCustomization,
		opts ...kkOps.Option) (*kkOps.UpdatePortalCustomizationResponse, error)
	GetPortalCustomization(ctx context.Context, portalID string,
		opts ...kkOps.Option) (*kkOps.GetPortalCustomizationResponse, error)
}

// PortalCustomizationAPIImpl provides an implementation of the PortalCustomizationAPI interface
type PortalCustomizationAPIImpl struct {
	SDK         *kkSDK.SDK
	BaseURL     string
	Token       string
	TokenSource apiutil.TokenSource
	HTTPClient  kkSDK.HTTPClient
}

// UpdatePortalCustomization implements the PortalCustomizationAPI interface
func (p *PortalCustomizationAPIImpl) UpdatePortalCustomization(
	ctx context.Context, portalID string, request *kkComponents.PortalCustomization,
	opts ...kkOps.Option,
) (*kkOps.UpdatePortalCustomizationResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	if requiresExplicitEmptyPortalMenu(request) {
		return p.updatePortalCustomizationWithExplicitEmptyMenu(ctx, portalID, request)
	}
	return p.SDK.PortalCustomization.UpdatePortalCustomization(ctx, portalID, request, opts...)
}

// GetPortalCustomization implements the PortalCustomizationAPI interface
func (p *PortalCustomizationAPIImpl) GetPortalCustomization(
	ctx context.Context, portalID string,
	opts ...kkOps.Option,
) (*kkOps.GetPortalCustomizationResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalCustomization.GetPortalCustomization(ctx, portalID, opts...)
}

type portalCustomizationPayload struct {
	Theme        *kkComponents.Theme        `json:"theme,omitempty"`
	Layout       *string                    `json:"layout,omitempty"`
	CSS          *string                    `json:"css,omitempty"`
	Menu         *portalCustomizationMenu   `json:"menu,omitempty"`
	SpecRenderer *kkComponents.SpecRenderer `json:"spec_renderer,omitempty"`
	Robots       *string                    `json:"robots,omitempty"`
}

type portalCustomizationMenu struct {
	Main           *[]kkComponents.PortalMenuItem          `json:"main,omitempty"`
	FooterSections *[]kkComponents.PortalFooterMenuSection `json:"footer_sections,omitempty"`
	FooterBottom   *[]kkComponents.PortalMenuItem          `json:"footer_bottom,omitempty"`
}

func requiresExplicitEmptyPortalMenu(customization *kkComponents.PortalCustomization) bool {
	if customization == nil || customization.Menu == nil {
		return false
	}

	menu := customization.Menu
	return menu.Main != nil && len(menu.Main) == 0 ||
		menu.FooterSections != nil && len(menu.FooterSections) == 0 ||
		menu.FooterBottom != nil && len(menu.FooterBottom) == 0
}

func marshalPortalCustomizationPayload(customization *kkComponents.PortalCustomization) ([]byte, error) {
	payload := portalCustomizationPayload{
		Theme:        customization.Theme,
		Layout:       customization.Layout,
		CSS:          customization.CSS,
		SpecRenderer: customization.SpecRenderer,
		Robots:       customization.Robots,
	}

	if customization.Menu != nil {
		payload.Menu = &portalCustomizationMenu{}
		if customization.Menu.Main != nil {
			payload.Menu.Main = &customization.Menu.Main
		}
		if customization.Menu.FooterSections != nil {
			payload.Menu.FooterSections = &customization.Menu.FooterSections
		}
		if customization.Menu.FooterBottom != nil {
			payload.Menu.FooterBottom = &customization.Menu.FooterBottom
		}
	}

	return json.Marshal(payload)
}

func (p *PortalCustomizationAPIImpl) updatePortalCustomizationWithExplicitEmptyMenu(
	ctx context.Context,
	portalID string,
	customization *kkComponents.PortalCustomization,
) (*kkOps.UpdatePortalCustomizationResponse, error) {
	if strings.TrimSpace(p.BaseURL) == "" {
		return nil, fmt.Errorf("base URL is required for portal customization requests")
	}

	payload, err := marshalPortalCustomizationPayload(customization)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal portal customization request: %w", err)
	}

	path := fmt.Sprintf("/v3/portals/%s/customization", url.PathEscape(portalID))
	result, err := p.request(
		ctx,
		http.MethodPatch,
		path,
		map[string]string{contentTypeHeader: applicationJSONContent},
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, err
	}

	response := &kkOps.UpdatePortalCustomizationResponse{
		ContentType: result.Header.Get(contentTypeHeader),
		StatusCode:  result.StatusCode,
		RawResponse: &http.Response{
			StatusCode: result.StatusCode,
			Header:     result.Header.Clone(),
			Body:       io.NopCloser(bytes.NewReader(result.Body)),
		},
	}

	if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
		body := strings.TrimSpace(string(result.Body))
		if body == "" {
			return nil, fmt.Errorf("update portal customization failed with status %d", result.StatusCode)
		}
		return nil, fmt.Errorf("update portal customization failed with status %d: %s", result.StatusCode, body)
	}

	if len(bytes.TrimSpace(result.Body)) == 0 {
		return response, nil
	}

	var customizationResponse kkComponents.PortalCustomization
	if err := json.Unmarshal(result.Body, &customizationResponse); err != nil {
		return nil, fmt.Errorf("failed to decode portal customization response: %w", err)
	}
	response.PortalCustomization = &customizationResponse

	return response, nil
}

func (p *PortalCustomizationAPIImpl) request(
	ctx context.Context,
	method string,
	path string,
	headers map[string]string,
	body io.Reader,
) (*apiutil.Result, error) {
	if p.TokenSource != nil {
		return apiutil.RequestWithTokenSource(ctx, p.HTTPClient, method, p.BaseURL, path, p.TokenSource, headers, body)
	}
	return apiutil.Request(ctx, p.HTTPClient, method, p.BaseURL, path, p.Token, headers, body)
}
