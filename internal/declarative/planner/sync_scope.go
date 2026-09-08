package planner

import (
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
)

func syncScopeMetadata(scope *resources.SyncScope) *PlanSyncScope {
	if scope == nil || !scope.HasAny() {
		return nil
	}

	rootTypes := scope.RootTypes()
	childScopes := scope.ChildScopes()
	rootChildTypes := scope.RootChildCollectionTypes()
	metadata := &PlanSyncScope{
		RootResourceTypes:          make([]string, 0, len(rootTypes)),
		ChildResourceTypes:         make([]PlanSyncChildScope, 0, len(childScopes)),
		RootChildResourceTypes:     make([]string, 0, len(rootChildTypes)),
		OrganizationUsers:          scope.OrganizationUsersScoped,
		OrganizationSystemAccounts: scope.OrganizationSystemAccountsScoped,
	}
	for _, rt := range rootTypes {
		metadata.RootResourceTypes = append(metadata.RootResourceTypes, string(rt))
	}
	for _, child := range childScopes {
		metadata.ChildResourceTypes = append(metadata.ChildResourceTypes, PlanSyncChildScope{
			ParentType:   string(child.ParentType),
			ParentRef:    child.ParentRef,
			ResourceType: string(child.ResourceType),
		})
	}
	for _, rt := range rootChildTypes {
		metadata.RootChildResourceTypes = append(metadata.RootChildResourceTypes, string(rt))
	}

	return metadata
}

func ensurePlanningSyncScope(rs *resources.ResourceSet) {
	if rs == nil || rs.SyncScope != nil {
		return
	}
	if rs.IsEmpty() && !rs.HasSyncScopeSelectors() {
		return
	}

	scope := rs.EnsureSyncScope()
	rs.InferRegisteredSyncScope(scope)
}

func excludeExternalOnlyControlPlaneSyncScope(rs *resources.ResourceSet) {
	if rs == nil || rs.SyncScope == nil || len(rs.ControlPlanes) == 0 {
		return
	}
	for i := range rs.ControlPlanes {
		if !rs.ControlPlanes[i].IsExternal() {
			return
		}
	}
	rs.SyncScope.RemoveRoot(resources.ResourceTypeControlPlane)
}

func validateSyncScope(scope *resources.SyncScope) error {
	rootChildTypes := scope.RootChildCollectionTypes()
	if len(rootChildTypes) == 0 {
		return validateParentScopes(scope)
	}

	names := make([]string, 0, len(rootChildTypes))
	for _, rt := range rootChildTypes {
		names = append(names, string(rt))
	}

	return fmt.Errorf(
		"sync requires empty child collections to be scoped under a parent resource; "+
			"%s cannot be used at the root with an empty list. "+
			"Move the empty collection under the parent resource, for example apis: [{ref: my-api, documents: []}]",
		strings.Join(names, ", "),
	)
}

func validateParentScopes(scope *resources.SyncScope) error {
	if scope == nil {
		return nil
	}
	for _, child := range scope.ChildScopes() {
		if !syncRootParentType(child.ParentType) || scope.RootInScope(child.ParentType) {
			continue
		}
		if tags.IsExternalPlaceholder(child.ParentRef) {
			continue
		}
		guidance := "add the parent resource collection or move the child collection under that parent"
		if syncParentTypeSupportsExternal(child.ParentType) {
			guidance += "; if the parent is managed elsewhere, declare it with _external in the parent " +
				"collection and nest the child collection there"
		}
		return fmt.Errorf(
			"sync child collection %s for %s %q requires the parent collection to be present; %s",
			child.ResourceType,
			child.ParentType,
			child.ParentRef,
			guidance,
		)
	}
	return nil
}

func syncRootParentType(rt resources.ResourceType) bool {
	// This switch intentionally handles only resource types that can own nested sync scope.
	//nolint:exhaustive
	switch rt {
	case resources.ResourceTypeAPI,
		resources.ResourceTypePortal,
		resources.ResourceTypeControlPlane,
		resources.ResourceTypeAIGateway,
		resources.ResourceTypeEventGatewayControlPlane,
		resources.ResourceTypeOrganizationTeam:
		return true
	default:
		return false
	}
}

func syncParentTypeSupportsExternal(rt resources.ResourceType) bool {
	//nolint:exhaustive
	switch rt {
	case resources.ResourceTypeAPI,
		resources.ResourceTypePortal,
		resources.ResourceTypeControlPlane,
		resources.ResourceTypeAIGateway,
		resources.ResourceTypeEventGatewayControlPlane,
		resources.ResourceTypeOrganizationTeam:
		return true
	default:
		return false
	}
}

// prepareScope ensures the planning sync scope is initialized and returns it.
// Returns (nil, true) when the caller should unconditionally plan (not sync mode).
// Returns (nil, false) when the caller should skip planning (sync, no resources).
// Returns (scope, false) when the caller should check the scope.
func (p *Planner) prepareScope(isSync bool) (*resources.SyncScope, bool) {
	if !isSync {
		return nil, true
	}
	if p == nil || p.resources == nil {
		return nil, false
	}
	ensurePlanningSyncScope(p.resources)
	return p.resources.SyncScope, false
}

func (p *Planner) shouldPlanRoot(plan *Plan, rt resources.ResourceType) bool {
	scope, planAll := p.prepareScope(plan != nil && plan.Metadata.Mode == PlanModeSync)
	if planAll {
		return true
	}
	return scope.RootInScope(rt) || scope.ParentHasChildScope(rt)
}

func (p *Planner) shouldPlanChild(
	plan *Plan,
	parentType resources.ResourceType,
	parentRef string,
	rt resources.ResourceType,
) bool {
	scope, planAll := p.prepareScope(plan != nil && plan.Metadata.Mode == PlanModeSync)
	if planAll {
		return true
	}
	return scope.ChildInScope(parentType, parentRef, rt)
}

func (p *Planner) shouldPlanOrganization(plan *Plan) bool {
	scope, planAll := p.prepareScope(plan != nil && plan.Metadata.Mode == PlanModeSync)
	if planAll {
		return true
	}
	return scope.RootInScope(resources.ResourceTypeOrganizationTeam) ||
		scope.OrganizationUsersInScope() ||
		scope.OrganizationSystemAccountsInScope()
}

func (p *Planner) shouldPlanOrganizationUsers(plan *Plan) bool {
	scope, planAll := p.prepareScope(plan != nil && plan.Metadata.Mode == PlanModeSync)
	if planAll {
		return true
	}
	return scope.OrganizationUsersInScope()
}

func (p *Planner) shouldPlanOrganizationSystemAccounts(plan *Plan) bool {
	scope, planAll := p.prepareScope(plan != nil && plan.Metadata.Mode == PlanModeSync)
	if planAll {
		return true
	}
	return scope.OrganizationSystemAccountsInScope()
}
