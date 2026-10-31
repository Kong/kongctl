package resources

import (
	"fmt"

	"github.com/kong/kongctl/internal/konnect/identitykeys"
	"github.com/kong/kongctl/internal/maturity"
)

// IdentityVaultKeyResource inherits lifecycle authority from a managed directory.
// Public key material determines identity; the API name is mutable and nonunique.
type IdentityVaultKeyResource struct {
	Ref       string `yaml:"ref"                 json:"ref"`
	Directory string `yaml:"directory,omitempty" json:"directory,omitempty"`
	Name      string `yaml:"name"                json:"name"`
	PublicKey string `yaml:"public_key"          json:"public_key"`
	konnectID string
}

func (k IdentityVaultKeyResource) GetRef() string        { return k.Ref }
func (k IdentityVaultKeyResource) GetType() ResourceType { return ResourceTypeIdentityVaultKey }
func (k IdentityVaultKeyResource) GetMoniker() string {
	key, _ := identitykeys.ParsePublicKey(k.PublicKey)
	return key.Kid
}
func (k IdentityVaultKeyResource) GetKonnectID() string              { return k.konnectID }
func (k *IdentityVaultKeyResource) SetKonnectID(id string)           { k.konnectID = id }
func (k *IdentityVaultKeyResource) SetDefaults()                     {}
func (k IdentityVaultKeyResource) GetKonnectMonikerFilter() string   { return k.GetMoniker() }
func (k *IdentityVaultKeyResource) TryMatchKonnectResource(any) bool { return false }
func (k IdentityVaultKeyResource) GetParentRef() *ResourceRef {
	if k.Directory == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeIdentityDirectory, Ref: NormalizeResourceRef(k.Directory)}
}

func (k IdentityVaultKeyResource) GetDependencies() []ResourceRef {
	if parent := k.GetParentRef(); parent != nil {
		return []ResourceRef{*parent}
	}
	return nil
}

func (k IdentityVaultKeyResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{"directory": string(ResourceTypeIdentityDirectory)}
}

func (k IdentityVaultKeyResource) Validate() error {
	if err := ValidateRef(k.Ref); err != nil {
		return err
	}
	if err := identitykeys.ValidateName(k.Name); err != nil {
		return err
	}
	_, err := identitykeys.ParsePublicKey(k.PublicKey)
	return err
}

func identityVaultKeySlice(rs *ResourceSet) *[]IdentityVaultKeyResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	return &rs.Identity.VaultKeys
}

func ensureIdentityVaultKeySlice(rs *ResourceSet) *[]IdentityVaultKeyResource {
	if rs.Identity == nil {
		rs.Identity = &IdentityResource{}
	}
	return &rs.Identity.VaultKeys
}

func identityVaultKeyDirectory(rs *ResourceSet, ref string) *IdentityDirectoryResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	for i := range rs.Identity.Directories {
		if rs.Identity.Directories[i].Ref == NormalizeResourceRef(ref) {
			return &rs.Identity.Directories[i]
		}
	}
	return nil
}

func validateIdentityVaultKeys(rs *ResourceSet, values []IdentityVaultKeyResource) error {
	if err := validateChildRefs[IdentityVaultKeyResource](rs, values); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, k := range values {
		parent := identityVaultKeyDirectory(rs, k.Directory)
		if parent == nil || parent.IsExternal() {
			return fmt.Errorf(
				"vault key %q requires a declared managed directory; external parents are read-only",
				k.Ref,
			)
		}
		key := parent.Ref + "\x00" + k.GetMoniker()
		if seen[key] {
			return fmt.Errorf("duplicate Vault public key material under directory %q", parent.Ref)
		}
		seen[key] = true
	}
	return nil
}

func init() {
	registerResourceTypeWithSliceAccessors(
		ResourceTypeIdentityVaultKey,
		identityVaultKeySlice,
		ensureIdentityVaultKeySlice,
		AutoExplain[IdentityVaultKeyResource](
			WithExplainAliases("identity.vault_keys", "identity.directories.vault_keys", "vault-key", "vault-keys"),
			WithExplainRecommendedFields("ref", "directory", "name", "public_key"),
			WithExplainFieldHint(
				"public_key",
				ExplainFieldHint{FileSample: "./vault-public-key.pem", PreferredTag: "!file", Notes: []string{
					"One PEM PKIX PUBLIC KEY: RSA >= 2048 bits or EC P-256/P-384/P-521. Never supply private material.",
					"The RFC 7638 kid fingerprint determines identity; name is mutable and not unique.",
				}},
			),
		),
		WithNamespaceFrom(func(rs *ResourceSet, k *IdentityVaultKeyResource) *IdentityDirectoryResource {
			return identityVaultKeyDirectory(rs, k.Directory)
		}),
		WithChildSyncScope(ResourceTypeIdentityDirectory),
		withReferenceValidation(105),
		WithExternalUnsupportedReason(
			"Vault keys inherit explicitly managed directory ownership; external parents are read-only",
		),
		WithMaturity(maturity.Metadata{Level: maturity.LevelTechPreview}),
		withChildLoad(ResourceTypeIdentityVaultKey, ensureIdentityVaultKeySlice,
			childLoad[IdentityVaultKeyResource, IdentityDirectoryResource]{
				family: ResourceTypeIdentityDirectory, extractOrder: 50, validateOrder: 60, validatePhase: 248,
				nested:    func(d *IdentityDirectoryResource) *[]IdentityVaultKeyResource { return &d.VaultKeys },
				setParent: func(k *IdentityVaultKeyResource, ref string) { k.Directory = ref },
				validate:  validateIdentityVaultKeys,
			}),
	)
}
