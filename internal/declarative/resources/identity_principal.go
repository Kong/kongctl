package resources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/kong/kongctl/internal/util"
)

func init() {
	registerExternalResourceTypeWithSliceAccessors(
		ResourceTypeIdentityPrincipal,
		identityPrincipalSlice,
		ensureIdentityPrincipalSlice,
		AutoExplain[IdentityPrincipalResource](
			WithExplainAliases("identity.principals", "principal", "principals"),
			WithExplainRecommendedFields("ref", "directory", "display_name"),
			WithExplainFieldHint("id", ExplainFieldHint{Notes: []string{
				"Optional assertion against an existing managed principal's UUID. " +
					"The namespace/ref match must already exist; no creation or adoption is authorized by this field.",
			}}),
			WithExplainFieldHint("metadata_mode", ExplainFieldHint{
				Enum: []any{IdentityMetadataModeMerge, IdentityMetadataModeSync},
				Notes: []string{
					"merge preserves unmentioned keys using fresh state; sync replaces the complete metadata map. " +
						"Omission retains established replacement behavior. Requires explicit metadata.",
				},
			}),
			WithExplainFieldHint(
				"active",
				ExplainFieldHint{Notes: []string{
					"Omission preserves the current state. Requires API responses exposing active. " +
						"Developer-portal principals cannot be deactivated.",
				}},
			),
			WithExplainFieldHint(
				"metadata",
				ExplainFieldHint{Notes: []string{
					"Signed 64-bit integers are preserved exactly. " +
						"Use strings for exact identifiers.",
				}},
			),
			WithExplainFieldHint(
				"type",
				ExplainFieldHint{
					Enum: []any{"user", "agent", nil},
					Notes: []string{
						"Create-only until API question Q9 is resolved. Omission preserves an existing type; null declares unclassified.",
					},
				},
			),
		),
		ExternalResolutionRegistration{
			Selectors:  []string{SchemaFieldID},
			ParentType: ResourceTypeIdentityDirectory, ParentFieldPath: "directory",
		},
		WithNamespace(210, principalNamespaceParticipant),
		WithNestedNamespaceGroups(func(rs *ResourceSet) [][]IdentityPrincipalResource {
			var groups [][]IdentityPrincipalResource
			if rs.Identity != nil {
				for i := range rs.Identity.Directories {
					groups = append(groups, rs.Identity.Directories[i].Principals)
				}
			}
			return groups
		}, principalNamespaceParticipant),
		WithIndependentChildOwnership(
			capturePrincipalScopes,
			func(p *IdentityPrincipalResource) *KongctlMeta { return p.Kongctl },
		),
		WithManagedBy(ManagedByPolicy{MaxEntries: 5, PersistsRef: true}),
		WithChildSyncScope(ResourceTypeIdentityDirectory),
		withReferenceValidation(100),
		withChildLoad(
			ResourceTypeIdentityPrincipal,
			identityPrincipalSlice,
			childLoad[IdentityPrincipalResource, IdentityDirectoryResource]{
				family: ResourceTypeIdentityDirectory, extractOrder: 10, validateOrder: 10, validatePhase: 230,
				nested:    func(d *IdentityDirectoryResource) *[]IdentityPrincipalResource { return &d.Principals },
				setParent: func(p *IdentityPrincipalResource, ref string) { p.Directory = ref },
				validate:  validateIdentityPrincipals,
			},
		),
	)
}

// IdentityPrincipalResource uses a persisted ref, never display_name, as identity.
type IdentityPrincipalResource struct {
	APIKeys      []IdentityPrincipalAPIKeyResource   `yaml:"api_keys,omitempty"     json:"api_keys,omitempty"`
	BasicAuths   []IdentityBasicAuthResource         `yaml:"basic_auths,omitempty"  json:"basic_auths,omitempty"`
	Identities   []IdentityPrincipalIdentityResource `yaml:"identities,omitempty"   json:"identities,omitempty"`
	External     *ExternalBlock                      `yaml:"_external,omitempty"    json:"_external,omitempty"`
	BaseResource `                                    yaml:",inline"                json:",inline"`
	Directory    string            `yaml:"directory,omitempty"    json:"directory,omitempty"`
	ID           string            `yaml:"id,omitempty"           json:"id,omitempty"`
	DisplayName  *string           `yaml:"display_name,omitempty" json:"display_name,omitempty"`
	Description  *string           `yaml:"description,omitempty"  json:"description,omitempty"`
	Type         *string           `yaml:"type,omitempty"         json:"type,omitempty"`
	Metadata     map[string]any    `yaml:"metadata,omitempty"     json:"metadata,omitempty"`
	MetadataMode string            `yaml:"metadata_mode,omitempty" json:"metadata_mode,omitempty"`
	Labels       map[string]string `yaml:"labels,omitempty"       json:"labels,omitempty"`
	Active       *bool             `yaml:"active,omitempty"       json:"active,omitempty"`
	typeDeclared bool
}

func (p *IdentityPrincipalResource) UnmarshalJSON(data []byte) error {
	type plain IdentityPrincipalResource
	var value plain
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if raw, present := fields["active"]; present && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("principal active must be a boolean")
	}
	if _, present := fields["metadata_mode"]; present && value.MetadataMode == "" {
		return fmt.Errorf("principal metadata_mode must be merge or sync")
	}
	if raw, present := fields["id"]; present && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("principal id assertion must be a UUID")
	}
	*p = IdentityPrincipalResource(value)
	_, p.typeDeclared = fields["type"]
	return nil
}

func (p IdentityPrincipalResource) MarshalJSON() ([]byte, error) {
	type plain IdentityPrincipalResource
	data, err := json.Marshal(plain(p))
	if err != nil {
		return nil, err
	}
	if !p.TypeDeclared() && p.Metadata == nil {
		return data, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if p.TypeDeclared() {
		typeData, err := json.Marshal(p.Type)
		if err != nil {
			return nil, err
		}
		fields["type"] = typeData
	}
	if p.Metadata != nil {
		metadata, err := json.Marshal(p.Metadata)
		if err != nil {
			return nil, err
		}
		fields["metadata"] = metadata
	}
	return json.Marshal(fields)
}

func (p IdentityPrincipalResource) TypeDeclared() bool { return p.typeDeclared || p.Type != nil }

func (p *IdentityPrincipalResource) DeclareUnclassified() { p.Type = nil; p.typeDeclared = true }

func (p IdentityPrincipalResource) GetType() ResourceType             { return ResourceTypeIdentityPrincipal }
func (p IdentityPrincipalResource) GetMoniker() string                { return p.Ref }
func (p IdentityPrincipalResource) GetLabels() map[string]string      { return p.Labels }
func (p *IdentityPrincipalResource) SetLabels(v map[string]string)    { p.Labels = v }
func (p *IdentityPrincipalResource) SetDefaults()                     {}
func (p *IdentityPrincipalResource) GetExternalBlock() *ExternalBlock { return p.External }
func (p *IdentityPrincipalResource) IsExternal() bool                 { return p.External != nil }
func (p IdentityPrincipalResource) Protected() bool                   { return p.Kongctl.IsProtected() }
func (p IdentityPrincipalResource) GetKonnectMonikerFilter() string   { return p.Ref }
func (p *IdentityPrincipalResource) TryMatchKonnectResource(value any) bool {
	candidate, ok := value.(interface {
		GetID() string
		GetManagedBy() map[string]string
	})
	if !ok {
		return false
	}
	metadata := candidate.GetManagedBy()
	if metadata[ManagedByToolKey] != ManagedByTool || metadata[ManagedByRefKey] != p.Ref ||
		metadata[ChildNamespaceKey] != GetNamespace(p.Kongctl) || candidate.GetID() == "" {
		return false
	}
	p.SetKonnectID(candidate.GetID())
	return true
}

func (p IdentityPrincipalResource) GetParentRef() *ResourceRef {
	if p.Directory == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeIdentityDirectory, Ref: NormalizeResourceRef(p.Directory)}
}

func (p IdentityPrincipalResource) GetDependencies() []ResourceRef {
	if parent := p.GetParentRef(); parent != nil {
		return []ResourceRef{*parent}
	}
	return nil
}

func (p IdentityPrincipalResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{"directory": string(ResourceTypeIdentityDirectory)}
}

func (p IdentityPrincipalResource) Validate() error {
	if err := ValidateRef(p.Ref); err != nil {
		return err
	}
	if p.IsExternal() {
		if p.MetadataMode != "" || p.ID != "" {
			return fmt.Errorf("external principals cannot request managed UUID assertions or metadata mutation modes")
		}
		if err := p.External.Validate(); err != nil {
			return err
		}
		if !util.IsValidUUID(p.External.ID) || p.External.Selector != nil {
			return fmt.Errorf(
				"external principal requires an explicit UUID in _external.id; display names are not unique",
			)
		}
		return nil
	}
	if p.ID != "" && !util.IsValidUUID(p.ID) {
		return fmt.Errorf("principal id assertion must be a UUID")
	}
	if p.MetadataMode != "" {
		if p.MetadataMode != IdentityMetadataModeMerge && p.MetadataMode != IdentityMetadataModeSync {
			return fmt.Errorf("principal metadata_mode must be merge or sync")
		}
		if p.Metadata == nil {
			return fmt.Errorf("principal metadata_mode requires explicit metadata")
		}
	}
	if p.Type != nil && *p.Type != "user" && *p.Type != "agent" {
		return fmt.Errorf("type must be user, agent, or null")
	}
	for _, field := range []struct {
		name  string
		value *string
	}{{"display_name", p.DisplayName}, {"description", p.Description}} {
		if field.value != nil && utf8.RuneCountInString(*field.value) > 255 {
			return fmt.Errorf("%s must be at most 255 characters", field.name)
		}
	}
	return ValidateIdentityMetadata(p.Metadata)
}

func validPrincipalMetadata(value any) bool {
	switch v := value.(type) {
	case string, bool, int, int32, int64:
		return true
	case json.Number:
		_, err := v.Int64()
		return err == nil
	case float64:
		return v == float64(int64(v))
	case []string, []int, []int64:
		return true
	case []any:
		kind := ""
		for _, entry := range v {
			next := "integer"
			if _, ok := entry.(string); ok {
				next = "string"
			} else {
				switch n := entry.(type) {
				case int, int64:
				case json.Number:
					if _, err := n.Int64(); err != nil {
						return false
					}
				case float64:
					if n != float64(int64(n)) {
						return false
					}
				default:
					return false
				}
			}
			if kind != "" && kind != next {
				return false
			}
			kind = next
		}
		return true
	default:
		return false
	}
}

func principalNamespaceParticipant(p *IdentityPrincipalResource) NamespaceParticipant {
	return NamespaceParticipant{
		Ref:               p.Ref,
		Label:             string(ResourceTypeIdentityPrincipal),
		Meta:              &p.Kongctl,
		SupportsProtected: true,
		External:          p.IsExternal(),
	}
}

func identityPrincipalSlice(rs *ResourceSet) *[]IdentityPrincipalResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	return &rs.Identity.Principals
}

func ensureIdentityPrincipalSlice(rs *ResourceSet) *[]IdentityPrincipalResource {
	if rs.Identity == nil {
		rs.Identity = &IdentityResource{}
	}
	return &rs.Identity.Principals
}

func validateIdentityPrincipals(rs *ResourceSet, values []IdentityPrincipalResource) error {
	for i := range values {
		p := &values[i]
		if err := validateCollectionResource(rs, p); err != nil {
			return err
		}
		parent, ok := rs.GetResourceByRef(NormalizeResourceRef(p.Directory))
		if !ok || parent.GetType() != ResourceTypeIdentityDirectory {
			return fmt.Errorf("principal %q must reference a declared directory (managed or _external)", p.Ref)
		}
	}
	return nil
}

func capturePrincipalScopes(rs *ResourceSet, defaultNamespace string) {
	if rs.Identity == nil {
		return
	}
	scope := rs.EnsureSyncScope()
	if scope.RootInScope(ResourceTypeIdentityDirectory) {
		scope.IdentityDirectoryNamespaces = make(map[string]struct{})
		if len(rs.Identity.Directories) == 0 {
			scope.IdentityDirectoryNamespaces[defaultNamespace] = struct{}{}
		}
		for _, d := range rs.Identity.Directories {
			if !d.IsExternal() {
				scope.IdentityDirectoryNamespaces[GetNamespace(d.Kongctl)] = struct{}{}
			}
		}
	}
	for _, d := range rs.Identity.Directories {
		var namespaces []string
		for _, p := range d.Principals {
			if !p.IsExternal() {
				namespaces = append(namespaces, GetNamespace(p.Kongctl))
			}
		}
		if len(d.Principals) != 0 && len(namespaces) == 0 {
			continue
		}
		CaptureOwnedChildCollection(scope, ResourceTypeIdentityPrincipal, d.Ref, defaultNamespace,
			scope.ChildInScope(ResourceTypeIdentityDirectory, d.Ref, ResourceTypeIdentityPrincipal), namespaces)
	}
	for _, p := range rs.Identity.Principals {
		if !p.IsExternal() {
			scope.AddChildNamespace(ResourceTypeIdentityPrincipal, p.Directory, GetNamespace(p.Kongctl))
		}
	}
}

func BuildPrincipalManagedBy(namespace, ref string, protected bool, version string) map[string]string {
	result := BuildChildManagedBy(namespace, protected, version)
	result[ManagedByRefKey] = ref
	return result
}
