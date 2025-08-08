package resources

import (
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/declarative/external"
)

// ExternalResourceResource represents a reference to an existing resource in Konnect
// that is not managed by this configuration but needs to be referenced by managed resources.
type ExternalResourceResource struct {
	// Declarative reference identifier
	Ref string `yaml:"ref" json:"ref"`

	// Resource type identifier (e.g., "portal", "api", "control_plane")
	ResourceType string `yaml:"resource_type" json:"resource_type"`

	// Direct ID specification (mutually exclusive with Selector)
	ID *string `yaml:"id,omitempty" json:"id,omitempty"`

	// Selector-based specification (mutually exclusive with ID)
	Selector *ExternalResourceSelector `yaml:"selector,omitempty" json:"selector,omitempty"`

	// Parent resource for hierarchical resources
	Parent *ExternalResourceParent `yaml:"parent,omitempty" json:"parent,omitempty"`

	// Runtime state (not serialized to YAML/JSON)
	resolvedID       string      `yaml:"-" json:"-"`
	resolvedResource interface{} `yaml:"-" json:"-"`
	resolved         bool        `yaml:"-" json:"-"`
}

// ExternalResourceSelector defines criteria for finding a resource by field matching
type ExternalResourceSelector struct {
	// Map of field names to expected values for matching
	MatchFields map[string]string `yaml:"match_fields" json:"match_fields"`
}

// ExternalResourceParent defines a parent resource for hierarchical resolution
type ExternalResourceParent struct {
	// Parent resource type
	ResourceType string `yaml:"resource_type" json:"resource_type"`

	// Parent resource ID (must be resolved before child)
	ID string `yaml:"id,omitempty" json:"id,omitempty"`

	// Alternative: reference to another external resource
	Ref string `yaml:"ref,omitempty" json:"ref,omitempty"`
}

// Interface implementations

// GetRef returns the declarative reference identifier
func (e ExternalResourceResource) GetRef() string {
	return e.Ref
}

// GetResourceType returns the resource type
func (e ExternalResourceResource) GetResourceType() string {
	return e.ResourceType
}

// Validate implements ResourceValidator interface
func (e ExternalResourceResource) Validate() error {
	// Validate ref field using common validation
	if err := ValidateRef(e.Ref); err != nil {
		return fmt.Errorf("invalid external resource ref: %w", err)
	}

	// Validate resource type
	if err := ValidateResourceType(e.ResourceType); err != nil {
		return fmt.Errorf("invalid resource_type in external resource %q: %w", e.Ref, err)
	}

	// Validate ID XOR Selector requirement
	if err := ValidateIDXORSelector(e.ID, e.Selector); err != nil {
		return fmt.Errorf("invalid external resource %q: %w", e.Ref, err)
	}

	// Validate selector if present
	if e.Selector != nil {
		if err := ValidateSelector(e.ResourceType, e.Selector); err != nil {
			return fmt.Errorf("invalid selector in external resource %q: %w", e.Ref, err)
		}
	}

	// Check if parent is required for this resource type
	registry := external.GetResolutionRegistry()
	metadata, exists := registry.GetResolutionMetadata(e.ResourceType)
	if exists && len(metadata.SupportedParents) > 0 {
		// Parent is required for this resource type
		if e.Parent == nil {
			return fmt.Errorf("external resource %q of type %q requires parent of type(s): %s",
				e.Ref, e.ResourceType, strings.Join(metadata.SupportedParents, ", "))
		}
	}

	// Validate parent if present
	if e.Parent != nil {
		if err := ValidateParent(e.ResourceType, e.Parent); err != nil {
			return fmt.Errorf("invalid parent in external resource %q: %w", e.Ref, err)
		}
	}

	return nil
}

// Runtime state methods

// SetResolvedID sets the resolved Konnect ID
func (e *ExternalResourceResource) SetResolvedID(id string) {
	e.resolvedID = id
	e.resolved = true
}

// GetResolvedID returns the resolved Konnect ID
func (e *ExternalResourceResource) GetResolvedID() string {
	return e.resolvedID
}

// SetResolvedResource sets the resolved resource object
func (e *ExternalResourceResource) SetResolvedResource(resource interface{}) {
	e.resolvedResource = resource
}

// GetResolvedResource returns the resolved resource object
func (e *ExternalResourceResource) GetResolvedResource() interface{} {
	return e.resolvedResource
}

// IsResolved returns whether this external resource has been resolved
func (e *ExternalResourceResource) IsResolved() bool {
	return e.resolved
}

// GetID returns the ID field as a pointer (implements ExternalResource interface)
func (e ExternalResourceResource) GetID() *string {
	return e.ID
}

// GetSelector returns the selector (implements Resource interface)
func (e ExternalResourceResource) GetSelector() external.Selector {
	if e.Selector == nil {
		return nil
	}
	return e.Selector
}

// GetParent returns the parent (implements Resource interface)
func (e ExternalResourceResource) GetParent() external.Parent {
	if e.Parent == nil {
		return nil
	}
	return e.Parent
}

// GetMatchFields returns the match fields from the selector (implements Selector interface)
func (s *ExternalResourceSelector) GetMatchFields() map[string]string {
	if s == nil {
		return nil
	}
	return s.MatchFields
}

// GetResourceType returns the resource type (implements Parent interface)
func (p *ExternalResourceParent) GetResourceType() string {
	if p == nil {
		return ""
	}
	return p.ResourceType
}

// GetID returns the ID (implements Parent interface)
func (p *ExternalResourceParent) GetID() string {
	if p == nil {
		return ""
	}
	return p.ID
}

// GetRef returns the reference (implements Parent interface)
func (p *ExternalResourceParent) GetRef() string {
	if p == nil {
		return ""
	}
	return p.Ref
}

// ExternalResourceError represents validation errors for external resources
type ExternalResourceError struct {
	Ref          string
	ResourceType string
	Field        string
	Message      string
	Cause        error
}

func (e *ExternalResourceError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("external resource %q (%s): %s in field %s",
			e.Ref, e.ResourceType, e.Message, e.Field)
	}
	return fmt.Sprintf("external resource %q (%s): %s",
		e.Ref, e.ResourceType, e.Message)
}

func (e *ExternalResourceError) Unwrap() error {
	return e.Cause
}

// NewExternalResourceError creates a new external resource error
func NewExternalResourceError(ref, resourceType, field, message string, cause error) *ExternalResourceError {
	return &ExternalResourceError{
		Ref:          ref,
		ResourceType: resourceType,
		Field:        field,
		Message:      message,
		Cause:        cause,
	}
}

// ValidateResourceType validates that the resource type is supported
func ValidateResourceType(resourceType string) error {
	if resourceType == "" {
		return fmt.Errorf("resource_type is required")
	}

	// Get supported resource types from resolution registry
	registry := external.GetResolutionRegistry()
	if !registry.IsSupported(resourceType) {
		supported := registry.GetSupportedTypes()
		return fmt.Errorf("unsupported resource_type %q, supported types: %s",
			resourceType, strings.Join(supported, ", "))
	}

	return nil
}

// ValidateIDXORSelector validates that exactly one of ID or Selector is specified
func ValidateIDXORSelector(id *string, selector *ExternalResourceSelector) error {
	hasID := id != nil && *id != ""
	hasSelector := selector != nil && len(selector.MatchFields) > 0

	if !hasID && !hasSelector {
		return fmt.Errorf("either 'id' or 'selector' must be specified")
	}

	if hasID && hasSelector {
		return fmt.Errorf("'id' and 'selector' are mutually exclusive, specify only one")
	}

	return nil
}

// ValidateSelector validates selector configuration for the given resource type
func ValidateSelector(resourceType string, selector *ExternalResourceSelector) error {
	if selector == nil {
		return fmt.Errorf("selector cannot be nil")
	}

	if len(selector.MatchFields) == 0 {
		return fmt.Errorf("selector.match_fields cannot be empty")
	}

	// Get supported fields from resolution registry
	registry := external.GetResolutionRegistry()
	supportedFields := registry.GetSupportedSelectorFields(resourceType)

	if supportedFields == nil {
		return fmt.Errorf("no supported selector fields defined for resource_type %q", resourceType)
	}

	for field := range selector.MatchFields {
		if !contains(supportedFields, field) {
			return fmt.Errorf("field %q is not supported for selector on resource_type %q, supported fields: %s",
				field, resourceType, strings.Join(supportedFields, ", "))
		}
	}

	return nil
}

// ValidateParent validates parent resource configuration
func ValidateParent(childResourceType string, parent *ExternalResourceParent) error {
	if parent == nil {
		return fmt.Errorf("parent cannot be nil")
	}

	// Validate parent resource type
	if err := ValidateResourceType(parent.ResourceType); err != nil {
		return fmt.Errorf("invalid parent resource_type: %w", err)
	}

	// Validate that exactly one of ID or Ref is specified
	hasID := parent.ID != ""
	hasRef := parent.Ref != ""

	if !hasID && !hasRef {
		return fmt.Errorf("parent must specify either 'id' or 'ref'")
	}

	if hasID && hasRef {
		return fmt.Errorf("parent 'id' and 'ref' are mutually exclusive")
	}

	// Validate parent-child relationship using resolution registry
	registry := external.GetResolutionRegistry()
	if !registry.IsValidParentChild(parent.ResourceType, childResourceType) {
		return fmt.Errorf("resource_type %q cannot have parent of type %q",
			childResourceType, parent.ResourceType)
	}

	return nil
}

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}