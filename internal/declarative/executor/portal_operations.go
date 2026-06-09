package executor

import (
	"context"
	"fmt"
	"log/slog"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/common"
	decerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/log"
)

// createPortal handles CREATE operations for portals
func (e *Executor) createPortal(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Get logger from context
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)

	logger.Debug("Creating portal",
		slog.Any("fields", change.Fields))

	// Extract portal fields
	var portal kkComps.CreatePortal

	// Map required fields
	if err := common.ValidateRequiredFields(change.Fields, []string{planner.FieldName}); err != nil {
		return "", common.WrapWithResourceContext(err, planner.ResourceTypePortal, "")
	}
	portal.Name = common.ExtractResourceName(change.Fields)

	// Map optional fields using utilities (SDK uses double pointers)
	common.MapOptionalStringFieldToPtr(&portal.Description, change.Fields, planner.FieldDescription)
	common.MapOptionalStringFieldToPtr(&portal.DisplayName, change.Fields, planner.FieldDisplayName)
	common.MapOptionalBoolFieldToPtr(&portal.AuthenticationEnabled, change.Fields, planner.FieldAuthenticationEnabled)
	common.MapOptionalBoolFieldToPtr(&portal.RbacEnabled, change.Fields, planner.FieldRBACEnabled)
	common.MapOptionalBoolFieldToPtr(&portal.AutoApproveDevelopers, change.Fields, planner.FieldAutoApproveDevelopers)
	common.MapOptionalBoolFieldToPtr(&portal.AutoApproveApplications, change.Fields, planner.FieldAutoApproveApplications)

	if defaultAPIVisibility, ok := change.Fields[planner.FieldDefaultAPIVisibility].(string); ok {
		visibility := kkComps.DefaultAPIVisibility(defaultAPIVisibility)
		portal.DefaultAPIVisibility = &visibility
	}

	if defaultPageVisibility, ok := change.Fields[planner.FieldDefaultPageVisibility].(string); ok {
		visibility := kkComps.DefaultPageVisibility(defaultPageVisibility)
		portal.DefaultPageVisibility = &visibility
	}

	if defaultAppAuthStrategyID, ok := change.Fields[planner.FieldDefaultApplicationStrategyID].(string); ok {
		portal.DefaultApplicationAuthStrategyID = &defaultAppAuthStrategyID
	}

	// Handle labels using centralized helper
	userLabels := labels.ExtractLabelsFromField(change.Fields[planner.FieldLabels])
	portalLabels := labels.BuildCreateLabels(userLabels, change.Namespace, change.Protection)

	// Convert to pointer map since portals use map[string]*string
	portal.Labels = labels.ConvertStringMapToPointerMap(portalLabels)

	logger.Debug("Portal will have labels",
		slog.Any("labels", portal.Labels))

	// Create the portal
	logger.Debug("Final portal before creation",
		slog.String("name", portal.Name),
		slog.Any("labels", portal.Labels))
	resp, err := e.client.CreatePortal(ctx, portal, change.Namespace)
	if err != nil {
		return "", common.FormatAPIError(planner.ResourceTypePortal, portal.Name, "create", err)
	}

	return resp.ID, nil
}

// updatePortal handles UPDATE operations for portals
func (e *Executor) updatePortal(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Get logger from context
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)

	// First, validate protection status at execution time
	portalName := getResourceName(change.Fields)
	portal, err := e.client.GetPortalByName(ctx, portalName)
	if err != nil {
		return "", decerrors.FormatResourceError("fetch", planner.ResourceTypePortal, portalName, change.Namespace, err)
	}
	if portal == nil {
		return "", decerrors.FormatValidationError(planner.ResourceTypePortal, portalName, "resource",
			"no longer exists - it may have been deleted by another process")
	}

	// Check if portal is protected
	// Protection changes are always allowed (to unprotect a resource)
	isProtected := portal.NormalizedLabels[labels.ProtectedKey] == "true"

	// Check if this is a protection change
	isProtectionChange := false

	// Handle both direct struct and map from JSON deserialization
	switch p := change.Protection.(type) {
	case planner.ProtectionChange:
		isProtectionChange = true
	case map[string]any:
		// From JSON deserialization
		if _, hasOld := p["old"].(bool); hasOld {
			if _, hasNew := p["new"].(bool); hasNew {
				isProtectionChange = true
			}
		}
	}

	if isProtected && !isProtectionChange {
		// Regular update to a protected resource is not allowed
		return "", decerrors.FormatProtectionError(planner.ResourceTypePortal, portalName, "update")
	}

	// Build sparse update request - only include fields that changed
	var updatePortal kkComps.UpdatePortal

	// Only include fields that are in the change.Fields map
	// These represent actual changes detected by the planner
	for field, value := range change.Fields {
		switch field {
		case planner.FieldName:
			if name, ok := value.(string); ok {
				updatePortal.Name = &name
			}
		case planner.FieldDescription:
			if desc, ok := value.(string); ok {
				updatePortal.Description = &desc
			}
		case planner.FieldDisplayName:
			if displayName, ok := value.(string); ok {
				updatePortal.DisplayName = &displayName
			}
		case planner.FieldAuthenticationEnabled:
			if authEnabled, ok := value.(bool); ok {
				updatePortal.AuthenticationEnabled = &authEnabled
			}
		case planner.FieldRBACEnabled:
			if rbacEnabled, ok := value.(bool); ok {
				updatePortal.RbacEnabled = &rbacEnabled
			}
		case planner.FieldAutoApproveDevelopers:
			if autoApproveDevelopers, ok := value.(bool); ok {
				updatePortal.AutoApproveDevelopers = &autoApproveDevelopers
			}
		case planner.FieldAutoApproveApplications:
			if autoApproveApplications, ok := value.(bool); ok {
				updatePortal.AutoApproveApplications = &autoApproveApplications
			}
		case planner.FieldDefaultApplicationStrategyID:
			if authID, ok := value.(string); ok {
				updatePortal.DefaultApplicationAuthStrategyID = &authID
			}
		case planner.FieldDefaultAPIVisibility:
			if visibility, ok := value.(string); ok {
				vis := kkComps.UpdatePortalDefaultAPIVisibility(visibility)
				updatePortal.DefaultAPIVisibility = &vis
			}
		case planner.FieldDefaultPageVisibility:
			if visibility, ok := value.(string); ok {
				vis := kkComps.UpdatePortalDefaultPageVisibility(visibility)
				updatePortal.DefaultPageVisibility = &vis
			}
			// Skip "labels" as they're handled separately below
		}
	}

	// Handle labels using centralized helper
	desiredLabels := labels.ExtractLabelsFromField(change.Fields[planner.FieldLabels])
	if desiredLabels != nil {
		// Get current labels if passed from planner
		currentLabels := labels.ExtractLabelsFromField(change.Fields[planner.FieldCurrentLabels])

		// Build update labels with removal support
		updatePortal.Labels = labels.BuildUpdateLabels(
			desiredLabels,
			currentLabels,
			change.Namespace,
			change.Protection,
		)

		logger.Debug("Update request labels (with removal support)",
			slog.Any("labels", updatePortal.Labels))
	} else {
		// If no labels in change, preserve existing labels with updated protection
		currentLabels := make(map[string]string)
		for k, v := range portal.Labels {
			if !labels.IsKongctlLabel(k) {
				currentLabels[k] = v
			}
		}

		// Build labels just for protection update
		updatePortal.Labels = labels.BuildUpdateLabels(currentLabels, currentLabels, change.Namespace, change.Protection)
	}

	// Update the portal
	resp, err := e.client.UpdatePortal(ctx, change.ResourceID, updatePortal, change.Namespace)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

// deletePortal handles DELETE operations for portals
func (e *Executor) deletePortal(ctx context.Context, change planner.PlannedChange) error {
	// First, validate protection status at execution time
	portal, err := e.client.GetPortalByName(ctx, getResourceName(change.Fields))
	if err != nil {
		return fmt.Errorf("failed to fetch portal for protection check: %w", err)
	}
	if portal == nil {
		// Portal already deleted, consider this success
		return nil
	}

	// Check if portal is protected
	isProtected := portal.NormalizedLabels[labels.ProtectedKey] == "true"
	if isProtected {
		return fmt.Errorf("resource is protected and cannot be deleted")
	}

	// Verify it's a managed resource
	if !labels.IsManagedResource(portal.NormalizedLabels) {
		return fmt.Errorf("cannot delete portal: not a KONGCTL-managed resource")
	}

	// Delete the portal with force=true to cascade delete child resources
	err = e.client.DeletePortal(ctx, change.ResourceID, true)
	if err != nil {
		return fmt.Errorf("failed to delete portal: %w", err)
	}

	return nil
}
