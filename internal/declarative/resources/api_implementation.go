package resources

import (
	"encoding/json"
	"fmt"
	"regexp"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// APIImplementationResource represents an API implementation in declarative configuration
type APIImplementationResource struct {
	kkComps.APIImplementation `yaml:",inline" json:",inline"`
	Ref     string       `yaml:"ref" json:"ref"`
	API     string       `yaml:"api,omitempty" json:"api,omitempty"` // Parent API reference (for root-level definitions)
	Kongctl *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
}

// GetKind returns the resource kind
func (i APIImplementationResource) GetKind() string {
	return "api_implementation"
}

// GetRef returns the reference identifier used for cross-resource references
func (i APIImplementationResource) GetRef() string {
	return i.Ref
}

// GetName returns the resource name
func (i APIImplementationResource) GetName() string {
	// API implementations don't have a name field, use ref as identifier
	return i.Ref
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
		if !isValidUUID(i.Service.ControlPlaneID) {
			// Not a UUID, so it's a reference to a declarative control plane
			mappings["service.control_plane_id"] = "control_plane"
		}
	}
	
	// Note: service.id is always external UUID, not ref-based
	return mappings
}

// Validate ensures the API implementation resource is valid
func (i APIImplementationResource) Validate() error {
	if i.Ref == "" {
		return fmt.Errorf("API implementation ref is required")
	}
	
	// Validate service information if present
	if i.Service != nil {
		if i.Service.ID == "" {
			return fmt.Errorf("API implementation service.id is required")
		}
		
		// Validate service.id is a UUID format (external system)
		if !isValidUUID(i.Service.ID) {
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
		Kongctl *KongctlMeta `json:"kongctl,omitempty"`
	}
	
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	
	// Set our custom fields
	i.Ref = temp.Ref
	i.API = temp.API
	i.Kongctl = temp.Kongctl
	
	// Map to SDK fields embedded in APIImplementation
	sdkData := map[string]interface{}{}
	
	if temp.ImplementationURL != "" {
		sdkData["implementation_url"] = temp.ImplementationURL
	}
	
	if temp.Service != nil {
		sdkData["service"] = map[string]interface{}{
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

// isValidUUID checks if a string is a valid UUID format
func isValidUUID(s string) bool {
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	return uuidRegex.MatchString(s)
}