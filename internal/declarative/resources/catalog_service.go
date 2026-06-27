package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeCatalogService,
		func(rs *ResourceSet) *[]CatalogServiceResource { return &rs.CatalogServices },
		AutoExplain[CatalogServiceResource](),
	)
}

// CatalogServiceResource represents a Service Catalog service in declarative configuration
type CatalogServiceResource struct {
	BaseResource                 `yaml:",inline" json:",inline"`
	kkComps.CreateCatalogService `yaml:",inline" json:",inline"`
}

// UnmarshalYAML decodes catalog service fields explicitly because the SDK
// request type only carries JSON tags.
func (c *CatalogServiceResource) UnmarshalYAML(unmarshal func(any) error) error {
	var raw struct {
		Ref          string            `yaml:"ref"`
		Kongctl      *KongctlMeta      `yaml:"kongctl,omitempty"`
		Name         string            `yaml:"name"`
		DisplayName  string            `yaml:"display_name"`
		Description  *string           `yaml:"description,omitempty"`
		Labels       map[string]string `yaml:"labels,omitempty"`
		CustomFields any               `yaml:"custom_fields,omitempty"`
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	c.BaseResource = BaseResource{
		Ref:     raw.Ref,
		Kongctl: raw.Kongctl,
	}
	c.CreateCatalogService = kkComps.CreateCatalogService{
		Name:         raw.Name,
		DisplayName:  raw.DisplayName,
		Description:  raw.Description,
		Labels:       raw.Labels,
		CustomFields: raw.CustomFields,
	}

	return nil
}

// UnmarshalJSON decodes catalog services explicitly because YAML loading goes
// through JSON tags and the embedded SDK request type has a custom unmarshaler.
func (c *CatalogServiceResource) UnmarshalJSON(data []byte) error {
	var raw struct {
		Ref          string            `json:"ref"`
		Kongctl      *KongctlMeta      `json:"kongctl,omitempty"`
		Name         string            `json:"name"`
		DisplayName  string            `json:"display_name"`
		Description  *string           `json:"description,omitempty"`
		Labels       map[string]string `json:"labels,omitempty"`
		CustomFields any               `json:"custom_fields,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	c.BaseResource = BaseResource{
		Ref:     raw.Ref,
		Kongctl: raw.Kongctl,
	}
	c.CreateCatalogService = kkComps.CreateCatalogService{
		Name:         raw.Name,
		DisplayName:  raw.DisplayName,
		Description:  raw.Description,
		Labels:       raw.Labels,
		CustomFields: raw.CustomFields,
	}

	return nil
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
