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
	return p.SDK.V3Portals.ListPortals(ctx, request)
}

// GetPortal implements the PortalAPI interface
func (p *InternalPortalAPI) GetPortal(ctx context.Context, id string) (*kkInternalOps.GetPortalResponse, error) {
	return p.SDK.V3Portals.GetPortal(ctx, id)
}

// GetAllPortals fetches all portals with pagination
func GetAllPortals(ctx context.Context, requestPageSize int64, kkClient *kkInternal.SDK,
) ([]kkInternalComps.PortalV3, error) {
	var allData []kkInternalComps.PortalV3

	var pageNumber int64 = 1
	for {
		req := kkInternalOps.ListPortalsRequest{
			PageSize:   kkInternal.Int64(requestPageSize),
			PageNumber: kkInternal.Int64(pageNumber),
		}

		res, err := kkClient.V3Portals.ListPortals(ctx, req)
		if err != nil {
			return nil, err
		}

		if res.ListPortalsResponseV3 != nil && len(res.ListPortalsResponseV3.Data) > 0 {
			allData = append(allData, res.ListPortalsResponseV3.Data...)
			pageNumber++
		} else {
			break
		}
	}

	return allData, nil
}

// DocInfo represents a portal document with minimal info needed for Terraform import
type DocInfo struct {
	ID   string
	Slug string
}

// SpecInfo represents a portal specification with minimal info needed for Terraform import
type SpecInfo struct {
	ID   string
	Name string
}

// PageInfo represents a portal page with minimal info needed for Terraform import
type PageInfo struct {
	ID   string
	Name string
	Slug string
}

// Int64 is a helper to convert int64 to *int64
func Int64(v int64) *int64 {
	return &v
}

// GetDocumentsForPortal returns a list of documents for a portal
// Returns an empty slice if the operation isn't supported
func GetDocumentsForPortal(ctx context.Context, portalAPI PortalAPI, portalID string) ([]DocInfo, error) {
	// TODO: Implement with V3PortalDocuments API when available using the same pattern as GetPagesForPortal
	// For now, return empty slice to indicate the feature is not yet implemented
	return []DocInfo{}, fmt.Errorf("documents API not fully implemented yet")
}

// GetSpecificationsForPortal returns a list of specifications for a portal
// Returns an empty slice if the operation isn't supported
func GetSpecificationsForPortal(ctx context.Context, portalAPI PortalAPI, portalID string) ([]SpecInfo, error) {
	// TODO: Implement with API Publications or similar API when available using the same pattern as GetPagesForPortal
	// Can use the SDK's ListAPIPublications method from the apipublication.go file when available
	// For now, return empty slice to indicate the feature is not yet implemented
	return []SpecInfo{}, fmt.Errorf("specifications API not fully implemented yet")
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

	// Check if the SDK supports the V3PortalPages API
	if internalAPI.SDK.V3PortalPages == nil {
		return nil, fmt.Errorf("SDK does not support V3PortalPages API")
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
		res, err := internalAPI.SDK.V3PortalPages.ListPortalPages(ctx, req)
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

// HasPortalSettings checks if the portal has settings that can be exported
// Returns false if the operation isn't supported
func HasPortalSettings(ctx context.Context, portalAPI PortalAPI, portalID string) bool {
	// TODO: Implement with V3PortalSettings API when available
	// Follow the same pattern as GetPagesForPortal but check for existence instead of listing
	// For now, return false to indicate the feature is not yet implemented
	return false
}
