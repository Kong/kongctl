package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
)

func (e *Executor) registerEventGatewayControlPlaneExecutor() {
	e.registerResourceExecutor(crudResourceExecutor(
		NewManagedLabelBaseExecutor(NewEventGatewayControlPlaneControlPlaneAdapter(e.client), e.client, e.dryRun),
	))
}

// registerEventGatewayChildExecutors couples construction, payload contracts,
// supported actions, and reference preparation. Deletes use the IDs in the plan.
func (e *Executor) registerEventGatewayChildExecutors() {
	client, dryRun := e.client, e.dryRun
	registerChild := func(resource resourceExecutor, prepare func(context.Context, *planner.PlannedChange) error) {
		e.registerResourceExecutor(prepareResourceWrites(resource, prepare))
	}
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewEventGatewayBackendClusterAdapter(client), client, dryRun),
	), e.prepareEventGatewayReference)

	virtualCluster := crudResourceExecutor(
		NewBaseExecutor(NewEventGatewayVirtualClusterAdapter(client), client, dryRun),
	)
	virtualCluster.create = prepareResourceWrite(virtualCluster.create, e.prepareEventGatewayVirtualClusterCreate)
	virtualCluster.update = prepareResourceWrite(virtualCluster.update, e.prepareEventGatewayVirtualClusterUpdate)
	e.registerResourceExecutor(virtualCluster)

	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewEventGatewayListenerAdapter(client), client, dryRun),
	), e.prepareEventGatewayReference)
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewEventGatewayListenerPolicyAdapter(client), client, dryRun),
	), e.prepareEventGatewayListenerPolicy)
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewEventGatewayClusterPolicyAdapter(client), client, dryRun),
	), e.prepareEventGatewayClusterPolicy)
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewEventGatewayProducePolicyAdapter(client), client, dryRun),
	), e.prepareEventGatewayProducePolicy)
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewEventGatewayConsumePolicyAdapter(client), client, dryRun),
	), e.prepareEventGatewayClusterPolicy)
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewEventGatewayDataPlaneCertificateAdapter(client), client, dryRun),
	), e.prepareEventGatewayReference)
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewEventGatewaySchemaRegistryAdapter(client), client, dryRun),
	), e.prepareEventGatewayReference)
	registerChild(createDeleteResourceExecutor(
		NewBaseExecutor(NewEventGatewayStaticKeyAdapter(client), client, dryRun),
	), e.prepareEventGatewayReference)
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewEventGatewayTLSTrustBundleAdapter(client), client, dryRun),
	), e.prepareEventGatewayReference)
}

func (e *Executor) prepareEventGatewayReference(ctx context.Context, change *planner.PlannedChange) error {
	// Resolve event gateway reference if needed
	if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID == "" {
		gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
		if err != nil {
			return fmt.Errorf("failed to resolve event gateway reference: %w", err)
		}
		// Update the reference with the resolved ID
		gatewayRef.ID = gatewayID
		change.References[planner.FieldEventGatewayID] = gatewayRef
	}
	return nil
}

func (e *Executor) prepareEventGatewayVirtualClusterCreate(ctx context.Context, change *planner.PlannedChange) error {
	// Resolve event gateway reference if needed.
	// When the gateway was already created at plan time, its ID is in change.Parent.ID.
	// When the gateway was being created in the same plan run, change.Parent is nil and
	// the ID is stored in change.References["event_gateway_id"] after resolution below.
	if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok &&
		unresolvedReferenceID(gatewayRef.ID) {
		gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
		if err != nil {
			return fmt.Errorf("failed to resolve event gateway reference: %w", err)
		}
		// Update the reference with the resolved ID
		gatewayRef.ID = gatewayID
		change.References[planner.FieldEventGatewayID] = gatewayRef
	}

	// Determine the effective gateway ID for backend cluster resolution.
	// Prefer the resolved reference over change.Parent (which is nil when the gateway
	// was not yet created at plan time).
	effectiveGatewayID := ""
	if change.Parent != nil {
		effectiveGatewayID = change.Parent.ID
	}
	if ref, ok := change.References[planner.FieldEventGatewayID]; ok && ref.ID != "" {
		effectiveGatewayID = ref.ID
	}

	// Resolve event gateway backend cluster reference if needed
	if backendClusterRef, ok := change.References[planner.FieldEventGatewayBackendClusterID]; ok &&
		unresolvedReferenceID(backendClusterRef.ID) {
		backendClusterID, err := e.resolveEventGatewayBackendClusterRef(ctx, effectiveGatewayID, backendClusterRef)
		if err != nil {
			return fmt.Errorf("failed to resolve event gateway backend cluster reference: %w", err)
		}
		// Update the reference with the resolved ID
		backendClusterRef.ID = backendClusterID
		change.References[planner.FieldEventGatewayBackendClusterID] = backendClusterRef
	}
	return nil
}

func (e *Executor) prepareEventGatewayVirtualClusterUpdate(ctx context.Context, change *planner.PlannedChange) error {
	if err := e.prepareEventGatewayReference(ctx, change); err != nil {
		return err
	}
	// Resolve event gateway backend cluster reference if needed
	if backendClusterRef, ok := change.References[planner.FieldEventGatewayBackendClusterID]; ok &&
		unresolvedReferenceID(backendClusterRef.ID) {
		backendClusterID, err := e.resolveEventGatewayBackendClusterRef(ctx, change.Parent.ID, backendClusterRef)
		if err != nil {
			return fmt.Errorf("failed to resolve event gateway backend cluster reference: %w", err)
		}
		backendClusterRef.ID = backendClusterID
		change.References[planner.FieldEventGatewayBackendClusterID] = backendClusterRef
	}
	return nil
}

func (e *Executor) prepareEventGatewayListenerPolicy(ctx context.Context, change *planner.PlannedChange) error {
	if err := e.prepareEventGatewayReference(ctx, change); err != nil {
		return err
	}
	// Resolve event gateway listener reference if needed
	if listenerRef, ok := change.References[planner.FieldEventGatewayListenerID]; ok && listenerRef.ID == "" {
		listenerID, err := e.resolveEventGatewayListenerRef(ctx, change, listenerRef)
		if err != nil {
			return fmt.Errorf("failed to resolve event gateway listener reference: %w", err)
		}
		listenerRef.ID = listenerID
		change.References[planner.FieldEventGatewayListenerID] = listenerRef
	}
	// Resolve event gateway virtual cluster reference if needed (for forward_to_virtual_cluster policies)
	if virtualClusterRef, ok := change.References[planner.FieldEventGatewayVirtualClusterID]; ok &&
		unresolvedReferenceID(virtualClusterRef.ID) {
		gatewayID := change.References[planner.FieldEventGatewayID].ID
		virtualClusterID, err := e.resolveEventGatewayVirtualClusterRef(ctx, gatewayID, virtualClusterRef)
		if err != nil {
			return fmt.Errorf("failed to resolve event gateway virtual cluster reference: %w", err)
		}
		virtualClusterRef.ID = virtualClusterID
		change.References[planner.FieldEventGatewayVirtualClusterID] = virtualClusterRef
	}
	return nil
}

func (e *Executor) prepareEventGatewayClusterPolicy(ctx context.Context, change *planner.PlannedChange) error {
	if err := e.prepareEventGatewayReference(ctx, change); err != nil {
		return err
	}
	// Resolve event gateway virtual cluster reference if needed
	if virtualClusterRef, ok := change.References[planner.FieldEventGatewayVirtualClusterID]; ok &&
		virtualClusterRef.ID == "" {
		gatewayID := change.References[planner.FieldEventGatewayID].ID
		virtualClusterID, err := e.resolveEventGatewayVirtualClusterRef(ctx, gatewayID, virtualClusterRef)
		if err != nil {
			return fmt.Errorf("failed to resolve event gateway virtual cluster reference: %w", err)
		}
		virtualClusterRef.ID = virtualClusterID
		change.References[planner.FieldEventGatewayVirtualClusterID] = virtualClusterRef
	}
	return nil
}

func (e *Executor) prepareEventGatewayProducePolicy(ctx context.Context, change *planner.PlannedChange) error {
	if err := e.prepareEventGatewayClusterPolicy(ctx, change); err != nil {
		return err
	}
	return e.syncResolvedEventGatewayProducePolicyConfigRefs(ctx, change)
}
