package resources

import (
	"encoding/json"
	"fmt"
	"regexp"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

var (
	googleTagManagerIDPattern = regexp.MustCompile(`^GTM-[A-Za-z0-9]+$`)
	googleAnalytics4IDPattern = regexp.MustCompile(`^G-[A-Za-z0-9-]+$`)
)

func init() {
	registerChildResourceType(
		ResourceTypePortalIntegration,
		func(rs *ResourceSet) *[]PortalIntegrationResource { return &rs.PortalIntegrations },
		AutoExplain[PortalIntegrationResource](),
		childLoad[PortalIntegrationResource, PortalResource]{
			family:        ResourceTypePortal,
			extractOrder:  40,
			validateOrder: 60,
			validate:      validatePortalIntegrationChildren,
			extract: extractPortalSingleton(
				func(p *PortalResource) **PortalIntegrationResource { return &p.Integrations },
				func(r *PortalIntegrationResource, ref string) { r.Portal = ref },
			),
		},
		WithChildSyncScope(ResourceTypePortal),
	)
}

// PortalIntegrationResource represents portal integration configuration (singleton child).
type PortalIntegrationResource struct {
	kkComps.PortalIntegrations `       yaml:",inline"          json:",inline"`
	Ref                        string `yaml:"ref"              json:"ref"`
	Portal                     string `yaml:"portal,omitempty" json:"portal,omitempty"`

	konnectID string `yaml:"-" json:"-"`
}

func (i PortalIntegrationResource) GetRef() string {
	return i.Ref
}

func (i PortalIntegrationResource) Validate() error {
	if err := ValidateRef(i.Ref); err != nil {
		return fmt.Errorf("invalid portal integration ref: %w", err)
	}

	if i.GoogleTagManager != nil {
		if i.GoogleTagManager.Type != "" &&
			i.GoogleTagManager.Type != kkComps.GoogleTagManagerIntegrationTypeTracking {
			return fmt.Errorf("google_tag_manager type must be 'tracking'")
		}
		if i.GoogleTagManager.ConfigData.ID == "" {
			return fmt.Errorf("google_tag_manager config_data.id is required")
		}
		if !googleTagManagerIDPattern.MatchString(i.GoogleTagManager.ConfigData.ID) {
			return fmt.Errorf("google_tag_manager config_data.id must be a valid GTM container ID")
		}
	}

	if i.GoogleAnalytics4 != nil {
		if i.GoogleAnalytics4.Type != "" &&
			i.GoogleAnalytics4.Type != kkComps.GoogleAnalytics4IntegrationTypeAnalytics {
			return fmt.Errorf("google_analytics_4 type must be 'analytics'")
		}
		if i.GoogleAnalytics4.ConfigData.ID == "" {
			return fmt.Errorf("google_analytics_4 config_data.id is required")
		}
		if !googleAnalytics4IDPattern.MatchString(i.GoogleAnalytics4.ConfigData.ID) {
			return fmt.Errorf("google_analytics_4 config_data.id must be a valid Google Analytics ID")
		}
	}

	return nil
}

func (i *PortalIntegrationResource) SetDefaults() {
	if i.GoogleTagManager != nil && i.GoogleTagManager.Type == "" {
		i.GoogleTagManager.Type = kkComps.GoogleTagManagerIntegrationTypeTracking
	}
	if i.GoogleAnalytics4 != nil && i.GoogleAnalytics4.Type == "" {
		i.GoogleAnalytics4.Type = kkComps.GoogleAnalytics4IntegrationTypeAnalytics
	}
}

func (i PortalIntegrationResource) GetType() ResourceType {
	return ResourceTypePortalIntegration
}

func (i PortalIntegrationResource) GetMoniker() string {
	return i.Ref
}

func (i PortalIntegrationResource) GetDependencies() []ResourceRef {
	if i.Portal == "" {
		return []ResourceRef{}
	}
	return []ResourceRef{{Kind: ResourceTypePortal, Ref: i.Portal}}
}

// GetReferenceFieldMappings returns cross-resource reference mappings for validation.
func (i PortalIntegrationResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{
		SchemaFieldPortal: string(ResourceTypePortal),
	}
}

func (i PortalIntegrationResource) GetKonnectID() string {
	return i.konnectID
}

func (i PortalIntegrationResource) GetKonnectMonikerFilter() string {
	// Singleton child matched via parent.
	return ""
}

func (i *PortalIntegrationResource) TryMatchKonnectResource(_ any) bool {
	// Matched via parent portal; no direct lookup.
	return false
}

// GetParentRef implements ResourceWithParent for inheritance of namespace and protection.
func (i PortalIntegrationResource) GetParentRef() *ResourceRef {
	if i.Portal == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypePortal, Ref: i.Portal}
}

// UnmarshalJSON rejects kongctl metadata on child resources.
func (i *PortalIntegrationResource) UnmarshalJSON(data []byte) error {
	var temp struct {
		Ref                        string `json:"ref"`
		Portal                     string `json:"portal,omitempty"`
		Kongctl                    any    `json:"kongctl,omitempty"`
		kkComps.PortalIntegrations `json:",inline"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	if temp.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on portal integration")
	}

	i.Ref = temp.Ref
	i.Portal = temp.Portal
	i.PortalIntegrations = temp.PortalIntegrations

	return nil
}

func validatePortalIntegrationChildren(_ *ResourceSet, children []PortalIntegrationResource) error {
	portalIntegrationRefs := make(map[string]bool)
	portalToIntegrationRef := make(map[string]string)
	for i := range children {
		integration := &children[i]
		if err := integration.Validate(); err != nil {
			return fmt.Errorf("invalid portal_integration %q: %w", integration.GetRef(), err)
		}
		if integration.Portal == "" {
			return fmt.Errorf("portal_integration %q must specify portal", integration.GetRef())
		}
		if portalIntegrationRefs[integration.GetRef()] {
			return fmt.Errorf("duplicate ref '%s' (already defined as portal_integration)", integration.GetRef())
		}
		portalIntegrationRefs[integration.GetRef()] = true

		if existingRef, ok := portalToIntegrationRef[integration.Portal]; ok {
			return fmt.Errorf(
				"multiple portal_integration entries target portal %q (%s and %s)",
				integration.Portal, existingRef, integration.GetRef(),
			)
		}
		portalToIntegrationRef[integration.Portal] = integration.GetRef()
	}
	return nil
}
