package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalAPI defines the interface for operations on Developer Portals
type PortalAPI interface {
	// Portal operations
	ListPortals(ctx context.Context, request kkOps.ListPortalsRequest) (*kkOps.ListPortalsResponse, error)
	GetPortal(ctx context.Context, id string) (*kkOps.GetPortalResponse, error)
	CreatePortal(ctx context.Context, portal kkComps.CreatePortal) (*kkOps.CreatePortalResponse, error)
	UpdatePortal(ctx context.Context, id string,
		portal kkComps.UpdatePortal) (*kkOps.UpdatePortalResponse, error)
	DeletePortal(ctx context.Context, id string, force bool) (*kkOps.DeletePortalResponse, error)
}

// PublicPortalAPI provides an implementation of the PortalAPI interface using the public SDK
type PublicPortalAPI struct {
	SDK *kkSDK.SDK
}

// ListPortals implements the PortalAPI interface
func (p *PublicPortalAPI) ListPortals(
	ctx context.Context,
	request kkOps.ListPortalsRequest,
) (*kkOps.ListPortalsResponse, error) {
	return p.SDK.Portals.ListPortals(ctx, request)
}

// GetPortal implements the PortalAPI interface
func (p *PublicPortalAPI) GetPortal(ctx context.Context, id string) (*kkOps.GetPortalResponse, error) {
	return p.SDK.Portals.GetPortal(ctx, id)
}

// CreatePortal implements the PortalAPI interface
func (p *PublicPortalAPI) CreatePortal(
	ctx context.Context,
	portal kkComps.CreatePortal,
) (*kkOps.CreatePortalResponse, error) {
	return p.SDK.Portals.CreatePortal(ctx, portal)
}

// UpdatePortal implements the PortalAPI interface
func (p *PublicPortalAPI) UpdatePortal(
	ctx context.Context,
	id string,
	portal kkComps.UpdatePortal,
) (*kkOps.UpdatePortalResponse, error) {
	return p.SDK.Portals.UpdatePortal(ctx, id, portal)
}

// DeletePortal implements the PortalAPI interface
func (p *PublicPortalAPI) DeletePortal(
	ctx context.Context,
	id string,
	force bool,
) (*kkOps.DeletePortalResponse, error) {
	var forceParam *kkOps.QueryParamForce
	if force {
		forceTrue := kkOps.QueryParamForceTrue
		forceParam = &forceTrue
	}
	return p.SDK.Portals.DeletePortal(ctx, id, forceParam)
}

// GetAllPortals fetches all portals with pagination
func GetAllPortals(ctx context.Context, requestPageSize int64, kkClient *kkSDK.SDK,
) ([]kkComps.Portal, error) {
	var allData []kkComps.Portal

	var pageNumber int64 = 1
	for {
		req := kkOps.ListPortalsRequest{
			PageSize:   kkSDK.Int64(requestPageSize),
			PageNumber: kkSDK.Int64(pageNumber),
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
	// Cast the portalAPI to PublicPortalAPI to access the SDK
	publicAPI, ok := portalAPI.(*PublicPortalAPI)
	if !ok || publicAPI == nil || publicAPI.SDK == nil {
		return nil, fmt.Errorf("invalid portal API implementation")
	}

	if publicAPI.SDK.Pages == nil {
		return nil, fmt.Errorf("SDK does not support Pages API")
	}

	var allPages []PageInfo

	// Note: The public SDK v0.6.0 doesn't support pagination for ListPortalPages
	// This is a limitation compared to the internal SDK
	// For now, we'll fetch all pages in a single request
	req := kkOps.ListPortalPagesRequest{
		PortalID: portalID,
	}

	// Call the SDK's ListPortalPages method
	res, err := publicAPI.SDK.Pages.ListPortalPages(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list portal pages: %w", err)
	}

	// Check if we have data in the response
	if res.ListPortalPagesResponse == nil || len(res.ListPortalPagesResponse.Data) == 0 {
		return allPages, nil
	}

	// Process all pages
	for _, page := range res.ListPortalPagesResponse.Data {
		pageInfo := PageInfo{
			ID:   page.ID,
			Name: page.Title, // Title field maps to Name in our PageInfo struct
			Slug: page.Slug,
		}
		allPages = append(allPages, pageInfo)
	}

	return allPages, nil
}

// GetSnippetsForPortal returns a list of snippets for a portal with pagination
func GetSnippetsForPortal(ctx context.Context, portalAPI PortalAPI, portalID string) ([]SnippetInfo, error) {
	// Cast the portalAPI to PublicPortalAPI to access the SDK
	publicAPI, ok := portalAPI.(*PublicPortalAPI)
	if !ok || publicAPI == nil || publicAPI.SDK == nil {
		return nil, fmt.Errorf("invalid portal API implementation")
	}

	// Check if the SDK supports the Snippets API
	if publicAPI.SDK.Snippets == nil {
		return nil, fmt.Errorf("SDK does not support Snippets API")
	}

	var allSnippets []SnippetInfo

	// Note: The public SDK v0.6.0 doesn't support pagination for ListPortalSnippets
	// This is a limitation compared to the internal SDK
	// For now, we'll fetch all snippets in a single request
	req := kkOps.ListPortalSnippetsRequest{
		PortalID: portalID,
	}

	// Call the SDK's ListPortalSnippets method
	res, err := publicAPI.SDK.Snippets.ListPortalSnippets(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list portal snippets: %w", err)
	}

	// Check if we have data in the response
	if res.ListPortalSnippetsResponse == nil || len(res.ListPortalSnippetsResponse.Data) == 0 {
		return allSnippets, nil
	}

	// Process all snippets
	for _, snippet := range res.ListPortalSnippetsResponse.Data {
		snippetInfo := SnippetInfo{
			ID:   snippet.ID,
			Name: snippet.Name,
		}
		allSnippets = append(allSnippets, snippetInfo)
	}

	return allSnippets, nil
}

// HasPortalSettings checks if the portal has settings that can be exported
// Returns false if the operation isn't supported
func HasPortalSettings(_ context.Context, _ PortalAPI, _ string) bool {
	// TODO: Implement with V3PortalSettings API when available
	// Follow the same pattern as GetPagesForPortal but check for existence instead of listing
	// For now, return false to indicate the feature is not yet implemented
	return false
}

// HasPortalAuthSettings checks if a portal has auth settings configured
func HasPortalAuthSettings(ctx context.Context, portalAPI PortalAPI, portalID string) bool {
	// Cast the portalAPI to PublicPortalAPI to access the SDK
	publicAPI, ok := portalAPI.(*PublicPortalAPI)
	if !ok || publicAPI == nil || publicAPI.SDK == nil {
		return false
	}

	// Check if the SDK supports the PortalAuthSettings API
	if publicAPI.SDK.PortalAuthSettings == nil {
		return false
	}

	// Check if we can get the auth settings for the portal
	// We don't need to actually fetch the data, just check if the API returns success
	// which means that auth settings exist
	_, err := publicAPI.SDK.PortalAuthSettings.GetPortalAuthenticationSettings(ctx, portalID)
	if err != nil {
		// If there's an error, the auth settings don't exist or couldn't be accessed
		return false
	}

	// No error means the auth settings exist
	return true
}

// HasPortalCustomization checks if a portal has customization settings configured
func HasPortalCustomization(ctx context.Context, portalAPI PortalAPI, portalID string) bool {
	// Cast the portalAPI to PublicPortalAPI to access the SDK
	publicAPI, ok := portalAPI.(*PublicPortalAPI)
	if !ok || publicAPI == nil || publicAPI.SDK == nil {
		return false
	}

	// Check if the SDK supports the PortalCustomization API
	if publicAPI.SDK.PortalCustomization == nil {
		return false
	}

	// Check if we can get the customization settings for the portal
	// We don't need to actually fetch the data, just check if the API returns success
	// which means that customization settings exist
	_, err := publicAPI.SDK.PortalCustomization.GetPortalCustomization(ctx, portalID)
	if err != nil {
		// If there's an error, the customization settings don't exist or couldn't be accessed
		return false
	}

	// No error means the customization settings exist
	return true
}

// HasCustomDomainForPortal checks if a portal has a custom domain configured
func HasCustomDomainForPortal(ctx context.Context, portalAPI PortalAPI, portalID string) bool {
	// Cast the portalAPI to PublicPortalAPI to access the SDK
	publicAPI, ok := portalAPI.(*PublicPortalAPI)
	if !ok || publicAPI == nil || publicAPI.SDK == nil {
		return false
	}

	// Check if the SDK supports the PortalCustomDomains API
	if publicAPI.SDK.PortalCustomDomains == nil {
		return false
	}

	// Check if we can get the custom domain for the portal
	// We don't need to actually fetch the data, just check if the API returns success
	// which means that a custom domain exists
	_, err := publicAPI.SDK.PortalCustomDomains.GetPortalCustomDomain(ctx, portalID)
	if err != nil {
		// If there's an error, the custom domain doesn't exist or couldn't be accessed
		return false
	}

	// No error means the custom domain exists
	return true
}
