package validator

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
)

func TestValidateNamespace(t *testing.T) {
	validator := NewNamespaceValidator()

	tests := []struct {
		name      string
		namespace string
		wantErr   bool
		errMsg    string
	}{
		// Valid cases
		{
			name:      "simple lowercase",
			namespace: "default",
			wantErr:   false,
		},
		{
			name:      "with hyphens",
			namespace: "my-team-namespace",
			wantErr:   false,
		},
		{
			name:      "with numbers",
			namespace: "team123",
			wantErr:   false,
		},
		{
			name:      "single character",
			namespace: "a",
			wantErr:   false,
		},
		{
			name:      "max length",
			namespace: strings.Repeat("a", 63),
			wantErr:   false,
		},
		{
			name:      "starts with number",
			namespace: "123team",
			wantErr:   false,
		},
		{
			name:      "complex valid",
			namespace: "team-123-prod-v2",
			wantErr:   false,
		},

		// Invalid cases
		{
			name:      "empty string",
			namespace: "",
			wantErr:   true,
			errMsg:    "namespace cannot be empty",
		},
		{
			name:      "uppercase letters",
			namespace: "MyNamespace",
			wantErr:   true,
			errMsg:    "must consist of lowercase alphanumeric",
		},
		{
			name:      "starts with hyphen",
			namespace: "-namespace",
			wantErr:   true,
			errMsg:    "must start and end with an alphanumeric character",
		},
		{
			name:      "ends with hyphen",
			namespace: "namespace-",
			wantErr:   true,
			errMsg:    "must start and end with an alphanumeric character",
		},
		{
			name:      "contains underscore",
			namespace: "my_namespace",
			wantErr:   true,
			errMsg:    "must consist of lowercase alphanumeric",
		},
		{
			name:      "contains space",
			namespace: "my namespace",
			wantErr:   true,
			errMsg:    "must consist of lowercase alphanumeric",
		},
		{
			name:      "contains special chars",
			namespace: "my.namespace",
			wantErr:   true,
			errMsg:    "must consist of lowercase alphanumeric",
		},
		{
			name:      "double hyphen",
			namespace: "my--namespace",
			wantErr:   true,
			errMsg:    "cannot contain consecutive hyphens",
		},
		{
			name:      "exceeds max length",
			namespace: strings.Repeat("a", 64),
			wantErr:   true,
			errMsg:    "exceeds maximum length of 63 characters",
		},
		{
			name:      "just hyphens",
			namespace: "---",
			wantErr:   true,
			errMsg:    "must start and end with an alphanumeric character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateNamespace(tt.namespace)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateNamespace() expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateNamespace() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateNamespace() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateNamespaces(t *testing.T) {
	validator := NewNamespaceValidator()

	tests := []struct {
		name       string
		namespaces []string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "all valid",
			namespaces: []string{"default", "team-a", "team-b", "platform"},
			wantErr:    false,
		},
		{
			name:       "with duplicates (allowed)",
			namespaces: []string{"default", "team-a", "default", "team-a"},
			wantErr:    false,
		},
		{
			name:       "one invalid",
			namespaces: []string{"default", "INVALID", "team-a"},
			wantErr:    true,
			errMsg:     "must consist of lowercase alphanumeric",
		},
		{
			name:       "empty list",
			namespaces: []string{},
			wantErr:    false,
		},
		{
			name:       "single namespace",
			namespaces: []string{"default"},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateNamespaces(tt.namespaces)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateNamespaces() expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateNamespaces() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateNamespaces() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestReservedNamespaces(t *testing.T) {
	// Test that reserved namespaces structure is properly initialized
	if ReservedNamespaces == nil {
		t.Error("ReservedNamespaces should not be nil")
	}

	// Currently no reserved namespaces, but test the mechanism
	originalReserved := ReservedNamespaces
	defer func() {
		ReservedNamespaces = originalReserved
	}()

	// Add a test reserved namespace
	ReservedNamespaces = map[string]bool{
		"system": true,
		"kube":   true,
	}

	validator := NewNamespaceValidator()

	tests := []struct {
		namespace string
		wantErr   bool
	}{
		{"system", true},
		{"kube", true},
		{"user", false},
		{"default", false},
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			err := validator.ValidateNamespace(tt.namespace)
			if tt.wantErr && err == nil {
				t.Errorf("Expected error for reserved namespace %s", tt.namespace)
			} else if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error for namespace %s: %v", tt.namespace, err)
			}
		})
	}
}

func TestParseNamespaceRequirementSlice(t *testing.T) {
	validator := NewNamespaceValidator()
	tests := []struct {
		name    string
		input   []string
		expect  NamespaceRequirement
		wantErr bool
	}{
		{
			name:   "empty slice - any namespace required",
			input:  []string{},
			expect: NamespaceRequirement{Mode: NamespaceRequirementAny, AllowedNamespaces: []string{}},
		},
		{
			name:   "single namespace",
			input:  []string{"foo"},
			expect: NamespaceRequirement{Mode: NamespaceRequirementSpecific, AllowedNamespaces: []string{"foo"}},
		},
		{
			name:  "multiple namespaces",
			input: []string{"foo", "bar", "baz"},
			expect: NamespaceRequirement{
				Mode:              NamespaceRequirementSpecific,
				AllowedNamespaces: []string{"foo", "bar", "baz"},
			},
		},
		{
			name:   "duplicate namespaces",
			input:  []string{"foo", "bar", "foo"},
			expect: NamespaceRequirement{Mode: NamespaceRequirementSpecific, AllowedNamespaces: []string{"foo", "bar"}},
		},
		{
			name:   "with empty strings",
			input:  []string{"foo", "", "bar"},
			expect: NamespaceRequirement{Mode: NamespaceRequirementSpecific, AllowedNamespaces: []string{"foo", "bar"}},
		},
		{
			name:    "invalid namespace",
			input:   []string{"foo", "Invalid!"},
			wantErr: true,
		},
		{
			name:   "all empty strings treated as any",
			input:  []string{"", "", ""},
			expect: NamespaceRequirement{Mode: NamespaceRequirementAny, AllowedNamespaces: []string{}},
		},
		{
			name:    "flag-like value detected",
			input:   []string{"--profile"},
			wantErr: true,
		},
		{
			name:    "short flag detected",
			input:   []string{"-p"},
			wantErr: true,
		},
		{
			name:    "mixed with valid namespace",
			input:   []string{"foo", "--profile"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := validator.ParseNamespaceRequirementSlice(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				// Check for helpful error message when flag is detected
				for _, val := range tt.input {
					if strings.HasPrefix(val, "-") {
						if !strings.Contains(err.Error(), "looks like a flag") {
							t.Fatalf("expected error message to mention flag detection, got: %v", err)
						}
						if !strings.Contains(err.Error(), "--require-any-namespace") {
							t.Fatalf("expected error message to suggest --require-any-namespace, got: %v", err)
						}
						break
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if req.Mode != tt.expect.Mode {
				t.Fatalf("mode mismatch: expected %v, got %v", tt.expect.Mode, req.Mode)
			}
			if len(req.AllowedNamespaces) != len(tt.expect.AllowedNamespaces) {
				t.Fatalf("allowed namespaces count mismatch: expected %d, got %d",
					len(tt.expect.AllowedNamespaces), len(req.AllowedNamespaces))
			}
			for i, ns := range req.AllowedNamespaces {
				if ns != tt.expect.AllowedNamespaces[i] {
					t.Fatalf("allowed namespace[%d] mismatch: expected %q, got %q",
						i, tt.expect.AllowedNamespaces[i], ns)
				}
			}
		})
	}
}

func TestParseNamespaceRequirement(t *testing.T) {
	validator := NewNamespaceValidator()

	tests := []struct {
		name    string
		input   string
		expect  NamespaceRequirement
		wantErr bool
	}{
		{
			name:   "empty",
			input:  "",
			expect: NamespaceRequirement{Mode: NamespaceRequirementNone},
		},
		{
			name:   "true",
			input:  "true",
			expect: NamespaceRequirement{Mode: NamespaceRequirementAny},
		},
		{
			name:   "any keyword",
			input:  "any",
			expect: NamespaceRequirement{Mode: NamespaceRequirementAny},
		},
		{
			name:   "specific namespace",
			input:  "team-alpha",
			expect: NamespaceRequirement{Mode: NamespaceRequirementSpecific, AllowedNamespaces: []string{"team-alpha"}},
		},
		{
			name:    "invalid namespace",
			input:   "Team!",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := validator.ParseNamespaceRequirement(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if req.Mode != tt.expect.Mode {
				t.Fatalf("mode mismatch: expected %v, got %v", tt.expect.Mode, req.Mode)
			}
			if len(req.AllowedNamespaces) != len(tt.expect.AllowedNamespaces) {
				t.Fatalf("allowed namespaces count mismatch: expected %d, got %d",
					len(tt.expect.AllowedNamespaces), len(req.AllowedNamespaces))
			}
			for i, ns := range req.AllowedNamespaces {
				if ns != tt.expect.AllowedNamespaces[i] {
					t.Fatalf("allowed namespace[%d] mismatch: expected %q, got %q",
						i, tt.expect.AllowedNamespaces[i], ns)
				}
			}
		})
	}
}

func TestValidateNamespaceRequirementAny(t *testing.T) {
	validator := NewNamespaceValidator()
	requirement := NamespaceRequirement{Mode: NamespaceRequirementAny}

	stringPtr := func(s string) *string { return &s }

	t.Run("passes with explicit and file default namespaces", func(t *testing.T) {
		rs := resources.ResourceSet{
			APIs: []resources.APIResource{
				{
					BaseResource: resources.BaseResource{
						Ref: "api-explicit",
						Kongctl: &resources.KongctlMeta{
							Namespace:       stringPtr("team"),
							NamespaceOrigin: resources.NamespaceOriginExplicit,
						},
					},
				},
				{
					BaseResource: resources.BaseResource{
						Ref: "api-default",
						Kongctl: &resources.KongctlMeta{
							Namespace:       stringPtr("team"),
							NamespaceOrigin: resources.NamespaceOriginFileDefault,
						},
					},
				},
			},
		}
		if err := validator.ValidateNamespaceRequirement(&rs, requirement); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails when resource relies on implicit default", func(t *testing.T) {
		rs := resources.ResourceSet{
			Portals: []resources.PortalResource{
				{
					BaseResource: resources.BaseResource{
						Ref: "portal-implicit",
						Kongctl: &resources.KongctlMeta{
							Namespace:       stringPtr("default"),
							NamespaceOrigin: resources.NamespaceOriginImplicitDefault,
						},
					},
				},
			},
		}
		if err := validator.ValidateNamespaceRequirement(&rs, requirement); err == nil {
			t.Fatalf("expected error but got nil")
		}
	})

	t.Run("empty configuration without defaults errors", func(t *testing.T) {
		rs := resources.ResourceSet{}
		if err := validator.ValidateNamespaceRequirement(&rs, requirement); err == nil {
			t.Fatalf("expected error but got nil")
		}
	})

	t.Run("empty configuration with defaults passes", func(t *testing.T) {
		rs := resources.ResourceSet{DefaultNamespaces: []string{"team"}, DefaultNamespace: "team"}
		if err := validator.ValidateNamespaceRequirement(&rs, requirement); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestValidateNamespaceRequirementSpecific(t *testing.T) {
	validator := NewNamespaceValidator()
	requirement := NamespaceRequirement{Mode: NamespaceRequirementSpecific, AllowedNamespaces: []string{"team"}}
	stringPtr := func(s string) *string { return &s }

	t.Run("passes when all resources match namespace", func(t *testing.T) {
		rs := resources.ResourceSet{
			APIs: []resources.APIResource{
				{
					BaseResource: resources.BaseResource{
						Ref: "api-explicit",
						Kongctl: &resources.KongctlMeta{
							Namespace:       stringPtr("team"),
							NamespaceOrigin: resources.NamespaceOriginExplicit,
						},
					},
				},
			},
			ControlPlanes: []resources.ControlPlaneResource{
				{
					BaseResource: resources.BaseResource{

						Ref: "cp-default",
						Kongctl: &resources.KongctlMeta{
							Namespace:       stringPtr("team"),
							NamespaceOrigin: resources.NamespaceOriginFileDefault,
						},
					},
				},
			},
		}
		if err := validator.ValidateNamespaceRequirement(&rs, requirement); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails when namespace mismatches", func(t *testing.T) {
		rs := resources.ResourceSet{
			APIs: []resources.APIResource{
				{
					BaseResource: resources.BaseResource{
						Ref: "api-wrong",
						Kongctl: &resources.KongctlMeta{
							Namespace:       stringPtr("other"),
							NamespaceOrigin: resources.NamespaceOriginExplicit,
						},
					},
				},
			},
		}
		if err := validator.ValidateNamespaceRequirement(&rs, requirement); err == nil {
			t.Fatalf("expected error but got nil")
		}
	})

	t.Run("fails when relying on implicit default", func(t *testing.T) {
		rs := resources.ResourceSet{
			ApplicationAuthStrategies: []resources.ApplicationAuthStrategyResource{
				{
					BaseResource: resources.BaseResource{
						Ref: "auth-implicit",
						Kongctl: &resources.KongctlMeta{
							Namespace:       stringPtr("default"),
							NamespaceOrigin: resources.NamespaceOriginImplicitDefault,
						},
					},
				},
			},
		}
		if err := validator.ValidateNamespaceRequirement(&rs, requirement); err == nil {
			t.Fatalf("expected error but got nil")
		}
	})

	t.Run("empty configuration with matching default passes", func(t *testing.T) {
		rs := resources.ResourceSet{DefaultNamespaces: []string{"team"}, DefaultNamespace: "team"}
		if err := validator.ValidateNamespaceRequirement(&rs, requirement); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty configuration with mismatched default errors", func(t *testing.T) {
		rs := resources.ResourceSet{DefaultNamespaces: []string{"other"}, DefaultNamespace: "other"}
		if err := validator.ValidateNamespaceRequirement(&rs, requirement); err == nil {
			t.Fatalf("expected error but got nil")
		}
	})
}
