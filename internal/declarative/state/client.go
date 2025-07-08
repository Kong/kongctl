package state

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/log"
)

// ClientConfig contains all the API interfaces needed by the state client
type ClientConfig struct {
	// Core APIs
	PortalAPI  helpers.PortalAPI
	APIAPI     helpers.APIAPI
	AppAuthAPI helpers.AppAuthStrategiesAPI
	
	// Portal child resource APIs
	PortalPageAPI          helpers.PortalPageAPI
	PortalCustomizationAPI helpers.PortalCustomizationAPI
	PortalCustomDomainAPI  helpers.PortalCustomDomainAPI
	PortalSnippetAPI       helpers.PortalSnippetAPI
	
	// API child resource APIs
	APIVersionAPI        helpers.APIVersionAPI
	APIPublicationAPI    helpers.APIPublicationAPI
	APIImplementationAPI helpers.APIImplementationAPI
	APIDocumentAPI       helpers.APIDocumentAPI
}

// Client wraps Konnect SDK for state management
type Client struct {
	// Core APIs
	portalAPI  helpers.PortalAPI
	apiAPI     helpers.APIAPI
	appAuthAPI helpers.AppAuthStrategiesAPI
	
	// Portal child resource APIs
	portalPageAPI          helpers.PortalPageAPI
	portalCustomizationAPI helpers.PortalCustomizationAPI
	portalCustomDomainAPI  helpers.PortalCustomDomainAPI
	portalSnippetAPI       helpers.PortalSnippetAPI
	
	// API child resource APIs
	apiVersionAPI        helpers.APIVersionAPI
	apiPublicationAPI    helpers.APIPublicationAPI
	apiImplementationAPI helpers.APIImplementationAPI
	apiDocumentAPI       helpers.APIDocumentAPI
}

// NewClient creates a new state client with the provided configuration
func NewClient(config ClientConfig) *Client {
	return &Client{
		// Core APIs
		portalAPI:  config.PortalAPI,
		apiAPI:     config.APIAPI,
		appAuthAPI: config.AppAuthAPI,
		
		// Portal child resource APIs
		portalPageAPI:          config.PortalPageAPI,
		portalCustomizationAPI: config.PortalCustomizationAPI,
		portalCustomDomainAPI:  config.PortalCustomDomainAPI,
		portalSnippetAPI:       config.PortalSnippetAPI,
		
		// API child resource APIs
		apiVersionAPI:        config.APIVersionAPI,
		apiPublicationAPI:    config.APIPublicationAPI,
		apiImplementationAPI: config.APIImplementationAPI,
		apiDocumentAPI:       config.APIDocumentAPI,
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

// APIVersion represents an API version for internal use
type APIVersion struct {
	ID            string
	Version       string
	PublishStatus string
	Deprecated    bool
	SunsetDate    string
}

// APIPublication represents an API publication for internal use
type APIPublication struct {
	ID                       string
	PortalID                 string
	AuthStrategyIDs          []string
	AutoApproveRegistrations bool
	Visibility               string
}

// APIImplementation represents an API implementation for internal use
type APIImplementation struct {
	ID                string
	ImplementationURL string
	Service           *struct {
		ID             string
		ControlPlaneID string
	}
}

// APIDocument represents an API document for internal use
type APIDocument struct {
	ID               string
	Content          string
	Title            string
	Slug             string
	Status           string
	ParentDocumentID string
}

// ApplicationAuthStrategy represents a normalized auth strategy for internal use
type ApplicationAuthStrategy struct {
	ID               string
	Name             string
	DisplayName      string
	StrategyType     string
	Configs          map[string]interface{}
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

// GetPortalByFilter finds a managed portal using a filter expression
func (c *Client) GetPortalByFilter(ctx context.Context, filter string) (*Portal, error) {
	if c.portalAPI == nil {
		return nil, fmt.Errorf("Portal API client not configured")
	}

	// Use the filter in the SDK list operation
	// For now, we'll use ListManagedPortals and filter locally
	// TODO: Update when SDK supports server-side filtering
	portals, err := c.ListManagedPortals(ctx)
	if err != nil {
		return nil, err
	}

	// Parse filter (e.g., "name[eq]=foo")
	if strings.HasPrefix(filter, "name[eq]=") {
		name := strings.TrimPrefix(filter, "name[eq]=")
		for _, p := range portals {
			if p.Name == name {
				return &p, nil
			}
		}
	}

	return nil, nil // Not found
}

// CreatePortal creates a new portal with management labels
func (c *Client) CreatePortal(
	ctx context.Context,
	portal kkComps.CreatePortal,
) (*kkComps.PortalResponse, error) {
	// Get logger from context
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)

	logger.Debug("CreatePortal called",
		slog.Any("labels", portal.Labels))

	// Add management labels
	normalized := labels.NormalizeLabels(portal.Labels)
	logger.Debug("Normalized labels",
		slog.Any("labels", normalized))

	normalized = labels.AddManagedLabels(normalized)
	logger.Debug("After adding managed labels",
		slog.Any("labels", normalized))

	portal.Labels = labels.DenormalizeLabels(normalized)
	// Log actual label values for debugging
	if portal.Labels != nil {
		for k, v := range portal.Labels {
			if v != nil {
				logger.Debug("Final portal label",
					slog.String("key", k),
					slog.String("value", *v))
			} else {
				logger.Debug("Final portal label",
					slog.String("key", k),
					slog.String("value", "<nil>"))
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
	// Add management labels directly to pointer map to preserve nil values
	// This allows label removal (nil values) to work correctly
	portal.Labels = labels.AddManagedLabelsToPointerMap(portal.Labels)

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

// GetAPIByFilter finds a managed API using a filter expression
func (c *Client) GetAPIByFilter(ctx context.Context, filter string) (*API, error) {
	if c.apiAPI == nil {
		return nil, fmt.Errorf("API client not configured")
	}

	// Use the filter in the SDK list operation
	// For now, we'll use ListManagedAPIs and filter locally
	// TODO: Update when SDK supports server-side filtering
	apis, err := c.ListManagedAPIs(ctx)
	if err != nil {
		return nil, err
	}

	// Parse filter (e.g., "name[eq]=foo")
	if strings.HasPrefix(filter, "name[eq]=") {
		name := strings.TrimPrefix(filter, "name[eq]=")
		for _, a := range apis {
			if a.Name == name {
				return &a, nil
			}
		}
	}

	return nil, nil // Not found
}

// GetAPIByRef finds a managed API by declarative ref (stored in labels)
// TODO: This will be replaced by filtered lookup in Phase 2
func (c *Client) GetAPIByRef(ctx context.Context, ref string) (*API, error) {
	apis, err := c.ListManagedAPIs(ctx)
	if err != nil {
		return nil, err
	}

	for _, a := range apis {
		// For now, we'll search by name assuming ref == name
		// This will be improved with proper identity resolution
		if a.Name == ref {
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

	// Get logger from context
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)

	logger.Debug("CreateAPI called",
		slog.Any("labels", api.Labels))

	// Add management labels - API labels are already non-pointer strings
	if api.Labels == nil {
		api.Labels = make(map[string]string)
	}

	api.Labels = labels.AddManagedLabels(api.Labels)
	logger.Debug("After adding managed labels",
		slog.Any("labels", api.Labels))

	// Log actual label values for debugging
	if api.Labels != nil {
		for k, v := range api.Labels {
			logger.Debug("Final API label",
				slog.String("key", k),
				slog.String("value", v))
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

	// Add management labels directly to pointer map to preserve nil values
	// This allows label removal (nil values) to work correctly
	api.Labels = labels.AddManagedLabelsToPointerMap(api.Labels)

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

// API Version methods

// ListAPIVersions returns all versions for an API
func (c *Client) ListAPIVersions(ctx context.Context, apiID string) ([]APIVersion, error) {
	if c.apiVersionAPI == nil {
		return nil, fmt.Errorf("API version client not configured")
	}

	var allVersions []APIVersion
	var pageNumber int64 = 1
	pageSize := int64(100)

	for {
		req := kkOps.ListAPIVersionsRequest{
			APIID:      apiID,
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
		}

		resp, err := c.apiVersionAPI.ListAPIVersions(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list API versions: %w", err)
		}

		if resp.ListAPIVersionResponse == nil || len(resp.ListAPIVersionResponse.Data) == 0 {
			break
		}

		for _, v := range resp.ListAPIVersionResponse.Data {
			version := APIVersion{
				ID:      v.ID,
				Version: v.Version,
				// Other fields not available in list response
			}
			allVersions = append(allVersions, version)
		}

		pageNumber++

		// Check if we've fetched all pages
		if resp.ListAPIVersionResponse.Meta.Page.Total <= float64(pageSize*(pageNumber-1)) {
			break
		}
	}

	return allVersions, nil
}

// CreateAPIVersion creates a new API version
func (c *Client) CreateAPIVersion(
	ctx context.Context, apiID string, version kkComps.CreateAPIVersionRequest,
) (*kkComps.APIVersionResponse, error) {
	if c.apiVersionAPI == nil {
		return nil, fmt.Errorf("API version client not configured")
	}

	resp, err := c.apiVersionAPI.CreateAPIVersion(ctx, apiID, version)
	if err != nil {
		return nil, fmt.Errorf("failed to create API version: %w", err)
	}

	if resp.APIVersionResponse == nil {
		return nil, fmt.Errorf("create API version response missing data")
	}

	return resp.APIVersionResponse, nil
}

// API Publication methods

// ListAPIPublications returns all publications for an API
func (c *Client) ListAPIPublications(ctx context.Context, apiID string) ([]APIPublication, error) {
	if c.apiPublicationAPI == nil {
		return nil, fmt.Errorf("API publication client not configured")
	}

	var allPublications []APIPublication
	var pageNumber int64 = 1
	pageSize := int64(100)

	for {
		req := kkOps.ListAPIPublicationsRequest{
			Filter: &kkComps.APIPublicationFilterParameters{
				APIID: &kkComps.UUIDFieldFilter{
					Eq: &apiID,
				},
			},
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
		}

		resp, err := c.apiPublicationAPI.ListAPIPublications(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list API publications: %w", err)
		}

		if resp.ListAPIPublicationResponse == nil || len(resp.ListAPIPublicationResponse.Data) == 0 {
			break
		}

		for _, p := range resp.ListAPIPublicationResponse.Data {
			pub := APIPublication{
				ID:              "", // Publications don't have a separate ID
				PortalID:        p.PortalID,
				AuthStrategyIDs: p.AuthStrategyIds,
			}
			if p.Visibility != nil {
				pub.Visibility = string(*p.Visibility)
			}
			// AutoApproveRegistrations not available in list response
			allPublications = append(allPublications, pub)
		}

		pageNumber++

		// Check if we've fetched all pages
		if resp.ListAPIPublicationResponse.Meta.Page.Total <= float64(pageSize*(pageNumber-1)) {
			break
		}
	}

	return allPublications, nil
}

// CreateAPIPublication creates a new API publication
func (c *Client) CreateAPIPublication(
	ctx context.Context, apiID string, portalID string, publication kkComps.APIPublication,
) (*kkComps.APIPublicationResponse, error) {
	if c.apiPublicationAPI == nil {
		return nil, fmt.Errorf("API publication client not configured")
	}

	req := kkOps.PublishAPIToPortalRequest{
		APIID:          apiID,
		PortalID:       portalID,
		APIPublication: publication,
	}

	resp, err := c.apiPublicationAPI.PublishAPIToPortal(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create API publication: %w", err)
	}

	if resp.APIPublicationResponse == nil {
		return nil, fmt.Errorf("create API publication response missing data")
	}

	return resp.APIPublicationResponse, nil
}

// DeleteAPIPublication deletes an API publication
func (c *Client) DeleteAPIPublication(ctx context.Context, apiID, portalID string) error {
	if c.apiPublicationAPI == nil {
		return fmt.Errorf("API publication client not configured")
	}

	_, err := c.apiPublicationAPI.DeletePublication(ctx, apiID, portalID)
	if err != nil {
		return fmt.Errorf("failed to delete API publication: %w", err)
	}
	return nil
}

// API Implementation methods
// Note: Implementation operations are limited in the SDK

// ListAPIImplementations returns all implementations for an API
func (c *Client) ListAPIImplementations(ctx context.Context, apiID string) ([]APIImplementation, error) {
	if c.apiImplementationAPI == nil {
		return nil, fmt.Errorf("API implementation client not configured")
	}

	var allImplementations []APIImplementation
	var pageNumber int64 = 1
	pageSize := int64(100)

	for {
		req := kkOps.ListAPIImplementationsRequest{
			Filter: &kkComps.APIImplementationFilterParameters{
				APIID: &kkComps.UUIDFieldFilter{
					Eq: &apiID,
				},
			},
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
		}

		resp, err := c.apiImplementationAPI.ListAPIImplementations(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list API implementations: %w", err)
		}

		if resp.ListAPIImplementationsResponse == nil || len(resp.ListAPIImplementationsResponse.Data) == 0 {
			break
		}

		for _, i := range resp.ListAPIImplementationsResponse.Data {
			impl := APIImplementation{
				ID: i.ID,
			}
			// ImplementationURL not available in list response
			if i.Service != nil {
				impl.Service = &struct {
					ID             string
					ControlPlaneID string
				}{
					ID:             i.Service.ID,
					ControlPlaneID: i.Service.ControlPlaneID,
				}
			}
			allImplementations = append(allImplementations, impl)
		}

		pageNumber++

		// Check if we've fetched all pages
		if resp.ListAPIImplementationsResponse.Meta.Page.Total <= float64(pageSize*(pageNumber-1)) {
			break
		}
	}

	return allImplementations, nil
}

// CreateAPIImplementation creates a new API implementation
// Note: This is a placeholder - SDK doesn't support implementation creation yet
func (c *Client) CreateAPIImplementation(
	_ context.Context, _ string, _ interface{},
) (*kkComps.APIImplementationResponse, error) {
	return nil, fmt.Errorf("API implementation creation not yet supported by SDK")
}

// DeleteAPIImplementation deletes an API implementation
// Note: This is a placeholder - SDK doesn't support implementation deletion yet
func (c *Client) DeleteAPIImplementation(_ context.Context, _, _ string) error {
	return fmt.Errorf("API implementation deletion not yet supported by SDK")
}

// API Document methods

// ListAPIDocuments returns all documents for an API
func (c *Client) ListAPIDocuments(ctx context.Context, apiID string) ([]APIDocument, error) {
	if c.apiDocumentAPI == nil {
		return nil, fmt.Errorf("API document client not configured")
	}

	var allDocuments []APIDocument

	// API Documents don't support pagination in request
	resp, err := c.apiDocumentAPI.ListAPIDocuments(ctx, apiID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list API documents: %w", err)
	}

	if resp.ListAPIDocumentResponse == nil {
		return allDocuments, nil
	}

	// Convert summary documents to our internal type
	for _, d := range resp.ListAPIDocumentResponse.Data {
		doc := APIDocument{
			ID:    d.ID,
			Title: d.Title,
			Slug:  d.Slug,
			// Content not available in list response
		}
		if d.ParentDocumentID != nil {
			doc.ParentDocumentID = *d.ParentDocumentID
		}
		if d.Status != nil {
			doc.Status = string(*d.Status)
		}
		allDocuments = append(allDocuments, doc)

		// Recursively add children if any
		if len(d.Children) > 0 {
			c.addChildDocuments(&allDocuments, d.Children)
		}
	}

	return allDocuments, nil
}

// addChildDocuments recursively adds child documents to the list
func (c *Client) addChildDocuments(allDocuments *[]APIDocument, children []kkComps.APIDocumentSummaryWithChildren) {
	for _, child := range children {
		doc := APIDocument{
			ID:    child.ID,
			Title: child.Title,
			Slug:  child.Slug,
			// Content not available in list response
		}
		if child.ParentDocumentID != nil {
			doc.ParentDocumentID = *child.ParentDocumentID
		}
		if child.Status != nil {
			doc.Status = string(*child.Status)
		}
		*allDocuments = append(*allDocuments, doc)

		// Recursively add children
		if len(child.Children) > 0 {
			c.addChildDocuments(allDocuments, child.Children)
		}
	}
}

// CreateAPIDocument creates a new API document
func (c *Client) CreateAPIDocument(
	ctx context.Context, apiID string, document kkComps.CreateAPIDocumentRequest,
) (*kkComps.APIDocumentResponse, error) {
	if c.apiDocumentAPI == nil {
		return nil, fmt.Errorf("API document client not configured")
	}

	resp, err := c.apiDocumentAPI.CreateAPIDocument(ctx, apiID, document)
	if err != nil {
		return nil, fmt.Errorf("failed to create API document: %w", err)
	}

	if resp.APIDocumentResponse == nil {
		return nil, fmt.Errorf("create API document response missing data")
	}

	return resp.APIDocumentResponse, nil
}

// UpdateAPIDocument updates an existing API document
func (c *Client) UpdateAPIDocument(
	ctx context.Context, apiID, documentID string, document kkComps.APIDocument,
) (*kkComps.APIDocumentResponse, error) {
	if c.apiDocumentAPI == nil {
		return nil, fmt.Errorf("API document client not configured")
	}

	resp, err := c.apiDocumentAPI.UpdateAPIDocument(ctx, apiID, documentID, document)
	if err != nil {
		return nil, fmt.Errorf("failed to update API document: %w", err)
	}

	if resp.APIDocumentResponse == nil {
		return nil, fmt.Errorf("update API document response missing data")
	}

	return resp.APIDocumentResponse, nil
}

// DeleteAPIDocument deletes an API document
func (c *Client) DeleteAPIDocument(ctx context.Context, apiID, documentID string) error {
	if c.apiDocumentAPI == nil {
		return fmt.Errorf("API document client not configured")
	}

	_, err := c.apiDocumentAPI.DeleteAPIDocument(ctx, apiID, documentID)
	if err != nil {
		return fmt.Errorf("failed to delete API document: %w", err)
	}
	return nil
}

// GetAPIDocument retrieves a single API document with full content
func (c *Client) GetAPIDocument(ctx context.Context, apiID, documentID string) (*APIDocument, error) {
	if c.apiDocumentAPI == nil {
		return nil, fmt.Errorf("API document client not configured")
	}

	resp, err := c.apiDocumentAPI.FetchAPIDocument(ctx, apiID, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API document: %w", err)
	}

	if resp.APIDocumentResponse == nil {
		return nil, fmt.Errorf("fetch API document response missing data")
	}

	// Convert to our internal type with full content
	doc := &APIDocument{
		ID:      resp.APIDocumentResponse.ID,
		Content: resp.APIDocumentResponse.Content,
		Title:   resp.APIDocumentResponse.Title,
		Slug:    resp.APIDocumentResponse.Slug,
	}

	if resp.APIDocumentResponse.ParentDocumentID != nil {
		doc.ParentDocumentID = *resp.APIDocumentResponse.ParentDocumentID
	}

	if resp.APIDocumentResponse.Status != nil {
		doc.Status = string(*resp.APIDocumentResponse.Status)
	}

	return doc, nil
}

// CreateApplicationAuthStrategy creates a new application auth strategy with management labels
func (c *Client) CreateApplicationAuthStrategy(
	ctx context.Context,
	authStrategy kkComps.CreateAppAuthStrategyRequest,
) (*kkOps.CreateAppAuthStrategyResponse, error) {
	if c.appAuthAPI == nil {
		return nil, fmt.Errorf("app auth API client not configured")
	}

	// Add management labels to the appropriate request type
	switch {
	case authStrategy.AppAuthStrategyKeyAuthRequest != nil:
		// Convert map[string]string to map[string]*string for normalization
		pointerLabels := make(map[string]*string)
		for k, v := range authStrategy.AppAuthStrategyKeyAuthRequest.Labels {
			val := v
			pointerLabels[k] = &val
		}

		normalized := labels.NormalizeLabels(pointerLabels)
		normalized = labels.AddManagedLabels(normalized)

		// Convert back to map[string]string
		stringLabels := make(map[string]string)
		for k, v := range normalized {
			stringLabels[k] = v
		}
		authStrategy.AppAuthStrategyKeyAuthRequest.Labels = stringLabels

	case authStrategy.AppAuthStrategyOpenIDConnectRequest != nil:
		// Convert map[string]string to map[string]*string for normalization
		pointerLabels := make(map[string]*string)
		for k, v := range authStrategy.AppAuthStrategyOpenIDConnectRequest.Labels {
			val := v
			pointerLabels[k] = &val
		}

		normalized := labels.NormalizeLabels(pointerLabels)
		normalized = labels.AddManagedLabels(normalized)

		// Convert back to map[string]string
		stringLabels := make(map[string]string)
		for k, v := range normalized {
			stringLabels[k] = v
		}
		authStrategy.AppAuthStrategyOpenIDConnectRequest.Labels = stringLabels

	default:
		return nil, fmt.Errorf("unsupported auth strategy type")
	}

	resp, err := c.appAuthAPI.CreateAppAuthStrategy(ctx, authStrategy)
	if err != nil {
		return nil, fmt.Errorf("failed to create application auth strategy: %w", err)
	}

	return resp, nil
}

// ListManagedAuthStrategies returns all KONGCTL-managed auth strategies
func (c *Client) ListManagedAuthStrategies(ctx context.Context) ([]ApplicationAuthStrategy, error) {
	if c.appAuthAPI == nil {
		return nil, fmt.Errorf("app auth API client not configured")
	}

	var allStrategies []ApplicationAuthStrategy
	var pageNumber int64 = 1
	pageSize := int64(100)

	for {
		req := kkOps.ListAppAuthStrategiesRequest{
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
		}

		resp, err := c.appAuthAPI.ListAppAuthStrategies(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list application auth strategies: %w", err)
		}

		if resp.ListAppAuthStrategiesResponse == nil || len(resp.ListAppAuthStrategiesResponse.Data) == 0 {
			break
		}

		// Process and filter auth strategies
		for _, s := range resp.ListAppAuthStrategiesResponse.Data {
			// Extract common fields based on strategy type
			var strategy ApplicationAuthStrategy
			var labelMap map[string]string

			// The SDK returns AppAuthStrategy which is a union type
			// We need to check which type it is by checking the embedded fields
			if s.AppAuthStrategyKeyAuthResponseAppAuthStrategyKeyAuthResponse != nil {
				keyAuthResp := s.AppAuthStrategyKeyAuthResponseAppAuthStrategyKeyAuthResponse
				strategy.ID = keyAuthResp.ID
				strategy.Name = keyAuthResp.Name
				strategy.DisplayName = keyAuthResp.DisplayName
				strategy.StrategyType = "key_auth"

				// Extract configs
				configs := make(map[string]interface{})
				keyAuthConfig := make(map[string]interface{})
				if keyAuthResp.Configs.KeyAuth.KeyNames != nil {
					keyAuthConfig["key_names"] = keyAuthResp.Configs.KeyAuth.KeyNames
				}
				configs["key-auth"] = keyAuthConfig
				strategy.Configs = configs

				labelMap = keyAuthResp.Labels

			} else if s.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyOpenIDConnectResponse != nil {
				oidcResp := s.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyOpenIDConnectResponse
				strategy.ID = oidcResp.ID
				strategy.Name = oidcResp.Name
				strategy.DisplayName = oidcResp.DisplayName
				strategy.StrategyType = "openid_connect"

				// Extract configs
				configs := make(map[string]interface{})
				oidcConfig := make(map[string]interface{})
				oidcConfig["issuer"] = oidcResp.Configs.OpenidConnect.Issuer
				if oidcResp.Configs.OpenidConnect.CredentialClaim != nil {
					oidcConfig["credential_claim"] = oidcResp.Configs.OpenidConnect.CredentialClaim
				}
				if oidcResp.Configs.OpenidConnect.Scopes != nil {
					oidcConfig["scopes"] = oidcResp.Configs.OpenidConnect.Scopes
				}
				if oidcResp.Configs.OpenidConnect.AuthMethods != nil {
					oidcConfig["auth_methods"] = oidcResp.Configs.OpenidConnect.AuthMethods
				}
				configs["openid-connect"] = oidcConfig
				strategy.Configs = configs

				labelMap = oidcResp.Labels
			} else {
				// Unknown type, skip
				continue
			}

			// Normalize labels
			if labelMap == nil {
				labelMap = make(map[string]string)
			}
			strategy.NormalizedLabels = labelMap

			// Only include if managed by kongctl
			if labels.IsManagedResource(labelMap) {
				allStrategies = append(allStrategies, strategy)
			}
		}

		pageNumber++

		// Check if we've fetched all pages
		if resp.ListAppAuthStrategiesResponse.Meta.Page.Total <= float64(pageSize*(pageNumber-1)) {
			break
		}
	}

	return allStrategies, nil
}

// GetAuthStrategyByName finds a managed auth strategy by name
func (c *Client) GetAuthStrategyByName(ctx context.Context, name string) (*ApplicationAuthStrategy, error) {
	strategies, err := c.ListManagedAuthStrategies(ctx)
	if err != nil {
		return nil, err
	}

	for _, s := range strategies {
		if s.Name == name {
			return &s, nil
		}
	}

	return nil, nil // Not found
}

// GetAuthStrategyByFilter finds a managed auth strategy using a filter expression
func (c *Client) GetAuthStrategyByFilter(ctx context.Context, filter string) (*ApplicationAuthStrategy, error) {
	if c.appAuthAPI == nil {
		return nil, fmt.Errorf("application auth API client not configured")
	}

	// Use the filter in the SDK list operation
	// For now, we'll use ListManagedAuthStrategies and filter locally
	// TODO: Update when SDK supports server-side filtering
	strategies, err := c.ListManagedAuthStrategies(ctx)
	if err != nil {
		return nil, err
	}

	// Parse filter (e.g., "name[eq]=foo")
	if strings.HasPrefix(filter, "name[eq]=") {
		name := strings.TrimPrefix(filter, "name[eq]=")
		for _, s := range strategies {
			if s.Name == name {
				return &s, nil
			}
		}
	}

	return nil, nil // Not found
}

// UpdateApplicationAuthStrategy updates an existing auth strategy with new management labels
func (c *Client) UpdateApplicationAuthStrategy(
	ctx context.Context,
	id string,
	authStrategy kkComps.UpdateAppAuthStrategyRequest,
) (*kkOps.UpdateAppAuthStrategyResponse, error) {
	if c.appAuthAPI == nil {
		return nil, fmt.Errorf("app auth API client not configured")
	}

	// Add management labels directly to pointer map to preserve nil values
	// This allows label removal (nil values) to work correctly
	authStrategy.Labels = labels.AddManagedLabelsToPointerMap(authStrategy.Labels)

	resp, err := c.appAuthAPI.UpdateAppAuthStrategy(ctx, id, authStrategy)
	if err != nil {
		return nil, fmt.Errorf("failed to update application auth strategy: %w", err)
	}

	return resp, nil
}

// DeleteApplicationAuthStrategy deletes an auth strategy by ID
func (c *Client) DeleteApplicationAuthStrategy(ctx context.Context, id string) error {
	if c.appAuthAPI == nil {
		return fmt.Errorf("app auth API client not configured")
	}

	_, err := c.appAuthAPI.DeleteAppAuthStrategy(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete application auth strategy: %w", err)
	}
	return nil
}

// Portal Child Resource Methods

// UpdatePortalCustomization updates portal customization settings
func (c *Client) UpdatePortalCustomization(
	ctx context.Context,
	portalID string,
	customization kkComps.PortalCustomization,
) error {
	if c.portalCustomizationAPI == nil {
		return fmt.Errorf("portal customization API not configured")
	}

	_, err := c.portalCustomizationAPI.UpdatePortalCustomization(ctx, portalID, &customization)
	if err != nil {
		return fmt.Errorf("failed to update portal customization: %w", err)
	}
	return nil
}

// CreatePortalCustomDomain creates a custom domain for a portal
func (c *Client) CreatePortalCustomDomain(
	ctx context.Context,
	portalID string,
	req kkComps.CreatePortalCustomDomainRequest,
) error {
	if c.portalCustomDomainAPI == nil {
		return fmt.Errorf("portal custom domain API not configured")
	}

	_, err := c.portalCustomDomainAPI.CreatePortalCustomDomain(ctx, portalID, req)
	if err != nil {
		return fmt.Errorf("failed to create portal custom domain: %w", err)
	}
	return nil
}

// UpdatePortalCustomDomain updates a portal custom domain
func (c *Client) UpdatePortalCustomDomain(
	ctx context.Context,
	portalID string,
	req kkComps.UpdatePortalCustomDomainRequest,
) error {
	if c.portalCustomDomainAPI == nil {
		return fmt.Errorf("portal custom domain API not configured")
	}

	_, err := c.portalCustomDomainAPI.UpdatePortalCustomDomain(ctx, portalID, req)
	if err != nil {
		return fmt.Errorf("failed to update portal custom domain: %w", err)
	}
	return nil
}

// DeletePortalCustomDomain deletes a portal custom domain
func (c *Client) DeletePortalCustomDomain(ctx context.Context, portalID string) error {
	if c.portalCustomDomainAPI == nil {
		return fmt.Errorf("portal custom domain API not configured")
	}

	_, err := c.portalCustomDomainAPI.DeletePortalCustomDomain(ctx, portalID)
	if err != nil {
		return fmt.Errorf("failed to delete portal custom domain: %w", err)
	}
	return nil
}

// CreatePortalPage creates a new page in a portal
func (c *Client) CreatePortalPage(
	ctx context.Context,
	portalID string,
	req kkComps.CreatePortalPageRequest,
) (string, error) {
	if c.portalPageAPI == nil {
		return "", fmt.Errorf("portal page API not configured")
	}

	resp, err := c.portalPageAPI.CreatePortalPage(ctx, portalID, req)
	if err != nil {
		return "", fmt.Errorf("failed to create portal page: %w", err)
	}

	if resp.PortalPageResponse == nil {
		return "", fmt.Errorf("no response data from create portal page")
	}

	return resp.PortalPageResponse.ID, nil
}

// UpdatePortalPage updates an existing page in a portal
func (c *Client) UpdatePortalPage(
	ctx context.Context,
	portalID string,
	pageID string,
	req kkComps.UpdatePortalPageRequest,
) error {
	if c.portalPageAPI == nil {
		return fmt.Errorf("portal page API not configured")
	}

	updateReq := kkOps.UpdatePortalPageRequest{
		PortalID:                portalID,
		PageID:                  pageID,
		UpdatePortalPageRequest: req,
	}

	_, err := c.portalPageAPI.UpdatePortalPage(ctx, updateReq)
	if err != nil {
		return fmt.Errorf("failed to update portal page: %w", err)
	}
	return nil
}

// DeletePortalPage deletes a page from a portal
func (c *Client) DeletePortalPage(ctx context.Context, portalID string, pageID string) error {
	if c.portalPageAPI == nil {
		return fmt.Errorf("portal page API not configured")
	}

	_, err := c.portalPageAPI.DeletePortalPage(ctx, portalID, pageID)
	if err != nil {
		return fmt.Errorf("failed to delete portal page: %w", err)
	}
	return nil
}

