package executor

import (
	"context"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/attributes"
	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/util"
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
	create.Name = kk.String(common.ExtractResourceName(fields))

	// Map optional fields using utilities (SDK uses double pointers)
	common.MapOptionalStringFieldToPtr(&create.Description, fields, "description")
	common.MapOptionalStringFieldToPtr(&create.Version, fields, "version")
	common.MapOptionalStringFieldToPtr(&create.Slug, fields, "slug")

	// Handle labels using centralized helper
	userLabels := labels.ExtractLabelsFromField(fields["labels"])
	create.Labels = labels.BuildCreateLabels(userLabels, namespace, protection)

	if attrs, ok := fields["attributes"]; ok {
		if normalized, ok := attributes.NormalizeAPIAttributes(attrs); ok {
			create.Attributes = normalized
		} else {
			create.Attributes = attrs
		}
	}

	return nil
}

// MapUpdateFields maps fields to UpdateAPIRequest
func (p *APIAdapter) MapUpdateFields(_ context.Context, execCtx *ExecutionContext, fields map[string]any,
	update *kkComps.UpdateAPIRequest, currentLabels map[string]string,
) error {
	// Extract namespace and protection from execution context
	namespace := execCtx.Namespace
	protection := execCtx.Protection

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
		case "version":
			if version, ok := value.(string); ok {
				update.Version = &version
			}
		case "slug":
			if slug, ok := value.(string); ok {
				update.Slug = &slug
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

	if attrs, ok := fields["attributes"]; ok {
		if normalized, ok := attributes.NormalizeAPIAttributes(attrs); ok {
			update.Attributes = normalized
		} else {
			update.Attributes = attrs
		}
	}

	return nil
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
	namespace string, _ *ExecutionContext,
) (string, error) {
	resp, err := p.client.UpdateAPI(ctx, id, req, namespace)
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
	return util.StringValue(a.api.Name)
}

func (a *APIResourceInfo) GetLabels() map[string]string {
	// API.Labels is already map[string]string
	return a.api.Labels
}

func (a *APIResourceInfo) GetNormalizedLabels() map[string]string {
	return a.api.NormalizedLabels
}
