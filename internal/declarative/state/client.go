package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"slices"
	"strings"

	// kk = Kong Konnect
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	kkErrors "github.com/Kong/sdk-konnect-go/models/sdkerrors"
	decerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/log"
	"github.com/kong/kongctl/internal/util/pagination"
)

// ClientConfig contains all the API interfaces needed by the state client
type ClientConfig struct {
	// Core APIs
	PortalAPI             helpers.PortalAPI
	APIAPI                helpers.APIAPI
	AppAuthAPI            helpers.AppAuthStrategiesAPI
	DCRProviderAPI        helpers.DCRProvidersAPI
	ControlPlaneAPI       helpers.ControlPlaneAPI
	GatewayServiceAPI     helpers.GatewayServiceAPI
	ControlPlaneGroupsAPI helpers.ControlPlaneGroupsAPI
	CatalogServiceAPI     helpers.CatalogServicesAPI

	// Portal child resource APIs
	PortalPageAPI             helpers.PortalPageAPI
	PortalAuthSettingsAPI     helpers.PortalAuthSettingsAPI
	PortalIdentityProviderAPI helpers.PortalIdentityProviderAPI
	PortalCustomizationAPI    helpers.PortalCustomizationAPI
	PortalCustomDomainAPI     helpers.PortalCustomDomainAPI
	PortalSnippetAPI          helpers.PortalSnippetAPI
	PortalTeamAPI             helpers.PortalTeamAPI
	PortalTeamRolesAPI        helpers.PortalTeamRolesAPI
	PortalEmailsAPI           helpers.PortalEmailsAPI
	AssetsAPI                 helpers.AssetsAPI

	// API child resource APIs
	APIVersionAPI        helpers.APIVersionAPI
	APIPublicationAPI    helpers.APIPublicationAPI
	APIImplementationAPI helpers.APIImplementationAPI
	APIDocumentAPI       helpers.APIDocumentAPI

	// Event Gateway APIs
	EGWControlPlaneAPI                  helpers.EGWControlPlaneAPI
	EventGatewayBackendClusterAPI       helpers.EventGatewayBackendClusterAPI
	EventGatewayVirtualClusterAPI       helpers.EventGatewayVirtualClusterAPI
	EventGatewayListenerAPI             helpers.EventGatewayListenerAPI
	EventGatewayListenerPolicyAPI       helpers.EventGatewayListenerPolicyAPI
	EventGatewayClusterPolicyAPI        helpers.EventGatewayClusterPolicyAPI
	EventGatewayProducePolicyAPI        helpers.EventGatewayProducePolicyAPI
	EventGatewayConsumePolicyAPI        helpers.EventGatewayConsumePolicyAPI
	EventGatewayDataPlaneCertificateAPI helpers.EventGatewayDataPlaneCertificateAPI
	EventGatewaySchemaRegistryAPI       helpers.EventGatewaySchemaRegistryAPI
	EventGatewayStaticKeyAPI            helpers.EventGatewayStaticKeyAPI
	EventGatewayTLSTrustBundleAPI       helpers.EventGatewayTLSTrustBundleAPI

	// Identity resources
	OrganizationTeamAPI helpers.OrganizationTeamAPI
}

// Client wraps Konnect SDK for state management
type Client struct {
	// Core APIs
	portalAPI             helpers.PortalAPI
	apiAPI                helpers.APIAPI
	appAuthAPI            helpers.AppAuthStrategiesAPI
	dcrProviderAPI        helpers.DCRProvidersAPI
	controlPlaneAPI       helpers.ControlPlaneAPI
	gatewayServiceAPI     helpers.GatewayServiceAPI
	controlPlaneGroupsAPI helpers.ControlPlaneGroupsAPI
	catalogServiceAPI     helpers.CatalogServicesAPI

	// Portal child resource APIs
	portalPageAPI             helpers.PortalPageAPI
	portalAuthSettingsAPI     helpers.PortalAuthSettingsAPI
	portalIdentityProviderAPI helpers.PortalIdentityProviderAPI
	portalCustomizationAPI    helpers.PortalCustomizationAPI
	portalCustomDomainAPI     helpers.PortalCustomDomainAPI
	portalSnippetAPI          helpers.PortalSnippetAPI
	portalTeamAPI             helpers.PortalTeamAPI
	portalTeamRolesAPI        helpers.PortalTeamRolesAPI
	portalEmailsAPI           helpers.PortalEmailsAPI
	assetsAPI                 helpers.AssetsAPI

	// API child resource APIs
	apiVersionAPI        helpers.APIVersionAPI
	apiPublicationAPI    helpers.APIPublicationAPI
	apiImplementationAPI helpers.APIImplementationAPI
	apiDocumentAPI       helpers.APIDocumentAPI

	// Event Gateway APIs
	egwControlPlaneAPI                  helpers.EGWControlPlaneAPI
	eventGatewayBackendClusterAPI       helpers.EventGatewayBackendClusterAPI
	eventGatewayVirtualClusterAPI       helpers.EventGatewayVirtualClusterAPI
	eventGatewayListenerAPI             helpers.EventGatewayListenerAPI
	eventGatewayListenerPolicyAPI       helpers.EventGatewayListenerPolicyAPI
	eventGatewayClusterPolicyAPI        helpers.EventGatewayClusterPolicyAPI
	eventGatewayProducePolicyAPI        helpers.EventGatewayProducePolicyAPI
	eventGatewayConsumePolicyAPI        helpers.EventGatewayConsumePolicyAPI
	eventGatewayDataPlaneCertificateAPI helpers.EventGatewayDataPlaneCertificateAPI
	eventGatewaySchemaRegistryAPI       helpers.EventGatewaySchemaRegistryAPI
	eventGatewayStaticKeyAPI            helpers.EventGatewayStaticKeyAPI
	eventGatewayTLSTrustBundleAPI       helpers.EventGatewayTLSTrustBundleAPI

	// Organization resource APIs
	organizationTeamAPI helpers.OrganizationTeamAPI
}

// NewClient creates a new state client with the provided configuration
func NewClient(config ClientConfig) *Client {
	return &Client{
		// Core APIs
		portalAPI:             config.PortalAPI,
		apiAPI:                config.APIAPI,
		appAuthAPI:            config.AppAuthAPI,
		dcrProviderAPI:        config.DCRProviderAPI,
		controlPlaneAPI:       config.ControlPlaneAPI,
		gatewayServiceAPI:     config.GatewayServiceAPI,
		controlPlaneGroupsAPI: config.ControlPlaneGroupsAPI,
		catalogServiceAPI:     config.CatalogServiceAPI,

		// Portal child resource APIs
		portalPageAPI:             config.PortalPageAPI,
		portalAuthSettingsAPI:     config.PortalAuthSettingsAPI,
		portalIdentityProviderAPI: config.PortalIdentityProviderAPI,
		portalCustomizationAPI:    config.PortalCustomizationAPI,
		portalCustomDomainAPI:     config.PortalCustomDomainAPI,
		portalSnippetAPI:          config.PortalSnippetAPI,
		portalTeamAPI:             config.PortalTeamAPI,
		portalTeamRolesAPI:        config.PortalTeamRolesAPI,
		portalEmailsAPI:           config.PortalEmailsAPI,
		assetsAPI:                 config.AssetsAPI,

		// API child resource APIs
		apiVersionAPI:        config.APIVersionAPI,
		apiPublicationAPI:    config.APIPublicationAPI,
		apiImplementationAPI: config.APIImplementationAPI,
		apiDocumentAPI:       config.APIDocumentAPI,

		// Event Gateway APIs
		egwControlPlaneAPI:                  config.EGWControlPlaneAPI,
		eventGatewayBackendClusterAPI:       config.EventGatewayBackendClusterAPI,
		eventGatewayVirtualClusterAPI:       config.EventGatewayVirtualClusterAPI,
		eventGatewayListenerAPI:             config.EventGatewayListenerAPI,
		eventGatewayListenerPolicyAPI:       config.EventGatewayListenerPolicyAPI,
		eventGatewayClusterPolicyAPI:        config.EventGatewayClusterPolicyAPI,
		eventGatewayProducePolicyAPI:        config.EventGatewayProducePolicyAPI,
		eventGatewayConsumePolicyAPI:        config.EventGatewayConsumePolicyAPI,
		eventGatewayDataPlaneCertificateAPI: config.EventGatewayDataPlaneCertificateAPI,
		eventGatewaySchemaRegistryAPI:       config.EventGatewaySchemaRegistryAPI,
		eventGatewayStaticKeyAPI:            config.EventGatewayStaticKeyAPI,
		eventGatewayTLSTrustBundleAPI:       config.EventGatewayTLSTrustBundleAPI,

		// Identity resource APIs
		organizationTeamAPI: config.OrganizationTeamAPI,
	}
}

// Portal represents a normalized portal for internal use
type Portal struct {
	kkComps.ListPortalsResponsePortal
	NormalizedLabels map[string]string // Non-pointer labels
}

// API represents a normalized API for internal use
type API struct {
	kkComps.APIResponseSchema
	NormalizedLabels map[string]string // Non-pointer labels
}

// ControlPlane represents a normalized control plane for internal use
type ControlPlane struct {
	kkComps.ControlPlane
	NormalizedLabels map[string]string // Non-pointer labels
	GroupMembers     []string
}

// GatewayService represents a gateway service for internal use.
type GatewayService struct {
	ID             string
	Name           string
	ControlPlaneID string
	Service        kkComps.ServiceOutput
}

// CatalogService represents a catalog service for internal use.
type CatalogService struct {
	kkComps.CatalogService
	NormalizedLabels map[string]string
}

// APIVersion represents an API version for internal use
type APIVersion struct {
	ID            string
	Version       string
	PublishStatus string
	Deprecated    bool
	SunsetDate    string
	Spec          string // API version spec content for content comparison
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

// PortalEmailTemplate represents a customized email template for a portal.
type PortalEmailTemplate struct {
	ID        string
	Name      string
	Label     string
	Enabled   bool
	Content   *PortalEmailTemplateContent
	Variables []kkComps.EmailTemplateVariableName
}

// PortalEmailTemplateContent captures the mutable email content fields.
type PortalEmailTemplateContent struct {
	Subject     *string
	Title       *string
	Body        *string
	ButtonLabel *string
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
	DCRProviderID    string
	DCRProviderName  string
	Configs          map[string]any
	NormalizedLabels map[string]string // Non-pointer labels
}

// DCRProvider represents a normalized DCR provider for internal use.
type DCRProvider struct {
	ID               string
	Name             string
	DisplayName      string
	DisplayNameSet   bool
	ProviderType     string
	Issuer           string
	DCRConfig        map[string]any
	NormalizedLabels map[string]string
}

type EventGatewayControlPlane struct {
	kkComps.EventGatewayInfo
	NormalizedLabels map[string]string // Non-pointer labels
}

type EventGatewayBackendCluster struct {
	kkComps.BackendCluster
	NormalizedLabels map[string]string // Non-pointer labels
}

type EventGatewayVirtualCluster struct {
	kkComps.VirtualCluster
	NormalizedLabels map[string]string // Non-pointer labels
}

// Team represents a normalized team for internal use
// I think this should be OrganizationTeam
type OrganizationTeam struct {
	kkComps.Team
	NormalizedLabels map[string]string // Non-pointer labels
}

type EventGatewayListener struct {
	kkComps.EventGatewayListener
	NormalizedLabels map[string]string // Non-pointer labels
}

// EventGatewayDataPlaneCertificate represents a data plane certificate for internal use
type EventGatewayDataPlaneCertificate struct {
	kkComps.EventGatewayDataPlaneCertificate
}

// EventGatewaySchemaRegistry represents a schema registry for internal use
type EventGatewaySchemaRegistry struct {
	kkComps.SchemaRegistry
	NormalizedLabels map[string]string // Non-pointer labels
	RawConfig        map[string]any    // Full config from raw API response
}

// ListManagedPortals returns all KONGCTL-managed portals in the specified namespaces
// If namespaces is empty, no resources are returned (breaking change from previous behavior)
// To get all managed resources across all namespaces, pass []string{"*"}
func (c *Client) ListManagedPortals(ctx context.Context, namespaces []string) ([]Portal, error) {
	// Validate API client
	if err := ValidateAPIClient(c.portalAPI, "Portal API"); err != nil {
		return nil, err
	}

	// Create paginated lister function
	lister := func(ctx context.Context, pageSize, pageNumber int64) ([]Portal, *PageMeta, error) {
		req := kkOps.ListPortalsRequest{
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
		}

		resp, err := c.portalAPI.ListPortals(ctx, req)
		if err != nil {
			return nil, nil, WrapAPIError(err, "list portals", nil)
		}

		if resp.ListPortalsResponse == nil {
			return []Portal{}, &PageMeta{Total: 0}, nil
		}

		var filteredPortals []Portal

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
						ListPortalsResponsePortal: p,
						NormalizedLabels:          normalized,
					}
					filteredPortals = append(filteredPortals, portal)
				}
			}
		}

		// Extract pagination metadata
		meta := &PageMeta{Total: resp.ListPortalsResponse.Meta.Page.Total}

		return filteredPortals, meta, nil
	}

	return PaginateAll(ctx, lister)
}

// ListAllPortals returns all portals, including non-managed ones
func (c *Client) ListAllPortals(ctx context.Context) ([]Portal, error) {
	// Validate API client
	if err := ValidateAPIClient(c.portalAPI, "Portal API"); err != nil {
		return nil, err
	}

	// Create paginated lister function
	lister := func(ctx context.Context, pageSize, pageNumber int64) ([]Portal, *PageMeta, error) {
		req := kkOps.ListPortalsRequest{
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
			// No labels filter - get ALL portals
		}

		resp, err := c.portalAPI.ListPortals(ctx, req)
		if err != nil {
			return nil, nil, WrapAPIError(err, "list all portals", nil)
		}

		if resp.ListPortalsResponse == nil {
			return []Portal{}, &PageMeta{Total: 0}, nil
		}

		var allPortals []Portal

		// Process all portals without filtering
		for _, p := range resp.ListPortalsResponse.Data {
			// Labels are already map[string]string in the SDK
			normalized := p.Labels
			if normalized == nil {
				normalized = make(map[string]string)
			}

			portal := Portal{
				ListPortalsResponsePortal: p,
				NormalizedLabels:          normalized,
			}
			allPortals = append(allPortals, portal)
		}

		// Extract pagination metadata
		meta := &PageMeta{Total: resp.ListPortalsResponse.Meta.Page.Total}

		return allPortals, meta, nil
	}

	return PaginateAll(ctx, lister)
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
	if after, ok := strings.CutPrefix(filter, "name[eq]="); ok {
		name := after
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
		return nil, WrapAPIError(err, "create portal", &ErrorWrapperOptions{
			ResourceType: "portal",
			ResourceName: portal.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if err := ValidateResponse(resp.PortalResponse, "create portal"); err != nil {
		return nil, err
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
		statusCode := decerrors.ExtractStatusCodeFromError(err)

		// Create enhanced error with context and hints
		ctx := decerrors.APIErrorContext{
			ResourceType: "portal",
			ResourceName: func() string {
				if portal.Name != nil {
					return *portal.Name
				}
				return ""
			}(), // May be nil for partial updates
			Operation:  "update",
			StatusCode: statusCode,
		}

		return nil, decerrors.EnhanceAPIError(err, ctx)
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
		statusCode := decerrors.ExtractStatusCodeFromError(err)

		// Create enhanced error with context and hints
		ctx := decerrors.APIErrorContext{
			ResourceType: "portal",
			ResourceName: id, // Using ID since we don't have name in delete context
			Operation:    "delete",
			StatusCode:   statusCode,
		}

		return decerrors.EnhanceAPIError(err, ctx)
	}
	return nil
}

// ListManagedControlPlanes returns all KONGCTL-managed control planes in the specified namespaces
// If namespaces is empty, no resources are returned (breaking change from previous behavior)
// To get all managed resources across all namespaces, pass []string{"*"}
func (c *Client) ListManagedControlPlanes(ctx context.Context, namespaces []string) ([]ControlPlane, error) {
	// Validate API client
	if err := ValidateAPIClient(c.controlPlaneAPI, "Control Plane API"); err != nil {
		return nil, err
	}

	// Create paginated lister function
	lister := func(ctx context.Context, pageSize, pageNumber int64) ([]ControlPlane, *PageMeta, error) {
		req := kkOps.ListControlPlanesRequest{
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
		}

		resp, err := c.controlPlaneAPI.ListControlPlanes(ctx, req)
		if err != nil {
			return nil, nil, WrapAPIError(err, "list control planes", nil)
		}

		if resp.ListControlPlanesResponse == nil {
			return []ControlPlane{}, &PageMeta{Total: 0}, nil
		}

		var filtered []ControlPlane

		for _, cp := range resp.ListControlPlanesResponse.Data {
			normalized := cp.Labels
			if normalized == nil {
				normalized = make(map[string]string)
			}

			if labels.IsManagedResource(normalized) &&
				shouldIncludeNamespace(normalized[labels.NamespaceKey], namespaces) {
				filtered = append(filtered, ControlPlane{
					ControlPlane:     cp,
					NormalizedLabels: normalized,
				})
			}
		}

		meta := &PageMeta{Total: resp.ListControlPlanesResponse.Meta.Page.Total}

		return filtered, meta, nil
	}

	return PaginateAll(ctx, lister)
}

// ListAllControlPlanes returns all control planes, including non-managed ones
func (c *Client) ListAllControlPlanes(ctx context.Context) ([]ControlPlane, error) {
	if err := ValidateAPIClient(c.controlPlaneAPI, "Control Plane API"); err != nil {
		return nil, err
	}

	var (
		pageNumber int64 = 1
		pageSize   int64 = 100
	)

	var all []ControlPlane

	for {
		req := kkOps.ListControlPlanesRequest{
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
		}

		resp, err := c.controlPlaneAPI.ListControlPlanes(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list control planes: %w", err)
		}

		if resp.ListControlPlanesResponse == nil || len(resp.ListControlPlanesResponse.Data) == 0 {
			break
		}

		for _, cp := range resp.ListControlPlanesResponse.Data {
			normalized := cp.Labels
			if normalized == nil {
				normalized = make(map[string]string)
			}

			all = append(all, ControlPlane{
				ControlPlane:     cp,
				NormalizedLabels: normalized,
			})
		}

		if resp.ListControlPlanesResponse.Meta.Page.Total <= float64(pageNumber*pageSize) {
			break
		}

		pageNumber++
	}

	return all, nil
}

// ListControlPlaneGroupMemberships returns all child control plane IDs for a control plane group.
func (c *Client) ListControlPlaneGroupMemberships(ctx context.Context, groupID string) ([]string, error) {
	if err := ValidateAPIClient(c.controlPlaneGroupsAPI, "Control Plane Groups API"); err != nil {
		return nil, err
	}

	const defaultPageSize int64 = 100
	pageSize := defaultPageSize

	var (
		memberIDs []string
		pageAfter *string
	)

	for {
		req := kkOps.GetControlPlanesIDGroupMembershipsRequest{
			ID:       groupID,
			PageSize: &pageSize,
		}

		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		resp, err := c.controlPlaneGroupsAPI.GetControlPlanesIDGroupMemberships(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list control plane group memberships", &ErrorWrapperOptions{
				ResourceType: "control_plane_group",
				ResourceName: groupID,
				UseEnhanced:  true,
			})
		}

		if resp == nil || resp.GetListGroupMemberships() == nil {
			break
		}

		for _, member := range resp.GetListGroupMemberships().GetData() {
			if member.ID != "" {
				memberIDs = append(memberIDs, member.ID)
			}
		}

		meta := resp.GetListGroupMemberships().GetMeta()
		nextCursor := pagination.ExtractPageAfterCursor(meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return memberIDs, nil
}

// UpsertControlPlaneGroupMemberships replaces the members of a control plane group.
func (c *Client) UpsertControlPlaneGroupMemberships(ctx context.Context, groupID string, memberIDs []string) error {
	if err := ValidateAPIClient(c.controlPlaneGroupsAPI, "Control Plane Groups API"); err != nil {
		return err
	}

	members := make([]kkComps.Members, 0, len(memberIDs))
	for _, id := range memberIDs {
		if strings.TrimSpace(id) == "" {
			continue
		}
		members = append(members, kkComps.Members{ID: id})
	}

	req := kkComps.GroupMembership{
		Members: members,
	}

	if _, err := c.controlPlaneGroupsAPI.PutControlPlanesIDGroupMemberships(ctx, groupID, &req); err != nil {
		return WrapAPIError(err, "upsert control plane group memberships", &ErrorWrapperOptions{
			ResourceType: "control_plane_group",
			ResourceName: groupID,
			UseEnhanced:  true,
		})
	}

	return nil
}

// ListGatewayServices returns all gateway services for the provided control plane.
func (c *Client) ListGatewayServices(ctx context.Context, controlPlaneID string) ([]GatewayService, error) {
	if err := ValidateAPIClient(c.gatewayServiceAPI, "Gateway Service API"); err != nil {
		return nil, err
	}

	const defaultPageSize int64 = 100
	pageSize := defaultPageSize
	var (
		services  []GatewayService
		hasOffset bool
		offsetVal string
	)

	for {
		req := kkOps.ListServiceRequest{
			ControlPlaneID: controlPlaneID,
			Size:           &pageSize,
		}

		if hasOffset {
			req.Offset = &offsetVal
		}

		resp, err := c.gatewayServiceAPI.ListService(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list gateway services: %w", err)
		}

		if resp == nil || resp.Object == nil {
			break
		}

		for _, svc := range resp.Object.Data {
			id := ""
			if svc.ID != nil {
				id = *svc.ID
			}

			name := ""
			if svc.Name != nil {
				name = *svc.Name
			}

			services = append(services, GatewayService{
				ID:             id,
				Name:           name,
				ControlPlaneID: controlPlaneID,
				Service:        svc,
			})
		}

		if resp.Object.Offset != nil && *resp.Object.Offset != "" && len(resp.Object.Data) > 0 {
			offsetVal = *resp.Object.Offset
			hasOffset = true
			continue
		}

		break
	}

	return services, nil
}

// GetControlPlaneByName finds a managed control plane by name
func (c *Client) GetControlPlaneByName(ctx context.Context, name string) (*ControlPlane, error) {
	controlPlanes, err := c.ListManagedControlPlanes(ctx, []string{"*"})
	if err != nil {
		return nil, err
	}

	for _, cp := range controlPlanes {
		if cp.Name == name {
			return &cp, nil
		}
	}

	// Fallback: look through all control planes and return ones that were previously managed
	allControlPlanes, err := c.ListAllControlPlanes(ctx)
	if err != nil {
		return nil, fmt.Errorf("fallback lookup failed: %w", err)
	}

	for _, cp := range allControlPlanes {
		if cp.Name == name && c.hasAnyKongctlLabels(cp.Labels) {
			return &cp, nil
		}
	}

	return nil, nil
}

// GetControlPlaneByFilter finds a managed control plane using a filter expression
func (c *Client) GetControlPlaneByFilter(ctx context.Context, filter string) (*ControlPlane, error) {
	if c.controlPlaneAPI == nil {
		return nil, fmt.Errorf("control plane API client not configured")
	}

	controlPlanes, err := c.ListManagedControlPlanes(ctx, []string{"*"})
	if err != nil {
		return nil, err
	}

	if after, ok := strings.CutPrefix(filter, "name[eq]="); ok {
		name := after
		for _, cp := range controlPlanes {
			if cp.Name == name {
				return &cp, nil
			}
		}
	}

	return nil, nil
}

// GetControlPlaneByID finds a control plane by ID (used for fallback during protection changes)
func (c *Client) GetControlPlaneByID(ctx context.Context, id string) (*ControlPlane, error) {
	if c.controlPlaneAPI == nil {
		return nil, fmt.Errorf("control plane API client not configured")
	}

	resp, err := c.controlPlaneAPI.GetControlPlane(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane by ID: %w", err)
	}

	if resp.ControlPlane == nil {
		return nil, nil
	}

	normalized := resp.ControlPlane.Labels
	if normalized == nil {
		normalized = make(map[string]string)
	}

	return &ControlPlane{
		ControlPlane:     *resp.ControlPlane,
		NormalizedLabels: normalized,
	}, nil
}

// CreateControlPlane creates a new control plane with management labels
func (c *Client) CreateControlPlane(
	ctx context.Context,
	controlPlane kkComps.CreateControlPlaneRequest,
	namespace string,
) (*kkComps.ControlPlane, error) {
	if err := ValidateAPIClient(c.controlPlaneAPI, "Control Plane API"); err != nil {
		return nil, err
	}

	logger := ctx.Value(log.LoggerKey).(*slog.Logger)
	logger.Debug("CreateControlPlane called",
		slog.Any("labels", controlPlane.Labels),
		slog.String("namespace", namespace))

	resp, err := c.controlPlaneAPI.CreateControlPlane(ctx, controlPlane)
	if err != nil {
		return nil, WrapAPIError(err, "create control plane", &ErrorWrapperOptions{
			ResourceType: "control_plane",
			ResourceName: controlPlane.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp.ControlPlane == nil {
		return nil, fmt.Errorf("create control plane response missing control plane data")
	}

	return resp.ControlPlane, nil
}

// UpdateControlPlane updates an existing control plane
func (c *Client) UpdateControlPlane(
	ctx context.Context,
	id string,
	controlPlane kkComps.UpdateControlPlaneRequest,
	namespace string,
) (*kkComps.ControlPlane, error) {
	if err := ValidateAPIClient(c.controlPlaneAPI, "Control Plane API"); err != nil {
		return nil, err
	}

	resp, err := c.controlPlaneAPI.UpdateControlPlane(ctx, id, controlPlane)
	if err != nil {
		statusCode := decerrors.ExtractStatusCodeFromError(err)

		ctx := decerrors.APIErrorContext{
			ResourceType: "control_plane",
			ResourceName: func() string {
				if controlPlane.Name != nil {
					return *controlPlane.Name
				}
				return ""
			}(),
			Operation:  "update",
			Namespace:  namespace,
			StatusCode: statusCode,
		}

		return nil, decerrors.EnhanceAPIError(err, ctx)
	}

	if resp.ControlPlane == nil {
		return nil, fmt.Errorf("update control plane response missing control plane data")
	}

	return resp.ControlPlane, nil
}

// DeleteControlPlane deletes a control plane by ID
func (c *Client) DeleteControlPlane(ctx context.Context, id string) error {
	if err := ValidateAPIClient(c.controlPlaneAPI, "Control Plane API"); err != nil {
		return err
	}

	_, err := c.controlPlaneAPI.DeleteControlPlane(ctx, id)
	if err != nil {
		statusCode := decerrors.ExtractStatusCodeFromError(err)
		ctx := decerrors.APIErrorContext{
			ResourceType: "control_plane",
			ResourceName: id,
			Operation:    "delete",
			StatusCode:   statusCode,
		}
		return decerrors.EnhanceAPIError(err, ctx)
	}

	return nil
}

// ListManagedAPIs returns all KONGCTL-managed APIs in the specified namespaces
// If namespaces is empty, no resources are returned (breaking change from previous behavior)
// To get all managed resources across all namespaces, pass []string{"*"}
func (c *Client) ListManagedAPIs(ctx context.Context, namespaces []string) ([]API, error) {
	// Validate API client
	if err := ValidateAPIClient(c.apiAPI, "API"); err != nil {
		return nil, err
	}

	// Create paginated lister function
	lister := func(ctx context.Context, pageSize, pageNumber int64) ([]API, *PageMeta, error) {
		req := kkOps.ListApisRequest{
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
		}

		resp, err := c.apiAPI.ListApis(ctx, req)
		if err != nil {
			return nil, nil, WrapAPIError(err, "list APIs", nil)
		}

		if resp.ListAPIResponse == nil {
			return []API{}, &PageMeta{Total: 0}, nil
		}

		var filteredAPIs []API

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
					filteredAPIs = append(filteredAPIs, api)
				}
			}
		}

		// Extract pagination metadata
		meta := &PageMeta{Total: resp.ListAPIResponse.Meta.Page.Total}

		return filteredAPIs, meta, nil
	}

	return PaginateAll(ctx, lister)
}

// GetAPIByName finds a managed API by name
func (c *Client) GetAPIByName(ctx context.Context, name string) (*API, error) {
	// Get logger from context
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)
	logger.Debug("Looking up API by name", "name", name)

	// Primary strategy: Standard managed resource lookup
	apis, err := c.ListManagedAPIs(ctx, []string{"*"})
	if err != nil {
		logger.Error("Failed to list managed APIs", "error", err)
		return nil, err
	}

	logger.Debug("Found managed APIs", "count", len(apis))

	for _, a := range apis {
		if a.Name == name {
			logger.Debug("Found API via managed lookup", "name", name, "id", a.ID)
			return &a, nil
		}
	}

	// Fallback strategy: Look for resources that might be undergoing protection changes
	// This includes resources that might temporarily appear "unmanaged" during updates
	logger.Debug("API not found in managed resources, trying fallback lookup", "name", name)

	allAPIs, err := c.ListAllAPIs(ctx)
	if err != nil {
		logger.Error("Fallback lookup failed", "error", err)
		return nil, fmt.Errorf("fallback lookup failed: %w", err)
	}

	logger.Debug("Found total APIs", "count", len(allAPIs))

	for _, a := range allAPIs {
		if a.Name == name {
			// Check if this resource has any KONGCTL labels (indicating it was managed)
			if c.hasAnyKongctlLabels(a.Labels) {
				logger.Warn("Found API via fallback - may indicate protection change issue",
					"name", name, "id", a.ID, "labels", a.Labels)
				return &a, nil
			}
		}
	}

	logger.Debug("API not found in any lookup strategy", "name", name)
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
	if after, ok := strings.CutPrefix(filter, "name[eq]="); ok {
		name := after
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
		statusCode := decerrors.ExtractStatusCodeFromError(err)

		// Create enhanced error with context and hints
		ctx := decerrors.APIErrorContext{
			ResourceType: "api",
			ResourceName: api.Name,
			Namespace:    namespace,
			Operation:    "create",
			StatusCode:   statusCode,
		}

		return nil, decerrors.EnhanceAPIError(err, ctx)
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

// ListManagedCatalogServices returns all KONGCTL-managed catalog services in the specified namespaces.
// If namespaces is empty, no resources are returned. To get all managed resources, pass []string{"*"}.
func (c *Client) ListManagedCatalogServices(ctx context.Context, namespaces []string) ([]CatalogService, error) {
	if err := ValidateAPIClient(c.catalogServiceAPI, "Catalog Service API"); err != nil {
		return nil, err
	}

	lister := func(ctx context.Context, pageSize, pageNumber int64) ([]CatalogService, *PageMeta, error) {
		req := kkOps.ListCatalogServicesRequest{
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
		}

		resp, err := c.catalogServiceAPI.ListCatalogServices(ctx, req)
		if err != nil {
			return nil, nil, WrapAPIError(err, "list catalog services", nil)
		}

		if resp.ListCatalogServicesResponse == nil {
			return []CatalogService{}, &PageMeta{Total: 0}, nil
		}

		var filtered []CatalogService
		for _, svc := range resp.ListCatalogServicesResponse.Data {
			normalized := svc.Labels
			if normalized == nil {
				normalized = make(map[string]string)
			}

			if labels.IsManagedResource(normalized) &&
				shouldIncludeNamespace(normalized[labels.NamespaceKey], namespaces) {
				filtered = append(filtered, CatalogService{
					CatalogService:   svc,
					NormalizedLabels: normalized,
				})
			}
		}

		meta := &PageMeta{Total: resp.ListCatalogServicesResponse.Meta.Page.Total}
		return filtered, meta, nil
	}

	return PaginateAll(ctx, lister)
}

// GetCatalogServiceByName finds a managed catalog service by name.
func (c *Client) GetCatalogServiceByName(ctx context.Context, name string) (*CatalogService, error) {
	if c.catalogServiceAPI == nil {
		return nil, fmt.Errorf("catalog service API not configured")
	}

	services, err := c.ListManagedCatalogServices(ctx, []string{"*"})
	if err != nil {
		return nil, err
	}

	for i := range services {
		if services[i].Name == name {
			return &services[i], nil
		}
	}

	return nil, nil
}

// GetCatalogServiceByID fetches a catalog service by ID.
func (c *Client) GetCatalogServiceByID(ctx context.Context, id string) (*CatalogService, error) {
	if c.catalogServiceAPI == nil {
		return nil, fmt.Errorf("catalog service API not configured")
	}

	resp, err := c.catalogServiceAPI.FetchCatalogService(ctx, id)
	if err != nil {
		return nil, WrapAPIError(err, "fetch catalog service", nil)
	}

	if resp.CatalogService == nil {
		return nil, nil
	}

	normalized := resp.CatalogService.Labels
	if normalized == nil {
		normalized = make(map[string]string)
	}

	return &CatalogService{
		CatalogService:   *resp.CatalogService,
		NormalizedLabels: normalized,
	}, nil
}

// CreateCatalogService creates a new catalog service with management labels.
func (c *Client) CreateCatalogService(
	ctx context.Context,
	req kkComps.CreateCatalogService,
	namespace string,
) (*kkComps.CatalogService, error) {
	if c.catalogServiceAPI == nil {
		return nil, fmt.Errorf("catalog service API not configured")
	}

	resp, err := c.catalogServiceAPI.CreateCatalogService(ctx, req)
	if err != nil {
		return nil, WrapAPIError(err, "create catalog service", &ErrorWrapperOptions{
			ResourceType: "catalog_service",
			ResourceName: req.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp.CatalogService == nil {
		return nil, fmt.Errorf("create catalog service response missing data")
	}

	return resp.CatalogService, nil
}

// UpdateCatalogService updates an existing catalog service.
func (c *Client) UpdateCatalogService(
	ctx context.Context,
	id string,
	req kkComps.UpdateCatalogService,
	namespace string,
) (*kkComps.CatalogService, error) {
	if c.catalogServiceAPI == nil {
		return nil, fmt.Errorf("catalog service API not configured")
	}

	resp, err := c.catalogServiceAPI.UpdateCatalogService(ctx, id, req)
	if err != nil {
		resourceName := ""
		if req.Name != nil {
			resourceName = *req.Name
		}
		return nil, WrapAPIError(err, "update catalog service", &ErrorWrapperOptions{
			ResourceType: "catalog_service",
			ResourceName: resourceName,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp.CatalogService == nil {
		return nil, fmt.Errorf("update catalog service response missing data")
	}

	return resp.CatalogService, nil
}

// DeleteCatalogService deletes a catalog service by ID.
func (c *Client) DeleteCatalogService(ctx context.Context, id string) error {
	if c.catalogServiceAPI == nil {
		return fmt.Errorf("catalog service API not configured")
	}

	_, err := c.catalogServiceAPI.DeleteCatalogService(ctx, id)
	if err != nil {
		return WrapAPIError(err, "delete catalog service", nil)
	}

	return nil
}

// ListAllAPIs returns all APIs without managed filtering (for fallback lookups)
func (c *Client) ListAllAPIs(ctx context.Context) ([]API, error) {
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

		for _, api := range resp.ListAPIResponse.Data {
			// Labels are already map[string]string in the SDK
			normalized := api.Labels
			if normalized == nil {
				normalized = make(map[string]string)
			}

			parsedAPI := API{
				APIResponseSchema: api,
				NormalizedLabels:  normalized,
			}
			allAPIs = append(allAPIs, parsedAPI)
		}

		// Check if we've retrieved all pages
		// Since Meta and Page are not pointers, we check the total count
		if resp.ListAPIResponse.Meta.Page.Total <= float64(pageNumber*pageSize) {
			break
		}

		pageNumber++
	}

	return allAPIs, nil
}

// hasAnyKongctlLabels checks if a resource has any KONGCTL labels
func (c *Client) hasAnyKongctlLabels(labels map[string]string) bool {
	if labels == nil {
		return false
	}
	for key := range labels {
		if strings.HasPrefix(key, "KONGCTL-") {
			return true
		}
	}
	return false
}

// GetAPIByID finds an API by ID (for fallback during protection changes)
func (c *Client) GetAPIByID(ctx context.Context, id string) (*API, error) {
	if c.apiAPI == nil {
		return nil, fmt.Errorf("API client not configured")
	}

	resp, err := c.apiAPI.FetchAPI(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get API by ID: %w", err)
	}

	if resp.APIResponseSchema == nil {
		return nil, nil
	}

	// Labels are already map[string]string in the SDK
	normalized := resp.APIResponseSchema.Labels
	if normalized == nil {
		normalized = make(map[string]string)
	}

	api := &API{
		APIResponseSchema: *resp.APIResponseSchema,
		NormalizedLabels:  normalized,
	}

	return api, nil
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
				// Other fields not available in list response - use defaults
				PublishStatus: "",
				Deprecated:    false,
				SunsetDate:    "",
				Spec:          "",
			}
			allVersions = append(allVersions, version)
		}

		pageNumber++

		// Check if we've fetched all pages
		if resp.ListAPIVersionResponse.Meta.Page.Total <= float64(pageSize*pageNumber) {
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

// UpdateAPIVersion updates an existing API version
func (c *Client) UpdateAPIVersion(
	ctx context.Context, apiID, versionID string, version kkComps.APIVersion,
) (*kkComps.APIVersionResponse, error) {
	if c.apiVersionAPI == nil {
		return nil, fmt.Errorf("API version client not configured")
	}

	// Create the request object as expected by the SDK
	req := kkOps.UpdateAPIVersionRequest{
		APIID:      apiID,
		VersionID:  versionID,
		APIVersion: version,
	}

	resp, err := c.apiVersionAPI.UpdateAPIVersion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to update API version: %w", err)
	}

	if resp.APIVersionResponse == nil {
		return nil, fmt.Errorf("update API version response missing data")
	}

	return resp.APIVersionResponse, nil
}

// FetchAPIVersion retrieves a single API version with full content
func (c *Client) FetchAPIVersion(ctx context.Context, apiID, versionID string) (*APIVersion, error) {
	if c.apiVersionAPI == nil {
		return nil, fmt.Errorf("API version client not configured")
	}

	resp, err := c.apiVersionAPI.FetchAPIVersion(ctx, apiID, versionID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API version: %w", err)
	}

	if resp == nil || resp.APIVersionResponse == nil {
		return nil, fmt.Errorf("fetch API version response missing data")
	}

	// Convert to our internal type with full content
	version := &APIVersion{
		ID:      resp.APIVersionResponse.ID,
		Version: resp.APIVersionResponse.Version,
	}

	// Set spec content if available
	if resp.APIVersionResponse.Spec != nil && resp.APIVersionResponse.Spec.Content != nil {
		version.Spec = *resp.APIVersionResponse.Spec.Content
	}

	return version, nil
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
				ID:                       "", // Publications don't have a separate ID
				PortalID:                 p.PortalID,
				AuthStrategyIDs:          p.AuthStrategyIds,
				AutoApproveRegistrations: p.AutoApproveRegistrations,
			}
			if p.Visibility != nil {
				pub.Visibility = string(*p.Visibility)
			}
			// AutoApproveRegistrations not available in list response
			allPublications = append(allPublications, pub)
		}

		pageNumber++

		// Check if we've fetched all pages
		if resp.ListAPIPublicationResponse.Meta.Page.Total <= float64(pageSize*pageNumber) {
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

		for _, item := range resp.ListAPIImplementationsResponse.Data {
			entity := item.APIImplementationListItemGatewayServiceEntity
			if entity == nil {
				continue
			}

			impl := APIImplementation{
				ID: entity.GetID(),
			}

			// ImplementationURL not available in list response
			if svc := entity.GetService(); svc != nil {
				impl.Service = &struct {
					ID             string
					ControlPlaneID string
				}{
					ID:             svc.GetID(),
					ControlPlaneID: svc.GetControlPlaneID(),
				}
			}

			allImplementations = append(allImplementations, impl)
		}

		pageNumber++

		// Check if we've fetched all pages
		if resp.ListAPIImplementationsResponse.Meta.Page.Total <= float64(pageSize*pageNumber) {
			break
		}
	}

	return allImplementations, nil
}

// CreateAPIImplementation creates a new API implementation
func (c *Client) CreateAPIImplementation(
	ctx context.Context, apiID string, implementation kkComps.APIImplementation,
) (*kkComps.APIImplementationResponse, error) {
	if err := ValidateAPIClient(c.apiImplementationAPI, "API Implementation API"); err != nil {
		return nil, err
	}

	resp, err := c.apiImplementationAPI.CreateAPIImplementation(ctx, apiID, implementation)
	if err != nil {
		return nil, fmt.Errorf("failed to create API implementation: %w", err)
	}

	if resp == nil || resp.APIImplementationResponse == nil {
		return nil, fmt.Errorf("API implementation creation returned no response")
	}

	return resp.APIImplementationResponse, nil
}

// DeleteAPIImplementation deletes an API implementation
func (c *Client) DeleteAPIImplementation(ctx context.Context, apiID, implementationID string) error {
	if err := ValidateAPIClient(c.apiImplementationAPI, "API Implementation API"); err != nil {
		return err
	}

	_, err := c.apiImplementationAPI.DeleteAPIImplementation(ctx, apiID, implementationID)
	if err != nil {
		return fmt.Errorf("failed to delete API implementation: %w", err)
	}

	return nil
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
	// Validate API client
	if err := ValidateAPIClient(c.appAuthAPI, "app auth API"); err != nil {
		return nil, err
	}

	// Create paginated lister function
	lister := func(ctx context.Context, pageSize, pageNumber int64) ([]ApplicationAuthStrategy, *PageMeta, error) {
		req := kkOps.ListAppAuthStrategiesRequest{
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
		}

		resp, err := c.appAuthAPI.ListAppAuthStrategies(ctx, req)
		if err != nil {
			return nil, nil, WrapAPIError(err, "list application auth strategies", nil)
		}

		if resp.ListAppAuthStrategiesResponse == nil {
			return []ApplicationAuthStrategy{}, &PageMeta{Total: 0}, nil
		}

		var filteredStrategies []ApplicationAuthStrategy

		// Process and filter auth strategies
		for _, s := range resp.ListAppAuthStrategiesResponse.Data {
			strategy := c.extractAuthStrategyFromUnion(s)
			if strategy == nil {
				// Unknown type, skip
				continue
			}

			// Check if resource has a namespace label (new criteria for managed resources)
			if labels.IsManagedResource(strategy.NormalizedLabels) {
				// Filter by namespace if specified
				if shouldIncludeNamespace(strategy.NormalizedLabels[labels.NamespaceKey], namespaces) {
					filteredStrategies = append(filteredStrategies, *strategy)
				}
			}
		}

		// Extract pagination metadata
		meta := &PageMeta{Total: resp.ListAppAuthStrategiesResponse.Meta.Page.Total}

		return filteredStrategies, meta, nil
	}

	return PaginateAll(ctx, lister)
}

// extractAuthStrategyFromUnion extracts a normalized auth strategy from the SDK union type
func (c *Client) extractAuthStrategyFromUnion(s kkComps.AppAuthStrategy) *ApplicationAuthStrategy {
	if s.AppAuthStrategyKeyAuthResponseAppAuthStrategyKeyAuthResponse != nil {
		keyAuthResp := s.AppAuthStrategyKeyAuthResponseAppAuthStrategyKeyAuthResponse
		return normalizeKeyAuthStrategy(
			keyAuthResp.ID,
			keyAuthResp.Name,
			keyAuthResp.DisplayName,
			keyAuthResp.Labels,
			keyAuthResp.Configs.KeyAuth.KeyNames,
			idValue(keyAuthResp.DcrProvider),
			nameValue(keyAuthResp.DcrProvider),
		)
	}

	if s.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyOpenIDConnectResponse != nil {
		oidcResp := s.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyOpenIDConnectResponse
		return normalizeOIDCStrategy(
			oidcResp.ID,
			oidcResp.Name,
			oidcResp.DisplayName,
			oidcResp.Labels,
			oidcResp.Configs.OpenidConnect.Issuer,
			oidcResp.Configs.OpenidConnect.CredentialClaim,
			oidcResp.Configs.OpenidConnect.Scopes,
			oidcResp.Configs.OpenidConnect.AuthMethods,
			idValue(oidcResp.DcrProvider),
			nameValue(oidcResp.DcrProvider),
		)
	}

	return nil
}

func (c *Client) extractAuthStrategyFromCreateResponse(
	resp *kkComps.CreateAppAuthStrategyResponse,
) *ApplicationAuthStrategy {
	if resp == nil {
		return nil
	}

	if keyAuthResp := resp.AppAuthStrategyKeyAuthResponse; keyAuthResp != nil {
		return normalizeKeyAuthStrategy(
			keyAuthResp.ID,
			keyAuthResp.Name,
			keyAuthResp.DisplayName,
			keyAuthResp.Labels,
			keyAuthResp.Configs.KeyAuth.KeyNames,
			idValue(keyAuthResp.DcrProvider),
			nameValue(keyAuthResp.DcrProvider),
		)
	}

	if oidcResp := resp.AppAuthStrategyOpenIDConnectResponse; oidcResp != nil {
		return normalizeOIDCStrategy(
			oidcResp.ID,
			oidcResp.Name,
			oidcResp.DisplayName,
			oidcResp.Labels,
			oidcResp.Configs.OpenidConnect.Issuer,
			oidcResp.Configs.OpenidConnect.CredentialClaim,
			oidcResp.Configs.OpenidConnect.Scopes,
			oidcResp.Configs.OpenidConnect.AuthMethods,
			idValue(oidcResp.DcrProvider),
			nameValue(oidcResp.DcrProvider),
		)
	}

	return nil
}

func normalizeLabelMap(m map[string]string) map[string]string {
	if m == nil {
		return make(map[string]string)
	}
	return m
}

func normalizeKeyAuthStrategy(
	id, name, displayName string,
	labelMap map[string]string,
	keyNames []string,
	dcrProviderID string,
	dcrProviderName string,
) *ApplicationAuthStrategy {
	strategy := &ApplicationAuthStrategy{
		ID:              id,
		Name:            name,
		DisplayName:     displayName,
		StrategyType:    "key_auth",
		DCRProviderID:   dcrProviderID,
		DCRProviderName: dcrProviderName,
		Configs: map[string]any{
			"key-auth": map[string]any{},
		},
	}

	if keyNames != nil {
		strategy.Configs["key-auth"].(map[string]any)["key_names"] = keyNames
	}

	strategy.NormalizedLabels = normalizeLabelMap(labelMap)

	return strategy
}

func normalizeOIDCStrategy(
	id, name, displayName string,
	labelMap map[string]string,
	issuer string,
	credentialClaim, scopes, authMethods []string,
	dcrProviderID string,
	dcrProviderName string,
) *ApplicationAuthStrategy {
	oidcConfig := map[string]any{
		"issuer": issuer,
	}
	if credentialClaim != nil {
		oidcConfig["credential_claim"] = credentialClaim
	}
	if scopes != nil {
		oidcConfig["scopes"] = scopes
	}
	if authMethods != nil {
		oidcConfig["auth_methods"] = authMethods
	}

	return &ApplicationAuthStrategy{
		ID:              id,
		Name:            name,
		DisplayName:     displayName,
		StrategyType:    "openid_connect",
		DCRProviderID:   dcrProviderID,
		DCRProviderName: dcrProviderName,
		Configs: map[string]any{
			"openid-connect": oidcConfig,
		},
		NormalizedLabels: normalizeLabelMap(labelMap),
	}
}

type normalizedDCRProviderPayload struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	DisplayName  *string           `json:"display_name"`
	ProviderType string            `json:"provider_type"`
	Issuer       string            `json:"issuer"`
	DCRConfig    map[string]any    `json:"dcr_config"`
	Labels       map[string]string `json:"labels"`
}

func normalizeDCRProviderFromAny(data any) (*DCRProvider, error) {
	payloadBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal DCR provider payload: %w", err)
	}

	var payload normalizedDCRProviderPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal DCR provider payload: %w", err)
	}

	displayName := ""
	if payload.DisplayName != nil {
		displayName = *payload.DisplayName
	}

	return &DCRProvider{
		ID:               payload.ID,
		Name:             payload.Name,
		DisplayName:      displayName,
		DisplayNameSet:   payload.DisplayName != nil,
		ProviderType:     payload.ProviderType,
		Issuer:           payload.Issuer,
		DCRConfig:        payload.DCRConfig,
		NormalizedLabels: normalizeLabelMap(payload.Labels),
	}, nil
}

func idValue(v any) string {
	if v == nil {
		return ""
	}

	payloadBytes, err := json.Marshal(v)
	if err != nil {
		return ""
	}

	var payload map[string]any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return ""
	}

	str, _ := payload["id"].(string)
	if str != "" {
		return str
	}
	str, _ = payload["ID"].(string)
	return str
}

func nameValue(v any) string {
	if v == nil {
		return ""
	}

	payloadBytes, err := json.Marshal(v)
	if err != nil {
		return ""
	}

	var payload map[string]any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return ""
	}

	str, _ := payload["name"].(string)
	if str != "" {
		return str
	}
	str, _ = payload["Name"].(string)
	return str
}

// CreateDCRProvider creates a new DCR provider with management labels.
func (c *Client) CreateDCRProvider(
	ctx context.Context,
	provider kkComps.CreateDcrProviderRequest,
	_ string,
) (*DCRProvider, error) {
	if c.dcrProviderAPI == nil {
		return nil, fmt.Errorf("dcr provider API client not configured")
	}

	if _, err := c.dcrProviderAPI.CreateDcrProvider(ctx, provider); err != nil {
		return nil, fmt.Errorf("failed to create DCR provider: %w", err)
	}

	providerBytes, err := json.Marshal(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect DCR provider create request: %w", err)
	}

	var payload normalizedDCRProviderPayload
	if err := json.Unmarshal(providerBytes, &payload); err != nil {
		return nil, fmt.Errorf("failed to read DCR provider create request: %w", err)
	}

	created, err := c.GetDCRProviderByName(ctx, payload.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch DCR provider after create: %w", err)
	}
	if created == nil {
		return nil, fmt.Errorf("created DCR provider not found by name: %s", payload.Name)
	}

	return created, nil
}

// ListManagedDCRProviders returns all KONGCTL-managed DCR providers in the specified namespaces.
func (c *Client) ListManagedDCRProviders(
	ctx context.Context, namespaces []string,
) ([]DCRProvider, error) {
	if err := ValidateAPIClient(c.dcrProviderAPI, "dcr provider API"); err != nil {
		return nil, err
	}

	lister := func(ctx context.Context, pageSize, pageNumber int64) ([]DCRProvider, *PageMeta, error) {
		req := kkOps.ListDcrProvidersRequest{
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
		}

		resp, err := c.dcrProviderAPI.ListDcrProviderPayloads(ctx, req)
		if err != nil {
			return nil, nil, WrapAPIError(err, "list dcr providers", nil)
		}

		if resp == nil {
			return []DCRProvider{}, &PageMeta{Total: 0}, nil
		}

		var filtered []DCRProvider
		for _, p := range resp.Data {
			provider, err := normalizeDCRProviderFromAny(p)
			if err != nil {
				return nil, nil, err
			}
			if labels.IsManagedResource(provider.NormalizedLabels) &&
				shouldIncludeNamespace(provider.NormalizedLabels[labels.NamespaceKey], namespaces) {
				filtered = append(filtered, *provider)
			}
		}

		meta := &PageMeta{Total: resp.Total}
		return filtered, meta, nil
	}

	return PaginateAll(ctx, lister)
}

func (c *Client) GetDCRProviderByName(ctx context.Context, name string) (*DCRProvider, error) {
	providers, err := c.ListManagedDCRProviders(ctx, []string{"*"})
	if err != nil {
		return nil, err
	}
	for _, provider := range providers {
		if provider.Name == name {
			return &provider, nil
		}
	}
	return nil, nil
}

func (c *Client) GetDCRProviderByID(ctx context.Context, id string) (*DCRProvider, error) {
	providers, err := c.ListManagedDCRProviders(ctx, []string{"*"})
	if err != nil {
		return nil, err
	}
	for _, provider := range providers {
		if provider.ID == id {
			return &provider, nil
		}
	}
	return nil, nil
}

func (c *Client) UpdateDCRProvider(
	ctx context.Context,
	id string,
	provider kkComps.UpdateDcrProviderRequest,
	_ string,
) error {
	if c.dcrProviderAPI == nil {
		return fmt.Errorf("dcr provider API client not configured")
	}
	if _, err := c.dcrProviderAPI.UpdateDcrProvider(ctx, id, provider); err != nil {
		return fmt.Errorf("failed to update DCR provider: %w", err)
	}
	return nil
}

func (c *Client) DeleteDCRProvider(ctx context.Context, id string) error {
	if c.dcrProviderAPI == nil {
		return fmt.Errorf("dcr provider API client not configured")
	}
	if _, err := c.dcrProviderAPI.DeleteDcrProvider(ctx, id); err != nil {
		return fmt.Errorf("failed to delete DCR provider: %w", err)
	}
	return nil
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

// GetAuthStrategyByID finds a managed auth strategy by ID.
func (c *Client) GetAuthStrategyByID(ctx context.Context, id string) (*ApplicationAuthStrategy, error) {
	if err := ValidateAPIClient(c.appAuthAPI, "app auth API"); err != nil {
		return nil, err
	}

	resp, err := c.appAuthAPI.GetAppAuthStrategy(ctx, id)
	if err != nil {
		return nil, WrapAPIError(err, "get application auth strategy by ID", &ErrorWrapperOptions{
			ResourceType: "application_auth_strategy",
			UseEnhanced:  true,
		})
	}
	if resp == nil {
		return nil, nil
	}

	return c.extractAuthStrategyFromCreateResponse(resp.GetCreateAppAuthStrategyResponse()), nil
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
	if after, ok := strings.CutPrefix(filter, "name[eq]="); ok {
		name := after
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

// GetPortalAuthSettings fetches current auth settings for a portal.
func (c *Client) GetPortalAuthSettings(
	ctx context.Context,
	portalID string,
) (*kkComps.PortalAuthenticationSettingsResponse, error) {
	if err := ValidateAPIClient(c.portalAuthSettingsAPI, "portal auth settings API"); err != nil {
		return nil, err
	}

	resp, err := c.portalAuthSettingsAPI.GetPortalAuthenticationSettings(ctx, portalID)
	if err != nil {
		return nil, WrapAPIError(err, "get portal auth settings", nil)
	}

	if resp.PortalAuthenticationSettingsResponse == nil {
		return nil, fmt.Errorf("no portal auth settings data in response")
	}

	return resp.PortalAuthenticationSettingsResponse, nil
}

// UpdatePortalAuthSettings updates portal authentication settings.
func (c *Client) UpdatePortalAuthSettings(
	ctx context.Context,
	portalID string,
	settings kkComps.PortalAuthenticationSettingsUpdateRequest,
) error {
	if err := ValidateAPIClient(c.portalAuthSettingsAPI, "portal auth settings API"); err != nil {
		return err
	}

	if logger, ok := ctx.Value(log.LoggerKey).(*slog.Logger); ok && logger != nil {
		logger.Debug("updating portal auth settings",
			"portal_id", portalID,
			"basic_auth_enabled", settings.BasicAuthEnabled,
			"idp_mapping_enabled", settings.IdpMappingEnabled,
			"konnect_mapping_enabled", settings.KonnectMappingEnabled,
		)
	}

	_, err := c.portalAuthSettingsAPI.UpdatePortalAuthenticationSettings(ctx, portalID, &settings)
	if err != nil {
		return WrapAPIError(err, "update portal auth settings", nil)
	}
	return nil
}

// ListPortalIdentityProviders returns all identity providers for a portal.
func (c *Client) ListPortalIdentityProviders(ctx context.Context, portalID string) ([]PortalIdentityProvider, error) {
	if err := ValidateAPIClient(c.portalIdentityProviderAPI, "portal identity provider API"); err != nil {
		return nil, err
	}

	resp, err := c.portalIdentityProviderAPI.ListPortalIdentityProviders(
		ctx,
		kkOps.GetPortalIdentityProvidersRequest{PortalID: portalID},
	)
	if err != nil {
		return nil, WrapAPIError(
			err,
			"list portal identity providers",
			&ErrorWrapperOptions{ResourceType: "portal_identity_provider", ResourceName: portalID, UseEnhanced: true},
		)
	}

	providers := make([]PortalIdentityProvider, 0, len(resp.IdentityProviders))
	for _, provider := range resp.IdentityProviders {
		providers = append(providers, normalizePortalIdentityProvider(provider))
	}

	return providers, nil
}

// GetPortalIdentityProvider fetches a single identity provider for a portal.
func (c *Client) GetPortalIdentityProvider(
	ctx context.Context,
	portalID string,
	id string,
) (*PortalIdentityProvider, error) {
	if err := ValidateAPIClient(c.portalIdentityProviderAPI, "portal identity provider API"); err != nil {
		return nil, err
	}

	resp, err := c.portalIdentityProviderAPI.GetPortalIdentityProvider(ctx, portalID, id)
	if err != nil {
		var notFound *kkErrors.NotFoundError
		if errors.As(err, &notFound) {
			return nil, nil
		}
		return nil, WrapAPIError(
			err,
			"get portal identity provider",
			&ErrorWrapperOptions{ResourceType: "portal_identity_provider", ResourceName: id, UseEnhanced: true},
		)
	}
	if resp == nil || resp.IdentityProvider == nil {
		return nil, nil
	}

	provider := normalizePortalIdentityProvider(*resp.IdentityProvider)
	return &provider, nil
}

// CreatePortalIdentityProvider creates a new identity provider for a portal.
func (c *Client) CreatePortalIdentityProvider(
	ctx context.Context,
	portalID string,
	body kkComps.CreateIdentityProvider,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.portalIdentityProviderAPI, "portal identity provider API"); err != nil {
		return "", err
	}

	resp, err := c.portalIdentityProviderAPI.CreatePortalIdentityProvider(ctx, portalID, body)
	if err != nil {
		resourceName := portalIdentityProviderName(body)
		return "", WrapAPIError(
			err,
			"create portal identity provider",
			&ErrorWrapperOptions{
				ResourceType: "portal_identity_provider",
				ResourceName: resourceName,
				Namespace:    namespace,
				UseEnhanced:  true,
			},
		)
	}
	if resp == nil || resp.IdentityProvider == nil || resp.IdentityProvider.ID == nil {
		return "", NewResponseValidationError("create portal identity provider", "IdentityProvider")
	}

	return *resp.IdentityProvider.ID, nil
}

// UpdatePortalIdentityProvider updates an identity provider for a portal.
func (c *Client) UpdatePortalIdentityProvider(
	ctx context.Context,
	portalID string,
	id string,
	body kkComps.UpdateIdentityProvider,
	namespace string,
) error {
	if err := ValidateAPIClient(c.portalIdentityProviderAPI, "portal identity provider API"); err != nil {
		return err
	}

	_, err := c.portalIdentityProviderAPI.UpdatePortalIdentityProvider(
		ctx,
		kkOps.UpdatePortalIdentityProviderRequest{PortalID: portalID, ID: id, UpdateIdentityProvider: body},
	)
	if err != nil {
		return WrapAPIError(
			err,
			"update portal identity provider",
			&ErrorWrapperOptions{
				ResourceType: "portal_identity_provider",
				ResourceName: id,
				Namespace:    namespace,
				UseEnhanced:  true,
			},
		)
	}
	return nil
}

// DeletePortalIdentityProvider deletes an identity provider from a portal.
func (c *Client) DeletePortalIdentityProvider(ctx context.Context, portalID string, id string) error {
	if err := ValidateAPIClient(c.portalIdentityProviderAPI, "portal identity provider API"); err != nil {
		return err
	}

	_, err := c.portalIdentityProviderAPI.DeletePortalIdentityProvider(ctx, portalID, id)
	if err != nil {
		return WrapAPIError(
			err,
			"delete portal identity provider",
			&ErrorWrapperOptions{ResourceType: "portal_identity_provider", ResourceName: id, UseEnhanced: true},
		)
	}
	return nil
}

func normalizePortalIdentityProvider(provider kkComps.IdentityProvider) PortalIdentityProvider {
	normalized := PortalIdentityProvider{Config: provider.Config}
	if provider.ID != nil {
		normalized.ID = *provider.ID
	}
	if provider.Type != nil {
		normalized.Type = *provider.Type
	}
	normalized.Enabled = provider.Enabled
	normalized.LoginPath = provider.LoginPath
	return normalized
}

func portalIdentityProviderName(body kkComps.CreateIdentityProvider) string {
	if body.Type == nil {
		return ""
	}
	return string(*body.Type)
}

// GetPortalEmailConfig fetches the current email configuration for a portal.
func (c *Client) GetPortalEmailConfig(ctx context.Context, portalID string) (*kkComps.PortalEmailConfig, error) {
	if err := ValidateAPIClient(c.portalEmailsAPI, "portal emails API"); err != nil {
		return nil, err
	}

	resp, err := c.portalEmailsAPI.GetEmailConfig(ctx, portalID)
	if err != nil {
		var notFound *kkErrors.NotFoundError
		if errors.As(err, &notFound) {
			return nil, nil
		}
		return nil, WrapAPIError(err, "get portal email config", nil)
	}

	if err := ValidateResponse(resp.PortalEmailConfig, "get portal email config"); err != nil {
		return nil, err
	}

	return resp.PortalEmailConfig, nil
}

// CreatePortalEmailConfig creates a new email configuration for a portal.
func (c *Client) CreatePortalEmailConfig(
	ctx context.Context,
	portalID string,
	body kkComps.PostPortalEmailConfig,
) (string, error) {
	if err := ValidateAPIClient(c.portalEmailsAPI, "portal emails API"); err != nil {
		return "", err
	}

	resp, err := c.portalEmailsAPI.CreatePortalEmailConfig(ctx, portalID, body)
	if err != nil {
		return "", WrapAPIError(err, "create portal email config", &ErrorWrapperOptions{
			ResourceType: "portal_email_config",
			ResourceName: portalID,
			UseEnhanced:  true,
		})
	}
	if resp == nil || resp.PortalEmailConfig == nil {
		return "", NewResponseValidationError("create portal email config", "PortalEmailConfig")
	}
	return resp.PortalEmailConfig.ID, nil
}

// UpdatePortalEmailConfig updates the email configuration for a portal.
func (c *Client) UpdatePortalEmailConfig(
	ctx context.Context,
	portalID string,
	body *kkComps.PatchPortalEmailConfig,
) (string, error) {
	if err := ValidateAPIClient(c.portalEmailsAPI, "portal emails API"); err != nil {
		return "", err
	}

	resp, err := c.portalEmailsAPI.UpdatePortalEmailConfig(ctx, portalID, body)
	if err != nil {
		return "", WrapAPIError(err, "update portal email config", &ErrorWrapperOptions{
			ResourceType: "portal_email_config",
			ResourceName: portalID,
			UseEnhanced:  true,
		})
	}
	if resp == nil || resp.PortalEmailConfig == nil {
		return "", NewResponseValidationError("update portal email config", "PortalEmailConfig")
	}
	return resp.PortalEmailConfig.ID, nil
}

// DeletePortalEmailConfig deletes the email configuration for a portal.
func (c *Client) DeletePortalEmailConfig(ctx context.Context, portalID string) error {
	if err := ValidateAPIClient(c.portalEmailsAPI, "portal emails API"); err != nil {
		return err
	}

	if _, err := c.portalEmailsAPI.DeletePortalEmailConfig(ctx, portalID); err != nil {
		return WrapAPIError(err, "delete portal email config", &ErrorWrapperOptions{
			ResourceType: "portal_email_config",
			ResourceName: portalID,
		})
	}
	return nil
}

// ListPortalCustomEmailTemplates returns customized templates for a portal.
func (c *Client) ListPortalCustomEmailTemplates(ctx context.Context, portalID string) ([]PortalEmailTemplate, error) {
	if err := ValidateAPIClient(c.portalEmailsAPI, "portal emails API"); err != nil {
		return nil, err
	}

	resp, err := c.portalEmailsAPI.ListPortalCustomEmailTemplates(ctx, portalID)
	if err != nil {
		return nil, WrapAPIError(err, "list portal email templates", &ErrorWrapperOptions{
			ResourceType: "portal_email_template",
			ResourceName: portalID,
		})
	}

	if resp == nil || resp.ListEmailTemplates == nil {
		return nil, nil
	}

	templates := make([]PortalEmailTemplate, 0, len(resp.ListEmailTemplates.Data))
	for _, tpl := range resp.ListEmailTemplates.Data {
		templates = append(templates, normalizePortalEmailTemplate(tpl))
	}
	return templates, nil
}

// GetPortalCustomEmailTemplate fetches a single customized email template.
func (c *Client) GetPortalCustomEmailTemplate(
	ctx context.Context,
	portalID string,
	name kkComps.EmailTemplateName,
) (*PortalEmailTemplate, error) {
	if err := ValidateAPIClient(c.portalEmailsAPI, "portal emails API"); err != nil {
		return nil, err
	}

	resp, err := c.portalEmailsAPI.GetPortalCustomEmailTemplate(ctx, portalID, name)
	if err != nil {
		var notFound *kkErrors.NotFoundError
		if errors.As(err, &notFound) {
			return nil, nil
		}
		return nil, WrapAPIError(err, "get portal email template", &ErrorWrapperOptions{
			ResourceType: "portal_email_template",
			ResourceName: string(name),
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.EmailTemplate == nil {
		return nil, NewResponseValidationError("get portal email template", "EmailTemplate")
	}

	tpl := normalizePortalEmailTemplate(*resp.EmailTemplate)
	return &tpl, nil
}

// UpdatePortalEmailTemplate creates or updates a customized email template.
func (c *Client) UpdatePortalEmailTemplate(
	ctx context.Context,
	portalID string,
	name kkComps.EmailTemplateName,
	payload kkComps.PatchCustomPortalEmailTemplatePayload,
) (string, error) {
	if err := ValidateAPIClient(c.portalEmailsAPI, "portal emails API"); err != nil {
		return "", err
	}

	req := kkOps.UpdatePortalCustomEmailTemplateRequest{
		PortalID:                              portalID,
		TemplateName:                          name,
		PatchCustomPortalEmailTemplatePayload: payload,
	}

	resp, err := c.portalEmailsAPI.UpdatePortalCustomEmailTemplate(ctx, req)
	if err != nil {
		return "", WrapAPIError(err, "update portal email template", &ErrorWrapperOptions{
			ResourceType: "portal_email_template",
			ResourceName: string(name),
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.EmailTemplate == nil {
		return "", NewResponseValidationError("update portal email template", "EmailTemplate")
	}

	return string(resp.EmailTemplate.Name), nil
}

// DeletePortalEmailTemplate deletes a customized email template.
func (c *Client) DeletePortalEmailTemplate(
	ctx context.Context,
	portalID string,
	name kkComps.EmailTemplateName,
) error {
	if err := ValidateAPIClient(c.portalEmailsAPI, "portal emails API"); err != nil {
		return err
	}

	if _, err := c.portalEmailsAPI.DeletePortalCustomEmailTemplate(ctx, portalID, name); err != nil {
		return WrapAPIError(err, "delete portal email template", &ErrorWrapperOptions{
			ResourceType: "portal_email_template",
			ResourceName: string(name),
		})
	}
	return nil
}

func normalizePortalEmailTemplate(t kkComps.EmailTemplate) PortalEmailTemplate {
	tpl := PortalEmailTemplate{
		ID:        string(t.Name),
		Name:      string(t.Name),
		Label:     t.Label,
		Enabled:   t.Enabled,
		Variables: t.Variables,
	}

	if t.Content != nil {
		tpl.Content = &PortalEmailTemplateContent{
			Subject:     t.Content.Subject,
			Title:       t.Content.Title,
			Body:        t.Content.Body,
			ButtonLabel: t.Content.ButtonLabel,
		}
	}

	return tpl
}

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

// GetPortalAssetLogo fetches the logo for a portal as a data URL
func (c *Client) GetPortalAssetLogo(ctx context.Context, portalID string) (string, error) {
	if c.assetsAPI == nil {
		return "", fmt.Errorf("assets API not configured")
	}

	resp, err := c.assetsAPI.GetPortalAssetLogo(ctx, portalID)
	if err != nil {
		return "", WrapAPIError(err, "get portal logo", &ErrorWrapperOptions{
			ResourceType: "portal_asset_logo",
			ResourceName: portalID,
			UseEnhanced:  true,
		})
	}

	if resp.PortalAssetResponse == nil {
		return "", fmt.Errorf("no portal asset response in logo response")
	}

	return resp.PortalAssetResponse.Data, nil
}

// ReplacePortalAssetLogo uploads a new logo for a portal
func (c *Client) ReplacePortalAssetLogo(ctx context.Context, portalID, dataURL string) error {
	if c.assetsAPI == nil {
		return fmt.Errorf("assets API not configured")
	}

	req := &kkComps.ReplacePortalImageAsset{
		Data: dataURL,
	}

	_, err := c.assetsAPI.ReplacePortalAssetLogo(ctx, portalID, req)
	if err != nil {
		return WrapAPIError(err, "replace portal logo", &ErrorWrapperOptions{
			ResourceType: "portal_asset_logo",
			ResourceName: portalID,
			UseEnhanced:  true,
		})
	}

	return nil
}

// GetPortalAssetFavicon fetches the favicon for a portal as a data URL
func (c *Client) GetPortalAssetFavicon(ctx context.Context, portalID string) (string, error) {
	if c.assetsAPI == nil {
		return "", fmt.Errorf("assets API not configured")
	}

	resp, err := c.assetsAPI.GetPortalAssetFavicon(ctx, portalID)
	if err != nil {
		return "", WrapAPIError(err, "get portal favicon", &ErrorWrapperOptions{
			ResourceType: "portal_asset_favicon",
			ResourceName: portalID,
			UseEnhanced:  true,
		})
	}

	if resp.PortalAssetResponse == nil {
		return "", fmt.Errorf("no portal asset response in favicon response")
	}

	return resp.PortalAssetResponse.Data, nil
}

// ReplacePortalAssetFavicon uploads a new favicon for a portal
func (c *Client) ReplacePortalAssetFavicon(ctx context.Context, portalID, dataURL string) error {
	if c.assetsAPI == nil {
		return fmt.Errorf("assets API not configured")
	}

	req := &kkComps.ReplacePortalImageAsset{
		Data: dataURL,
	}

	_, err := c.assetsAPI.ReplacePortalAssetFavicon(ctx, portalID, req)
	if err != nil {
		return WrapAPIError(err, "replace portal favicon", &ErrorWrapperOptions{
			ResourceType: "portal_asset_favicon",
			ResourceName: portalID,
			UseEnhanced:  true,
		})
	}

	return nil
}

// GetPortalCustomDomain fetches the current custom domain for a portal.
func (c *Client) GetPortalCustomDomain(ctx context.Context, portalID string) (*PortalCustomDomain, error) {
	if err := ValidateAPIClient(c.portalCustomDomainAPI, "portal custom domain API"); err != nil {
		return nil, err
	}

	resp, err := c.portalCustomDomainAPI.GetPortalCustomDomain(ctx, portalID)
	if err != nil {
		var notFound *kkErrors.NotFoundError
		if errors.As(err, &notFound) {
			return nil, nil
		}
		return nil, WrapAPIError(err, "get portal custom domain", nil)
	}

	if err := ValidateResponse(resp, "get portal custom domain"); err != nil {
		return nil, err
	}

	if resp.PortalCustomDomain == nil {
		return nil, NewResponseValidationError("get portal custom domain", "PortalCustomDomain")
	}

	domain := resp.PortalCustomDomain

	skipCACheck := domain.Ssl.SkipCaCheck
	uploadedAt := domain.Ssl.UploadedAt
	expiresAt := domain.Ssl.ExpiresAt

	validationErrors := append([]string(nil), domain.Ssl.ValidationErrors...)
	if len(validationErrors) == 0 {
		validationErrors = nil
	}

	return &PortalCustomDomain{
		ID:                       portalID,
		PortalID:                 portalID,
		Hostname:                 domain.Hostname,
		Enabled:                  domain.Enabled,
		DomainVerificationMethod: string(domain.Ssl.DomainVerificationMethod),
		VerificationStatus:       string(domain.Ssl.VerificationStatus),
		ValidationErrors:         validationErrors,
		SkipCACheck:              skipCACheck,
		UploadedAt:               uploadedAt,
		ExpiresAt:                expiresAt,
		CnameStatus:              string(domain.CnameStatus),
		CreatedAt:                domain.CreatedAt,
		UpdatedAt:                domain.UpdatedAt,
	}, nil
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
			ID:    p.ID,
			Slug:  p.Slug,
			Title: p.Title,
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
		if resp.ListPortalSnippetsResponse.Meta.Page.Total <= float64(pageSize*pageNumber) {
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

// Portal Team Methods

// ListPortalTeams returns all teams for a portal
func (c *Client) ListPortalTeams(ctx context.Context, portalID string) ([]PortalTeam, error) {
	if c.portalTeamAPI == nil {
		return nil, fmt.Errorf("portal team API not configured")
	}

	var allTeams []PortalTeam
	var pageNumber int64 = 1
	pageSize := int64(100)

	for {
		req := kkOps.ListPortalTeamsRequest{
			PortalID:   portalID,
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
		}

		resp, err := c.portalTeamAPI.ListPortalTeams(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list portal teams", &ErrorWrapperOptions{
				ResourceType: "portal_team",
				UseEnhanced:  true,
			})
		}

		if resp.ListPortalTeamsResponse == nil || len(resp.ListPortalTeamsResponse.Data) == 0 {
			break
		}

		// Process teams
		for _, t := range resp.ListPortalTeamsResponse.Data {
			team := PortalTeam{
				ID:   "",
				Name: "",
			}

			// Handle optional pointer fields from SDK
			if t.ID != nil {
				team.ID = *t.ID
			}
			if t.Name != nil {
				team.Name = *t.Name
			}
			if t.Description != nil {
				team.Description = *t.Description
			}
			team.CanOwnApplications = t.CanOwnApplications

			allTeams = append(allTeams, team)
		}

		pageNumber++

		// Check if we've fetched all pages
		if resp.ListPortalTeamsResponse.Meta.Page.Total <= float64(pageSize*pageNumber) {
			break
		}
	}

	return allTeams, nil
}

// CreatePortalTeam creates a new portal team
func (c *Client) CreatePortalTeam(
	ctx context.Context,
	portalID string,
	req kkComps.PortalCreateTeamRequest,
	namespace string,
) (string, error) {
	if c.portalTeamAPI == nil {
		return "", fmt.Errorf("portal team API not configured")
	}

	resp, err := c.portalTeamAPI.CreatePortalTeam(ctx, portalID, &req)
	if err != nil {
		return "", WrapAPIError(err, "create portal team", &ErrorWrapperOptions{
			ResourceType: "portal_team",
			ResourceName: req.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp.PortalTeamResponse == nil {
		return "", fmt.Errorf("no response data from create portal team")
	}

	teamID := ""
	if resp.PortalTeamResponse.ID != nil {
		teamID = *resp.PortalTeamResponse.ID
	}

	return teamID, nil
}

// UpdatePortalTeam updates a portal team
func (c *Client) UpdatePortalTeam(
	ctx context.Context,
	portalID string,
	teamID string,
	req kkComps.PortalUpdateTeamRequest,
	namespace string,
) error {
	if c.portalTeamAPI == nil {
		return fmt.Errorf("portal team API not configured")
	}

	updateReq := kkOps.UpdatePortalTeamRequest{
		PortalID:                portalID,
		TeamID:                  teamID,
		PortalUpdateTeamRequest: &req,
	}

	_, err := c.portalTeamAPI.UpdatePortalTeam(ctx, updateReq)
	if err != nil {
		teamName := ""
		if req.Name != nil {
			teamName = *req.Name
		}
		return WrapAPIError(err, "update portal team", &ErrorWrapperOptions{
			ResourceType: "portal_team",
			ResourceName: teamName,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	return nil
}

// DeletePortalTeam deletes a portal team
func (c *Client) DeletePortalTeam(ctx context.Context, portalID string, teamID string) error {
	if c.portalTeamAPI == nil {
		return fmt.Errorf("portal team API not configured")
	}

	_, err := c.portalTeamAPI.DeletePortalTeam(ctx, teamID, portalID)
	if err != nil {
		return WrapAPIError(err, "delete portal team", &ErrorWrapperOptions{
			ResourceType: "portal_team",
			UseEnhanced:  true,
		})
	}
	return nil
}

// ListPortalTeamRoles returns all assigned roles for a portal team
func (c *Client) ListPortalTeamRoles(ctx context.Context, portalID string, teamID string) ([]PortalTeamRole, error) {
	if c.portalTeamRolesAPI == nil {
		return nil, fmt.Errorf("portal team roles API not configured")
	}

	lister := func(ctx context.Context, pageSize, pageNumber int64) ([]PortalTeamRole, *PageMeta, error) {
		req := kkOps.ListPortalTeamRolesRequest{
			PortalID:   portalID,
			TeamID:     teamID,
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
		}

		resp, err := c.portalTeamRolesAPI.ListPortalTeamRoles(ctx, req)
		if err != nil {
			return nil, nil, WrapAPIError(err, "list portal team roles", &ErrorWrapperOptions{
				ResourceType: "portal_team_role",
				UseEnhanced:  true,
			})
		}

		if resp.AssignedPortalRoleCollectionResponse == nil {
			return []PortalTeamRole{}, &PageMeta{Total: 0}, nil
		}

		var allRoles []PortalTeamRole
		for _, r := range resp.AssignedPortalRoleCollectionResponse.Data {
			role := PortalTeamRole{
				ID:             r.ID,
				RoleName:       r.RoleName,
				EntityID:       r.EntityID,
				EntityTypeName: r.EntityTypeName,
				EntityRegion:   string(r.EntityRegion),
				TeamID:         teamID,
				PortalID:       portalID,
			}
			allRoles = append(allRoles, role)
		}

		meta := &PageMeta{Total: resp.AssignedPortalRoleCollectionResponse.Meta.Page.Total}

		return allRoles, meta, nil
	}

	return PaginateAll(ctx, lister)
}

// AssignPortalTeamRole assigns a role to a portal team
func (c *Client) AssignPortalTeamRole(
	ctx context.Context,
	portalID string,
	teamID string,
	req kkComps.PortalAssignRoleRequest,
	namespace string,
) (string, error) {
	if c.portalTeamRolesAPI == nil {
		return "", fmt.Errorf("portal team roles API not configured")
	}

	assignReq := kkOps.AssignRoleToPortalTeamsRequest{
		PortalID:                portalID,
		TeamID:                  teamID,
		PortalAssignRoleRequest: &req,
	}

	resp, err := c.portalTeamRolesAPI.AssignRoleToPortalTeams(ctx, assignReq)
	if err != nil {
		roleName := req.RoleName
		return "", WrapAPIError(err, "assign portal team role", &ErrorWrapperOptions{
			ResourceType: "portal_team_role",
			ResourceName: roleName,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp.PortalAssignedRoleResponse == nil {
		return "", fmt.Errorf("no response data from assign portal team role")
	}

	return resp.PortalAssignedRoleResponse.ID, nil
}

// RemovePortalTeamRole removes an assigned role from a portal team
func (c *Client) RemovePortalTeamRole(ctx context.Context, portalID string, teamID string, roleID string) error {
	if c.portalTeamRolesAPI == nil {
		return fmt.Errorf("portal team roles API not configured")
	}

	removeReq := kkOps.RemoveRoleFromPortalTeamRequest{
		PortalID: portalID,
		TeamID:   teamID,
		RoleID:   roleID,
	}

	_, err := c.portalTeamRolesAPI.RemoveRoleFromPortalTeam(ctx, removeReq)
	if err != nil {
		return WrapAPIError(err, "remove portal team role", &ErrorWrapperOptions{
			ResourceType: "portal_team_role",
			UseEnhanced:  true,
		})
	}

	return nil
}

func (c *Client) ListManagedEventGatewayControlPlanes(
	ctx context.Context,
	namespaces []string,
) ([]EventGatewayControlPlane, error) {
	// Validate API client is initialized
	if err := ValidateAPIClient(c.egwControlPlaneAPI, "event gateway control plane API"); err != nil {
		return nil, err
	}

	var allData []kkComps.EventGatewayInfo
	var pageAfter *string

	for {
		req := kkOps.ListEventGatewaysRequest{}

		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := c.egwControlPlaneAPI.ListEGWControlPlanes(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list event gateway control planes", nil)
		}

		// If response is nil, break the loop
		if res.ListEventGatewaysResponse == nil {
			return []EventGatewayControlPlane{}, nil
		}

		allData = append(allData, res.ListEventGatewaysResponse.Data...)

		if res.ListEventGatewaysResponse.Meta.Page.Next == nil {
			break
		}

		u, err := url.Parse(*res.ListEventGatewaysResponse.Meta.Page.Next)
		if err != nil {
			return nil, WrapAPIError(err, "list event gateway control planes: invalid cursor", nil)
		}

		values := u.Query()
		pageAfter = new(values.Get("page[after]"))
	}

	var filteredEGWControlPlanes []EventGatewayControlPlane
	for _, f := range allData {
		// Filter by managed status and namespace
		if labels.IsManagedResource(f.Labels) {
			if shouldIncludeNamespace(f.Labels[labels.NamespaceKey], namespaces) {
				eventGatewayControlPlane := EventGatewayControlPlane{
					f,
					f.Labels,
				}
				filteredEGWControlPlanes = append(filteredEGWControlPlanes, eventGatewayControlPlane)
			}
		}
	}
	return filteredEGWControlPlanes, nil
}

func (c *Client) CreateEventGatewayControlPlane(
	ctx context.Context,
	req kkComps.CreateGatewayRequest,
	namespace string,
) (string, error) {
	resp, err := c.egwControlPlaneAPI.CreateEGWControlPlane(ctx, req)
	if err != nil {
		return "", WrapAPIError(err, "create event gateway control plane", &ErrorWrapperOptions{
			ResourceType: "event_gateway",
			ResourceName: "", // Adjust based on SDK
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if err := ValidateResponse(resp.EventGatewayInfo, "create event gateway control plane"); err != nil {
		return "", err
	}

	return resp.EventGatewayInfo.ID, nil
}

func (c *Client) UpdateEventGatewayControlPlane(
	ctx context.Context,
	id string,
	req kkComps.UpdateGatewayRequest,
	namespace string,
) (string, error) {
	resp, err := c.egwControlPlaneAPI.UpdateEGWControlPlane(ctx, id, req)
	if err != nil {
		return "", WrapAPIError(err, "update event gateway control plane", &ErrorWrapperOptions{
			ResourceType: "event_gateway",
			ResourceName: "", // Adjust based on SDK
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	return resp.EventGatewayInfo.ID, nil
}

func (c *Client) GetEventGatewayControlPlaneByID(ctx context.Context, id string) (*EventGatewayControlPlane, error) {
	resp, err := c.egwControlPlaneAPI.FetchEGWControlPlane(ctx, id)
	if err != nil {
		return nil, WrapAPIError(err, "get event gateway control plane by ID", &ErrorWrapperOptions{
			ResourceType: "event_gateway",
			ResourceName: "", // Adjust based on SDK
			UseEnhanced:  true,
		})
	}

	if resp.EventGatewayInfo == nil {
		return nil, nil
	}

	// Labels are already map[string]string in the SDK
	normalized := resp.EventGatewayInfo.Labels
	if normalized == nil {
		normalized = make(map[string]string)
	}

	eventGateway := &EventGatewayControlPlane{
		EventGatewayInfo: *resp.EventGatewayInfo,
		NormalizedLabels: normalized,
	}

	return eventGateway, nil
}

func (c *Client) GetEventGatewayControlPlaneByName(
	ctx context.Context, name string,
) (*EventGatewayControlPlane, error) {
	// List all event gateways and filter by name
	gateways, err := c.ListManagedEventGatewayControlPlanes(ctx, []string{"*"})
	if err != nil {
		return nil, WrapAPIError(err, "list event gateways to find by name", &ErrorWrapperOptions{
			ResourceType: "event_gateway",
			ResourceName: name,
			UseEnhanced:  true,
		})
	}

	for _, gw := range gateways {
		if gw.Name == name {
			return &gw, nil
		}
	}

	return nil, nil
}

func (c *Client) DeleteEventGatewayControlPlane(ctx context.Context, id string) error {
	// Placeholder for future implementation
	_, err := c.egwControlPlaneAPI.DeleteEGWControlPlane(ctx, id)
	if err != nil {
		return WrapAPIError(err, "delete event gateway control plane", nil)
	}
	return nil
}

func (c *Client) ListEventGatewayBackendClusters(
	ctx context.Context,
	gatewayID string,
) ([]EventGatewayBackendCluster, error) {
	// Validate API client is initialized
	if err := ValidateAPIClient(c.eventGatewayBackendClusterAPI, "event gateway backend cluster API"); err != nil {
		return nil, err
	}

	var allData []kkComps.BackendCluster
	var pageAfter *string

	for {
		req := kkOps.ListEventGatewayBackendClustersRequest{
			GatewayID: gatewayID,
		}

		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := c.eventGatewayBackendClusterAPI.ListEventGatewayBackendClusters(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list event gateway backend clusters", nil)
		}

		// If response is nil, break the loop
		if res.ListBackendClustersResponse == nil {
			return []EventGatewayBackendCluster{}, nil
		}

		allData = append(allData, res.ListBackendClustersResponse.Data...)

		if res.ListBackendClustersResponse.Meta.Page.Next == nil {
			break
		}

		u, err := url.Parse(*res.ListBackendClustersResponse.Meta.Page.Next)
		if err != nil {
			return nil, WrapAPIError(err, "list event gateway backend clusters: invalid cursor", nil)
		}

		values := u.Query()
		pageAfter = new(values.Get("page[after]"))
	}

	var backendClusters []EventGatewayBackendCluster
	for _, bc := range allData {
		// Normalize labels
		normalized := bc.Labels
		if normalized == nil {
			normalized = make(map[string]string)
		}

		backendClusters = append(backendClusters, EventGatewayBackendCluster{
			BackendCluster:   bc,
			NormalizedLabels: normalized,
		})
	}

	return backendClusters, nil
}

func (c *Client) CreateEventGatewayBackendCluster(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateBackendClusterRequest,
	namespace string,
) (string, error) {
	resp, err := c.eventGatewayBackendClusterAPI.CreateEventGatewayBackendCluster(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(err, "create event gateway backend cluster", &ErrorWrapperOptions{
			ResourceType: "event_gateway_backend_cluster",
			ResourceName: req.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if err := ValidateResponse(resp.BackendCluster, "create event gateway backend cluster"); err != nil {
		return "", err
	}

	return resp.BackendCluster.ID, nil
}

func (c *Client) GetEventGatewayBackendCluster(
	ctx context.Context,
	gatewayID string,
	clusterID string,
) (*EventGatewayBackendCluster, error) {
	resp, err := c.eventGatewayBackendClusterAPI.FetchEventGatewayBackendCluster(ctx, gatewayID, clusterID)
	if err != nil {
		return nil, WrapAPIError(err, "get event gateway backend cluster by ID", &ErrorWrapperOptions{
			ResourceType: "event_gateway_backend_cluster",
			UseEnhanced:  true,
		})
	}

	if resp.BackendCluster == nil {
		return nil, nil
	}

	// Normalize labels
	normalized := resp.BackendCluster.Labels
	if normalized == nil {
		normalized = make(map[string]string)
	}

	backendCluster := &EventGatewayBackendCluster{
		BackendCluster:   *resp.BackendCluster,
		NormalizedLabels: normalized,
	}

	return backendCluster, nil
}

func (c *Client) GetEventGatewayBackendClusterByName(
	ctx context.Context, gatewayID string, name string,
) (*EventGatewayBackendCluster, error) {
	// List all event gateway backend cluster and filter by name
	backendClusters, err := c.ListEventGatewayBackendClusters(ctx, gatewayID)
	if err != nil {
		return nil, WrapAPIError(err, "list event gateway backend clusters to find by name", &ErrorWrapperOptions{
			ResourceType: "event_gateway_backend_cluster",
			ResourceName: name,
			UseEnhanced:  true,
		})
	}

	for _, bc := range backendClusters {
		if bc.Name == name {
			return &bc, nil
		}
	}

	return nil, nil
}

func (c *Client) UpdateEventGatewayBackendCluster(
	ctx context.Context,
	gatewayID string,
	clusterID string,
	req kkComps.UpdateBackendClusterRequest,
	namespace string,
) (string, error) {
	resp, err := c.eventGatewayBackendClusterAPI.UpdateEventGatewayBackendCluster(ctx, gatewayID, clusterID, req)
	if err != nil {
		return "", WrapAPIError(err, "update event gateway backend cluster", &ErrorWrapperOptions{
			ResourceType: "event_gateway_backend_cluster",
			ResourceName: req.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	return resp.BackendCluster.ID, nil
}

func (c *Client) DeleteEventGatewayBackendCluster(
	ctx context.Context,
	gatewayID string,
	clusterID string,
) error {
	_, err := c.eventGatewayBackendClusterAPI.DeleteEventGatewayBackendCluster(ctx, gatewayID, clusterID)
	if err != nil {
		return WrapAPIError(err, "delete event gateway backend cluster", nil)
	}
	return nil
}

func (c *Client) ListManagedOrganizationTeams(ctx context.Context, namespaces []string) ([]OrganizationTeam, error) {
	if err := ValidateAPIClient(c.organizationTeamAPI, "organization team API"); err != nil {
		return nil, err
	}

	// Create paginated lister function
	lister := func(ctx context.Context, pageSize, pageNumber int64) ([]OrganizationTeam, *PageMeta, error) {
		req := kkOps.ListTeamsRequest{
			PageSize:   &pageSize,
			PageNumber: &pageNumber,
		}

		resp, err := c.organizationTeamAPI.ListOrganizationTeams(ctx, req)
		if err != nil {
			return nil, nil, WrapAPIError(err, "list teams", nil)
		}

		if resp.TeamCollection == nil {
			return []OrganizationTeam{}, &PageMeta{Total: 0}, nil
		}

		var filtered []OrganizationTeam

		for _, t := range resp.TeamCollection.Data {
			normalized := t.Labels
			if normalized == nil {
				normalized = make(map[string]string)
			}

			if labels.IsManagedResource(normalized) &&
				shouldIncludeNamespace(normalized[labels.NamespaceKey], namespaces) {
				filtered = append(filtered, OrganizationTeam{
					Team:             t,
					NormalizedLabels: normalized,
				})
			}
		}

		meta := &PageMeta{Total: resp.TeamCollection.Meta.Page.Total}

		return filtered, meta, nil
	}

	return PaginateAll(ctx, lister)
}

// GetOrganizationTeamByName finds a managed organization team by name
func (c *Client) GetOrganizationTeamByName(ctx context.Context, name string) (*OrganizationTeam, error) {
	// Search across all namespaces for backward compatibility
	teams, err := c.ListManagedOrganizationTeams(ctx, []string{"*"})
	if err != nil {
		return nil, err
	}

	for _, t := range teams {
		if t.Name != nil && *t.Name == name {
			return &t, nil
		}
	}

	return nil, nil // Not found
}

func (c *Client) GetOrganizationTeamByID(ctx context.Context, id string) (*OrganizationTeam, error) {
	resp, err := c.organizationTeamAPI.GetOrganizationTeam(ctx, id)
	if err != nil {
		return nil, WrapAPIError(err, "get team by ID", &ErrorWrapperOptions{
			ResourceType: "team",
			ResourceName: "",
			UseEnhanced:  true,
		})
	}

	if resp.Team == nil {
		return nil, nil
	}

	normalized := resp.Team.Labels
	if normalized == nil {
		normalized = make(map[string]string)
	}

	team := &OrganizationTeam{
		Team:             *resp.Team,
		NormalizedLabels: normalized,
	}

	return team, nil
}

func (c *Client) CreateOrganizationTeam(ctx context.Context, team *kkComps.CreateTeam,
	namespace string,
) (string, error) {
	resp, err := c.organizationTeamAPI.CreateOrganizationTeam(ctx, team)
	if err != nil {
		return "", WrapAPIError(err, "create organization team", &ErrorWrapperOptions{
			ResourceType: "organization_team",
			ResourceName: team.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if err := ValidateResponse(resp.Team, "create organization team"); err != nil {
		return "", err
	}

	if resp.Team.ID == nil {
		return "", NewResponseValidationError("create organization team", "Team.ID")
	}

	return *resp.Team.ID, nil
}

func (c *Client) UpdateOrganizationTeam(ctx context.Context, teamID string,
	team *kkComps.UpdateTeam, namespace string,
) (string, error) {
	resp, err := c.organizationTeamAPI.UpdateOrganizationTeam(ctx, teamID, team)
	if err != nil {
		return "", WrapAPIError(err, "update organization team", &ErrorWrapperOptions{
			ResourceType: "organization_team",
			ResourceName: getString(team.Name),
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if err := ValidateResponse(resp.Team, "update organization team"); err != nil {
		return "", err
	}

	if resp.Team.ID == nil {
		return "", NewResponseValidationError("update organization team", "Team.ID")
	}

	return *resp.Team.ID, nil
}

func (c *Client) DeleteOrganizationTeam(ctx context.Context, teamID string) error {
	_, err := c.organizationTeamAPI.DeleteOrganizationTeam(ctx, teamID)
	if err != nil {
		return WrapAPIError(err, "delete organization team", &ErrorWrapperOptions{
			ResourceType: "organization_team",
			ResourceName: teamID,
			UseEnhanced:  true,
		})
	}

	return nil
}

func getString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// shouldIncludeNamespace checks if a resource's namespace should be included based on filter
func shouldIncludeNamespace(resourceNamespace string, namespaces []string) bool {
	// Empty namespace list means no resources should be returned
	if len(namespaces) == 0 {
		return false
	}

	// Check for wildcard (all namespaces)
	if slices.Contains(namespaces, "*") {
		return true
	}

	// Check if resource's namespace is in the filter list
	return slices.Contains(namespaces, resourceNamespace)
}

// Event Gateway Virtual Cluster Methods

func (c *Client) ListEventGatewayVirtualClusters(
	ctx context.Context,
	gatewayID string,
) ([]EventGatewayVirtualCluster, error) {
	// Validate API client is initialized
	if err := ValidateAPIClient(c.eventGatewayVirtualClusterAPI, "event gateway virtual cluster API"); err != nil {
		return nil, err
	}

	var allData []kkComps.VirtualCluster
	var pageAfter *string

	for {
		req := kkOps.ListEventGatewayVirtualClustersRequest{
			GatewayID: gatewayID,
		}

		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := c.eventGatewayVirtualClusterAPI.ListEventGatewayVirtualClusters(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list event gateway virtual clusters", nil)
		}

		// If response is nil, break the loop
		if res.ListVirtualClustersResponse == nil {
			return []EventGatewayVirtualCluster{}, nil
		}

		allData = append(allData, res.ListVirtualClustersResponse.Data...)

		if res.ListVirtualClustersResponse.Meta.Page.Next == nil {
			break
		}

		u, err := url.Parse(*res.ListVirtualClustersResponse.Meta.Page.Next)
		if err != nil {
			return nil, WrapAPIError(err, "list event gateway virtual clusters: invalid cursor", nil)
		}

		values := u.Query()
		pageAfter = new(values.Get("page[after]"))
	}

	var virtualClusters []EventGatewayVirtualCluster
	for _, vc := range allData {
		// Normalize labels
		normalized := vc.Labels
		if normalized == nil {
			normalized = make(map[string]string)
		}

		virtualClusters = append(virtualClusters, EventGatewayVirtualCluster{
			VirtualCluster:   vc,
			NormalizedLabels: normalized,
		})
	}

	return virtualClusters, nil
}

func (c *Client) CreateEventGatewayVirtualCluster(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateVirtualClusterRequest,
	namespace string,
) (string, error) {
	resp, err := c.eventGatewayVirtualClusterAPI.CreateEventGatewayVirtualCluster(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(err, "create event gateway virtual cluster", &ErrorWrapperOptions{
			ResourceType: "event_gateway_virtual_cluster",
			ResourceName: req.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if err := ValidateResponse(resp.VirtualCluster, "create event gateway virtual cluster"); err != nil {
		return "", err
	}

	return resp.VirtualCluster.ID, nil
}

func (c *Client) GetEventGatewayVirtualCluster(
	ctx context.Context,
	gatewayID string,
	clusterID string,
) (*EventGatewayVirtualCluster, error) {
	resp, err := c.eventGatewayVirtualClusterAPI.FetchEventGatewayVirtualCluster(ctx, gatewayID, clusterID)
	if err != nil {
		return nil, WrapAPIError(err, "get event gateway virtual cluster by ID", &ErrorWrapperOptions{
			ResourceType: "event_gateway_virtual_cluster",
			UseEnhanced:  true,
		})
	}

	if resp.VirtualCluster == nil {
		return nil, nil
	}

	// Normalize labels
	normalized := resp.VirtualCluster.Labels
	if normalized == nil {
		normalized = make(map[string]string)
	}

	virtualCluster := &EventGatewayVirtualCluster{
		VirtualCluster:   *resp.VirtualCluster,
		NormalizedLabels: normalized,
	}

	return virtualCluster, nil
}

// GetEventGatewayVirtualClusterByName retrieves a virtual cluster by name within a gateway
func (c *Client) GetEventGatewayVirtualClusterByName(
	ctx context.Context,
	gatewayID string,
	name string,
) (*EventGatewayVirtualCluster, error) {
	clusters, err := c.ListEventGatewayVirtualClusters(ctx, gatewayID)
	if err != nil {
		return nil, err
	}

	for i, cluster := range clusters {
		if cluster.Name == name {
			return &clusters[i], nil
		}
	}

	return nil, nil
}

func (c *Client) UpdateEventGatewayVirtualCluster(
	ctx context.Context,
	gatewayID string,
	clusterID string,
	req kkComps.UpdateVirtualClusterRequest,
	namespace string,
) (string, error) {
	resp, err := c.eventGatewayVirtualClusterAPI.UpdateEventGatewayVirtualCluster(ctx, gatewayID, clusterID, req)
	if err != nil {
		return "", WrapAPIError(err, "update event gateway virtual cluster", &ErrorWrapperOptions{
			ResourceType: "event_gateway_virtual_cluster",
			ResourceName: req.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	return resp.VirtualCluster.ID, nil
}

func (c *Client) DeleteEventGatewayVirtualCluster(
	ctx context.Context,
	gatewayID string,
	clusterID string,
) error {
	_, err := c.eventGatewayVirtualClusterAPI.DeleteEventGatewayVirtualCluster(ctx, gatewayID, clusterID)
	if err != nil {
		return WrapAPIError(err, "delete event gateway virtual cluster", nil)
	}
	return nil
}

// Event Gateway Listener Methods

func (c *Client) ListEventGatewayListeners(
	ctx context.Context,
	gatewayID string,
) ([]EventGatewayListener, error) {
	// Validate API client is initialized
	if err := ValidateAPIClient(c.eventGatewayListenerAPI, "event gateway listener API"); err != nil {
		return nil, err
	}

	var allData []kkComps.EventGatewayListener
	var pageAfter *string

	for {
		req := kkOps.ListEventGatewayListenersRequest{
			GatewayID: gatewayID,
		}

		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := c.eventGatewayListenerAPI.ListEventGatewayListeners(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list event gateway listeners", nil)
		}

		// If response is nil, break the loop
		if res.ListEventGatewayListenersResponse == nil {
			return []EventGatewayListener{}, nil
		}

		allData = append(allData, res.ListEventGatewayListenersResponse.Data...)

		if res.ListEventGatewayListenersResponse.Meta.Page.Next == nil {
			break
		}

		u, err := url.Parse(*res.ListEventGatewayListenersResponse.Meta.Page.Next)
		if err != nil {
			return nil, WrapAPIError(err, "list event gateway listeners: invalid cursor", nil)
		}

		values := u.Query()
		pageAfter = new(values.Get("page[after]"))
	}

	var listeners []EventGatewayListener
	for _, l := range allData {
		// Normalize labels
		normalized := l.Labels
		if normalized == nil {
			normalized = make(map[string]string)
		}

		listeners = append(listeners, EventGatewayListener{
			EventGatewayListener: l,
			NormalizedLabels:     normalized,
		})
	}

	return listeners, nil
}

func (c *Client) CreateEventGatewayListener(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateEventGatewayListenerRequest,
	namespace string,
) (string, error) {
	resp, err := c.eventGatewayListenerAPI.CreateEventGatewayListener(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(err, "create event gateway listener", &ErrorWrapperOptions{
			ResourceType: "event_gateway_listener",
			ResourceName: req.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if err := ValidateResponse(resp.EventGatewayListener, "create event gateway listener"); err != nil {
		return "", err
	}

	return resp.EventGatewayListener.ID, nil
}

func (c *Client) GetEventGatewayListener(
	ctx context.Context,
	gatewayID string,
	listenerID string,
) (*EventGatewayListener, error) {
	resp, err := c.eventGatewayListenerAPI.FetchEventGatewayListener(ctx, gatewayID, listenerID)
	if err != nil {
		return nil, WrapAPIError(err, "get event gateway listener by ID", &ErrorWrapperOptions{
			ResourceType: "event_gateway_listener",
			UseEnhanced:  true,
		})
	}

	if resp.EventGatewayListener == nil {
		return nil, nil
	}

	// Normalize labels
	normalized := resp.EventGatewayListener.Labels
	if normalized == nil {
		normalized = make(map[string]string)
	}

	listener := &EventGatewayListener{
		EventGatewayListener: *resp.EventGatewayListener,
		NormalizedLabels:     normalized,
	}

	return listener, nil
}

func (c *Client) UpdateEventGatewayListener(
	ctx context.Context,
	gatewayID string,
	listenerID string,
	req kkComps.UpdateEventGatewayListenerRequest,
	namespace string,
) (string, error) {
	resp, err := c.eventGatewayListenerAPI.UpdateEventGatewayListener(ctx, gatewayID, listenerID, req)
	if err != nil {
		return "", WrapAPIError(err, "update event gateway listener", &ErrorWrapperOptions{
			ResourceType: "event_gateway_listener",
			ResourceName: req.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	return resp.EventGatewayListener.ID, nil
}

func (c *Client) DeleteEventGatewayListener(
	ctx context.Context,
	gatewayID string,
	listenerID string,
) error {
	_, err := c.eventGatewayListenerAPI.DeleteEventGatewayListener(ctx, gatewayID, listenerID)
	if err != nil {
		return WrapAPIError(err, "delete event gateway listener", nil)
	}
	return nil
}

// ---- Event Gateway Listener Policy operations ----

// EventGatewayListenerPolicyInfo wraps an Event Gateway Listener Policy for internal use.
// RawConfig contains the full config from the raw API response since the SDK's
// EventGatewayListenerPolicyConfig struct is empty and doesn't capture actual config data.
type EventGatewayListenerPolicyInfo struct {
	kkComps.EventGatewayListenerPolicy
	NormalizedLabels map[string]string
	RawConfig        map[string]any
}

// listenerPolicyRawResponse is used to parse the raw API response to get full config.
type listenerPolicyRawResponse struct {
	Type           string            `json:"type"`
	Name           *string           `json:"name,omitempty"`
	Description    *string           `json:"description,omitempty"`
	Enabled        *bool             `json:"enabled,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	ID             string            `json:"id"`
	Config         map[string]any    `json:"config"`
	CreatedAt      string            `json:"created_at"`
	UpdatedAt      string            `json:"updated_at"`
	ParentPolicyID *string           `json:"parent_policy_id,omitempty"`
}

func (c *Client) ListEventGatewayListenerPolicies(
	ctx context.Context,
	gatewayID string,
	listenerID string,
) ([]EventGatewayListenerPolicyInfo, error) {
	if err := ValidateAPIClient(c.eventGatewayListenerPolicyAPI, "event gateway listener policy API"); err != nil {
		return nil, err
	}

	req := kkOps.ListEventGatewayListenerPoliciesRequest{
		GatewayID:  gatewayID,
		ListenerID: listenerID,
	}

	res, err := c.eventGatewayListenerPolicyAPI.ListEventGatewayListenerPolicies(ctx, req)
	if err != nil {
		return nil, WrapAPIError(err, "list event gateway listener policies", nil)
	}

	if res.ListEventGatewayListenerPoliciesResponse == nil {
		return []EventGatewayListenerPolicyInfo{}, nil
	}

	// Try to parse raw response to get full config data
	rawConfigByID := make(map[string]map[string]any)
	if res.RawResponse != nil && res.RawResponse.Body != nil {
		bodyBytes, readErr := io.ReadAll(res.RawResponse.Body)
		if readErr == nil && len(bodyBytes) > 0 {
			var rawPolicies []listenerPolicyRawResponse
			if jsonErr := json.Unmarshal(bodyBytes, &rawPolicies); jsonErr == nil {
				for _, rp := range rawPolicies {
					if rp.ID != "" && rp.Config != nil {
						rawConfigByID[rp.ID] = rp.Config
					}
				}
			}
		}
	}

	var policies []EventGatewayListenerPolicyInfo
	for _, p := range res.ListEventGatewayListenerPoliciesResponse {
		normalized := p.Labels
		if normalized == nil {
			normalized = make(map[string]string)
		}
		policies = append(policies, EventGatewayListenerPolicyInfo{
			EventGatewayListenerPolicy: p,
			NormalizedLabels:           normalized,
			RawConfig:                  rawConfigByID[p.ID],
		})
	}

	return policies, nil
}

func (c *Client) CreateEventGatewayListenerPolicy(
	ctx context.Context,
	gatewayID string,
	listenerID string,
	req kkComps.EventGatewayListenerPolicyCreate,
	namespace string,
) (string, error) {
	createReq := kkOps.CreateEventGatewayListenerPolicyRequest{
		GatewayID:                        gatewayID,
		ListenerID:                       listenerID,
		EventGatewayListenerPolicyCreate: req,
	}

	resp, err := c.eventGatewayListenerPolicyAPI.CreateEventGatewayListenerPolicy(ctx, createReq)
	if err != nil {
		return "", WrapAPIError(err, "create event gateway listener policy", &ErrorWrapperOptions{
			ResourceType: "event_gateway_listener_policy",
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if err := ValidateResponse(resp.EventGatewayListenerPolicy, "create event gateway listener policy"); err != nil {
		return "", err
	}

	return resp.EventGatewayListenerPolicy.ID, nil
}

func (c *Client) UpdateEventGatewayListenerPolicy(
	ctx context.Context,
	gatewayID string,
	listenerID string,
	policyID string,
	req kkComps.EventGatewayListenerPolicyUpdate,
	namespace string,
) (string, error) {
	updateReq := kkOps.UpdateEventGatewayListenerPolicyRequest{
		GatewayID:                        gatewayID,
		ListenerID:                       listenerID,
		PolicyID:                         policyID,
		EventGatewayListenerPolicyUpdate: req,
	}

	resp, err := c.eventGatewayListenerPolicyAPI.UpdateEventGatewayListenerPolicy(ctx, updateReq)
	if err != nil {
		return "", WrapAPIError(err, "update event gateway listener policy", &ErrorWrapperOptions{
			ResourceType: "event_gateway_listener_policy",
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	return resp.EventGatewayListenerPolicy.ID, nil
}

func (c *Client) DeleteEventGatewayListenerPolicy(
	ctx context.Context,
	gatewayID string,
	listenerID string,
	policyID string,
) error {
	deleteReq := kkOps.DeleteEventGatewayListenerPolicyRequest{
		GatewayID:  gatewayID,
		ListenerID: listenerID,
		PolicyID:   policyID,
	}

	_, err := c.eventGatewayListenerPolicyAPI.DeleteEventGatewayListenerPolicy(ctx, deleteReq)
	if err != nil {
		return WrapAPIError(err, "delete event gateway listener policy", nil)
	}
	return nil
}

func (c *Client) GetEventGatewayListenerPolicy(
	ctx context.Context,
	gatewayID string,
	listenerID string,
	policyID string,
) (*EventGatewayListenerPolicyInfo, error) {
	req := kkOps.GetEventGatewayListenerPolicyRequest{
		GatewayID:  gatewayID,
		ListenerID: listenerID,
		PolicyID:   policyID,
	}

	resp, err := c.eventGatewayListenerPolicyAPI.GetEventGatewayListenerPolicy(ctx, req)
	if err != nil {
		return nil, WrapAPIError(err, "get event gateway listener policy", nil)
	}

	if resp.EventGatewayListenerPolicy == nil {
		return nil, nil
	}

	normalized := resp.EventGatewayListenerPolicy.Labels
	if normalized == nil {
		normalized = make(map[string]string)
	}

	return &EventGatewayListenerPolicyInfo{
		EventGatewayListenerPolicy: *resp.EventGatewayListenerPolicy,
		NormalizedLabels:           normalized,
	}, nil
}

// ---- Event Gateway Cluster Policy operations ----

// EventGatewayClusterPolicyInfo wraps an Event Gateway Cluster Policy for internal use.
type EventGatewayClusterPolicyInfo struct {
	kkComps.EventGatewayPolicy
	NormalizedLabels map[string]string
	RawConfig        map[string]any
}

// clusterPolicyRawResponse is used to parse the raw API response to get full config.
type clusterPolicyRawResponse struct {
	Type        string            `json:"type"`
	Name        *string           `json:"name,omitempty"`
	Description *string           `json:"description,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	ID          string            `json:"id"`
	Config      map[string]any    `json:"config"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
	Condition   *string           `json:"condition,omitempty"`
}

func (c *Client) ListEventGatewayClusterPolicies(
	ctx context.Context,
	gatewayID string,
	virtualClusterID string,
) ([]EventGatewayClusterPolicyInfo, error) {
	if err := ValidateAPIClient(c.eventGatewayClusterPolicyAPI, "event gateway cluster policy API"); err != nil {
		return nil, err
	}

	req := kkOps.ListEventGatewayVirtualClusterClusterLevelPoliciesRequest{
		GatewayID:        gatewayID,
		VirtualClusterID: virtualClusterID,
	}

	res, err := c.eventGatewayClusterPolicyAPI.ListEventGatewayVirtualClusterClusterLevelPolicies(ctx, req)
	if err != nil {
		return nil, WrapAPIError(err, "list event gateway cluster policies", nil)
	}

	if res.ListClusterPoliciesResponse == nil {
		return []EventGatewayClusterPolicyInfo{}, nil
	}

	// Try to parse raw response to get full config data
	rawConfigByID := make(map[string]map[string]any)
	if res.RawResponse != nil && res.RawResponse.Body != nil {
		bodyBytes, readErr := io.ReadAll(res.RawResponse.Body)
		if readErr == nil && len(bodyBytes) > 0 {
			var rawPolicies []clusterPolicyRawResponse
			if jsonErr := json.Unmarshal(bodyBytes, &rawPolicies); jsonErr == nil {
				for _, rp := range rawPolicies {
					if rp.ID != "" && rp.Config != nil {
						rawConfigByID[rp.ID] = rp.Config
					}
				}
			}
		}
	}

	var policies []EventGatewayClusterPolicyInfo
	for _, p := range res.ListClusterPoliciesResponse {
		normalized := p.Labels
		if normalized == nil {
			normalized = make(map[string]string)
		}
		policies = append(policies, EventGatewayClusterPolicyInfo{
			EventGatewayPolicy: p,
			NormalizedLabels:   normalized,
			RawConfig:          rawConfigByID[p.ID],
		})
	}

	return policies, nil
}

func (c *Client) CreateEventGatewayClusterPolicy(
	ctx context.Context,
	gatewayID string,
	virtualClusterID string,
	req kkComps.EventGatewayClusterPolicyModify,
	namespace string,
) (string, error) {
	createReq := kkOps.CreateEventGatewayVirtualClusterClusterLevelPolicyRequest{
		GatewayID:                       gatewayID,
		VirtualClusterID:                virtualClusterID,
		EventGatewayClusterPolicyModify: &req,
	}

	resp, err := c.eventGatewayClusterPolicyAPI.CreateEventGatewayVirtualClusterClusterLevelPolicy(ctx, createReq)
	if err != nil {
		return "", WrapAPIError(err, "create event gateway cluster policy", &ErrorWrapperOptions{
			ResourceType: "event_gateway_virtual_cluster_cluster_policy",
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if err := ValidateResponse(resp.EventGatewayPolicy, "create event gateway cluster policy"); err != nil {
		return "", err
	}

	return resp.EventGatewayPolicy.ID, nil
}

func (c *Client) UpdateEventGatewayClusterPolicy(
	ctx context.Context,
	gatewayID string,
	virtualClusterID string,
	policyID string,
	req kkComps.EventGatewayClusterPolicyModify,
	namespace string,
) (string, error) {
	updateReq := kkOps.UpdateEventGatewayVirtualClusterClusterLevelPolicyRequest{
		GatewayID:                       gatewayID,
		VirtualClusterID:                virtualClusterID,
		PolicyID:                        policyID,
		EventGatewayClusterPolicyModify: &req,
	}

	resp, err := c.eventGatewayClusterPolicyAPI.UpdateEventGatewayVirtualClusterClusterLevelPolicy(ctx, updateReq)
	if err != nil {
		return "", WrapAPIError(err, "update event gateway cluster policy", &ErrorWrapperOptions{
			ResourceType: "event_gateway_virtual_cluster_cluster_policy",
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	return resp.EventGatewayPolicy.ID, nil
}

func (c *Client) DeleteEventGatewayClusterPolicy(
	ctx context.Context,
	gatewayID string,
	virtualClusterID string,
	policyID string,
) error {
	deleteReq := kkOps.DeleteEventGatewayVirtualClusterClusterLevelPolicyRequest{
		GatewayID:        gatewayID,
		VirtualClusterID: virtualClusterID,
		PolicyID:         policyID,
	}

	_, err := c.eventGatewayClusterPolicyAPI.DeleteEventGatewayVirtualClusterClusterLevelPolicy(ctx, deleteReq)
	if err != nil {
		return WrapAPIError(err, "delete event gateway cluster policy", nil)
	}
	return nil
}

func (c *Client) GetEventGatewayClusterPolicy(
	ctx context.Context,
	gatewayID string,
	virtualClusterID string,
	policyID string,
) (*EventGatewayClusterPolicyInfo, error) {
	req := kkOps.GetEventGatewayVirtualClusterClusterLevelPolicyRequest{
		GatewayID:        gatewayID,
		VirtualClusterID: virtualClusterID,
		PolicyID:         policyID,
	}

	resp, err := c.eventGatewayClusterPolicyAPI.GetEventGatewayVirtualClusterClusterLevelPolicy(ctx, req)
	if err != nil {
		return nil, WrapAPIError(err, "get event gateway cluster policy", nil)
	}

	if resp.EventGatewayPolicy == nil {
		return nil, nil
	}

	normalized := resp.EventGatewayPolicy.Labels
	if normalized == nil {
		normalized = make(map[string]string)
	}

	return &EventGatewayClusterPolicyInfo{
		EventGatewayPolicy: *resp.EventGatewayPolicy,
		NormalizedLabels:   normalized,
	}, nil
}

// ---- Event Gateway Virtual Cluster Produce Policy operations ----

// EventGatewayVirtualClusterProducePolicyInfo wraps a Produce Policy for internal use.
type EventGatewayVirtualClusterProducePolicyInfo struct {
	kkComps.EventGatewayPolicy
	RawConfig map[string]any
}

// producePolicyRawResponse is used to parse the raw API response to get full config.
type producePolicyRawResponse struct {
	Type   string         `json:"type"`
	Name   *string        `json:"name,omitempty"`
	ID     string         `json:"id"`
	Config map[string]any `json:"config"`
}

func (c *Client) ListEventGatewayVirtualClusterProducePolicies(
	ctx context.Context,
	gatewayID string,
	virtualClusterID string,
) ([]EventGatewayVirtualClusterProducePolicyInfo, error) {
	if err := ValidateAPIClient(c.eventGatewayProducePolicyAPI, "event gateway produce policy API"); err != nil {
		return nil, err
	}

	req := kkOps.ListEventGatewayVirtualClusterProducePoliciesRequest{
		GatewayID:        gatewayID,
		VirtualClusterID: virtualClusterID,
	}

	res, err := c.eventGatewayProducePolicyAPI.ListEventGatewayVirtualClusterProducePolicies(ctx, req)
	if err != nil {
		return nil, WrapAPIError(err, "list event gateway virtual cluster produce policies", nil)
	}

	if res.ListProducePoliciesResponse == nil {
		return []EventGatewayVirtualClusterProducePolicyInfo{}, nil
	}

	// Try to parse raw response to get full config data
	rawConfigByID := make(map[string]map[string]any)
	if res.RawResponse != nil && res.RawResponse.Body != nil {
		bodyBytes, readErr := io.ReadAll(res.RawResponse.Body)
		if readErr == nil && len(bodyBytes) > 0 {
			var rawPolicies []producePolicyRawResponse
			if jsonErr := json.Unmarshal(bodyBytes, &rawPolicies); jsonErr == nil {
				for _, rp := range rawPolicies {
					if rp.ID != "" && rp.Config != nil {
						rawConfigByID[rp.ID] = rp.Config
					}
				}
			}
		}
	}

	var policies []EventGatewayVirtualClusterProducePolicyInfo
	for _, p := range res.ListProducePoliciesResponse {
		policies = append(policies, EventGatewayVirtualClusterProducePolicyInfo{
			EventGatewayPolicy: p,
			RawConfig:          rawConfigByID[p.ID],
		})
	}

	return policies, nil
}

func (c *Client) CreateEventGatewayVirtualClusterProducePolicy(
	ctx context.Context,
	gatewayID string,
	virtualClusterID string,
	req kkComps.EventGatewayProducePolicyCreate,
	namespace string,
) (string, error) {
	createReq := kkOps.CreateEventGatewayVirtualClusterProducePolicyRequest{
		GatewayID:                       gatewayID,
		VirtualClusterID:                virtualClusterID,
		EventGatewayProducePolicyCreate: &req,
	}

	resp, err := c.eventGatewayProducePolicyAPI.CreateEventGatewayVirtualClusterProducePolicy(ctx, createReq)
	if err != nil {
		name := extractProducePolicyCreateName(req)
		return "", WrapAPIError(err, "create event gateway virtual cluster produce policy", &ErrorWrapperOptions{
			ResourceType: "event_gateway_virtual_cluster_produce_policy",
			ResourceName: name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if err := ValidateResponse(
		resp.EventGatewayPolicy, "create event gateway virtual cluster produce policy",
	); err != nil {
		return "", err
	}

	return resp.EventGatewayPolicy.ID, nil
}

func (c *Client) UpdateEventGatewayVirtualClusterProducePolicy(
	ctx context.Context,
	gatewayID string,
	virtualClusterID string,
	policyID string,
	req kkComps.EventGatewayProducePolicyUpdate,
	namespace string,
) (string, error) {
	updateReq := kkOps.UpdateEventGatewayVirtualClusterProducePolicyRequest{
		GatewayID:                       gatewayID,
		VirtualClusterID:                virtualClusterID,
		PolicyID:                        policyID,
		EventGatewayProducePolicyUpdate: &req,
	}

	resp, err := c.eventGatewayProducePolicyAPI.UpdateEventGatewayVirtualClusterProducePolicy(ctx, updateReq)
	if err != nil {
		name := extractProducePolicyUpdateName(req)
		return "", WrapAPIError(err, "update event gateway virtual cluster produce policy", &ErrorWrapperOptions{
			ResourceType: "event_gateway_virtual_cluster_produce_policy",
			ResourceName: name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	return resp.EventGatewayPolicy.ID, nil
}

func (c *Client) DeleteEventGatewayVirtualClusterProducePolicy(
	ctx context.Context,
	gatewayID string,
	virtualClusterID string,
	policyID string,
) error {
	deleteReq := kkOps.DeleteEventGatewayVirtualClusterProducePolicyRequest{
		GatewayID:        gatewayID,
		VirtualClusterID: virtualClusterID,
		PolicyID:         policyID,
	}

	_, err := c.eventGatewayProducePolicyAPI.DeleteEventGatewayVirtualClusterProducePolicy(ctx, deleteReq)
	if err != nil {
		return WrapAPIError(err, "delete event gateway virtual cluster produce policy", nil)
	}
	return nil
}

// extractProducePolicyCreateName extracts the policy name from the union create type.
func extractProducePolicyCreateName(req kkComps.EventGatewayProducePolicyCreate) string {
	if req.EventGatewayModifyHeadersPolicyCreate != nil && req.EventGatewayModifyHeadersPolicyCreate.Name != nil {
		return *req.EventGatewayModifyHeadersPolicyCreate.Name
	}
	if req.EventGatewayProduceSchemaValidationPolicy != nil &&
		req.EventGatewayProduceSchemaValidationPolicy.Name != nil {
		return *req.EventGatewayProduceSchemaValidationPolicy.Name
	}
	if req.EventGatewayEncryptPolicy != nil && req.EventGatewayEncryptPolicy.Name != nil {
		return *req.EventGatewayEncryptPolicy.Name
	}
	return ""
}

// extractProducePolicyUpdateName extracts the policy name from the union update type.
func extractProducePolicyUpdateName(req kkComps.EventGatewayProducePolicyUpdate) string {
	if req.EventGatewayModifyHeadersPolicy != nil && req.EventGatewayModifyHeadersPolicy.Name != nil {
		return *req.EventGatewayModifyHeadersPolicy.Name
	}
	if req.EventGatewayProduceSchemaValidationPolicy != nil &&
		req.EventGatewayProduceSchemaValidationPolicy.Name != nil {
		return *req.EventGatewayProduceSchemaValidationPolicy.Name
	}
	if req.EventGatewayEncryptPolicy != nil && req.EventGatewayEncryptPolicy.Name != nil {
		return *req.EventGatewayEncryptPolicy.Name
	}
	return ""
}

func (c *Client) GetEventGatewayVirtualClusterProducePolicy(
	ctx context.Context,
	gatewayID string,
	virtualClusterID string,
	policyID string,
) (*EventGatewayVirtualClusterProducePolicyInfo, error) {
	req := kkOps.GetEventGatewayVirtualClusterProducePolicyRequest{
		GatewayID:        gatewayID,
		VirtualClusterID: virtualClusterID,
		PolicyID:         policyID,
	}

	resp, err := c.eventGatewayProducePolicyAPI.GetEventGatewayVirtualClusterProducePolicy(ctx, req)
	if err != nil {
		return nil, WrapAPIError(err, "get event gateway virtual cluster produce policy", nil)
	}

	if resp.EventGatewayPolicy == nil {
		return nil, nil
	}

	return &EventGatewayVirtualClusterProducePolicyInfo{
		EventGatewayPolicy: *resp.EventGatewayPolicy,
		RawConfig:          nil,
	}, nil
}

// ---- Event Gateway Consume Policy operations ----

// EventGatewayConsumePolicyInfo wraps an Event Gateway Consume Policy for internal use.
// RawConfig contains the full config from the raw API response since the SDK's
// EventGatewayPolicyConfig struct is empty and doesn't capture actual config data.
type EventGatewayConsumePolicyInfo struct {
	kkComps.EventGatewayPolicy
	NormalizedLabels map[string]string
	RawConfig        map[string]any
}

// consumePolicyRawResponse is used to parse the raw API response to get full config.
type consumePolicyRawResponse struct {
	Type           string            `json:"type"`
	Name           *string           `json:"name,omitempty"`
	Description    *string           `json:"description,omitempty"`
	Enabled        *bool             `json:"enabled,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	ID             string            `json:"id"`
	Config         map[string]any    `json:"config"`
	CreatedAt      string            `json:"created_at"`
	UpdatedAt      string            `json:"updated_at"`
	ParentPolicyID *string           `json:"parent_policy_id,omitempty"`
	Condition      *string           `json:"condition,omitempty"`
}

func (c *Client) ListEventGatewayConsumePolicies(
	ctx context.Context,
	gatewayID string,
	virtualClusterID string,
) ([]EventGatewayConsumePolicyInfo, error) {
	if err := ValidateAPIClient(c.eventGatewayConsumePolicyAPI, "event gateway consume policy API"); err != nil {
		return nil, err
	}

	req := kkOps.ListEventGatewayVirtualClusterConsumePoliciesRequest{
		GatewayID:        gatewayID,
		VirtualClusterID: virtualClusterID,
	}

	res, err := c.eventGatewayConsumePolicyAPI.ListEventGatewayVirtualClusterConsumePolicies(ctx, req)
	if err != nil {
		return nil, WrapAPIError(err, "list event gateway consume policies", nil)
	}

	if res.ListConsumePoliciesResponse == nil {
		return []EventGatewayConsumePolicyInfo{}, nil
	}

	// Try to parse raw response to get full config data
	rawConfigByID := make(map[string]map[string]any)
	if res.RawResponse != nil && res.RawResponse.Body != nil {
		bodyBytes, readErr := io.ReadAll(res.RawResponse.Body)
		if readErr == nil && len(bodyBytes) > 0 {
			var rawPolicies []consumePolicyRawResponse
			if jsonErr := json.Unmarshal(bodyBytes, &rawPolicies); jsonErr == nil {
				for _, rp := range rawPolicies {
					if rp.ID != "" && rp.Config != nil {
						rawConfigByID[rp.ID] = rp.Config
					}
				}
			}
		}
	}

	var policies []EventGatewayConsumePolicyInfo
	for _, p := range res.ListConsumePoliciesResponse {
		normalized := p.Labels
		if normalized == nil {
			normalized = make(map[string]string)
		}
		policies = append(policies, EventGatewayConsumePolicyInfo{
			EventGatewayPolicy: p,
			NormalizedLabels:   normalized,
			RawConfig:          rawConfigByID[p.ID],
		})
	}

	return policies, nil
}

func (c *Client) CreateEventGatewayConsumePolicy(
	ctx context.Context,
	gatewayID string,
	virtualClusterID string,
	req kkComps.EventGatewayConsumePolicyCreate,
	namespace string,
) (string, error) {
	createReq := kkOps.CreateEventGatewayVirtualClusterConsumePolicyRequest{
		GatewayID:                       gatewayID,
		VirtualClusterID:                virtualClusterID,
		EventGatewayConsumePolicyCreate: &req,
	}

	resp, err := c.eventGatewayConsumePolicyAPI.CreateEventGatewayVirtualClusterConsumePolicy(ctx, createReq)
	if err != nil {
		return "", WrapAPIError(err, "create event gateway consume policy", &ErrorWrapperOptions{
			ResourceType: "event_gateway_virtual_cluster_consume_policy",
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if err := ValidateResponse(resp.EventGatewayPolicy, "create event gateway consume policy"); err != nil {
		return "", err
	}

	return resp.EventGatewayPolicy.ID, nil
}

func (c *Client) UpdateEventGatewayConsumePolicy(
	ctx context.Context,
	gatewayID string,
	virtualClusterID string,
	policyID string,
	req kkComps.EventGatewayConsumePolicyUpdate,
	namespace string,
) (string, error) {
	updateReq := kkOps.UpdateEventGatewayVirtualClusterConsumePolicyRequest{
		GatewayID:                       gatewayID,
		VirtualClusterID:                virtualClusterID,
		PolicyID:                        policyID,
		EventGatewayConsumePolicyUpdate: &req,
	}

	resp, err := c.eventGatewayConsumePolicyAPI.UpdateEventGatewayVirtualClusterConsumePolicy(ctx, updateReq)
	if err != nil {
		return "", WrapAPIError(err, "update event gateway consume policy", &ErrorWrapperOptions{
			ResourceType: "event_gateway_virtual_cluster_consume_policy",
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	return resp.EventGatewayPolicy.ID, nil
}

func (c *Client) DeleteEventGatewayConsumePolicy(
	ctx context.Context,
	gatewayID string,
	virtualClusterID string,
	policyID string,
) error {
	deleteReq := kkOps.DeleteEventGatewayVirtualClusterConsumePolicyRequest{
		GatewayID:        gatewayID,
		VirtualClusterID: virtualClusterID,
		PolicyID:         policyID,
	}

	_, err := c.eventGatewayConsumePolicyAPI.DeleteEventGatewayVirtualClusterConsumePolicy(ctx, deleteReq)
	if err != nil {
		return WrapAPIError(err, "delete event gateway consume policy", nil)
	}
	return nil
}

func (c *Client) GetEventGatewayConsumePolicy(
	ctx context.Context,
	gatewayID string,
	virtualClusterID string,
	policyID string,
) (*EventGatewayConsumePolicyInfo, error) {
	req := kkOps.GetEventGatewayVirtualClusterConsumePolicyRequest{
		GatewayID:        gatewayID,
		VirtualClusterID: virtualClusterID,
		PolicyID:         policyID,
	}

	resp, err := c.eventGatewayConsumePolicyAPI.GetEventGatewayVirtualClusterConsumePolicy(ctx, req)
	if err != nil {
		return nil, WrapAPIError(err, "get event gateway consume policy", nil)
	}

	if resp.EventGatewayPolicy == nil {
		return nil, nil
	}

	normalized := resp.EventGatewayPolicy.Labels
	if normalized == nil {
		normalized = make(map[string]string)
	}

	return &EventGatewayConsumePolicyInfo{
		EventGatewayPolicy: *resp.EventGatewayPolicy,
		NormalizedLabels:   normalized,
	}, nil
}

// Event Gateway Data Plane Certificate Methods

func (c *Client) ListEventGatewayDataPlaneCertificates(
	ctx context.Context,
	gatewayID string,
) ([]EventGatewayDataPlaneCertificate, error) {
	// Validate API client is initialized
	if err := ValidateAPIClient(c.eventGatewayDataPlaneCertificateAPI,
		"event gateway data plane certificate API"); err != nil {
		return nil, err
	}

	var allData []kkComps.EventGatewayDataPlaneCertificate
	var pageAfter *string

	for {
		req := kkOps.ListEventGatewayDataPlaneCertificatesRequest{
			GatewayID: gatewayID,
		}

		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := c.eventGatewayDataPlaneCertificateAPI.ListEventGatewayDataPlaneCertificates(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list event gateway data plane certificates", nil)
		}

		// If response is nil, break the loop
		if res.ListEventGatewayDataPlaneCertificatesResponse == nil {
			return []EventGatewayDataPlaneCertificate{}, nil
		}

		allData = append(allData, res.ListEventGatewayDataPlaneCertificatesResponse.Data...)

		if res.ListEventGatewayDataPlaneCertificatesResponse.Meta == nil ||
			res.ListEventGatewayDataPlaneCertificatesResponse.Meta.Page.Next == nil {
			break
		}

		u, err := url.Parse(*res.ListEventGatewayDataPlaneCertificatesResponse.Meta.Page.Next)
		if err != nil {
			return nil, WrapAPIError(err, "list event gateway data plane certificates: invalid cursor", nil)
		}

		values := u.Query()
		pageAfter = new(values.Get("page[after]"))
	}

	var certs []EventGatewayDataPlaneCertificate
	for _, cert := range allData {
		certs = append(certs, EventGatewayDataPlaneCertificate{
			EventGatewayDataPlaneCertificate: cert,
		})
	}

	return certs, nil
}

func (c *Client) CreateEventGatewayDataPlaneCertificate(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateEventGatewayDataPlaneCertificateRequest,
	namespace string,
) (string, error) {
	resp, err := c.eventGatewayDataPlaneCertificateAPI.CreateEventGatewayDataPlaneCertificate(ctx, gatewayID, req)
	if err != nil {
		name := ""
		if req.Name != nil {
			name = *req.Name
		}
		return "", WrapAPIError(err, "create event gateway data plane certificate", &ErrorWrapperOptions{
			ResourceType: "event_gateway_data_plane_certificate",
			ResourceName: name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if err := ValidateResponse(resp.EventGatewayDataPlaneCertificate,
		"create event gateway data plane certificate"); err != nil {
		return "", err
	}

	return resp.EventGatewayDataPlaneCertificate.ID, nil
}

func (c *Client) GetEventGatewayDataPlaneCertificate(
	ctx context.Context,
	gatewayID string,
	certificateID string,
) (*EventGatewayDataPlaneCertificate, error) {
	resp, err := c.eventGatewayDataPlaneCertificateAPI.FetchEventGatewayDataPlaneCertificate(
		ctx, gatewayID, certificateID)
	if err != nil {
		return nil, WrapAPIError(err, "get event gateway data plane certificate by ID", &ErrorWrapperOptions{
			ResourceType: "event_gateway_data_plane_certificate",
			UseEnhanced:  true,
		})
	}

	if resp.EventGatewayDataPlaneCertificate == nil {
		return nil, nil
	}

	return &EventGatewayDataPlaneCertificate{
		EventGatewayDataPlaneCertificate: *resp.EventGatewayDataPlaneCertificate,
	}, nil
}

func (c *Client) UpdateEventGatewayDataPlaneCertificate(
	ctx context.Context,
	gatewayID string,
	certificateID string,
	req kkComps.UpdateEventGatewayDataPlaneCertificateRequest,
	namespace string,
) (string, error) {
	resp, err := c.eventGatewayDataPlaneCertificateAPI.UpdateEventGatewayDataPlaneCertificate(
		ctx, gatewayID, certificateID, req)
	if err != nil {
		name := ""
		if req.Name != nil {
			name = *req.Name
		}
		return "", WrapAPIError(err, "update event gateway data plane certificate", &ErrorWrapperOptions{
			ResourceType: "event_gateway_data_plane_certificate",
			ResourceName: name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	return resp.EventGatewayDataPlaneCertificate.ID, nil
}

func (c *Client) DeleteEventGatewayDataPlaneCertificate(
	ctx context.Context,
	gatewayID string,
	certificateID string,
) error {
	_, err := c.eventGatewayDataPlaneCertificateAPI.DeleteEventGatewayDataPlaneCertificate(
		ctx, gatewayID, certificateID)
	if err != nil {
		return WrapAPIError(err, "delete event gateway data plane certificate", nil)
	}
	return nil
}

// ListEventGatewaySchemaRegistries returns all schema registries for a given event gateway.
// It uses cursor-based pagination to retrieve all pages.
func (c *Client) ListEventGatewaySchemaRegistries(
	ctx context.Context,
	gatewayID string,
) ([]EventGatewaySchemaRegistry, error) {
	if err := ValidateAPIClient(c.eventGatewaySchemaRegistryAPI, "event gateway schema registry API"); err != nil {
		return nil, err
	}

	var allData []kkComps.SchemaRegistry
	var rawConfigByID map[string]map[string]any
	var pageAfter *string

	for {
		req := kkOps.ListEventGatewaySchemaRegistriesRequest{
			GatewayID: gatewayID,
		}

		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := c.eventGatewaySchemaRegistryAPI.ListEventGatewaySchemaRegistries(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list event gateway schema registries", nil)
		}

		if res.ListSchemaRegistriesResponse == nil {
			return []EventGatewaySchemaRegistry{}, nil
		}

		// Try to parse raw response to get full config data (SDK config struct is opaque)
		if rawConfigByID == nil {
			rawConfigByID = make(map[string]map[string]any)
		}
		if res.RawResponse != nil && res.RawResponse.Body != nil {
			bodyBytes, readErr := io.ReadAll(res.RawResponse.Body)
			if readErr == nil && len(bodyBytes) > 0 {
				var rawResp struct {
					Data []struct {
						ID     string         `json:"id"`
						Config map[string]any `json:"config"`
					} `json:"data"`
				}
				if jsonErr := json.Unmarshal(bodyBytes, &rawResp); jsonErr == nil {
					for _, item := range rawResp.Data {
						if item.ID != "" && item.Config != nil {
							rawConfigByID[item.ID] = item.Config
						}
					}
				}
			}
		}

		allData = append(allData, res.ListSchemaRegistriesResponse.Data...)

		if res.ListSchemaRegistriesResponse.Meta == nil ||
			res.ListSchemaRegistriesResponse.Meta.Page.Next == nil {
			break
		}

		u, err := url.Parse(*res.ListSchemaRegistriesResponse.Meta.Page.Next)
		if err != nil {
			return nil, WrapAPIError(err, "list event gateway schema registries: invalid cursor", nil)
		}

		values := u.Query()
		pageAfter = new(values.Get("page[after]"))
	}

	registries := make([]EventGatewaySchemaRegistry, 0, len(allData))
	for _, sr := range allData {
		normalized := sr.Labels
		if normalized == nil {
			normalized = make(map[string]string)
		}
		registries = append(registries, EventGatewaySchemaRegistry{
			SchemaRegistry:   sr,
			NormalizedLabels: normalized,
			RawConfig:        rawConfigByID[sr.ID],
		})
	}

	return registries, nil
}

// GetEventGatewaySchemaRegistryByID retrieves a single schema registry by ID.
func (c *Client) GetEventGatewaySchemaRegistryByID(
	ctx context.Context,
	gatewayID string,
	schemaRegistryID string,
) (*EventGatewaySchemaRegistry, error) {
	if err := ValidateAPIClient(c.eventGatewaySchemaRegistryAPI, "event gateway schema registry API"); err != nil {
		return nil, err
	}

	resp, err := c.eventGatewaySchemaRegistryAPI.GetEventGatewaySchemaRegistry(ctx, gatewayID, schemaRegistryID)
	if err != nil {
		return nil, WrapAPIError(err, "get event gateway schema registry", nil)
	}

	if resp.SchemaRegistry == nil {
		return nil, nil
	}

	normalized := resp.SchemaRegistry.Labels
	if normalized == nil {
		normalized = make(map[string]string)
	}

	return &EventGatewaySchemaRegistry{
		SchemaRegistry:   *resp.SchemaRegistry,
		NormalizedLabels: normalized,
	}, nil
}

// GetEventGatewaySchemaRegistryByName looks up a schema registry by name.
func (c *Client) GetEventGatewaySchemaRegistryByName(
	ctx context.Context,
	gatewayID string,
	name string,
) (*EventGatewaySchemaRegistry, error) {
	registries, err := c.ListEventGatewaySchemaRegistries(ctx, gatewayID)
	if err != nil {
		return nil, err
	}

	for i := range registries {
		if registries[i].Name == name {
			return &registries[i], nil
		}
	}

	return nil, nil
}

// CreateEventGatewaySchemaRegistry creates a new schema registry for an event gateway.
func (c *Client) CreateEventGatewaySchemaRegistry(
	ctx context.Context,
	gatewayID string,
	req kkComps.SchemaRegistryCreate,
	_ string, // namespace (not applicable to schema registry, which has no management labels)
) (string, error) {
	resp, err := c.eventGatewaySchemaRegistryAPI.CreateEventGatewaySchemaRegistry(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(err, "create event gateway schema registry", nil)
	}

	if err := ValidateResponse(resp.SchemaRegistry, "create event gateway schema registry"); err != nil {
		return "", err
	}

	return resp.SchemaRegistry.ID, nil
}

// UpdateEventGatewaySchemaRegistry updates an existing schema registry.
func (c *Client) UpdateEventGatewaySchemaRegistry(
	ctx context.Context,
	gatewayID string,
	schemaRegistryID string,
	req kkComps.SchemaRegistryUpdate,
	_ string, // namespace
) (string, error) {
	resp, err := c.eventGatewaySchemaRegistryAPI.UpdateEventGatewaySchemaRegistry(
		ctx, gatewayID, schemaRegistryID, req)
	if err != nil {
		return "", WrapAPIError(err, "update event gateway schema registry", nil)
	}

	if err := ValidateResponse(resp.SchemaRegistry, "update event gateway schema registry"); err != nil {
		return "", err
	}

	return resp.SchemaRegistry.ID, nil
}

// DeleteEventGatewaySchemaRegistry deletes a schema registry by ID.
func (c *Client) DeleteEventGatewaySchemaRegistry(
	ctx context.Context,
	gatewayID string,
	schemaRegistryID string,
) error {
	_, err := c.eventGatewaySchemaRegistryAPI.DeleteEventGatewaySchemaRegistry(ctx, gatewayID, schemaRegistryID)
	if err != nil {
		return WrapAPIError(err, "delete event gateway schema registry", nil)
	}
	return nil
}

// EventGatewayStaticKey represents an event gateway static key for internal use.
type EventGatewayStaticKey struct {
	kkComps.EventGatewayStaticKey
}

// ListEventGatewayStaticKeys lists all static keys for a gateway using cursor-based pagination.
func (c *Client) ListEventGatewayStaticKeys(
	ctx context.Context,
	gatewayID string,
) ([]EventGatewayStaticKey, error) {
	if err := ValidateAPIClient(c.eventGatewayStaticKeyAPI, "event gateway static key API"); err != nil {
		return nil, err
	}

	var allData []kkComps.EventGatewayStaticKey
	var pageAfter *string

	for {
		req := kkOps.ListEventGatewayStaticKeysRequest{
			GatewayID: gatewayID,
		}

		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := c.eventGatewayStaticKeyAPI.ListEventGatewayStaticKeys(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list event gateway static keys", nil)
		}

		if res.ListEventGatewayStaticKeysResponse == nil {
			return []EventGatewayStaticKey{}, nil
		}

		allData = append(allData, res.ListEventGatewayStaticKeysResponse.Data...)

		if res.ListEventGatewayStaticKeysResponse.Meta == nil ||
			res.ListEventGatewayStaticKeysResponse.Meta.Page.Next == nil {
			break
		}

		u, err := url.Parse(*res.ListEventGatewayStaticKeysResponse.Meta.Page.Next)
		if err != nil {
			return nil, WrapAPIError(err, "list event gateway static keys: invalid cursor", nil)
		}

		values := u.Query()
		after := values.Get("page[after]")
		pageAfter = &after
	}

	result := make([]EventGatewayStaticKey, 0, len(allData))
	for _, sk := range allData {
		result = append(result, EventGatewayStaticKey{EventGatewayStaticKey: sk})
	}

	return result, nil
}

// CreateEventGatewayStaticKey creates a new static key for an event gateway.
func (c *Client) CreateEventGatewayStaticKey(
	ctx context.Context,
	gatewayID string,
	req kkComps.EventGatewayStaticKeyCreate,
	_ string, // namespace
) (string, error) {
	if err := ValidateAPIClient(c.eventGatewayStaticKeyAPI, "event gateway static key API"); err != nil {
		return "", err
	}

	createReq := kkOps.CreateEventGatewayStaticKeyRequest{
		GatewayID:                   gatewayID,
		EventGatewayStaticKeyCreate: &req,
	}

	resp, err := c.eventGatewayStaticKeyAPI.CreateEventGatewayStaticKey(ctx, createReq)
	if err != nil {
		return "", WrapAPIError(err, "create event gateway static key", &ErrorWrapperOptions{
			ResourceType: "event_gateway_static_key",
			ResourceName: req.Name,
			UseEnhanced:  true,
		})
	}

	if err := ValidateResponse(resp.EventGatewayStaticKey, "create event gateway static key"); err != nil {
		return "", err
	}

	return resp.EventGatewayStaticKey.ID, nil
}

// GetEventGatewayStaticKey retrieves a static key by ID.
func (c *Client) GetEventGatewayStaticKey(
	ctx context.Context,
	gatewayID string,
	staticKeyID string,
) (*EventGatewayStaticKey, error) {
	if err := ValidateAPIClient(c.eventGatewayStaticKeyAPI, "event gateway static key API"); err != nil {
		return nil, err
	}

	req := kkOps.GetEventGatewayStaticKeyRequest{
		GatewayID:   gatewayID,
		StaticKeyID: staticKeyID,
	}

	resp, err := c.eventGatewayStaticKeyAPI.GetEventGatewayStaticKey(ctx, req)
	if err != nil {
		return nil, WrapAPIError(err, "get event gateway static key", &ErrorWrapperOptions{
			ResourceType: "event_gateway_static_key",
			UseEnhanced:  true,
		})
	}

	if resp.EventGatewayStaticKey == nil {
		return nil, nil
	}

	return &EventGatewayStaticKey{EventGatewayStaticKey: *resp.EventGatewayStaticKey}, nil
}

// DeleteEventGatewayStaticKey deletes a static key by ID.
func (c *Client) DeleteEventGatewayStaticKey(
	ctx context.Context,
	gatewayID string,
	staticKeyID string,
) error {
	if err := ValidateAPIClient(c.eventGatewayStaticKeyAPI, "event gateway static key API"); err != nil {
		return err
	}

	deleteReq := kkOps.DeleteEventGatewayStaticKeyRequest{
		GatewayID:   gatewayID,
		StaticKeyID: staticKeyID,
	}

	_, err := c.eventGatewayStaticKeyAPI.DeleteEventGatewayStaticKey(ctx, deleteReq)
	if err != nil {
		return WrapAPIError(err, "delete event gateway static key", nil)
	}

	return nil
}

// EventGatewayTLSTrustBundle represents an event gateway TLS trust bundle for internal use.
type EventGatewayTLSTrustBundle struct {
	kkComps.TLSTrustBundle
	NormalizedLabels map[string]string
}

// ListEventGatewayTLSTrustBundles lists all TLS trust bundles for a gateway using cursor-based pagination.
func (c *Client) ListEventGatewayTLSTrustBundles(
	ctx context.Context,
	gatewayID string,
) ([]EventGatewayTLSTrustBundle, error) {
	if err := ValidateAPIClient(c.eventGatewayTLSTrustBundleAPI, "event gateway TLS trust bundle API"); err != nil {
		return nil, err
	}

	var allData []kkComps.TLSTrustBundle
	var pageAfter *string

	for {
		req := kkOps.ListEventGatewayTLSTrustBundlesRequest{
			GatewayID: gatewayID,
		}

		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := c.eventGatewayTLSTrustBundleAPI.ListEventGatewayTLSTrustBundles(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list event gateway TLS trust bundles", nil)
		}

		if res.ListTLSTrustBundlesResponse == nil {
			return []EventGatewayTLSTrustBundle{}, nil
		}

		allData = append(allData, res.ListTLSTrustBundlesResponse.Data...)

		if res.ListTLSTrustBundlesResponse.Meta == nil ||
			res.ListTLSTrustBundlesResponse.Meta.Page.Next == nil {
			break
		}

		u, err := url.Parse(*res.ListTLSTrustBundlesResponse.Meta.Page.Next)
		if err != nil {
			return nil, WrapAPIError(err, "list event gateway TLS trust bundles: invalid cursor", nil)
		}

		values := u.Query()
		after := values.Get("page[after]")
		pageAfter = &after
	}

	result := make([]EventGatewayTLSTrustBundle, 0, len(allData))
	for _, tb := range allData {
		result = append(result, EventGatewayTLSTrustBundle{
			TLSTrustBundle:   tb,
			NormalizedLabels: tb.Labels,
		})
	}

	return result, nil
}

// CreateEventGatewayTLSTrustBundle creates a new TLS trust bundle for an event gateway.
func (c *Client) CreateEventGatewayTLSTrustBundle(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateTLSTrustBundleRequest,
	_ string, // namespace
) (string, error) {
	if err := ValidateAPIClient(c.eventGatewayTLSTrustBundleAPI, "event gateway TLS trust bundle API"); err != nil {
		return "", err
	}

	createReq := kkOps.CreateEventGatewayTLSTrustBundleRequest{
		GatewayID:                   gatewayID,
		CreateTLSTrustBundleRequest: req,
	}

	resp, err := c.eventGatewayTLSTrustBundleAPI.CreateEventGatewayTLSTrustBundle(ctx, createReq)
	if err != nil {
		return "", WrapAPIError(err, "create event gateway TLS trust bundle", &ErrorWrapperOptions{
			ResourceType: "event_gateway_tls_trust_bundle",
			ResourceName: req.Name,
			UseEnhanced:  true,
		})
	}

	if err := ValidateResponse(resp.TLSTrustBundle, "create event gateway TLS trust bundle"); err != nil {
		return "", err
	}

	return resp.TLSTrustBundle.ID, nil
}

// GetEventGatewayTLSTrustBundle retrieves a TLS trust bundle by ID.
func (c *Client) GetEventGatewayTLSTrustBundle(
	ctx context.Context,
	gatewayID string,
	bundleID string,
) (*EventGatewayTLSTrustBundle, error) {
	if err := ValidateAPIClient(c.eventGatewayTLSTrustBundleAPI, "event gateway TLS trust bundle API"); err != nil {
		return nil, err
	}

	req := kkOps.GetEventGatewayTLSTrustBundleRequest{
		GatewayID:        gatewayID,
		TLSTrustBundleID: bundleID,
	}

	resp, err := c.eventGatewayTLSTrustBundleAPI.GetEventGatewayTLSTrustBundle(ctx, req)
	if err != nil {
		return nil, WrapAPIError(err, "get event gateway TLS trust bundle", &ErrorWrapperOptions{
			ResourceType: "event_gateway_tls_trust_bundle",
			UseEnhanced:  true,
		})
	}

	if resp.TLSTrustBundle == nil {
		return nil, nil
	}

	return &EventGatewayTLSTrustBundle{
		TLSTrustBundle:   *resp.TLSTrustBundle,
		NormalizedLabels: resp.TLSTrustBundle.Labels,
	}, nil
}

// UpdateEventGatewayTLSTrustBundle updates a TLS trust bundle.
func (c *Client) UpdateEventGatewayTLSTrustBundle(
	ctx context.Context,
	gatewayID string,
	bundleID string,
	req kkComps.UpdateTLSTrustBundleRequest,
	_ string, // namespace
) (string, error) {
	if err := ValidateAPIClient(c.eventGatewayTLSTrustBundleAPI, "event gateway TLS trust bundle API"); err != nil {
		return "", err
	}

	updateReq := kkOps.UpdateEventGatewayTLSTrustBundleRequest{
		GatewayID:                   gatewayID,
		TLSTrustBundleID:            bundleID,
		UpdateTLSTrustBundleRequest: req,
	}

	resp, err := c.eventGatewayTLSTrustBundleAPI.UpdateEventGatewayTLSTrustBundle(ctx, updateReq)
	if err != nil {
		return "", WrapAPIError(err, "update event gateway TLS trust bundle", &ErrorWrapperOptions{
			ResourceType: "event_gateway_tls_trust_bundle",
			UseEnhanced:  true,
		})
	}

	if err := ValidateResponse(resp.TLSTrustBundle, "update event gateway TLS trust bundle"); err != nil {
		return "", err
	}

	return resp.TLSTrustBundle.ID, nil
}

// DeleteEventGatewayTLSTrustBundle deletes a TLS trust bundle by ID.
func (c *Client) DeleteEventGatewayTLSTrustBundle(
	ctx context.Context,
	gatewayID string,
	bundleID string,
) error {
	if err := ValidateAPIClient(c.eventGatewayTLSTrustBundleAPI, "event gateway TLS trust bundle API"); err != nil {
		return err
	}

	deleteReq := kkOps.DeleteEventGatewayTLSTrustBundleRequest{
		GatewayID:        gatewayID,
		TLSTrustBundleID: bundleID,
	}

	_, err := c.eventGatewayTLSTrustBundleAPI.DeleteEventGatewayTLSTrustBundle(ctx, deleteReq)
	if err != nil {
		return WrapAPIError(err, "delete event gateway TLS trust bundle", nil)
	}

	return nil
}
