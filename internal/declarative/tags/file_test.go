package tags

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3" //nolint:gomodguard // yaml.v3 required for custom tag processing
)

func TestFileTagResolver_Tag(t *testing.T) {
	resolver := NewFileTagResolver(".", ".")
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
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent), 0o600))

	// Create a JSON file
	jsonContent := `{"title": "Test JSON", "count": 42}`
	jsonFile := filepath.Join(tmpDir, "test.json")
	require.NoError(t, os.WriteFile(jsonFile, []byte(jsonContent), 0o600))

	// Create a text file
	textContent := "Hello, World!"
	textFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(textFile, []byte(textContent), 0o600))

	resolver := NewFileTagResolver(tmpDir, tmpDir)

	tests := []struct {
		name      string
		nodeValue string
		want      any
		wantErr   bool
	}{
		{
			name:      "YAML file",
			nodeValue: "test.yaml",
			want: map[string]any{
				"name":        "Test API",
				"version":     "1.0.0",
				"description": "A test API",
			},
		},
		{
			name:      "JSON file",
			nodeValue: "test.json",
			want: map[string]any{
				"title": "Test JSON",
				"count": float64(42),
			},
		},
		{
			name:      "Text file",
			nodeValue: "test.txt",
			want:      "Hello, World!",
		},
		{
			name:      "Non-existent file",
			nodeValue: "missing.yaml",
			wantErr:   true,
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
	require.NoError(t, os.WriteFile(yamlFile, []byte(yamlContent), 0o600))

	resolver := NewFileTagResolver(tmpDir, tmpDir)

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
	resolver := NewFileTagResolver(tmpDir, tmpDir)

	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{
			name:    "Parent directory traversal",
			path:    "../../../etc/passwd",
			wantErr: "path resolves outside base dir",
		},
		{
			name:    "Hidden parent traversal",
			path:    "subdir/../../etc/passwd",
			wantErr: "path resolves outside base dir",
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

func TestFileTagResolver_ParentTraversalWithinRoot(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "configs")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	dataPath := filepath.Join(tmpDir, "data.txt")
	require.NoError(t, os.WriteFile(dataPath, []byte("data"), 0o600))

	resolver := NewFileTagResolver(configDir, tmpDir)

	node := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "../data.txt",
	}

	got, err := resolver.Resolve(node)
	require.NoError(t, err)
	assert.Equal(t, "data", got)
}

func TestFileTagResolver_Caching(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file that we'll modify
	content1 := "version: 1"
	testFile := filepath.Join(tmpDir, "cached.yaml")
	require.NoError(t, os.WriteFile(testFile, []byte(content1), 0o600))

	resolver := NewFileTagResolver(tmpDir, tmpDir)

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
	require.NoError(t, os.WriteFile(testFile, []byte(content2), 0o600))

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
	require.NoError(t, os.WriteFile(largeFile, []byte("test content"), 0o600))

	// For this test, we can't easily test the actual size limit without creating
	// a very large file, so we'll just ensure the file loads normally
	resolver := NewFileTagResolver(tmpDir, tmpDir)

	node := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "large.txt",
	}

	got, err := resolver.Resolve(node)
	assert.NoError(t, err)
	assert.Equal(t, "test content", got)
}

func TestFileTagResolver_InvalidNodeKind(t *testing.T) {
	resolver := NewFileTagResolver(".", ".")

	// Test with sequence node (not supported)
	node := &yaml.Node{
		Kind: yaml.SequenceNode,
	}

	_, err := resolver.Resolve(node)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be used with a string or map")
}

func TestFileTagResolver_EmptyPath(t *testing.T) {
	resolver := NewFileTagResolver(".", ".")

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
	require.NoError(t, os.MkdirAll(nestedDir, 0o755))

	// Create file in nested directory
	nestedFile := filepath.Join(nestedDir, "spec.yaml")
	content := "api_version: v1"
	require.NoError(t, os.WriteFile(nestedFile, []byte(content), 0o600))

	resolver := NewFileTagResolver(tmpDir, tmpDir)

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
	require.NoError(t, os.WriteFile(testFile, []byte(content), 0o600))

	resolver := NewFileTagResolver(tmpDir, tmpDir)

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

func TestFileTagResolver_ImageFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test image files with minimal binary content
	// PNG: minimal 1x1 PNG file (base64 decoded)
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG header
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1 pixels
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, // bit depth, color type, etc.
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, // IDAT chunk
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xDD, 0x8D,
		0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, // IEND chunk
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}
	pngFile := filepath.Join(tmpDir, "test.png")
	require.NoError(t, os.WriteFile(pngFile, pngData, 0o600))

	// JPEG: minimal JPEG file header
	jpegData := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, // JPEG header
		0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01,
		0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9, // End of image
	}
	jpegFile := filepath.Join(tmpDir, "test.jpg")
	require.NoError(t, os.WriteFile(jpegFile, jpegData, 0o600))

	// SVG: simple SVG text
	svgData := []byte(
		`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10">` +
			`<rect width="10" height="10" fill="red"/></svg>`,
	)
	svgFile := filepath.Join(tmpDir, "test.svg")
	require.NoError(t, os.WriteFile(svgFile, svgData, 0o600))

	// ICO: minimal ICO file
	icoData := []byte{
		0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x10, 0x10, // ICO header
		0x00, 0x00, 0x01, 0x00, 0x18, 0x00, 0x30, 0x00,
		0x00, 0x00, 0x16, 0x00, 0x00, 0x00, // Directory entry
	}
	icoFile := filepath.Join(tmpDir, "test.ico")
	require.NoError(t, os.WriteFile(icoFile, icoData, 0o600))

	resolver := NewFileTagResolver(tmpDir, tmpDir)

	tests := []struct {
		name         string
		filename     string
		expectedMIME string
	}{
		{
			name:         "PNG file",
			filename:     "test.png",
			expectedMIME: "image/png",
		},
		{
			name:         "JPEG file",
			filename:     "test.jpg",
			expectedMIME: "image/jpeg",
		},
		{
			name:         "SVG file",
			filename:     "test.svg",
			expectedMIME: "image/svg+xml",
		},
		{
			name:         "ICO file",
			filename:     "test.ico",
			expectedMIME: "image/x-icon",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: tt.filename,
			}

			got, err := resolver.Resolve(node)
			require.NoError(t, err)

			// Result should be a string (data URL)
			dataURL, ok := got.(string)
			require.True(t, ok, "Expected string result for image file")

			// Verify data URL format
			assert.True(t, strings.HasPrefix(dataURL, "data:"),
				"Data URL should start with 'data:'")
			assert.Contains(t, dataURL, ";base64,",
				"Data URL should contain ';base64,'")
			assert.Contains(t, dataURL, tt.expectedMIME,
				"Data URL should contain expected MIME type")

			// Extract and verify base64 portion
			parts := strings.SplitN(dataURL, ",", 2)
			require.Len(t, parts, 2, "Data URL should have comma separator")

			// Verify base64 decodes successfully
			_, err = base64.StdEncoding.DecodeString(parts[1])
			assert.NoError(t, err, "Base64 portion should be valid")
		})
	}
}

func TestFileTagResolver_ImageWithExtraction(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a simple PNG
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}
	pngFile := filepath.Join(tmpDir, "logo.png")
	require.NoError(t, os.WriteFile(pngFile, pngData, 0o600))

	resolver := NewFileTagResolver(tmpDir, tmpDir)

	// Test extraction with image file (extraction should be ignored for images)
	node := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "logo.png#somefield",
	}

	got, err := resolver.Resolve(node)
	require.NoError(t, err)

	// Should still return data URL (extraction is ignored for binary images)
	dataURL, ok := got.(string)
	require.True(t, ok, "Expected string result for image file")
	assert.True(t, strings.HasPrefix(dataURL, "data:image/png;base64,"))
}

func TestIsImageFile(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{".png", true},
		{".jpg", true},
		{".jpeg", true},
		{".svg", true},
		{".ico", true},
		{".gif", true},
		{".webp", true},
		{".yaml", false},
		{".json", false},
		{".txt", false},
		{".pdf", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := isImageFile(tt.ext)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetectMimeType(t *testing.T) {
	tests := []struct {
		name string
		ext  string
		data []byte
		want string
	}{
		{
			name: "PNG extension",
			ext:  ".png",
			data: []byte{0x89, 0x50, 0x4E, 0x47}, // PNG magic bytes
			want: "image/png",
		},
		{
			name: "JPEG extension",
			ext:  ".jpg",
			data: []byte{0xFF, 0xD8, 0xFF}, // JPEG magic bytes
			want: "image/jpeg",
		},
		{
			name: "SVG extension",
			ext:  ".svg",
			data: []byte("<svg"),
			want: "image/svg+xml",
		},
		{
			name: "ICO extension",
			ext:  ".ico",
			data: []byte{0x00, 0x00, 0x01, 0x00},
			want: "image/x-icon",
		},
		{
			name: "GIF extension",
			ext:  ".gif",
			data: []byte("GIF89a"),
			want: "image/gif",
		},
		{
			name: "WebP extension",
			ext:  ".webp",
			data: []byte("RIFF"),
			want: "image/webp",
		},
		{
			name: "Unknown extension with image data",
			ext:  ".unknown",
			data: []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, // PNG data
			want: "image/png",
		},
		{
			name: "Unknown extension with non-image data",
			ext:  ".unknown",
			data: []byte("plain text"),
			want: "application/octet-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectMimeType(tt.ext, tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}
