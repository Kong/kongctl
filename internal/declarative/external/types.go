package external

import (
	"context"
	"time"
)

// ResolutionMetadata contains metadata needed to resolve external resources from Konnect
type ResolutionMetadata struct {
	// Human-readable name
	Name string

	// Supported fields for selector matching
	SelectorFields []string

	// Supported parent resource types
	SupportedParents []string

	// Supported child resource types
	SupportedChildren []string

	// Adapter for resolving resources via SDK
	ResolutionAdapter ResolutionAdapter
}

// ResolutionAdapter defines the interface for resolving external resources via SDK
type ResolutionAdapter interface {
	// GetByID retrieves a resource by its Konnect ID
	GetByID(ctx context.Context, id string, parent *ResolvedParent) (interface{}, error)

	// GetBySelector retrieves resources matching selector criteria
	GetBySelector(ctx context.Context, selector map[string]string, parent *ResolvedParent) ([]interface{}, error)
}

// ResolvedParent contains information about a resolved parent resource
type ResolvedParent struct {
	ResourceType string
	ID           string
	Resource     interface{}
}

// ResolvedResource holds the resolved data for an external resource
type ResolvedResource struct {
	ID           string            // Resolved Konnect ID
	Resource     interface{}       // Full SDK response object
	ResourceType string            // Resource type (portal, api, etc.)
	Ref          string            // Original reference from config
	Parent       *ResolvedResource // Parent resource if applicable
	ResolvedAt   time.Time         // Resolution timestamp
}

// DependencyNode represents a node in the dependency graph
type DependencyNode struct {
	Ref          string   // External resource reference
	ResourceType string   // Resource type
	ParentRef    string   // Parent reference (empty for top-level)
	ChildRefs    []string // Child references
	Resolved     bool     // Resolution status
}

// DependencyGraph manages resolution ordering
type DependencyGraph struct {
	Nodes           map[string]*DependencyNode // All nodes by ref
	ResolutionOrder []string                   // Topologically sorted order
}

// ConfigurationContext provides context about where in the configuration an error occurred
type ConfigurationContext struct {
	FilePath string
	Line     int
	Column   int
}

// ParentResourceContext provides context about parent resources for errors
type ParentResourceContext struct {
	ParentType string
	ParentID   string
	ParentName string
}

// ResourceSummary provides a summary of a resource for error messages
type ResourceSummary struct {
	ID     string
	Name   string
	Fields map[string]string
}

// SDKErrorType categorizes SDK errors for better user messaging
type SDKErrorType int

const (
	SDKErrorNetwork SDKErrorType = iota
	SDKErrorAuthentication
	SDKErrorAuthorization
	SDKErrorNotFound
	SDKErrorValidation
	SDKErrorServerError
	SDKErrorUnknown
)

// String returns a human-readable string for the SDK error type
func (t SDKErrorType) String() string {
	switch t {
	case SDKErrorNetwork:
		return "Network Error"
	case SDKErrorAuthentication:
		return "Authentication Error"
	case SDKErrorAuthorization:
		return "Authorization Error"
	case SDKErrorNotFound:
		return "Resource Not Found"
	case SDKErrorValidation:
		return "Validation Error"
	case SDKErrorServerError:
		return "Server Error"
	case SDKErrorUnknown:
		return "Unknown Error"
	default:
		return "Unknown Error"
	}
}

// ResourceValidationError represents a validation error for external resources
type ResourceValidationError struct {
	Ref           string
	ResourceType  string
	Field         string
	Value         string
	Message       string
	Suggestions   []string
	ConfigContext *ConfigurationContext
	Cause         error
}

// Error implements the error interface
func (e *ResourceValidationError) Error() string {
	msg := e.Message
	if e.Ref != "" {
		msg = "external resource \"" + e.Ref + "\": " + msg
	}
	if e.Field != "" {
		msg += " (field: " + e.Field + ")"
	}
	if len(e.Suggestions) > 0 {
		msg += "\nSuggestions:\n"
		for i, suggestion := range e.Suggestions {
			msg += "  " + string(rune('1'+i)) + ". " + suggestion + "\n"
		}
	}
	if e.ConfigContext != nil && e.ConfigContext.FilePath != "" {
		msg += "\nConfiguration: " + e.ConfigContext.FilePath
		if e.ConfigContext.Line > 0 {
			msg += ":" + string(rune('0'+e.ConfigContext.Line))
		}
	}
	return msg
}

// Unwrap returns the underlying cause
func (e *ResourceValidationError) Unwrap() error {
	return e.Cause
}

// ResourceResolutionError represents a resolution error for external resources
type ResourceResolutionError struct {
	Ref            string
	ResourceType   string
	Selector       map[string]string
	MatchedCount   int
	MatchedDetails []ResourceSummary
	Suggestions    []string
	ParentContext  *ParentResourceContext
	Cause          error
}

// Error implements the error interface
func (e *ResourceResolutionError) Error() string {
	msg := "external resource \"" + e.Ref + "\" resolution failed"
	
	if e.MatchedCount == 0 {
		msg += ": no matching resources found"
	} else {
		msg += ": matched " + string(rune('0'+e.MatchedCount)) + " resources (expected exactly 1)"
	}
	
	msg += "\n  Resource type: " + e.ResourceType
	
	if len(e.Selector) > 0 {
		msg += "\n  Selector:"
		for k, v := range e.Selector {
			msg += "\n    " + k + ": " + v
		}
	}
	
	if e.ParentContext != nil {
		msg += "\n  Parent: " + e.ParentContext.ParentType
		if e.ParentContext.ParentName != "" {
			msg += " (" + e.ParentContext.ParentName + ")"
		}
	}
	
	if len(e.MatchedDetails) > 0 {
		msg += "\n  Matched resources:"
		for i, detail := range e.MatchedDetails {
			if i >= 5 {
				msg += "\n    ... and " + string(rune('0'+len(e.MatchedDetails)-5)) + " more"
				break
			}
			msg += "\n    - " + detail.ID
			if detail.Name != "" {
				msg += " (" + detail.Name + ")"
			}
		}
	}
	
	if len(e.Suggestions) > 0 {
		msg += "\n  Suggestions:"
		for _, suggestion := range e.Suggestions {
			msg += "\n    - " + suggestion
		}
	}
	
	return msg
}

// Unwrap returns the underlying cause
func (e *ResourceResolutionError) Unwrap() error {
	return e.Cause
}

// ResourceSDKError represents an SDK error for external resources
type ResourceSDKError struct {
	Ref          string
	ResourceType string
	Operation    string
	SDKErrorType SDKErrorType
	HTTPStatus   int
	Message      string
	UserMessage  string
	Suggestions  []string
	Cause        error
}

// Error implements the error interface
func (e *ResourceSDKError) Error() string {
	msg := e.UserMessage
	if msg == "" {
		msg = e.Message
	}
	
	msg = "external resource \"" + e.Ref + "\": " + msg
	msg += "\n  Resource type: " + e.ResourceType
	msg += "\n  Operation: " + e.Operation
	msg += "\n  Error type: " + e.SDKErrorType.String()
	
	if e.HTTPStatus > 0 {
		msg += "\n  HTTP status: " + string(rune('0'+e.HTTPStatus/100)) + 
			string(rune('0'+(e.HTTPStatus/10)%10)) + 
			string(rune('0'+e.HTTPStatus%10))
	}
	
	if len(e.Suggestions) > 0 {
		msg += "\n  Suggestions:"
		for _, suggestion := range e.Suggestions {
			msg += "\n    - " + suggestion
		}
	}
	
	return msg
}

// Unwrap returns the underlying cause
func (e *ResourceSDKError) Unwrap() error {
	return e.Cause
}

// Resource is an interface for external resource types
// This avoids circular imports with the resources package
type Resource interface {
	GetRef() string
	GetResourceType() string
	GetID() *string
	GetSelector() Selector
	GetParent() Parent
	SetResolvedID(id string)
	SetResolvedResource(resource interface{})
	IsResolved() bool
}

// Selector is an interface for external resource selectors
type Selector interface {
	GetMatchFields() map[string]string
}

// Parent is an interface for external resource parents
type Parent interface {
	GetResourceType() string
	GetID() string
	GetRef() string
}