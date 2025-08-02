package tags

// We need gopkg.in/yaml.v3 for custom YAML tag support which sigs.k8s.io/yaml doesn't provide
//nolint:gomodguard // yaml.v3 required for custom tag processing
import "gopkg.in/yaml.v3"

// TagResolver processes custom YAML tags
type TagResolver interface {
	// Tag returns the YAML tag this resolver handles (e.g., "!file")
	Tag() string
	
	// Resolve processes a YAML node with this tag and returns the resolved value
	Resolve(node *yaml.Node) (interface{}, error)
}

// FileRef represents a file reference with optional value extraction
type FileRef struct {
	Path    string `yaml:"path"`    // Path to the file to load
	Extract string `yaml:"extract"` // Optional: path to extract value (e.g., "info.title")
}

// ResolvedValue represents a value that was resolved from a tag
type ResolvedValue struct {
	Value  interface{} // The resolved value
	Source string      // Source information for error reporting
}