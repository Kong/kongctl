package executor

import (
	"context"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// CatalogServiceAdapter implements ResourceOperations for catalog services.
type CatalogServiceAdapter struct {
	client *state.Client
}

// NewCatalogServiceAdapter creates a new catalog service adapter.
func NewCatalogServiceAdapter(client *state.Client) *CatalogServiceAdapter {
	return &CatalogServiceAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateCatalogService.
func (a *CatalogServiceAdapter) MapCreateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateCatalogService,
) error {
	namespace := execCtx.Namespace
	protection := execCtx.Protection

	create.Name = common.ExtractResourceName(fields)

	if displayName, ok := fields["display_name"].(string); ok {
		create.DisplayName = displayName
	}

	common.MapOptionalStringFieldToPtr(&create.Description, fields, "description")

	userLabels := labels.ExtractLabelsFromField(fields["labels"])
	create.Labels = labels.BuildCreateLabels(userLabels, namespace, protection)

	if customFields, ok := fields["custom_fields"]; ok {
		create.CustomFields = customFields
	}

	return nil
}

// MapUpdateFields maps planner fields to UpdateCatalogService.
func (a *CatalogServiceAdapter) MapUpdateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateCatalogService,
	currentLabels map[string]string,
) error {
	namespace := execCtx.Namespace
	protection := execCtx.Protection

	for field, value := range fields {
		switch field {
		case "name":
			if name, ok := value.(string); ok {
				update.Name = &name
			}
		case "display_name":
			if displayName, ok := value.(string); ok {
				update.DisplayName = &displayName
			}
		case "description":
			if desc, ok := value.(string); ok {
				update.Description = &desc
			}
		case "custom_fields":
			update.CustomFields = value
		}
	}

	desiredLabels := labels.ExtractLabelsFromField(fields["labels"])
	if desiredLabels != nil {
		plannerCurrentLabels := labels.ExtractLabelsFromField(fields[planner.FieldCurrentLabels])
		if plannerCurrentLabels != nil {
			currentLabels = plannerCurrentLabels
		}
		update.Labels = labels.BuildUpdateLabels(desiredLabels, currentLabels, namespace, protection)
	} else if currentLabels != nil {
		update.Labels = labels.BuildUpdateLabels(currentLabels, currentLabels, namespace, protection)
	}

	return nil
}

// Create creates a catalog service.
func (a *CatalogServiceAdapter) Create(
	ctx context.Context,
	req kkComps.CreateCatalogService,
	namespace string,
	_ *ExecutionContext,
) (string, error) {
	resp, err := a.client.CreateCatalogService(ctx, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// Update updates a catalog service.
func (a *CatalogServiceAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateCatalogService,
	namespace string,
	_ *ExecutionContext,
) (string, error) {
	resp, err := a.client.UpdateCatalogService(ctx, id, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// Delete deletes a catalog service.
func (a *CatalogServiceAdapter) Delete(ctx context.Context, id string, _ *ExecutionContext) error {
	return a.client.DeleteCatalogService(ctx, id)
}

// GetByName fetches a catalog service by name.
func (a *CatalogServiceAdapter) GetByName(ctx context.Context, name string) (ResourceInfo, error) {
	svc, err := a.client.GetCatalogServiceByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, nil
	}
	return &catalogServiceResourceInfo{svc: svc}, nil
}

// GetByID fetches a catalog service by ID.
func (a *CatalogServiceAdapter) GetByID(ctx context.Context, id string, _ *ExecutionContext) (ResourceInfo, error) {
	svc, err := a.client.GetCatalogServiceByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, nil
	}
	return &catalogServiceResourceInfo{svc: svc}, nil
}

// ResourceType returns the resource type.
func (a *CatalogServiceAdapter) ResourceType() string {
	return "catalog_service"
}

// RequiredFields returns required fields for create.
func (a *CatalogServiceAdapter) RequiredFields() []string {
	return []string{"name", "display_name"}
}

// SupportsUpdate indicates update support.
func (a *CatalogServiceAdapter) SupportsUpdate() bool {
	return true
}

type catalogServiceResourceInfo struct {
	svc *state.CatalogService
}

func (c *catalogServiceResourceInfo) GetID() string {
	return c.svc.ID
}

func (c *catalogServiceResourceInfo) GetName() string {
	return c.svc.Name
}

func (c *catalogServiceResourceInfo) GetLabels() map[string]string {
	return c.svc.Labels
}

func (c *catalogServiceResourceInfo) GetNormalizedLabels() map[string]string {
	return c.svc.NormalizedLabels
}
