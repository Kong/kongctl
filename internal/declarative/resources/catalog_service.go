package resources

import (
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// CatalogServiceResource represents a Service Catalog service in declarative configuration
type CatalogServiceResource struct {
	BaseResource
	kkComps.CreateCatalogService `yaml:",inline" json:",inline"`
}

// GetType returns the resource type
func (c CatalogServiceResource) GetType() ResourceType {
	return ResourceTypeCatalogService
}

// GetMoniker returns the resource moniker (service name)
func (c CatalogServiceResource) GetMoniker() string {
	return c.Name
}

// GetDependencies returns references to other resources this service depends on
func (c CatalogServiceResource) GetDependencies() []ResourceRef {
	return []ResourceRef{}
}

// GetLabels returns the labels for this resource
func (c CatalogServiceResource) GetLabels() map[string]string {
	return c.Labels
}

// SetLabels sets the labels for this resource
func (c *CatalogServiceResource) SetLabels(labels map[string]string) {
	c.Labels = labels
}

// Validate ensures the CatalogService resource is valid
func (c CatalogServiceResource) Validate() error {
	if err := ValidateRef(c.Ref); err != nil {
		return fmt.Errorf("invalid catalog service ref: %w", err)
	}
	if c.Name == "" {
		return fmt.Errorf("name is required for catalog service %s", c.Ref)
	}
	if c.DisplayName == "" {
		return fmt.Errorf("display_name is required for catalog service %s", c.Ref)
	}
	return nil
}

// SetDefaults applies default values to CatalogService resource
func (c *CatalogServiceResource) SetDefaults() {
	if c.Name == "" {
		c.Name = c.Ref
	}
	if c.DisplayName == "" {
		c.DisplayName = c.Name
	}
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (c CatalogServiceResource) GetKonnectMonikerFilter() string {
	return c.BaseResource.GetKonnectMonikerFilter(c.Name)
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource.
func (c *CatalogServiceResource) TryMatchKonnectResource(konnectResource any) bool {
	return c.TryMatchByName(c.Name, konnectResource, matchOptions{})
}
