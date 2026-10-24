package planner

import (
	"context"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestOrganizationMembershipReconciliation(t *testing.T) {
	for _, owner := range []string{"user", "system account"} {
		for _, tc := range []struct {
			name      string
			mode      PlanMode
			refs      []string
			creates   []int
			createID  string
			deletes   []string
			duplicate string
		}{
			{
				name: "resolved team without observed membership creates", mode: PlanModeApply,
				refs: []string{"ghost"}, creates: []int{0}, createID: "ghost-id",
			},
			{
				name: "apply retains observations", mode: PlanModeApply,
				refs: []string{"known", "new-a"}, creates: []int{1},
			},
			{
				name: "sync prunes only scoped stale memberships", mode: PlanModeSync,
				refs: []string{"known", "new-a"}, creates: []int{1}, deletes: []string{"stale-id"},
			},
			{
				name: "duplicate unresolved ref preserves earlier create without pruning", mode: PlanModeSync,
				refs: []string{"new-a", "new-a"}, creates: []int{0}, duplicate: "new-a",
			},
			{
				name: "duplicate resolved ID fails even when observed", mode: PlanModeSync,
				refs: []string{"known", "alias"}, duplicate: "known-id",
			},
			{
				name: "distinct unresolved refs create in desired order", mode: PlanModeSync,
				refs: []string{"new-b", "new-a", "known"}, creates: []int{0, 1}, deletes: []string{"stale-id"},
			},
			{
				name: "unknown teams skip duplicate detection", mode: PlanModeSync,
				refs: []string{"missing", "missing", "known"}, deletes: []string{"stale-id"},
			},
			{
				name: "unresolved ref retains legacy pruning key without matching observation", mode: PlanModeSync,
				refs: []string{"stale-id", "known"}, creates: []int{0},
			},
			{
				name: "foreign unapplied team cannot protect same-named observed membership", mode: PlanModeSync,
				refs: []string{"known", "foreign"}, deletes: []string{"stale-id"},
			},
			{
				name: "empty sync prunes scoped observations", mode: PlanModeSync,
				deletes: []string{"known-id", "stale-id"},
			},
		} {
			t.Run(owner+"/"+tc.name, func(t *testing.T) {
				const namespace = "membership-namespace"
				user := resources.OrganizationUserResource{Ref: "owner", Email: "owner@example.com"}
				user.SetKonnectID("owner-id")
				account := resources.OrganizationSystemAccountResource{Ref: "owner", Name: "Owner"}
				account.SetKonnectID("owner-id")
				rs := &resources.ResourceSet{Organization: &resources.OrganizationResource{}}
				var desiredTeams []resources.OrganizationTeamResource
				for _, ref := range []string{"known", "alias", "ghost", "new-a", "new-b", "stale-id", "missing", "foreign"} {
					team := resources.OrganizationTeamResource{
						BaseResource: resources.BaseResource{Ref: ref, Kongctl: plannerNamespaceMeta(namespace)},
						CreateTeam:   kkComps.CreateTeam{Name: ref},
					}
					if ref == "foreign" {
						team.Name = "stale"
						team.Kongctl = plannerNamespaceMeta("other-namespace")
					}
					if ref == "alias" {
						team.SetKonnectID("known-id")
					}
					if ref == "ghost" {
						team.SetKonnectID("ghost-id")
					}
					rs.OrganizationTeams = append(rs.OrganizationTeams, team)
					// Keep the declaration selectable while testing resolution's skip contract.
					if ref != "missing" && ref != "foreign" {
						desiredTeams = append(desiredTeams, team)
					}
				}
				for i, ref := range tc.refs {
					membershipRef := fmt.Sprintf("membership-%d", i)
					rs.OrganizationUserTeamMemberships = append(rs.OrganizationUserTeamMemberships,
						resources.OrganizationUserTeamMembershipResource{Ref: membershipRef, User: "owner", Team: ref})
					rs.OrganizationSystemAccountTeamMemberships = append(rs.OrganizationSystemAccountTeamMemberships,
						resources.OrganizationSystemAccountTeamMembershipResource{
							Ref: membershipRef, SystemAccount: "owner", Team: ref,
						})
				}
				observed := []kkComps.Team{
					{ID: new("known-id"), Name: new("known")},
					{ID: new("stale-id"), Name: new("stale")},
					{ID: new("outside-id"), Name: new("outside")},
				}
				queries := 0
				clientConfig := state.ClientConfig{}
				kind := ResourceTypeOrganizationUserTeamMembership
				ownerField := FieldUserID
				if owner == "user" {
					rs.Organization.Users = []resources.OrganizationUserResource{user}
					clientConfig.OrganizationMembershipAPI = &organizationTeamMembershipAPIStub{
						listUserTeams: func(_ context.Context, req kkOps.ListUserTeamsRequest,
							_ ...kkOps.Option,
						) (*kkOps.ListUserTeamsResponse, error) {
							require.Equal(t, "owner-id", req.UserID)
							queries++
							return &kkOps.ListUserTeamsResponse{UserTeamMembershipCollection: userTeamMembershipCollection(observed...)}, nil
						},
					}
				} else {
					kind = ResourceTypeOrganizationSystemAccountTeamMembership
					ownerField = FieldSystemAccountID
					rs.Organization.SystemAccounts = []resources.OrganizationSystemAccountResource{account}
					clientConfig.SystemAccountMembershipAPI = &systemAccountTeamMembershipAPIStub{
						listSystemAccountTeams: func(_ context.Context, req kkOps.GetSystemAccountsAccountIDTeamsRequest,
							_ ...kkOps.Option,
						) (*kkOps.GetSystemAccountsAccountIDTeamsResponse, error) {
							require.Equal(t, "owner-id", req.AccountID)
							queries++
							return &kkOps.GetSystemAccountsAccountIDTeamsResponse{
								TeamCollection: teamCollection(observed...),
							}, nil
						},
					}
				}
				p := NewPlanner(state.NewClient(clientConfig), discardPlannerLogger())
				p.resources = rs
				tp := NewOrganizationTeamPlanner(NewBasePlanner(p)).(*OrganizationTeamPlannerImpl)
				plan := NewPlan("1.0", "test", tc.mode)
				for _, ref := range []string{"new-a", "new-b", "stale-id"} {
					plan.AddChange(PlannedChange{
						ID: "create-" + ref, ResourceType: ResourceTypeOrganizationTeam,
						ResourceRef: ref, Action: ActionCreate,
					})
				}
				current := map[string]state.OrganizationTeam{
					"known": {TeamResponse: kkComps.TeamResponse{ID: "known-id", Name: "known"}},
					"stale": {TeamResponse: kkComps.TeamResponse{ID: "stale-id", Name: "stale"}},
				}
				var err error
				if owner == "user" {
					err = tp.planOrganizationUserTeamMembershipChanges(
						t.Context(),
						namespace,
						desiredTeams,
						current,
						plan,
					)
				} else {
					err = tp.planOrganizationSystemAccountTeamMembershipChanges(
						t.Context(), namespace, desiredTeams, current, plan,
					)
				}
				if tc.duplicate != "" {
					require.EqualError(t, err, fmt.Sprintf("duplicate organization %s team membership %q",
						owner, "owner-id|"+tc.duplicate))
				} else {
					require.NoError(t, err)
				}
				require.Equal(t, 1, queries)
				changes := plan.Changes[3:]
				require.Len(t, changes, len(tc.creates)+len(tc.deletes))
				for i, desiredIndex := range tc.creates {
					change := changes[i]
					ref := tc.refs[desiredIndex]
					require.Equal(t, ActionCreate, change.Action)
					require.Equal(t, fmt.Sprintf("membership-%d", desiredIndex), change.ResourceRef)
					require.Equal(t, ReferenceInfo{Ref: ref, ID: tc.createID, LookupFields: map[string]string{FieldName: ref}},
						change.References[FieldTeamID])
					if tc.createID == "" {
						require.Equal(t, []string{"create-" + ref}, change.DependsOn)
					} else {
						require.Empty(t, change.DependsOn)
					}
				}
				var deleted []string
				for _, change := range changes[len(tc.creates):] {
					require.Equal(t, ActionDelete, change.Action)
					require.Equal(t, "owner|"+change.ResourceID, change.ResourceRef)
					require.Equal(t, ReferenceInfo{ID: change.ResourceID}, change.References[FieldTeamID])
					deleted = append(deleted, change.ResourceID)
				}
				require.ElementsMatch(t, tc.deletes, deleted)
				for _, change := range changes {
					require.Equal(t, kind, change.ResourceType)
					require.Equal(t, namespace, change.Namespace)
					require.Equal(t, ReferenceInfo{Ref: "owner", ID: "owner-id"}, change.References[ownerField])
				}
			})
		}
	}
}
