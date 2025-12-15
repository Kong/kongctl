package resources

import (
	"fmt"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// CatalogServiceResource represents a Service Catalog service in declarative configuration
type CatalogServiceResource struct {
	kkComps.CreateCatalogService `yaml:",inline" json:",inline"`
	Ref                          string       `yaml:"ref"               json:"ref"`
	Kongctl                      *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`

	konnectID string `yaml:"-" json:"-"`
}

// GetType returns the resource type
func (c CatalogServiceResource) GetType() ResourceType {
	return ResourceTypeCatalogService
}

// GetRef returns the reference identifier used for cross-resource references
func (c CatalogServiceResource) GetRef() string {
	return c.Ref
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

// GetKonnectID returns the resolved Konnect ID if available
func (c CatalogServiceResource) GetKonnectID() string {
	return c.konnectID
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (c CatalogServiceResource) GetKonnectMonikerFilter() string {
	if c.Name == "" {
		return ""
	}
	return fmt.Sprintf("name[eq]=%s", c.Name)
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource
func (c *CatalogServiceResource) TryMatchKonnectResource(konnectResource any) bool {
	v := reflect.ValueOf(konnectResource)

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return false
	}

	nameField := v.FieldByName("Name")
	idField := v.FieldByName("ID")

	if !nameField.IsValid() || !idField.IsValid() {
		return false
	}

	if nameField.Kind() == reflect.String && idField.Kind() == reflect.String {
		if nameField.String() == c.Name {
			c.konnectID = idField.String()
			return true
		}
	}

	return false
}
