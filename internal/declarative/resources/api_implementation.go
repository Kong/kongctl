package resources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/util"
)

const (
	apiImplementationTypeService      = "service"
	apiImplementationTypeControlPlane = "control_plane"
)

func init() {
	registerResourceType(
		ResourceTypeAPIImplementation,
		func(rs *ResourceSet) *[]APIImplementationResource { return &rs.APIImplementations },
		AutoExplain[APIImplementationResource](
			WithExplainSchemaBuilder(apiImplementationExplainNode),
		),
	)
}

// APIImplementationResource represents an API implementation in declarative configuration
type APIImplementationResource struct {
	kkComps.APIImplementation `       yaml:",inline"       json:",inline"`
	Ref                       string `yaml:"ref"           json:"ref"`
	// Parent API reference (for root-level definitions)
	API string `yaml:"api,omitempty" json:"api,omitempty"`

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

// GetType returns the resource type
func (i APIImplementationResource) GetType() ResourceType {
	return ResourceTypeAPIImplementation
}

// GetRef returns the reference identifier used for cross-resource references
func (i APIImplementationResource) GetRef() string {
	return i.Ref
}

// GetMoniker returns the resource moniker (for implementations, this is empty)
func (i APIImplementationResource) GetMoniker() string {
	// API implementations don't have a unique identifier
	return ""
}

func (i APIImplementationResource) getService() *kkComps.APIImplementationService {
	if i.ServiceReference == nil {
		return nil
	}
	return i.ServiceReference.GetService()
}

func (i APIImplementationResource) getControlPlane() *kkComps.APIImplementationControlPlaneInput {
	if i.ControlPlaneReference == nil {
		return nil
	}
	return i.ControlPlaneReference.GetControlPlane()
}

// GetDependencies returns references to other resources this API implementation depends on
func (i APIImplementationResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if i.API != "" {
		// Dependency on parent API when defined at root level
		deps = append(deps, ResourceRef{Kind: ResourceTypeAPI, Ref: i.API})
	}
	// Note: Control plane dependency is handled through reference field mappings
	return deps
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (i APIImplementationResource) GetReferenceFieldMappings() map[string]string {
	// Only include control_plane_id mapping if it's not a UUID
	mappings := make(map[string]string)

	if service := i.getService(); service != nil && service.ControlPlaneID != "" {
		// Check if control_plane_id is a UUID - if so, it's an external reference
		if !util.IsValidUUID(service.ControlPlaneID) {
			// Not a UUID, so it's a reference to a declarative control plane
			mappings["service.control_plane_id"] = string(ResourceTypeControlPlane)
		}
	}

	if service := i.getService(); service != nil && service.ID != "" {
		if !util.IsValidUUID(service.ID) && !tags.IsRefPlaceholder(service.ID) {
			mappings["service.id"] = string(ResourceTypeGatewayService)
		}
	}

	if controlPlane := i.getControlPlane(); controlPlane != nil && controlPlane.ID != "" {
		if !util.IsValidUUID(controlPlane.ID) {
			mappings["control_plane.control_plane_id"] = string(ResourceTypeControlPlane)
		}
	}

	return mappings
}

// Validate ensures the API implementation resource is valid
func (i APIImplementationResource) Validate() error {
	if err := ValidateRef(i.Ref); err != nil {
		return fmt.Errorf("invalid API implementation ref: %w", err)
	}

	service := i.getService()
	controlPlane := i.getControlPlane()
	if (service == nil) == (controlPlane == nil) {
		return fmt.Errorf("API implementation must define exactly one of service or control_plane")
	}

	if service != nil {
		if service.ID == "" {
			return fmt.Errorf("API implementation service.id is required")
		}

		if service.ControlPlaneID == "" {
			return fmt.Errorf("API implementation service.control_plane_id is required")
		}

		if i.Type != "" && i.Type != kkComps.APIImplementationTypeServiceReference {
			return fmt.Errorf("API implementation type does not match service payload")
		}
	}

	if controlPlane != nil {
		if controlPlane.ID == "" {
			return fmt.Errorf("API implementation control_plane.control_plane_id is required")
		}
		if i.Type != "" && i.Type != kkComps.APIImplementationTypeControlPlaneReference {
			return fmt.Errorf("API implementation type does not match control_plane payload")
		}
	}

	return nil
}

// SetDefaults applies default values to API implementation resource
func (i *APIImplementationResource) SetDefaults() {
	// API implementations typically don't need default values
}

// GetKonnectID returns the resolved Konnect ID if available
func (i APIImplementationResource) GetKonnectID() string {
	return i.konnectID
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (i APIImplementationResource) GetKonnectMonikerFilter() string {
	// API implementations don't support filtering
	return ""
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource
func (i *APIImplementationResource) TryMatchKonnectResource(konnectResource any) bool {
	v, ok := implementationStructValue(reflect.ValueOf(konnectResource))
	if !ok {
		return false
	}
	id := implementationStringField(v, "ID")

	if service := i.getService(); service != nil {
		for _, path := range [][]string{{"Service"}, {"ServiceReference", "Service"}} {
			remote, found := implementationNestedStruct(v, path...)
			if !found {
				continue
			}
			serviceID := implementationStringField(remote, "ID")
			controlPlaneID := implementationStringField(remote, "ControlPlaneID")
			if serviceID == service.ID && controlPlaneID == service.ControlPlaneID {
				i.konnectID = id
				return true
			}
		}
	}

	if controlPlane := i.getControlPlane(); controlPlane != nil {
		for _, path := range [][]string{{"ControlPlane"}, {"ControlPlaneReference", "ControlPlane"}} {
			remote, found := implementationNestedStruct(v, path...)
			if !found {
				continue
			}
			controlPlaneID := implementationStringField(remote, "ID")
			if controlPlaneID == controlPlane.ID {
				i.konnectID = id
				return true
			}
		}
	}

	return false
}

func implementationNestedStruct(value reflect.Value, path ...string) (reflect.Value, bool) {
	current, ok := implementationStructValue(value)
	if !ok {
		return reflect.Value{}, false
	}
	for _, fieldName := range path {
		current = current.FieldByName(fieldName)
		current, ok = implementationStructValue(current)
		if !ok {
			return reflect.Value{}, false
		}
	}
	return current, true
}

func implementationStructValue(value reflect.Value) (reflect.Value, bool) {
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}, false
		}
		value = value.Elem()
	}
	return value, value.IsValid() && value.Kind() == reflect.Struct
}

func implementationStringField(value reflect.Value, name string) string {
	field := value.FieldByName(name)
	if !field.IsValid() || field.Kind() != reflect.String {
		return ""
	}
	return field.String()
}

// GetParentRef returns the parent API reference for ResourceWithParent interface
func (i APIImplementationResource) GetParentRef() *ResourceRef {
	if i.API != "" {
		return &ResourceRef{Kind: ResourceTypeAPI, Ref: i.API}
	}
	return nil
}

// MarshalJSON ensures implementation metadata (ref, api) are included.
// Without this, the embedded APIImplementation's MarshalJSON is promoted and drops metadata fields.
func (i APIImplementationResource) MarshalJSON() ([]byte, error) {
	if i.ServiceReference != nil && i.ControlPlaneReference != nil {
		return nil, fmt.Errorf("API implementation must define exactly one of service or control_plane")
	}

	payload := make(map[string]any)

	implBytes, err := json.Marshal(i.APIImplementation)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(implBytes, &payload); err != nil {
		return nil, err
	}

	payload["ref"] = i.Ref
	if i.API != "" {
		payload["api"] = i.API
	}
	if i.ServiceReference != nil {
		payload["type"] = apiImplementationTypeService
	}
	if i.ControlPlaneReference != nil {
		payload["type"] = apiImplementationTypeControlPlane
	}

	return json.Marshal(payload)
}

// UnmarshalJSON implements custom JSON unmarshaling to handle SDK types
func (i *APIImplementationResource) UnmarshalJSON(data []byte) error {
	// Temporary struct to capture all fields
	var temp struct {
		Ref               string `json:"ref"`
		API               string `json:"api,omitempty"`
		Type              string `json:"type,omitempty"`
		ImplementationURL string `json:"implementation_url,omitempty"`
		Service           *struct {
			ID             string `json:"id"`
			ControlPlaneID string `json:"control_plane_id"`
		} `json:"service,omitempty"`
		ControlPlane *struct {
			ControlPlaneID string `json:"control_plane_id"`
		} `json:"control_plane,omitempty"`
		Kongctl any `json:"kongctl,omitempty"`
	}

	// Use a decoder with DisallowUnknownFields to catch typos
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&temp); err != nil {
		return err
	}

	// Set our custom fields
	i.Ref = temp.Ref
	i.API = temp.API

	if temp.Type != "" && temp.Type != apiImplementationTypeService &&
		temp.Type != apiImplementationTypeControlPlane {
		return fmt.Errorf("API implementation type must be service or control_plane (got %q)", temp.Type)
	}

	// Check if kongctl field was provided and reject it
	if temp.Kongctl != nil {
		return fmt.Errorf("kongctl metadata is not supported on child resources (API implementations)")
	}

	if temp.Service != nil && temp.ControlPlane != nil {
		return fmt.Errorf("API implementation must define exactly one of service or control_plane")
	}
	if temp.Service != nil {
		if temp.Type == apiImplementationTypeControlPlane {
			return fmt.Errorf("API implementation type control_plane does not match service payload")
		}
		i.APIImplementation = kkComps.CreateAPIImplementationServiceReference(kkComps.ServiceReference{
			Service: &kkComps.APIImplementationService{
				ID:             temp.Service.ID,
				ControlPlaneID: temp.Service.ControlPlaneID,
			},
		})
		return nil
	}
	if temp.ControlPlane != nil {
		if temp.Type == apiImplementationTypeService {
			return fmt.Errorf("API implementation type service does not match control_plane payload")
		}
		i.APIImplementation = kkComps.CreateAPIImplementationControlPlaneReference(kkComps.ControlPlaneReference{
			ControlPlane: &kkComps.APIImplementationControlPlaneInput{ID: temp.ControlPlane.ControlPlaneID},
		})
		return nil
	}

	i.APIImplementation = kkComps.APIImplementation{}
	return nil
}
