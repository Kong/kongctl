package helpers

import (
	"context"
	"fmt"

	kkInternal "github.com/Kong/sdk-konnect-go-internal"
	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
	kkInternalOps "github.com/Kong/sdk-konnect-go-internal/models/operations"
)

// PortalAPI defines the interface for operations on Developer Portals
type PortalAPI interface {
	// Portal operations
	ListPortals(ctx context.Context, request kkInternalOps.ListPortalsRequest) (*kkInternalOps.ListPortalsResponse, error)
	GetPortal(ctx context.Context, id string) (*kkInternalOps.GetPortalResponse, error)
}

// InternalPortalAPI provides an implementation of the PortalAPI interface using the internal SDK
type InternalPortalAPI struct {
	SDK *kkInternal.SDK
}

// ListPortals implements the PortalAPI interface
func (p *InternalPortalAPI) ListPortals(
	ctx context.Context,
	request kkInternalOps.ListPortalsRequest,
) (*kkInternalOps.ListPortalsResponse, error) {
	return p.SDK.Portals.ListPortals(ctx, request)
}

// GetPortal implements the PortalAPI interface
func (p *InternalPortalAPI) GetPortal(ctx context.Context, id string) (*kkInternalOps.GetPortalResponse, error) {
	return p.SDK.Portals.GetPortal(ctx, id)
}

// GetAllPortals fetches all portals with pagination
func GetAllPortals(ctx context.Context, requestPageSize int64, kkClient *kkInternal.SDK,
) ([]kkInternalComps.Portal, error) {
	var allData []kkInternalComps.Portal

	var pageNumber int64 = 1
	for {
		req := kkInternalOps.ListPortalsRequest{
			PageSize:   kkInternal.Int64(requestPageSize),
			PageNumber: kkInternal.Int64(pageNumber),
		}

		res, err := kkClient.Portals.ListPortals(ctx, req)
		if err != nil {
			return nil, err
		}

		if res.ListPortalsResponse != nil && len(res.ListPortalsResponse.Data) > 0 {
			allData = append(allData, res.ListPortalsResponse.Data...)
			pageNumber++
		} else {
			break
		}
	}

	return allData, nil
}

// PageInfo represents a portal page with minimal info needed for Terraform import
type PageInfo struct {
	ID   string
	Name string
	Slug string
}

// SnippetInfo represents a portal snippet with minimal info needed for Terraform import
type SnippetInfo struct {
	ID   string
	Name string
}

// Int64 is a helper to convert int64 to *int64
func Int64(v int64) *int64 {
	return &v
}

// GetPagesForPortal returns a list of pages for a portal with pagination
// This function is designed to be used with the dump command to export portal pages
// as Terraform import blocks.
func GetPagesForPortal(ctx context.Context, portalAPI PortalAPI, portalID string) ([]PageInfo, error) {
	// Cast the portalAPI to InternalPortalAPI to access the SDK
	internalAPI, ok := portalAPI.(*InternalPortalAPI)
	if !ok || internalAPI == nil || internalAPI.SDK == nil {
		return nil, fmt.Errorf("invalid portal API implementation")
	}

	if internalAPI.SDK.Pages == nil {
		return nil, fmt.Errorf("SDK does not support Pages API")
	}

	var allPages []PageInfo
	var pageNumber int64 = 1
	const pageSize int64 = 100 // Default page size for pagination

	// Keep fetching pages until there are no more
	for {
		// Create a request to list portal pages for the specified portal
		req := kkInternalOps.ListPortalPagesRequest{
			PortalID:   portalID,
			PageSize:   Int64(pageSize),
			PageNumber: Int64(pageNumber),
		}

		// Call the SDK's ListPortalPages method
		res, err := internalAPI.SDK.Pages.ListPortalPages(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list portal pages: %w", err)
		}

		// Check if we have data in the response
		if res.ListPortalPagesResponse == nil || len(res.ListPortalPagesResponse.Data) == 0 {
			break
		}

		// Process the pages in the current batch
		for _, page := range res.ListPortalPagesResponse.Data {
			pageInfo := PageInfo{
				ID:   page.ID,
				Name: page.Title, // Title field maps to Name in our PageInfo struct
				Slug: page.Slug,
			}
			allPages = append(allPages, pageInfo)
		}

		// Increment the page number for the next request
		pageNumber++

		// If there are no more pages, break out of the loop
		// Check if we've received all pages based on the meta information
		if res.ListPortalPagesResponse.Meta.Page.Total <= float64(pageSize*(pageNumber-1)) {
			break
		}
	}

	return allPages, nil
}

// GetSnippetsForPortal returns a list of snippets for a portal with pagination
func GetSnippetsForPortal(ctx context.Context, portalAPI PortalAPI, portalID string) ([]SnippetInfo, error) {
	// Cast the portalAPI to InternalPortalAPI to access the SDK
	internalAPI, ok := portalAPI.(*InternalPortalAPI)
	if !ok || internalAPI == nil || internalAPI.SDK == nil {
		return nil, fmt.Errorf("invalid portal API implementation")
	}

	// Check if the SDK supports the V3PortalSnippets API
	if internalAPI.SDK.Snippets == nil {
		return nil, fmt.Errorf("SDK does not support V3PortalSnippets API")
	}

	var allSnippets []SnippetInfo
	var pageNumber int64 = 1
	const pageSize int64 = 100 // Default page size for pagination

	// Keep fetching snippets until there are no more
	for {
		// Create a request to list portal snippets for the specified portal
		req := kkInternalOps.ListPortalSnippetsRequest{
			PortalID:   portalID,
			PageSize:   Int64(pageSize),
			PageNumber: Int64(pageNumber),
		}

		// Call the SDK's ListPortalSnippets method
		res, err := internalAPI.SDK.Snippets.ListPortalSnippets(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list portal snippets: %w", err)
		}

		// Check if we have data in the response
		if res.ListPortalSnippetsResponse == nil || len(res.ListPortalSnippetsResponse.Data) == 0 {
			break
		}

		// Process the snippets in the current batch
		for _, snippet := range res.ListPortalSnippetsResponse.Data {
			snippetInfo := SnippetInfo{
				ID:   snippet.ID,
				Name: snippet.Name,
			}
			allSnippets = append(allSnippets, snippetInfo)
		}

		// Increment the page number for the next request
		pageNumber++

		// If there are no more pages, break out of the loop
		// Check if we've received all pages based on the meta information
		if res.ListPortalSnippetsResponse.Meta.Page.Total <= float64(pageSize*(pageNumber-1)) {
			break
		}
	}

	return allSnippets, nil
}

// HasPortalSettings checks if the portal has settings that can be exported
// Returns false if the operation isn't supported
func HasPortalSettings(ctx context.Context, portalAPI PortalAPI, portalID string) bool {
	// TODO: Implement with V3PortalSettings API when available
	// Follow the same pattern as GetPagesForPortal but check for existence instead of listing
	// For now, return false to indicate the feature is not yet implemented
	return false
}

// HasPortalAuthSettings checks if a portal has auth settings configured
func HasPortalAuthSettings(ctx context.Context, portalAPI PortalAPI, portalID string) bool {
	// Cast the portalAPI to InternalPortalAPI to access the SDK
	internalAPI, ok := portalAPI.(*InternalPortalAPI)
	if !ok || internalAPI == nil || internalAPI.SDK == nil {
		return false
	}

	// Check if the SDK supports the PortalAuthSettings API
	if internalAPI.SDK.PortalAuthSettings == nil {
		return false
	}

	// Check if we can get the auth settings for the portal
	// We don't need to actually fetch the data, just check if the API returns success
	// which means that auth settings exist
	_, err := internalAPI.SDK.PortalAuthSettings.GetPortalAuthenticationSettings(ctx, portalID)
	if err != nil {
		// If there's an error, the auth settings don't exist or couldn't be accessed
		return false
	}

	// No error means the auth settings exist
	return true
}

// HasCustomDomainForPortal checks if a portal has a custom domain configured
func HasCustomDomainForPortal(ctx context.Context, portalAPI PortalAPI, portalID string) bool {
	// Cast the portalAPI to InternalPortalAPI to access the SDK
	internalAPI, ok := portalAPI.(*InternalPortalAPI)
	if !ok || internalAPI == nil || internalAPI.SDK == nil {
		return false
	}

	// Check if the SDK supports the V3PortalCustomDomains API
	if internalAPI.SDK.PortalCustomDomains == nil {
		return false
	}

	// Check if we can get the custom domain for the portal
	// We don't need to actually fetch the data, just check if the API returns success
	// which means that a custom domain exists
	_, err := internalAPI.SDK.PortalCustomDomains.GetPortalCustomDomain(ctx, portalID)
	if err != nil {
		// If there's an error, the custom domain doesn't exist or couldn't be accessed
		return false
	}

	// No error means the custom domain exists
	return true
}
