package loader

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLocalFieldResolver(t *testing.T) {
	t.Run("with logger", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		resolver := NewLocalFieldResolver(logger)

		assert.NotNil(t, resolver)
		assert.Equal(t, logger, resolver.logger)
	})

	t.Run("with nil logger", func(t *testing.T) {
		resolver := NewLocalFieldResolver(nil)

		assert.NotNil(t, resolver)
		assert.NotNil(t, resolver.logger)
	})
}

func TestLocalFieldResolver_CanResolve(t *testing.T) {
	resolver := NewLocalFieldResolver(nil)

	// Local resolver should handle all types
	testCases := []string{
		"portal",
		"api",
		"application_auth_strategy",
		"unknown_type",
	}

	for _, resourceType := range testCases {
		t.Run(resourceType, func(t *testing.T) {
			assert.True(t, resolver.CanResolve(resourceType))
		})
	}
}

func TestLocalFieldResolver_ResolveField(t *testing.T) {
	// Create test resources with minimal fields we know exist
	portal := resources.PortalResource{
		Ref: "test-portal",
	}
	portal.Name = stringPtr("Test Portal")

	api := resources.APIResource{
		Ref: "test-api",
	}
	api.Name = stringPtr("Test API")

	resolver := NewLocalFieldResolver(nil)

	tests := []struct {
		name     string
		resource resources.Resource
		field    string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple field - ref",
			resource: &portal,
			field:    "Ref",
			expected: "test-portal",
		},
		{
			name:     "embedded field - name",
			resource: &portal,
			field:    "Name",
			expected: "Test Portal",
		},
		{
			name:     "api ref field",
			resource: &api,
			field:    "Ref",
			expected: "test-api",
		},
		{
			name:     "api name field",
			resource: &api,
			field:    "Name",
			expected: "Test API",
		},
		{
			name:     "non-existent field",
			resource: &portal,
			field:    "NonExistent",
			wantErr:  true,
		},
		{
			name:     "empty field",
			resource: &portal,
			field:    "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.ResolveField(tt.resource, tt.field)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func createPortal(ref, name string) resources.PortalResource {
	p := resources.PortalResource{Ref: ref}
	p.Name = stringPtr(name)
	return p
}

func createAPI(ref, name string) resources.APIResource {
	a := resources.APIResource{Ref: ref}
	a.Name = stringPtr(name)
	return a
}

func TestResolveReferences_Basic(t *testing.T) {
	// Create test logger for context
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: log.LevelTrace}))
	ctx := context.WithValue(context.Background(), log.LoggerKey, logger)

	// Test basic reference resolution using Description field (which is *string)
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			createPortal("my-portal", "My Portal"),
		},
		APIs: []resources.APIResource{
			createAPI("my-api", "My API"),
		},
	}

	// Set up reference placeholder in Description field
	placeholder := tags.RefPlaceholderPrefix + "my-portal#Name"
	rs.APIs[0].Description = &placeholder

	// Execute reference resolution
	err := ResolveReferences(ctx, rs)
	require.NoError(t, err)

	// Validate the reference was resolved
	assert.NotNil(t, rs.APIs[0].Description)
	assert.Equal(t, "My Portal", *rs.APIs[0].Description)
}

func TestResolveReferences_WithoutContext(t *testing.T) {
	// Test with context.Background() (no logger in context)
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			createPortal("test-portal", "Test Portal"),
		},
		APIs: []resources.APIResource{
			createAPI("test-api", "Test API"),
		},
	}

	placeholder := tags.RefPlaceholderPrefix + "test-portal#Name"
	rs.APIs[0].Description = &placeholder

	err := ResolveReferences(context.Background(), rs)
	assert.NoError(t, err)
	assert.NotNil(t, rs.APIs[0].Description)
	assert.Equal(t, "Test Portal", *rs.APIs[0].Description)
}

func TestResolveReferences_ErrorCases(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.WithValue(context.Background(), log.LoggerKey, logger)

	tests := []struct {
		name        string
		resourceSet *resources.ResourceSet
		setupRefs   func(*resources.ResourceSet)
		wantErr     bool
	}{
		{
			name: "reference to non-existent resource",
			resourceSet: &resources.ResourceSet{
				APIs: []resources.APIResource{
					createAPI("my-api", "My API"),
				},
			},
			setupRefs: func(rs *resources.ResourceSet) {
				placeholder := tags.RefPlaceholderPrefix + "non-existent#Name"
				rs.APIs[0].Description = &placeholder
			},
			wantErr: true,
		},
		{
			name: "reference to non-existent field (deferred to planning)",
			resourceSet: &resources.ResourceSet{
				Portals: []resources.PortalResource{
					createPortal("my-portal", "My Portal"),
				},
				APIs: []resources.APIResource{
					createAPI("my-api", "My API"),
				},
			},
			setupRefs: func(rs *resources.ResourceSet) {
				placeholder := tags.RefPlaceholderPrefix + "my-portal#NonExistentField"
				rs.APIs[0].Description = &placeholder
			},
			wantErr: false, // Now deferred to planning phase
		},
		{
			name: "invalid placeholder format",
			resourceSet: &resources.ResourceSet{
				APIs: []resources.APIResource{
					createAPI("my-api", "My API"),
				},
			},
			setupRefs: func(rs *resources.ResourceSet) {
				placeholder := "__REF__:invalid-format-no-hash"
				rs.APIs[0].Description = &placeholder
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test data
			if tt.setupRefs != nil {
				tt.setupRefs(tt.resourceSet)
			}

			// Execute reference resolution
			err := ResolveReferences(ctx, tt.resourceSet)

			// Check results
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsRefPlaceholder(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid placeholder",
			input:    tags.RefPlaceholderPrefix + "portal#id",
			expected: true,
		},
		{
			name:     "regular string",
			input:    "regular-string",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "partial prefix",
			input:    "__REF",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tags.IsRefPlaceholder(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseRefPlaceholder(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantRef   string
		wantField string
		wantOk    bool
	}{
		{
			name:      "valid placeholder",
			input:     tags.RefPlaceholderPrefix + "portal#id",
			wantRef:   "portal",
			wantField: "id",
			wantOk:    true,
		},
		{
			name:      "valid placeholder with name field",
			input:     tags.RefPlaceholderPrefix + "auth-strategy#name",
			wantRef:   "auth-strategy",
			wantField: "name",
			wantOk:    true,
		},
		{
			name:   "not a placeholder",
			input:  "regular-string",
			wantOk: false,
		},
		{
			name:   "malformed - missing hash",
			input:  tags.RefPlaceholderPrefix + "missing-hash",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, field, ok := tags.ParseRefPlaceholder(tt.input)
			assert.Equal(t, tt.wantOk, ok)
			if tt.wantOk {
				assert.Equal(t, tt.wantRef, ref)
				assert.Equal(t, tt.wantField, field)
			} else {
				assert.Empty(t, ref)
				assert.Empty(t, field)
			}
		})
	}
}

// Integration test with actual YAML tag processing
func TestIntegrationWithTagProcessing(t *testing.T) {
	// This test simulates the full flow: YAML -> tag processing -> reference resolution
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.WithValue(context.Background(), log.LoggerKey, logger)

	// Create ResourceSet as it would appear after YAML parsing and tag processing
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			createPortal("my-portal", "My Portal Name"),
		},
		APIs: []resources.APIResource{
			createAPI("my-api", "My API"),
		},
	}

	// This would be set by RefTagResolver.Resolve()
	placeholder := "__REF__:my-portal#Name"
	rs.APIs[0].Description = &placeholder

	// Run reference resolution
	err := ResolveReferences(ctx, rs)
	require.NoError(t, err)

	// Validate the reference was resolved
	assert.NotNil(t, rs.APIs[0].Description)
	assert.Equal(t, "My Portal Name", *rs.APIs[0].Description)

	// Verify original resource is unchanged
	assert.Equal(t, "My Portal Name", rs.Portals[0].Name)
}

// Test logging output (manual verification)
func TestLoggingOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping logging test in short mode")
	}

	// Create a string builder to capture log output
	var logOutput strings.Builder
	logger := slog.New(slog.NewTextHandler(&logOutput, &slog.HandlerOptions{
		Level: log.LevelTrace,
	}))
	ctx := context.WithValue(context.Background(), log.LoggerKey, logger)

	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			createPortal("test-portal", "Test Portal"),
		},
		APIs: []resources.APIResource{
			createAPI("test-api", "Test API"),
		},
	}

	placeholder := tags.RefPlaceholderPrefix + "test-portal#Name"
	rs.APIs[0].Description = &placeholder

	err := ResolveReferences(ctx, rs)
	require.NoError(t, err)

	// Verify some expected log messages are present
	logStr := logOutput.String()
	assert.Contains(t, logStr, "Starting reference resolution")
	assert.Contains(t, logStr, "Found reference placeholder")
	assert.Contains(t, logStr, "Reference resolved")
	assert.Contains(t, logStr, "Reference resolution completed")

	// Verify resolution worked
	assert.NotNil(t, rs.APIs[0].Description)
	assert.Equal(t, "Test Portal", *rs.APIs[0].Description)
}
