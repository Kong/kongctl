package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
)

func (e *Executor) registerAPIExecutor() {
	e.registerResourceExecutor(crudResourceExecutor(
		NewManagedLabelBaseExecutor(NewAPIAdapter(e.client), e.client, e.dryRun),
	))
}

func (e *Executor) registerAPIChildExecutors() {
	client, dryRun := e.client, e.dryRun
	e.registerResourceExecutor(prepareResourceWrites(crudResourceExecutor(
		NewBaseExecutor(NewAPIVersionAdapter(client), client, dryRun),
	), e.prepareAPIReference))

	// Publication PUT uses the create mapping and operation for both actions.
	publicationBase := NewBaseCreateDeleteExecutor(NewAPIPublicationAdapter(client), dryRun)
	publication := createDeleteResourceExecutor(publicationBase)
	publication.contract = upsertPayloadContract[kkComps.APIPublication]{base: publicationBase}
	publication.update = publication.create
	e.registerResourceExecutor(prepareResourceWrites(publication, e.prepareAPIPublicationReferences))

	document := prepareResourceWrites(crudResourceExecutor(
		NewBaseExecutor(NewAPIDocumentAdapter(client), client, dryRun),
	), e.prepareAPIParentDocumentReference)
	e.registerResourceExecutor(prepareResourceExecutor(document, e.prepareAPIReference))

	e.registerResourceExecutor(prepareResourceWrites(createDeleteResourceExecutor(
		NewBaseCreateDeleteExecutor(NewAPIImplementationAdapter(client), dryRun),
	), e.prepareAPIReference))
}

func (e *Executor) prepareAPIPublicationReferences(ctx context.Context, change *planner.PlannedChange) error {
	if err := e.prepareAPIReference(ctx, change); err != nil {
		return err
	}
	if err := e.preparePortalReference(ctx, change); err != nil {
		return err
	}
	return e.syncResolvedAPIPublicationAuthStrategyIDs(ctx, change)
}

func (e *Executor) prepareAPIReference(ctx context.Context, change *planner.PlannedChange) error {
	// First resolve API reference if needed
	if apiRef, ok := change.References[planner.FieldAPIID]; ok && apiRef.ID == "" {
		apiID, err := e.resolveAPIRef(ctx, apiRef)
		if err != nil {
			return fmt.Errorf("failed to resolve API reference: %w", err)
		}
		// Update the reference with the resolved ID
		apiRef.ID = apiID
		change.References[planner.FieldAPIID] = apiRef
	}
	return nil
}

func (e *Executor) prepareAPIParentDocumentReference(ctx context.Context, change *planner.PlannedChange) error {
	if parentRef, ok := change.References[planner.FieldParentDocumentID]; ok &&
		parentRef.Ref != "" && parentRef.ID == "" {
		apiID := ""
		if apiInfo, exists := change.References[planner.FieldAPIID]; exists {
			apiID = apiInfo.ID
		}
		if apiID == "" && change.Parent != nil {
			apiID = change.Parent.ID
		}
		resolvedParentID, err := e.resolveAPIDocumentRef(ctx, apiID, parentRef)
		if err != nil {
			return fmt.Errorf("failed to resolve parent document reference: %w", err)
		}
		parentRef.ID = resolvedParentID
		change.References[planner.FieldParentDocumentID] = parentRef
	}
	return nil
}
