package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// APIVersionResource represents an API version in declarative configuration
type APIVersionResource struct {
	kkComps.CreateAPIVersionRequest `yaml:",inline" json:",inline"`
	Ref     string       `yaml:"ref" json:"ref"`
	API     string       `yaml:"api,omitempty" json:"api,omitempty"` // Parent API reference (for root-level definitions)
	Kongctl *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
}

// GetKind returns the resource kind
func (v APIVersionResource) GetKind() string {
	return "api_version"
}

// GetRef returns the reference identifier used for cross-resource references
func (v APIVersionResource) GetRef() string {
	return v.Ref
}

// GetName returns the resource name
func (v APIVersionResource) GetName() string {
	// API versions use version field as name
	if v.Version != nil {
		return *v.Version
	}
	return ""
}

// GetDependencies returns references to other resources this API version depends on
func (v APIVersionResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if v.API != "" {
		// Dependency on parent API when defined at root level
		deps = append(deps, ResourceRef{Kind: "api", Ref: v.API})
	}
	return deps
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (v APIVersionResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{} // No outbound references besides parent
}

// Validate ensures the API version resource is valid
func (v APIVersionResource) Validate() error {
	if v.Ref == "" {
		return fmt.Errorf("API version ref is required")
	}
	// Parent API validation happens through dependency system
	return nil
}

// SetDefaults applies default values to API version resource
func (v *APIVersionResource) SetDefaults() {
	// API versions typically don't need default values
}

// GetParentRef returns the parent API reference for ResourceWithParent interface
func (v APIVersionResource) GetParentRef() *ResourceRef {
	if v.API != "" {
		return &ResourceRef{Kind: "api", Ref: v.API}
	}
	return nil
}

// UnmarshalJSON implements custom JSON unmarshaling to handle SDK types
func (v *APIVersionResource) UnmarshalJSON(data []byte) error {
	// Temporary struct to capture all fields
	var temp struct {
		Ref           string      `json:"ref"`
		API           string      `json:"api,omitempty"`
		Version       string      `json:"version"`
		PublishStatus string      `json:"publish_status,omitempty"`
		Deprecated    bool        `json:"deprecated,omitempty"`
		SunsetDate    string      `json:"sunset_date,omitempty"`
		Kongctl       *KongctlMeta `json:"kongctl,omitempty"`
		Spec          interface{} `json:"spec,omitempty"`
	}
	
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	
	// Set our custom fields
	v.Ref = temp.Ref
	v.API = temp.API
	v.Kongctl = temp.Kongctl
	
	// Map to SDK fields embedded in CreateAPIVersionRequest
	sdkData := map[string]interface{}{
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
	
	// Handle spec field - it could be a string, a map, or a wrapped object
	if temp.Spec != nil {
		var specContent string
		
		// Check if it's already in the SDK format with content field
		if specMap, ok := temp.Spec.(map[string]interface{}); ok {
			if content, hasContent := specMap["content"].(string); hasContent {
				// Already in correct format
				specContent = content
			} else {
				// It's a raw OpenAPI spec object, convert to JSON string
				specJSON, err := json.Marshal(temp.Spec)
				if err != nil {
					return fmt.Errorf("failed to marshal spec to JSON: %w", err)
				}
				specContent = string(specJSON)
			}
		} else if specStr, ok := temp.Spec.(string); ok {
			// It's already a string
			specContent = specStr
		} else {
			// Unknown format, try to marshal it
			specJSON, err := json.Marshal(temp.Spec)
			if err != nil {
				return fmt.Errorf("failed to marshal spec to JSON: %w", err)
			}
			specContent = string(specJSON)
		}
		
		sdkData["spec"] = map[string]interface{}{
			"content": specContent,
		}
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