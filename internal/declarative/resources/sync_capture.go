package resources

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

type nestedSyncScopeCapture func(*SyncScope, string, map[string]any, []SyncCollection) error

// WithNestedSyncScopeCapture registers a root's specialized presence policy.
// Ordinary ownership paths remain derived; policies handle coupled collections
// and non-resource declaration shapes before extraction loses key presence.
func WithNestedSyncScopeCapture(capture nestedSyncScopeCapture) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		if capture == nil || ops.syncScope == nil || ops.syncScope.parentType != "" ||
			ops.syncScope.captureNested != nil {
			return fmt.Errorf("nested scope capture requires one policy on a registered root")
		}
		ops.syncScope.captureNested = capture
		return nil
	}
}

type syncSelector struct {
	parentKey string
	path      func() []string
	populated func(*ResourceSet) bool
	mark      func(*SyncScope)
}

var syncSelectors = make(map[ResourceType]syncSelector)

// Selectors carry group-level scope without entering the resource lifecycle.
func registerSyncSelector[R any](
	kind ResourceType,
	parentKey string,
	source func(*ResourceSet) []R,
	mark func(*SyncScope),
) {
	if _, exists := syncSelectors[kind]; exists || kind == "" || parentKey == "" || source == nil || mark == nil {
		panic("sync selector requires a unique type, parent key, source, and marker")
	}
	syncSelectors[kind] = syncSelector{
		parentKey: parentKey,
		path: sync.OnceValue(func() []string {
			path := resourceSetDeclarationPath(reflect.TypeFor[R]())
			if len(path) == 0 {
				panic("sync selector requires a declaration collection: " + string(kind))
			}
			return path
		}),
		populated: func(rs *ResourceSet) bool { return len(source(rs)) > 0 },
		mark:      mark,
	}
}

// WithSelectorAssignmentSyncScope captures root-level assignment parent keys.
// Fallback inference marks the selector group rather than individual children.
func WithSelectorAssignmentSyncScope(selectorType ResourceType) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		if selectorType == "" {
			return fmt.Errorf("assignment sync scope requires a selector")
		}
		if err := withSyncScope(selectorType)(ops); err != nil {
			return err
		}
		ops.syncScope.selectorAssignment = true
		return nil
	}
}

// HasSyncScopeSelectors includes selectors that do not contribute to IsEmpty.
func (rs *ResourceSet) HasSyncScopeSelectors() bool {
	if rs == nil {
		return false
	}
	for _, selector := range syncSelectors {
		if selector.populated(rs) {
			return true
		}
	}
	return false
}

// CaptureDeclared records explicit collection presence before nested resource
// extraction. The loader owns YAML decoding; resource policies own sync scope.
func (s *SyncScope) CaptureDeclared(raw map[string]any) error {
	collections := cachedSyncCollections()
	for _, collection := range collections {
		if collection.ParentType == "" {
			if _, ok := scopeValueAtPath(raw, collection.RootPath); ok {
				s.AddRoot(collection.ResourceType)
			}
		}
	}
	for _, collection := range collections {
		if collection.ParentType != "" {
			if err := s.captureRootChild(raw, collection); err != nil {
				return err
			}
		}
	}
	for _, collection := range collections {
		if collection.ParentType == "" || registry[collection.ResourceType].syncScope.selectorAssignment {
			continue
		}
		if registry[collection.ParentType].syncScope.captureNested != nil {
			continue
		}
		for _, path := range collection.NestedPaths {
			s.captureNestedPath(raw, path, collection.ParentType, collection.ResourceType)
		}
	}
	for _, collection := range collections {
		capture := registry[collection.ResourceType].syncScope.captureNested
		if capture == nil {
			continue
		}
		var children []SyncCollection
		for _, child := range collections {
			if child.ParentType == collection.ResourceType {
				children = append(children, child)
			}
		}
		value, _ := scopeValueAtPath(raw, collection.RootPath)
		items, _ := value.([]any)
		for _, item := range items {
			parent, ok := item.(map[string]any)
			if !ok {
				continue
			}
			ref, _ := parent[SchemaFieldRef].(string)
			if ref != "" {
				if err := capture(s, ref, parent, children); err != nil {
					return err
				}
			}
		}
	}
	for _, selector := range syncSelectors {
		if _, present := scopeValueAtPath(raw, selector.path()); present {
			selector.mark(s)
		}
	}
	return nil
}

func (s *SyncScope) captureRootChild(raw map[string]any, collection SyncCollection) error {
	value, present := scopeValueAtPath(raw, collection.RootPath)
	if !present {
		return nil
	}
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		if collection.EmptyRootMessage != "" {
			return fmt.Errorf(
				"%s cannot be empty because %s",
				strings.Join(collection.RootPath, "."),
				collection.EmptyRootMessage,
			)
		}
		s.AddRootChildCollection(collection.ResourceType)
		return nil
	}
	for _, item := range items {
		if child, ok := item.(map[string]any); ok {
			if parentRef, ok := child[collection.ParentKey].(string); ok && parentRef != "" {
				s.AddChild(collection.ParentType, parentRef, collection.ResourceType)
			}
		}
	}
	return nil
}

func (s *SyncScope) captureNestedPath(raw map[string]any, path []string, parentType, resourceType ResourceType) {
	if len(path) == 1 {
		if ref, ok := raw[SchemaFieldRef].(string); ok && ref != "" {
			if _, present := raw[path[0]]; present {
				s.addNestedChild(parentType, ref, resourceType)
			}
		}
		return
	}
	// Grouping objects retain their map shape; resource collections are arrays.
	switch value := raw[path[0]].(type) {
	case map[string]any:
		s.captureNestedPath(value, path[1:], parentType, resourceType)
	case []any:
		for _, item := range value {
			if parent, ok := item.(map[string]any); ok {
				s.captureNestedPath(parent, path[1:], parentType, resourceType)
			}
		}
	}
}

func (s *SyncScope) addNestedChild(parentType ResourceType, ref string, resourceType ResourceType) {
	s.AddChild(parentType, ref, resourceType)
	for _, related := range registry[resourceType].syncScope.coScopes {
		s.AddChild(parentType, ref, related)
	}
}

func scopeValueAtPath(raw map[string]any, path []string) (any, bool) {
	for _, key := range path[:len(path)-1] {
		group, ok := raw[key].(map[string]any)
		if !ok {
			return nil, false
		}
		raw = group
	}
	value, present := raw[path[len(path)-1]]
	return value, present
}
