package resources

import (
	"fmt"
	"reflect"
	"slices"
)

type syncScopeRegistration struct {
	parentType ResourceType
}

// SyncCollection describes an opted-in collection's sync ownership and YAML
// locations. An empty ParentType identifies a managed root collection.
type SyncCollection struct {
	ResourceType  ResourceType
	RootKey       string
	ParentType    ResourceType
	ParentKey     string
	ParentRootKey string
	NestedKeys    []string
}

// WithRootSyncScope opts a root collection into shared loader scope capture
// and planner inference. Grouped roots retain their dedicated scope handling.
func WithRootSyncScope() ResourceRegistrationOption {
	return withSyncScope("")
}

// WithChildSyncScope opts an ordinary child collection into shared scope
// handling. Its sync owner must match GetParentRef and a root-only parent
// relationship. Singleton, grouping, and indirect ownership need dedicated
// handling rather than this capability.
func WithChildSyncScope(parentType ResourceType) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		if parentType == "" || !reflect.PointerTo(ops.explain.typ).Implements(reflect.TypeFor[ResourceWithParent]()) {
			return fmt.Errorf("child sync scope requires an owner and ResourceWithParent")
		}
		return withSyncScope(parentType)(ops)
	}
}

func withSyncScope(parentType ResourceType) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		if ops.syncScope != nil {
			return fmt.Errorf("sync scope capability is already registered")
		}
		if resourceSetRootKey(ops.explain.typ) == "" {
			return fmt.Errorf("sync scope capability requires a root-level YAML collection")
		}
		ops.syncScope = &syncScopeRegistration{parentType: parentType}
		return nil
	}
}

// SyncCollections derives locations from the same struct metadata as explain
// and parent selectors from relationship descriptors. Scope semantics remain
// opt-in: structural containment alone does not imply sync ownership.
func SyncCollections() []SyncCollection {
	var collections []SyncCollection
	kinds := RegisteredTypes()
	slices.Sort(kinds)
	for _, kind := range kinds {
		ops := registry[kind]
		if ops.syncScope == nil {
			continue
		}
		collection := SyncCollection{
			ResourceType: kind,
			RootKey:      resourceSetRootKey(ops.explain.typ),
			ParentType:   ops.syncScope.parentType,
		}
		if collection.ParentType != "" {
			parent, ok := registry[collection.ParentType]
			if !ok || parent.syncScope == nil || parent.syncScope.parentType != "" {
				panic("sync collection requires a registered root owner: " + string(kind))
			}
			collection.ParentRootKey = resourceSetRootKey(parent.explain.typ)
			for _, relationship := range RelationshipDescriptorsForType(kind) {
				if relationship.Kind == RelationshipKindKongctlParentSelector && relationship.RootOnly &&
					relationship.TargetType == collection.ParentType {
					if collection.ParentKey != "" {
						panic("ambiguous sync parent selector: " + string(kind))
					}
					collection.ParentKey = relationship.FieldPath
				}
			}
			if collection.ParentKey == "" {
				panic("sync collection requires a root-only parent selector: " + string(kind))
			}
			for _, field := range nestedResourceFields(parent.explain.typ) {
				if field.resourceType == kind && field.array {
					collection.NestedKeys = append(collection.NestedKeys, field.name)
				}
			}
		}
		collections = append(collections, collection)
	}
	return collections
}

// InferRegisteredSyncScope adds scopes supported by populated resource slices.
// Callers must retain an existing explicit SyncScope: slices cannot represent
// the loader's distinction between omitted and explicitly empty collections.
func (rs *ResourceSet) InferRegisteredSyncScope(scope *SyncScope) {
	if rs == nil || scope == nil {
		return
	}
	for kind, ops := range registry {
		if ops.syncScope == nil {
			continue
		}
		if ops.syncScope.parentType == "" {
			if ops.count(rs) > 0 {
				scope.AddRoot(kind)
			}
			continue
		}
		ops.forEach(rs, func(resource Resource) bool {
			if parent := resource.(ResourceWithParent).GetParentRef(); parent != nil {
				scope.AddChild(ops.syncScope.parentType, parent.Ref, kind)
			}
			return true
		})
	}
}
