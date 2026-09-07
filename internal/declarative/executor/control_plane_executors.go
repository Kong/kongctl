package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
)

func (e *Executor) registerControlPlaneExecutor() {
	resource := crudResourceExecutor(
		NewManagedLabelBaseExecutor(NewControlPlaneAdapter(e.client), e.client, e.dryRun),
	)
	resource.create = afterResourceWrite(resource.create, e.syncControlPlaneGroupMembers)
	resource.update = afterResourceWrite(resource.update, e.syncControlPlaneGroupMembers)
	remove := resource.remove
	resource.remove = func(ctx context.Context, change *planner.PlannedChange) error {
		if err := e.detachControlPlaneGroupMembers(ctx, change); err != nil {
			return fmt.Errorf("failed to detach control plane group members: %w", err)
		}
		return remove(ctx, change)
	}
	e.registerResourceExecutor(resource)
}

func (e *Executor) registerControlPlaneDataPlaneCertificateExecutor() {
	e.registerResourceExecutor(prepareResourceWrites(createDeleteResourceExecutor(
		NewBaseCreateDeleteExecutor(NewControlPlaneDataPlaneCertificateAdapter(e.client), e.dryRun),
	), e.prepareControlPlaneCertificateReference))
}

func (e *Executor) prepareControlPlaneCertificateReference(ctx context.Context, change *planner.PlannedChange) error {
	if controlPlaneRef, ok := change.References[planner.FieldControlPlaneID]; ok && controlPlaneRef.ID == "" {
		controlPlaneID, err := e.resolveControlPlaneRef(ctx, controlPlaneRef)
		if err != nil {
			return fmt.Errorf("failed to resolve control plane reference: %w", err)
		}
		controlPlaneRef.ID = controlPlaneID
		change.References[planner.FieldControlPlaneID] = controlPlaneRef
	}
	return nil
}
