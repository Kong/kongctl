package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/state"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// APIImplementationAdapter is a placeholder adapter for API implementations
// This resource type is not yet supported by the SDK
type APIImplementationAdapter struct {
	client *state.Client
}

// NewAPIImplementationAdapter creates a new API implementation adapter
func NewAPIImplementationAdapter(client *state.Client) *APIImplementationAdapter {
	return &APIImplementationAdapter{client: client}
}

// MapCreateFields is not implemented
func (a *APIImplementationAdapter) MapCreateFields(_ context.Context, _ *ExecutionContext, _ map[string]any,
	_ *kkComps.CreateAPIVersionRequest) error {
	return fmt.Errorf("API implementations are not yet supported by the SDK")
}

// MapUpdateFields is not implemented
func (a *APIImplementationAdapter) MapUpdateFields(_ context.Context, _ *ExecutionContext, _ map[string]any,
	_ *kkComps.APIVersion, _ map[string]string) error {
	return fmt.Errorf("API implementations are not yet supported by the SDK")
}

// Create is not implemented
func (a *APIImplementationAdapter) Create(_ context.Context, _ kkComps.CreateAPIVersionRequest,
	_ string) (string, error) {
	return "", fmt.Errorf("API implementations are not yet supported by the SDK")
}

// Update is not implemented
func (a *APIImplementationAdapter) Update(_ context.Context, _ string, _ kkComps.APIVersion,
	_ string) (string, error) {
	return "", fmt.Errorf("API implementations are not yet supported by the SDK")
}

// Delete is not implemented
func (a *APIImplementationAdapter) Delete(_ context.Context, _ string, _ *ExecutionContext) error {
	return fmt.Errorf("API implementations are not yet supported by the SDK")
}

// GetByName is not implemented
func (a *APIImplementationAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("API implementations are not yet supported by the SDK")
}

// ResourceType returns the resource type name
func (a *APIImplementationAdapter) ResourceType() string {
	return "api_implementation"
}

// RequiredFields returns the required fields for creation
func (a *APIImplementationAdapter) RequiredFields() []string {
	// Return empty slice as this resource is not supported
	return []string{}
}

// SupportsUpdate returns false as implementations are not supported
func (a *APIImplementationAdapter) SupportsUpdate() bool {
	return false
}

// APIImplementationResourceInfo is a placeholder for API implementation resource info
type APIImplementationResourceInfo struct {
	// Empty struct as implementations are not supported
}

func (a *APIImplementationResourceInfo) GetID() string {
	return ""
}

func (a *APIImplementationResourceInfo) GetName() string {
	return ""
}

func (a *APIImplementationResourceInfo) GetLabels() map[string]string {
	return make(map[string]string)
}

func (a *APIImplementationResourceInfo) GetNormalizedLabels() map[string]string {
	return make(map[string]string)
}