package resources

import (
	"testing"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// TestAPIResource_Labels tests the new label behavior after removing custom UnmarshalJSON
func TestAPIResource_Labels(t *testing.T) {
	tests := []struct {
		name           string
		yamlContent    string
		expectNil      bool
		expectLength   int
	}{
		{
			name: "labels with values",
			yamlContent: `
ref: test-api
name: test-api
labels:
  foo: bar
  baz: qux
`,
			expectNil:    false,
			expectLength: 2,
		},
		{
			name: "empty labels object",
			yamlContent: `
ref: test-api
name: test-api
labels: {}
`,
			expectNil:    false,
			expectLength: 0,
		},
		{
			name: "labels null - now treated as nil",
			yamlContent: `
ref: test-api
name: test-api
labels: null
`,
			expectNil:    true,
			expectLength: 0,
		},
		{
			name: "labels with all values commented - treated as null/nil",
			yamlContent: `
ref: test-api
name: test-api
labels:
  #foo: bar
  #baz: qux
`,
			expectNil:    true,
			expectLength: 0,
		},
		{
			name: "no labels field",
			yamlContent: `
ref: test-api
name: test-api
`,
			expectNil:    true,
			expectLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var api APIResource
			err := yaml.Unmarshal([]byte(tt.yamlContent), &api)
			require.NoError(t, err)
			
			if tt.expectNil {
				assert.Nil(t, api.Labels)
			} else {
				assert.NotNil(t, api.Labels)
				assert.Len(t, api.Labels, tt.expectLength)
			}
		})
	}
}

// TestPortalResource_Labels tests the new label behavior for portals
func TestPortalResource_Labels(t *testing.T) {
	tests := []struct {
		name           string
		yamlContent    string
		expectNil      bool
		expectLength   int
	}{
		{
			name: "labels with values",
			yamlContent: `
ref: test-portal
name: test-portal
labels:
  foo: bar
  baz: qux
`,
			expectNil:    false,
			expectLength: 2,
		},
		{
			name: "empty labels object",
			yamlContent: `
ref: test-portal
name: test-portal
labels: {}
`,
			expectNil:    false,
			expectLength: 0,
		},
		{
			name: "labels null - now treated as nil",
			yamlContent: `
ref: test-portal
name: test-portal
labels: null
`,
			expectNil:    true,
			expectLength: 0,
		},
		{
			name: "labels with all values commented - treated as null/nil",
			yamlContent: `
ref: test-portal
name: test-portal
labels:
  #foo: bar
  #baz: qux
`,
			expectNil:    true,
			expectLength: 0,
		},
		{
			name: "no labels field",
			yamlContent: `
ref: test-portal
name: test-portal
`,
			expectNil:    true,
			expectLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var portal PortalResource
			err := yaml.Unmarshal([]byte(tt.yamlContent), &portal)
			require.NoError(t, err)
			
			if tt.expectNil {
				assert.Nil(t, portal.Labels)
			} else {
				assert.NotNil(t, portal.Labels)
				assert.Len(t, portal.Labels, tt.expectLength)
			}
		})
	}
}

// TestAuthStrategyResource_Labels tests the new label behavior for auth strategies
func TestAuthStrategyResource_Labels(t *testing.T) {
	tests := []struct {
		name           string
		yamlContent    string
		expectNil      bool
		expectLength   int
	}{
		{
			name: "labels with values",
			yamlContent: `
ref: test-auth
name: test-auth
display_name: Test Auth
strategy_type: key_auth
configs:
  key_auth:
    key_names:
      - X-API-Key
labels:
  foo: bar
  baz: qux
`,
			expectNil:    false,
			expectLength: 2,
		},
		{
			name: "empty labels object",
			yamlContent: `
ref: test-auth
name: test-auth
display_name: Test Auth
strategy_type: key_auth
configs:
  key_auth:
    key_names:
      - X-API-Key
labels: {}
`,
			expectNil:    false,
			expectLength: 0,
		},
		{
			name: "labels null - now treated as nil",
			yamlContent: `
ref: test-auth
name: test-auth
display_name: Test Auth
strategy_type: key_auth
configs:
  key_auth:
    key_names:
      - X-API-Key
labels: null
`,
			expectNil:    true,
			expectLength: 0,
		},
		{
			name: "labels with all values commented - treated as null/nil",
			yamlContent: `
ref: test-auth
name: test-auth
display_name: Test Auth
strategy_type: key_auth
configs:
  key_auth:
    key_names:
      - X-API-Key
labels:
  #foo: bar
  #baz: qux
`,
			expectNil:    true,
			expectLength: 0,
		},
		{
			name: "no labels field",
			yamlContent: `
ref: test-auth
name: test-auth
display_name: Test Auth
strategy_type: key_auth
configs:
  key_auth:
    key_names:
      - X-API-Key
`,
			expectNil:    true,
			expectLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var auth ApplicationAuthStrategyResource
			err := yaml.Unmarshal([]byte(tt.yamlContent), &auth)
			require.NoError(t, err)
			
			labels := auth.GetLabels()
			if tt.expectNil {
				assert.Nil(t, labels)
			} else {
				assert.NotNil(t, labels)
				assert.Len(t, labels, tt.expectLength)
			}
		})
	}
}