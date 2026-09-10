package resources

import (
	"fmt"
	"reflect"
)

// GetResourcesByNamespace returns shallow copies from the registered collection,
// in source order. Grouping locations used only before extraction are excluded.
// A resource must register namespace selection through WithNamespace or
// WithNamespaceFrom; asking for an unsupported kind is a programming error.
func GetResourcesByNamespace[R any, RPtr interface {
	*R
	Resource
}](rs *ResourceSet, namespace string) []R {
	kind := RPtr(new(R)).GetType()
	matches := namespaceMatcher(kind)
	var filtered []R
	registry[kind].forEach(rs, func(resource Resource) bool {
		if matches(rs, resource, namespace) {
			filtered = append(filtered, *any(resource).(*R))
		}
		return true
	})
	return filtered
}

// WithNamespaceFrom selects a child using its owner's registered namespace
// policy. The typed lookup preserves each family's reference-matching behavior;
// a missing owner excludes the child.
func WithNamespaceFrom[R, P any, PPtr interface {
	*P
	Resource
}](owner func(*ResourceSet, *R) *P) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		if ops.matchesNamespace != nil {
			return fmt.Errorf("namespace capability is already registered")
		}
		if owner == nil || ops.explain.typ != reflect.TypeFor[R]() {
			return fmt.Errorf("namespace owner accessor must match the registered resource type")
		}
		// Record the dependency without requiring the owner to register first.
		ownerKind := PPtr(new(P)).GetType()
		ops.namespaceOwner = ownerKind
		ops.matchesNamespace = func(rs *ResourceSet, resource Resource, namespace string) bool {
			parent := owner(rs, any(resource).(*R))
			if parent == nil {
				return false
			}
			return namespaceMatcher(ownerKind)(rs, PPtr(parent), namespace)
		}
		return nil
	}
}

func namespaceMatcher(kind ResourceType) func(*ResourceSet, Resource, string) bool {
	matches := registry[kind].matchesNamespace
	if matches == nil {
		panic("resource type " + string(kind) + " has no namespace selection")
	}
	return matches
}

func resourceNamespaceMatches(external bool, meta *KongctlMeta, namespace string) bool {
	if external {
		return namespace == NamespaceExternal
	}
	return GetNamespace(meta) == namespace
}
