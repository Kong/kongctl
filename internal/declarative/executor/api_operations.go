package executor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/log"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// Use debug env var from labels package

// createAPI handles CREATE operations for APIs
func (e *Executor) createAPI(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Get logger from context
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)
	
	logger.Debug("Creating API",
		slog.Any("fields", change.Fields))
	
	// Extract API fields
	var api kkComps.CreateAPIRequest
	
	// Map required fields
	if err := common.ValidateRequiredFields(change.Fields, []string{"name"}); err != nil {
		return "", common.WrapWithResourceContext(err, "api", "")
	}
	api.Name = common.ExtractResourceName(change.Fields)
	
	// Map optional fields using utilities (SDK uses double pointers)
	common.MapOptionalStringFieldToPtr(&api.Description, change.Fields, "description")
	
	// Handle labels using centralized helper
	userLabels := labels.ExtractLabelsFromField(change.Fields["labels"])
	api.Labels = labels.BuildCreateLabels(userLabels, change.Namespace, change.Protection)
	
	logger.Debug("API will have labels",
		slog.Any("labels", api.Labels))
	
	// Create the API
	logger.Debug("Final API before creation",
		slog.String("name", api.Name),
		slog.Any("labels", api.Labels))
	resp, err := e.client.CreateAPI(ctx, api, change.Namespace)
	if err != nil {
		return "", common.FormatAPIError("api", api.Name, "create", err)
	}
	
	return resp.ID, nil
}


// deleteAPI handles DELETE operations for APIs
func (e *Executor) deleteAPI(ctx context.Context, change planner.PlannedChange) error {
	// First, validate protection status at execution time
	api, err := e.client.GetAPIByName(ctx, getResourceName(change.Fields))
	if err != nil {
		return fmt.Errorf("failed to fetch API for protection check: %w", err)
	}
	if api == nil {
		// API already deleted, consider this success
		return nil
	}
	
	// Check if API is protected
	isProtected := labels.IsProtectedResource(api.NormalizedLabels)
	if isProtected {
		return fmt.Errorf("resource is protected and cannot be deleted")
	}
	
	// Verify it's a managed resource
	if !labels.IsManagedResource(api.NormalizedLabels) {
		return fmt.Errorf("cannot delete API: not a KONGCTL-managed resource")
	}
	
	// Delete the API
	err = e.client.DeleteAPI(ctx, change.ResourceID)
	if err != nil {
		return fmt.Errorf("failed to delete API: %w", err)
	}
	
	return nil
}