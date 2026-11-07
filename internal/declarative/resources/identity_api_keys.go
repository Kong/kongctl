package resources

import (
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"
)

const (
	IdentityAPIKeyCreateTypeKey = "kongctl-create-type" //nolint:gosec // Public metadata key, not credential material.
	IdentityAPIKeyGeneratedType = "v1"
)

// IdentityAPIKeyFields records generated creation intent. Lifecycle authority
// comes from the current managed parent, never copied namespace/protection.
type IdentityAPIKeyFields struct {
	Ref             string     `yaml:"ref"                  json:"ref"`
	Type            string     `yaml:"type"                 json:"type"`
	Name            *string    `yaml:"name,omitempty"       json:"name,omitempty"`
	ExpiresAt       *time.Time `yaml:"expires_at,omitempty" json:"expires_at,omitempty"`
	konnectID       string
	nameDeclared    bool
	expiresDeclared bool
}

func (k IdentityAPIKeyFields) GetRef() string                    { return k.Ref }
func (k IdentityAPIKeyFields) GetMoniker() string                { return k.Ref }
func (k IdentityAPIKeyFields) GetKonnectID() string              { return k.konnectID }
func (k *IdentityAPIKeyFields) SetKonnectID(id string)           { k.konnectID = id }
func (k *IdentityAPIKeyFields) SetDefaults()                     {}
func (k IdentityAPIKeyFields) GetKonnectMonikerFilter() string   { return k.Ref }
func (k *IdentityAPIKeyFields) TryMatchKonnectResource(any) bool { return false }
func (k IdentityAPIKeyFields) NameDeclared() bool                { return k.nameDeclared || k.Name != nil }

func (k IdentityAPIKeyFields) ExpirationDeclared() bool {
	return k.expiresDeclared || k.ExpiresAt != nil
}
func (k *IdentityAPIKeyFields) DeclareNoExpiration() { k.ExpiresAt = nil; k.expiresDeclared = true }

func (k IdentityAPIKeyFields) Validate() error {
	if err := ValidateRef(k.Ref); err != nil {
		return err
	}
	if k.Type != IdentityAPIKeyGeneratedType {
		return fmt.Errorf(
			"generated API keys require explicit type: v1; imported keys use explicit credential commands",
		)
	}
	if k.NameDeclared() && k.Name == nil {
		return fmt.Errorf("API-key null name clearing is unsupported (Q29); omit name to preserve it")
	}
	if k.Name != nil && (!utf8.ValidString(*k.Name) || utf8.RuneCountInString(*k.Name) > 255) {
		return fmt.Errorf("API-key name must contain at most 255 UTF-8 characters")
	}
	return nil
}

func (k *IdentityAPIKeyFields) observeFields(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	_, k.nameDeclared = fields["name"]
	_, k.expiresDeclared = fields["expires_at"]
	return nil
}

func (k IdentityAPIKeyFields) marshal(data []byte) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if k.NameDeclared() && k.Name == nil {
		fields["name"] = json.RawMessage("null")
	}
	if k.ExpirationDeclared() && k.ExpiresAt == nil {
		fields["expires_at"] = json.RawMessage("null")
	}
	return json.Marshal(fields)
}

// IdentityAPIKeyRef recognizes the generated-key convention without granting
// lifecycle authority. Unknown and legacy maps stay outside this convention.
func IdentityAPIKeyRef(metadata map[string]string) (string, bool, error) {
	if metadata[ManagedByToolKey] != ManagedByTool ||
		metadata[IdentityAPIKeyCreateTypeKey] != IdentityAPIKeyGeneratedType {
		return "", false, nil
	}
	if len(metadata) != 4 && len(metadata) != 5 {
		return "", false, fmt.Errorf("API key has an incompatible generated identity marker shape")
	}
	if namespace, found := metadata[ChildNamespaceKey]; found {
		if namespace == "" || len(metadata) != 5 {
			return "", false, fmt.Errorf("API key has invalid independent namespace metadata")
		}
	} else if len(metadata) != 4 {
		return "", false, fmt.Errorf("API key has incompatible generated identity marker shape")
	}
	if _, found := metadata[ChildProtectedKey]; found {
		return "", false, fmt.Errorf("API key has incompatible independent protection metadata")
	}
	ref := metadata[ManagedByRefKey]
	if err := ValidateRef(ref); err != nil || metadata[ManagedByVersionKey] == "" {
		return "", false, fmt.Errorf("API key has invalid generated identity or creator metadata")
	}
	return ref, true, nil
}

type IdentityPrincipalAPIKeyResource struct {
	IdentityAPIKeyFields `       yaml:",inline"             json:",inline"`
	Principal            string       `yaml:"principal,omitempty" json:"principal,omitempty"`
	Kongctl              *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
}

type IdentityGroupAPIKeyResource struct {
	IdentityAPIKeyFields `       yaml:",inline"         json:",inline"`
	Group                string `yaml:"group,omitempty" json:"group,omitempty"`
}

func (k *IdentityPrincipalAPIKeyResource) UnmarshalJSON(data []byte) error {
	type plain IdentityPrincipalAPIKeyResource
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*k = IdentityPrincipalAPIKeyResource(value)
	return k.observeFields(data)
}

func (k IdentityPrincipalAPIKeyResource) MarshalJSON() ([]byte, error) {
	type plain IdentityPrincipalAPIKeyResource
	data, err := json.Marshal(plain(k))
	if err != nil {
		return nil, err
	}
	return k.marshal(data)
}

func (k *IdentityGroupAPIKeyResource) UnmarshalJSON(data []byte) error {
	type plain IdentityGroupAPIKeyResource
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*k = IdentityGroupAPIKeyResource(value)
	return k.observeFields(data)
}

func (k IdentityGroupAPIKeyResource) MarshalJSON() ([]byte, error) {
	type plain IdentityGroupAPIKeyResource
	data, err := json.Marshal(plain(k))
	if err != nil {
		return nil, err
	}
	return k.marshal(data)
}

func (k IdentityPrincipalAPIKeyResource) GetType() ResourceType {
	return ResourceTypeIdentityPrincipalAPIKey
}

func (k IdentityGroupAPIKeyResource) GetType() ResourceType { return ResourceTypeIdentityGroupAPIKey }

func (k IdentityPrincipalAPIKeyResource) GetParentRef() *ResourceRef {
	if k.Principal == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeIdentityPrincipal, Ref: NormalizeResourceRef(k.Principal)}
}

func (k IdentityGroupAPIKeyResource) GetParentRef() *ResourceRef {
	if k.Group == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeIdentityGroup, Ref: NormalizeResourceRef(k.Group)}
}

func (k IdentityPrincipalAPIKeyResource) GetDependencies() []ResourceRef {
	if p := k.GetParentRef(); p != nil {
		return []ResourceRef{*p}
	}
	return nil
}

func (k IdentityGroupAPIKeyResource) GetDependencies() []ResourceRef {
	if p := k.GetParentRef(); p != nil {
		return []ResourceRef{*p}
	}
	return nil
}

func (k IdentityPrincipalAPIKeyResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{SchemaFieldPrincipal: string(ResourceTypeIdentityPrincipal)}
}

func (k IdentityGroupAPIKeyResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{SchemaFieldGroup: string(ResourceTypeIdentityGroup)}
}

func identityPrincipalByRef(rs *ResourceSet, ref string) *IdentityPrincipalResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	var found *IdentityPrincipalResource
	visitIdentityPrincipals(rs, func(p *IdentityPrincipalResource) {
		if p.Ref == NormalizeResourceRef(ref) {
			found = p
		}
	})
	return found
}

func principalAPIKeySlice(rs *ResourceSet) *[]IdentityPrincipalAPIKeyResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	return &rs.Identity.PrincipalAPIKeys
}

func ensurePrincipalAPIKeySlice(rs *ResourceSet) *[]IdentityPrincipalAPIKeyResource {
	if rs.Identity == nil {
		rs.Identity = &IdentityResource{}
	}
	return &rs.Identity.PrincipalAPIKeys
}

func groupAPIKeySlice(rs *ResourceSet) *[]IdentityGroupAPIKeyResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	return &rs.Identity.GroupAPIKeys
}

func ensureGroupAPIKeySlice(rs *ResourceSet) *[]IdentityGroupAPIKeyResource {
	if rs.Identity == nil {
		rs.Identity = &IdentityResource{}
	}
	return &rs.Identity.GroupAPIKeys
}

func init() {
	policy := ManagedByPolicy{
		MaxEntries: 5, PersistsRef: true, CreateOnly: true,
		CreationValues: map[string]string{IdentityAPIKeyCreateTypeKey: IdentityAPIKeyGeneratedType},
	}
	registerResourceTypeWithSliceAccessors(
		ResourceTypeIdentityPrincipalAPIKey,
		principalAPIKeySlice,
		ensurePrincipalAPIKeySlice,
		AutoExplain[IdentityPrincipalAPIKeyResource](
			WithExplainAliases("identity.principal_api_keys", "principal-api-key", "principal-api-keys"),
			WithExplainRecommendedFields("ref", "principal", "type"),
			WithExplainFieldHint("type", ExplainFieldHint{Enum: []any{IdentityAPIKeyGeneratedType}}),
			WithExplainFieldHint(
				"expires_at",
				ExplainFieldHint{
					Literal: "2030-01-01T00:00:00Z",
					Notes:   []string{"Omission preserves expiration; null clears it."},
				},
			),
		),
		withPrincipalAPIKeyNamespaces(),
		WithManagedBy(ManagedByPolicy{
			MaxEntries: 5, PersistsRef: true, PersistsNamespace: true, CreateOnly: true,
			CreationValues: map[string]string{IdentityAPIKeyCreateTypeKey: IdentityAPIKeyGeneratedType},
		}),
		WithChildSyncScope(ResourceTypeIdentityPrincipal),
		withReferenceValidation(111),
		WithExternalUnsupportedReason(
			"API-key adoption is unavailable; declare generated keys under a managed or external principal",
		),
		withChildLoad(ResourceTypeIdentityPrincipalAPIKey, ensurePrincipalAPIKeySlice,
			childLoad[IdentityPrincipalAPIKeyResource, IdentityPrincipalResource]{
				family:       ResourceTypeIdentityDirectory,
				extractOrder: 50, validateOrder: 64, validatePhase: 252,
				nested:    func(p *IdentityPrincipalResource) *[]IdentityPrincipalAPIKeyResource { return &p.APIKeys },
				setParent: func(k *IdentityPrincipalAPIKeyResource, ref string) { k.Principal = ref },
				validate: func(rs *ResourceSet, values []IdentityPrincipalAPIKeyResource) error {
					for i := range values {
						if err := validateCollectionResource(rs, &values[i]); err != nil {
							return err
						}
						p := identityPrincipalByRef(rs, values[i].Principal)
						if p == nil {
							return fmt.Errorf("API key %q requires a declared principal", values[i].Ref)
						}
						meta := values[i].Kongctl
						if meta != nil && meta.Protected != nil {
							return fmt.Errorf("API key %q cannot set protection: API-key management metadata is immutable", values[i].Ref)
						}
						if !p.IsExternal() && meta != nil && meta.NamespaceOrigin == NamespaceOriginExplicit {
							return fmt.Errorf("API key %q inherits its managed principal namespace", values[i].Ref)
						}
						if !p.IsExternal() {
							values[i].Kongctl = nil
						}
					}
					return nil
				},
			}),
	)
	registerResourceTypeWithSliceAccessors(
		ResourceTypeIdentityGroupAPIKey,
		groupAPIKeySlice,
		ensureGroupAPIKeySlice,
		AutoExplain[IdentityGroupAPIKeyResource](
			WithExplainAliases("identity.group_api_keys", "group-api-key", "group-api-keys"),
			WithExplainRecommendedFields("ref", "group", "type"),
			WithExplainFieldHint("type", ExplainFieldHint{Enum: []any{IdentityAPIKeyGeneratedType}}),
			WithExplainFieldHint("name", ExplainFieldHint{Notes: []string{"Group key names are create-only (Q29)."}}),
			WithExplainFieldHint(
				"expires_at",
				ExplainFieldHint{
					Literal: "2030-01-01T00:00:00Z",
					Notes:   []string{"Omission preserves expiration; null clears it."},
				},
			),
		),
		WithNamespaceFrom(func(rs *ResourceSet, k *IdentityGroupAPIKeyResource) *IdentityGroupResource {
			return identityGroupByRef(rs, k.Group)
		}),
		WithManagedBy(policy),
		WithChildSyncScope(ResourceTypeIdentityGroup),
		withReferenceValidation(112),
		WithExternalUnsupportedReason(
			"generated API keys require managed-parent authority; imported key adoption is unavailable",
		),
		withChildLoad(ResourceTypeIdentityGroupAPIKey, ensureGroupAPIKeySlice,
			childLoad[IdentityGroupAPIKeyResource, IdentityGroupResource]{
				family:       ResourceTypeIdentityDirectory,
				extractOrder: 60, validateOrder: 65, validatePhase: 253,
				nested:    func(g *IdentityGroupResource) *[]IdentityGroupAPIKeyResource { return &g.APIKeys },
				setParent: func(k *IdentityGroupAPIKeyResource, ref string) { k.Group = ref },
				validate: func(rs *ResourceSet, values []IdentityGroupAPIKeyResource) error {
					for i := range values {
						if err := validateCollectionResource(rs, &values[i]); err != nil {
							return err
						}
						g := identityGroupByRef(rs, values[i].Group)
						if g == nil || g.IsExternal() {
							return fmt.Errorf("API key %q requires a declared managed ordinary group", values[i].Ref)
						}
					}
					return nil
				},
			}),
	)
}
