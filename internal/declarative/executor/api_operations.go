package executor

import (
	"context"
	"fmt"
	"os"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

const debugEnvVar = "KONGCTL_DEBUG"

// createAPI handles CREATE operations for APIs
func (e *Executor) createAPI(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Debug logging
	debugEnabled := os.Getenv(debugEnvVar) == labels.TrueValue
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG [api_operations]: "+format+"\n", args...)
		}
	}
	
	debugLog("Creating API with fields: %+v", change.Fields)
	
	// Extract API fields
	var api kkComps.CreateAPIRequest
	
	// Required fields
	if name, ok := change.Fields["name"].(string); ok {
		api.Name = name
	} else {
		return "", fmt.Errorf("API name is required")
	}
	
	// Optional fields
	if desc, ok := change.Fields["description"].(string); ok {
		api.Description = &desc
	}
	
	// Handle labels - preserve user labels and protected label
	// The state client will add management labels (managed, last-updated)
	if labelsField, ok := change.Fields["labels"].(map[string]interface{}); ok {
		debugLog("Found labels in fields: %+v", labelsField)
		apiLabels := make(map[string]string)
		for k, v := range labelsField {
			if strVal, ok := v.(string); ok {
				// Allow user labels and the protected label
				if !labels.IsKongctlLabel(k) || k == labels.ProtectedKey {
					apiLabels[k] = strVal
					debugLog("Adding label: %s=%s", k, strVal)
				}
			}
		}
		if len(apiLabels) > 0 {
			api.Labels = apiLabels
			debugLog("API will have %d labels", len(apiLabels))
		} else {
			debugLog("No labels to set on API")
		}
	} else {
		debugLog("No labels field found in change")
	}
	
	// Create the API
	debugLog("Final API before creation: Name=%s, Labels=%+v", api.Name, api.Labels)
	resp, err := e.client.CreateAPI(ctx, api)
	if err != nil {
		return "", err
	}
	
	return resp.ID, nil
}

// updateAPI handles UPDATE operations for APIs
func (e *Executor) updateAPI(ctx context.Context, change planner.PlannedChange) (string, error) {
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
	var protectionChange planner.ProtectionChange
	
	// Handle both direct struct and map from JSON deserialization
	switch p := change.Protection.(type) {
	case planner.ProtectionChange:
		isProtectionChange = true
		protectionChange = p
	case map[string]interface{}:
		// From JSON deserialization
		if oldVal, hasOld := p["old"].(bool); hasOld {
			if newVal, hasNew := p["new"].(bool); hasNew {
				isProtectionChange = true
				protectionChange = planner.ProtectionChange{
					Old: oldVal,
					New: newVal,
				}
			}
		}
	}
	
	if isProtected && !isProtectionChange {
		// Regular update to a protected resource is not allowed
		return "", fmt.Errorf("resource is protected and cannot be updated")
	}
	
	// Build sparse update request - only include fields that changed
	var updateAPI kkComps.UpdateAPIRequest
	
	// Name is always required for updates
	if name, ok := change.Fields["name"].(string); ok {
		updateAPI.Name = &name
	} else {
		return "", fmt.Errorf("API name is required")
	}
	
	// Only include fields that are in the change.Fields map (excluding "name")
	// These represent actual changes detected by the planner
	for field, value := range change.Fields {
		switch field {
		case "description":
			if desc, ok := value.(string); ok {
				updateAPI.Description = &desc
			}
		// Skip "name" as it's already handled above
		// Skip "labels" as they're handled separately below
		}
	}
	
	// Handle labels - preserve existing user labels from the API
	userLabels := make(map[string]string)
	for k, v := range api.Labels {
		if !labels.IsKongctlLabel(k) {
			userLabels[k] = v
		}
	}
	
	// Apply any label updates from the change
	protectionFromFields := ""
	if labelsField, ok := change.Fields["labels"].(map[string]interface{}); ok {
		for k, v := range labelsField {
			if strVal, ok := v.(string); ok {
				if k == labels.ProtectedKey {
					// Track protection label from fields
					protectionFromFields = strVal
				} else if !labels.IsKongctlLabel(k) {
					userLabels[k] = strVal
				}
			}
		}
	}
	
	// Update management labels with new timestamp
	allLabels := labels.AddManagedLabels(userLabels)
	
	// Handle protection label changes
	if isProtectionChange {
		if protectionChange.New {
			// Setting protection to true
			allLabels[labels.ProtectedKey] = labels.TrueValue
		} else {
			// Setting protection to false
			allLabels[labels.ProtectedKey] = labels.FalseValue
		}
	} else if protectionFromFields != "" {
		// Use protection value from fields if provided
		allLabels[labels.ProtectedKey] = protectionFromFields
	} else if isProtected {
		// Preserve existing protection
		allLabels[labels.ProtectedKey] = labels.TrueValue
	}
	
	updateAPI.Labels = labels.DenormalizeLabels(allLabels)
	
	// Update the API
	resp, err := e.client.UpdateAPI(ctx, change.ResourceID, updateAPI)
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