package executor

import (
	"context"
	"fmt"
	"log/slog"

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
	
	// Map fields
	if name, ok := change.Fields["name"].(string); ok {
		api.Name = name
	}
	
	// Optional fields
	if desc, ok := change.Fields["description"].(string); ok {
		api.Description = &desc
	}
	
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
		return "", err
	}
	
	return resp.ID, nil
}

// updateAPI handles UPDATE operations for APIs
func (e *Executor) updateAPI(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Get logger from context
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)
	
	// First, validate protection status at execution time
	api, err := e.client.GetAPIByName(ctx, getResourceName(change.Fields))
	if err != nil {
		return "", fmt.Errorf("failed to fetch API for protection check: %w", err)
	}
	if api == nil {
		return "", fmt.Errorf("API no longer exists")
	}
	
	// Check if API is protected
	// Protection changes are always allowed (to unprotect a resource)
	isProtected := labels.IsProtectedResource(api.NormalizedLabels)
	
	// Check if this is a protection change
	isProtectionChange := false
	
	// Handle both direct struct and map from JSON deserialization
	switch p := change.Protection.(type) {
	case planner.ProtectionChange:
		isProtectionChange = true
	case map[string]interface{}:
		// From JSON deserialization
		if _, hasOld := p["old"].(bool); hasOld {
			if _, hasNew := p["new"].(bool); hasNew {
				isProtectionChange = true
			}
		}
	}
	
	if isProtected && !isProtectionChange {
		// Regular update to a protected resource is not allowed
		return "", fmt.Errorf("resource is protected and cannot be updated")
	}
	
	// Build sparse update request - only include fields that changed
	var updateAPI kkComps.UpdateAPIRequest
	
	// Only include fields that are in the change.Fields map
	// These represent actual changes detected by the planner
	for field, value := range change.Fields {
		switch field {
		case "name":
			if name, ok := value.(string); ok {
				updateAPI.Name = &name
			}
		case "description":
			if desc, ok := value.(string); ok {
				updateAPI.Description = &desc
			}
		// Skip "labels" as they're handled separately below
		}
	}
	
	// Handle labels using centralized helper
	desiredLabels := labels.ExtractLabelsFromField(change.Fields["labels"])
	if desiredLabels != nil {
		// Get current labels if passed from planner
		currentLabels := labels.ExtractLabelsFromField(change.Fields[planner.FieldCurrentLabels])
		
		// Build update labels with removal support
		updateAPI.Labels = labels.BuildUpdateLabels(desiredLabels, currentLabels, change.Namespace, change.Protection)
		
		logger.Debug("Update request labels (with removal support)",
			slog.Any("labels", updateAPI.Labels))
	} else {
		// If no labels in change, preserve existing labels with updated protection
		currentLabels := make(map[string]string)
		for k, v := range api.Labels {
			if !labels.IsKongctlLabel(k) {
				currentLabels[k] = v
			}
		}
		
		// Build labels just for protection update
		updateAPI.Labels = labels.BuildUpdateLabels(currentLabels, currentLabels, change.Namespace, change.Protection)
	}
	
	// Update the API
	resp, err := e.client.UpdateAPI(ctx, change.ResourceID, updateAPI, change.Namespace)
	if err != nil {
		return "", err
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