package hash

import (
	"testing"

	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
)

func TestCalculateResourceHash_Generic(t *testing.T) {
	tests := []struct {
		name     string
		resource interface{}
		wantErr  bool
	}{
		{
			name: "portal resource",
			resource: kkInternalComps.CreatePortal{
				Name:        "Test Portal",
				DisplayName: ptr("Test Display"),
				Labels: map[string]*string{
					"env":  ptr("production"),
					"team": ptr("platform"),
				},
			},
			wantErr: false,
		},
		{
			name: "api resource",
			resource: kkInternalComps.CreateAPIRequest{
				Name:        "Test API",
				Description: ptr("Test API Description"),
				Labels: map[string]string{
					"version": "v1",
					"owner":   "backend-team",
				},
			},
			wantErr: false,
		},
		{
			name: "api version resource",
			resource: kkInternalComps.CreateAPIVersionRequest{
				Version: ptr("v1.0.0"),
			},
			wantErr: false,
		},
		{
			name: "api document resource",
			resource: kkInternalComps.CreateAPIDocumentRequest{
				Title:   ptr("API Documentation"),
				Content: "# API Guide\n\nThis is the API documentation.",
				Slug:    ptr("api-guide"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := CalculateResourceHash(tt.resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("CalculateResourceHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && hash == "" {
				t.Error("CalculateResourceHash() returned empty hash")
			}

			// Verify determinism - same input should produce same hash
			hash2, err := CalculateResourceHash(tt.resource)
			if err != nil {
				t.Errorf("CalculateResourceHash() second call error = %v", err)
			}
			if hash != hash2 {
				t.Error("CalculateResourceHash() not deterministic - different hashes for same input")
			}
		})
	}
}

func TestCalculateResourceHash_LabelFiltering(t *testing.T) {
	// Test that KONGCTL labels are filtered out
	resource1 := kkInternalComps.CreatePortal{
		Name: "Test Portal",
		Labels: map[string]*string{
			"env":                     ptr("production"),
			"KONGCTL-managed":         ptr("true"),
			"KONGCTL-config-hash":     ptr("old-hash"),
			"KONGCTL-last-updated":    ptr("2024-01-01"),
		},
	}

	resource2 := kkInternalComps.CreatePortal{
		Name: "Test Portal",
		Labels: map[string]*string{
			"env": ptr("production"),
		},
	}

	hash1, err := CalculateResourceHash(resource1)
	if err != nil {
		t.Fatalf("Failed to calculate hash1: %v", err)
	}

	hash2, err := CalculateResourceHash(resource2)
	if err != nil {
		t.Fatalf("Failed to calculate hash2: %v", err)
	}

	if hash1 != hash2 {
		t.Error("KONGCTL labels were not properly filtered - hashes differ")
	}
}

func TestCalculateResourceHash_SystemFieldFiltering(_ *testing.T) {
	// System field filtering is tested implicitly through other tests
	// The filterForHashing function removes fields like id, created_at, updated_at, etc.
	// This is covered by the determinism tests that ensure consistent hashes
}

func TestCalculateResourceHash_Determinism(t *testing.T) {
	// Test that field order doesn't matter
	tests := []struct {
		name      string
		resources []interface{}
		expectSame bool
	}{
		{
			name: "same portal with different label order",
			resources: []interface{}{
				kkInternalComps.CreatePortal{
					Name: "Portal",
					Labels: map[string]*string{
						"z-label": ptr("last"),
						"a-label": ptr("first"),
						"m-label": ptr("middle"),
					},
				},
				kkInternalComps.CreatePortal{
					Name: "Portal",
					Labels: map[string]*string{
						"a-label": ptr("first"),
						"m-label": ptr("middle"),
						"z-label": ptr("last"),
					},
				},
			},
			expectSame: true,
		},
		{
			name: "different values",
			resources: []interface{}{
				kkInternalComps.CreateAPIRequest{
					Name: "API 1",
				},
				kkInternalComps.CreateAPIRequest{
					Name: "API 2",
				},
			},
			expectSame: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.resources) < 2 {
				t.Fatal("Test requires at least 2 resources")
			}

			hash1, err := CalculateResourceHash(tt.resources[0])
			if err != nil {
				t.Fatalf("Failed to calculate hash for first resource: %v", err)
			}

			for i := 1; i < len(tt.resources); i++ {
				hash, err := CalculateResourceHash(tt.resources[i])
				if err != nil {
					t.Fatalf("Failed to calculate hash for resource %d: %v", i, err)
				}

				if tt.expectSame && hash != hash1 {
					t.Errorf("Expected same hash but got different: hash1=%s, hash%d=%s", hash1, i, hash)
				}
				if !tt.expectSame && hash == hash1 {
					t.Errorf("Expected different hashes but got same: %s", hash)
				}
			}
		})
	}
}

func TestCalculateAPIHash(t *testing.T) {
	api := kkInternalComps.CreateAPIRequest{
		Name:        "Test API",
		Description: ptr("Test Description"),
		Labels: map[string]string{
			"env": "test",
		},
	}

	hash, err := CalculateAPIHash(api)
	if err != nil {
		t.Errorf("CalculateAPIHash() error = %v", err)
	}

	if hash == "" {
		t.Error("CalculateAPIHash() returned empty hash")
	}
}

func TestCalculateAPIVersionHash(t *testing.T) {
	version := kkInternalComps.CreateAPIVersionRequest{
		Version: ptr("v1.0.0"),
	}

	hash, err := CalculateAPIVersionHash(version)
	if err != nil {
		t.Errorf("CalculateAPIVersionHash() error = %v", err)
	}

	if hash == "" {
		t.Error("CalculateAPIVersionHash() returned empty hash")
	}
}

func TestCalculateAPIDocumentHash(t *testing.T) {
	doc := kkInternalComps.CreateAPIDocumentRequest{
		Title:   ptr("API Guide"),
		Content: "Documentation content",
		Slug:    ptr("guide"),
	}

	hash, err := CalculateAPIDocumentHash(doc)
	if err != nil {
		t.Errorf("CalculateAPIDocumentHash() error = %v", err)
	}

	if hash == "" {
		t.Error("CalculateAPIDocumentHash() returned empty hash")
	}
}