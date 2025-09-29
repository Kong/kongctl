package resources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/util"
)

// APIDocumentResource represents an API document in declarative configuration
type APIDocumentResource struct {
	kkComps.CreateAPIDocumentRequest `                      yaml:",inline"                       json:",inline"`
	Ref                              string `yaml:"ref"                           json:"ref"`
	// Parent API reference (for root-level definitions)
	API               string                `yaml:"api,omitempty"                 json:"api,omitempty"`
	ParentDocumentID  string                `yaml:"parent_document_id,omitempty"  json:"parent_document_id,omitempty"`
	ParentDocumentRef string                `yaml:"parent_document_ref,omitempty" json:"parent_document_ref,omitempty"`
	Children          []APIDocumentResource `yaml:"children,omitempty"            json:"children,omitempty"`

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

// GetType returns the resource type
func (d APIDocumentResource) GetType() ResourceType {
	return ResourceTypeAPIDocument
}

// GetRef returns the reference identifier used for cross-resource references
func (d APIDocumentResource) GetRef() string {
	return d.Ref
}

// GetMoniker returns the resource moniker (for documents, this is the slug)
func (d APIDocumentResource) GetMoniker() string {
	// API documents use slug as moniker
	if d.Slug != nil {
		return *d.Slug
	}
	return ""
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
	if d.API != "" {
		mappings["api"] = "api"
	}
	if d.ParentDocumentRef != "" {
		mappings["parent_document_ref"] = "api_document"
	}
	return mappings
}

// Validate ensures the API document resource is valid
func (d APIDocumentResource) Validate() error {
	if err := ValidateRef(d.Ref); err != nil {
		return fmt.Errorf("invalid API document ref: %w", err)
	}
	if d.Content == "" {
		return fmt.Errorf("API document content is required")
	}

	// Validate slug format using Konnect's regex pattern
	if d.Slug != nil && *d.Slug != "" {
		slugRegex := regexp.MustCompile(`^[\w-]+$`)
		if !slugRegex.MatchString(*d.Slug) {
			return fmt.Errorf(
				"invalid slug %q: slugs must contain only letters, numbers, underscores, and hyphens",
				*d.Slug,
			)
		}
	}

	// Validate children recursively
	for i, child := range d.Children {
		if child.API != "" {
			return fmt.Errorf("child document[%d] ref=%q should not define api (inherited from parent)", i, child.Ref)
		}
		if err := child.Validate(); err != nil {
			return fmt.Errorf("child document[%d] ref=%q validation failed: %w", i, child.Ref, err)
		}
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

	// If slug is not set but title is provided, generate slug from title
	// This matches the Konnect API behavior: "defaults to slugify(title)"
	if d.Slug == nil && d.Title != nil && *d.Title != "" {
		slug := util.GenerateSlug(*d.Title)
		d.Slug = &slug
	}

	for i := range d.Children {
		d.Children[i].SetDefaults()
	}
}

// GetKonnectID returns the resolved Konnect ID if available
func (d APIDocumentResource) GetKonnectID() string {
	return d.konnectID
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (d APIDocumentResource) GetKonnectMonikerFilter() string {
	// API documents don't support filtering directly
	// They must be looked up through parent API
	return ""
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource
func (d *APIDocumentResource) TryMatchKonnectResource(konnectResource any) bool {
	// For API documents, we match by slug
	// Use reflection to access fields from state.APIDocument
	v := reflect.ValueOf(konnectResource)

	// Handle pointer types
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Ensure we have a struct
	if v.Kind() != reflect.Struct {
		return false
	}

	// Look for Slug and ID fields
	slugField := v.FieldByName("Slug")
	idField := v.FieldByName("ID")

	// Extract values if fields are valid
	if slugField.IsValid() && idField.IsValid() &&
		slugField.Kind() == reflect.String && idField.Kind() == reflect.String {
		if d.Slug != nil && slugField.String() == *d.Slug {
			d.konnectID = idField.String()
			return true
		}
	}

	return false
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
		Ref               string                `json:"ref"`
		API               string                `json:"api,omitempty"`
		Content           string                `json:"content"`
		Title             *string               `json:"title,omitempty"`
		Slug              *string               `json:"slug,omitempty"`
		Status            *string               `json:"status,omitempty"`
		ParentDocumentID  string                `json:"parent_document_id,omitempty"`
		ParentDocumentRef string                `json:"parent_document_ref,omitempty"`
		Children          []APIDocumentResource `json:"children,omitempty"`
		Kongctl           any                   `json:"kongctl,omitempty"`
	}

	// Use a decoder with DisallowUnknownFields to catch typos
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&temp); err != nil {
		return err
	}

	// Set our custom fields
	d.Ref = temp.Ref
	d.API = temp.API
	d.ParentDocumentID = temp.ParentDocumentID
	d.ParentDocumentRef = temp.ParentDocumentRef
	d.Children = temp.Children

	// Check if kongctl field was provided and reject it
	if temp.Kongctl != nil {
		return fmt.Errorf("kongctl metadata is not supported on child resources (API documents)")
	}

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
