package resources

import (
	"bytes"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeDashboard,
		func(rs *ResourceSet) *[]DashboardResource { return &rs.Dashboards },
		AutoExplain[DashboardResource](
			WithExplainAliases("dashboards"),
			WithExplainRecommendedFields("ref", "name", "definition"),
		),
	)
}

// DashboardResource represents a Konnect Analytics custom dashboard.
type DashboardResource struct {
	BaseResource
	Name          string            `yaml:"name"             json:"name"`
	Definition    kkComps.Dashboard `yaml:"definition"       json:"definition"`
	Labels        map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	definitionSet bool              `yaml:"-"                json:"-"`
}

func (d DashboardResource) GetType() ResourceType {
	return ResourceTypeDashboard
}

func (d DashboardResource) GetMoniker() string {
	return d.Ref
}

func (d DashboardResource) GetDependencies() []ResourceRef {
	return []ResourceRef{}
}

func (d DashboardResource) GetLabels() map[string]string {
	return d.Labels
}

func (d *DashboardResource) SetLabels(labels map[string]string) {
	d.Labels = labels
}

func (d DashboardResource) Validate() error {
	if err := ValidateRef(d.Ref); err != nil {
		return fmt.Errorf("invalid dashboard ref: %w", err)
	}
	if d.Name == "" {
		return fmt.Errorf("name is required for dashboard %s", d.Ref)
	}
	if !d.definitionSet && d.Definition.Tiles == nil {
		return fmt.Errorf("definition is required for dashboard %s", d.Ref)
	}
	if d.Definition.Tiles == nil {
		return fmt.Errorf("definition.tiles is required for dashboard %s", d.Ref)
	}
	return nil
}

func (d *DashboardResource) SetDefaults() {}

func (d DashboardResource) GetKonnectMonikerFilter() string {
	return d.Name
}

func (d *DashboardResource) TryMatchKonnectResource(konnectResource any) bool {
	if id := tryMatchByField(konnectResource, "Name", d.Name); id != "" {
		d.SetKonnectID(id)
		return true
	}
	return false
}

func (d DashboardResource) MarshalJSON() ([]byte, error) {
	type alias struct {
		Ref        string            `json:"ref"`
		Kongctl    *KongctlMeta      `json:"kongctl,omitempty"`
		Name       string            `json:"name"`
		Definition kkComps.Dashboard `json:"definition"`
		Labels     map[string]string `json:"labels,omitempty"`
	}

	return json.Marshal(alias{
		Ref:        d.Ref,
		Kongctl:    d.Kongctl,
		Name:       d.Name,
		Definition: d.Definition,
		Labels:     d.Labels,
	})
}

func (d *DashboardResource) UnmarshalJSON(data []byte) error {
	var temp struct {
		Ref        string             `json:"ref"`
		Kongctl    *KongctlMeta       `json:"kongctl,omitempty"`
		Name       string             `json:"name"`
		Definition *kkComps.Dashboard `json:"definition"`
		Labels     map[string]string  `json:"labels,omitempty"`
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&temp); err != nil {
		return err
	}

	d.Ref = temp.Ref
	d.Kongctl = temp.Kongctl
	d.Name = temp.Name
	d.Labels = temp.Labels
	if temp.Definition != nil {
		d.Definition = *temp.Definition
		d.definitionSet = true
	}

	return nil
}

func (d *DashboardResource) UnmarshalYAML(unmarshal func(any) error) error {
	var temp struct {
		Ref        string             `yaml:"ref"`
		Kongctl    *KongctlMeta       `yaml:"kongctl,omitempty"`
		Name       string             `yaml:"name"`
		Definition *kkComps.Dashboard `yaml:"definition"`
		Labels     map[string]string  `yaml:"labels,omitempty"`
	}

	if err := unmarshal(&temp); err != nil {
		return err
	}

	d.Ref = temp.Ref
	d.Kongctl = temp.Kongctl
	d.Name = temp.Name
	d.Labels = temp.Labels
	if temp.Definition != nil {
		d.Definition = *temp.Definition
		d.definitionSet = true
	}

	return nil
}
