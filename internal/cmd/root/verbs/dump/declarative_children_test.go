package dump

import "testing"

func TestNormalizePortalPageSlug(t *testing.T) {
	tests := []struct {
		name        string
		rawSlug     string
		parentPage  string
		wantSlug    string
		expectError bool
	}{
		{
			name:       "root slug",
			rawSlug:    "/",
			parentPage: "",
			wantSlug:   "/",
		},
		{
			name:        "root slug with parent",
			rawSlug:     "/",
			parentPage:  "parent-id",
			expectError: true,
		},
		{
			name:     "leading slash",
			rawSlug:  "/apis",
			wantSlug: "apis",
		},
		{
			name:     "trailing slash",
			rawSlug:  "apis/",
			wantSlug: "apis",
		},
		{
			name:     "no slashes",
			rawSlug:  "apis",
			wantSlug: "apis",
		},
		{
			name:     "trim whitespace",
			rawSlug:  "  /getting-started  ",
			wantSlug: "getting-started",
		},
		{
			name:        "multi segment with leading slash",
			rawSlug:     "/guides/publish-apis",
			expectError: true,
		},
		{
			name:        "multi segment without leading slash",
			rawSlug:     "guides/publish-apis",
			expectError: true,
		},
		{
			name:        "empty slug",
			rawSlug:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePortalPageSlug(tt.rawSlug, tt.parentPage)
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil (slug=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantSlug {
				t.Fatalf("expected slug %q, got %q", tt.wantSlug, got)
			}
		})
	}
}

func TestResolveAPIPublicationRef(t *testing.T) {
	apiID := "api-123"
	portalID := "portal-456"

	t.Run("uses publication id when provided", func(t *testing.T) {
		ref, err := resolveAPIPublicationRef(apiID, portalID, "pub-789")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref != "pub-789" {
			t.Fatalf("expected ref to be publication id, got %q", ref)
		}
	})

	t.Run("generates ref when id missing", func(t *testing.T) {
		ref, err := resolveAPIPublicationRef(apiID, portalID, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := buildChildRef("api-publication", apiID, portalID)
		if ref != expected {
			t.Fatalf("expected ref %q, got %q", expected, ref)
		}
	})

	t.Run("errors when portal id missing", func(t *testing.T) {
		if _, err := resolveAPIPublicationRef(apiID, "", ""); err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}
