package executor

import (
	"context"

	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
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
func (p *APIAdapter) MapCreateFields(ctx context.Context, fields map[string]interface{},
	create *kkComps.CreateAPIRequest) error {
	// Extract namespace and protection from context
	namespace, _ := ctx.Value(contextKeyNamespace).(string)
	protection := ctx.Value(contextKeyProtection)

	// Map required fields
	create.Name = common.ExtractResourceName(fields)

	// Map optional fields using utilities (SDK uses double pointers)
	common.MapOptionalStringFieldToPtr(&create.Description, fields, "description")

	// Handle labels using centralized helper
	userLabels := labels.ExtractLabelsFromField(fields["labels"])
	create.Labels = labels.BuildCreateLabels(userLabels, namespace, protection)

	return nil
}

// MapUpdateFields maps fields to UpdateAPIRequest
func (p *APIAdapter) MapUpdateFields(ctx context.Context, fields map[string]interface{},
	update *kkComps.UpdateAPIRequest, currentLabels map[string]string) error {
	// Extract namespace and protection from context
	namespace, _ := ctx.Value(contextKeyNamespace).(string)
	protection := ctx.Value(contextKeyProtection)

	// Only include fields that are in the fields map
	// These represent actual changes detected by the planner
	for field, value := range fields {
		switch field {
		case "name":
			if name, ok := value.(string); ok {
				update.Name = &name
			}
		case "description":
			if desc, ok := value.(string); ok {
				update.Description = &desc
			}
		// Skip "labels" as they're handled separately below
		}
	}

	// Handle labels using centralized helper
	desiredLabels := labels.ExtractLabelsFromField(fields["labels"])
	if desiredLabels != nil {
		// Get current labels if passed from planner
		plannerCurrentLabels := labels.ExtractLabelsFromField(fields[planner.FieldCurrentLabels])
		if plannerCurrentLabels != nil {
			currentLabels = plannerCurrentLabels
		}

		// Build update labels with removal support
		update.Labels = labels.BuildUpdateLabels(desiredLabels, currentLabels, namespace, protection)
	} else if currentLabels != nil {
		// If no labels in change, preserve existing labels with updated protection
		update.Labels = labels.BuildUpdateLabels(currentLabels, currentLabels, namespace, protection)
	}

	return nil
}

// Create creates a new API
func (p *APIAdapter) Create(ctx context.Context, req kkComps.CreateAPIRequest, namespace string) (string, error) {
	resp, err := p.client.CreateAPI(ctx, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// Update updates an existing API
func (p *APIAdapter) Update(ctx context.Context, id string, req kkComps.UpdateAPIRequest,
	namespace string) (string, error) {
	resp, err := p.client.UpdateAPI(ctx, id, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// Delete deletes an API
func (p *APIAdapter) Delete(ctx context.Context, id string) error {
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

// ResourceType returns the resource type name
func (p *APIAdapter) ResourceType() string {
	return "api"
}

// RequiredFields returns the required fields for creation
func (p *APIAdapter) RequiredFields() []string {
	return []string{"name"}
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