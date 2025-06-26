package resources

import (
	"encoding/json"
	"fmt"
	"regexp"

	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
)

// APIImplementationResource represents an API implementation in declarative configuration
type APIImplementationResource struct {
	kkInternalComps.APIImplementation `yaml:",inline" json:",inline"`
	Ref     string       `yaml:"ref" json:"ref"`
	Kongctl *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
}

// GetRef returns the reference identifier used for cross-resource references
func (i APIImplementationResource) GetRef() string {
	return i.Ref
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

// UnmarshalJSON implements custom JSON unmarshaling to handle SDK types
func (i *APIImplementationResource) UnmarshalJSON(data []byte) error {
	// Temporary struct to capture all fields
	var temp struct {
		Ref                string `json:"ref"`
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