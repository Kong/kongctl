package resources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/util"
)

// APIImplementationResource represents an API implementation in declarative configuration
type APIImplementationResource struct {
	kkComps.APIImplementation `yaml:",inline" json:",inline"`
	Ref     string       `yaml:"ref" json:"ref"`
	API     string       `yaml:"api,omitempty" json:"api,omitempty"` // Parent API reference (for root-level definitions)
	
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
	
	if i.Service != nil && i.Service.ControlPlaneID != "" {
		// Check if control_plane_id is a UUID - if so, it's an external reference
		if !util.IsValidUUID(i.Service.ControlPlaneID) {
			// Not a UUID, so it's a reference to a declarative control plane
			mappings["service.control_plane_id"] = "control_plane"
		}
	}
	
	// Note: service.id is always external UUID, not ref-based
	return mappings
}

// Validate ensures the API implementation resource is valid
func (i APIImplementationResource) Validate() error {
	if err := ValidateRef(i.Ref); err != nil {
		return fmt.Errorf("invalid API implementation ref: %w", err)
	}
	
	// Validate service information if present
	if i.Service != nil {
		if i.Service.ID == "" {
			return fmt.Errorf("API implementation service.id is required")
		}
		
		// Validate service.id is a UUID format (external system)
		if !util.IsValidUUID(i.Service.ID) {
			return fmt.Errorf("API implementation service.id must be a valid UUID (external service managed by decK)")
		}
		
		if i.Service.ControlPlaneID == "" {
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
	// For API implementations, we match by service ID + control plane ID
	// Use reflection to access fields from state.APIImplementation
	v := reflect.ValueOf(konnectResource)
	
	// Handle pointer types
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	
	// Ensure we have a struct
	if v.Kind() != reflect.Struct {
		return false
	}
	
	// Look for Service and ID fields
	serviceField := v.FieldByName("Service")
	idField := v.FieldByName("ID")
	
	if serviceField.IsValid() && !serviceField.IsNil() && idField.IsValid() {
		// Service is a pointer to struct
		svc := serviceField.Elem()
		if svc.Kind() == reflect.Struct {
			svcIDField := svc.FieldByName("ID")
			cpIDField := svc.FieldByName("ControlPlaneID")
			
			if svcIDField.IsValid() && cpIDField.IsValid() && i.Service != nil {
				if svcIDField.String() == i.Service.ID && 
				   cpIDField.String() == i.Service.ControlPlaneID {
					i.konnectID = idField.String()
					return true
				}
			}
		}
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
		Ref                string `json:"ref"`
		API                string `json:"api,omitempty"`
		ImplementationURL  string `json:"implementation_url,omitempty"`
		Service            *struct {
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

