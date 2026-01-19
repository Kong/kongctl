package planner

import (
	"fmt"
	"sync/atomic"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// BasePlanner provides common functionality for all resource planners
type BasePlanner struct {
	planner         *Planner
	changeIDCounter *atomic.Int32
}

// NewBasePlanner creates a new base planner instance
func NewBasePlanner(p *Planner) *BasePlanner {
	return &BasePlanner{
		planner:         p,
		changeIDCounter: &atomic.Int32{},
	}
}

// NextChangeID generates a unique change ID
func (b *BasePlanner) NextChangeID(action ActionType, resourceType string, resourceRef string) string {
	return b.planner.nextChangeID(action, resourceType, resourceRef)
}

// ValidateProtection validates protection status for an operation
func (b *BasePlanner) ValidateProtection(resourceType, resourceName string, isProtected bool, action ActionType) error {
	return b.planner.validateProtection(resourceType, resourceName, isProtected, action)
}

// ValidateProtectionWithChange validates protection status for an operation with protection change info
func (b *BasePlanner) ValidateProtectionWithChange(
	resourceType, resourceName string,
	isProtected bool,
	action ActionType,
	protectionChange *ProtectionChange,
	hasOtherFieldChanges bool,
) error {
	return b.planner.validateProtectionWithChange(resourceType, resourceName, isProtected,
		action, protectionChange, hasOtherFieldChanges)
}

// GetString safely dereferences a string pointer
func (b *BasePlanner) GetString(s *string) string {
	return getString(s)
}

// GetClient returns the state client
func (b *BasePlanner) GetClient() *state.Client {
	return b.planner.client
}

// GetDesiredPortals returns desired portal resources from the specified namespace
func (b *BasePlanner) GetDesiredPortals(namespace string) []resources.PortalResource {
	return b.planner.resources.GetPortalsByNamespace(namespace)
}

// GetDesiredControlPlanes returns desired control plane resources from the specified namespace
func (b *BasePlanner) GetDesiredControlPlanes(namespace string) []resources.ControlPlaneResource {
	return b.planner.resources.GetControlPlanesByNamespace(namespace)
}

// GetDesiredCatalogServices returns desired catalog service resources from the specified namespace
func (b *BasePlanner) GetDesiredCatalogServices(namespace string) []resources.CatalogServiceResource {
	return b.planner.resources.GetCatalogServicesByNamespace(namespace)
}

// GetDesiredAuthStrategies returns desired auth strategy resources from the specified namespace
func (b *BasePlanner) GetDesiredAuthStrategies(namespace string) []resources.ApplicationAuthStrategyResource {
	return b.planner.resources.GetAuthStrategiesByNamespace(namespace)
}

// GetDesiredAPIs returns desired API resources from the specified namespace
func (b *BasePlanner) GetDesiredAPIs(namespace string) []resources.APIResource {
	return b.planner.resources.GetAPIsByNamespace(namespace)
}

// GetDesiredAPIVersions returns desired API version resources from the specified namespace
func (b *BasePlanner) GetDesiredAPIVersions(namespace string) []resources.APIVersionResource {
	return b.planner.resources.GetAPIVersionsByNamespace(namespace)
}

// GetDesiredAPIPublications returns desired API publication resources from the specified namespace
func (b *BasePlanner) GetDesiredAPIPublications(namespace string) []resources.APIPublicationResource {
	return b.planner.resources.GetAPIPublicationsByNamespace(namespace)
}

// GetDesiredAPIImplementations returns desired API implementation resources from the specified namespace
func (b *BasePlanner) GetDesiredAPIImplementations(namespace string) []resources.APIImplementationResource {
	return b.planner.resources.GetAPIImplementationsByNamespace(namespace)
}

// GetDesiredAPIDocuments returns desired API document resources from the specified namespace
func (b *BasePlanner) GetDesiredAPIDocuments(namespace string) []resources.APIDocumentResource {
	return b.planner.resources.GetAPIDocumentsByNamespace(namespace)
}

// GetDesiredPortalCustomizations returns desired portal customization resources from the specified namespace
func (b *BasePlanner) GetDesiredPortalCustomizations(namespace string) []resources.PortalCustomizationResource {
	return b.planner.resources.GetPortalCustomizationsByNamespace(namespace)
}

// GetDesiredPortalAuthSettings returns desired portal auth settings resources from the specified namespace
func (b *BasePlanner) GetDesiredPortalAuthSettings(namespace string) []resources.PortalAuthSettingsResource {
	return b.planner.resources.GetPortalAuthSettingsByNamespace(namespace)
}

// GetDesiredPortalCustomDomains returns desired portal custom domain resources from the specified namespace
func (b *BasePlanner) GetDesiredPortalCustomDomains(namespace string) []resources.PortalCustomDomainResource {
	return b.planner.resources.GetPortalCustomDomainsByNamespace(namespace)
}

// GetDesiredPortalEmailConfigs returns desired portal email config resources from the specified namespace
func (b *BasePlanner) GetDesiredPortalEmailConfigs(namespace string) []resources.PortalEmailConfigResource {
	return b.planner.resources.GetPortalEmailConfigsByNamespace(namespace)
}

// GetDesiredPortalPages returns desired portal page resources from the specified namespace
func (b *BasePlanner) GetDesiredPortalPages(namespace string) []resources.PortalPageResource {
	return b.planner.resources.GetPortalPagesByNamespace(namespace)
}

// GetDesiredPortalSnippets returns desired portal snippet resources from the specified namespace
func (b *BasePlanner) GetDesiredPortalSnippets(namespace string) []resources.PortalSnippetResource {
	return b.planner.resources.GetPortalSnippetsByNamespace(namespace)
}

// GetDesiredEventGatewayControlPlanes returns desired EGW CP resources from the specified namespace
func (b *BasePlanner) GetDesiredEventGatewayControlPlanes(
	namespace string,
) []resources.EventGatewayControlPlaneResource {
	return b.planner.resources.GetEventGatewayControlPlanesByNamespace(namespace)
}

// GetDesiredTeams returns desired team resources from the specified namespace
func (b *BasePlanner) GetDesiredTeams(namespace string) []resources.TeamResource {
	return b.planner.resources.GetTeamsByNamespace(namespace)
}

// GetGenericPlanner returns the generic planner instance
func (b *BasePlanner) GetGenericPlanner() *GenericPlanner {
	if b == nil || b.planner == nil {
		return nil
	}
	return b.planner.genericPlanner
}

// CollectProtectionErrors collects protection validation errors for batch reporting
type ProtectionErrorCollector struct {
	errors []error
}

// Add adds a protection error to the collector
func (c *ProtectionErrorCollector) Add(err error) {
	if err != nil {
		c.errors = append(c.errors, err)
	}
}

// HasErrors returns true if any errors were collected
func (c *ProtectionErrorCollector) HasErrors() bool {
	return len(c.errors) > 0
}

// Error returns a combined error message
func (c *ProtectionErrorCollector) Error() error {
	if !c.HasErrors() {
		return nil
	}

	errMsg := "Cannot generate plan due to protected resources:\n"
	for _, err := range c.errors {
		errMsg += fmt.Sprintf("- %s\n", err.Error())
	}
	errMsg += "\nTo proceed, first update these resources to set protected: false"
	return fmt.Errorf("%s", errMsg)
}
