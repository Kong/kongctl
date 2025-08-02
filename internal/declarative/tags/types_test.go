package tags

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3" //nolint:gomodguard // yaml.v3 required for custom tag processing
)

func TestFileRef_YAML(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected FileRef
	}{
		{
			name: "path only",
			yaml: `path: ./specs/api.yaml`,
			expected: FileRef{
				Path: "./specs/api.yaml",
			},
		},
		{
			name: "path with extraction",
			yaml: `
path: ./specs/api.yaml
extract: info.title`,
			expected: FileRef{
				Path:    "./specs/api.yaml",
				Extract: "info.title",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ref FileRef
			err := yaml.Unmarshal([]byte(tt.yaml), &ref)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, ref)
		})
	}
}

func TestResolvedValue(t *testing.T) {
	rv := ResolvedValue{
		Value:  "test value",
		Source: "file://test.yaml",
	}
	
	assert.Equal(t, "test value", rv.Value)
	assert.Equal(t, "file://test.yaml", rv.Source)
}