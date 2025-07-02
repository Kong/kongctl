package tags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractValue(t *testing.T) {
	// Test data structure
	testData := map[string]interface{}{
		"info": map[string]interface{}{
			"title":       "Test API",
			"version":     "1.0.0",
			"description": "A test API",
			"contact": map[string]interface{}{
				"name":  "Test User",
				"email": "test@example.com",
			},
		},
		"servers": []interface{}{
			map[string]interface{}{
				"url":         "https://api.example.com",
				"description": "Production server",
			},
		},
		"tags": []string{"tag1", "tag2", "tag3"},
	}

	tests := []struct {
		name    string
		data    interface{}
		path    string
		want    interface{}
		wantErr bool
	}{
		{
			name: "simple field",
			data: testData,
			path: "info.title",
			want: "Test API",
		},
		{
			name: "nested field",
			data: testData,
			path: "info.contact.email",
			want: "test@example.com",
		},
		{
			name: "empty path returns data",
			data: testData,
			path: "",
			want: testData,
		},
		{
			name:    "non-existent path",
			data:    testData,
			path:    "info.nonexistent",
			wantErr: true,
		},
		{
			name:    "invalid path through array",
			data:    testData,
			path:    "tags.invalid",
			wantErr: true,
		},
		{
			name: "struct field access",
			data: struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			}{
				Name:    "test",
				Version: "1.0",
			},
			path: "name",
			want: "test",
		},
		{
			name: "case insensitive struct field",
			data: struct {
				Name string
			}{
				Name: "test",
			},
			path: "name", // lowercase
			want: "test",
		},
		{
			name: "json tag field access",
			data: struct {
				FieldName string `json:"field_name"`
			}{
				FieldName: "value",
			},
			path: "field_name",
			want: "value",
		},
		{
			name: "pointer to value navigation",
			data: func() interface{} {
				data := map[string]interface{}{
					"field": "test",
				}
				return &data
			}(),
			path: "field",
			want: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractValue(tt.data, tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestGetAvailablePaths(t *testing.T) {
	testData := map[string]interface{}{
		"info": map[string]interface{}{
			"title":   "Test",
			"version": "1.0",
		},
		"tags": []string{"tag1", "tag2"},
	}

	paths := GetAvailablePaths(testData, "", 2)
	
	// Should contain top-level keys
	assert.Contains(t, paths, "info")
	assert.Contains(t, paths, "tags")
	
	// Should contain nested paths
	assert.Contains(t, paths, "info.title")
	assert.Contains(t, paths, "info.version")
	
	// Test with prefix
	paths = GetAvailablePaths(testData["info"], "info", 1)
	assert.Contains(t, paths, "info.title")
	assert.Contains(t, paths, "info.version")
	
	// Test max depth
	deepData := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": "value",
			},
		},
	}
	
	paths = GetAvailablePaths(deepData, "", 1)
	assert.Contains(t, paths, "a")
	assert.NotContains(t, paths, "a.b") // Limited by maxDepth
}

func TestGetAvailablePaths_Struct(t *testing.T) {
	type TestStruct struct {
		Name        string `json:"name"`
		Version     string `yaml:"ver"`
		Description string
		private     string // Should be skipped
	}
	
	data := TestStruct{
		Name:        "test",
		Version:     "1.0",
		Description: "desc",
		private:     "hidden",
	}
	
	paths := GetAvailablePaths(data, "", 1)
	
	// Should use JSON tag for Name
	assert.Contains(t, paths, "name")
	// Should use YAML tag for Version
	assert.Contains(t, paths, "ver")
	// Should use field name for Description
	assert.Contains(t, paths, "Description")
	// Should not contain private field
	assert.NotContains(t, paths, "private")
}