package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// PortalTeamGroupMappingExecutor updates portal team IdP group mappings.
type PortalTeamGroupMappingExecutor struct {
	client *state.Client
	dryRun bool
}

// NewPortalTeamGroupMappingExecutor creates a portal team group mapping executor.
func NewPortalTeamGroupMappingExecutor(client *state.Client, dryRun bool) *PortalTeamGroupMappingExecutor {
	return &PortalTeamGroupMappingExecutor{client: client, dryRun: dryRun}
}

// Update applies a partial mapping update for one team.
func (p *PortalTeamGroupMappingExecutor) Update(ctx context.Context, change planner.PlannedChange) (string, error) {
	portalID, err := portalIDFromChange(change)
	if err != nil {
		return "", err
	}

	teamID, err := teamIDFromChange(change)
	if err != nil {
		return "", err
	}

	groups, ok := change.Fields[planner.FieldGroups].([]string)
	if !ok {
		rawGroups, ok := change.Fields[planner.FieldGroups].([]any)
		if !ok {
			return "", fmt.Errorf("groups is required")
		}
		groups = make([]string, 0, len(rawGroups))
		for _, rawGroup := range rawGroups {
			group, ok := rawGroup.(string)
			if !ok {
				return "", fmt.Errorf("groups must contain strings")
			}
			groups = append(groups, group)
		}
	}

	if p.dryRun {
		return teamID, nil
	}

	if err := p.client.UpdatePortalTeamGroupMapping(ctx, portalID, teamID, groups); err != nil {
		return "", err
	}
	return teamID, nil
}

func portalIDFromChange(change planner.PlannedChange) (string, error) {
	if ref, ok := change.References[planner.FieldPortalID]; ok && ref.ID != "" {
		return ref.ID, nil
	}
	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}
	return "", fmt.Errorf("portal ID is required for portal team group mapping")
}

func teamIDFromChange(change planner.PlannedChange) (string, error) {
	if teamID, ok := change.Fields[planner.FieldTeamID].(string); ok && teamID != "" {
		return teamID, nil
	}
	if ref, ok := change.References[planner.FieldTeamID]; ok && ref.ID != "" {
		return ref.ID, nil
	}
	return "", fmt.Errorf("team ID is required for portal team group mapping")
}
