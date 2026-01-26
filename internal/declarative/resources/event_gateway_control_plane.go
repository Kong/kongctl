package resources

import (
	"fmt"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

type EventGatewayControlPlaneResource struct {
	kkComps.CreateGatewayRequest
	Ref     string       `json:"ref"               yaml:"ref"`
	Kongctl *KongctlMeta `json:"kongctl,omitempty" yaml:"kongctl,omitempty"`

	// Nested child resources
	BackendClusters []EventGatewayBackendClusterResource `yaml:"backend_clusters,omitempty" json:"backend_clusters,omitempty"`

	// Resolved Konnect ID (not serialized)
	konnectID string `json:"-" yaml:"-"`
}

func (e EventGatewayControlPlaneResource) GetType() ResourceType {
	return ResourceTypeEventGatewayControlPlane
}

func (e EventGatewayControlPlaneResource) GetRef() string {
	return e.Ref
}

func (e EventGatewayControlPlaneResource) GetMoniker() string {
	return e.Name
}

func (e EventGatewayControlPlaneResource) GetKonnectID() string {
	return e.konnectID
}

func (e EventGatewayControlPlaneResource) GetDependencies() []ResourceRef {
	return []ResourceRef{}
}

func (e EventGatewayControlPlaneResource) GetLabels() map[string]string {
	return e.Labels
}

func (e *EventGatewayControlPlaneResource) SetLabels(labels map[string]string) {
	// Convert map to SDK format
	e.Labels = labels
}

func (e EventGatewayControlPlaneResource) Validate() error {
	if err := ValidateRef(e.Ref); err != nil {
		return fmt.Errorf("invalid Event Gateway Control Plane ref: %w", err)
	}
	return nil
}

func (e *EventGatewayControlPlaneResource) SetDefaults() {
	if e.Name == "" {
		e.Name = e.Ref
	}

	// if e.BackendClusters != nil {
	// 	for _, bc := range e.BackendClusters {
	// 		bc.SetDefaults()
	// 	}
	// }
}

func (e EventGatewayControlPlaneResource) GetKonnectMonikerFilter() string {
	if e.Name == "" {
		return ""
	}
	return fmt.Sprintf("name[eq]=%s", e.Name)
}

func (e *EventGatewayControlPlaneResource) TryMatchKonnectResource(konnectResource any) bool {
	v := reflect.ValueOf(konnectResource)

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return false
	}

	nameField := v.FieldByName("Name")
	idField := v.FieldByName("ID")

	if !nameField.IsValid() || !idField.IsValid() {
		eventGatewayField := v.FieldByName("EventGatewayInfo")
		if eventGatewayField.IsValid() && eventGatewayField.Kind() == reflect.Struct {
			nameField = eventGatewayField.FieldByName("Name")
			idField = eventGatewayField.FieldByName("ID")
		}
	}

	// Extract values if fields are valid
	if nameField.IsValid() && idField.IsValid() &&
		nameField.Kind() == reflect.String && idField.Kind() == reflect.String {
		if nameField.String() == e.Name {
			e.konnectID = idField.String()
			return true
		}
	}

	return false
}
