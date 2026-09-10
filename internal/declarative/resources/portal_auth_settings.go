package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerChildResourceType(
		ResourceTypePortalAuthSettings,
		func(rs *ResourceSet) *[]PortalAuthSettingsResource { return &rs.PortalAuthSettings },
		AutoExplain[PortalAuthSettingsResource](
			WithExplainSchemaBuilder(portalAuthSettingsExplainNode),
		),
		childLoad[PortalAuthSettingsResource, PortalResource]{
			family:        ResourceTypePortal,
			extractOrder:  20,
			validateOrder: 40,
			validate:      validatePortalAuthSettingsChildren,
			extract: extractPortalSingleton(
				func(p *PortalResource) **PortalAuthSettingsResource { return &p.AuthSettings },
				func(r *PortalAuthSettingsResource, ref string) { r.Portal = ref },
			),
		},
		WithChildSyncScope(ResourceTypePortal),
	)
}

func portalAuthSettingsExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	node, err := autoExplainConcreteNode[PortalAuthSettingsResource](
		defaultExplainHints(ResourceTypePortalAuthSettings),
	)
	if err != nil {
		return nil, err
	}

	for _, field := range []string{
		"oidc_auth_enabled",
		"saml_auth_enabled",
		"oidc_team_mapping_enabled",
		"oidc_issuer",
		"oidc_client_id",
		"oidc_client_secret",
		"oidc_scopes",
		"oidc_claim_mappings",
	} {
		node.rejectLoadField(field, "portal_auth_settings "+PortalAuthSettingsDeprecatedFieldMessage(field))
		explainRemoveField(node, field)
	}

	return node, nil
}

// PortalAuthSettingsDeprecatedFieldMessage returns the shared migration
// guidance for an unsupported legacy portal authentication field.
func PortalAuthSettingsDeprecatedFieldMessage(field string) string {
	return fmt.Sprintf(
		"uses deprecated field %q; move identity provider configuration to identity_providers",
		field,
	)
}

// PortalAuthSettingsResource represents portal authentication settings (singleton child).
type PortalAuthSettingsResource struct {
	kkComps.PortalAuthenticationSettingsUpdateRequest `       yaml:",inline"          json:",inline"`
	Ref                                               string `yaml:"ref,omitempty"    json:"ref,omitempty"`
	Portal                                            string `yaml:"portal,omitempty" json:"portal,omitempty"`

	konnectID string `yaml:"-" json:"-"`
}

func (a PortalAuthSettingsResource) GetRef() string {
	return a.Ref
}

func (a PortalAuthSettingsResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid portal auth settings ref: %w", err)
	}
	return nil
}

func (a *PortalAuthSettingsResource) SetDefaults() {}

func (a PortalAuthSettingsResource) GetType() ResourceType {
	return ResourceTypePortalAuthSettings
}

func (a PortalAuthSettingsResource) GetMoniker() string {
	return a.Ref
}

func (a PortalAuthSettingsResource) GetDependencies() []ResourceRef {
	return []ResourceRef{}
}

func (a PortalAuthSettingsResource) GetKonnectID() string {
	return a.konnectID
}

func (a PortalAuthSettingsResource) GetKonnectMonikerFilter() string {
	// Singleton child matched via parent.
	return ""
}

func (a *PortalAuthSettingsResource) TryMatchKonnectResource(_ any) bool {
	// Matched via parent portal; no direct lookup.
	return false
}

// GetParentRef implements ResourceWithParent for inheritance of namespace and protection.
func (a PortalAuthSettingsResource) GetParentRef() *ResourceRef {
	if a.Portal == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypePortal, Ref: a.Portal}
}

// UnmarshalJSON rejects kongctl metadata on child resources.
func (a *PortalAuthSettingsResource) UnmarshalJSON(data []byte) error {
	var temp struct {
		Ref                                               string `json:"ref"`
		Portal                                            string `json:"portal,omitempty"`
		Kongctl                                           any    `json:"kongctl,omitempty"`
		kkComps.PortalAuthenticationSettingsUpdateRequest `json:",inline"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	if temp.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on portal auth settings")
	}

	a.Ref = temp.Ref
	a.Portal = temp.Portal
	a.PortalAuthenticationSettingsUpdateRequest = temp.PortalAuthenticationSettingsUpdateRequest

	return nil
}

func validatePortalAuthSettingsChildren(_ *ResourceSet, children []PortalAuthSettingsResource) error {
	for i := range children {
		settings := &children[i]
		if err := settings.Validate(); err != nil {
			return fmt.Errorf("invalid portal_auth_settings %q: %w", settings.GetRef(), err)
		}
		if settings.Portal == "" {
			return fmt.Errorf("portal_auth_settings %q must specify portal", settings.GetRef())
		}
		if deprecatedField := deprecatedPortalAuthSettingsField(settings); deprecatedField != "" {
			return fmt.Errorf(
				"portal_auth_settings %q %s",
				settings.GetRef(),
				PortalAuthSettingsDeprecatedFieldMessage(deprecatedField),
			)
		}
		for j := i + 1; j < len(children); j++ {
			if children[j].GetRef() == settings.GetRef() {
				return fmt.Errorf(
					"duplicate ref %s (already defined as portal_auth_settings)",
					settings.GetRef(),
				)
			}
		}
	}
	return nil
}

func deprecatedPortalAuthSettingsField(settings *PortalAuthSettingsResource) string {
	switch {
	case settings.OidcAuthEnabled != nil:
		return "oidc_auth_enabled"
	case settings.SamlAuthEnabled != nil:
		return "saml_auth_enabled"
	case settings.OidcTeamMappingEnabled != nil:
		return "oidc_team_mapping_enabled"
	case settings.OidcIssuer != nil:
		return "oidc_issuer"
	case settings.OidcClientID != nil:
		return "oidc_client_id"
	case settings.OidcClientSecret != nil:
		return "oidc_client_secret"
	case settings.OidcScopes != nil:
		return "oidc_scopes"
	case settings.OidcClaimMappings != nil:
		return "oidc_claim_mappings"
	default:
		return ""
	}
}
