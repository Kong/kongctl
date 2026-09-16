package resources

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectorLoadRegistrations(t *testing.T) {
	tests := []struct {
		selector ResourceType
		want     []ResourceType
	}{
		{ResourceTypeOrganizationUser, []ResourceType{
			ResourceTypeOrganizationUserTeamMembership, ResourceTypeOrganizationUserRole,
		}},
		{ResourceTypeOrganizationSystemAccount, []ResourceType{
			ResourceTypeOrganizationSystemAccountTeamMembership, ResourceTypeOrganizationSystemAccountRole,
		}},
	}

	for _, tt := range tests {
		t.Run(string(tt.selector), func(t *testing.T) {
			require.Contains(t, selectorLoaders, tt.selector)
			require.False(t, IsRegistered(tt.selector), "selectors must stay outside the resource registry")
			for _, phase := range []struct {
				name          string
				registrations []childLoadRegistration
			}{
				{"extraction", childExtractors[tt.selector]},
				{"validation", childValidators[tt.selector]},
			} {
				t.Run(phase.name, func(t *testing.T) {
					var got []ResourceType
					for _, registration := range phase.registrations {
						got = append(got, registration.kind)
					}
					require.Equal(t, tt.want, got)
				})
			}
		})
	}
}

func TestSelectorLoadRejectsDuplicateOwner(t *testing.T) {
	original := selectorLoaders[ResourceTypeOrganizationUser]
	t.Cleanup(func() { selectorLoaders[ResourceTypeOrganizationUser] = original })

	require.PanicsWithValue(t, "duplicate selector loader: "+string(ResourceTypeOrganizationUser), func() {
		registerSelectorLoader(
			ResourceTypeOrganizationUser, (*ResourceSet).organizationUsers, validateOrganizationUserSelectors,
		)
	})
}

func TestSelectorLoadRejectsInvalidChildBinding(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*childLoad[OrganizationUserRoleResource, OrganizationUserResource])
		want      string
	}{
		{
			"wrong family",
			func(load *childLoad[OrganizationUserRoleResource, OrganizationUserResource]) {
				load.family = ResourceTypeOrganizationSystemAccount
			},
			"selector child loader requires its registered selector family: ",
		},
		{
			"nested validation",
			func(load *childLoad[OrganizationUserRoleResource, OrganizationUserResource]) {
				load.validateNested = func(*ResourceSet, *OrganizationUserResource) error { return nil }
			},
			"selector child loader validates selectors through their owner registration: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			load := childLoad[OrganizationUserRoleResource, OrganizationUserResource]{
				family:        ResourceTypeOrganizationUser,
				extractOrder:  20,
				validateOrder: 20,
				extract:       func(*ResourceSet, *OrganizationUserResource, *[]OrganizationUserRoleResource) {},
				validate:      validateOrganizationUserRoles,
			}
			tt.configure(&load)
			require.PanicsWithValue(t, tt.want+string(ResourceTypeOrganizationUserRole), func() {
				registerSelectorChildResourceType(
					selectorLoader[OrganizationUserResource]{kind: ResourceTypeOrganizationUser},
					ResourceTypeOrganizationUserRole,
					func(rs *ResourceSet) *[]OrganizationUserRoleResource { return &rs.OrganizationUserRoles },
					AutoExplain[OrganizationUserRoleResource](),
					load,
				)
			})
		})
	}
}

func TestSelectorLoadRejectsParentSourceMismatch(t *testing.T) {
	for _, kind := range []ResourceType{ResourceTypeOrganizationUserRole, ResourceTypeOrganizationTeamRole} {
		t.Run(string(kind), func(t *testing.T) {
			require.NotNil(t, registry[kind].load)
			registration := *registry[kind].load
			if registration.extractSelector != nil {
				registration.extractSelector = nil
				registration.extract = func(*ResourceSet, Resource) {}
			} else {
				registration.extract = nil
				registration.extractSelector = func(*ResourceSet, any) {}
			}
			require.PanicsWithValue(t, "child loader extraction must match its parent source: "+string(kind), func() {
				registerChildLoader(kind, registration)
			})
		})
	}
}

func TestSelectorLoadOrganizationGroupValidation(t *testing.T) {
	type assignment struct {
		kind ResourceType
		rs   ResourceSet
	}
	tests := []struct {
		selector    ResourceType
		ownerLabel  string
		missing     string
		assignments []assignment
	}{
		{
			selector:   ResourceTypeOrganizationUser,
			ownerLabel: "organization user",
			missing:    "organization.users must define selectors for root-level user assignments",
			assignments: []assignment{
				{ResourceTypeOrganizationUserTeamMembership, ResourceSet{
					OrganizationUserTeamMemberships: []OrganizationUserTeamMembershipResource{
						{Ref: "assignment", User: "owner", Team: "team"},
					},
				}},
				{ResourceTypeOrganizationUserRole, ResourceSet{
					OrganizationUserRoles: []OrganizationUserRoleResource{
						{
							Ref: "assignment", User: "owner", RoleName: "Viewer",
							EntityID: "*", EntityTypeName: "APIs", EntityRegion: "us",
						},
					},
				}},
			},
		},
		{
			selector:   ResourceTypeOrganizationSystemAccount,
			ownerLabel: "organization system account",
			missing:    "organization.system-accounts must define selectors for root-level system account assignments",
			assignments: []assignment{
				{ResourceTypeOrganizationSystemAccountTeamMembership, ResourceSet{
					OrganizationSystemAccountTeamMemberships: []OrganizationSystemAccountTeamMembershipResource{
						{Ref: "assignment", SystemAccount: "owner", Team: "team"},
					},
				}},
				{ResourceTypeOrganizationSystemAccountRole, ResourceSet{
					OrganizationSystemAccountRoles: []OrganizationSystemAccountRoleResource{
						{
							Ref: "assignment", SystemAccount: "owner", RoleName: "Viewer",
							EntityID: "*", EntityTypeName: "APIs", EntityRegion: "us",
						},
					},
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.selector), func(t *testing.T) {
			t.Run("no assignments", func(t *testing.T) {
				rs := &ResourceSet{}
				require.NoError(t, rs.ValidateRegisteredSelectorChildren(tt.selector))
				rs.Organization = &OrganizationResource{}
				require.NoError(t, rs.ValidateRegisteredSelectorChildren(tt.selector))
			})
			for _, assignment := range tt.assignments {
				t.Run(string(assignment.kind), func(t *testing.T) {
					rs := assignment.rs
					require.EqualError(t, rs.ValidateRegisteredSelectorChildren(tt.selector), tt.missing)
					rs.Organization = &OrganizationResource{}
					require.EqualError(t, rs.ValidateRegisteredSelectorChildren(tt.selector),
						fmt.Sprintf("%s %q references unknown %s: owner", assignment.kind, "assignment", tt.ownerLabel))
				})
			}
		})
	}
}
