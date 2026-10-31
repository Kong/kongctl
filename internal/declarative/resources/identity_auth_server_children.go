package resources

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/maturity"
	"github.com/kong/kongctl/internal/util"
)

// IdentityAuthServerChild inherits ownership from an explicitly managed parent.
type IdentityAuthServerChild struct {
	Ref        string `yaml:"ref" json:"ref"`
	AuthServer string `yaml:"auth_server,omitempty" json:"auth_server,omitempty"`
	Name       string `yaml:"name" json:"name"`
	konnectID  string
}

func (c IdentityAuthServerChild) GetRef() string                  { return c.Ref }
func (c IdentityAuthServerChild) GetMoniker() string              { return c.Name }
func (c IdentityAuthServerChild) GetKonnectID() string            { return c.konnectID }
func (c *IdentityAuthServerChild) SetKonnectID(id string)         { c.konnectID = id }
func (c *IdentityAuthServerChild) SetDefaults()                   {}
func (c IdentityAuthServerChild) GetKonnectMonikerFilter() string { return c.Name }
func (c *IdentityAuthServerChild) TryMatchKonnectResource(value any) bool {
	id, ok := tryMatchByNameWithExternal(c.Name, value, matchOptions{}, nil)
	if ok {
		c.konnectID = id
	}
	return ok
}

func (c IdentityAuthServerChild) GetParentRef() *ResourceRef {
	if c.AuthServer == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeIdentityAuthServer, Ref: NormalizeResourceRef(c.AuthServer)}
}

func (c IdentityAuthServerChild) GetDependencies() []ResourceRef {
	if parent := c.GetParentRef(); parent != nil {
		return []ResourceRef{*parent}
	}
	return nil
}

func (c IdentityAuthServerChild) GetReferenceFieldMappings() map[string]string {
	return map[string]string{"auth_server": string(ResourceTypeIdentityAuthServer)}
}

func (c IdentityAuthServerChild) Validate() error {
	if err := ValidateRef(c.Ref); err != nil {
		return err
	}
	if c.Name == "" {
		return fmt.Errorf("auth-server child name is required")
	}
	return nil
}

type IdentityAuthServerScopeResource struct {
	IdentityAuthServerChild `yaml:",inline" json:",inline"`
	Description             *string `yaml:"description,omitempty" json:"description,omitempty"`
	Default                 *bool   `yaml:"default,omitempty" json:"default,omitempty"`
	IncludeInMetadata       *bool   `yaml:"include_in_metadata,omitempty" json:"include_in_metadata,omitempty"`
	Enabled                 *bool   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

func (s IdentityAuthServerScopeResource) GetType() ResourceType {
	return ResourceTypeIdentityAuthServerScope
}

type IdentityAuthServerClaimResource struct {
	IdentityAuthServerChild `yaml:",inline" json:",inline"`
	Value                   string   `yaml:"value" json:"value"`
	IncludeInToken          *bool    `yaml:"include_in_token,omitempty" json:"include_in_token,omitempty"`
	IncludeInAllScopes      *bool    `yaml:"include_in_all_scopes,omitempty" json:"include_in_all_scopes,omitempty"`
	IncludeInScopes         []string `yaml:"include_in_scopes,omitempty" json:"include_in_scopes,omitzero"`
	Enabled                 *bool    `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

func (c IdentityAuthServerClaimResource) GetType() ResourceType {
	return ResourceTypeIdentityAuthServerClaim
}

func (c IdentityAuthServerClaimResource) Validate() error {
	if err := c.IdentityAuthServerChild.Validate(); err != nil {
		return err
	}
	if c.Value == "" {
		return fmt.Errorf("claim value is required")
	}
	for _, value := range c.IncludeInScopes {
		if util.IsValidUUID(value) {
			continue
		}
		if _, selector, ok := tags.ParseRefPlaceholder(value); !ok || selector != "id" {
			return fmt.Errorf("include_in_scopes accepts scope UUIDs or !ref scope#id")
		}
	}
	return nil
}

func (c IdentityAuthServerClaimResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{
		"auth_server":       string(ResourceTypeIdentityAuthServer),
		"include_in_scopes": string(ResourceTypeIdentityAuthServerScope),
	}
}

func identityAuthServerByRef(rs *ResourceSet, ref string) *IdentityAuthServerResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	for i := range rs.Identity.AuthServers {
		if rs.Identity.AuthServers[i].Ref == NormalizeResourceRef(ref) {
			return &rs.Identity.AuthServers[i]
		}
	}
	return nil
}

func validateAuthServerChildren[R any, RPtr interface {
	*R
	Resource
	ResourceWithParent
}](
	rs *ResourceSet, children []R,
) error {
	if err := validateChildRefs[R, RPtr](rs, children); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for i := range children {
		child := RPtr(&children[i])
		parent := child.GetParentRef()
		if parent == nil {
			return fmt.Errorf("%s %q requires auth_server", child.GetType(), child.GetRef())
		}
		server := identityAuthServerByRef(rs, parent.Ref)
		if server == nil || server.IsExternal() {
			return fmt.Errorf("%s %q requires a declared managed auth server; external parents are read-only",
				child.GetType(), child.GetRef())
		}
		if claim, ok := any(child).(*IdentityAuthServerClaimResource); ok {
			for _, value := range claim.IncludeInScopes {
				ref, _, tagged := tags.ParseRefPlaceholder(value)
				if !tagged {
					continue
				}
				target, found := rs.GetResourceByRef(ref)
				if !found {
					continue
				}
				if target.GetType() != ResourceTypeIdentityAuthServerScope {
					return fmt.Errorf("claim scope reference must identify an auth-server scope")
				}
				scopeParent := target.(ResourceWithParent).GetParentRef()
				if scopeParent == nil || scopeParent.Ref != parent.Ref {
					return fmt.Errorf("claim scope reference must belong to the same auth server")
				}
			}
		}
		key := parent.Ref + "\x00" + child.GetMoniker()
		if seen[key] {
			return fmt.Errorf("duplicate %s name %q under auth server %q",
				child.GetType(), child.GetMoniker(), parent.Ref)
		}
		seen[key] = true
	}
	return nil
}

func identityScopeSlice(rs *ResourceSet) *[]IdentityAuthServerScopeResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	return &rs.Identity.Scopes
}

func ensureIdentityScopeSlice(rs *ResourceSet) *[]IdentityAuthServerScopeResource {
	if rs.Identity == nil {
		rs.Identity = &IdentityResource{}
	}
	return &rs.Identity.Scopes
}

func identityClaimSlice(rs *ResourceSet) *[]IdentityAuthServerClaimResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	return &rs.Identity.Claims
}

func ensureIdentityClaimSlice(rs *ResourceSet) *[]IdentityAuthServerClaimResource {
	if rs.Identity == nil {
		rs.Identity = &IdentityResource{}
	}
	return &rs.Identity.Claims
}

func init() {
	registerResourceTypeWithSliceAccessors(
		ResourceTypeIdentityAuthServerScope, identityScopeSlice, ensureIdentityScopeSlice,
		AutoExplain[IdentityAuthServerScopeResource](
			WithExplainAliases("identity.scopes", "identity.auth_servers.scopes", "auth-server-scopes"),
			WithExplainRecommendedFields("ref", "auth_server", "name"),
			WithExplainFieldHint("default", ExplainFieldHint{Default: false}),
			WithExplainFieldHint("enabled", ExplainFieldHint{Default: true}),
			WithExplainFieldHint("include_in_metadata", ExplainFieldHint{Default: false}),
		),
		WithNamespaceFrom(func(rs *ResourceSet, s *IdentityAuthServerScopeResource) *IdentityAuthServerResource {
			return identityAuthServerByRef(rs, s.AuthServer)
		}),
		WithChildSyncScope(ResourceTypeIdentityAuthServer), withReferenceValidation(110),
		WithExternalUnsupportedReason("scopes inherit managed auth-server ownership; use UUIDs for read-only scope links"),
		WithMaturity(maturity.Metadata{Level: maturity.LevelTechPreview}),
		withChildLoad(ResourceTypeIdentityAuthServerScope, ensureIdentityScopeSlice,
			childLoad[IdentityAuthServerScopeResource, IdentityAuthServerResource]{
				family: ResourceTypeIdentityAuthServer, extractOrder: 10, validateOrder: 10, validatePhase: 250,
				nested:    func(s *IdentityAuthServerResource) *[]IdentityAuthServerScopeResource { return &s.Scopes },
				setParent: func(c *IdentityAuthServerScopeResource, ref string) { c.AuthServer = ref },
				validate:  validateAuthServerChildren[IdentityAuthServerScopeResource],
			}),
	)
	registerResourceTypeWithSliceAccessors(
		ResourceTypeIdentityAuthServerClaim, identityClaimSlice, ensureIdentityClaimSlice,
		AutoExplain[IdentityAuthServerClaimResource](
			WithExplainAliases("identity.claims", "identity.auth_servers.claims", "auth-server-claims"),
			WithExplainRecommendedFields("ref", "auth_server", "name", "value"),
			WithExplainFieldHint("enabled", ExplainFieldHint{Default: true}),
			WithExplainFieldHint("include_in_token", ExplainFieldHint{Default: false}),
			WithExplainFieldHint("include_in_all_scopes", ExplainFieldHint{Default: false}),
		),
		WithNamespaceFrom(func(rs *ResourceSet, c *IdentityAuthServerClaimResource) *IdentityAuthServerResource {
			return identityAuthServerByRef(rs, c.AuthServer)
		}),
		WithChildSyncScope(ResourceTypeIdentityAuthServer), withReferenceValidation(120),
		WithExternalUnsupportedReason("claims inherit managed auth-server ownership and reject ambiguous names"),
		WithMaturity(maturity.Metadata{Level: maturity.LevelTechPreview}),
		withChildLoad(ResourceTypeIdentityAuthServerClaim, ensureIdentityClaimSlice,
			childLoad[IdentityAuthServerClaimResource, IdentityAuthServerResource]{
				family: ResourceTypeIdentityAuthServer, extractOrder: 20, validateOrder: 20, validatePhase: 260,
				nested:    func(s *IdentityAuthServerResource) *[]IdentityAuthServerClaimResource { return &s.Claims },
				setParent: func(c *IdentityAuthServerClaimResource, ref string) { c.AuthServer = ref },
				validate:  validateAuthServerChildren[IdentityAuthServerClaimResource],
			}),
	)
}
