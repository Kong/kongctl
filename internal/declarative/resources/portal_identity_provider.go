package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypePortalIdentityProvider,
		func(rs *ResourceSet) *[]PortalIdentityProviderResource { return &rs.PortalIdentityProviders },
		AutoExplain[PortalIdentityProviderResource](),
	)
}

// PortalIdentityProviderResource represents a portal identity provider child resource.
type PortalIdentityProviderResource struct {
	kkComps.CreateIdentityProvider `       yaml:",inline"          json:",inline"`
	Ref                            string `yaml:"ref"              json:"ref"`
	Portal                         string `yaml:"portal,omitempty" json:"portal,omitempty"`

	konnectID string `yaml:"-" json:"-"`
}

// GetType returns the resource type.
func (p PortalIdentityProviderResource) GetType() ResourceType {
	return ResourceTypePortalIdentityProvider
}

// GetRef returns the reference identifier.
func (p PortalIdentityProviderResource) GetRef() string {
	return p.Ref
}

// GetMoniker returns the resource moniker.
func (p PortalIdentityProviderResource) GetMoniker() string {
	if p.Type == nil {
		return p.Ref
	}
	return string(*p.Type)
}

// GetDependencies returns references to other resources this provider depends on.
func (p PortalIdentityProviderResource) GetDependencies() []ResourceRef {
	var deps []ResourceRef
	if p.Portal != "" {
		deps = append(deps, ResourceRef{Kind: "portal", Ref: p.Portal})
	}
	return deps
}

// Validate ensures the portal identity provider resource is valid.
func (p PortalIdentityProviderResource) Validate() error {
	if err := ValidateRef(p.Ref); err != nil {
		return fmt.Errorf("invalid portal identity provider ref: %w", err)
	}

	if p.Type == nil || !p.Type.IsExact() {
		return fmt.Errorf("identity provider type is required")
	}

	if p.Config == nil {
		return fmt.Errorf("identity provider config is required")
	}

	switch p.Config.Type {
	case kkComps.CreateIdentityProviderConfigTypeOIDCIdentityProviderConfig:
		if *p.Type != kkComps.IdentityProviderTypeOidc {
			return fmt.Errorf("identity provider type %q does not match oidc config", *p.Type)
		}
		if p.Config.OIDCIdentityProviderConfig == nil {
			return fmt.Errorf("oidc identity provider config is required")
		}
	case kkComps.CreateIdentityProviderConfigTypeSAMLIdentityProviderConfigInput:
		if *p.Type != kkComps.IdentityProviderTypeSaml {
			return fmt.Errorf("identity provider type %q does not match saml config", *p.Type)
		}
		if p.Config.SAMLIdentityProviderConfigInput == nil {
			return fmt.Errorf("saml identity provider config is required")
		}
	default:
		return fmt.Errorf("identity provider config type is required")
	}

	return nil
}

// SetDefaults applies default values to the resource.
func (p *PortalIdentityProviderResource) SetDefaults() {}

// GetKonnectID returns the resolved Konnect ID if available.
func (p PortalIdentityProviderResource) GetKonnectID() string {
	return p.konnectID
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup.
func (p PortalIdentityProviderResource) GetKonnectMonikerFilter() string {
	return ""
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource.
func (p *PortalIdentityProviderResource) TryMatchKonnectResource(_ any) bool {
	return false
}

// GetParentRef implements ResourceWithParent for inheritance of namespace and protection.
func (p PortalIdentityProviderResource) GetParentRef() *ResourceRef {
	if p.Portal == "" {
		return nil
	}
	return &ResourceRef{Kind: string(ResourceTypePortal), Ref: p.Portal}
}

// UnmarshalJSON rejects kongctl metadata on child resources.
func (p *PortalIdentityProviderResource) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	allowedKeys := map[string]struct{}{
		"ref":        {},
		"portal":     {},
		"kongctl":    {},
		"type":       {},
		"enabled":    {},
		"login_path": {},
		"config":     {},
	}
	for key := range raw {
		if _, ok := allowedKeys[key]; !ok {
			return fmt.Errorf("json: unknown field %q", key)
		}
	}

	p.CreateIdentityProvider = kkComps.CreateIdentityProvider{}

	if v, ok := raw["ref"]; ok {
		if err := json.Unmarshal(v, &p.Ref); err != nil {
			return err
		}
	}

	if v, ok := raw["portal"]; ok {
		if err := json.Unmarshal(v, &p.Portal); err != nil {
			return err
		}
	}

	if v, ok := raw["kongctl"]; ok {
		var kongctl any
		if err := json.Unmarshal(v, &kongctl); err != nil {
			return err
		}
		if kongctl != nil {
			return fmt.Errorf("kongctl metadata not supported on portal identity providers")
		}
	}

	if v, ok := raw["type"]; ok {
		var providerType kkComps.IdentityProviderType
		if err := json.Unmarshal(v, &providerType); err != nil {
			return err
		}
		p.Type = &providerType
	}

	if v, ok := raw["enabled"]; ok {
		var enabled bool
		if err := json.Unmarshal(v, &enabled); err != nil {
			return err
		}
		p.Enabled = &enabled
	}

	if v, ok := raw["login_path"]; ok {
		var loginPath string
		if err := json.Unmarshal(v, &loginPath); err != nil {
			return err
		}
		p.LoginPath = &loginPath
	}

	if v, ok := raw["config"]; ok {
		var config kkComps.CreateIdentityProviderConfig
		if err := json.Unmarshal(v, &config); err != nil {
			return err
		}
		p.Config = &config
	}

	return nil
}
