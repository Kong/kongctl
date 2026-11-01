package resources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"unicode/utf8"

	"github.com/kong/kongctl/internal/maturity"
)

var identityGroupNamePattern = regexp.MustCompile(`^[A-Za-z0-9]+$`)

type IdentityGroupResource struct {
	APIKeys      []IdentityGroupAPIKeyResource     `yaml:"api_keys,omitempty"    json:"api_keys,omitempty"`
	Memberships  []IdentityGroupMembershipResource `yaml:"memberships,omitempty" json:"memberships,omitempty"`
	BaseResource `                                  yaml:",inline"               json:",inline"`
	Directory    string            `yaml:"directory,omitempty"   json:"directory,omitempty"`
	Name         string            `yaml:"name"                  json:"name"`
	Description  *string           `yaml:"description,omitempty" json:"description,omitempty"`
	Type         *string           `yaml:"type,omitempty"        json:"type,omitempty"`
	Metadata     map[string]any    `yaml:"metadata,omitempty"    json:"metadata,omitempty"`
	Labels       map[string]string `yaml:"labels,omitempty"      json:"labels,omitempty"`
	External     *ExternalBlock    `yaml:"_external,omitempty"   json:"_external,omitempty"`
}

func init() {
	registerExternalResourceTypeWithSliceAccessors(
		ResourceTypeIdentityGroup,
		identityGroupSlice,
		ensureIdentityGroupSlice,
		AutoExplain[IdentityGroupResource](
			WithExplainAliases("identity.groups", "identity.group", "group", "groups"),
			WithExplainRecommendedFields("ref", "directory", "name"),
			WithExplainFieldHint("name", ExplainFieldHint{Literal: "MyGroup", Notes: []string{
				"Required: 1-255 ASCII letters or digits, without spaces or punctuation.",
			}}),
			WithExplainFieldHint("type", ExplainFieldHint{Enum: []any{"group"}, Notes: []string{
				"Only directly provisioned ordinary groups are managed; team/importer writes are deferred (Q32).",
			}}),
		),
		ExternalResolutionRegistration{
			Selectors: []string{SchemaFieldName}, ParentType: ResourceTypeIdentityDirectory,
			ParentFieldPath: "directory",
		},
		WithNamespace(212, groupNamespaceParticipant),
		WithNestedNamespaceGroups(func(rs *ResourceSet) [][]IdentityGroupResource {
			var result [][]IdentityGroupResource
			if rs.Identity != nil {
				for i := range rs.Identity.Directories {
					result = append(result, rs.Identity.Directories[i].Groups)
				}
			}
			return result
		}, groupNamespaceParticipant),
		WithIndependentChildOwnership(
			captureIdentityGroupScopes,
			func(g *IdentityGroupResource) *KongctlMeta { return g.Kongctl },
		),
		WithManagedBy(ManagedByPolicy{MaxEntries: 5}),
		WithChildSyncScope(ResourceTypeIdentityDirectory),
		withReferenceValidation(102),
		WithMaturity(
			maturity.Metadata{Level: maturity.LevelTechPreview, Message: "Requires the Kong Identity SDK preview."},
		),
		withChildLoad(
			ResourceTypeIdentityGroup, identityGroupSlice,
			childLoad[IdentityGroupResource, IdentityDirectoryResource]{
				family: ResourceTypeIdentityDirectory, extractOrder: 20, validateOrder: 30, validatePhase: 245,
				nested:    func(d *IdentityDirectoryResource) *[]IdentityGroupResource { return &d.Groups },
				setParent: func(g *IdentityGroupResource, ref string) { g.Directory = ref },
				validate:  validateIdentityGroups,
			},
		),
	)
}

func (g *IdentityGroupResource) UnmarshalJSON(data []byte) error {
	type plain IdentityGroupResource
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode((*plain)(g))
}

func (g IdentityGroupResource) GetType() ResourceType             { return ResourceTypeIdentityGroup }
func (g IdentityGroupResource) GetMoniker() string                { return g.Name }
func (g IdentityGroupResource) GetLabels() map[string]string      { return g.Labels }
func (g *IdentityGroupResource) SetLabels(v map[string]string)    { g.Labels = v }
func (g *IdentityGroupResource) SetDefaults()                     {}
func (g *IdentityGroupResource) GetExternalBlock() *ExternalBlock { return g.External }
func (g *IdentityGroupResource) IsExternal() bool                 { return g.External != nil }
func (g IdentityGroupResource) Protected() bool                   { return g.Kongctl.IsProtected() }
func (g IdentityGroupResource) GetKonnectMonikerFilter() string   { return g.Name }
func (g *IdentityGroupResource) TryMatchKonnectResource(value any) bool {
	id, ok := tryMatchByNameWithExternal(g.Name, value, matchOptions{}, g.External)
	if ok {
		g.SetKonnectID(id)
	}
	return ok
}

func (g IdentityGroupResource) GetParentRef() *ResourceRef {
	if g.Directory == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeIdentityDirectory, Ref: NormalizeResourceRef(g.Directory)}
}

func (g IdentityGroupResource) GetDependencies() []ResourceRef {
	if p := g.GetParentRef(); p != nil {
		return []ResourceRef{*p}
	}
	return nil
}

func (g IdentityGroupResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{"directory": string(ResourceTypeIdentityDirectory)}
}

func (g IdentityGroupResource) Validate() error {
	if err := ValidateRef(g.Ref); err != nil {
		return err
	}
	if g.IsExternal() {
		return g.External.Validate()
	}
	if len(g.Name) == 0 || len(g.Name) > 255 || !identityGroupNamePattern.MatchString(g.Name) {
		return fmt.Errorf("group name is required; use 1-255 ASCII letters or digits")
	}
	if g.Type != nil && *g.Type != "group" {
		return fmt.Errorf("only ordinary type group is supported; team/importer writes are deferred (Q32)")
	}
	if g.Description != nil && utf8.RuneCountInString(*g.Description) > 255 {
		return fmt.Errorf("group description must be at most 255 characters")
	}
	return ValidateIdentityMetadata(g.Metadata)
}

func groupNamespaceParticipant(g *IdentityGroupResource) NamespaceParticipant {
	return NamespaceParticipant{
		Ref: g.Ref, Label: string(ResourceTypeIdentityGroup), Meta: &g.Kongctl,
		SupportsProtected: true, External: g.IsExternal(),
	}
}

func identityGroupSlice(rs *ResourceSet) *[]IdentityGroupResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	return &rs.Identity.Groups
}

func ensureIdentityGroupSlice(rs *ResourceSet) *[]IdentityGroupResource {
	if rs.Identity == nil {
		rs.Identity = &IdentityResource{}
	}
	return &rs.Identity.Groups
}

func validateIdentityGroups(rs *ResourceSet, values []IdentityGroupResource) error {
	seen := make(map[string]bool)
	for i := range values {
		g := &values[i]
		if err := validateCollectionResource(rs, g); err != nil {
			return err
		}
		parent, ok := rs.GetResourceByRef(NormalizeResourceRef(g.Directory))
		if !ok || parent.GetType() != ResourceTypeIdentityDirectory {
			return fmt.Errorf("group %q must reference a declared directory", g.Ref)
		}
		key := NormalizeResourceRef(g.Directory) + "\x00" + g.Name
		if !g.IsExternal() && seen[key] {
			return fmt.Errorf("duplicate group name %q in directory %q", g.Name, g.Directory)
		}
		if !g.IsExternal() {
			seen[key] = true
		}
	}
	return nil
}

func captureIdentityGroupScopes(rs *ResourceSet, defaultNamespace string) {
	if rs.Identity == nil {
		return
	}
	scope := rs.EnsureSyncScope()
	for _, d := range rs.Identity.Directories {
		var namespaces []string
		for _, g := range d.Groups {
			if !g.IsExternal() {
				namespaces = append(namespaces, GetNamespace(g.Kongctl))
			}
		}
		if len(d.Groups) > 0 && len(namespaces) == 0 {
			continue
		}
		CaptureOwnedChildCollection(scope, ResourceTypeIdentityGroup, d.Ref, defaultNamespace,
			scope.ChildInScope(ResourceTypeIdentityDirectory, d.Ref, ResourceTypeIdentityGroup), namespaces)
	}
	for _, g := range rs.Identity.Groups {
		if !g.IsExternal() {
			scope.AddChildNamespace(ResourceTypeIdentityGroup, g.Directory, GetNamespace(g.Kongctl))
		}
	}
}
