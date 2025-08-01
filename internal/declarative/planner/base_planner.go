package planner

import (
	"fmt"
	"sync/atomic"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// BasePlanner provides common functionality for all resource planners
type BasePlanner struct {
	planner      *Planner
	changeIDCounter *atomic.Int32
}

// NewBasePlanner creates a new base planner instance
func NewBasePlanner(p *Planner) *BasePlanner {
	return &BasePlanner{
		planner:      p,
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

// GetDesiredPortals returns desired portal resources
func (b *BasePlanner) GetDesiredPortals() []resources.PortalResource {
	return b.planner.desiredPortals
}

// GetDesiredAuthStrategies returns desired auth strategy resources
func (b *BasePlanner) GetDesiredAuthStrategies() []resources.ApplicationAuthStrategyResource {
	return b.planner.desiredAuthStrategies
}

// GetDesiredAPIs returns desired API resources
func (b *BasePlanner) GetDesiredAPIs() []resources.APIResource {
	return b.planner.desiredAPIs
}

// GetDesiredAPIVersions returns desired API version resources
func (b *BasePlanner) GetDesiredAPIVersions() []resources.APIVersionResource {
	return b.planner.desiredAPIVersions
}

// GetDesiredAPIPublications returns desired API publication resources
func (b *BasePlanner) GetDesiredAPIPublications() []resources.APIPublicationResource {
	return b.planner.desiredAPIPublications
}

// GetDesiredAPIImplementations returns desired API implementation resources
func (b *BasePlanner) GetDesiredAPIImplementations() []resources.APIImplementationResource {
	return b.planner.desiredAPIImplementations
}

// GetDesiredAPIDocuments returns desired API document resources
func (b *BasePlanner) GetDesiredAPIDocuments() []resources.APIDocumentResource {
	return b.planner.desiredAPIDocuments
}

// GetDesiredPortalCustomizations returns desired portal customization resources
func (b *BasePlanner) GetDesiredPortalCustomizations() []resources.PortalCustomizationResource {
	return b.planner.desiredPortalCustomizations
}

// GetDesiredPortalCustomDomains returns desired portal custom domain resources
func (b *BasePlanner) GetDesiredPortalCustomDomains() []resources.PortalCustomDomainResource {
	return b.planner.desiredPortalCustomDomains
}

// GetDesiredPortalPages returns desired portal page resources
func (b *BasePlanner) GetDesiredPortalPages() []resources.PortalPageResource {
	return b.planner.desiredPortalPages
}

// GetDesiredPortalSnippets returns desired portal snippet resources
func (b *BasePlanner) GetDesiredPortalSnippets() []resources.PortalSnippetResource {
	return b.planner.desiredPortalSnippets
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