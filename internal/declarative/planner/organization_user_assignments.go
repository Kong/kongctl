package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/util"
)

func (t *OrganizationTeamPlannerImpl) planOrganizationUserAssignmentChanges(
	ctx context.Context,
	namespace string,
	desiredTeams []resources.OrganizationTeamResource,
	currentByName map[string]state.OrganizationTeam,
	plan *Plan,
) error {
	if err := t.planOrganizationUserTeamMembershipChanges(ctx, namespace, desiredTeams, currentByName, plan); err != nil {
		return err
	}
	return t.planOrganizationUserRoleChanges(ctx, namespace, plan)
}

func (t *OrganizationTeamPlannerImpl) planOrganizationUserTeamMembershipChanges(
	ctx context.Context,
	namespace string,
	desiredTeams []resources.OrganizationTeamResource,
	currentByName map[string]state.OrganizationTeam,
	plan *Plan,
) error {
	desired := t.GetDesiredOrganizationUserTeamMemberships(namespace)
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	membershipsByUser := make(map[string][]resources.OrganizationUserTeamMembershipResource)
	for _, membership := range desired {
		membershipsByUser[membership.User] = append(membershipsByUser[membership.User], membership)
	}

	if plan.Metadata.Mode == PlanModeSync {
		for _, user := range t.organizationUsersByNamespace(namespace) {
			if _, ok := membershipsByUser[user.Ref]; !ok {
				membershipsByUser[user.Ref] = []resources.OrganizationUserTeamMembershipResource{}
			}
		}
	}
	scopedTeamIDs := make(map[string]bool)
	for _, team := range currentByName {
		if id := util.GetString(team.ID); id != "" {
			scopedTeamIDs[id] = true
		}
	}

	for userRef, memberships := range membershipsByUser {
		user := t.organizationUserByRef(userRef)
		if user == nil || user.GetKonnectID() == "" {
			continue
		}

		currentMemberships, err := t.planner.client.ListOrganizationUserTeams(ctx, user.GetKonnectID())
		if err != nil {
			if state.IsAPIClientError(err) {
				return nil
			}
			return fmt.Errorf("failed to list organization teams for user %s: %w", userRef, err)
		}

		existingByTeamID := make(map[string]state.OrganizationUserTeamMembership)
		for _, membership := range currentMemberships {
			if plan.Metadata.Mode == PlanModeSync && !scopedTeamIDs[membership.TeamID] {
				continue
			}
			existingByTeamID[membership.TeamID] = membership
		}

		desiredTeamIDs := make(map[string]bool)
		for _, membership := range memberships {
			team, teamID, teamName := t.resolveOrganizationTeamForAssignment(membership.Team, desiredTeams, currentByName)
			if team == nil {
				continue
			}
			teamKey := teamID
			if teamKey == "" {
				teamKey = membership.Team
			}
			key := user.GetKonnectID() + "|" + teamKey
			if desiredTeamIDs[key] {
				return fmt.Errorf("duplicate organization user team membership %q", key)
			}
			desiredTeamIDs[key] = true
			if teamID != "" {
				if _, ok := existingByTeamID[teamID]; ok {
					continue
				}
			}
			t.planOrganizationUserTeamMembershipCreate(
				namespace,
				userRef,
				user.GetKonnectID(),
				membership.Ref,
				membership.Team,
				teamID,
				teamName,
				plan,
			)
		}

		if plan.Metadata.Mode == PlanModeSync {
			for teamID, existing := range existingByTeamID {
				if !desiredTeamIDs[user.GetKonnectID()+"|"+teamID] {
					t.planOrganizationUserTeamMembershipDelete(namespace, userRef, user.GetKonnectID(), existing, plan)
				}
			}
		}
	}

	return nil
}

func (t *OrganizationTeamPlannerImpl) planOrganizationUserRoleChanges(
	ctx context.Context,
	namespace string,
	plan *Plan,
) error {
	desired := t.GetDesiredOrganizationUserRoles(namespace)
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	rolesByUser := make(map[string][]resources.OrganizationUserRoleResource)
	for _, role := range desired {
		rolesByUser[role.User] = append(rolesByUser[role.User], role)
	}
	if plan.Metadata.Mode == PlanModeSync {
		for _, user := range t.organizationUsersByNamespace(namespace) {
			if _, ok := rolesByUser[user.Ref]; !ok {
				rolesByUser[user.Ref] = []resources.OrganizationUserRoleResource{}
			}
		}
	}
	scopedEntityIDs := t.organizationRoleScopedEntityIDs(namespace)

	for userRef, roles := range rolesByUser {
		user := t.organizationUserByRef(userRef)
		if user == nil || user.GetKonnectID() == "" {
			continue
		}

		currentRoles, err := t.planner.client.ListOrganizationUserRoles(ctx, user.GetKonnectID())
		if err != nil {
			if state.IsAPIClientError(err) {
				return nil
			}
			return fmt.Errorf("failed to list organization roles for user %s: %w", userRef, err)
		}

		existingRoles := make(map[string]state.OrganizationUserRole)
		for _, role := range currentRoles {
			if plan.Metadata.Mode == PlanModeSync && !scopedEntityIDs[role.EntityID] {
				continue
			}
			key := buildOrganizationTeamRoleKey(role.RoleName, role.EntityID, role.EntityTypeName, role.EntityRegion)
			existingRoles[key] = role
		}

		desiredKeys := make(map[string]bool)
		for _, role := range roles {
			key := buildOrganizationTeamRoleKey(
				role.RoleName,
				t.resolveOrganizationTeamRoleEntityID(role.EntityID, role.EntityTypeName),
				role.EntityTypeName,
				role.EntityRegion,
			)
			if desiredKeys[key] {
				return fmt.Errorf("duplicate organization user role assignment %q for user %q", key, userRef)
			}
			desiredKeys[key] = true
			if _, ok := existingRoles[key]; ok {
				continue
			}
			t.planOrganizationUserRoleCreate(namespace, userRef, user.GetKonnectID(), role, plan)
		}

		if plan.Metadata.Mode == PlanModeSync {
			for key, existingRole := range existingRoles {
				if !desiredKeys[key] {
					t.planOrganizationUserRoleDelete(namespace, userRef, user.GetKonnectID(), existingRole, plan)
				}
			}
		}
	}

	return nil
}

func (t *OrganizationTeamPlannerImpl) planOrganizationUserTeamMembershipCreate(
	namespace string,
	userRef string,
	userID string,
	membershipRef string,
	teamRef string,
	teamID string,
	teamName string,
	plan *Plan,
) {
	dependencies := []string{}
	if teamChangeID := findChangeID(plan, ResourceTypeOrganizationTeam, teamRef); teamChangeID != "" {
		dependencies = append(dependencies, teamChangeID)
	}

	change := PlannedChange{
		ID:           t.planner.nextChangeID(ActionCreate, ResourceTypeOrganizationUserTeamMembership, membershipRef),
		ResourceType: ResourceTypeOrganizationUserTeamMembership,
		ResourceRef:  membershipRef,
		Action:       ActionCreate,
		Fields: map[string]any{
			FieldUserID: userID,
			FieldTeamID: teamID,
		},
		References: map[string]ReferenceInfo{
			FieldUserID: {Ref: userRef, ID: userID},
			FieldTeamID: {
				Ref: teamRef,
				ID:  teamID,
				LookupFields: map[string]string{
					FieldName: teamName,
				},
			},
		},
		Namespace: namespace,
		DependsOn: dependencies,
	}
	plan.AddChange(change)
}

func (t *OrganizationTeamPlannerImpl) planOrganizationUserTeamMembershipDelete(
	namespace string,
	userRef string,
	userID string,
	membership state.OrganizationUserTeamMembership,
	plan *Plan,
) {
	change := PlannedChange{
		ID: t.planner.nextChangeID(
			ActionDelete,
			ResourceTypeOrganizationUserTeamMembership,
			userRef+"|"+membership.TeamID,
		),
		ResourceType: ResourceTypeOrganizationUserTeamMembership,
		ResourceRef:  userRef + "|" + membership.TeamID,
		ResourceID:   membership.TeamID,
		Action:       ActionDelete,
		Fields: map[string]any{
			FieldUserID: userID,
			FieldTeamID: membership.TeamID,
		},
		References: map[string]ReferenceInfo{
			FieldUserID: {Ref: userRef, ID: userID},
			FieldTeamID: {ID: membership.TeamID},
		},
		Namespace: namespace,
		Parent:    &ParentInfo{Ref: userRef, ID: userID},
	}
	plan.AddChange(change)
}

func (t *OrganizationTeamPlannerImpl) planOrganizationUserRoleCreate(
	namespace string,
	userRef string,
	userID string,
	role resources.OrganizationUserRoleResource,
	plan *Plan,
) {
	dependencies := []string{}
	if tags.IsRefPlaceholder(role.EntityID) {
		dependencies = appendRoleEntityDependency(dependencies, plan, role.EntityID, role.EntityTypeName)
	}

	refs := map[string]ReferenceInfo{
		FieldUserID: {Ref: userRef, ID: userID},
	}
	if tags.IsRefPlaceholder(role.EntityID) {
		refs[FieldEntityID] = ReferenceInfo{Ref: role.EntityID}
	}

	change := PlannedChange{
		ID:           t.planner.nextChangeID(ActionCreate, ResourceTypeOrganizationUserRole, role.GetRef()),
		ResourceType: ResourceTypeOrganizationUserRole,
		ResourceRef:  role.GetRef(),
		Action:       ActionCreate,
		Fields: map[string]any{
			FieldRoleName:       role.RoleName,
			FieldEntityID:       role.EntityID,
			FieldEntityTypeName: role.EntityTypeName,
			FieldEntityRegion:   role.EntityRegion,
		},
		References: refs,
		Namespace:  namespace,
		DependsOn:  dependencies,
	}
	plan.AddChange(change)
}

func (t *OrganizationTeamPlannerImpl) planOrganizationUserRoleDelete(
	namespace string,
	userRef string,
	userID string,
	role state.OrganizationUserRole,
	plan *Plan,
) {
	roleKey := buildOrganizationTeamRoleKey(role.RoleName, role.EntityID, role.EntityTypeName, role.EntityRegion)
	roleRef := userRef + "|" + roleKey
	change := PlannedChange{
		ID:           t.planner.nextChangeID(ActionDelete, ResourceTypeOrganizationUserRole, roleRef),
		ResourceType: ResourceTypeOrganizationUserRole,
		ResourceRef:  roleRef,
		ResourceID:   role.ID,
		Action:       ActionDelete,
		Fields: map[string]any{
			FieldRoleName:       role.RoleName,
			FieldEntityID:       role.EntityID,
			FieldEntityTypeName: role.EntityTypeName,
			FieldEntityRegion:   role.EntityRegion,
		},
		References: map[string]ReferenceInfo{
			FieldUserID: {Ref: userRef, ID: userID},
		},
		Namespace: namespace,
		Parent:    &ParentInfo{Ref: userRef, ID: userID},
	}
	plan.AddChange(change)
}

func (t *OrganizationTeamPlannerImpl) organizationUsersByNamespace(
	namespace string,
) []resources.OrganizationUserResource {
	if t.planner.resources == nil || t.planner.resources.Organization == nil {
		return nil
	}
	var users []resources.OrganizationUserResource
	for _, user := range t.planner.resources.Organization.Users {
		if resources.GetNamespace(user.Kongctl) == namespace {
			users = append(users, user)
		}
	}
	return users
}

func (t *OrganizationTeamPlannerImpl) organizationUserByRef(userRef string) *resources.OrganizationUserResource {
	if t.planner.resources == nil || t.planner.resources.Organization == nil {
		return nil
	}
	for i := range t.planner.resources.Organization.Users {
		if t.planner.resources.Organization.Users[i].Ref == userRef {
			return &t.planner.resources.Organization.Users[i]
		}
	}
	return nil
}

func (t *OrganizationTeamPlannerImpl) organizationRoleScopedEntityIDs(namespace string) map[string]bool {
	scoped := map[string]bool{
		"*": true,
	}
	if t.planner == nil || t.planner.resources == nil {
		return scoped
	}
	for _, api := range t.planner.resources.GetAPIsByNamespace(namespace) {
		if id := api.GetKonnectID(); id != "" {
			scoped[id] = true
		}
	}
	for _, portal := range t.planner.resources.GetPortalsByNamespace(namespace) {
		if id := portal.GetKonnectID(); id != "" {
			scoped[id] = true
		}
	}
	for _, controlPlane := range t.planner.resources.GetControlPlanesByNamespace(namespace) {
		if id := controlPlane.GetKonnectID(); id != "" {
			scoped[id] = true
		}
	}
	return scoped
}

func (t *OrganizationTeamPlannerImpl) resolveOrganizationTeamForAssignment(
	teamRef string,
	desiredTeams []resources.OrganizationTeamResource,
	currentByName map[string]state.OrganizationTeam,
) (*resources.OrganizationTeamResource, string, string) {
	for i := range desiredTeams {
		team := &desiredTeams[i]
		if team.Ref != teamRef {
			continue
		}
		teamName := team.Name
		if team.IsExternal() && team.External.Selector != nil {
			if selectorName := team.External.Selector.MatchFields[FieldName]; selectorName != "" {
				teamName = selectorName
			}
		}
		teamID := team.GetKonnectID()
		if teamID == "" && team.IsExternal() {
			teamID = team.External.ID
		}
		if current, ok := currentByName[teamName]; ok && teamID == "" {
			teamID = util.GetString(current.ID)
		}
		return team, teamID, teamName
	}
	return nil, "", ""
}
