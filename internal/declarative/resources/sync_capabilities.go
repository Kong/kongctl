package resources

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
)

type syncScopeRegistration struct {
	parentType       ResourceType
	emptyRootOrder   int
	emptyRootMessage string
	nestedWithinRoot ResourceType
}

// SyncCollection describes an opted-in collection's sync ownership and YAML
// locations. An empty ParentType identifies a managed root collection.
type SyncCollection struct {
	ResourceType     ResourceType
	RootKey          string
	ParentType       ResourceType
	ParentKey        string
	NestedPaths      [][]string
	EmptyRootMessage string
	emptyRootOrder   int
}

// ChildSyncScopeOption configures explicit child-scope compatibility policies.
type ChildSyncScopeOption func(*syncScopeRegistration) error

// WithEmptyRootCollectionError preserves a loader-stage empty-collection
// rejection and its diagnostic order. Without it, empty root child collections
// retain planner-stage validation.
func WithEmptyRootCollectionError(order int, message string) ChildSyncScopeOption {
	return func(scope *syncScopeRegistration) error {
		if order <= 0 || strings.TrimSpace(message) == "" {
			return fmt.Errorf("empty root collection error requires a positive order and message")
		}
		if scope.emptyRootOrder != 0 {
			return fmt.Errorf("empty root collection error is already registered")
		}
		scope.emptyRootOrder = order
		scope.emptyRootMessage = message
		return nil
	}
}

// WithNestedScopeWithin limits nested scope capture to declarations under the
// given root collection. Root-level child declarations and planner inference
// remain available. Use this for established placement-specific behavior.
func WithNestedScopeWithin(rootType ResourceType) ChildSyncScopeOption {
	return func(scope *syncScopeRegistration) error {
		if rootType == "" || scope.nestedWithinRoot != "" {
			return fmt.Errorf("nested scope requires exactly one root collection")
		}
		scope.nestedWithinRoot = rootType
		return nil
	}
}

// WithRootSyncScope opts a root collection into shared loader scope capture
// and planner inference. Grouped roots retain their dedicated scope handling.
func WithRootSyncScope() ResourceRegistrationOption {
	return withSyncScope("")
}

// WithChildSyncScope opts a child collection into shared scope handling.
// Its sync owner must match GetParentRef and a root-only parent relationship.
// Owners may themselves be children. Singleton, grouping, and indirect
// ownership retain dedicated handling.
func WithChildSyncScope(parentType ResourceType, options ...ChildSyncScopeOption) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		if parentType == "" || !reflect.PointerTo(ops.explain.typ).Implements(reflect.TypeFor[ResourceWithParent]()) {
			return fmt.Errorf("child sync scope requires an owner and ResourceWithParent")
		}
		if err := withSyncScope(parentType)(ops); err != nil {
			return err
		}
		for _, option := range options {
			if err := option(ops.syncScope); err != nil {
				return err
			}
		}
		return nil
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

// Defer derivation until every resource's init function has registered its
// metadata. Cross-resource validation must not depend on registration order.
var cachedSyncCollections = sync.OnceValue(computeSyncCollections)

// SyncCollections returns independent copies, including every nested path.
// Locations and parent selectors are derived once from declaration structure
// and relationships. Early empty-collection errors retain their declared order.
func SyncCollections() []SyncCollection {
	collections := slices.Clone(cachedSyncCollections())
	for i := range collections {
		collections[i].NestedPaths = slices.Clone(collections[i].NestedPaths)
		for j := range collections[i].NestedPaths {
			collections[i].NestedPaths[j] = slices.Clone(collections[i].NestedPaths[j])
		}
	}
	return collections
}

func computeSyncCollections() []SyncCollection {
	kinds := RegisteredTypes()
	slices.Sort(kinds)
	byType := make(map[ResourceType]SyncCollection)
	errorOrders := make(map[int]ResourceType)
	for _, kind := range kinds {
		ops := registry[kind]
		if ops.syncScope == nil {
			continue
		}
		if order := ops.syncScope.emptyRootOrder; order > 0 {
			if existing, ok := errorOrders[order]; ok {
				panic("duplicate empty root collection error order: " + string(existing) + " and " + string(kind))
			}
			errorOrders[order] = kind
		}
		collection := SyncCollection{
			ResourceType:     kind,
			RootKey:          resourceSetRootKey(ops.explain.typ),
			ParentType:       ops.syncScope.parentType,
			EmptyRootMessage: ops.syncScope.emptyRootMessage,
			emptyRootOrder:   ops.syncScope.emptyRootOrder,
		}
		if collection.ParentType != "" {
			parent, ok := registry[collection.ParentType]
			if !ok || parent.syncScope == nil {
				panic("sync collection requires a registered owner: " + string(kind))
			}
			collection.ParentKey = syncParentKey(kind, collection.ParentType)
		}
		byType[kind] = collection
	}

	var collections []SyncCollection
	for _, kind := range kinds {
		collection, ok := byType[kind]
		if !ok {
			continue
		}
		collection.NestedPaths = syncDeclarationPaths(kind, byType, make(map[ResourceType]bool))[1:]
		if rootType := registry[kind].syncScope.nestedWithinRoot; rootType != "" {
			root, ok := byType[rootType]
			if !ok || root.ParentType != "" {
				panic("nested sync scope requires a registered root: " + string(kind))
			}
			collection.NestedPaths = slices.DeleteFunc(collection.NestedPaths, func(path []string) bool {
				return path[0] != root.RootKey
			})
			if len(collection.NestedPaths) == 0 {
				panic("nested sync scope root is not an ancestor: " + string(kind))
			}
		}
		collections = append(collections, collection)
	}
	slices.SortFunc(collections, func(a, b SyncCollection) int {
		if a.emptyRootOrder > 0 && b.emptyRootOrder == 0 {
			return -1
		}
		if a.emptyRootOrder == 0 && b.emptyRootOrder > 0 {
			return 1
		}
		if order := cmp.Compare(a.emptyRootOrder, b.emptyRootOrder); order != 0 {
			return order
		}
		return cmp.Compare(a.ResourceType, b.ResourceType)
	})
	return collections
}

func syncParentKey(kind, parentType ResourceType) string {
	var key string
	for _, relationship := range RelationshipDescriptorsForType(kind) {
		if relationship.Kind == RelationshipKindKongctlParentSelector && relationship.RootOnly &&
			relationship.TargetType == parentType {
			if key != "" {
				panic("ambiguous sync parent selector: " + string(kind))
			}
			key = relationship.FieldPath
		}
	}
	if key == "" {
		panic("sync collection requires a root-only parent selector: " + string(kind))
	}
	return key
}

// Follow declared sync ownership, not every structural relationship: indirect
// owners and recursive documents must not acquire additional deletion scope.
func syncDeclarationPaths(
	kind ResourceType,
	collections map[ResourceType]SyncCollection,
	visiting map[ResourceType]bool,
) [][]string {
	if visiting[kind] {
		panic("cycle in sync collection ownership: " + string(kind))
	}
	visiting[kind] = true
	defer delete(visiting, kind)

	collection := collections[kind]
	paths := [][]string{{collection.RootKey}}
	if collection.ParentType == "" {
		return paths
	}
	parentPaths := syncDeclarationPaths(collection.ParentType, collections, visiting)
	for _, field := range nestedResourceFields(registry[collection.ParentType].explain.typ) {
		if field.resourceType != kind || !field.array {
			continue
		}
		for _, parentPath := range parentPaths {
			paths = append(paths, append(slices.Clone(parentPath), field.name))
		}
	}
	return paths
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
