package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/state"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// APIDocumentAdapter implements ResourceOperations for API documents
type APIDocumentAdapter struct {
	client  *state.Client
	execCtx *ExecutionContext // Store execution context for helper methods
}

// NewAPIDocumentAdapter creates a new API document adapter
func NewAPIDocumentAdapter(client *state.Client) *APIDocumentAdapter {
	return &APIDocumentAdapter{client: client}
}

// MapCreateFields maps fields to CreateAPIDocumentRequest
func (a *APIDocumentAdapter) MapCreateFields(
	_ context.Context, execCtx *ExecutionContext, fields map[string]any,
	create *kkComps.CreateAPIDocumentRequest) error {
	// Store execution context for use in helper methods
	a.execCtx = execCtx
	
	// Required fields
	title, ok := fields["title"].(string)
	if !ok {
		return fmt.Errorf("title is required")
	}
	create.Title = &title

	content, ok := fields["content"].(string)
	if !ok {
		return fmt.Errorf("content is required")
	}
	create.Content = content

	// Optional fields
	if slug, ok := fields["slug"].(string); ok {
		create.Slug = &slug
	}

	if statusStr, ok := fields["status"].(string); ok {
		status := kkComps.APIDocumentStatus(statusStr)
		create.Status = &status
	}

	return nil
}

// MapUpdateFields maps fields to APIDocument
func (a *APIDocumentAdapter) MapUpdateFields(_ context.Context, _ *ExecutionContext, fields map[string]any,
	update *kkComps.APIDocument, _ map[string]string) error {
	// Optional fields - all fields are optional for updates
	if title, ok := fields["title"].(string); ok {
		update.Title = &title
	}

	if content, ok := fields["content"].(string); ok {
		update.Content = &content
	}

	if slug, ok := fields["slug"].(string); ok {
		update.Slug = &slug
	}

	if statusStr, ok := fields["status"].(string); ok {
		status := kkComps.APIDocumentStatus(statusStr)
		update.Status = &status
	}

	return nil
}

// Create creates a new API document
func (a *APIDocumentAdapter) Create(ctx context.Context, req kkComps.CreateAPIDocumentRequest,
	_ string) (string, error) {
	// Get API ID from context
	apiID, err := a.getAPIID(ctx)
	if err != nil {
		return "", err
	}

	resp, err := a.client.CreateAPIDocument(ctx, apiID, req)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("API document creation returned no response")
	}
	return resp.ID, nil
}

// Update updates an existing API document
func (a *APIDocumentAdapter) Update(ctx context.Context, id string, req kkComps.APIDocument,
	_ string) (string, error) {
	// Get API ID from context
	apiID, err := a.getAPIID(ctx)
	if err != nil {
		return "", err
	}

	_, err = a.client.UpdateAPIDocument(ctx, apiID, id, req)
	if err != nil {
		return "", err
	}
	return id, nil
}

// Delete deletes an API document
func (a *APIDocumentAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	// Get API ID from execution context
	apiID, err := a.getAPIIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}

	return a.client.DeleteAPIDocument(ctx, apiID, id)
}

// GetByName gets an API document by slug
func (a *APIDocumentAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	// API documents don't have a direct "get by name" method
	// The planner handles this by searching through the list
	return nil, nil
}

// GetByID gets an API document by ID using API context
func (a *APIDocumentAdapter) GetByID(ctx context.Context, id string) (ResourceInfo, error) {
	// Get API ID from context using existing pattern
	apiID, err := a.getAPIID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get API ID for document lookup: %w", err)
	}
	
	// Use existing client method
	document, err := a.client.GetAPIDocument(ctx, apiID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get API document: %w", err)
	}
	if document == nil {
		return nil, nil
	}
	
	return &APIDocumentResourceInfo{document: document}, nil
}

// ResourceType returns the resource type name
func (a *APIDocumentAdapter) ResourceType() string {
	return "api_document"
}

// RequiredFields returns the required fields for creation
func (a *APIDocumentAdapter) RequiredFields() []string {
	return []string{"title", "content"}
}

// SupportsUpdate returns true as documents support updates
func (a *APIDocumentAdapter) SupportsUpdate() bool {
	return true
}

// getAPIID extracts the API ID from the stored execution context (used for Create operations)
func (a *APIDocumentAdapter) getAPIID(_ context.Context) (string, error) {
	// Use stored context (for Create operations)
	if a.execCtx != nil && a.execCtx.PlannedChange != nil {
		change := *a.execCtx.PlannedChange
		if apiRef, ok := change.References["api_id"]; ok && apiRef.ID != "" {
			return apiRef.ID, nil
		}
	}
	
	return "", fmt.Errorf("API ID is required for document operations")
}

// getAPIIDFromExecutionContext extracts the API ID from ExecutionContext parameter (used for Delete operations)
func (a *APIDocumentAdapter) getAPIIDFromExecutionContext(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context is required for document operations")
	}
	
	change := *execCtx.PlannedChange
	
	// Priority 1: Check References (for Create operations)
	if apiRef, ok := change.References["api_id"]; ok && apiRef.ID != "" {
		return apiRef.ID, nil
	}
	
	// Priority 2: Check Parent field (for Delete operations)
	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}
	
	return "", fmt.Errorf("API ID is required for document operations")
}

// APIDocumentResourceInfo implements ResourceInfo for API documents
type APIDocumentResourceInfo struct {
	document *state.APIDocument
}

func (a *APIDocumentResourceInfo) GetID() string {
	return a.document.ID
}

func (a *APIDocumentResourceInfo) GetName() string {
	return a.document.Title
}

func (a *APIDocumentResourceInfo) GetLabels() map[string]string {
	// API documents don't support labels in the SDK
	return make(map[string]string)
}

func (a *APIDocumentResourceInfo) GetNormalizedLabels() map[string]string {
	return make(map[string]string)
}