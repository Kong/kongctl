package tags

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3" //nolint:gomodguard // yaml.v3 required for custom tag processing
	k8syaml "sigs.k8s.io/yaml" // For JSON support
)

const (
	// MaxFileSize is the maximum file size we'll load (10MB)
	MaxFileSize = 10 * 1024 * 1024
)

// FileTagResolver handles !file tags for loading external content
type FileTagResolver struct {
	baseDir string
	cache   map[string]interface{}
	mu      sync.RWMutex
}

// NewFileTagResolver creates a new file tag resolver
func NewFileTagResolver(baseDir string) *FileTagResolver {
	return &FileTagResolver{
		baseDir: baseDir,
		cache:   make(map[string]interface{}),
	}
}

// Tag returns the YAML tag this resolver handles
func (f *FileTagResolver) Tag() string {
	return "!file"
}

// Resolve processes a YAML node with the !file tag
func (f *FileTagResolver) Resolve(node *yaml.Node) (interface{}, error) {
	// Handle two formats:
	// 1. String scalar: !file ./path/to/file.yaml
	// 2. Mapping: !file {path: ./file.yaml, extract: info.title}
	
	switch node.Kind { //nolint:exhaustive // We only support scalar and mapping nodes
	case yaml.ScalarNode:
		// Simple string format
		return f.loadFile(node.Value, "")
		
	case yaml.MappingNode:
		// Map format with optional extraction
		var fileRef FileRef
		if err := node.Decode(&fileRef); err != nil {
			return nil, fmt.Errorf("invalid !file tag format: %w", err)
		}
		
		if fileRef.Path == "" {
			return nil, fmt.Errorf("!file tag requires 'path' field")
		}
		
		return f.loadFile(fileRef.Path, fileRef.Extract)
		
	default:
		return nil, fmt.Errorf("!file tag must be used with a string or map, got %v", node.Kind)
	}
}

// loadFile loads a file and optionally extracts a value
func (f *FileTagResolver) loadFile(path string, extractPath string) (interface{}, error) {
	// Validate the path
	if err := f.validatePath(path); err != nil {
		return nil, err
	}
	
	// Resolve the full path
	fullPath := f.resolvePath(path)
	
	// Check cache first
	cacheKey := fullPath
	if extractPath != "" {
		cacheKey = fmt.Sprintf("%s#%s", fullPath, extractPath)
	}
	
	if cached := f.getCached(cacheKey); cached != nil {
		return cached, nil
	}
	
	// Load the file
	data, err := f.readFile(fullPath)
	if err != nil {
		return nil, err
	}
	
	// Parse the content based on extension
	content, err := f.parseContent(fullPath, data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	
	// Extract value if path is specified
	var result = content
	if extractPath != "" {
		result, err = ExtractValue(content, extractPath)
		if err != nil {
			return nil, fmt.Errorf("failed to extract '%s' from %s: %w", extractPath, path, err)
		}
	}
	
	// Cache the result
	f.setCached(cacheKey, result)
	
	return result, nil
}

// validatePath ensures the path is safe to use
func (f *FileTagResolver) validatePath(path string) error {
	// Clean the path first
	cleaned := filepath.Clean(path)
	
	// Check for absolute paths
	if filepath.IsAbs(path) {
		return fmt.Errorf("absolute paths are not allowed: %s", path)
	}
	
	// Check for parent directory traversal after cleaning
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("parent directory traversal is not allowed: %s", path)
	}
	
	return nil
}

// resolvePath resolves a path relative to the base directory
func (f *FileTagResolver) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(f.baseDir, path)
}

// readFile reads a file with size limits
func (f *FileTagResolver) readFile(path string) ([]byte, error) {
	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to stat file %s: %w", path, err)
	}
	
	// Check file size
	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("file %s is too large (%d bytes, max %d)", path, info.Size(), MaxFileSize)
	}
	
	// Read the file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer file.Close()
	
	data, err := io.ReadAll(io.LimitReader(file, MaxFileSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	
	return data, nil
}

// parseContent parses file content based on extension
func (f *FileTagResolver) parseContent(path string, data []byte) (interface{}, error) {
	ext := strings.ToLower(filepath.Ext(path))
	
	switch ext {
	case ".yaml", ".yml":
		var content interface{}
		if err := k8syaml.Unmarshal(data, &content); err != nil {
			return nil, err
		}
		return content, nil
		
	case ".json":
		var content interface{}
		// sigs.k8s.io/yaml handles both YAML and JSON
		if err := k8syaml.Unmarshal(data, &content); err != nil {
			return nil, err
		}
		return content, nil
		
	default:
		// For other files, return as string
		return string(data), nil
	}
}

// getCached retrieves a cached value
func (f *FileTagResolver) getCached(key string) interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.cache[key]
}

// setCached stores a value in the cache
func (f *FileTagResolver) setCached(key string, value interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cache[key] = value
}