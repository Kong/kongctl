package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/util"
)

func (t *OrganizationTeamPlannerImpl) planOrganizationSystemAccountAssignmentChanges(
	ctx context.Context,
	namespace string,
	desiredTeams []resources.OrganizationTeamResource,
	currentByName map[string]state.OrganizationTeam,
	plan *Plan,
) error {
	if err := t.planOrganizationSystemAccountTeamMembershipChanges(
		ctx,
		namespace,
		desiredTeams,
		currentByName,
		plan,
	); err != nil {
		return err
	}
	return t.planOrganizationSystemAccountRoleChanges(ctx, namespace, plan)
}

func (t *OrganizationTeamPlannerImpl) planOrganizationSystemAccountTeamMembershipChanges(
	ctx context.Context,
	namespace string,
	desiredTeams []resources.OrganizationTeamResource,
	currentByName map[string]state.OrganizationTeam,
	plan *Plan,
) error {
	desired := t.GetDesiredOrganizationSystemAccountTeamMemberships(namespace)
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	membershipsByAccount := make(map[string][]resources.OrganizationSystemAccountTeamMembershipResource)
	for _, membership := range desired {
		membershipsByAccount[membership.SystemAccount] = append(membershipsByAccount[membership.SystemAccount], membership)
	}
	if plan.Metadata.Mode == PlanModeSync {
		for _, account := range t.organizationSystemAccountsByNamespace(namespace) {
			if _, ok := membershipsByAccount[account.Ref]; !ok {
				membershipsByAccount[account.Ref] = []resources.OrganizationSystemAccountTeamMembershipResource{}
			}
		}
	}

	scopedTeamIDs := make(map[string]bool)
	for _, team := range currentByName {
		if id := util.GetString(team.ID); id != "" {
			scopedTeamIDs[id] = true
		}
	}

	for accountRef, memberships := range membershipsByAccount {
		account := t.organizationSystemAccountByRef(accountRef)
		if account == nil || account.GetKonnectID() == "" {
			continue
		}

		currentMemberships, err := t.planner.client.ListOrganizationSystemAccountTeams(ctx, account.GetKonnectID())
		if err != nil {
			if state.IsAPIClientError(err) {
				return nil
			}
			return fmt.Errorf("failed to list organization teams for system account %s: %w", accountRef, err)
		}

		existingByTeamID := make(map[string]state.OrganizationSystemAccountTeamMembership)
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
			key := account.GetKonnectID() + "|" + teamKey
			if desiredTeamIDs[key] {
				return fmt.Errorf("duplicate organization system account team membership %q", key)
			}
			desiredTeamIDs[key] = true
			if teamID != "" {
				if _, ok := existingByTeamID[teamID]; ok {
					continue
				}
			}
			t.planOrganizationSystemAccountTeamMembershipCreate(
				namespace,
				accountRef,
				account.GetKonnectID(),
				membership.Ref,
				membership.Team,
				teamID,
				teamName,
				plan,
			)
		}

		if plan.Metadata.Mode == PlanModeSync {
			for teamID, existing := range existingByTeamID {
				if !desiredTeamIDs[account.GetKonnectID()+"|"+teamID] {
					t.planOrganizationSystemAccountTeamMembershipDelete(
						namespace,
						accountRef,
						account.GetKonnectID(),
						existing,
						plan,
					)
				}
			}
		}
	}

	return nil
}

func (t *OrganizationTeamPlannerImpl) planOrganizationSystemAccountRoleChanges(
	ctx context.Context,
	namespace string,
	plan *Plan,
) error {
	desired := t.GetDesiredOrganizationSystemAccountRoles(namespace)
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	rolesByAccount := make(map[string][]resources.OrganizationSystemAccountRoleResource)
	for _, role := range desired {
		rolesByAccount[role.SystemAccount] = append(rolesByAccount[role.SystemAccount], role)
	}
	if plan.Metadata.Mode == PlanModeSync {
		for _, account := range t.organizationSystemAccountsByNamespace(namespace) {
			if _, ok := rolesByAccount[account.Ref]; !ok {
				rolesByAccount[account.Ref] = []resources.OrganizationSystemAccountRoleResource{}
			}
		}
	}
	scopedEntityIDs := t.organizationUserRoleScopedEntityIDs(namespace)

	for accountRef, roles := range rolesByAccount {
		account := t.organizationSystemAccountByRef(accountRef)
		if account == nil || account.GetKonnectID() == "" {
			continue
		}

		currentRoles, err := t.planner.client.ListOrganizationSystemAccountRoles(ctx, account.GetKonnectID())
		if err != nil {
			if state.IsAPIClientError(err) {
				return nil
			}
			return fmt.Errorf("failed to list organization roles for system account %s: %w", accountRef, err)
		}

		existingRoles := make(map[string]state.OrganizationSystemAccountRole)
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
				t.resolveOrganizationTeamRoleEntityID(role.EntityID),
				role.EntityTypeName,
				role.EntityRegion,
			)
			if desiredKeys[key] {
				return fmt.Errorf("duplicate organization system account role assignment %q for system account %q",
					key, accountRef)
			}
			desiredKeys[key] = true
			if _, ok := existingRoles[key]; ok {
				continue
			}
			t.planOrganizationSystemAccountRoleCreate(namespace, accountRef, account.GetKonnectID(), role, plan)
		}

		if plan.Metadata.Mode == PlanModeSync {
			for key, existingRole := range existingRoles {
				if !desiredKeys[key] {
					t.planOrganizationSystemAccountRoleDelete(namespace, accountRef, account.GetKonnectID(), existingRole, plan)
				}
			}
		}
	}

	return nil
}

func (t *OrganizationTeamPlannerImpl) planOrganizationSystemAccountTeamMembershipCreate(
	namespace string,
	accountRef string,
	accountID string,
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
		ID: t.planner.nextChangeID(
			ActionCreate,
			ResourceTypeOrganizationSystemAccountTeamMembership,
			membershipRef,
		),
		ResourceType: ResourceTypeOrganizationSystemAccountTeamMembership,
		ResourceRef:  membershipRef,
		Action:       ActionCreate,
		Fields: map[string]any{
			FieldSystemAccountID: accountID,
			FieldTeamID:          teamID,
		},
		References: map[string]ReferenceInfo{
			FieldSystemAccountID: {Ref: accountRef, ID: accountID},
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

func (t *OrganizationTeamPlannerImpl) planOrganizationSystemAccountTeamMembershipDelete(
	namespace string,
	accountRef string,
	accountID string,
	membership state.OrganizationSystemAccountTeamMembership,
	plan *Plan,
) {
	change := PlannedChange{
		ID: t.planner.nextChangeID(
			ActionDelete,
			ResourceTypeOrganizationSystemAccountTeamMembership,
			accountRef+"|"+membership.TeamID,
		),
		ResourceType: ResourceTypeOrganizationSystemAccountTeamMembership,
		ResourceRef:  accountRef + "|" + membership.TeamID,
		ResourceID:   membership.TeamID,
		Action:       ActionDelete,
		Fields: map[string]any{
			FieldSystemAccountID: accountID,
			FieldTeamID:          membership.TeamID,
		},
		References: map[string]ReferenceInfo{
			FieldSystemAccountID: {Ref: accountRef, ID: accountID},
			FieldTeamID:          {ID: membership.TeamID},
		},
		Namespace: namespace,
		Parent:    &ParentInfo{Ref: accountRef, ID: accountID},
	}
	plan.AddChange(change)
}

func (t *OrganizationTeamPlannerImpl) planOrganizationSystemAccountRoleCreate(
	namespace string,
	accountRef string,
	accountID string,
	role resources.OrganizationSystemAccountRoleResource,
	plan *Plan,
) {
	dependencies := []string{}
	if tags.IsRefPlaceholder(role.EntityID) {
		if apiRef, _, ok := tags.ParseRefPlaceholder(role.EntityID); ok {
			if apiChangeID := findChangeID(plan, string(resources.ResourceTypeAPI), apiRef); apiChangeID != "" {
				dependencies = append(dependencies, apiChangeID)
			}
		}
	}

	refs := map[string]ReferenceInfo{
		FieldSystemAccountID: {Ref: accountRef, ID: accountID},
	}
	if tags.IsRefPlaceholder(role.EntityID) {
		refs[FieldEntityID] = ReferenceInfo{Ref: role.EntityID}
	}

	change := PlannedChange{
		ID:           t.planner.nextChangeID(ActionCreate, ResourceTypeOrganizationSystemAccountRole, role.GetRef()),
		ResourceType: ResourceTypeOrganizationSystemAccountRole,
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

func (t *OrganizationTeamPlannerImpl) planOrganizationSystemAccountRoleDelete(
	namespace string,
	accountRef string,
	accountID string,
	role state.OrganizationSystemAccountRole,
	plan *Plan,
) {
	roleKey := buildOrganizationTeamRoleKey(role.RoleName, role.EntityID, role.EntityTypeName, role.EntityRegion)
	roleRef := accountRef + "|" + roleKey
	change := PlannedChange{
		ID:           t.planner.nextChangeID(ActionDelete, ResourceTypeOrganizationSystemAccountRole, roleRef),
		ResourceType: ResourceTypeOrganizationSystemAccountRole,
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
			FieldSystemAccountID: {Ref: accountRef, ID: accountID},
		},
		Namespace: namespace,
		Parent:    &ParentInfo{Ref: accountRef, ID: accountID},
	}
	plan.AddChange(change)
}

func (t *OrganizationTeamPlannerImpl) organizationSystemAccountsByNamespace(
	namespace string,
) []resources.OrganizationSystemAccountResource {
	if t.planner.resources == nil || t.planner.resources.Organization == nil {
		return nil
	}
	var systemAccounts []resources.OrganizationSystemAccountResource
	for _, account := range t.planner.resources.Organization.SystemAccounts {
		if resources.GetNamespace(account.Kongctl) == namespace {
			systemAccounts = append(systemAccounts, account)
		}
	}
	return systemAccounts
}

func (t *OrganizationTeamPlannerImpl) organizationSystemAccountByRef(
	accountRef string,
) *resources.OrganizationSystemAccountResource {
	if t.planner.resources == nil || t.planner.resources.Organization == nil {
		return nil
	}
	for i := range t.planner.resources.Organization.SystemAccounts {
		if t.planner.resources.Organization.SystemAccounts[i].Ref == accountRef {
			return &t.planner.resources.Organization.SystemAccounts[i]
		}
	}
	return nil
}
