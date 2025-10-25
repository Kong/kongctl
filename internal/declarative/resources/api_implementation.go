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

// GetDependencies returns references to other resources this API implementation depends on
func (i APIImplementationResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if i.API != "" {
		// Dependency on parent API when defined at root level
		deps = append(deps, ResourceRef{Kind: "api", Ref: i.API})
	}
	// Note: Control plane dependency is handled through reference field mappings
	return deps
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (i APIImplementationResource) GetReferenceFieldMappings() map[string]string {
	// Only include control_plane_id mapping if it's not a UUID
	mappings := make(map[string]string)

	service := i.getService()
	if service != nil {
		cpID := service.GetControlPlaneID()
		if cpID != "" && !util.IsValidUUID(cpID) {
			// Not a UUID, so it's a reference to a declarative control plane
			mappings["service.control_plane_id"] = "control_plane"
		}
		serviceID := service.GetID()
		if serviceID != "" && !util.IsValidUUID(serviceID) && !tags.IsRefPlaceholder(serviceID) {
			mappings["service.id"] = string(ResourceTypeGatewayService)
		}
	}

	return mappings
}

// Validate ensures the API implementation resource is valid
func (i APIImplementationResource) Validate() error {
	if err := ValidateRef(i.Ref); err != nil {
		return fmt.Errorf("invalid API implementation ref: %w", err)
	}

	// Validate service information if present
	if service := i.getService(); service != nil {
		if service.GetID() == "" {
			return fmt.Errorf("API implementation service.id is required")
		}

		if service.GetControlPlaneID() == "" {
			return fmt.Errorf("API implementation service.control_plane_id is required")
		}

		// control_plane_id can be either a UUID (external) or a reference (declarative)
		// Both are valid - no additional validation needed here
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
	serviceID, controlPlaneID := i.serviceIdentifiers()
	if serviceID == "" || controlPlaneID == "" {
		return false
	}

	if id, ok := matchImplementationKnownTypes(konnectResource, serviceID, controlPlaneID); ok {
		i.konnectID = id
		return true
	}

	if id, ok := matchImplementationByReflection(konnectResource, serviceID, controlPlaneID); ok {
		i.konnectID = id
		return true
	}

	return false
}

// GetParentRef returns the parent API reference for ResourceWithParent interface
func (i APIImplementationResource) GetParentRef() *ResourceRef {
	if i.API != "" {
		return &ResourceRef{Kind: "api", Ref: i.API}
	}
	return nil
}

// UnmarshalJSON implements custom JSON unmarshaling to handle SDK types
func (i *APIImplementationResource) UnmarshalJSON(data []byte) error {
	// Temporary struct to capture all fields
	var temp struct {
		Ref               string `json:"ref"`
		API               string `json:"api,omitempty"`
		ImplementationURL string `json:"implementation_url,omitempty"`
		Service           *struct {
			ID             string `json:"id"`
			ControlPlaneID string `json:"control_plane_id"`
		} `json:"service,omitempty"`
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

	// Check if kongctl field was provided and reject it
	if temp.Kongctl != nil {
		return fmt.Errorf("kongctl metadata is not supported on child resources (API implementations)")
	}

	// Map to SDK fields embedded in APIImplementation
	sdkData := map[string]any{}

	if temp.ImplementationURL != "" {
		sdkData["implementation_url"] = temp.ImplementationURL
	}

	if temp.Service != nil {
		sdkData["service"] = map[string]any{
			"id":               temp.Service.ID,
			"control_plane_id": temp.Service.ControlPlaneID,
		}
	}

	sdkBytes, err := json.Marshal(sdkData)
	if err != nil {
		return err
	}

	// Unmarshal into the embedded SDK type
	if err := json.Unmarshal(sdkBytes, &i.APIImplementation); err != nil {
		return err
	}

	return nil
}

func (i APIImplementationResource) getService() *kkComps.APIImplementationService {
	if i.ServiceReference == nil {
		return nil
	}
	return i.ServiceReference.GetService()
}

// GetService returns the gateway service implementing this API, if defined.
func (i APIImplementationResource) GetService() *kkComps.APIImplementationService {
	return i.getService()
}

func (i APIImplementationResource) serviceIdentifiers() (string, string) {
	service := i.getService()
	if service == nil {
		return "", ""
	}
	return service.GetID(), service.GetControlPlaneID()
}

func matchImplementationKnownTypes(konnectResource any, serviceID, controlPlaneID string) (string, bool) {
	switch res := konnectResource.(type) {
	case *kkComps.APIImplementationResponse:
		return matchImplementationResponse(res, serviceID, controlPlaneID)
	case kkComps.APIImplementationResponse:
		return matchImplementationResponse(&res, serviceID, controlPlaneID)
	case *kkComps.APIImplementationListItem:
		return matchImplementationListItem(res, serviceID, controlPlaneID)
	case kkComps.APIImplementationListItem:
		return matchImplementationListItem(&res, serviceID, controlPlaneID)
	}
	return "", false
}

func matchImplementationResponse(
	res *kkComps.APIImplementationResponse,
	serviceID, controlPlaneID string,
) (string, bool) {
	if res == nil || res.APIImplementationResponseServiceReference == nil {
		return "", false
	}

	ref := res.APIImplementationResponseServiceReference
	service := ref.GetService()
	if service == nil {
		return "", false
	}

	if service.GetID() == serviceID && service.GetControlPlaneID() == controlPlaneID {
		return ref.GetID(), true
	}

	return "", false
}

func matchImplementationListItem(
	item *kkComps.APIImplementationListItem,
	serviceID, controlPlaneID string,
) (string, bool) {
	if item == nil || item.APIImplementationListItemGatewayServiceEntity == nil {
		return "", false
	}

	entity := item.APIImplementationListItemGatewayServiceEntity
	service := entity.GetService()
	if service == nil {
		return "", false
	}

	if service.GetID() == serviceID && service.GetControlPlaneID() == controlPlaneID {
		return entity.GetID(), true
	}

	return "", false
}

func matchImplementationByReflection(konnectResource any, serviceID, controlPlaneID string) (string, bool) {
	v := reflect.ValueOf(konnectResource)
	if !v.IsValid() {
		return "", false
	}

	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return "", false
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return "", false
	}

	serviceField := v.FieldByName("Service")
	idField := v.FieldByName("ID")

	serviceValue, ok := derefStructValue(serviceField)
	if !ok || !idField.IsValid() || idField.Kind() != reflect.String {
		return "", false
	}

	serviceIDField := serviceValue.FieldByName("ID")
	cpIDField := serviceValue.FieldByName("ControlPlaneID")
	if !serviceIDField.IsValid() || !cpIDField.IsValid() ||
		serviceIDField.Kind() != reflect.String || cpIDField.Kind() != reflect.String {
		return "", false
	}

	if serviceIDField.String() == serviceID && cpIDField.String() == controlPlaneID {
		return idField.String(), true
	}

	return "", false
}

func derefStructValue(v reflect.Value) (reflect.Value, bool) {
	if !v.IsValid() {
		return reflect.Value{}, false
	}

	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}, false
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}

	return v, true
}
