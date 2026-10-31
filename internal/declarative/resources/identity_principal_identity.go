package resources

import (
	"fmt"
	"slices"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/maturity"
	"github.com/kong/kongctl/internal/util"
)

// IdentityPrincipalIdentityResource declares an association, not a credential.
// Its persisted ref identifies it independently of the external match fields.
type IdentityPrincipalIdentityResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	External     *ExternalBlock `yaml:"_external,omitempty"        json:"_external,omitempty"`
	Principal    string         `yaml:"principal,omitempty"        json:"principal,omitempty"`
	Type         string         `yaml:"type"                       json:"type"`
	Issuer       *string        `yaml:"issuer,omitempty"           json:"issuer,omitempty"`

	Claim *kkComps.KongPrincipalIdentityOIDCClaim `yaml:"claim,omitempty"            json:"claim,omitempty"`

	AuthServerID   *string           `yaml:"auth_server_id,omitempty"   json:"auth_server_id,omitempty"`
	ClientID       *string           `yaml:"client_id,omitempty"        json:"client_id,omitempty"`
	ControlPlaneID *string           `yaml:"control_plane_id,omitempty" json:"control_plane_id,omitempty"`
	ConsumerID     *string           `yaml:"consumer_id,omitempty"      json:"consumer_id,omitempty"`
	WorkspaceID    *string           `yaml:"workspace_id,omitempty"     json:"workspace_id,omitempty"`
	Key            *string           `yaml:"key,omitempty"              json:"key,omitempty"`
	Value          *string           `yaml:"value,omitempty"            json:"value,omitempty"`
	Labels         map[string]string `yaml:"labels,omitempty"           json:"labels,omitzero"`
}

func init() {
	registerExternalResourceTypeWithSliceAccessors(
		ResourceTypeIdentityPrincipalIdentity,
		identityPrincipalIdentitySlice,
		ensureIdentityPrincipalIdentitySlice,
		AutoExplain[IdentityPrincipalIdentityResource](
			WithExplainSchemaBuilder(principalIdentityExplainNode),
			WithExplainAliases("identity.identities", "principal-identity", "principal-identities"),
			WithExplainRecommendedFields("ref", "principal", "type"),
			WithExplainFieldHint("auth_server_id", ExplainFieldHint{RefKind: string(ResourceTypeIdentityAuthServer)}),
			WithExplainFieldHint("client_id", ExplainFieldHint{RefKind: string(ResourceTypeIdentityAuthServerClient)}),
			WithExplainFieldHint("consumer_id", ExplainFieldHint{RefKind: string(ResourceTypeAIGatewayConsumer)}),
			WithExplainFieldHint(
				"workspace_id",
				ExplainFieldHint{
					Notes: []string{
						"Configuration is deferred pending a verified API contract (Q25). Reads preserve the field.",
					},
				},
			),
			WithExplainFieldHint(
				"type",
				ExplainFieldHint{Enum: []any{"oidc", "auth_server_client", "control_plane_consumer", "custom"}},
			),
		),
		ExternalResolutionRegistration{
			Selectors: []string{SchemaFieldID}, ParentType: ResourceTypeIdentityPrincipal,
			ParentFieldPath: "principal",
		},
		WithNamespace(211, principalIdentityNamespaceParticipant),
		WithNestedNamespaceGroups(func(rs *ResourceSet) [][]IdentityPrincipalIdentityResource {
			var groups [][]IdentityPrincipalIdentityResource
			visitIdentityPrincipals(rs, func(p *IdentityPrincipalResource) { groups = append(groups, p.Identities) })
			return groups
		}, principalIdentityNamespaceParticipant),
		WithIndependentChildOwnership(capturePrincipalIdentityScopes,
			func(i *IdentityPrincipalIdentityResource) *KongctlMeta { return i.Kongctl }),
		WithManagedBy(ManagedByPolicy{MaxEntries: 5, PersistsRef: true}),
		WithChildSyncScope(ResourceTypeIdentityPrincipal),
		withReferenceValidation(101),
		WithMaturity(
			maturity.Metadata{Level: maturity.LevelTechPreview, Message: "Requires the Kong Identity SDK preview."},
		),
		withChildLoad(ResourceTypeIdentityPrincipalIdentity, ensureIdentityPrincipalIdentitySlice,
			childLoad[IdentityPrincipalIdentityResource, IdentityPrincipalResource]{
				family: ResourceTypeIdentityDirectory, extractOrder: 10, validateOrder: 20, validatePhase: 240,
				nested:    func(p *IdentityPrincipalResource) *[]IdentityPrincipalIdentityResource { return &p.Identities },
				setParent: func(i *IdentityPrincipalIdentityResource, ref string) { i.Principal = ref },
				validate:  validatePrincipalIdentities,
			}),
	)
}

func principalIdentityExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	custom, err := explainVariantNode[kkComps.KongPrincipalIdentityCustom]("type", "custom")
	if err != nil {
		return nil, err
	}
	oidc, err := explainVariantNode[kkComps.KongPrincipalIdentityOIDC]("type", "oidc")
	if err != nil {
		return nil, err
	}
	client, err := explainVariantNode[kkComps.KongPrincipalIdentityAuthServerClient]("type", "auth_server_client")
	if err != nil {
		return nil, err
	}
	consumer, err := explainVariantNode[kkComps.KongPrincipalIdentityCPConsumer]("type", "control_plane_consumer")
	if err != nil {
		return nil, err
	}
	external, err := autoExplainConcreteNode[IdentityPrincipalIdentityResource](nil)
	if err != nil {
		return nil, err
	}
	branches := []*ExplainNode{custom, oidc, client, consumer}
	for idx, branch := range branches {
		explainRemoveField(branch, "managed_by")
		branches[idx] = explainWithCommonFields(branch, explainResourceRefField(), explainKongctlField(),
			explainRefField("principal", ResourceTypeIdentityPrincipal, true))
	}
	if field, ok := external.property("type"); ok {
		field.Required = false
	}
	if field, ok := external.property("_external"); ok {
		field.Required = true
	}
	return explainUnionNode(append(branches, external)...), nil
}

func (i IdentityPrincipalIdentityResource) GetType() ResourceType {
	return ResourceTypeIdentityPrincipalIdentity
}
func (i IdentityPrincipalIdentityResource) GetMoniker() string                { return i.Ref }
func (i IdentityPrincipalIdentityResource) GetLabels() map[string]string      { return i.Labels }
func (i *IdentityPrincipalIdentityResource) SetLabels(v map[string]string)    { i.Labels = v }
func (i *IdentityPrincipalIdentityResource) SetDefaults()                     {}
func (i *IdentityPrincipalIdentityResource) GetExternalBlock() *ExternalBlock { return i.External }
func (i *IdentityPrincipalIdentityResource) IsExternal() bool                 { return i.External != nil }

func (i IdentityPrincipalIdentityResource) Protected() bool                 { return i.Kongctl.IsProtected() }
func (i IdentityPrincipalIdentityResource) GetKonnectMonikerFilter() string { return i.Ref }
func (i *IdentityPrincipalIdentityResource) TryMatchKonnectResource(value any) bool {
	v, ok := value.(interface {
		GetID() string
		GetManagedBy() map[string]string
	})
	if !ok {
		return false
	}
	m := v.GetManagedBy()
	if m[ManagedByToolKey] != ManagedByTool || m[ManagedByRefKey] != i.Ref ||
		m[ChildNamespaceKey] != GetNamespace(i.Kongctl) || v.GetID() == "" {
		return false
	}
	i.SetKonnectID(v.GetID())
	return true
}

func (i IdentityPrincipalIdentityResource) GetParentRef() *ResourceRef {
	if i.Principal == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeIdentityPrincipal, Ref: NormalizeResourceRef(i.Principal)}
}

func (i IdentityPrincipalIdentityResource) GetDependencies() []ResourceRef {
	if parent := i.GetParentRef(); parent != nil {
		return []ResourceRef{*parent}
	}
	return nil
}

func (i IdentityPrincipalIdentityResource) GetReferenceFieldMappings() map[string]string {
	// Association fields use typed payload references. Client IDs are opaque,
	// and control_plane_id may target either an API or an AI Gateway.
	return map[string]string{"principal": string(ResourceTypeIdentityPrincipal)}
}

// ReferencesTarget describes API associations without treating them as ownership.
func (i IdentityPrincipalIdentityResource) ReferencesTarget(kind ResourceType, id, parentID string) bool {
	value := func(v *string) string {
		if v == nil {
			return ""
		}
		return *v
	}
	// Only association target kinds participate in this predicate.
	//nolint:exhaustive // Other resource kinds cannot be principal identity targets.
	switch kind {
	case ResourceTypeIdentityAuthServer:
		return i.Type == "auth_server_client" && value(i.AuthServerID) == id
	case ResourceTypeIdentityAuthServerClient:
		return i.Type == "auth_server_client" && value(i.AuthServerID) == parentID && value(i.ClientID) == id
	case ResourceTypeControlPlane, ResourceTypeAIGateway:
		return i.Type == "control_plane_consumer" && value(i.ControlPlaneID) == id
	case ResourceTypeAIGatewayConsumer:
		return i.Type == "control_plane_consumer" && value(i.ControlPlaneID) == parentID && value(i.ConsumerID) == id
	default:
		return false
	}
}

// PublicFields is the API payload subset. kongctl and parent selectors never
// enter SDK requests. Nil and explicit empty labels remain distinct.
func (i IdentityPrincipalIdentityResource) PublicFields() map[string]any {
	v := map[string]any{"type": i.Type}
	for _, field := range []struct {
		key   string
		value *string
	}{
		{"issuer", i.Issuer},
		{"auth_server_id", i.AuthServerID},
		{"client_id", i.ClientID},
		{"control_plane_id", i.ControlPlaneID},
		{"consumer_id", i.ConsumerID},
		{"workspace_id", i.WorkspaceID},
		{"key", i.Key},
		{"value", i.Value},
	} {
		if field.value != nil {
			v[field.key] = *field.value
		}
	}
	if i.Claim != nil {
		v["claim"] = *i.Claim
	}
	if i.Labels != nil {
		v["labels"] = i.Labels
	}
	return v
}

func (i IdentityPrincipalIdentityResource) Validate() error {
	if err := ValidateRef(i.Ref); err != nil {
		return err
	}
	if i.IsExternal() {
		if err := i.External.Validate(); err != nil {
			return err
		}
		if !util.IsValidUUID(i.External.ID) || i.External.Selector != nil {
			return fmt.Errorf("external identity requires an explicit UUID in _external.id")
		}
		return nil
	}
	if i.WorkspaceID != nil {
		return fmt.Errorf("identity workspace_id configuration is deferred pending API question Q25")
	}
	var required []string
	switch i.Type {
	case "oidc":
		required = []string{"issuer", "claim"}
	case "auth_server_client":
		required = []string{"auth_server_id", "client_id"}
	case "control_plane_consumer":
		required = []string{"control_plane_id", "consumer_id"}
	case "custom":
		required = []string{"key", "value"}
	default:
		return fmt.Errorf("identity type must be oidc, auth_server_client, control_plane_consumer, or custom")
	}
	fields := i.PublicFields()
	for _, key := range required {
		if _, present := fields[key]; !present {
			return fmt.Errorf("%s identity requires explicit %s", i.Type, key)
		}
	}
	for key := range fields {
		if key == "type" || key == "labels" || slices.Contains(required, key) ||
			key == "workspace_id" && i.Type == "control_plane_consumer" {
			continue
		}
		return fmt.Errorf("%s does not belong to %s identities", key, i.Type)
	}
	return nil
}

func principalIdentityNamespaceParticipant(i *IdentityPrincipalIdentityResource) NamespaceParticipant {
	return NamespaceParticipant{
		Ref: i.Ref, Label: string(ResourceTypeIdentityPrincipalIdentity), Meta: &i.Kongctl,
		SupportsProtected: true, External: i.IsExternal(),
	}
}

func identityPrincipalIdentitySlice(rs *ResourceSet) *[]IdentityPrincipalIdentityResource {
	if rs == nil || rs.Identity == nil {
		return nil
	}
	return &rs.Identity.Identities
}

func ensureIdentityPrincipalIdentitySlice(rs *ResourceSet) *[]IdentityPrincipalIdentityResource {
	if rs.Identity == nil {
		rs.Identity = &IdentityResource{}
	}
	return &rs.Identity.Identities
}

func visitIdentityPrincipals(rs *ResourceSet, visit func(*IdentityPrincipalResource)) {
	if rs.Identity == nil {
		return
	}
	for d := range rs.Identity.Directories {
		for p := range rs.Identity.Directories[d].Principals {
			visit(&rs.Identity.Directories[d].Principals[p])
		}
	}
	for p := range rs.Identity.Principals {
		visit(&rs.Identity.Principals[p])
	}
}

func capturePrincipalIdentityScopes(rs *ResourceSet, defaultNamespace string) {
	if rs.Identity == nil {
		return
	}
	scope := rs.EnsureSyncScope()
	visitIdentityPrincipals(rs, func(p *IdentityPrincipalResource) {
		var namespaces []string
		for _, i := range p.Identities {
			if !i.IsExternal() {
				namespaces = append(namespaces, GetNamespace(i.Kongctl))
			}
		}
		if len(p.Identities) != 0 && len(namespaces) == 0 {
			return
		}
		CaptureOwnedChildCollection(scope, ResourceTypeIdentityPrincipalIdentity, p.Ref, defaultNamespace,
			scope.ChildInScope(ResourceTypeIdentityPrincipal, p.Ref, ResourceTypeIdentityPrincipalIdentity), namespaces)
	})
	for _, i := range rs.Identity.Identities {
		if !i.IsExternal() {
			scope.AddChildNamespace(ResourceTypeIdentityPrincipalIdentity, i.Principal, GetNamespace(i.Kongctl))
		}
	}
}

func validatePrincipalIdentities(rs *ResourceSet, values []IdentityPrincipalIdentityResource) error {
	for idx := range values {
		i := &values[idx]
		if err := validateCollectionResource(rs, i); err != nil {
			return err
		}
		p, ok := rs.GetResourceByRef(NormalizeResourceRef(i.Principal))
		if !ok || p.GetType() != ResourceTypeIdentityPrincipal {
			return fmt.Errorf("identity %q must reference a declared principal", i.Ref)
		}
		for _, field := range []struct {
			key   string
			value *string
			types []ResourceType
		}{
			{"auth_server_id", i.AuthServerID, []ResourceType{ResourceTypeIdentityAuthServer}},
			{"client_id", i.ClientID, []ResourceType{ResourceTypeIdentityAuthServerClient}},
			{"control_plane_id", i.ControlPlaneID, []ResourceType{ResourceTypeControlPlane, ResourceTypeAIGateway}},
			{"consumer_id", i.ConsumerID, []ResourceType{ResourceTypeAIGatewayConsumer}},
		} {
			if field.value == nil {
				continue
			}
			if ref, selector, found := tags.ParseRefPlaceholder(*field.value); found {
				target, ok := rs.GetResourceByRef(ref)
				if !ok || !slices.Contains(field.types, target.GetType()) || selector != "id" {
					return fmt.Errorf("identity %q %s requires a matching resource#id reference", i.Ref, field.key)
				}
			}
		}
	}
	return nil
}
