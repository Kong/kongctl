package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeEventGatewayTLSTrustBundle,
		func(rs *ResourceSet) *[]EventGatewayTLSTrustBundleResource {
			return &rs.EventGatewayTLSTrustBundles
		},
		AutoExplain[EventGatewayTLSTrustBundleResource](),
	)
}

// EventGatewayTLSTrustBundleResource represents an Event Gateway TLS Trust Bundle resource.
// Trust bundles define trusted certificate authorities used for mTLS client certificate
// verification. They are referenced by TLS listener policies.
// This resource supports update operations.
type EventGatewayTLSTrustBundleResource struct {
	kkComps.CreateTLSTrustBundleRequest `yaml:",inline" json:",inline"`
	Ref                                 string `yaml:"ref"                     json:"ref"`
	// Parent Event Gateway reference (for root-level definitions)
	EventGateway string `yaml:"event_gateway,omitempty" json:"event_gateway,omitempty"`

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

func (e EventGatewayTLSTrustBundleResource) GetType() ResourceType {
	return ResourceTypeEventGatewayTLSTrustBundle
}

func (e EventGatewayTLSTrustBundleResource) GetRef() string {
	return e.Ref
}

func (e EventGatewayTLSTrustBundleResource) GetMoniker() string {
	return e.Name
}

func (e EventGatewayTLSTrustBundleResource) GetDependencies() []ResourceRef {
	var deps []ResourceRef
	if e.EventGateway != "" {
		deps = append(deps, ResourceRef{Kind: ResourceTypeEventGatewayControlPlane, Ref: e.EventGateway})
	}
	return deps
}

func (e EventGatewayTLSTrustBundleResource) GetKonnectID() string {
	return e.konnectID
}

func (e EventGatewayTLSTrustBundleResource) Validate() error {
	if err := ValidateRef(e.Ref); err != nil {
		return fmt.Errorf("invalid TLS trust bundle ref: %w", err)
	}
	if e.Name == "" {
		return fmt.Errorf("TLS trust bundle %q is missing required field: name", e.Ref)
	}
	if e.Config.TrustedCa == "" {
		return fmt.Errorf("TLS trust bundle %q is missing required field: config.trusted_ca", e.Ref)
	}
	return nil
}

func (e *EventGatewayTLSTrustBundleResource) SetDefaults() {
	if e.Name == "" {
		e.Name = e.Ref
	}
}

func (e EventGatewayTLSTrustBundleResource) GetKonnectMonikerFilter() string {
	return fmt.Sprintf("name[eq]=%s", e.Name)
}

func (e *EventGatewayTLSTrustBundleResource) TryMatchKonnectResource(konnectResource any) bool {
	if id := tryMatchByField(konnectResource, "Name", e.Name); id != "" {
		e.konnectID = id
		return true
	}
	return false
}

// GetParentRef REQUIRED: marks resource as a child of Event Gateway.
func (e EventGatewayTLSTrustBundleResource) GetParentRef() *ResourceRef {
	if e.EventGateway != "" {
		return &ResourceRef{Kind: ResourceTypeEventGatewayControlPlane, Ref: e.EventGateway}
	}
	return nil
}

// MarshalJSON ensures trust bundle metadata (ref, event_gateway) are included.
// Without this, the embedded CreateTLSTrustBundleRequest's MarshalJSON is promoted and
// drops metadata fields.
func (e EventGatewayTLSTrustBundleResource) MarshalJSON() ([]byte, error) {
	type alias struct {
		Ref          string                       `json:"ref"`
		EventGateway string                       `json:"event_gateway,omitempty"`
		Name         string                       `json:"name"`
		Description  *string                      `json:"description,omitempty"`
		Config       kkComps.TLSTrustBundleConfig `json:"config"`
		Labels       map[string]string            `json:"labels,omitempty"`
	}

	payload := alias{
		Ref:          e.Ref,
		EventGateway: e.EventGateway,
		Name:         e.Name,
		Description:  e.Description,
		Config:       e.Config,
		Labels:       e.Labels,
	}

	return json.Marshal(payload)
}

// UnmarshalJSON rejects kongctl metadata (not supported on child resources).
func (e *EventGatewayTLSTrustBundleResource) UnmarshalJSON(data []byte) error {
	var temp struct {
		Ref          string                       `json:"ref"`
		EventGateway string                       `json:"event_gateway,omitempty"`
		Kongctl      any                          `json:"kongctl,omitempty"`
		Name         string                       `json:"name"`
		Description  *string                      `json:"description,omitempty"`
		Config       kkComps.TLSTrustBundleConfig `json:"config"`
		Labels       map[string]string            `json:"labels,omitempty"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	if temp.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on child resources")
	}

	e.Ref = temp.Ref
	e.EventGateway = temp.EventGateway
	e.Name = temp.Name
	e.Description = temp.Description
	e.Config = temp.Config
	e.Labels = temp.Labels

	return nil
}
