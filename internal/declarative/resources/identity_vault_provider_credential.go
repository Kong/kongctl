package resources

import (
	"fmt"
	"strconv"

	"github.com/kong/kongctl/internal/maturity"
)

// IdentityVaultProviderCredentialResource inherits managed-directory authority
// through an explicitly declared static/shared provider. Ref is local only.
type IdentityVaultProviderCredentialResource struct {
	Ref       string  `yaml:"ref"                json:"ref"`
	Provider  string  `yaml:"provider,omitempty" json:"provider,omitempty"`
	Selector  *string `yaml:"selector,omitempty" json:"selector,omitempty"`
	Priority  *int64  `yaml:"priority,omitempty" json:"priority,omitempty"`
	Secret    *string `yaml:"secret,omitempty"   json:"secret,omitempty"`
	konnectID string
}

func (c IdentityVaultProviderCredentialResource) GetRef() string { return c.Ref }
func (c IdentityVaultProviderCredentialResource) GetType() ResourceType {
	return ResourceTypeIdentityVaultProviderCredential
}

func (c IdentityVaultProviderCredentialResource) GetMoniker() string {
	return IdentityVaultCredentialSlot(c.Priority)
}

func IdentityVaultCredentialSlot(priority *int64) string {
	if priority == nil {
		return "default"
	}
	return "priority-" + strconv.FormatInt(*priority, 10)
}
func (c IdentityVaultProviderCredentialResource) GetKonnectID() string    { return c.konnectID }
func (c *IdentityVaultProviderCredentialResource) SetKonnectID(id string) { c.konnectID = id }
func (c *IdentityVaultProviderCredentialResource) SetDefaults()           {}
func (c IdentityVaultProviderCredentialResource) GetKonnectMonikerFilter() string {
	return c.GetMoniker()
}
func (c *IdentityVaultProviderCredentialResource) TryMatchKonnectResource(any) bool { return false }
func (c IdentityVaultProviderCredentialResource) GetParentRef() *ResourceRef {
	if c.Provider == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeIdentityVaultProvider, Ref: NormalizeResourceRef(c.Provider)}
}

func (c IdentityVaultProviderCredentialResource) GetDependencies() []ResourceRef {
	if parent := c.GetParentRef(); parent != nil {
		return []ResourceRef{*parent}
	}
	return nil
}

func (c IdentityVaultProviderCredentialResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{"provider": string(ResourceTypeIdentityVaultProvider)}
}

func (c IdentityVaultProviderCredentialResource) Validate() error {
	if err := ValidateRef(c.Ref); err != nil {
		return err
	}
	if (c.Selector == nil) != (c.Priority == nil) || c.Priority != nil && *c.Priority < 0 {
		return fmt.Errorf("credential selector and nonnegative priority must be supplied together")
	}
	return nil
}

func (c IdentityVaultProviderCredentialResource) MutablePayloadMap() (map[string]any, error) {
	fields := make(map[string]any)
	if c.Selector != nil {
		fields["selector"] = *c.Selector
		fields["priority"] = *c.Priority
	}
	if c.Secret != nil {
		fields["secret"] = *c.Secret
	}
	return fields, nil
}

func IdentityVaultCredentialProvider(rs *ResourceSet, ref string) *IdentityVaultProviderResource {
	if rs == nil {
		return nil
	}
	value, ok := rs.GetResourceByRef(NormalizeResourceRef(ref))
	if !ok {
		return nil
	}
	provider, _ := value.(*IdentityVaultProviderResource)
	return provider
}

func identityVaultProviderCredentialSlice(rs *ResourceSet) *[]IdentityVaultProviderCredentialResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	return &rs.Identity.VaultProviderCredentials
}

func ensureIdentityVaultProviderCredentialSlice(rs *ResourceSet) *[]IdentityVaultProviderCredentialResource {
	if rs.Identity == nil {
		rs.Identity = &IdentityResource{}
	}
	return &rs.Identity.VaultProviderCredentials
}

func validateIdentityVaultProviderCredentials(rs *ResourceSet, values []IdentityVaultProviderCredentialResource) error {
	if err := validateChildRefs[IdentityVaultProviderCredentialResource](rs, values); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, value := range values {
		provider := IdentityVaultCredentialProvider(rs, value.Provider)
		if provider == nil || provider.TemplateName != "static_secret" || provider.CredentialType == nil ||
			*provider.CredentialType != "shared" {
			return fmt.Errorf("vault credential %q requires a declared static_secret/shared provider", value.Ref)
		}
		directory := identityVaultProviderDirectory(rs, provider.Directory)
		if directory == nil || directory.IsExternal() {
			return fmt.Errorf("vault credential %q requires a managed directory", value.Ref)
		}
		key := directory.Ref + "\x00" + provider.Name + "\x00" + value.GetMoniker()
		if seen[key] {
			return fmt.Errorf("duplicate credential routing slot under provider %q", provider.Ref)
		}
		seen[key] = true
	}
	return nil
}

func init() {
	registerResourceTypeWithSliceAccessors(
		ResourceTypeIdentityVaultProviderCredential,
		identityVaultProviderCredentialSlice,
		ensureIdentityVaultProviderCredentialSlice,
		AutoExplain[IdentityVaultProviderCredentialResource](
			WithExplainAliases(
				"identity.vault_provider_credentials",
				"identity.directories.vault_providers.shared_credentials",
				"vault-provider-credential",
			),
			WithExplainRecommendedFields("ref", "provider"),
			WithExplainFieldHint(
				"secret",
				ExplainFieldHint{
					Notes: []string{
						"Use !secret with a deferred source. Creation requires material; " +
							"updates require explicit secret-write selection.",
					},
				},
			),
		),
		WithNamespaceFrom(
			func(rs *ResourceSet, c *IdentityVaultProviderCredentialResource) *IdentityVaultProviderResource {
				return IdentityVaultCredentialProvider(rs, c.Provider)
			},
		),
		WithChildSyncScope(ResourceTypeIdentityVaultProvider),
		withReferenceValidation(108),
		WithExternalUnsupportedReason(
			"Static shared credentials inherit explicitly managed-directory authority through their provider",
		),
		WithMaturity(maturity.Metadata{Level: maturity.LevelTechPreview}),
		withChildLoad(ResourceTypeIdentityVaultProviderCredential, ensureIdentityVaultProviderCredentialSlice,
			childLoad[IdentityVaultProviderCredentialResource, IdentityVaultProviderResource]{
				family: ResourceTypeIdentityDirectory, extractOrder: 53, validateOrder: 63, validatePhase: 251,
				nested: func(p *IdentityVaultProviderResource) *[]IdentityVaultProviderCredentialResource {
					return &p.SharedCredentials
				},
				setParent: func(c *IdentityVaultProviderCredentialResource, ref string) { c.Provider = ref },
				validate:  validateIdentityVaultProviderCredentials,
			}),
	)
}
