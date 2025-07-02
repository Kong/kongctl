package tags

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3" //nolint:gomodguard // yaml.v3 required for custom tag processing
)

// mockTagResolver is a mock implementation for testing
type mockTagResolver struct {
	tag          string
	resolveFunc  func(node *yaml.Node) (interface{}, error)
}

func (m *mockTagResolver) Tag() string {
	return m.tag
}

func (m *mockTagResolver) Resolve(node *yaml.Node) (interface{}, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(node)
	}
	return "resolved", nil
}

func TestResolverRegistry_Register(t *testing.T) {
	registry := NewResolverRegistry()
	
	// Initially no resolvers
	assert.False(t, registry.HasResolvers())
	
	// Register a resolver
	resolver := &mockTagResolver{tag: "!test"}
	registry.Register(resolver)
	
	assert.True(t, registry.HasResolvers())
}

func TestResolverRegistry_Process(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		resolver TagResolver
		expected string
		wantErr  bool
	}{
		{
			name:  "simple tag replacement",
			input: `value: !test simple`,
			resolver: &mockTagResolver{
				tag: "!test",
				resolveFunc: func(_ *yaml.Node) (interface{}, error) {
					return "replaced", nil
				},
			},
			expected: `value: replaced
`,
		},
		{
			name:  "nested structure",
			input: `
api:
  name: !test name
  version: "1.0"`,
			resolver: &mockTagResolver{
				tag: "!test",
				resolveFunc: func(_ *yaml.Node) (interface{}, error) {
					return "My API", nil
				},
			},
			expected: `api:
  name: My API
  version: "1.0"
`,
		},
		{
			name:  "multiple tags",
			input: `
name: !test name
description: !test desc`,
			resolver: &mockTagResolver{
				tag: "!test",
				resolveFunc: func(node *yaml.Node) (interface{}, error) {
					// Return different values based on node value
					if node.Value == "name" {
						return "Test Name", nil
					}
					return "Test Description", nil
				},
			},
			expected: `name: Test Name
description: Test Description
`,
		},
		{
			name:  "no tags",
			input: `
name: regular value
count: 42`,
			resolver: nil,
			expected: `name: regular value
count: 42
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewResolverRegistry()
			if tt.resolver != nil {
				registry.Register(tt.resolver)
			}

			result, err := registry.Process([]byte(tt.input))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, string(result))
			}
		})
	}
}

func TestResolverRegistry_ProcessNode(t *testing.T) {
	registry := NewResolverRegistry()
	
	// Register a test resolver
	resolver := &mockTagResolver{
		tag: "!test",
		resolveFunc: func(_ *yaml.Node) (interface{}, error) {
			return map[string]string{
				"key": "value",
			}, nil
		},
	}
	registry.Register(resolver)

	// Test processing a node with our tag
	input := `data: !test placeholder`
	
	var doc yaml.Node
	err := yaml.Unmarshal([]byte(input), &doc)
	assert.NoError(t, err)

	err = registry.processNode(&doc)
	assert.NoError(t, err)

	// Marshal back and check result
	output, err := yaml.Marshal(&doc)
	assert.NoError(t, err)
	assert.Contains(t, string(output), "key: value")
}

func TestResolverRegistry_InvalidYAML(t *testing.T) {
	registry := NewResolverRegistry()
	
	invalidYAML := `{this is not: valid yaml`
	_, err := registry.Process([]byte(invalidYAML))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML")
}