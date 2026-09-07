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
	if rs.IsEmpty() && !hasOrganizationAssignmentSelectors(rs) {
		return
	}

	scope := rs.EnsureSyncScope()
	rs.InferRegisteredSyncScope(scope)
	addRootIfPresent(scope, resources.ResourceTypeDashboard, len(rs.Dashboards))
	addRootIfPresent(scope, resources.ResourceTypeOrganizationTeam, len(rs.OrganizationTeams))

	addPortalChildScopes(scope, rs)
	addOrganizationChildScopes(scope, rs)
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

func hasOrganizationAssignmentSelectors(rs *resources.ResourceSet) bool {
	return rs != nil && rs.Organization != nil &&
		(len(rs.Organization.Users) > 0 || len(rs.Organization.SystemAccounts) > 0)
}

func addRootIfPresent(scope *resources.SyncScope, rt resources.ResourceType, count int) {
	if count > 0 {
		scope.AddRoot(rt)
	}
}

func addPortalChildScopes(scope *resources.SyncScope, rs *resources.ResourceSet) {
	for _, child := range rs.PortalCustomizations {
		scope.AddChild(resources.ResourceTypePortal, child.Portal, resources.ResourceTypePortalCustomization)
	}
	for _, child := range rs.PortalAuthSettings {
		scope.AddChild(resources.ResourceTypePortal, child.Portal, resources.ResourceTypePortalAuthSettings)
	}
	for _, child := range rs.PortalIPAllowLists {
		scope.AddChild(resources.ResourceTypePortal, child.Portal, resources.ResourceTypePortalIPAllowList)
	}
	for _, child := range rs.PortalIntegrations {
		scope.AddChild(resources.ResourceTypePortal, child.Portal, resources.ResourceTypePortalIntegration)
	}
	for _, child := range rs.PortalIdentityProviders {
		scope.AddChild(resources.ResourceTypePortal, child.Portal, resources.ResourceTypePortalIdentityProvider)
	}
	for _, child := range rs.PortalTeamGroupMappings {
		scope.AddChild(resources.ResourceTypePortal, child.Portal, resources.ResourceTypePortalTeamGroupMapping)
	}
	for _, child := range rs.PortalCustomDomains {
		scope.AddChild(resources.ResourceTypePortal, child.Portal, resources.ResourceTypePortalCustomDomain)
	}
	for _, child := range rs.PortalPages {
		scope.AddChild(resources.ResourceTypePortal, child.Portal, resources.ResourceTypePortalPage)
	}
	for _, child := range rs.PortalSnippets {
		scope.AddChild(resources.ResourceTypePortal, child.Portal, resources.ResourceTypePortalSnippet)
	}
	for _, child := range rs.PortalTeams {
		scope.AddChild(resources.ResourceTypePortal, child.Portal, resources.ResourceTypePortalTeam)
		scope.AddChild(resources.ResourceTypePortal, child.Portal, resources.ResourceTypePortalTeamRole)
	}
	for _, child := range rs.PortalTeamRoles {
		scope.AddChild(resources.ResourceTypePortal, child.Portal, resources.ResourceTypePortalTeamRole)
	}
	for _, child := range rs.PortalAssetLogos {
		scope.AddChild(resources.ResourceTypePortal, child.Portal, resources.ResourceTypePortalAssetLogo)
	}
	for _, child := range rs.PortalAssetFavicons {
		scope.AddChild(resources.ResourceTypePortal, child.Portal, resources.ResourceTypePortalAssetFavicon)
	}
	for _, child := range rs.PortalEmailConfigs {
		scope.AddChild(resources.ResourceTypePortal, child.Portal, resources.ResourceTypePortalEmailConfig)
	}
	for _, child := range rs.PortalEmailTemplates {
		scope.AddChild(resources.ResourceTypePortal, child.Portal, resources.ResourceTypePortalEmailTemplate)
	}
	for _, child := range rs.PortalAuditLogWebhooks {
		scope.AddChild(resources.ResourceTypePortal, child.Portal, resources.ResourceTypePortalAuditLogWebhook)
	}
}

func addOrganizationChildScopes(scope *resources.SyncScope, rs *resources.ResourceSet) {
	for _, role := range rs.OrganizationTeamRoles {
		scope.AddChild(resources.ResourceTypeOrganizationTeam, role.Team, resources.ResourceTypeOrganizationTeamRole)
	}
	if (rs.Organization != nil && len(rs.Organization.Users) > 0) ||
		len(rs.OrganizationUserTeamMemberships) > 0 ||
		len(rs.OrganizationUserRoles) > 0 {
		scope.MarkOrganizationUsersScoped()
	}
	if (rs.Organization != nil && len(rs.Organization.SystemAccounts) > 0) ||
		len(rs.OrganizationSystemAccountTeamMemberships) > 0 ||
		len(rs.OrganizationSystemAccountRoles) > 0 {
		scope.MarkOrganizationSystemAccountsScoped()
	}
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
