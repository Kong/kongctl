package resources

import (
	"cmp"
	"slices"
)

// SyncScope records the resource collections that were explicitly present in
// declarative input. Sync planning uses this to distinguish omitted collections
// from collections intentionally set to zero desired resources.
type SyncScope struct {
	RootResourceTypes                map[ResourceType]struct{}
	ChildResourceTypes               map[ResourceType]map[string]map[ResourceType]struct{}
	RootChildResourceTypes           map[ResourceType]struct{}
	OrganizationUsersScoped          bool
	OrganizationSystemAccountsScoped bool
}

// ChildSyncScope identifies a child collection scoped under a specific parent.
type ChildSyncScope struct {
	ParentType   ResourceType
	ParentRef    string
	ResourceType ResourceType
}

// NewSyncScope creates an empty sync scope.
func NewSyncScope() *SyncScope {
	return &SyncScope{
		RootResourceTypes:      make(map[ResourceType]struct{}),
		ChildResourceTypes:     make(map[ResourceType]map[string]map[ResourceType]struct{}),
		RootChildResourceTypes: make(map[ResourceType]struct{}),
	}
}

// EnsureSyncScope returns the ResourceSet scope, creating it if necessary.
func (rs *ResourceSet) EnsureSyncScope() *SyncScope {
	if rs.SyncScope == nil {
		rs.SyncScope = NewSyncScope()
	}
	return rs.SyncScope
}

// MergeSyncScope merges sync scope from another ResourceSet.
func (rs *ResourceSet) MergeSyncScope(other *ResourceSet) {
	if other == nil || other.SyncScope == nil {
		return
	}
	rs.EnsureSyncScope().Merge(other.SyncScope)
}

// AddRoot marks a top-level resource collection as in scope.
func (s *SyncScope) AddRoot(rt ResourceType) {
	if s == nil || rt == "" {
		return
	}
	if s.RootResourceTypes == nil {
		s.RootResourceTypes = make(map[ResourceType]struct{})
	}
	s.RootResourceTypes[rt] = struct{}{}
}

// RootInScope reports whether a top-level resource collection is in scope.
func (s *SyncScope) RootInScope(rt ResourceType) bool {
	if s == nil {
		return false
	}
	_, ok := s.RootResourceTypes[rt]
	return ok
}

// RemoveRoot removes a top-level resource collection from sync scope.
func (s *SyncScope) RemoveRoot(rt ResourceType) {
	if s == nil {
		return
	}
	delete(s.RootResourceTypes, rt)
}

// AddChild marks a child resource collection as in scope for one parent ref.
func (s *SyncScope) AddChild(parentType ResourceType, parentRef string, rt ResourceType) {
	parentRef = NormalizeResourceRef(parentRef)
	if s == nil || parentRef == "" {
		return
	}
	if s.ChildResourceTypes == nil {
		s.ChildResourceTypes = make(map[ResourceType]map[string]map[ResourceType]struct{})
	}
	if s.ChildResourceTypes[parentType] == nil {
		s.ChildResourceTypes[parentType] = make(map[string]map[ResourceType]struct{})
	}
	if s.ChildResourceTypes[parentType][parentRef] == nil {
		s.ChildResourceTypes[parentType][parentRef] = make(map[ResourceType]struct{})
	}
	s.ChildResourceTypes[parentType][parentRef][rt] = struct{}{}
}

// ChildInScope reports whether a child resource collection is in scope for a parent ref.
func (s *SyncScope) ChildInScope(parentType ResourceType, parentRef string, rt ResourceType) bool {
	if s == nil {
		return false
	}
	byParentRef, ok := s.ChildResourceTypes[parentType]
	if !ok {
		return false
	}
	byChildType, ok := byParentRef[parentRef]
	if !ok {
		return false
	}
	_, ok = byChildType[rt]
	return ok
}

// RebindChildParent moves child sync scope from a declarative selector value to
// its planner-resolved parent ID.
func (s *SyncScope) RebindChildParent(parentType ResourceType, oldRef, newRef string) {
	if s == nil || oldRef == "" || newRef == "" || oldRef == newRef {
		return
	}
	byParentRef := s.ChildResourceTypes[parentType]
	if byParentRef == nil {
		return
	}
	childTypes := byParentRef[oldRef]
	if childTypes == nil {
		return
	}
	if byParentRef[newRef] == nil {
		byParentRef[newRef] = make(map[ResourceType]struct{})
	}
	for childType := range childTypes {
		byParentRef[newRef][childType] = struct{}{}
	}
	delete(byParentRef, oldRef)
}

// AddRootChildCollection records an explicit root-level empty child collection.
// This cannot express a parent and is rejected by sync planning with guidance.
func (s *SyncScope) AddRootChildCollection(rt ResourceType) {
	if s == nil || rt == "" {
		return
	}
	if s.RootChildResourceTypes == nil {
		s.RootChildResourceTypes = make(map[ResourceType]struct{})
	}
	s.RootChildResourceTypes[rt] = struct{}{}
}

// RootChildCollectionTypes returns root-level child collection types with no parent scope.
func (s *SyncScope) RootChildCollectionTypes() []ResourceType {
	if s == nil {
		return nil
	}
	types := make([]ResourceType, 0, len(s.RootChildResourceTypes))
	for rt := range s.RootChildResourceTypes {
		types = append(types, rt)
	}
	slices.Sort(types)
	return types
}

// MarkOrganizationUsersScoped records that organization.users was present.
func (s *SyncScope) MarkOrganizationUsersScoped() {
	if s != nil {
		s.OrganizationUsersScoped = true
	}
}

// MarkOrganizationSystemAccountsScoped records that organization.system-accounts was present.
func (s *SyncScope) MarkOrganizationSystemAccountsScoped() {
	if s != nil {
		s.OrganizationSystemAccountsScoped = true
	}
}

// OrganizationUsersInScope reports whether organization.users was present.
func (s *SyncScope) OrganizationUsersInScope() bool {
	return s != nil && s.OrganizationUsersScoped
}

// OrganizationSystemAccountsInScope reports whether organization.system-accounts was present.
func (s *SyncScope) OrganizationSystemAccountsInScope() bool {
	return s != nil && s.OrganizationSystemAccountsScoped
}

// HasAny reports whether any scope was recorded.
func (s *SyncScope) HasAny() bool {
	if s == nil {
		return false
	}
	return len(s.RootResourceTypes) > 0 ||
		len(s.ChildResourceTypes) > 0 ||
		len(s.RootChildResourceTypes) > 0 ||
		s.OrganizationUsersScoped ||
		s.OrganizationSystemAccountsScoped
}

// RootTypes returns sorted root resource types.
func (s *SyncScope) RootTypes() []ResourceType {
	if s == nil {
		return nil
	}
	types := make([]ResourceType, 0, len(s.RootResourceTypes))
	for rt := range s.RootResourceTypes {
		types = append(types, rt)
	}
	slices.Sort(types)
	return types
}

// ChildScopes returns sorted child collection scopes.
func (s *SyncScope) ChildScopes() []ChildSyncScope {
	if s == nil {
		return nil
	}
	total := 0
	for _, byRef := range s.ChildResourceTypes {
		for _, byType := range byRef {
			total += len(byType)
		}
	}
	scopes := make([]ChildSyncScope, 0, total)
	for parentType, byParentRef := range s.ChildResourceTypes {
		for parentRef, byChildType := range byParentRef {
			for rt := range byChildType {
				scopes = append(scopes, ChildSyncScope{
					ParentType:   parentType,
					ParentRef:    parentRef,
					ResourceType: rt,
				})
			}
		}
	}
	slices.SortFunc(scopes, func(a, b ChildSyncScope) int {
		if c := cmp.Compare(a.ParentType, b.ParentType); c != 0 {
			return c
		}
		if c := cmp.Compare(a.ParentRef, b.ParentRef); c != 0 {
			return c
		}
		return cmp.Compare(a.ResourceType, b.ResourceType)
	})
	return scopes
}

// Merge copies another scope into this one.
func (s *SyncScope) Merge(other *SyncScope) {
	if s == nil || other == nil {
		return
	}
	for _, rt := range other.RootTypes() {
		s.AddRoot(rt)
	}
	for _, scope := range other.ChildScopes() {
		s.AddChild(scope.ParentType, scope.ParentRef, scope.ResourceType)
	}
	for _, rt := range other.RootChildCollectionTypes() {
		s.AddRootChildCollection(rt)
	}
	if other.OrganizationUsersScoped {
		s.MarkOrganizationUsersScoped()
	}
	if other.OrganizationSystemAccountsScoped {
		s.MarkOrganizationSystemAccountsScoped()
	}
}
