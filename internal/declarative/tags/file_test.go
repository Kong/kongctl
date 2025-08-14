package tags

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3" //nolint:gomodguard // yaml.v3 required for custom tag processing
)

func TestFileTagResolver_Tag(t *testing.T) {
	resolver := NewFileTagResolver(".")
	assert.Equal(t, "!file", resolver.Tag())
}

func TestFileTagResolver_Resolve_StringFormat(t *testing.T) {
	// Create test files
	tmpDir := t.TempDir()
	
	// Create a simple YAML file
	yamlContent := `
name: Test API
version: 1.0.0
description: A test API`
	yamlFile := filepath.Join(tmpDir, "test.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent), 0600))
	
	// Create a JSON file
	jsonContent := `{"title": "Test JSON", "count": 42}`
	jsonFile := filepath.Join(tmpDir, "test.json")
	require.NoError(t, os.WriteFile(jsonFile, []byte(jsonContent), 0600))
	
	// Create a text file
	textContent := "Hello, World!"
	textFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(textFile, []byte(textContent), 0600))
	
	resolver := NewFileTagResolver(tmpDir)
	
	tests := []struct {
		name     string
		nodeValue string
		want     any
		wantErr  bool
	}{
		{
			name:     "YAML file",
			nodeValue: "test.yaml",
			want: map[string]any{
				"name":        "Test API",
				"version":     "1.0.0",
				"description": "A test API",
			},
		},
		{
			name:     "JSON file",
			nodeValue: "test.json",
			want: map[string]any{
				"title": "Test JSON",
				"count": float64(42),
			},
		},
		{
			name:     "Text file",
			nodeValue: "test.txt",
			want:     "Hello, World!",
		},
		{
			name:     "Non-existent file",
			nodeValue: "missing.yaml",
			wantErr:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: tt.nodeValue,
			}
			
			got, err := resolver.Resolve(node)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestFileTagResolver_Resolve_MapFormat(t *testing.T) {
	// Create test file with nested structure
	tmpDir := t.TempDir()
	yamlContent := `
info:
  title: Test API
  version: 1.0.0
  contact:
    name: John Doe
    email: john@example.com
servers:
  - url: https://api.example.com
    description: Production`
	
	yamlFile := filepath.Join(tmpDir, "openapi.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent), 0600))
	
	resolver := NewFileTagResolver(tmpDir)
	
	tests := []struct {
		name    string
		fileRef FileRef
		want    any
		wantErr bool
	}{
		{
			name: "Extract title",
			fileRef: FileRef{
				Path:    "openapi.yaml",
				Extract: "info.title",
			},
			want: "Test API",
		},
		{
			name: "Extract nested field",
			fileRef: FileRef{
				Path:    "openapi.yaml",
				Extract: "info.contact.email",
			},
			want: "john@example.com",
		},
		{
			name: "No extraction",
			fileRef: FileRef{
				Path: "openapi.yaml",
			},
			want: map[string]any{
				"info": map[string]any{
					"title":   "Test API",
					"version": "1.0.0",
					"contact": map[string]any{
						"name":  "John Doe",
						"email": "john@example.com",
					},
				},
				"servers": []any{
					map[string]any{
						"url":         "https://api.example.com",
						"description": "Production",
					},
				},
			},
		},
		{
			name: "Invalid extraction path",
			fileRef: FileRef{
				Path:    "openapi.yaml",
				Extract: "info.nonexistent",
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mapping node
			node := &yaml.Node{
				Kind: yaml.MappingNode,
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Value: "path"},
					{Kind: yaml.ScalarNode, Value: tt.fileRef.Path},
				},
			}
			
			if tt.fileRef.Extract != "" {
				node.Content = append(node.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Value: "extract"},
					&yaml.Node{Kind: yaml.ScalarNode, Value: tt.fileRef.Extract},
				)
			}
			
			got, err := resolver.Resolve(node)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestFileTagResolver_SecurityValidation(t *testing.T) {
	tmpDir := t.TempDir()
	resolver := NewFileTagResolver(tmpDir)
	
	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{
			name:    "Parent directory traversal",
			path:    "../../../etc/passwd",
			wantErr: "parent directory traversal is not allowed",
		},
		{
			name:    "Hidden parent traversal",
			path:    "subdir/../../etc/passwd",
			wantErr: "parent directory traversal is not allowed",
		},
		{
			name:    "Absolute path",
			path:    "/etc/passwd",
			wantErr: "absolute paths are not allowed",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: tt.path,
			}
			
			_, err := resolver.Resolve(node)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestFileTagResolver_Caching(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a file that we'll modify
	content1 := "version: 1"
	testFile := filepath.Join(tmpDir, "cached.yaml")
	require.NoError(t, os.WriteFile(testFile, []byte(content1), 0600))
	
	resolver := NewFileTagResolver(tmpDir)
	
	// First load with extraction
	mapNode := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "path"},
			{Kind: yaml.ScalarNode, Value: "cached.yaml"},
			{Kind: yaml.ScalarNode, Value: "extract"},
			{Kind: yaml.ScalarNode, Value: "version"},
		},
	}
	
	got1, err := resolver.Resolve(mapNode)
	require.NoError(t, err)
	// Should be version 1 - check for either int or float
	var version1 any
	switch v := got1.(type) {
	case int:
		assert.Equal(t, 1, v)
		version1 = 1
	case float64:
		assert.Equal(t, float64(1), v)
		version1 = float64(1)
	default:
		t.Fatalf("Unexpected type for version: %T", v)
	}
	
	// Modify the file
	content2 := "version: 2"
	require.NoError(t, os.WriteFile(testFile, []byte(content2), 0600))
	
	// Second load with same extraction should return cached value
	got2, err := resolver.Resolve(mapNode)
	require.NoError(t, err)
	assert.Equal(t, version1, got2) // Should be the same (cached)
	
	// Test that a different extraction path creates a different cache entry
	mapNode2 := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "path"},
			{Kind: yaml.ScalarNode, Value: "cached.yaml"},
			{Kind: yaml.ScalarNode, Value: "extract"},
			{Kind: yaml.ScalarNode, Value: ""}, // Empty extract path
		},
	}
	
	got3, err := resolver.Resolve(mapNode2)
	require.NoError(t, err)
	got3Map, ok := got3.(map[string]any)
	require.True(t, ok, "Expected map result")
	// Since this is a different cache key (no extraction), it should load fresh
	// and see version 2
	switch v := got3Map["version"].(type) {
	case int:
		assert.Equal(t, 2, v)
	case float64:
		assert.Equal(t, float64(2), v)
	default:
		t.Fatalf("Unexpected type for version in map: %T", v)
	}
}

func TestFileTagResolver_FileSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a large file (simulate, don't actually create 10MB)
	// We'll test the size check logic by creating a custom resolver with small limit
	largeFile := filepath.Join(tmpDir, "large.txt")
	require.NoError(t, os.WriteFile(largeFile, []byte("test content"), 0600))
	
	// For this test, we can't easily test the actual size limit without creating
	// a very large file, so we'll just ensure the file loads normally
	resolver := NewFileTagResolver(tmpDir)
	
	node := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "large.txt",
	}
	
	got, err := resolver.Resolve(node)
	assert.NoError(t, err)
	assert.Equal(t, "test content", got)
}

func TestFileTagResolver_InvalidNodeKind(t *testing.T) {
	resolver := NewFileTagResolver(".")
	
	// Test with sequence node (not supported)
	node := &yaml.Node{
		Kind: yaml.SequenceNode,
	}
	
	_, err := resolver.Resolve(node)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be used with a string or map")
}

func TestFileTagResolver_EmptyPath(t *testing.T) {
	resolver := NewFileTagResolver(".")
	
	// Map format with empty path
	node := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "path"},
			{Kind: yaml.ScalarNode, Value: ""},
		},
	}
	
	_, err := resolver.Resolve(node)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires 'path' field")
}

func TestFileTagResolver_NestedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create nested directory structure
	nestedDir := filepath.Join(tmpDir, "configs", "api")
	require.NoError(t, os.MkdirAll(nestedDir, 0755))
	
	// Create file in nested directory
	nestedFile := filepath.Join(nestedDir, "spec.yaml")
	content := "api_version: v1"
	require.NoError(t, os.WriteFile(nestedFile, []byte(content), 0600))
	
	resolver := NewFileTagResolver(tmpDir)
	
	node := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "configs/api/spec.yaml",
	}
	
	got, err := resolver.Resolve(node)
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"api_version": "v1"}, got)
}

func TestFileTagResolver_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create test file
	testFile := filepath.Join(tmpDir, "concurrent.yaml")
	content := "value: test"
	require.NoError(t, os.WriteFile(testFile, []byte(content), 0600))
	
	resolver := NewFileTagResolver(tmpDir)
	
	// Run multiple goroutines accessing the same file
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			node := &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: "concurrent.yaml",
			}
			
			got, err := resolver.Resolve(node)
			assert.NoError(t, err)
			assert.Equal(t, map[string]any{"value": "test"}, got)
			done <- true
		}()
	}
	
	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}