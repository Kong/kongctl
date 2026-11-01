package resources

import (
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/konnect/identityproviders"
	"github.com/kong/kongctl/internal/maturity"
)

// IdentityVaultProviderResource inherits ownership from an explicitly managed
// directory. The API stores no independent namespace, ref or protection.
type IdentityVaultProviderResource struct {
	Ref                   string   `yaml:"ref"                              json:"ref"`
	Directory             string   `yaml:"directory,omitempty"              json:"directory,omitempty"`
	Name                  string   `yaml:"name"                             json:"name"`
	TemplateName          string   `yaml:"template_name"                    json:"template_name"`
	ClientID              *string  `yaml:"client_id,omitempty"              json:"client_id,omitempty"`
	ClientSecret          *string  `yaml:"client_secret,omitempty"          json:"client_secret,omitempty"`
	CredentialType        *string  `yaml:"credential_type,omitempty"        json:"credential_type,omitempty"`
	DCREnabled            *bool    `yaml:"dcr_enabled,omitempty"            json:"dcr_enabled,omitempty"`
	PKCEEnabled           *bool    `yaml:"pkce_enabled,omitempty"           json:"pkce_enabled,omitempty"`
	Enabled               *bool    `yaml:"enabled,omitempty"                json:"enabled,omitempty"`
	Scopes                []string `yaml:"scopes,omitempty"                 json:"scopes,omitempty"`
	TokenEndpoint         *string  `yaml:"token_endpoint,omitempty"         json:"token_endpoint,omitempty"`
	AuthorizationEndpoint *string  `yaml:"authorization_endpoint,omitempty" json:"authorization_endpoint,omitempty"` //nolint:lll
	RegistrationEndpoint  *string  `yaml:"registration_endpoint,omitempty"  json:"registration_endpoint,omitempty"`  //nolint:lll
	RefreshBufferSeconds  *int64   `yaml:"refresh_buffer_seconds,omitempty" json:"refresh_buffer_seconds,omitempty"` //nolint:lll
	konnectID             string
}

func (p IdentityVaultProviderResource) GetRef() string { return p.Ref }
func (p IdentityVaultProviderResource) GetType() ResourceType {
	return ResourceTypeIdentityVaultProvider
}
func (p IdentityVaultProviderResource) GetMoniker() string                { return p.Name }
func (p IdentityVaultProviderResource) GetKonnectID() string              { return p.konnectID }
func (p *IdentityVaultProviderResource) SetKonnectID(id string)           { p.konnectID = id }
func (p *IdentityVaultProviderResource) SetDefaults()                     {}
func (p IdentityVaultProviderResource) GetKonnectMonikerFilter() string   { return p.Name }
func (p *IdentityVaultProviderResource) TryMatchKonnectResource(any) bool { return false }
func (p IdentityVaultProviderResource) GetParentRef() *ResourceRef {
	if p.Directory == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeIdentityDirectory, Ref: NormalizeResourceRef(p.Directory)}
}

func (p IdentityVaultProviderResource) GetDependencies() []ResourceRef {
	if parent := p.GetParentRef(); parent != nil {
		return []ResourceRef{*parent}
	}
	return nil
}

func (p IdentityVaultProviderResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{SchemaFieldDirectory: string(ResourceTypeIdentityDirectory)}
}

// CreateBody is the API payload only; local refs, directory selectors and
// enabled (absent from the create API) are deliberately separate.
func (p IdentityVaultProviderResource) CreateBody() kkComps.CreateVaultProviderBody {
	return kkComps.CreateVaultProviderBody{
		Name: p.Name, TemplateName: kkComps.ProviderTemplateName(p.TemplateName),
		ClientID: p.ClientID, ClientSecret: p.ClientSecret, CredentialType: p.CredentialType,
		DcrEnabled: p.DCREnabled, PkceEnabled: p.PKCEEnabled, Scopes: p.Scopes,
		TokenEndpoint: p.TokenEndpoint, AuthorizationEndpoint: p.AuthorizationEndpoint,
		RegistrationEndpoint: p.RegistrationEndpoint, RefreshBufferSeconds: p.RefreshBufferSeconds,
	}
}

func (p IdentityVaultProviderResource) Validate() error {
	if err := ValidateRef(p.Ref); err != nil {
		return err
	}
	return identityproviders.ValidateCreate(p.CreateBody())
}

func identityVaultProviderSlice(rs *ResourceSet) *[]IdentityVaultProviderResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	return &rs.Identity.VaultProviders
}

func ensureIdentityVaultProviderSlice(rs *ResourceSet) *[]IdentityVaultProviderResource {
	if rs.Identity == nil {
		rs.Identity = &IdentityResource{}
	}
	return &rs.Identity.VaultProviders
}

func identityVaultProviderDirectory(rs *ResourceSet, ref string) *IdentityDirectoryResource {
	if rs == nil {
		return nil
	}
	parent, ok := rs.GetResourceByRef(NormalizeResourceRef(ref))
	if !ok {
		return nil
	}
	directory, _ := parent.(*IdentityDirectoryResource)
	return directory
}

func validateIdentityVaultProviders(rs *ResourceSet, values []IdentityVaultProviderResource) error {
	if err := validateChildRefs[IdentityVaultProviderResource](rs, values); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, p := range values {
		parent := identityVaultProviderDirectory(rs, p.Directory)
		if parent == nil || parent.IsExternal() {
			return fmt.Errorf(
				"vault provider %q requires a declared managed directory; external parents are read-only",
				p.Ref,
			)
		}
		key := parent.Ref + "\x00" + p.Name
		if seen[key] {
			return fmt.Errorf("duplicate Vault provider name under directory %q", parent.Ref)
		}
		seen[key] = true
	}
	return nil
}

func init() {
	registerResourceTypeWithSliceAccessors(
		ResourceTypeIdentityVaultProvider,
		identityVaultProviderSlice,
		ensureIdentityVaultProviderSlice,
		AutoExplain[IdentityVaultProviderResource](
			WithExplainAliases(
				"identity.vault_providers",
				"identity.directories.vault_providers",
				"vault-provider",
				"vault-providers",
			),
			WithExplainRecommendedFields("ref", "directory", "name", "template_name", "credential_type"),
			WithExplainFieldHint("template_name", ExplainFieldHint{Literal: "static_secret"}),
			WithExplainFieldHint("credential_type", ExplainFieldHint{Literal: "shared"}),
			WithExplainFieldHint("client_secret", ExplainFieldHint{Notes: []string{
				"Use a deferred secret source. Updates require explicit secret-write selection.",
			}}),
		),
		WithNamespaceFrom(func(rs *ResourceSet, p *IdentityVaultProviderResource) *IdentityDirectoryResource {
			return identityVaultProviderDirectory(rs, p.Directory)
		}),
		WithChildSyncScope(ResourceTypeIdentityDirectory),
		withReferenceValidation(106),
		WithExternalUnsupportedReason(
			"Vault providers inherit explicit managed-directory ownership; external parents are read-only",
		),
		WithMaturity(maturity.Metadata{Level: maturity.LevelTechPreview}),
		withChildLoad(ResourceTypeIdentityVaultProvider, ensureIdentityVaultProviderSlice,
			childLoad[IdentityVaultProviderResource, IdentityDirectoryResource]{
				family: ResourceTypeIdentityDirectory, extractOrder: 51, validateOrder: 61, validatePhase: 249,
				nested:    func(d *IdentityDirectoryResource) *[]IdentityVaultProviderResource { return &d.VaultProviders },
				setParent: func(p *IdentityVaultProviderResource, ref string) { p.Directory = ref },
				validate:  validateIdentityVaultProviders,
			}),
	)
}
