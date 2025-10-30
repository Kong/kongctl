package executor

import (
	"context"
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/state"
)

// APIImplementationAdapter implements CreateDeleteOperations for API implementations.
type APIImplementationAdapter struct {
	client *state.Client
}

// NewAPIImplementationAdapter creates a new API implementation adapter.
func NewAPIImplementationAdapter(client *state.Client) *APIImplementationAdapter {
	return &APIImplementationAdapter{client: client}
}

// MapCreateFields maps planned change fields into an APIImplementation request payload.
func (a *APIImplementationAdapter) MapCreateFields(
	_ context.Context, _ *ExecutionContext, fields map[string]any, create *kkComps.APIImplementation,
) error {
	if create == nil {
		return fmt.Errorf("create request must not be nil")
	}

	serviceValue, ok := fields["service"]
	if !ok {
		return fmt.Errorf("service is required for API implementations")
	}

	serviceMap, ok := serviceValue.(map[string]any)
	if !ok {
		return fmt.Errorf("service must be an object")
	}

	serviceID, err := getStringField(serviceMap, "id")
	if err != nil {
		return fmt.Errorf("service.id is required: %w", err)
	}

	controlPlaneID, err := getStringField(serviceMap, "control_plane_id")
	if err != nil {
		return fmt.Errorf("service.control_plane_id is required: %w", err)
	}

	create.ServiceReference = &kkComps.ServiceReference{
		Service: &kkComps.APIImplementationService{
			ID:             serviceID,
			ControlPlaneID: controlPlaneID,
		},
	}
	create.Type = kkComps.APIImplementationTypeServiceReference

	return nil
}

// Create creates a new API implementation via the state client.
func (a *APIImplementationAdapter) Create(ctx context.Context, req kkComps.APIImplementation,
	_ string, execCtx *ExecutionContext,
) (string, error) {
	apiID, err := a.getAPIIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}

	resp, err := a.client.CreateAPIImplementation(ctx, apiID, req)
	if err != nil {
		return "", err
	}

	if resp == nil {
		return "", fmt.Errorf("create API implementation response missing data")
	}

	if sr := resp.APIImplementationResponseServiceReference; sr != nil {
		return sr.GetID(), nil
	}

	return "", fmt.Errorf("unexpected API implementation response format")
}

// Delete removes an API implementation.
func (a *APIImplementationAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	apiID, err := a.getAPIIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}

	return a.client.DeleteAPIImplementation(ctx, apiID, id)
}

// GetByName returns nil because API implementations do not expose a name-based lookup.
func (a *APIImplementationAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, nil
}

// GetByID returns nil because API implementations currently do not expose a fetch-by-ID helper.
func (a *APIImplementationAdapter) GetByID(_ context.Context, _ string, _ *ExecutionContext) (ResourceInfo, error) {
	return nil, nil
}

// ResourceType identifies the resource handled by this adapter.
func (a *APIImplementationAdapter) ResourceType() string {
	return "api_implementation"
}

// RequiredFields lists the required fields for creation.
func (a *APIImplementationAdapter) RequiredFields() []string {
	return []string{"service"}
}

// MapUpdateFields reports that updates are not supported.
func (a *APIImplementationAdapter) MapUpdateFields(
	_ context.Context, _ *ExecutionContext, _ map[string]any,
	_ *kkComps.APIImplementation, _ map[string]string,
) error {
	return fmt.Errorf("API implementations do not support update operations")
}

// Update is not supported for API implementations.
func (a *APIImplementationAdapter) Update(
	_ context.Context, _ string, _ kkComps.APIImplementation, _ string, _ *ExecutionContext,
) (string, error) {
	return "", fmt.Errorf("API implementations do not support update operations")
}

// SupportsUpdate returns false as API implementations don't support updates.
func (a *APIImplementationAdapter) SupportsUpdate() bool {
	return false
}

// APIImplementationResourceInfo implements ResourceInfo for API implementations.
type APIImplementationResourceInfo struct {
	implementation *state.APIImplementation
}

func (a *APIImplementationResourceInfo) GetID() string {
	return a.implementation.ID
}

func (a *APIImplementationResourceInfo) GetName() string {
	return a.implementation.ID // Implementations don't have names, use ID
}

func (a *APIImplementationResourceInfo) GetLabels() map[string]string {
	return make(map[string]string)
}

func (a *APIImplementationResourceInfo) GetNormalizedLabels() map[string]string {
	return make(map[string]string)
}

func (a *APIImplementationAdapter) getAPIIDFromExecutionContext(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context is required for API implementation operations")
	}

	change := *execCtx.PlannedChange

	if apiRef, ok := change.References["api_id"]; ok {
		if apiRef.ID != "" {
			return apiRef.ID, nil
		}
	}

	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}

	return "", fmt.Errorf("API ID is required for API implementation operations")
}

func getStringField(m map[string]any, key string) (string, error) {
	value, ok := m[key]
	if !ok {
		return "", fmt.Errorf("field %s missing", key)
	}

	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return "", fmt.Errorf("field %s is empty", key)
		}
		return v, nil
	case fmt.Stringer:
		str := v.String()
		if strings.TrimSpace(str) == "" {
			return "", fmt.Errorf("field %s is empty", key)
		}
		return str, nil
	default:
		return "", fmt.Errorf("field %s must be string", key)
	}
}
