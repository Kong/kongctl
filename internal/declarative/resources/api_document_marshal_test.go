package resources

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func TestAPIDocumentResourceMarshalJSONIncludesMetadata(t *testing.T) {
	status := kkComps.APIDocumentStatusPublished
	title := "Doc Title"
	slug := "doc-slug"

	doc := APIDocumentResource{
		CreateAPIDocumentRequest: kkComps.CreateAPIDocumentRequest{
			Content: "content",
			Title:   &title,
			Slug:    &slug,
			Status:  &status,
		},
		Ref:               "doc-ref",
		API:               "api-ref",
		ParentDocumentRef: "parent-ref",
		Children: []APIDocumentResource{
			{
				CreateAPIDocumentRequest: kkComps.CreateAPIDocumentRequest{
					Content: "child content",
					Slug:    &slug,
				},
				Ref: "child-ref",
			},
		},
	}

	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if payload["ref"] != "doc-ref" {
		t.Fatalf("expected ref %q, got %v", "doc-ref", payload["ref"])
	}
	if payload["api"] != "api-ref" {
		t.Fatalf("expected api %q, got %v", "api-ref", payload["api"])
	}
	if payload["parent_document_ref"] != "parent-ref" {
		t.Fatalf("expected parent_document_ref %q, got %v", "parent-ref", payload["parent_document_ref"])
	}
	if payload["content"] != "content" {
		t.Fatalf("expected content %q, got %v", "content", payload["content"])
	}
	if payload["slug"] != "doc-slug" {
		t.Fatalf("expected slug %q, got %v", "doc-slug", payload["slug"])
	}

	children, ok := payload["children"].([]any)
	if !ok || len(children) != 1 {
		t.Fatalf("expected children to contain one entry, got %v", payload["children"])
	}
}
