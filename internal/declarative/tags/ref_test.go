package tags

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3" //nolint:gomodguard // yaml.v3 required for custom tag processing
)

func TestRefTagResolver_Tag(t *testing.T) {
	resolver := NewRefTagResolver(".")
	assert.Equal(t, "!ref", resolver.Tag())
}

func TestRefTagResolver_Resolve(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "ref with default field",
			input:    "getting-started-portal",
			expected: "__REF__:getting-started-portal#id",
		},
		{
			name:     "ref with explicit field",
			input:    "getting-started-portal#name",
			expected: "__REF__:getting-started-portal#name",
		},
		{
			name:     "ref with different field",
			input:    "my-auth-strategy#ref",
			expected: "__REF__:my-auth-strategy#ref",
		},
		{
			name:     "ref with hyphenated resource name",
			input:    "user-management-api#id",
			expected: "__REF__:user-management-api#id",
		},
		{
			name:     "ref with underscore resource name",
			input:    "control_plane_prod#name",
			expected: "__REF__:control_plane_prod#name",
		},
		{
			name:    "empty ref",
			input:   "",
			wantErr: true,
		},
		{
			name:    "empty field after hash",
			input:   "portal#",
			wantErr: true,
		},
		{
			name:    "only hash character",
			input:   "#",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewRefTagResolver(".")
			node := &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: tt.input,
			}

			result, err := resolver.Resolve(node)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "!ref tag")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestRefTagResolver_InvalidNodeKind(t *testing.T) {
	resolver := NewRefTagResolver(".")

	tests := []struct {
		name     string
		nodeKind yaml.Kind
	}{
		{"mapping node", yaml.MappingNode},
		{"sequence node", yaml.SequenceNode},
		{"alias node", yaml.AliasNode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &yaml.Node{
				Kind: tt.nodeKind,
			}

			_, err := resolver.Resolve(node)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "must be used with a string")
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
			input:    "__REF__:portal#id",
			expected: true,
		},
		{
			name:     "valid placeholder with name field",
			input:    "__REF__:auth-strategy#name",
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
			name:     "UUID string",
			input:    "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			expected: false,
		},
		{
			name:     "partial prefix",
			input:    "__REF",
			expected: false,
		},
		{
			name:     "prefix with different suffix",
			input:    "__REF__other",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRefPlaceholder(tt.input)
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
			name:      "valid placeholder with id field",
			input:     "__REF__:portal#id",
			wantRef:   "portal",
			wantField: "id",
			wantOk:    true,
		},
		{
			name:      "valid placeholder with name field",
			input:     "__REF__:auth-strategy#name",
			wantRef:   "auth-strategy",
			wantField: "name",
			wantOk:    true,
		},
		{
			name:      "valid placeholder with ref field",
			input:     "__REF__:control_plane_prod#ref",
			wantRef:   "control_plane_prod",
			wantField: "ref",
			wantOk:    true,
		},
		{
			name:   "not a placeholder",
			input:  "regular-string",
			wantOk: false,
		},
		{
			name:   "malformed placeholder - missing hash",
			input:  "__REF__:missing-hash",
			wantOk: false,
		},
		{
			name:   "malformed placeholder - empty",
			input:  "__REF__:",
			wantOk: false,
		},
		{
			name:   "empty string",
			input:  "",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, field, ok := ParseRefPlaceholder(tt.input)
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

func TestRefTagResolver_BaseDir(t *testing.T) {
	// Test that baseDir is stored correctly (though not used in current implementation)
	baseDir := "/some/path"
	resolver := NewRefTagResolver(baseDir)
	assert.Equal(t, baseDir, resolver.baseDir)
}

// TestRefTagResolver_Integration tests the resolver integrated with YAML processing
func TestRefTagResolver_Integration(t *testing.T) {
	resolver := NewRefTagResolver(".")

	// Test YAML processing with ref tags
	yamlContent := `
test_field: !ref portal-ref#name
`

	// Parse with yaml.v3
	var doc yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &doc)
	assert.NoError(t, err)

	// Find the ref tag node
	var refNode *yaml.Node
	if len(doc.Content) > 0 {
		// Look for mapping with test_field
		mapping := doc.Content[0]
		if mapping.Kind == yaml.MappingNode {
			for i := 0; i < len(mapping.Content); i += 2 {
				if i+1 < len(mapping.Content) {
					key := mapping.Content[i]
					value := mapping.Content[i+1]
					if key.Value == "test_field" && value.Tag == "!ref" {
						refNode = value
						break
					}
				}
			}
		}
	}

	assert.NotNil(t, refNode, "Should find ref tag node")
	assert.Equal(t, "!ref", refNode.Tag)
	assert.Equal(t, "portal-ref#name", refNode.Value)

	// Resolve the tag
	result, err := resolver.Resolve(refNode)
	assert.NoError(t, err)
	assert.Equal(t, "__REF__:portal-ref#name", result)
}
