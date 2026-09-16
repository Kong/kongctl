package resources

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/tags"
)

func validateOrganizationUserSelectors(rs *ResourceSet) error {
	if rs.Organization == nil {
		if rs.hasRegisteredSelectorAssignments(ResourceTypeOrganizationUser) {
			return fmt.Errorf("organization.users must define selectors for root-level user assignments")
		}
		return nil
	}

	users := rs.Organization.Users
	userRefs := make(map[string]bool)
	for i := range users {
		user := &users[i]
		if err := user.Validate(); err != nil {
			return fmt.Errorf("invalid organization user %q: %w", user.Ref, err)
		}
		if existing, found := rs.GetResourceByRef(user.Ref); found {
			return fmt.Errorf("duplicate ref '%s' (already defined as %s)", user.Ref, existing.GetType())
		}
		if userRefs[user.Ref] {
			return fmt.Errorf("duplicate organization user ref: %s", user.Ref)
		}
		userRefs[user.Ref] = true
	}

	return nil
}

func validateOrganizationUserTeamMemberships(
	rs *ResourceSet,
	memberships []OrganizationUserTeamMembershipResource,
) error {
	userRefs := selectorRefs(rs.organizationUsers())
	assignmentRefs := make(map[string]bool)

	for i := range memberships {
		membership := &memberships[i]
		if err := membership.Validate(); err != nil {
			return fmt.Errorf("invalid organization_user_team_membership %q: %w", membership.GetRef(), err)
		}
		if assignmentRefs[membership.GetRef()] {
			return fmt.Errorf("duplicate organization user team membership: %s", membership.GetRef())
		}
		assignmentRefs[membership.GetRef()] = true
		if !userRefs[membership.User] {
			return fmt.Errorf(
				"organization_user_team_membership %q references unknown organization user: %s",
				membership.GetRef(),
				membership.User,
			)
		}
		if tags.IsExternalPlaceholder(membership.Team) {
			continue
		}
		if resource, found := rs.GetResourceByRef(membership.Team); !found {
			return fmt.Errorf("organization_user_team_membership %q references unknown organization_team: %s",
				membership.GetRef(), membership.Team)
		} else if resource.GetType() != ResourceTypeOrganizationTeam {
			return fmt.Errorf("organization_user_team_membership %q references %s but expected organization_team: %s",
				membership.GetRef(), resource.GetType(), membership.Team)
		}
	}

	return nil
}

func validateOrganizationUserRoles(rs *ResourceSet, roles []OrganizationUserRoleResource) error {
	userRefs := selectorRefs(rs.organizationUsers())

	roleRefs := make(map[string]bool)
	for i := range roles {
		role := &roles[i]
		if err := role.Validate(); err != nil {
			return fmt.Errorf("invalid organization_user_role %q: %w", role.GetRef(), err)
		}
		if roleRefs[role.GetRef()] {
			return fmt.Errorf("duplicate organization_user_role ref: %s", role.GetRef())
		}
		roleRefs[role.GetRef()] = true
		if !userRefs[role.User] {
			return fmt.Errorf(
				"organization_user_role %q references unknown organization user: %s",
				role.GetRef(),
				role.User,
			)
		}
		if err := ValidateRoleEntityReference(
			ResourceTypeOrganizationUserRole, role.GetRef(), role.EntityID, role.EntityTypeName, rs,
		); err != nil {
			return err
		}
	}

	return nil
}
