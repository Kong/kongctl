package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
)

// registerOrganizationExecutors preserves create/delete-only assignments and
// their action-specific reference requirements alongside the managed team root.
func (e *Executor) registerOrganizationExecutors() {
	client, dryRun := e.client, e.dryRun
	e.registerResourceExecutor(crudResourceExecutor(
		NewManagedLabelBaseExecutor(NewOrganizationTeamAdapter(client), client, dryRun),
	))

	teamRole := createDeleteResourceExecutor(
		NewBaseExecutor(NewOrganizationTeamRoleAdapter(client), client, dryRun),
	)
	teamRole.create = prepareResourceWrite(teamRole.create, e.resolveRoleEntityRef)
	e.registerResourceExecutor(prepareResourceExecutor(teamRole, e.prepareOrganizationTeamReference))

	e.registerResourceExecutor(prepareResourceWrites(createDeleteResourceExecutor(
		NewBaseExecutor(NewOrganizationUserTeamMembershipAdapter(client), client, dryRun),
	), e.prepareOrganizationTeamReference))
	e.registerResourceExecutor(prepareResourceWrites(createDeleteResourceExecutor(
		NewBaseExecutor(NewOrganizationUserRoleAdapter(client), client, dryRun),
	), e.resolveRoleEntityRef))
	e.registerResourceExecutor(prepareResourceWrites(createDeleteResourceExecutor(
		NewBaseExecutor(NewOrganizationSystemAccountTeamMembershipAdapter(client), client, dryRun),
	), e.prepareOrganizationTeamReference))
	e.registerResourceExecutor(prepareResourceWrites(createDeleteResourceExecutor(
		NewBaseExecutor(NewOrganizationSystemAccountRoleAdapter(client), client, dryRun),
	), e.resolveRoleEntityRef))
}

func (e *Executor) prepareOrganizationTeamReference(ctx context.Context, change *planner.PlannedChange) error {
	if teamRef, ok := change.References[planner.FieldTeamID]; ok && teamRef.ID == "" {
		teamID, err := e.resolveOrganizationTeamRef(ctx, teamRef)
		if err != nil {
			return fmt.Errorf("failed to resolve organization team reference: %w", err)
		}
		teamRef.ID = teamID
		change.References[planner.FieldTeamID] = teamRef
	}
	return nil
}
