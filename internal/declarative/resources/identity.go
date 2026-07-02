package resources

import "fmt"

const (
	// IdentityDirectoryMinTTLSecs is the minimum TTL accepted by the Kong Identity directory API.
	IdentityDirectoryMinTTLSecs int64 = 300
	// IdentityDirectoryMaxTTLSecs is the maximum TTL accepted by the Kong Identity directory API.
	IdentityDirectoryMaxTTLSecs int64 = 86400
)

func init() {
	registerResourceType(
		ResourceTypeIdentityDirectory,
		func(rs *ResourceSet) *[]IdentityDirectoryResource { return &rs.IdentityDirectories },
		AutoExplain[IdentityDirectoryResource](
			WithExplainAliases("identity.directory", "identity.directories", "directory", "directories"),
			WithExplainRecommendedFields("ref", "name"),
			WithExplainFieldHint("allowed_control_planes", ExplainFieldHint{
				RefKind: string(ResourceTypeControlPlane),
				Notes:   []string{"Each entry may be a control plane ID or !ref to a control_plane resource."},
			}),
		),
	)
}

// IdentityResource represents the identity grouping in declarative configuration.
type IdentityResource struct {
	Directories []IdentityDirectoryResource `yaml:"directories,omitempty" json:"directories,omitempty"`
}

// IdentityDirectoryResource represents a Kong Identity directory in declarative configuration.
type IdentityDirectoryResource struct {
	BaseResource
	Name                  string            `yaml:"name,omitempty"                     json:"name,omitempty"`
	Description           *string           `yaml:"description,omitempty"              json:"description,omitempty"`
	AllowedControlPlanes  []string          `yaml:"allowed_control_planes,omitempty"   json:"allowed_control_planes,omitempty"`   //nolint:lll
	AllowAllControlPlanes *bool             `yaml:"allow_all_control_planes,omitempty" json:"allow_all_control_planes,omitempty"` //nolint:lll
	TTLSecs               *int64            `yaml:"ttl_secs,omitempty"                 json:"ttl_secs,omitempty"`
	NegativeTTLSecs       *int64            `yaml:"negative_ttl_secs,omitempty"        json:"negative_ttl_secs,omitempty"`
	Labels                map[string]string `yaml:"labels,omitempty"                   json:"labels,omitempty"`
}

func (d IdentityDirectoryResource) GetType() ResourceType {
	return ResourceTypeIdentityDirectory
}

func (d IdentityDirectoryResource) GetMoniker() string {
	return d.Name
}

func (d IdentityDirectoryResource) GetDependencies() []ResourceRef {
	return []ResourceRef{}
}

func (d IdentityDirectoryResource) GetLabels() map[string]string {
	return d.Labels
}

func (d *IdentityDirectoryResource) SetLabels(labels map[string]string) {
	d.Labels = labels
}

func (d IdentityDirectoryResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{
		"allowed_control_planes": string(ResourceTypeControlPlane),
	}
}

func (d IdentityDirectoryResource) Validate() error {
	if err := ValidateRef(d.Ref); err != nil {
		return fmt.Errorf("invalid identity directory ref: %w", err)
	}
	if d.Name == "" {
		return fmt.Errorf("name is required")
	}
	if err := validateIdentityDirectoryTTL("ttl_secs", d.TTLSecs); err != nil {
		return err
	}
	if err := validateIdentityDirectoryTTL("negative_ttl_secs", d.NegativeTTLSecs); err != nil {
		return err
	}
	return nil
}

func validateIdentityDirectoryTTL(field string, value *int64) error {
	if value == nil {
		return nil
	}
	if *value < IdentityDirectoryMinTTLSecs || *value > IdentityDirectoryMaxTTLSecs {
		return fmt.Errorf("%s must be between %d and %d",
			field, IdentityDirectoryMinTTLSecs, IdentityDirectoryMaxTTLSecs)
	}
	return nil
}

func (d *IdentityDirectoryResource) SetDefaults() {
	if d.Name == "" {
		d.Name = d.Ref
	}
}

func (d IdentityDirectoryResource) GetKonnectMonikerFilter() string {
	return d.BaseResource.GetKonnectMonikerFilter(d.Name)
}

func (d *IdentityDirectoryResource) TryMatchKonnectResource(konnectResource any) bool {
	return d.TryMatchByName(d.Name, konnectResource, matchOptions{sdkType: "KongDirectory"})
}
