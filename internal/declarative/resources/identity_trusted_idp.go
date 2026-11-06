package resources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/konnect/identitytrust"
)

// IdentityTrustedIDPResource inherits directory lifecycle authority. The exact
// issuer identifies the configuration, never its nonunique display name.
type IdentityTrustedIDPResource struct {
	Ref                    string                 `yaml:"ref"                                json:"ref"`
	Directory              string                 `yaml:"directory,omitempty"                json:"directory,omitempty"`
	IssuerURL              string                 `yaml:"issuer_url"                         json:"issuer_url"`
	DisplayName            *string                `yaml:"display_name,omitempty"             json:"display_name,omitempty"`
	JwksURI                *string                `yaml:"jwks_uri,omitempty"                 json:"jwks_uri,omitempty"`
	JitProvisioningEnabled *bool                  `yaml:"jit_provisioning_enabled,omitempty" json:"jit_provisioning_enabled,omitempty"` //nolint:lll
	VaultAccessEnabled     *bool                  `yaml:"vault_access_enabled,omitempty"     json:"vault_access_enabled,omitempty"`     //nolint:lll
	Jit                    *kkComps.TrustedIdpJit `yaml:"jit,omitempty"                      json:"jit,omitempty"`
	konnectID              string
	displayDeclared        bool
	jwksDeclared           bool
}

func (p *IdentityTrustedIDPResource) UnmarshalJSON(data []byte) error {
	type plain IdentityTrustedIDPResource
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, key := range []string{"jit", "jit_provisioning_enabled", "vault_access_enabled"} {
		if raw, present := fields[key]; present && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fmt.Errorf("trusted IdP %s cannot be null", key)
		}
	}
	if value.Jit != nil {
		var jit map[string]json.RawMessage
		if err := json.Unmarshal(fields["jit"], &jit); err != nil {
			return err
		}
		if raw, present := jit["mapping"]; present {
			if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
				return fmt.Errorf("trusted IdP jit.mapping cannot be null; use jit: {} to clear the mapping")
			}
			var mapping map[string]json.RawMessage
			if err := json.Unmarshal(raw, &mapping); err != nil {
				return err
			}
			for key, raw := range mapping {
				if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
					return fmt.Errorf(
						"trusted IdP jit.mapping.%s cannot be null; omit it from the replacement mapping",
						key,
					)
				}
			}
		}
	}
	*p = IdentityTrustedIDPResource(value)
	_, p.displayDeclared = fields["display_name"]
	_, p.jwksDeclared = fields["jwks_uri"]
	return nil
}

func (p IdentityTrustedIDPResource) MarshalJSON() ([]byte, error) {
	type plain IdentityTrustedIDPResource
	data, err := json.Marshal(plain(p))
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if p.DisplayNameDeclared() && p.DisplayName == nil {
		fields["display_name"] = json.RawMessage("null")
	}
	if p.JwksURIDeclared() && p.JwksURI == nil {
		fields["jwks_uri"] = json.RawMessage("null")
	}
	return json.Marshal(fields)
}

func (p IdentityTrustedIDPResource) DisplayNameDeclared() bool {
	return p.displayDeclared || p.DisplayName != nil
}

func (p IdentityTrustedIDPResource) JwksURIDeclared() bool { return p.jwksDeclared || p.JwksURI != nil }

func (p *IdentityTrustedIDPResource) DeclareNullDisplayName() {
	p.DisplayName = nil
	p.displayDeclared = true
}

func (p *IdentityTrustedIDPResource) DeclareDiscoveryJWKS()            { p.JwksURI = nil; p.jwksDeclared = true }
func (p IdentityTrustedIDPResource) GetRef() string                    { return p.Ref }
func (p IdentityTrustedIDPResource) GetType() ResourceType             { return ResourceTypeIdentityTrustedIDP }
func (p IdentityTrustedIDPResource) GetMoniker() string                { return p.IssuerURL }
func (p IdentityTrustedIDPResource) GetKonnectID() string              { return p.konnectID }
func (p *IdentityTrustedIDPResource) SetKonnectID(id string)           { p.konnectID = id }
func (p *IdentityTrustedIDPResource) SetDefaults()                     {}
func (p IdentityTrustedIDPResource) GetKonnectMonikerFilter() string   { return p.IssuerURL }
func (p *IdentityTrustedIDPResource) TryMatchKonnectResource(any) bool { return false }
func (p IdentityTrustedIDPResource) GetParentRef() *ResourceRef {
	if p.Directory == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeIdentityDirectory, Ref: NormalizeResourceRef(p.Directory)}
}

func (p IdentityTrustedIDPResource) GetDependencies() []ResourceRef {
	if parent := p.GetParentRef(); parent != nil {
		return []ResourceRef{*parent}
	}
	return nil
}

func (p IdentityTrustedIDPResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{SchemaFieldDirectory: string(ResourceTypeIdentityDirectory)}
}

func (p IdentityTrustedIDPResource) CreateBody() kkComps.CreateTrustedIdpPayload {
	return kkComps.CreateTrustedIdpPayload{
		IssuerURL: p.IssuerURL, DisplayName: p.DisplayName, JwksURI: p.JwksURI,
		JitProvisioningEnabled: p.JitProvisioningEnabled, VaultAccessEnabled: p.VaultAccessEnabled, Jit: p.Jit,
	}
}

func (p IdentityTrustedIDPResource) Validate() error {
	if err := ValidateRef(p.Ref); err != nil {
		return err
	}
	return identitytrust.ValidatePublic(p.IssuerURL, p.DisplayName, p.JwksURI, p.Jit)
}

func identityTrustedIDPSlice(rs *ResourceSet) *[]IdentityTrustedIDPResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	return &rs.Identity.TrustedIDPs
}

func ensureIdentityTrustedIDPSlice(rs *ResourceSet) *[]IdentityTrustedIDPResource {
	if rs.Identity == nil {
		rs.Identity = &IdentityResource{}
	}
	return &rs.Identity.TrustedIDPs
}

func validateIdentityTrustedIDPs(rs *ResourceSet, values []IdentityTrustedIDPResource) error {
	if err := validateChildRefs[IdentityTrustedIDPResource](rs, values); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, value := range values {
		parent := identityVaultProviderDirectory(rs, value.Directory)
		if parent == nil || parent.IsExternal() {
			return fmt.Errorf(
				"trusted IdP %q requires a declared managed directory; external parents are read-only",
				value.Ref,
			)
		}
		key := parent.Ref + "\x00" + value.IssuerURL
		if seen[key] {
			return fmt.Errorf("duplicate trusted IdP issuer under directory %q", parent.Ref)
		}
		seen[key] = true
	}
	return nil
}

func init() {
	registerResourceTypeWithSliceAccessors(
		ResourceTypeIdentityTrustedIDP,
		identityTrustedIDPSlice,
		ensureIdentityTrustedIDPSlice,
		AutoExplain[IdentityTrustedIDPResource](
			WithExplainSchemaBuilder(identityTrustedIDPExplainNode),
			WithExplainAliases(
				"identity.trusted_idps",
				"identity.directories.trusted_idps",
				"trusted-idp",
				"trusted-idps",
			),
			WithExplainRecommendedFields("ref", "directory", "issuer_url"),
			WithExplainFieldHint("issuer_url", ExplainFieldHint{Literal: "https://issuer.example.com"}),
			WithExplainFieldHint(
				"jwks_uri",
				ExplainFieldHint{
					Notes: []string{"Null selects OIDC discovery; omission preserves the current override."},
				},
			),
			WithExplainFieldHint(
				"jit",
				ExplainFieldHint{Notes: []string{"Omission preserves the current mapping. An empty object clears it."}},
			),
		),
		WithNamespaceFrom(func(rs *ResourceSet, p *IdentityTrustedIDPResource) *IdentityDirectoryResource {
			return identityVaultProviderDirectory(rs, p.Directory)
		}),
		WithChildSyncScope(ResourceTypeIdentityDirectory),
		withReferenceValidation(107),
		WithExternalUnsupportedReason(
			"Trusted IdPs inherit explicit managed-directory ownership; external parents are read-only",
		),
		withChildLoad(ResourceTypeIdentityTrustedIDP, ensureIdentityTrustedIDPSlice,
			childLoad[IdentityTrustedIDPResource, IdentityDirectoryResource]{
				family: ResourceTypeIdentityDirectory, extractOrder: 52, validateOrder: 62, validatePhase: 250,
				nested:    func(d *IdentityDirectoryResource) *[]IdentityTrustedIDPResource { return &d.TrustedIDPs },
				setParent: func(p *IdentityTrustedIDPResource, ref string) { p.Directory = ref },
				validate:  validateIdentityTrustedIDPs,
			}),
	)
}

func identityTrustedIDPExplainNode(ctx ExplainBuildContext) (*ExplainNode, error) {
	hints := defaultExplainHints(ctx.ResourceType)
	maps.Copy(hints, ctx.Registration.fieldHints)
	node, err := autoExplainNode(ctx.Registration.typ, nil, hints, nil)
	if err != nil {
		return nil, err
	}
	for _, path := range []string{
		"jit_provisioning_enabled", "vault_access_enabled", "jit", "jit.mapping",
		"jit.mapping.email", "jit.mapping.groups", "jit.mapping.display_name",
	} {
		if child, ok := node.lookup(strings.Split(path, ".")); ok {
			child.Nullable = false
		}
	}
	return node, nil
}
