package hash

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/labels"
	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
)

func TestCalculatePortalHash(t *testing.T) {
	tests := []struct {
		name        string
		portal      kkInternalComps.CreatePortal
		wantErr     bool
		expectEqual bool // for comparing multiple hashes
		compare     *kkInternalComps.CreatePortal
	}{
		{
			name: "basic portal",
			portal: kkInternalComps.CreatePortal{
				Name:        "Test Portal",
				DisplayName: ptr("Test Display"),
			},
			wantErr: false,
		},
		{
			name: "portal with all fields",
			portal: kkInternalComps.CreatePortal{
				Name:                            "Full Portal",
				DisplayName:                     ptr("Full Display"),
				Description:                     ptr("Test description"),
				AuthenticationEnabled:           ptrBool(true),
				RbacEnabled:                    ptrBool(false),
				DefaultAPIVisibility:           (*kkInternalComps.DefaultAPIVisibility)(ptr("public")),
				DefaultPageVisibility:          (*kkInternalComps.DefaultPageVisibility)(ptr("private")),
				DefaultApplicationAuthStrategyID: ptr("auth-123"),
				AutoApproveDevelopers:          ptrBool(true),
				AutoApproveApplications:        ptrBool(false),
			},
			wantErr: false,
		},
		{
			name: "portal with user labels",
			portal: kkInternalComps.CreatePortal{
				Name: "Labeled Portal",
				Labels: map[string]*string{
					"env":   ptr("production"),
					"team":  ptr("platform"),
					"owner": ptr("john"),
				},
			},
			wantErr: false,
		},
		{
			name: "portal with KONGCTL labels - should be excluded",
			portal: kkInternalComps.CreatePortal{
				Name: "Managed Portal",
				Labels: map[string]*string{
					"env":                       ptr("production"),
					labels.ManagedKey:          ptr("true"),
					labels.ConfigHashKey:       ptr("old-hash"),
					labels.LastUpdatedKey:      ptr("2024-01-01"),
					"team":                     ptr("platform"),
				},
			},
			wantErr:     false,
			expectEqual: true,
			compare: &kkInternalComps.CreatePortal{
				Name: "Managed Portal",
				Labels: map[string]*string{
					"env":  ptr("production"),
					"team": ptr("platform"),
				},
			},
		},
		{
			name: "deterministic hash - same input",
			portal: kkInternalComps.CreatePortal{
				Name:        "Deterministic Test",
				DisplayName: ptr("Display"),
				Labels: map[string]*string{
					"z-label": ptr("last"),
					"a-label": ptr("first"),
					"m-label": ptr("middle"),
				},
			},
			wantErr:     false,
			expectEqual: true,
			compare: &kkInternalComps.CreatePortal{
				Name:        "Deterministic Test",
				DisplayName: ptr("Display"),
				Labels: map[string]*string{
					"a-label": ptr("first"),
					"m-label": ptr("middle"),
					"z-label": ptr("last"),
				},
			},
		},
		{
			name: "different values produce different hash",
			portal: kkInternalComps.CreatePortal{
				Name:        "Portal A",
				DisplayName: ptr("Display A"),
			},
			wantErr:     false,
			expectEqual: false,
			compare: &kkInternalComps.CreatePortal{
				Name:        "Portal B",
				DisplayName: ptr("Display A"),
			},
		},
		{
			name: "nil vs empty string treated differently",
			portal: kkInternalComps.CreatePortal{
				Name:        "Portal",
				Description: nil,
			},
			wantErr:     false,
			expectEqual: false,
			compare: &kkInternalComps.CreatePortal{
				Name:        "Portal",
				Description: ptr(""),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1, err := CalculatePortalHash(tt.portal)
			if (err != nil) != tt.wantErr {
				t.Errorf("CalculatePortalHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify hash is not empty
				if hash1 == "" {
					t.Error("CalculatePortalHash() returned empty hash")
				}

				// Verify deterministic behavior
				hash2, err := CalculatePortalHash(tt.portal)
				if err != nil {
					t.Errorf("CalculatePortalHash() second call error = %v", err)
				}
				if hash1 != hash2 {
					t.Error("CalculatePortalHash() not deterministic - different hashes for same input")
				}

				// Compare with another portal if specified
				if tt.compare != nil {
					hashCompare, err := CalculatePortalHash(*tt.compare)
					if err != nil {
						t.Errorf("CalculatePortalHash() compare error = %v", err)
					}

					if tt.expectEqual && hash1 != hashCompare {
						t.Errorf("Expected equal hashes but got different:\n  hash1: %s\n  hash2: %s", hash1, hashCompare)
					}
					if !tt.expectEqual && hash1 == hashCompare {
						t.Errorf("Expected different hashes but got same: %s", hash1)
					}
				}
			}
		})
	}
}

func TestComparePortalHash(t *testing.T) {
	// Create a portal and calculate its hash
	createPortal := kkInternalComps.CreatePortal{
		Name:                  "Test Portal",
		DisplayName:           ptr("Test Display"),
		Description:           ptr("Test description"),
		AuthenticationEnabled: ptrBool(true),
		RbacEnabled:          ptrBool(false),
		DefaultAPIVisibility: (*kkInternalComps.DefaultAPIVisibility)(ptr("public")),
		DefaultPageVisibility: (*kkInternalComps.DefaultPageVisibility)(ptr("public")),
		AutoApproveDevelopers: ptrBool(false),
		AutoApproveApplications: ptrBool(false),
		Labels: map[string]*string{
			"env": ptr("production"),
		},
	}

	expectedHash, err := CalculatePortalHash(createPortal)
	if err != nil {
		t.Fatalf("Failed to calculate expected hash: %v", err)
	}

	tests := []struct {
		name         string
		portal       kkInternalComps.PortalResponse
		expectedHash string
		wantMatch    bool
		wantErr      bool
	}{
		{
			name: "matching portal",
			portal: kkInternalComps.PortalResponse{
				ID:                      "portal-123",
				Name:                    "Test Portal",
				DisplayName:             "Test Display",
				Description:             ptr("Test description"),
				AuthenticationEnabled:   true,
				RbacEnabled:            false,
				DefaultAPIVisibility:   kkInternalComps.PortalResponseDefaultAPIVisibilityPublic,
				DefaultPageVisibility:  kkInternalComps.PortalResponseDefaultPageVisibilityPublic,
				AutoApproveDevelopers:   false,
				AutoApproveApplications: false,
				Labels: map[string]string{
					"env": "production",
				},
			},
			expectedHash: expectedHash,
			wantMatch:    true,
			wantErr:      false,
		},
		{
			name: "portal with KONGCTL labels still matches",
			portal: kkInternalComps.PortalResponse{
				ID:                      "portal-123",
				Name:                    "Test Portal",
				DisplayName:             "Test Display",
				Description:             ptr("Test description"),
				AuthenticationEnabled:   true,
				RbacEnabled:            false,
				DefaultAPIVisibility:   kkInternalComps.PortalResponseDefaultAPIVisibilityPublic,
				DefaultPageVisibility:  kkInternalComps.PortalResponseDefaultPageVisibilityPublic,
				AutoApproveDevelopers:   false,
				AutoApproveApplications: false,
				Labels: map[string]string{
					"env":                 "production",
					labels.ManagedKey:     "true",
					labels.ConfigHashKey:  expectedHash,
					labels.LastUpdatedKey: "2024-01-01",
				},
			},
			expectedHash: expectedHash,
			wantMatch:    true,
			wantErr:      false,
		},
		{
			name: "portal with different field",
			portal: kkInternalComps.PortalResponse{
				ID:                      "portal-123",
				Name:                    "Test Portal",
				DisplayName:             "Different Display", // Changed
				Description:             ptr("Test description"),
				AuthenticationEnabled:   true,
				RbacEnabled:            false,
				DefaultAPIVisibility:   kkInternalComps.PortalResponseDefaultAPIVisibilityPublic,
				DefaultPageVisibility:  kkInternalComps.PortalResponseDefaultPageVisibilityPublic,
				AutoApproveDevelopers:   false,
				AutoApproveApplications: false,
				Labels: map[string]string{
					"env": "production",
				},
			},
			expectedHash: expectedHash,
			wantMatch:    false,
			wantErr:      false,
		},
		{
			name: "portal with different label",
			portal: kkInternalComps.PortalResponse{
				ID:                      "portal-123",
				Name:                    "Test Portal",
				DisplayName:             "Test Display",
				Description:             ptr("Test description"),
				AuthenticationEnabled:   true,
				RbacEnabled:            false,
				DefaultAPIVisibility:   kkInternalComps.PortalResponseDefaultAPIVisibilityPublic,
				DefaultPageVisibility:  kkInternalComps.PortalResponseDefaultPageVisibilityPublic,
				AutoApproveDevelopers:   false,
				AutoApproveApplications: false,
				Labels: map[string]string{
					"env": "staging", // Changed
				},
			},
			expectedHash: expectedHash,
			wantMatch:    false,
			wantErr:      false,
		},
		{
			name: "portal with additional label",
			portal: kkInternalComps.PortalResponse{
				ID:                      "portal-123",
				Name:                    "Test Portal",
				DisplayName:             "Test Display",
				Description:             ptr("Test description"),
				AuthenticationEnabled:   true,
				RbacEnabled:            false,
				DefaultAPIVisibility:   kkInternalComps.PortalResponseDefaultAPIVisibilityPublic,
				DefaultPageVisibility:  kkInternalComps.PortalResponseDefaultPageVisibilityPublic,
				AutoApproveDevelopers:   false,
				AutoApproveApplications: false,
				Labels: map[string]string{
					"env":  "production",
					"team": "platform", // Additional
				},
			},
			expectedHash: expectedHash,
			wantMatch:    false,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := ComparePortalHash(tt.portal, tt.expectedHash)
			if (err != nil) != tt.wantErr {
				t.Errorf("ComparePortalHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if match != tt.wantMatch {
				t.Errorf("ComparePortalHash() = %v, want %v", match, tt.wantMatch)
			}
		})
	}
}

func TestHashConsistency(t *testing.T) {
	// Test that hashes are consistent across multiple runs
	portal := kkInternalComps.CreatePortal{
		Name:        "Consistency Test",
		DisplayName: ptr("Display Name"),
		Labels: map[string]*string{
			"env":     ptr("test"),
			"version": ptr("v1"),
		},
	}

	hashes := make([]string, 10)
	for i := 0; i < 10; i++ {
		hash, err := CalculatePortalHash(portal)
		if err != nil {
			t.Fatalf("Failed to calculate hash on iteration %d: %v", i, err)
		}
		hashes[i] = hash
	}

	// All hashes should be identical
	firstHash := hashes[0]
	for i, hash := range hashes {
		if hash != firstHash {
			t.Errorf("Hash inconsistency on iteration %d: expected %s, got %s", i, firstHash, hash)
		}
	}
}

// Helper functions
func ptr(s string) *string {
	return &s
}

func ptrBool(b bool) *bool {
	return &b
}