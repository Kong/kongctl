package adapters

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/external"
	"github.com/kong/kongctl/internal/declarative/state"
)

// BaseAdapter provides common functionality for all resolution adapters
type BaseAdapter struct {
	client *state.Client
}

// NewBaseAdapter creates a new base adapter with state client
func NewBaseAdapter(client *state.Client) *BaseAdapter {
	return &BaseAdapter{client: client}
}

// ValidateParentContext validates parent context for child resources
func (b *BaseAdapter) ValidateParentContext(parent *external.ResolvedParent, expectedType string) error {
	if parent == nil {
		return fmt.Errorf("parent context required for child resource")
	}
	if parent.ResourceType != expectedType {
		return fmt.Errorf("invalid parent type: expected %s, got %s", expectedType, parent.ResourceType)
	}
	if parent.ID == "" {
		return fmt.Errorf("parent ID is required")
	}
	return nil
}

// FilterBySelector filters resources by selector fields and ensures exactly one match
func (b *BaseAdapter) FilterBySelector(resources []interface{}, selector map[string]string, 
	getField func(interface{}, string) string) (interface{}, error) {
	
	var matches []interface{}
	for _, resource := range resources {
		match := true
		for field, value := range selector {
			if getField(resource, field) != value {
				match = false
				break
			}
		}
		if match {
			matches = append(matches, resource)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no resources found matching selector: %v", selector)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("selector matched %d resources, expected 1: %v", len(matches), selector)
	}

	return matches[0], nil
}

// GetClient returns the state client for use by concrete adapters
func (b *BaseAdapter) GetClient() *state.Client {
	return b.client
}