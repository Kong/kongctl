package state

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/errors"
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

// PortalPage represents a portal page for internal use
type PortalPage struct {
	ID               string
	Slug             string
	Title            string
	Content          string // Will be empty from list, populated from fetch
	Description      string
	Visibility       string
	Status           string
	ParentPageID     string
	NormalizedLabels map[string]string
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

// ListManagedPortals returns all KONGCTL-managed portals in the specified namespaces
// If namespaces is empty, no resources are returned (breaking change from previous behavior)
// To get all managed resources across all namespaces, pass []string{"*"}
func (c *Client) ListManagedPortals(ctx context.Context, namespaces []string) ([]Portal, error) {
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

			// Check if resource has a namespace label (new criteria for managed resources)
			if labels.IsManagedResource(normalized) {
				// Filter by namespace if specified
				if shouldIncludeNamespace(normalized[labels.NamespaceKey], namespaces) {
					portal := Portal{
						Portal:           p,
						NormalizedLabels: normalized,
					}
					allPortals = append(allPortals, portal)
				}
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
	// Search across all namespaces for backward compatibility
	portals, err := c.ListManagedPortals(ctx, []string{"*"})
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
	// Search across all namespaces for backward compatibility
	portals, err := c.ListManagedPortals(ctx, []string{"*"})
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
	namespace string,
) (*kkComps.PortalResponse, error) {
	// Get logger from context
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)

	logger.Debug("CreatePortal called",
		slog.Any("labels", portal.Labels),
		slog.String("namespace", namespace))

	// Labels have already been built by the executor using BuildCreateLabels
	// Just log for debugging
	if portal.Labels != nil {
		for k, v := range portal.Labels {
			if v != nil {
				logger.Debug("Final portal label",
					slog.String("key", k),
					slog.String("value", *v))
			} else {
				logger.Debug("Final portal label",
					slog.String("key", k),
					slog.String("value", "[nil]"))
			}
		}
	}

	resp, err := c.portalAPI.CreatePortal(ctx, portal)
	if err != nil {
		// Extract status code from error if possible
		statusCode := errors.ExtractStatusCodeFromError(err)
		
		// Create enhanced error with context and hints
		ctx := errors.APIErrorContext{
			ResourceType: "portal",
			ResourceName: portal.Name,
			Namespace:    namespace,
			Operation:    "create",
			StatusCode:   statusCode,
		}
		
		return nil, errors.EnhanceAPIError(err, ctx)
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
	_ string, // namespace - labels already built by executor
) (*kkComps.PortalResponse, error) {
	// Labels have already been built by the executor using BuildUpdateLabels
	// which includes namespace and protection labels with removal support

	resp, err := c.portalAPI.UpdatePortal(ctx, id, portal)
	if err != nil {
		// Extract status code from error if possible
		statusCode := errors.ExtractStatusCodeFromError(err)
		
		// Create enhanced error with context and hints
		ctx := errors.APIErrorContext{
			ResourceType: "portal",
			ResourceName: func() string {
				if portal.Name != nil {
					return *portal.Name
				}
				return ""
			}(), // May be nil for partial updates
			Operation:    "update",
			StatusCode:   statusCode,
		}
		
		return nil, errors.EnhanceAPIError(err, ctx)
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
		// Extract status code from error if possible
		statusCode := errors.ExtractStatusCodeFromError(err)
		
		// Create enhanced error with context and hints
		ctx := errors.APIErrorContext{
			ResourceType: "portal",
			ResourceName: id, // Using ID since we don't have name in delete context
			Operation:    "delete",
			StatusCode:   statusCode,
		}
		
		return errors.EnhanceAPIError(err, ctx)
	}
	return nil
}

// ListManagedAPIs returns all KONGCTL-managed APIs in the specified namespaces
// If namespaces is empty, no resources are returned (breaking change from previous behavior)
// To get all managed resources across all namespaces, pass []string{"*"}
func (c *Client) ListManagedAPIs(ctx context.Context, namespaces []string) ([]API, error) {
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

			// Check if resource has a namespace label (new criteria for managed resources)
			if labels.IsManagedResource(normalized) {
				// Filter by namespace if specified
				if shouldIncludeNamespace(normalized[labels.NamespaceKey], namespaces) {
					api := API{
						APIResponseSchema: a,
						NormalizedLabels:  normalized,
					}
					allAPIs = append(allAPIs, api)
				}
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
	// Search across all namespaces for backward compatibility
	apis, err := c.ListManagedAPIs(ctx, []string{"*"})
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
	// Search across all namespaces for backward compatibility
	apis, err := c.ListManagedAPIs(ctx, []string{"*"})
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
	// Search across all namespaces for backward compatibility
	apis, err := c.ListManagedAPIs(ctx, []string{"*"})
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
	namespace string,
) (*kkComps.APIResponseSchema, error) {
	if c.apiAPI == nil {
		return nil, fmt.Errorf("API client not configured")
	}

	// Get logger from context
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)

	logger.Debug("CreateAPI called",
		slog.Any("labels", api.Labels),
		slog.String("namespace", namespace))

	// Labels have already been built by the executor using BuildCreateLabels
	// Just log for debugging
	if api.Labels != nil {
		for k, v := range api.Labels {
			logger.Debug("Final API label",
				slog.String("key", k),
				slog.String("value", v))
		}
	}

	resp, err := c.apiAPI.CreateAPI(ctx, api)
	if err != nil {
		// Extract status code from error if possible
		statusCode := errors.ExtractStatusCodeFromError(err)
		
		// Create enhanced error with context and hints
		ctx := errors.APIErrorContext{
			ResourceType: "api",
			ResourceName: api.Name,
			Namespace:    namespace,
			Operation:    "create",
			StatusCode:   statusCode,
		}
		
		return nil, errors.EnhanceAPIError(err, ctx)
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
	_ string, // namespace - labels already built by executor
) (*kkComps.APIResponseSchema, error) {
	if c.apiAPI == nil {
		return nil, fmt.Errorf("API client not configured")
	}

	// Labels have already been built by the executor using BuildUpdateLabels
	// which includes namespace and protection labels with removal support

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

// DeleteAPIVersion deletes an API version
func (c *Client) DeleteAPIVersion(ctx context.Context, apiID string, versionID string) error {
	if c.apiVersionAPI == nil {
		return fmt.Errorf("API version client not configured")
	}

	_, err := c.apiVersionAPI.DeleteAPIVersion(ctx, apiID, versionID)
	if err != nil {
		return fmt.Errorf("failed to delete API version: %w", err)
	}

	return nil
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
	_ string, // namespace - labels already built by executor
) (*kkOps.CreateAppAuthStrategyResponse, error) {
	if c.appAuthAPI == nil {
		return nil, fmt.Errorf("app auth API client not configured")
	}

	// Labels have already been built by the executor using BuildCreateLabels
	// Just pass through to the API

	resp, err := c.appAuthAPI.CreateAppAuthStrategy(ctx, authStrategy)
	if err != nil {
		return nil, fmt.Errorf("failed to create application auth strategy: %w", err)
	}

	return resp, nil
}

// ListManagedAuthStrategies returns all KONGCTL-managed auth strategies in the specified namespaces
// If namespaces is empty, no resources are returned (breaking change from previous behavior)
// To get all managed resources across all namespaces, pass []string{"*"}
func (c *Client) ListManagedAuthStrategies(
	ctx context.Context, namespaces []string,
) ([]ApplicationAuthStrategy, error) {
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

			// Check if resource has a namespace label (new criteria for managed resources)
			if labels.IsManagedResource(labelMap) {
				// Filter by namespace if specified
				if shouldIncludeNamespace(labelMap[labels.NamespaceKey], namespaces) {
					allStrategies = append(allStrategies, strategy)
				}
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
	// Search across all namespaces for backward compatibility
	strategies, err := c.ListManagedAuthStrategies(ctx, []string{"*"})
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
	// Search across all namespaces for backward compatibility
	strategies, err := c.ListManagedAuthStrategies(ctx, []string{"*"})
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
	_ string, // namespace - labels already built by executor
) (*kkOps.UpdateAppAuthStrategyResponse, error) {
	if c.appAuthAPI == nil {
		return nil, fmt.Errorf("app auth API client not configured")
	}

	// Labels have already been built by the executor using BuildUpdateLabels
	// which includes namespace and protection labels with removal support

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

// GetPortalCustomization fetches the current customization for a portal
func (c *Client) GetPortalCustomization(
	ctx context.Context,
	portalID string,
) (*kkComps.PortalCustomization, error) {
	if c.portalCustomizationAPI == nil {
		return nil, fmt.Errorf("portal customization API not configured")
	}
	
	resp, err := c.portalCustomizationAPI.GetPortalCustomization(ctx, portalID)
	if err != nil {
		return nil, fmt.Errorf("failed to get portal customization: %w", err)
	}
	
	if resp.PortalCustomization == nil {
		return nil, fmt.Errorf("no customization data in response")
	}
	
	return resp.PortalCustomization, nil
}

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

// ListManagedPortalPages returns all KONGCTL-managed portal pages for a portal
func (c *Client) ListManagedPortalPages(ctx context.Context, portalID string) ([]PortalPage, error) {
	if c.portalPageAPI == nil {
		return nil, fmt.Errorf("portal page API not configured")
	}

	var allPages []PortalPage

	// List all pages for the portal (without pagination for now - portal pages typically don't have many entries)
	req := kkOps.ListPortalPagesRequest{
		PortalID: portalID,
	}

	resp, err := c.portalPageAPI.ListPortalPages(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list portal pages: %w", err)
	}

	if resp.ListPortalPagesResponse == nil {
		return allPages, nil
	}

	// Process pages recursively to build flat list
	c.processPortalPages(&allPages, resp.ListPortalPagesResponse.Data, "")

	// Note: Portal pages don't have labels in the SDK, so we can't filter for managed pages
	// For now, return all pages and let the planner handle matching
	return allPages, nil
}

// processPortalPages recursively processes portal pages and their children
func (c *Client) processPortalPages(allPages *[]PortalPage, pages []kkComps.PortalPageInfo, parentID string) {
	for _, p := range pages {
		page := PortalPage{
			ID:           p.ID,
			Slug:         p.Slug,
			Title:        p.Title,
			// Content not available in list response
			Visibility:   string(p.Visibility),
			Status:       string(p.Status),
			ParentPageID: parentID,
		}

		// Normalize description
		if p.Description != nil {
			page.Description = *p.Description
		}

		// Note: Labels are not available in list response for portal pages
		// We'll need to fetch individual pages to get labels for filtering
		page.NormalizedLabels = make(map[string]string)

		*allPages = append(*allPages, page)

		// Recursively process children
		if len(p.Children) > 0 {
			c.processPortalPages(allPages, p.Children, p.ID)
		}
	}
}

// GetPortalPage fetches a single portal page with full details including content
func (c *Client) GetPortalPage(ctx context.Context, portalID string, pageID string) (*PortalPage, error) {
	if c.portalPageAPI == nil {
		return nil, fmt.Errorf("portal page API not configured")
	}

	resp, err := c.portalPageAPI.GetPortalPage(ctx, portalID, pageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get portal page: %w", err)
	}

	if resp.PortalPageResponse == nil {
		return nil, fmt.Errorf("no response data from get portal page")
	}

	pageResp := resp.PortalPageResponse
	page := &PortalPage{
		ID:         pageResp.ID,
		Slug:       pageResp.Slug,
		Title:      pageResp.Title,
		Content:    pageResp.Content,
		Visibility: string(pageResp.Visibility),
		Status:     string(pageResp.Status),
	}
	
	// Handle nullable parent page ID
	if pageResp.ParentPageID != nil {
		page.ParentPageID = *pageResp.ParentPageID
	}

	// Normalize description
	if pageResp.Description != nil {
		page.Description = *pageResp.Description
	}

	// Note: Portal pages don't have labels in the SDK response
	// We'll track managed status through a different mechanism if needed
	page.NormalizedLabels = make(map[string]string)

	return page, nil
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

// Portal Snippet Methods

// ListPortalSnippets returns all snippets for a portal
func (c *Client) ListPortalSnippets(ctx context.Context, portalID string) ([]PortalSnippet, error) {
	if c.portalSnippetAPI == nil {
		return nil, fmt.Errorf("portal snippet API not configured")
	}

	var allSnippets []PortalSnippet
	var pageNumber int64 = 1
	pageSize := int64(100)

	for {
		req := kkOps.ListPortalSnippetsRequest{
			PortalID:   portalID,
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
		}

		resp, err := c.portalSnippetAPI.ListPortalSnippets(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list portal snippets: %w", err)
		}

		if resp.ListPortalSnippetsResponse == nil || len(resp.ListPortalSnippetsResponse.Data) == 0 {
			break
		}

		// Process snippets
		for _, s := range resp.ListPortalSnippetsResponse.Data {
			snippet := PortalSnippet{
				ID:         s.ID,
				Name:       s.Name,
				Visibility: string(s.Visibility),
				Status:     string(s.Status),
			}

			// Title is always present (not a pointer)
			snippet.Title = s.Title
			
			// Handle optional fields
			if s.Description != nil {
				snippet.Description = *s.Description
			}

			// Note: Content not available in list response
			// Note: Labels not available for portal snippets
			snippet.NormalizedLabels = make(map[string]string)

			allSnippets = append(allSnippets, snippet)
		}

		pageNumber++

		// Check if we've fetched all pages
		if resp.ListPortalSnippetsResponse.Meta.Page.Total <= float64(pageSize*(pageNumber-1)) {
			break
		}
	}

	return allSnippets, nil
}

// GetPortalSnippet fetches a single portal snippet with full details including content
func (c *Client) GetPortalSnippet(ctx context.Context, portalID string, snippetID string) (*PortalSnippet, error) {
	if c.portalSnippetAPI == nil {
		return nil, fmt.Errorf("portal snippet API not configured")
	}

	resp, err := c.portalSnippetAPI.GetPortalSnippet(ctx, portalID, snippetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get portal snippet: %w", err)
	}

	if resp.PortalSnippetResponse == nil {
		return nil, fmt.Errorf("no response data from get portal snippet")
	}

	snippetResp := resp.PortalSnippetResponse
	snippet := &PortalSnippet{
		ID:         snippetResp.ID,
		Name:       snippetResp.Name,
		Content:    snippetResp.Content,
		Visibility: string(snippetResp.Visibility),
		Status:     string(snippetResp.Status),
	}

	// Handle optional fields
	if snippetResp.Title != nil {
		snippet.Title = *snippetResp.Title
	}
	if snippetResp.Description != nil {
		snippet.Description = *snippetResp.Description
	}

	// Note: Portal snippets don't have labels in the SDK response
	snippet.NormalizedLabels = make(map[string]string)

	return snippet, nil
}

// CreatePortalSnippet creates a new snippet in a portal
func (c *Client) CreatePortalSnippet(
	ctx context.Context,
	portalID string,
	req kkComps.CreatePortalSnippetRequest,
) (string, error) {
	if c.portalSnippetAPI == nil {
		return "", fmt.Errorf("portal snippet API not configured")
	}

	resp, err := c.portalSnippetAPI.CreatePortalSnippet(ctx, portalID, req)
	if err != nil {
		return "", fmt.Errorf("failed to create portal snippet: %w", err)
	}

	if resp.PortalSnippetResponse == nil {
		return "", fmt.Errorf("no response data from create portal snippet")
	}

	return resp.PortalSnippetResponse.ID, nil
}

// UpdatePortalSnippet updates an existing snippet in a portal
func (c *Client) UpdatePortalSnippet(
	ctx context.Context,
	portalID string,
	snippetID string,
	req kkComps.UpdatePortalSnippetRequest,
) error {
	if c.portalSnippetAPI == nil {
		return fmt.Errorf("portal snippet API not configured")
	}

	updateReq := kkOps.UpdatePortalSnippetRequest{
		PortalID:                   portalID,
		SnippetID:                  snippetID,
		UpdatePortalSnippetRequest: req,
	}

	_, err := c.portalSnippetAPI.UpdatePortalSnippet(ctx, updateReq)
	if err != nil {
		return fmt.Errorf("failed to update portal snippet: %w", err)
	}
	return nil
}

// DeletePortalSnippet deletes a snippet from a portal
func (c *Client) DeletePortalSnippet(ctx context.Context, portalID string, snippetID string) error {
	if c.portalSnippetAPI == nil {
		return fmt.Errorf("portal snippet API not configured")
	}

	_, err := c.portalSnippetAPI.DeletePortalSnippet(ctx, portalID, snippetID)
	if err != nil {
		return fmt.Errorf("failed to delete portal snippet: %w", err)
	}
	return nil
}

// shouldIncludeNamespace checks if a resource's namespace should be included based on filter
func shouldIncludeNamespace(resourceNamespace string, namespaces []string) bool {
	// Empty namespace list means no resources should be returned
	if len(namespaces) == 0 {
		return false
	}
	
	// Check for wildcard (all namespaces)
	for _, ns := range namespaces {
		if ns == "*" {
			return true
		}
	}
	
	// Check if resource's namespace is in the filter list
	for _, ns := range namespaces {
		if resourceNamespace == ns {
			return true
		}
	}
	
	return false
}

