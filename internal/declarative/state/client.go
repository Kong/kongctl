package state

import (
	"context"
	"fmt"
	"os"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// Client wraps Konnect SDK for state management
type Client struct {
	portalAPI helpers.PortalAPI
	apiAPI    helpers.APIAPI
}

// NewClient creates a new state client
func NewClient(portalAPI helpers.PortalAPI) *Client {
	return &Client{
		portalAPI: portalAPI,
	}
}

// NewClientWithAPIs creates a new state client with API support
func NewClientWithAPIs(portalAPI helpers.PortalAPI, apiAPI helpers.APIAPI) *Client {
	return &Client{
		portalAPI: portalAPI,
		apiAPI:    apiAPI,
	}
}

// Portal represents a normalized portal for internal use
type Portal struct {
	kkComps.Portal
	NormalizedLabels map[string]string // Non-pointer labels
}

// API represents a normalized API for internal use
type API struct {
	kkComps.APIResponseSchema
	NormalizedLabels map[string]string // Non-pointer labels
}

// ListManagedPortals returns all KONGCTL-managed portals
func (c *Client) ListManagedPortals(ctx context.Context) ([]Portal, error) {
	var allPortals []Portal
	var pageNumber int64 = 1
	pageSize := int64(100)

	for {
		req := kkOps.ListPortalsRequest{
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
		}

		resp, err := c.portalAPI.ListPortals(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list portals: %w", err)
		}

		if resp.ListPortalsResponse == nil || len(resp.ListPortalsResponse.Data) == 0 {
			break
		}

		// Process and filter portals
		for _, p := range resp.ListPortalsResponse.Data {
			// Labels are already map[string]string in the SDK
			normalized := p.Labels
			if normalized == nil {
				normalized = make(map[string]string)
			}

			if labels.IsManagedResource(normalized) {
				portal := Portal{
					Portal:           p,
					NormalizedLabels: normalized,
				}
				allPortals = append(allPortals, portal)
			}
		}

		pageNumber++

		// Check if we've fetched all pages
		if resp.ListPortalsResponse.Meta.Page.Total <= float64(pageSize*(pageNumber-1)) {
			break
		}
	}

	return allPortals, nil
}

// GetPortalByName finds a managed portal by name
func (c *Client) GetPortalByName(ctx context.Context, name string) (*Portal, error) {
	portals, err := c.ListManagedPortals(ctx)
	if err != nil {
		return nil, err
	}

	for _, p := range portals {
		if p.Name == name {
			return &p, nil
		}
	}

	return nil, nil // Not found
}

// CreatePortal creates a new portal with management labels
func (c *Client) CreatePortal(
	ctx context.Context,
	portal kkComps.CreatePortal,
) (*kkComps.PortalResponse, error) {
	// Debug logging
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == "true"
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG [state/client]: "+format+"\n", args...)
		}
	}
	
	debugLog("CreatePortal called with labels: %+v", portal.Labels)
	
	// Add management labels
	normalized := labels.NormalizeLabels(portal.Labels)
	debugLog("Normalized labels: %+v", normalized)
	
	normalized = labels.AddManagedLabels(normalized)
	debugLog("After adding managed labels: %+v", normalized)
	
	portal.Labels = labels.DenormalizeLabels(normalized)
	// Log actual label values for debugging
	if portal.Labels != nil {
		debugLog("Final labels for portal:")
		for k, v := range portal.Labels {
			if v != nil {
				debugLog("  %s = %s", k, *v)
			} else {
				debugLog("  %s = <nil>", k)
			}
		}
	}

	resp, err := c.portalAPI.CreatePortal(ctx, portal)
	if err != nil {
		return nil, fmt.Errorf("failed to create portal: %w", err)
	}

	if resp.PortalResponse == nil {
		return nil, fmt.Errorf("create portal response missing portal data")
	}

	return resp.PortalResponse, nil
}

// UpdatePortal updates an existing portal with new management labels
func (c *Client) UpdatePortal(
	ctx context.Context,
	id string,
	portal kkComps.UpdatePortal,
) (*kkComps.PortalResponse, error) {
	// Add management labels
	normalized := labels.NormalizeLabels(portal.Labels)
	normalized = labels.AddManagedLabels(normalized)
	portal.Labels = labels.DenormalizeLabels(normalized)

	resp, err := c.portalAPI.UpdatePortal(ctx, id, portal)
	if err != nil {
		return nil, fmt.Errorf("failed to update portal: %w", err)
	}

	if resp.PortalResponse == nil {
		return nil, fmt.Errorf("update portal response missing portal data")
	}

	return resp.PortalResponse, nil
}

// DeletePortal deletes a portal by ID
func (c *Client) DeletePortal(ctx context.Context, id string, force bool) error {
	_, err := c.portalAPI.DeletePortal(ctx, id, force)
	if err != nil {
		return fmt.Errorf("failed to delete portal: %w", err)
	}
	return nil
}

// ListManagedAPIs returns all KONGCTL-managed APIs
func (c *Client) ListManagedAPIs(ctx context.Context) ([]API, error) {
	if c.apiAPI == nil {
		return nil, fmt.Errorf("API client not configured")
	}

	var allAPIs []API
	var pageNumber int64 = 1
	pageSize := int64(100)

	for {
		req := kkOps.ListApisRequest{
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
		}

		resp, err := c.apiAPI.ListApis(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list APIs: %w", err)
		}

		if resp.ListAPIResponse == nil || len(resp.ListAPIResponse.Data) == 0 {
			break
		}

		// Process and filter APIs
		for _, a := range resp.ListAPIResponse.Data {
			// Labels are already map[string]string in the SDK
			normalized := a.Labels
			if normalized == nil {
				normalized = make(map[string]string)
			}

			if labels.IsManagedResource(normalized) {
				api := API{
					APIResponseSchema: a,
					NormalizedLabels:  normalized,
				}
				allAPIs = append(allAPIs, api)
			}
		}

		pageNumber++

		// Check if we've fetched all pages
		if resp.ListAPIResponse.Meta.Page.Total <= float64(pageSize*(pageNumber-1)) {
			break
		}
	}

	return allAPIs, nil
}

// GetAPIByName finds a managed API by name
func (c *Client) GetAPIByName(ctx context.Context, name string) (*API, error) {
	apis, err := c.ListManagedAPIs(ctx)
	if err != nil {
		return nil, err
	}

	for _, a := range apis {
		if a.Name == name {
			return &a, nil
		}
	}

	return nil, nil // Not found
}

// CreateAPI creates a new API with management labels
func (c *Client) CreateAPI(
	ctx context.Context,
	api kkComps.CreateAPIRequest,
) (*kkComps.APIResponseSchema, error) {
	if c.apiAPI == nil {
		return nil, fmt.Errorf("API client not configured")
	}

	// Debug logging
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == "true"
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG [state/client]: "+format+"\n", args...)
		}
	}
	
	debugLog("CreateAPI called with labels: %+v", api.Labels)
	
	// Add management labels - API labels are already non-pointer strings
	if api.Labels == nil {
		api.Labels = make(map[string]string)
	}
	
	api.Labels = labels.AddManagedLabels(api.Labels)
	debugLog("After adding managed labels: %+v", api.Labels)
	
	// Log actual label values for debugging
	if api.Labels != nil {
		debugLog("Final labels for API:")
		for k, v := range api.Labels {
			debugLog("  %s = %s", k, v)
		}
	}

	resp, err := c.apiAPI.CreateAPI(ctx, api)
	if err != nil {
		return nil, fmt.Errorf("failed to create API: %w", err)
	}

	if resp.APIResponseSchema == nil {
		return nil, fmt.Errorf("create API response missing API data")
	}

	return resp.APIResponseSchema, nil
}

// UpdateAPI updates an existing API with new management labels
func (c *Client) UpdateAPI(
	ctx context.Context,
	id string,
	api kkComps.UpdateAPIRequest,
) (*kkComps.APIResponseSchema, error) {
	if c.apiAPI == nil {
		return nil, fmt.Errorf("API client not configured")
	}

	// Add management labels
	normalized := labels.NormalizeLabels(api.Labels)
	normalized = labels.AddManagedLabels(normalized)
	api.Labels = labels.DenormalizeLabels(normalized)

	resp, err := c.apiAPI.UpdateAPI(ctx, id, api)
	if err != nil {
		return nil, fmt.Errorf("failed to update API: %w", err)
	}

	if resp.APIResponseSchema == nil {
		return nil, fmt.Errorf("update API response missing API data")
	}

	return resp.APIResponseSchema, nil
}

// DeleteAPI deletes an API by ID
func (c *Client) DeleteAPI(ctx context.Context, id string) error {
	if c.apiAPI == nil {
		return fmt.Errorf("API client not configured")
	}

	_, err := c.apiAPI.DeleteAPI(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete API: %w", err)
	}
	return nil
}