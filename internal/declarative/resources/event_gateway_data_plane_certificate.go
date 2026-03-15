package resources

import (
	"encoding/json"
	"fmt"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeEventGatewayDataPlaneCertificate,
		func(rs *ResourceSet) *[]EventGatewayDataPlaneCertificateResource {
			return &rs.EventGatewayDataPlaneCertificates
		},
		AutoExplain[EventGatewayDataPlaneCertificateResource](),
	)
}

type EventGatewayDataPlaneCertificateResource struct {
	kkComps.CreateEventGatewayDataPlaneCertificateRequest `       yaml:",inline"                 json:",inline"`
	Ref                                                   string `yaml:"ref"                     json:"ref"`
	// Parent Event Gateway reference (for root-level definitions)
	EventGateway string `yaml:"event_gateway,omitempty" json:"event_gateway,omitempty"`

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

func (e EventGatewayDataPlaneCertificateResource) GetType() ResourceType {
	return ResourceTypeEventGatewayDataPlaneCertificate
}

func (e EventGatewayDataPlaneCertificateResource) GetRef() string {
	return e.Ref
}

func (e EventGatewayDataPlaneCertificateResource) GetMoniker() string {
	if e.Name != nil {
		return *e.Name
	}
	return e.Ref
}

func (e EventGatewayDataPlaneCertificateResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if e.EventGateway != "" {
		// Dependency on parent Event Gateway when defined at root level
		deps = append(deps, ResourceRef{Kind: "event_gateway", Ref: e.EventGateway})
	}
	return deps
}

func (e EventGatewayDataPlaneCertificateResource) GetKonnectID() string {
	return e.konnectID
}

func (e EventGatewayDataPlaneCertificateResource) Validate() error {
	if err := ValidateRef(e.Ref); err != nil {
		return fmt.Errorf("invalid data plane certificate ref: %w", err)
	}

	// Certificate is required
	if e.Certificate == "" {
		return fmt.Errorf("certificate is required")
	}

	return nil
}

func (e *EventGatewayDataPlaneCertificateResource) SetDefaults() {
	// If Name is not set, use ref as default
	if e.Name == nil {
		name := e.Ref
		e.Name = &name
	}
}

func (e EventGatewayDataPlaneCertificateResource) GetKonnectMonikerFilter() string {
	if e.Name != nil {
		return fmt.Sprintf("name[eq]=%s", *e.Name)
	}
	return ""
	// Note: the API may not support filtering by name
}

func (e *EventGatewayDataPlaneCertificateResource) TryMatchKonnectResource(konnectResource any) bool {
	v := reflect.ValueOf(konnectResource)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return false
	}

	nameField := v.FieldByName("Name")
	idField := v.FieldByName("ID")

	if nameField.IsValid() && idField.IsValid() &&
		idField.Kind() == reflect.String {
		// Name field is a *string, so need to handle pointer
		if nameField.Kind() == reflect.Pointer && !nameField.IsNil() {
			nameVal := nameField.Elem()
			if nameVal.Kind() == reflect.String && e.Name != nil && nameVal.String() == *e.Name {
				e.konnectID = idField.String()
				return true
			}
		} else if nameField.Kind() == reflect.String {
			if e.Name != nil && nameField.String() == *e.Name {
				e.konnectID = idField.String()
				return true
			}
		}
	}

	return false
}

// REQUIRED: Implement ResourceWithParent
func (e EventGatewayDataPlaneCertificateResource) GetParentRef() *ResourceRef {
	if e.EventGateway != "" {
		return &ResourceRef{Kind: "event_gateway", Ref: e.EventGateway}
	}
	return nil
}

// MarshalJSON ensures certificate metadata (ref, event_gateway) are included.
// Without this, the embedded CreateEventGatewayDataPlaneCertificateRequest's MarshalJSON is promoted
// and drops metadata fields.
func (e EventGatewayDataPlaneCertificateResource) MarshalJSON() ([]byte, error) {
	type alias struct {
		Ref          string `json:"ref"`
		EventGateway string `json:"event_gateway,omitempty"`

		// Fields from kkComps.CreateEventGatewayDataPlaneCertificateRequest
		Certificate string  `json:"certificate"`
		Name        *string `json:"name,omitempty"`
		Description *string `json:"description,omitempty"`
	}

	payload := alias{
		Ref:          e.Ref,
		EventGateway: e.EventGateway,
		Certificate:  e.Certificate,
		Name:         e.Name,
		Description:  e.Description,
	}

	return json.Marshal(payload)
}

// Custom JSON unmarshaling to reject kongctl metadata
func (e *EventGatewayDataPlaneCertificateResource) UnmarshalJSON(data []byte) error {
	// Temporary structure for unmarshaling resource metadata together with
	// the CreateEventGatewayDataPlaneCertificateRequest fields from the SDK.
	var temp struct {
		Ref          string `json:"ref"`
		EventGateway string `json:"event_gateway,omitempty"`
		Kongctl      any    `json:"kongctl,omitempty"`

		// Fields from kkComps.CreateEventGatewayDataPlaneCertificateRequest
		Certificate string  `json:"certificate"`
		Name        *string `json:"name,omitempty"`
		Description *string `json:"description,omitempty"`
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

	// Populate embedded CreateEventGatewayDataPlaneCertificateRequest fields
	e.Certificate = temp.Certificate
	e.Name = temp.Name
	e.Description = temp.Description

	return nil
}
