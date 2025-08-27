package resources

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func TestAPIDocumentResource_SetDefaults(t *testing.T) {
	tests := []struct {
		name          string
		doc           *APIDocumentResource
		expectedSlug  string
		expectSlugSet bool
	}{
		{
			name: "generates slug from title when slug not provided",
			doc: &APIDocumentResource{
				CreateAPIDocumentRequest: kkComps.CreateAPIDocumentRequest{
					Title:   stringPtr("Main Document"),
					Content: "Content here",
				},
			},
			expectedSlug:  "main-document",
			expectSlugSet: true,
		},
		{
			name: "does not override existing slug",
			doc: &APIDocumentResource{
				CreateAPIDocumentRequest: kkComps.CreateAPIDocumentRequest{
					Title:   stringPtr("Main Document"),
					Slug:    stringPtr("custom-slug"),
					Content: "Content here",
				},
			},
			expectedSlug:  "custom-slug",
			expectSlugSet: true,
		},
		{
			name: "does not generate slug when title is nil",
			doc: &APIDocumentResource{
				CreateAPIDocumentRequest: kkComps.CreateAPIDocumentRequest{
					Content: "Content here",
				},
			},
			expectSlugSet: false,
		},
		{
			name: "does not generate slug when title is empty",
			doc: &APIDocumentResource{
				CreateAPIDocumentRequest: kkComps.CreateAPIDocumentRequest{
					Title:   stringPtr(""),
					Content: "Content here",
				},
			},
			expectSlugSet: false,
		},
		{
			name: "handles special characters in title",
			doc: &APIDocumentResource{
				CreateAPIDocumentRequest: kkComps.CreateAPIDocumentRequest{
					Title:   stringPtr("API v2.0 (Beta)"),
					Content: "Content here",
				},
			},
			expectedSlug:  "api-v20-beta", // Dots are removed per server implementation
			expectSlugSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.doc.SetDefaults()

			// Check status is always set to published
			if tt.doc.Status == nil || *tt.doc.Status != kkComps.APIDocumentStatusPublished {
				t.Error("expected status to be set to published")
			}

			// Check slug generation
			if tt.expectSlugSet {
				if tt.doc.Slug == nil {
					t.Error("expected slug to be set")
				} else if *tt.doc.Slug != tt.expectedSlug {
					t.Errorf("expected slug %q, got %q", tt.expectedSlug, *tt.doc.Slug)
				}
			} else {
				if tt.doc.Slug != nil {
					t.Errorf("expected slug to be nil, got %q", *tt.doc.Slug)
				}
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
