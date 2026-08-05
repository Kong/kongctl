package helpers

import (
	"io"
	"net/http"
	"strings"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComponents "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type portalCustomizationCapturingClient struct {
	request *http.Request
	body    []byte
}

func (c *portalCustomizationCapturingClient) Do(req *http.Request) (*http.Response, error) {
	c.request = req
	body, readErr := io.ReadAll(req.Body)
	closeErr := req.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	c.body = body

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{}`)),
		Request:    req,
	}, nil
}

func TestPortalCustomizationAPIImplUpdatePreservesExplicitEmptyMenuLists(t *testing.T) {
	client := &portalCustomizationCapturingClient{}
	sdk := kkSDK.New(
		kkSDK.WithServerURL("https://example.test"),
		kkSDK.WithClient(client),
	)
	layout := "sidebar"
	api := &PortalCustomizationAPIImpl{
		SDK:        sdk,
		BaseURL:    "https://example.test",
		Token:      "test-token",
		HTTPClient: client,
	}

	_, err := api.UpdatePortalCustomization(t.Context(), "portal-123", &kkComponents.PortalCustomization{
		Layout: &layout,
		Menu: &kkComponents.Menu{
			Main:           []kkComponents.PortalMenuItem{},
			FooterSections: []kkComponents.PortalFooterMenuSection{},
			FooterBottom:   []kkComponents.PortalMenuItem{},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, client.request)
	assert.Equal(t, http.MethodPatch, client.request.Method)
	assert.Equal(t, "https://example.test/v3/portals/portal-123/customization", client.request.URL.String())
	assert.Equal(t, "Bearer test-token", client.request.Header.Get("Authorization"))
	assert.JSONEq(t, `{
		"layout": "sidebar",
		"menu": {
			"main": [],
			"footer_sections": [],
			"footer_bottom": []
		}
	}`, string(client.body))
}

func TestPortalCustomizationAPIImplUpdateUsesSDKForNonEmptyMenuLists(t *testing.T) {
	client := &portalCustomizationCapturingClient{}
	sdk := kkSDK.New(
		kkSDK.WithServerURL("https://example.test"),
		kkSDK.WithClient(client),
	)
	api := &PortalCustomizationAPIImpl{SDK: sdk}

	_, err := api.UpdatePortalCustomization(t.Context(), "portal-123", &kkComponents.PortalCustomization{
		Menu: &kkComponents.Menu{
			Main: []kkComponents.PortalMenuItem{{
				Path:       "/docs",
				Title:      "Docs",
				Visibility: kkComponents.PortalMenuItemVisibilityPublic,
			}},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, client.request)
	assert.JSONEq(t, `{
		"menu": {
			"main": [{
				"path": "/docs",
				"title": "Docs",
				"visibility": "public",
				"external": false
			}]
		}
	}`, string(client.body))
}
