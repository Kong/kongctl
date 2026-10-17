package executor

import (
	"context"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/fieldcontracts"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// APIAdapter implements ResourceOperations for APIs
type APIAdapter struct {
	client *state.Client
}

// NewAPIAdapter creates a new API adapter
func NewAPIAdapter(client *state.Client) *APIAdapter {
	return &APIAdapter{client: client}
}

// MapCreateFields maps fields to CreateAPIRequest
func (p *APIAdapter) MapCreateFields(_ context.Context, execCtx *ExecutionContext, fields map[string]any,
	create *kkComps.CreateAPIRequest,
) error {
	// Extract namespace and protection from execution context
	namespace := execCtx.Namespace
	protection := execCtx.Protection

	// Map required fields
	create.Name = common.ExtractResourceName(fields)

	fieldcontracts.MapAPICreateFields(fields, create)

	// Handle labels using centralized helper
	userLabels := labels.ExtractLabelsFromField(fields[planner.FieldLabels])
	create.Labels = labels.BuildCreateLabels(userLabels, namespace, protection)

	return nil
}

// MapUpdateFields maps fields to UpdateAPIRequest
func (p *APIAdapter) MapUpdateFields(_ context.Context, _ *ExecutionContext, fields map[string]any,
	update *kkComps.UpdateAPIRequest, _ map[string]string,
) error {
	if name, ok := fields[planner.FieldName].(string); ok {
		update.Name = &name
	}
	fieldcontracts.MapAPIUpdateFields(fields, update)

	return nil
}

func (p *APIAdapter) MapUpdateLabels(
	execCtx *ExecutionContext,
	update *kkComps.UpdateAPIRequest,
	desiredLabels map[string]string,
	currentLabels map[string]string,
) {
	mapPointerUpdateLabels(&update.Labels, execCtx, desiredLabels, currentLabels)
}

// Create creates a new API
func (p *APIAdapter) Create(ctx context.Context, req kkComps.CreateAPIRequest,
	namespace string, _ *ExecutionContext,
) (string, error) {
	resp, err := p.client.CreateAPI(ctx, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// Update updates an existing API
func (p *APIAdapter) Update(ctx context.Context, id string, req kkComps.UpdateAPIRequest,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	resp, err := p.client.UpdateAPI(ctx, id, req, namespace, execCtx.allowsOperationRemoval())
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// Delete deletes an API
func (p *APIAdapter) Delete(ctx context.Context, id string, _ *ExecutionContext) error {
	return p.client.DeleteAPI(ctx, id)
}

// GetByName gets an API by name
func (p *APIAdapter) GetByName(ctx context.Context, name string) (ResourceInfo, error) {
	api, err := p.client.GetAPIByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if api == nil {
		return nil, nil
	}
	return &APIResourceInfo{api: api}, nil
}

// GetByID gets an API by ID
func (p *APIAdapter) GetByID(ctx context.Context, id string, _ *ExecutionContext) (ResourceInfo, error) {
	api, err := p.client.GetAPIByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if api == nil {
		return nil, nil
	}
	return &APIResourceInfo{api: api}, nil
}

// ResourceType returns the resource type name
func (p *APIAdapter) ResourceType() string {
	return planner.ResourceTypeAPI
}

// RequiredFields returns the required fields for creation
func (p *APIAdapter) RequiredFields() []string {
	return []string{planner.FieldName}
}

// SupportsUpdate returns true as APIs support updates
func (p *APIAdapter) SupportsUpdate() bool {
	return true
}

// APIResourceInfo wraps an API to implement ResourceInfo
type APIResourceInfo struct {
	api *state.API
}

func (a *APIResourceInfo) GetID() string {
	return a.api.ID
}

func (a *APIResourceInfo) GetName() string {
	return a.api.Name
}

func (a *APIResourceInfo) GetLabels() map[string]string {
	// API.Labels is already map[string]string
	return a.api.Labels
}

func (a *APIResourceInfo) GetNormalizedLabels() map[string]string {
	return a.api.NormalizedLabels
}
