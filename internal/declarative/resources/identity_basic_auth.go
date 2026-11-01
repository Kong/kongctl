package resources

import (
	"fmt"
	"unicode/utf8"

	"github.com/kong/kongctl/internal/maturity"
)

// Username records have a unique directory-wide moniker; refs remain local.
// Password children have a separate lifecycle and are not declared here.
type IdentityBasicAuthResource struct {
	BaseResource `               yaml:",inline"             json:",inline"`
	Principal    string         `yaml:"principal,omitempty" json:"principal,omitempty"`
	Username     string         `yaml:"username"            json:"username"`
	External     *ExternalBlock `yaml:"_external,omitempty" json:"_external,omitempty"`
}

func init() {
	registerExternalResourceTypeWithSliceAccessors(
		ResourceTypeIdentityBasicAuth,
		identityBasicAuthSlice,
		ensureIdentityBasicAuthSlice,
		AutoExplain[IdentityBasicAuthResource](
			WithExplainAliases("identity.basic_auths", "basic-auth", "basic-auths"),
			WithExplainRecommendedFields("ref", "principal", "username"),
			WithExplainFieldHint("username", ExplainFieldHint{Literal: "worker@example.test", Notes: []string{
				"Required, 1-255 characters; unique within a directory. Passwords are managed separately.",
			}}),
		),
		ExternalResolutionRegistration{
			Selectors: []string{SchemaFieldUsername}, ParentType: ResourceTypeIdentityPrincipal,
			ParentFieldPath: SchemaFieldPrincipal,
		},
		WithNamespace(213, basicAuthNamespaceParticipant),
		WithNestedNamespaceGroups(func(rs *ResourceSet) [][]IdentityBasicAuthResource {
			var groups [][]IdentityBasicAuthResource
			visitIdentityPrincipals(rs, func(p *IdentityPrincipalResource) { groups = append(groups, p.BasicAuths) })
			return groups
		}, basicAuthNamespaceParticipant),
		WithIndependentChildOwnership(captureBasicAuthScopes,
			func(b *IdentityBasicAuthResource) *KongctlMeta { return b.Kongctl }),
		WithManagedBy(ManagedByPolicy{MaxEntries: 5}),
		WithChildSyncScope(ResourceTypeIdentityPrincipal),
		withReferenceValidation(104),
		WithMaturity(
			maturity.Metadata{Level: maturity.LevelTechPreview, Message: "Requires the Kong Identity SDK preview."},
		),
		withChildLoad(ResourceTypeIdentityBasicAuth, ensureIdentityBasicAuthSlice,
			childLoad[IdentityBasicAuthResource, IdentityPrincipalResource]{
				family: ResourceTypeIdentityDirectory, extractOrder: 40, validateOrder: 50, validatePhase: 247,
				nested:    func(p *IdentityPrincipalResource) *[]IdentityBasicAuthResource { return &p.BasicAuths },
				setParent: func(b *IdentityBasicAuthResource, ref string) { b.Principal = ref },
				validate:  validateIdentityBasicAuths,
			}),
	)
}

func (b IdentityBasicAuthResource) GetType() ResourceType             { return ResourceTypeIdentityBasicAuth }
func (b IdentityBasicAuthResource) GetMoniker() string                { return b.Username }
func (b IdentityBasicAuthResource) GetLabels() map[string]string      { return nil }
func (b *IdentityBasicAuthResource) SetLabels(map[string]string)      {}
func (b *IdentityBasicAuthResource) SetDefaults()                     {}
func (b *IdentityBasicAuthResource) GetExternalBlock() *ExternalBlock { return b.External }
func (b *IdentityBasicAuthResource) IsExternal() bool                 { return b.External != nil }
func (b IdentityBasicAuthResource) Protected() bool                   { return b.Kongctl.IsProtected() }
func (b IdentityBasicAuthResource) GetKonnectMonikerFilter() string   { return b.Username }
func (b *IdentityBasicAuthResource) TryMatchKonnectResource(value any) bool {
	id, ok := tryMatchByNameWithExternal(b.Username, value, matchOptions{}, b.External)
	if ok {
		b.SetKonnectID(id)
	}
	return ok
}

func (b IdentityBasicAuthResource) GetParentRef() *ResourceRef {
	if b.Principal == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeIdentityPrincipal, Ref: NormalizeResourceRef(b.Principal)}
}

func (b IdentityBasicAuthResource) GetDependencies() []ResourceRef {
	if parent := b.GetParentRef(); parent != nil {
		return []ResourceRef{*parent}
	}
	return nil
}

func (b IdentityBasicAuthResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{"principal": string(ResourceTypeIdentityPrincipal)}
}

func (b IdentityBasicAuthResource) Validate() error {
	if err := ValidateRef(b.Ref); err != nil {
		return err
	}
	if b.IsExternal() {
		return b.External.Validate()
	}
	if !utf8.ValidString(b.Username) || utf8.RuneCountInString(b.Username) < 1 ||
		utf8.RuneCountInString(b.Username) > 255 {
		return fmt.Errorf("basic-auth username is required; use 1-255 characters")
	}
	return nil
}

func basicAuthNamespaceParticipant(b *IdentityBasicAuthResource) NamespaceParticipant {
	return NamespaceParticipant{
		Ref: b.Ref, Label: string(ResourceTypeIdentityBasicAuth), Meta: &b.Kongctl,
		SupportsProtected: true, External: b.IsExternal(),
	}
}

func identityBasicAuthSlice(rs *ResourceSet) *[]IdentityBasicAuthResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	return &rs.Identity.BasicAuths
}

func ensureIdentityBasicAuthSlice(rs *ResourceSet) *[]IdentityBasicAuthResource {
	if rs.Identity == nil {
		rs.Identity = &IdentityResource{}
	}
	return &rs.Identity.BasicAuths
}

func validateIdentityBasicAuths(rs *ResourceSet, values []IdentityBasicAuthResource) error {
	seen := make(map[string]bool)
	for idx := range values {
		b := &values[idx]
		if err := validateCollectionResource(rs, b); err != nil {
			return err
		}
		parent, ok := rs.GetResourceByRef(NormalizeResourceRef(b.Principal))
		if !ok || parent.GetType() != ResourceTypeIdentityPrincipal {
			return fmt.Errorf("basic-auth %q must reference a declared principal", b.Ref)
		}
		principal := parent.(*IdentityPrincipalResource)
		key := NormalizeResourceRef(principal.Directory) + "\x00" + b.Username
		if !b.IsExternal() && seen[key] {
			return fmt.Errorf("duplicate basic-auth username %q in the same directory", b.Username)
		}
		if !b.IsExternal() {
			seen[key] = true
		}
	}
	return nil
}

func captureBasicAuthScopes(rs *ResourceSet, defaultNamespace string) {
	if rs.Identity == nil {
		return
	}
	scope := rs.EnsureSyncScope()
	visitIdentityPrincipals(rs, func(p *IdentityPrincipalResource) {
		var namespaces []string
		for _, b := range p.BasicAuths {
			if !b.IsExternal() {
				namespaces = append(namespaces, GetNamespace(b.Kongctl))
			}
		}
		if len(p.BasicAuths) != 0 && len(namespaces) == 0 {
			return
		}
		CaptureOwnedChildCollection(scope, ResourceTypeIdentityBasicAuth, p.Ref, defaultNamespace,
			scope.ChildInScope(ResourceTypeIdentityPrincipal, p.Ref, ResourceTypeIdentityBasicAuth), namespaces)
	})
	for _, b := range rs.Identity.BasicAuths {
		if !b.IsExternal() {
			scope.AddChildNamespace(ResourceTypeIdentityBasicAuth, b.Principal, GetNamespace(b.Kongctl))
		}
	}
}
