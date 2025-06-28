package state

import (
	"context"
	"fmt"
	"os"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
	kkInternalOps "github.com/Kong/sdk-konnect-go-internal/models/operations"
)

// Client wraps Konnect SDK for state management
type Client struct {
	portalAPI helpers.PortalAPI
}

// NewClient creates a new state client
func NewClient(portalAPI helpers.PortalAPI) *Client {
	return &Client{
		portalAPI: portalAPI,
	}
}

// Portal represents a normalized portal for internal use
type Portal struct {
	kkInternalComps.Portal
	NormalizedLabels map[string]string // Non-pointer labels
}

// ListManagedPortals returns all KONGCTL-managed portals
func (c *Client) ListManagedPortals(ctx context.Context) ([]Portal, error) {
	var allPortals []Portal
	var pageNumber int64 = 1
	pageSize := int64(100)

	for {
		req := kkInternalOps.ListPortalsRequest{
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
	portal kkInternalComps.CreatePortal,
	configHash string,
) (*kkInternalComps.PortalResponse, error) {
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
	
	normalized = labels.AddManagedLabels(normalized, configHash)
	debugLog("After adding managed labels: %+v", normalized)
	
	portal.Labels = labels.DenormalizeLabels(normalized)
	debugLog("Final denormalized labels: %+v", portal.Labels)

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
	portal kkInternalComps.UpdatePortal,
	configHash string,
) (*kkInternalComps.PortalResponse, error) {
	// Add management labels
	normalized := labels.NormalizeLabels(portal.Labels)
	normalized = labels.AddManagedLabels(normalized, configHash)
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