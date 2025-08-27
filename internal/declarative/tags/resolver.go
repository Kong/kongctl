package tags

import (
	"bytes"
	"fmt"
	"sync"

	"gopkg.in/yaml.v3" //nolint:gomodguard // yaml.v3 required for custom tag processing
)

// ResolverRegistry manages tag resolvers and processes YAML with custom tags
type ResolverRegistry struct {
	resolvers map[string]TagResolver
	mu        sync.RWMutex
}

// NewResolverRegistry creates a new tag resolver registry
func NewResolverRegistry() *ResolverRegistry {
	return &ResolverRegistry{
		resolvers: make(map[string]TagResolver),
	}
}

// Register adds a tag resolver to the registry
func (r *ResolverRegistry) Register(resolver TagResolver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resolvers[resolver.Tag()] = resolver
}

// HasResolvers returns true if any resolvers are registered
func (r *ResolverRegistry) HasResolvers() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.resolvers) > 0
}

// Process takes YAML data with custom tags and returns processed YAML
func (r *ResolverRegistry) Process(data []byte) ([]byte, error) {
	// Parse the YAML to get the document structure
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Process custom tags in the document
	if err := r.processNode(&doc); err != nil {
		return nil, fmt.Errorf("failed to process tags: %w", err)
	}

	// Marshal back to YAML
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc); err != nil {
		return nil, fmt.Errorf("failed to encode YAML: %w", err)
	}

	return buf.Bytes(), nil
}

// processNode recursively processes YAML nodes and resolves custom tags
func (r *ResolverRegistry) processNode(node *yaml.Node) error {
	if node == nil {
		return nil
	}

	// Check if this node has a custom tag
	// Custom tags start with ! but not !! (which are built-in YAML tags)
	if node.Tag != "" && len(node.Tag) > 1 && node.Tag[0] == '!' && node.Tag[1] != '!' {
		r.mu.RLock()
		resolver, exists := r.resolvers[node.Tag]
		r.mu.RUnlock()

		if exists {
			// Resolve the tag
			resolved, err := resolver.Resolve(node)
			if err != nil {
				return fmt.Errorf("failed to resolve tag %s: %w", node.Tag, err)
			}

			// Replace the node with the resolved value
			if err := r.replaceNodeWithValue(node, resolved); err != nil {
				return fmt.Errorf("failed to replace node: %w", err)
			}

			// After replacement, the node content has changed, so we should return
			// to avoid processing the new content as if it had tags
			return nil
		}

		// Unknown tag - return an error
		return fmt.Errorf("unsupported YAML tag: %s", node.Tag)
	}

	// Process child nodes based on node kind
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if err := r.processNode(child); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			if err := r.processNode(child); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		// Process key-value pairs (every two nodes form a pair)
		for i := 0; i < len(node.Content); i += 2 {
			// Process the value node (keys shouldn't have custom tags)
			if i+1 < len(node.Content) {
				if err := r.processNode(node.Content[i+1]); err != nil {
					return err
				}
			}
		}
	case yaml.ScalarNode, yaml.AliasNode:
		// Scalar and alias nodes don't have children to process
		// Nothing to do here
	}

	return nil
}

// replaceNodeWithValue replaces a node's content with the resolved value
func (r *ResolverRegistry) replaceNodeWithValue(node *yaml.Node, value any) error {
	// Create a temporary node to marshal the value
	tempNode := &yaml.Node{}
	if err := tempNode.Encode(value); err != nil {
		return fmt.Errorf("failed to encode resolved value: %w", err)
	}

	// Copy the content from the temporary node to the original node
	node.Kind = tempNode.Kind
	node.Tag = tempNode.Tag
	node.Value = tempNode.Value
	node.Content = tempNode.Content
	node.Style = tempNode.Style

	return nil
}
