package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// APIDocumentAdapter implements ResourceOperations for API documents
type APIDocumentAdapter struct {
	client *state.Client
}

// NewAPIDocumentAdapter creates a new API document adapter
func NewAPIDocumentAdapter(client *state.Client) *APIDocumentAdapter {
	return &APIDocumentAdapter{client: client}
}

// MapCreateFields maps fields to CreateAPIDocumentRequest
func (a *APIDocumentAdapter) MapCreateFields(_ context.Context, fields map[string]interface{},
	create *kkComps.CreateAPIDocumentRequest) error {
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
func (a *APIDocumentAdapter) MapUpdateFields(_ context.Context, fields map[string]interface{},
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
func (a *APIDocumentAdapter) Delete(ctx context.Context, id string) error {
	// Get API ID from context
	apiID, err := a.getAPIID(ctx)
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

// getAPIID extracts the API ID from the context
func (a *APIDocumentAdapter) getAPIID(ctx context.Context) (string, error) {
	// Get the planned change from context
	change, ok := ctx.Value(contextKeyPlannedChange).(planner.PlannedChange)
	if !ok {
		return "", fmt.Errorf("planned change not found in context")
	}

	// Get API ID from references
	if apiRef, ok := change.References["api_id"]; ok {
		if apiRef.ID != "" {
			return apiRef.ID, nil
		}
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