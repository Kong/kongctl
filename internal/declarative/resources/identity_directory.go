package resources

import (
	"fmt"
	"regexp"
	"unicode/utf8"

	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/maturity"
	"github.com/kong/kongctl/internal/util"
)

var identityDirectoryNamePattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

func init() {
	registerExternalResourceTypeWithSliceAccessors(
		ResourceTypeIdentityDirectory,
		identityDirectorySlice,
		ensureIdentityDirectorySlice,
		AutoExplain[IdentityDirectoryResource](
			WithExplainAliases("identity.directory", "directory", "directories", "identity_directories"),
			WithExplainRecommendedFields("ref", "name", "allow_all_control_planes"),
			WithExplainFieldHint("ttl_secs", ExplainFieldHint{Default: int64(600), Literal: "600"}),
			WithExplainFieldHint("negative_ttl_secs", ExplainFieldHint{Default: int64(600), Literal: "600"}),
			WithExplainFieldHint("allowed_control_planes", ExplainFieldHint{
				RefKind: string(ResourceTypeControlPlane),
				Notes:   []string{"Control plane UUIDs or !ref control-plane#id expressions."},
			}),
		),
		ExternalResolutionRegistration{Selectors: []string{SchemaFieldName}},
		WithNamespace(200, func(d *IdentityDirectoryResource) NamespaceParticipant {
			return NamespaceParticipant{
				Ref: d.Ref, Label: string(ResourceTypeIdentityDirectory),
				SupportsProtected: true, Meta: &d.Kongctl, External: d.IsExternal(),
			}
		}),
		WithManagedBy(ManagedByPolicy{MaxEntries: 5, OwnsLifecycle: true}),
		WithMaturity(maturity.Metadata{
			Level:   maturity.LevelTechPreview,
			Message: "Requires the Kong Identity SDK preview and regional directory-admin access.",
		}),
		WithRootSyncScope(),
		withCollectionValidation(
			220,
			namedCollectionValidation(func(d *IdentityDirectoryResource) string { return d.Name }),
		),
		withReferenceValidation(90),
	)
}

// IdentityResource groups Kong Identity management declarations.
type IdentityResource struct {
	Identities  []IdentityPrincipalIdentityResource `yaml:"identities,omitempty"   json:"identities,omitempty"`
	Clients     []IdentityAuthServerClientResource  `yaml:"clients,omitempty"      json:"clients,omitempty"`
	Scopes      []IdentityAuthServerScopeResource   `yaml:"scopes,omitempty"       json:"scopes,omitempty"`
	Claims      []IdentityAuthServerClaimResource   `yaml:"claims,omitempty"       json:"claims,omitempty"`
	AuthServers []IdentityAuthServerResource        `yaml:"auth_servers,omitempty" json:"auth_servers,omitempty"`
	Principals  []IdentityPrincipalResource         `yaml:"principals,omitempty"   json:"principals,omitempty"`
	Directories []IdentityDirectoryResource         `yaml:"directories,omitempty"  json:"directories,omitempty"`
}

func identityDirectorySlice(rs *ResourceSet) *[]IdentityDirectoryResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	return &rs.Identity.Directories
}

func ensureIdentityDirectorySlice(rs *ResourceSet) *[]IdentityDirectoryResource {
	if rs.Identity == nil {
		rs.Identity = &IdentityResource{}
	}
	return &rs.Identity.Directories
}

// IdentityDirectoryResource declares one regional Kong Identity directory.
type IdentityDirectoryResource struct {
	BaseResource          `                            yaml:",inline"                            json:",inline"`
	Principals            []IdentityPrincipalResource `yaml:"principals,omitempty"               json:"principals,omitempty"` //nolint:lll
	Name                  string                      `yaml:"name"                               json:"name"`
	Description           *string                     `yaml:"description,omitempty"              json:"description,omitempty"`                              //nolint:lll
	AllowedControlPlanes  []string                    `yaml:"allowed_control_planes,omitempty"   json:"allowed_control_planes,omitempty"`                   //nolint:lll
	AllowAllControlPlanes *bool                       `yaml:"allow_all_control_planes,omitempty" json:"allow_all_control_planes,omitempty" default:"false"` //nolint:lll
	TTLSecs               *int64                      `yaml:"ttl_secs,omitempty"                 json:"ttl_secs,omitempty"                 default:"600"`   //nolint:lll
	NegativeTTLSecs       *int64                      `yaml:"negative_ttl_secs,omitempty"        json:"negative_ttl_secs,omitempty"        default:"600"`   //nolint:lll
	VaultEnabled          *bool                       `yaml:"vault_enabled,omitempty"            json:"vault_enabled,omitempty"`                            //nolint:lll
	Labels                map[string]string           `yaml:"labels,omitempty"                   json:"labels,omitempty"`                                   //nolint:lll
	External              *ExternalBlock              `yaml:"_external,omitempty"                json:"_external,omitempty"`                                //nolint:lll
}

func (d IdentityDirectoryResource) GetType() ResourceType              { return ResourceTypeIdentityDirectory }
func (d IdentityDirectoryResource) GetMoniker() string                 { return d.Name }
func (d IdentityDirectoryResource) GetLabels() map[string]string       { return d.Labels }
func (d *IdentityDirectoryResource) SetLabels(value map[string]string) { d.Labels = value }
func (d *IdentityDirectoryResource) SetDefaults()                      {}
func (d *IdentityDirectoryResource) GetExternalBlock() *ExternalBlock  { return d.External }
func (d *IdentityDirectoryResource) IsExternal() bool                  { return d.External != nil }
func (d IdentityDirectoryResource) GetKonnectMonikerFilter() string    { return d.Name }
func (d *IdentityDirectoryResource) TryMatchKonnectResource(value any) bool {
	id, ok := tryMatchByNameWithExternal(d.Name, value, matchOptions{}, d.External)
	if ok {
		d.SetKonnectID(id)
	}
	return ok
}

func (d IdentityDirectoryResource) GetDependencies() []ResourceRef {
	var result []ResourceRef
	for _, value := range d.AllowedControlPlanes {
		if ref, _, ok := tags.ParseRefPlaceholder(value); ok {
			result = append(result, ResourceRef{Kind: ResourceTypeControlPlane, Ref: ref})
		}
	}
	return result
}

func (d IdentityDirectoryResource) Protected() bool { return d.Kongctl.IsProtected() }

func (d IdentityDirectoryResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{"allowed_control_planes": string(ResourceTypeControlPlane)}
}

func (d IdentityDirectoryResource) Validate() error {
	if err := ValidateRef(d.Ref); err != nil {
		return fmt.Errorf("invalid directory ref: %w", err)
	}
	if d.IsExternal() {
		return d.External.Validate()
	}
	if len(d.Name) == 0 || len(d.Name) > 255 || !identityDirectoryNamePattern.MatchString(d.Name) {
		return fmt.Errorf("directory name is required; use 1-255 lowercase letters, digits, underscores or hyphens")
	}
	if d.Description != nil && utf8.RuneCountInString(*d.Description) > 255 {
		return fmt.Errorf("directory description must be at most 255 characters")
	}
	for _, ttl := range []struct {
		name  string
		value *int64
	}{
		{"ttl_secs", d.TTLSecs}, {"negative_ttl_secs", d.NegativeTTLSecs},
	} {
		if ttl.value != nil && (*ttl.value < 300 || *ttl.value > 86400) {
			return fmt.Errorf("%s must be between 300 and 86400", ttl.name)
		}
	}
	if len(d.AllowedControlPlanes) > 32 {
		return fmt.Errorf("allowed_control_planes supports at most 32 entries")
	}
	for _, value := range d.AllowedControlPlanes {
		if tags.IsExternalPlaceholder(value) {
			continue
		}
		if _, field, ok := tags.ParseRefPlaceholder(value); ok && field == "id" {
			continue
		}
		if !util.IsValidUUID(value) {
			return fmt.Errorf("allowed_control_planes entries must be UUIDs or !ref control-plane#id expressions")
		}
	}
	return ValidateChildManagedBy(BuildChildManagedBy(GetNamespace(d.Kongctl), d.Protected(), ""), false)
}
