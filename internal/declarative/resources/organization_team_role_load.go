package resources

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/tags"
)

// ValidateOrganizationTeamRoles checks flattened role identities and references.
func ValidateOrganizationTeamRoles(rs *ResourceSet, roles []OrganizationTeamRoleResource) error {
	roleRefs := make(map[string]bool)

	for i := range roles {
		role := &roles[i]
		if err := role.Validate(); err != nil {
			return fmt.Errorf("invalid organization_team_role %q: %w", role.GetRef(), err)
		}

		if existing, found := rs.GetResourceByRef(role.GetRef()); found {
			if existing.GetType() != ResourceTypeOrganizationTeamRole {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					role.GetRef(), existing.GetType())
			}
		}

		if roleRefs[role.GetRef()] {
			return fmt.Errorf("duplicate organization_team_role ref: %s", role.GetRef())
		}
		roleRefs[role.GetRef()] = true

		if err := validateOrganizationTeamRoleReferences(role, rs); err != nil {
			return err
		}
	}

	return nil
}

func validateOrganizationTeamRoleReferences(
	role *OrganizationTeamRoleResource,
	rs *ResourceSet,
) error {
	if !tags.IsExternalPlaceholder(role.Team) {
		if resource, found := rs.GetResourceByRef(role.Team); !found {
			return fmt.Errorf("organization_team_role %q references unknown organization_team: %s",
				role.GetRef(), role.Team)
		} else if resource.GetType() != ResourceTypeOrganizationTeam {
			return fmt.Errorf("organization_team_role %q references %s but expected organization_team: %s",
				role.GetRef(), resource.GetType(), role.Team)
		}
	}

	return ValidateRoleEntityReference(
		ResourceTypeOrganizationTeamRole,
		role.GetRef(),
		role.EntityID,
		role.EntityTypeName,
		rs,
	)
}
