package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// PortalDomainAdapter implements ResourceOperations for portal custom domains
type PortalDomainAdapter struct {
	client *state.Client
}

// NewPortalDomainAdapter creates a new portal domain adapter
func NewPortalDomainAdapter(client *state.Client) *PortalDomainAdapter {
	return &PortalDomainAdapter{client: client}
}

// MapCreateFields maps fields to CreatePortalCustomDomainRequest
func (p *PortalDomainAdapter) MapCreateFields(_ context.Context, fields map[string]interface{},
	create *kkComps.CreatePortalCustomDomainRequest) error {
	
	// Required fields
	hostname, ok := fields["hostname"].(string)
	if !ok {
		return fmt.Errorf("hostname is required")
	}
	create.Hostname = hostname
	
	enabled, ok := fields["enabled"].(bool)
	if !ok {
		return fmt.Errorf("enabled is required")
	}
	create.Enabled = enabled
	
	// Handle SSL settings
	if sslData, ok := fields["ssl"].(map[string]interface{}); ok {
		ssl := kkComps.CreatePortalCustomDomainSSL{}
		if method, ok := sslData["domain_verification_method"].(string); ok {
			ssl.DomainVerificationMethod = kkComps.PortalCustomDomainVerificationMethod(method)
		}
		create.Ssl = ssl
	}
	
	return nil
}

// MapUpdateFields maps fields to UpdatePortalCustomDomainRequest
func (p *PortalDomainAdapter) MapUpdateFields(_ context.Context, fields map[string]interface{},
	update *kkComps.UpdatePortalCustomDomainRequest, _ map[string]string) error {
	// Only enabled field can be updated
	if enabled, ok := fields["enabled"].(bool); ok {
		update.Enabled = &enabled
	}
	
	return nil
}

// Create creates a new portal custom domain
func (p *PortalDomainAdapter) Create(ctx context.Context, req kkComps.CreatePortalCustomDomainRequest,
	_ string) (string, error) {
	// Get portal ID from parent reference
	portalID, err := p.getPortalID(ctx)
	if err != nil {
		return "", err
	}
	
	err = p.client.CreatePortalCustomDomain(ctx, portalID, req)
	if err != nil {
		return "", err
	}
	
	// Custom domain doesn't return an ID, use portal ID instead
	return portalID, nil
}

// Update updates an existing portal custom domain
func (p *PortalDomainAdapter) Update(ctx context.Context, id string, 
	req kkComps.UpdatePortalCustomDomainRequest, _ string) (string, error) {
	// For custom domains, the ID is actually the portal ID
	err := p.client.UpdatePortalCustomDomain(ctx, id, req)
	if err != nil {
		return "", err
	}
	return id, nil
}

// Delete deletes a portal custom domain
func (p *PortalDomainAdapter) Delete(ctx context.Context, id string) error {
	// For custom domains, the ID is actually the portal ID
	return p.client.DeletePortalCustomDomain(ctx, id)
}

// GetByName gets a portal custom domain by name (hostname)
func (p *PortalDomainAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	// Portal custom domains don't have a direct "get by name" method
	// They are singleton resources per portal
	// For now, return nil to indicate not found
	return nil, nil
}

// ResourceType returns the resource type name
func (p *PortalDomainAdapter) ResourceType() string {
	return "portal_custom_domain"
}

// RequiredFields returns the required fields for creation
func (p *PortalDomainAdapter) RequiredFields() []string {
	return []string{"hostname", "enabled"}
}

// SupportsUpdate returns true as custom domains support updates (enabled field only)
func (p *PortalDomainAdapter) SupportsUpdate() bool {
	return true
}

// getPortalID extracts the portal ID from the context
func (p *PortalDomainAdapter) getPortalID(ctx context.Context) (string, error) {
	// Get the planned change from context
	change, ok := ctx.Value(contextKeyPlannedChange).(planner.PlannedChange)
	if !ok {
		return "", fmt.Errorf("planned change not found in context")
	}
	
	// Get portal ID from references
	if portalRef, ok := change.References["portal_id"]; ok {
		if portalRef.ID != "" {
			return portalRef.ID, nil
		}
	}
	
	return "", fmt.Errorf("portal ID is required for custom domain")
}

// PortalDomainResourceInfo implements ResourceInfo for portal custom domains
type PortalDomainResourceInfo struct {
	portalID string
	hostname string
}

func (p *PortalDomainResourceInfo) GetID() string {
	return p.portalID
}

func (p *PortalDomainResourceInfo) GetName() string {
	return p.hostname
}

func (p *PortalDomainResourceInfo) GetLabels() map[string]string {
	// Portal custom domains don't support labels
	return make(map[string]string)
}

func (p *PortalDomainResourceInfo) GetNormalizedLabels() map[string]string {
	return make(map[string]string)
}