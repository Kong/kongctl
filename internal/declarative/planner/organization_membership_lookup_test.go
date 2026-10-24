package planner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestOrganizationMembershipLookupSync(t *testing.T) {
	for _, owner := range []string{"system-accounts", "users"} {
		for _, tc := range []struct {
			name        string
			present     bool
			desired     bool
			action      ActionType
			declareTeam bool
			stale       bool
		}{
			{name: "preserve existing", present: true, desired: true},
			{name: "preserve desired and prune stale", present: true, desired: true, stale: true, action: ActionDelete},
			{name: "create missing", desired: true, action: ActionCreate},
			{name: "prune omitted", present: true, action: ActionDelete},
			{name: "preserve with separate team manifest", present: true, desired: true, declareTeam: true},
			{name: "create with separate team manifest", desired: true, action: ActionCreate, declareTeam: true},
		} {
			t.Run(owner+"/"+tc.name, func(t *testing.T) {
				const teamID = "a1111111-1111-4111-8111-111111111111"
				const staleTeamID = "a2222222-2222-4222-8222-222222222222"
				file := filepath.Join(t.TempDir(), "access.yaml")
				memberships := "      teams: []"
				if tc.desired {
					memberships = `      teams:
        - ref: bookings-customer-data-membership
          team: !lookup {name: customer-data}`
				}
				require.NoError(t, os.WriteFile(file, fmt.Appendf(nil, `
_defaults:
  kongctl:
    namespace: platform
organization:
  %s:
    - ref: bookings-automation
      id: b1111111-1111-4111-8111-111111111111
%s
`, owner, memberships), 0o600))
				sources := []loader.Source{{Type: loader.SourceTypeFile, Path: file}}
				if tc.declareTeam {
					teamFile := filepath.Join(filepath.Dir(file), "teams.yaml")
					require.NoError(t, os.WriteFile(teamFile, []byte(`
_defaults:
  kongctl:
    namespace: platform
organization:
  teams:
    - ref: customer-data-team
      name: customer-data
`), 0o600))
					sources = append(sources, loader.Source{Type: loader.SourceTypeFile, Path: teamFile})
				}
				rs, err := loader.New().LoadFromSources(sources, false)
				require.NoError(t, err)
				// Identity lookup is independent of membership reconciliation.
				const ownerID = "b1111111-1111-4111-8111-111111111111"
				kind := ResourceTypeOrganizationSystemAccountTeamMembership
				if owner == "system-accounts" {
					rs.Organization.SystemAccounts[0].SetKonnectID(ownerID)
				} else {
					kind = ResourceTypeOrganizationUserTeamMembership
					rs.Organization.Users[0].SetKonnectID(ownerID)
				}
				var teams []kkComps.Team
				if tc.present {
					teams = append(teams, kkComps.Team{ID: new(teamID), Name: new("customer-data")})
				}
				observedTeams := []kkComps.TeamResponse{{
					ID: teamID, Name: "customer-data",
					Labels: map[string]string{labels.NamespaceKey: "platform"},
				}}
				if tc.stale {
					teams = append(teams, kkComps.Team{ID: new(staleTeamID), Name: new("stale-team")})
					observedTeams = append(observedTeams, kkComps.TeamResponse{
						ID: staleTeamID, Name: "stale-team",
						Labels: map[string]string{labels.NamespaceKey: "platform"},
					})
				}
				client := state.NewClient(state.ClientConfig{
					OrganizationTeamAPI: &organizationTeamAPIStub{
						listOrganizationTeams: func(context.Context, kkOps.ListTeamsRequest) (*kkOps.ListTeamsResponse, error) {
							return &kkOps.ListTeamsResponse{TeamCollectionResponse: &kkComps.TeamCollectionResponse{
								Data: observedTeams,
								Meta: &kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: float64(len(observedTeams))}},
							}}, nil
						},
					},
					OrganizationMembershipAPI: &organizationTeamMembershipAPIStub{
						listUserTeams: func(
							_ context.Context, req kkOps.ListUserTeamsRequest, _ ...kkOps.Option,
						) (*kkOps.ListUserTeamsResponse, error) {
							require.Equal(t, ownerID, req.UserID)
							return &kkOps.ListUserTeamsResponse{UserTeamMembershipCollection: userTeamMembershipCollection(teams...)}, nil
						},
					},
					SystemAccountMembershipAPI: &systemAccountTeamMembershipAPIStub{
						listSystemAccountTeams: func(_ context.Context, req kkOps.GetSystemAccountsAccountIDTeamsRequest,
							_ ...kkOps.Option,
						) (*kkOps.GetSystemAccountsAccountIDTeamsResponse, error) {
							require.Equal(t, ownerID, req.AccountID)
							return &kkOps.GetSystemAccountsAccountIDTeamsResponse{TeamCollection: teamCollection(teams...)}, nil
						},
					},
				})
				plan, err := NewPlanner(client, discardPlannerLogger()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
				require.NoError(t, err)
				if tc.action == "" {
					require.Empty(t, plan.Changes)
				} else {
					require.Len(t, plan.Changes, 1)
					require.Equal(t, tc.action, plan.Changes[0].Action)
					require.Equal(t, kind, plan.Changes[0].ResourceType)
					wantTeamID := teamID
					if tc.stale {
						wantTeamID = staleTeamID
					}
					require.Equal(t, wantTeamID, plan.Changes[0].References[FieldTeamID].ID)
				}
			})
		}
	}
}
