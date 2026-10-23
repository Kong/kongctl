package planner

import (
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func buildDesiredTeamIDs[M any](
	t *OrganizationTeamPlannerImpl,
	memberships []M,
	matchOwner func(M) bool,
	teamRef func(M) string,
	namespace string,
	currentByName map[string]state.OrganizationTeam,
) map[string]bool {
	desiredTeamIDs := make(map[string]bool)
	for _, membership := range memberships {
		if !matchOwner(membership) {
			continue
		}
		team, id, _ := t.resolveOrganizationTeamForAssignment(
			teamRef(membership), t.planner.resources.OrganizationTeams, currentByName,
		)
		// Foreign managed teams must not borrow this namespace's name-matched IDs.
		if id != "" && (team.IsExternal() || resources.GetNamespace(team.Kongctl) == namespace) {
			desiredTeamIDs[id] = true
		}
	}
	return desiredTeamIDs
}
