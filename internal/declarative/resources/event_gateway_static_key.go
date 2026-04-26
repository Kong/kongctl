package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeEventGatewayStaticKey,
		func(rs *ResourceSet) *[]EventGatewayStaticKeyResource { return &rs.EventGatewayStaticKeys },
		AutoExplain[EventGatewayStaticKeyResource](),
	)
}

// EventGatewayStaticKeyResource represents an Event Gateway Static Key resource.
// Static keys are used by Encrypt and Decrypt policies to encrypt data at rest.
// This resource does not support update operations – changes are implemented as
// delete + create.
type EventGatewayStaticKeyResource struct {
	kkComps.EventGatewayStaticKeyCreate `yaml:",inline" json:",inline"`
	Ref                                 string `yaml:"ref"                     json:"ref"`
	// Parent Event Gateway reference (for root-level definitions)
	EventGateway string `yaml:"event_gateway,omitempty" json:"event_gateway,omitempty"`

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

func (e EventGatewayStaticKeyResource) GetType() ResourceType {
	return ResourceTypeEventGatewayStaticKey
}

func (e EventGatewayStaticKeyResource) GetRef() string {
	return e.Ref
}

func (e EventGatewayStaticKeyResource) GetMoniker() string {
	return e.Name
}

func (e EventGatewayStaticKeyResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if e.EventGateway != "" {
		// Dependency on parent Event Gateway when defined at root level
		deps = append(deps, ResourceRef{Kind: ResourceTypeEventGatewayControlPlane, Ref: e.EventGateway})
	}
	return deps
}

func (e EventGatewayStaticKeyResource) GetKonnectID() string {
	return e.konnectID
}

func (e EventGatewayStaticKeyResource) Validate() error {
	if err := ValidateRef(e.Ref); err != nil {
		return fmt.Errorf("invalid static key ref: %w", err)
	}
	if e.Name == "" {
		return fmt.Errorf("static key %q is missing required field: name", e.Ref)
	}
	if e.Value == "" {
		return fmt.Errorf("static key %q is missing required field: value", e.Ref)
	}
	return nil
}

func (e *EventGatewayStaticKeyResource) SetDefaults() {
	if e.Name == "" {
		e.Name = e.Ref
	}
}

func (e EventGatewayStaticKeyResource) GetKonnectMonikerFilter() string {
	return fmt.Sprintf("name[eq]=%s", e.Name) // TODO: the API does not support filtering by name.
}

func (e *EventGatewayStaticKeyResource) TryMatchKonnectResource(konnectResource any) bool {
	if id := tryMatchByField(konnectResource, "Name", e.Name); id != "" {
		e.konnectID = id
		return true
	}
	return false
}

// GetParentRef REQUIRED: marks resource as a child of Event Gateway.
func (e EventGatewayStaticKeyResource) GetParentRef() *ResourceRef {
	if e.EventGateway != "" {
		return &ResourceRef{Kind: ResourceTypeEventGatewayControlPlane, Ref: e.EventGateway}
	}
	return nil
}

// MarshalJSON ensures static key metadata (ref, event_gateway) are included.
// Without this, the embedded EventGatewayStaticKeyCreate's MarshalJSON is promoted and
// drops metadata fields.
func (e EventGatewayStaticKeyResource) MarshalJSON() ([]byte, error) {
	type alias struct {
		Ref          string            `json:"ref"`
		EventGateway string            `json:"event_gateway,omitempty"`
		Name         string            `json:"name"`
		Description  *string           `json:"description,omitempty"`
		Labels       map[string]string `json:"labels,omitempty"`
		Value        string            `json:"value"`
	}

	payload := alias{
		Ref:          e.Ref,
		EventGateway: e.EventGateway,
		Name:         e.Name,
		Description:  e.Description,
		Labels:       e.Labels,
		Value:        e.Value,
	}

	return json.Marshal(payload)
}

// UnmarshalJSON rejects kongctl metadata (not supported on child resources).
func (e *EventGatewayStaticKeyResource) UnmarshalJSON(data []byte) error {
	var temp struct {
		Ref          string            `json:"ref"`
		EventGateway string            `json:"event_gateway,omitempty"`
		Kongctl      any               `json:"kongctl,omitempty"`
		Name         string            `json:"name"`
		Description  *string           `json:"description,omitempty"`
		Labels       map[string]string `json:"labels,omitempty"`
		Value        string            `json:"value"`
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
	e.Labels = temp.Labels
	e.Value = temp.Value

	return nil
}
