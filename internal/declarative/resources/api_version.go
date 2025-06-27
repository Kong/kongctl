package resources

import (
	"encoding/json"
	"fmt"

	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
)

// APIVersionResource represents an API version in declarative configuration
type APIVersionResource struct {
	kkInternalComps.CreateAPIVersionRequest `yaml:",inline" json:",inline"`
	Ref     string       `yaml:"ref" json:"ref"`
	Kongctl *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
	// Note: api_id removed - implicit from parent API structure
}

// GetRef returns the reference identifier used for cross-resource references
func (v APIVersionResource) GetRef() string {
	return v.Ref
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (v APIVersionResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{} // No outbound references - parent API is implicit
}

// Validate ensures the API version resource is valid
func (v APIVersionResource) Validate() error {
	if v.Ref == "" {
		return fmt.Errorf("API version ref is required")
	}
	// Note: api_id validation removed - implicit from parent API structure
	return nil
}

// SetDefaults applies default values to API version resource
func (v *APIVersionResource) SetDefaults() {
	// API versions typically don't need default values
}

// UnmarshalJSON implements custom JSON unmarshaling to handle SDK types
func (v *APIVersionResource) UnmarshalJSON(data []byte) error {
	// Temporary struct to capture all fields
	var temp struct {
		Ref           string `json:"ref"`
		Name          string `json:"name"`
		Version       string `json:"version"`
		PublishStatus string `json:"publish_status,omitempty"`
		Deprecated    bool   `json:"deprecated,omitempty"`
		SunsetDate    string `json:"sunset_date,omitempty"`
		Kongctl       *KongctlMeta `json:"kongctl,omitempty"`
	}
	
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	
	// Set our custom fields
	v.Ref = temp.Ref
	v.Kongctl = temp.Kongctl
	
	// Map to SDK fields embedded in CreateAPIVersionRequest
	// The SDK type likely has these fields, we need to set them correctly
	// For now, let's just marshal back into the embedded type
	sdkData := map[string]interface{}{
		"name":    temp.Name,
		"version": temp.Version,
	}
	
	if temp.PublishStatus != "" {
		sdkData["publish_status"] = temp.PublishStatus
	}
	if temp.Deprecated {
		sdkData["deprecated"] = temp.Deprecated
	}
	if temp.SunsetDate != "" {
		sdkData["sunset_date"] = temp.SunsetDate
	}
	
	sdkBytes, err := json.Marshal(sdkData)
	if err != nil {
		return err
	}
	
	// Unmarshal into the embedded SDK type
	if err := json.Unmarshal(sdkBytes, &v.CreateAPIVersionRequest); err != nil {
		return err
	}
	
	return nil
}