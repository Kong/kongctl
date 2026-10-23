package planner

// A team can be declared before it has a remote ID. Its ref participates in
// duplicate detection but cannot prove that a membership already exists.
type teamMembershipTarget struct {
	ref  string
	id   string
	name string
}

type teamMembershipOperations[D, C any] struct {
	resolve   func(D) (teamMembershipTarget, bool)
	duplicate func(string) error
	create    func(D, teamMembershipTarget)
	remove    func(C)
}

// reconcileTeamMemberships owns decisions within one observed owner. Callers
// supply the scoped remote-ID index and retain observation and change builders.
// Resolution and duplicate checks stay interleaved with creates; an error
// preserves earlier creates and prevents pruning, just as in role assignments.
func reconcileTeamMemberships[D, C any](
	ownerID string,
	desired []D,
	current map[string]C,
	ops teamMembershipOperations[D, C],
	prune bool,
) error {
	desiredKeys := make(map[string]bool, len(desired))
	for _, membership := range desired {
		team, ok := ops.resolve(membership)
		if !ok {
			continue
		}
		teamKey := team.id
		if teamKey == "" {
			teamKey = team.ref
		}
		key := ownerID + "|" + teamKey
		if desiredKeys[key] {
			return ops.duplicate(key)
		}
		desiredKeys[key] = true
		if team.id != "" {
			if _, exists := current[team.id]; exists {
				continue
			}
		}
		ops.create(membership, team)
	}
	if prune {
		for teamID, membership := range current {
			if !desiredKeys[ownerID+"|"+teamID] {
				ops.remove(membership)
			}
		}
	}
	return nil
}
