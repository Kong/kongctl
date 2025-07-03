package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// createAPIDocument creates a new API document
func (e *Executor) createAPIDocument(ctx context.Context, change planner.PlannedChange) (string, error) {
	if e.client == nil {
		return "", fmt.Errorf("client not configured")
	}

	// Get parent API ID
	parentAPIID, err := e.getParentAPIID(ctx, change)
	if err != nil {
		return "", err
	}

	// Build request
	req := kkComps.CreateAPIDocumentRequest{}

	// Map fields to SDK request - Content is required
	if content, ok := change.Fields["content"].(string); ok {
		req.Content = content
	} else {
		return "", fmt.Errorf("content is required for API document")
	}

	if title, ok := change.Fields["title"].(string); ok {
		req.Title = &title
	}
	if slug, ok := change.Fields["slug"].(string); ok {
		req.Slug = &slug
	}
	if status, ok := change.Fields["status"].(string); ok {
		s := kkComps.APIDocumentStatus(status)
		req.Status = &s
	}
	if parentDocID, ok := change.Fields["parent_document_id"].(string); ok {
		req.ParentDocumentID = &parentDocID
	}

	// Create the document
	resp, err := e.client.CreateAPIDocument(ctx, parentAPIID, req)
	if err != nil {
		return "", fmt.Errorf("failed to create API document: %w", err)
	}

	return resp.ID, nil
}

// updateAPIDocument updates an existing API document
func (e *Executor) updateAPIDocument(ctx context.Context, change planner.PlannedChange) (string, error) {
	if e.client == nil {
		return "", fmt.Errorf("client not configured")
	}

	// Get parent API ID
	parentAPIID, err := e.getParentAPIID(ctx, change)
	if err != nil {
		return "", err
	}

	// Build request
	req := kkComps.APIDocument{}

	// Map fields to SDK request
	if content, ok := change.Fields["content"].(string); ok {
		req.Content = &content
	}
	if title, ok := change.Fields["title"].(string); ok {
		req.Title = &title
	}
	if slug, ok := change.Fields["slug"].(string); ok {
		req.Slug = &slug
	}
	if status, ok := change.Fields["status"].(string); ok {
		s := kkComps.APIDocumentStatus(status)
		req.Status = &s
	}
	if parentDocID, ok := change.Fields["parent_document_id"].(string); ok {
		req.ParentDocumentID = &parentDocID
	}

	// Update the document
	resp, err := e.client.UpdateAPIDocument(ctx, parentAPIID, change.ResourceID, req)
	if err != nil {
		return "", fmt.Errorf("failed to update API document: %w", err)
	}

	return resp.ID, nil
}

// deleteAPIDocument deletes an API document
func (e *Executor) deleteAPIDocument(ctx context.Context, change planner.PlannedChange) error {
	if e.client == nil {
		return fmt.Errorf("client not configured")
	}

	// Get parent API ID
	parentAPIID, err := e.getParentAPIID(ctx, change)
	if err != nil {
		return err
	}

	// Delete the document
	if err := e.client.DeleteAPIDocument(ctx, parentAPIID, change.ResourceID); err != nil {
		return fmt.Errorf("failed to delete API document: %w", err)
	}

	return nil
}