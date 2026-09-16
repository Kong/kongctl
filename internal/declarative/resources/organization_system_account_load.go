package resources

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/tags"
)

func validateOrganizationSystemAccountSelectors(rs *ResourceSet) error {
	if rs.Organization == nil {
		if rs.hasRegisteredSelectorAssignments(ResourceTypeOrganizationSystemAccount) {
			return fmt.Errorf(
				"organization.system-accounts must define selectors for root-level system account assignments",
			)
		}
		return nil
	}

	systemAccounts := rs.Organization.SystemAccounts
	userRefs := selectorRefs(rs.organizationUsers())

	systemAccountRefs := make(map[string]bool)
	for i := range systemAccounts {
		systemAccount := &systemAccounts[i]
		if err := systemAccount.Validate(); err != nil {
			return fmt.Errorf("invalid organization system account %q: %w", systemAccount.Ref, err)
		}
		if existing, found := rs.GetResourceByRef(systemAccount.Ref); found {
			return fmt.Errorf("duplicate ref '%s' (already defined as %s)", systemAccount.Ref, existing.GetType())
		}
		if userRefs[systemAccount.Ref] {
			return fmt.Errorf(
				"duplicate ref '%s' (already defined as %s)",
				systemAccount.Ref,
				ResourceTypeOrganizationUser,
			)
		}
		if systemAccountRefs[systemAccount.Ref] {
			return fmt.Errorf("duplicate organization system account ref: %s", systemAccount.Ref)
		}
		systemAccountRefs[systemAccount.Ref] = true
	}

	return nil
}

func validateOrganizationSystemAccountTeamMemberships(
	rs *ResourceSet,
	memberships []OrganizationSystemAccountTeamMembershipResource,
) error {
	systemAccountRefs := selectorRefs(rs.organizationSystemAccounts())
	assignmentRefs := make(map[string]bool)

	for i := range memberships {
		membership := &memberships[i]
		if err := membership.Validate(); err != nil {
			return fmt.Errorf("invalid organization_system_account_team_membership %q: %w", membership.GetRef(), err)
		}
		if assignmentRefs[membership.GetRef()] {
			return fmt.Errorf("duplicate organization system account team membership: %s", membership.GetRef())
		}
		assignmentRefs[membership.GetRef()] = true
		if !systemAccountRefs[membership.SystemAccount] {
			return fmt.Errorf(
				"organization_system_account_team_membership %q references unknown organization system account: %s",
				membership.GetRef(),
				membership.SystemAccount,
			)
		}
		if tags.IsExternalPlaceholder(membership.Team) {
			continue
		}
		if resource, found := rs.GetResourceByRef(membership.Team); !found {
			return fmt.Errorf("organization_system_account_team_membership %q references unknown organization_team: %s",
				membership.GetRef(), membership.Team)
		} else if resource.GetType() != ResourceTypeOrganizationTeam {
			return fmt.Errorf(
				"organization_system_account_team_membership %q references %s but expected organization_team: %s",
				membership.GetRef(),
				resource.GetType(),
				membership.Team,
			)
		}
	}

	return nil
}

func validateOrganizationSystemAccountRoles(rs *ResourceSet, roles []OrganizationSystemAccountRoleResource) error {
	systemAccountRefs := selectorRefs(rs.organizationSystemAccounts())

	roleRefs := make(map[string]bool)
	for i := range roles {
		role := &roles[i]
		if err := role.Validate(); err != nil {
			return fmt.Errorf("invalid organization_system_account_role %q: %w", role.GetRef(), err)
		}
		if roleRefs[role.GetRef()] {
			return fmt.Errorf("duplicate organization_system_account_role ref: %s", role.GetRef())
		}
		roleRefs[role.GetRef()] = true
		if !systemAccountRefs[role.SystemAccount] {
			return fmt.Errorf(
				"organization_system_account_role %q references unknown organization system account: %s",
				role.GetRef(),
				role.SystemAccount,
			)
		}
		if err := ValidateRoleEntityReference(
			ResourceTypeOrganizationSystemAccountRole, role.GetRef(), role.EntityID, role.EntityTypeName, rs,
		); err != nil {
			return err
		}
	}

	return nil
}
