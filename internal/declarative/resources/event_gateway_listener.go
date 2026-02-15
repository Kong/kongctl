package resources

import (
	"encoding/json"
	"fmt"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

type EventGatewayListenerResource struct {
	kkComps.CreateEventGatewayListenerRequest `       yaml:",inline"                 json:",inline"`
	Ref                                       string `yaml:"ref"                     json:"ref"`
	// Parent Event Gateway reference (for root-level definitions)
	EventGateway string `yaml:"event_gateway,omitempty" json:"event_gateway,omitempty"`

	// Nested child resources
	ListenerPolicies []EventGatewayListenerPolicyResource `yaml:"listener_policies,omitempty" json:"listener_policies,omitempty"` //nolint:lll

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

func (e EventGatewayListenerResource) GetType() ResourceType {
	return ResourceTypeEventGatewayListener
}

func (e EventGatewayListenerResource) GetRef() string {
	return e.Ref
}

func (e EventGatewayListenerResource) GetMoniker() string {
	return e.Name
}

func (e EventGatewayListenerResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if e.EventGateway != "" {
		// Dependency on parent Event Gateway when defined at root level
		deps = append(deps, ResourceRef{Kind: "event_gateway", Ref: e.EventGateway})
	}
	return deps
}

func (e EventGatewayListenerResource) GetKonnectID() string {
	return e.konnectID
}

func (e EventGatewayListenerResource) Validate() error {
	if err := ValidateRef(e.Ref); err != nil {
		return fmt.Errorf("invalid child ref: %w", err)
	}

	// Validate listener policies
	listenerPolicyRefs := make(map[string]bool)
	for i, lp := range e.ListenerPolicies {
		if err := lp.Validate(); err != nil {
			return fmt.Errorf("invalid listener policy %d: %w", i, err)
		}
		if listenerPolicyRefs[lp.GetRef()] {
			return fmt.Errorf("duplicate listener policy ref: %s", lp.GetRef())
		}
		listenerPolicyRefs[lp.GetRef()] = true
	}

	return nil
}

func (e *EventGatewayListenerResource) SetDefaults() {
	// If Name is not set, use ref as default
	if e.Name == "" {
		e.Name = e.Ref
	}

	for i := range e.ListenerPolicies {
		e.ListenerPolicies[i].SetDefaults()
	}
}

func (e EventGatewayListenerResource) GetKonnectMonikerFilter() string {
	return fmt.Sprintf("name[eq]=%s", e.Name) // TODO: the API does not support filtering by name.
}

func (e *EventGatewayListenerResource) TryMatchKonnectResource(konnectResource any) bool {
	v := reflect.ValueOf(konnectResource)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return false
	}

	nameField := v.FieldByName("Name")
	idField := v.FieldByName("ID")

	if nameField.IsValid() && idField.IsValid() &&
		nameField.Kind() == reflect.String && idField.Kind() == reflect.String {
		if nameField.String() == e.Name {
			e.konnectID = idField.String()
			return true
		}
	}

	return false
}

// REQUIRED: Implement ResourceWithParent
func (e EventGatewayListenerResource) GetParentRef() *ResourceRef {
	if e.EventGateway != "" {
		return &ResourceRef{Kind: "event_gateway", Ref: e.EventGateway}
	}
	return nil
}

// MarshalJSON ensures listener metadata (ref, event_gateway, listener_policies) are included.
// Without this, the embedded CreateEventGatewayListenerRequest's MarshalJSON is promoted and drops metadata fields.
func (e EventGatewayListenerResource) MarshalJSON() ([]byte, error) {
	type alias struct {
		Ref          string `json:"ref"`
		EventGateway string `json:"event_gateway,omitempty"`

		// Fields from kkComps.CreateEventGatewayListenerRequest
		Name        string                             `json:"name"`
		Description *string                            `json:"description,omitempty"`
		Addresses   []string                           `json:"addresses"`
		Ports       []kkComps.EventGatewayListenerPort `json:"ports"`
		Labels      map[string]string                  `json:"labels,omitempty"`

		// Child resources
		ListenerPolicies []EventGatewayListenerPolicyResource `json:"listener_policies,omitempty"`
	}

	payload := alias{
		Ref:              e.Ref,
		EventGateway:     e.EventGateway,
		Name:             e.Name,
		Description:      e.Description,
		Addresses:        e.Addresses,
		Ports:            e.Ports,
		Labels:           e.Labels,
		ListenerPolicies: e.ListenerPolicies,
	}

	return json.Marshal(payload)
}

// Custom JSON unmarshaling to reject kongctl metadata
func (e *EventGatewayListenerResource) UnmarshalJSON(data []byte) error {
	// Temporary structure for unmarshaling resource metadata together with
	// the CreateEventGatewayListenerRequest fields from the SDK.
	var temp struct {
		Ref          string `json:"ref"`
		EventGateway string `json:"event_gateway,omitempty"`
		Kongctl      any    `json:"kongctl,omitempty"`

		// Fields from kkComps.CreateEventGatewayListenerRequest
		Name             string                               `json:"name"`
		Description      *string                              `json:"description,omitempty"`
		Addresses        []string                             `json:"addresses"`
		Ports            []kkComps.EventGatewayListenerPort   `json:"ports"`
		Labels           map[string]string                    `json:"labels,omitempty"`
		ListenerPolicies []EventGatewayListenerPolicyResource `json:"listener_policies,omitempty"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	if temp.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on child resources")
	}

	// Populate resource metadata
	e.Ref = temp.Ref
	e.EventGateway = temp.EventGateway

	// Populate embedded CreateEventGatewayListenerRequest fields
	e.Name = temp.Name
	e.Description = temp.Description
	e.Addresses = temp.Addresses

	// Normalize ports: convert integer ports to string ports
	e.Ports = make([]kkComps.EventGatewayListenerPort, len(temp.Ports))
	for i, port := range temp.Ports {
		if port.Type == kkComps.EventGatewayListenerPortTypeInteger && port.Integer != nil {
			// Convert integer port to string
			strValue := fmt.Sprintf("%d", *port.Integer)
			e.Ports[i] = kkComps.EventGatewayListenerPort{
				Str:  &strValue,
				Type: kkComps.EventGatewayListenerPortTypeStr,
			}
		} else {
			// Keep as-is
			e.Ports[i] = port
		}
	}

	e.Labels = temp.Labels
	e.ListenerPolicies = temp.ListenerPolicies

	return nil
}
