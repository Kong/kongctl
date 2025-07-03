package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// APIDocumentResource represents an API document in declarative configuration
type APIDocumentResource struct {
	kkComps.CreateAPIDocumentRequest `yaml:",inline" json:",inline"`
	Ref              string       `yaml:"ref" json:"ref"`
	// Parent API reference (for root-level definitions)
	API string `yaml:"api,omitempty" json:"api,omitempty"`
	ParentDocumentID string       `yaml:"parent_document_id,omitempty" json:"parent_document_id,omitempty"`
	Kongctl          *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
}

// GetKind returns the resource kind
func (d APIDocumentResource) GetKind() string {
	return "api_document"
}

// GetRef returns the reference identifier used for cross-resource references
func (d APIDocumentResource) GetRef() string {
	return d.Ref
}

// GetName returns the resource name
func (d APIDocumentResource) GetName() string {
	// Use title as name if available
	if d.Title != nil && *d.Title != "" {
		return *d.Title
	}
	// Fall back to ref
	return d.Ref
}

// GetDependencies returns references to other resources this API document depends on
func (d APIDocumentResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if d.API != "" {
		// Dependency on parent API when defined at root level
		deps = append(deps, ResourceRef{Kind: "api", Ref: d.API})
	}
	// Note: Parent document dependency is handled through reference field mappings
	return deps
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (d APIDocumentResource) GetReferenceFieldMappings() map[string]string {
	mappings := make(map[string]string)
	if d.ParentDocumentID != "" {
		mappings["parent_document_id"] = "api_document"
	}
	return mappings
}

// Validate ensures the API document resource is valid
func (d APIDocumentResource) Validate() error {
	if d.Ref == "" {
		return fmt.Errorf("API document ref is required")
	}
	if d.Content == "" {
		return fmt.Errorf("API document content is required")
	}
	// Parent API validation happens through dependency system
	return nil
}

// SetDefaults applies default values to API document resource
func (d *APIDocumentResource) SetDefaults() {
	// If status is not set, default to "published"
	if d.Status == nil {
		status := kkComps.APIDocumentStatusPublished
		d.Status = &status
	}
}

// GetParentRef returns the parent API reference for ResourceWithParent interface
func (d APIDocumentResource) GetParentRef() *ResourceRef {
	if d.API != "" {
		return &ResourceRef{Kind: "api", Ref: d.API}
	}
	return nil
}

// UnmarshalJSON implements custom JSON unmarshaling to handle SDK types
func (d *APIDocumentResource) UnmarshalJSON(data []byte) error {
	// Temporary struct to capture all fields
	var temp struct {
		Ref              string  `json:"ref"`
		API              string  `json:"api,omitempty"`
		Content          string  `json:"content"`
		Title            *string `json:"title,omitempty"`
		Slug             *string `json:"slug,omitempty"`
		Status           *string `json:"status,omitempty"`
		ParentDocumentID string  `json:"parent_document_id,omitempty"`
		Kongctl          *KongctlMeta `json:"kongctl,omitempty"`
	}
	
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	
	// Set our custom fields
	d.Ref = temp.Ref
	d.API = temp.API
	d.ParentDocumentID = temp.ParentDocumentID
	d.Kongctl = temp.Kongctl
	
	// Map to SDK fields embedded in CreateAPIDocumentRequest
	d.Content = temp.Content
	d.Title = temp.Title
	d.Slug = temp.Slug
	
	// Handle status enum if present
	if temp.Status != nil {
		status := kkComps.APIDocumentStatus(*temp.Status)
		d.Status = &status
	}
	
	// Handle ParentDocumentID for SDK field (pointer type)
	if temp.ParentDocumentID != "" {
		d.CreateAPIDocumentRequest.ParentDocumentID = &temp.ParentDocumentID
	}
	
	return nil
}