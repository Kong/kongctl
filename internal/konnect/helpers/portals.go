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
func (p *InternalPortalAPI) ListPortals(ctx context.Context, request kkInternalOps.ListPortalsRequest) (*kkInternalOps.ListPortalsResponse, error) {
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
	// SDK doesn't currently expose the documents API fully
	// For now, return empty slice to allow compilation
	return []DocInfo{}, fmt.Errorf("documents API not fully supported yet")
}

// GetSpecificationsForPortal returns a list of specifications for a portal
// Returns an empty slice if the operation isn't supported
func GetSpecificationsForPortal(ctx context.Context, portalAPI PortalAPI, portalID string) ([]SpecInfo, error) {
	// SDK doesn't currently expose the specifications API fully
	// For now, return empty slice to allow compilation
	return []SpecInfo{}, fmt.Errorf("specifications API not fully supported yet")
}

// GetPagesForPortal returns a list of pages for a portal
// Returns an empty slice if the operation isn't supported
func GetPagesForPortal(ctx context.Context, portalAPI PortalAPI, portalID string) ([]PageInfo, error) {
	// SDK doesn't currently expose the pages API fully
	// For now, return empty slice to allow compilation
	return []PageInfo{}, fmt.Errorf("pages API not fully supported yet")
}

// HasPortalSettings checks if the portal has settings that can be exported
// Returns false if the operation isn't supported
func HasPortalSettings(ctx context.Context, portalAPI PortalAPI, portalID string) bool {
	// SDK doesn't currently expose the settings API fully
	// For now, return false to allow compilation
	return false
}