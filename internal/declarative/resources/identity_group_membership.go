package resources

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/util"
)

// IdentityGroupMembershipResource inherits authority from its managed group.
// The entry ID routes requests; the principal ID identifies the relationship.
type IdentityGroupMembershipResource struct {
	Ref       string                      `yaml:"ref"             json:"ref"`
	Group     string                      `yaml:"group,omitempty" json:"group,omitempty"`
	Member    IdentityGroupMemberResource `yaml:"member"          json:"member"`
	konnectID string
}

type IdentityGroupMemberResource struct {
	Type string `yaml:"type" json:"type"`
	ID   string `yaml:"id"   json:"id"`
}

func identityGroupByRef(rs *ResourceSet, ref string) *IdentityGroupResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	for i := range rs.Identity.Groups {
		if rs.Identity.Groups[i].Ref == NormalizeResourceRef(ref) {
			return &rs.Identity.Groups[i]
		}
	}
	return nil
}

func init() {
	registerResourceTypeWithSliceAccessors(
		ResourceTypeIdentityGroupMembership,
		identityGroupMembershipSlice,
		ensureIdentityGroupMembershipSlice,
		AutoExplain[IdentityGroupMembershipResource](
			WithExplainAliases("identity.memberships", "group-membership", "group-memberships"),
			WithExplainRecommendedFields("ref", "group", "member"),
			WithExplainFieldHint("member.type", ExplainFieldHint{Enum: []any{"principal"}}),
			WithExplainFieldHint("member.id", ExplainFieldHint{RefKind: string(ResourceTypeIdentityPrincipal)}),
		),
		WithNamespaceFrom(func(rs *ResourceSet, m *IdentityGroupMembershipResource) *IdentityGroupResource {
			return identityGroupByRef(rs, m.Group)
		}),
		WithChildSyncScope(ResourceTypeIdentityGroup),
		withReferenceValidation(103),
		WithExternalUnsupportedReason(
			"memberships inherit managed ordinary-group ownership; external groups are read-only",
		),
		withChildLoad(ResourceTypeIdentityGroupMembership, ensureIdentityGroupMembershipSlice,
			childLoad[IdentityGroupMembershipResource, IdentityGroupResource]{
				family: ResourceTypeIdentityDirectory, extractOrder: 30, validateOrder: 40, validatePhase: 246,
				nested:    func(g *IdentityGroupResource) *[]IdentityGroupMembershipResource { return &g.Memberships },
				setParent: func(m *IdentityGroupMembershipResource, ref string) { m.Group = ref },
				validate:  validateIdentityGroupMemberships,
			}),
	)
}

func (m IdentityGroupMembershipResource) GetRef() string { return m.Ref }
func (m IdentityGroupMembershipResource) GetType() ResourceType {
	return ResourceTypeIdentityGroupMembership
}

func (m IdentityGroupMembershipResource) GetMoniker() string {
	return m.Member.Type + "/" + m.Member.ID
}
func (m IdentityGroupMembershipResource) GetKonnectID() string            { return m.konnectID }
func (m *IdentityGroupMembershipResource) SetKonnectID(id string)         { m.konnectID = id }
func (m *IdentityGroupMembershipResource) SetDefaults()                   {}
func (m IdentityGroupMembershipResource) GetKonnectMonikerFilter() string { return m.GetMoniker() }
func (m *IdentityGroupMembershipResource) TryMatchKonnectResource(value any) bool {
	id, ok := tryMatchByNameWithExternal(m.GetMoniker(), value, matchOptions{}, nil)
	if ok {
		m.konnectID = id
	}
	return ok
}

func (m IdentityGroupMembershipResource) GetParentRef() *ResourceRef {
	if m.Group == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeIdentityGroup, Ref: NormalizeResourceRef(m.Group)}
}

func (m IdentityGroupMembershipResource) GetDependencies() []ResourceRef {
	if p := m.GetParentRef(); p != nil {
		return []ResourceRef{*p}
	}
	return nil
}

func (m IdentityGroupMembershipResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{"group": string(ResourceTypeIdentityGroup)}
}

func (m IdentityGroupMembershipResource) Validate() error {
	if err := ValidateRef(m.Ref); err != nil {
		return err
	}
	if m.Member.Type != "principal" {
		return fmt.Errorf("group memberships require member.type principal; nested groups are deferred (Q32)")
	}
	if util.IsValidUUID(m.Member.ID) {
		return nil
	}
	if _, selector, ok := tags.ParseRefPlaceholder(m.Member.ID); !ok || selector != "id" {
		return fmt.Errorf("membership member.id requires a principal UUID or !ref principal#id")
	}
	return nil
}

func identityGroupMembershipSlice(rs *ResourceSet) *[]IdentityGroupMembershipResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	return &rs.Identity.Memberships
}

func ensureIdentityGroupMembershipSlice(rs *ResourceSet) *[]IdentityGroupMembershipResource {
	if rs.Identity == nil {
		rs.Identity = &IdentityResource{}
	}
	return &rs.Identity.Memberships
}

func validateIdentityGroupMemberships(rs *ResourceSet, values []IdentityGroupMembershipResource) error {
	seen := make(map[string]bool)
	for i := range values {
		m := &values[i]
		if err := validateCollectionResource(rs, m); err != nil {
			return err
		}
		group := identityGroupByRef(rs, m.Group)
		if group == nil || group.IsExternal() {
			return fmt.Errorf(
				"membership %q requires a declared managed ordinary group; external groups are read-only",
				m.Ref,
			)
		}
		if ref, _, tagged := tags.ParseRefPlaceholder(m.Member.ID); tagged {
			target, found := rs.GetResourceByRef(ref)
			if !found || target.GetType() != ResourceTypeIdentityPrincipal {
				return fmt.Errorf("membership %q must reference a declared principal", m.Ref)
			}

		}
		key := group.Ref + "\x00" + m.GetMoniker()
		if seen[key] {
			return fmt.Errorf("duplicate member under group %q", group.Ref)
		}
		seen[key] = true
	}
	return nil
}
