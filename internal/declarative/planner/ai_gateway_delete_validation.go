package planner

import "context"

// ValidateAIGatewayChildDeletion observes current users immediately before an
// executor deletes a child. Earlier detachments/deletions in a saved plan have
// already run; future intentions cannot authorize deleting a retained user.
func (p *Planner) ValidateAIGatewayChildDeletion(ctx context.Context, change PlannedChange) error {
	ref, id := aiGatewayChangeGateway(change)
	plan := NewPlan(CurrentPlanVersion, "execution", PlanModeSync)
	change.Action = ActionDelete
	plan.AddChange(change)
	for _, policy := range aiGatewayDeleteResolvers {
		if policy.targetType == change.ResourceType {
			return policy.resolve(p, ctx, change.Namespace, ref, id, plan)
		}
	}
	return nil
}
