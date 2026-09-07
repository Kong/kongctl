package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
)

func (e *Executor) registerPortalExecutor() {
	e.registerResourceExecutor(prepareResourceWrites(crudResourceExecutor(
		NewManagedLabelBaseExecutor(NewPortalAdapter(e.client), e.client, e.dryRun),
	), e.syncResolvedPortalDefaultAuthStrategyID))
}

// registerPortalChildExecutors keeps singleton aliases, supported actions, and
// action-specific routing with the typed constructors and payload contracts.
func (e *Executor) registerPortalChildExecutors() {
	client, dryRun := e.client, e.dryRun
	registerChild := func(resource resourceExecutor) {
		e.registerResourceExecutor(prepareResourceExecutor(resource, e.preparePortalReference))
	}
	e.registerResourceExecutor(portalSingletonResourceExecutor(
		e,
		NewBaseSingletonExecutor(NewPortalCustomizationAdapter(client), dryRun),
	))
	e.registerResourceExecutor(portalSingletonResourceExecutor(
		e,
		NewBaseSingletonExecutor(NewPortalAuthSettingsAdapter(client), dryRun),
	))
	e.registerResourceExecutor(portalSingletonResourceExecutor(
		e,
		NewBaseSingletonExecutor(NewPortalIntegrationAdapter(client), dryRun),
	))
	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewPortalIdentityProviderAdapter(client), client, dryRun),
	))
	e.registerResourceExecutor(portalSingletonResourceExecutor(
		e,
		NewBaseSingletonExecutor(NewPortalAssetLogoAdapter(client), dryRun),
	))
	e.registerResourceExecutor(portalSingletonResourceExecutor(
		e,
		NewBaseSingletonExecutor(NewPortalAssetFaviconAdapter(client), dryRun),
	))

	domain := crudResourceExecutor(NewBaseExecutor(NewPortalDomainAdapter(client), client, dryRun))
	domain.create = prepareResourceWrite(domain.create, e.preparePortalReference)
	e.registerResourceExecutor(domain)

	registerChild(crudResourceExecutor(
		NewBaseExecutor(NewPortalIPAllowListAdapter(client), client, dryRun),
	))
	registerChild(prepareResourceWrites(crudResourceExecutor(
		NewBaseExecutor(NewPortalPageAdapter(client), client, dryRun),
	), e.preparePortalPageParentReference))
	registerChild(crudResourceExecutor(NewBaseExecutor(NewPortalSnippetAdapter(client), client, dryRun)))
	registerChild(crudResourceExecutor(NewBaseExecutor(NewPortalTeamAdapter(client), client, dryRun)))

	groupMapping := NewPortalTeamGroupMappingExecutor(client, dryRun)
	registerChild(prepareResourceWrites(resourceExecutor{
		contract: groupMapping,
		update: func(ctx context.Context, change *planner.PlannedChange) (string, error) {
			return groupMapping.Update(ctx, *change)
		},
	}, e.preparePortalTeamGroupMappingReference))

	teamRole := createDeleteResourceExecutor(NewBaseExecutor(NewPortalTeamRoleAdapter(client), client, dryRun))
	teamRole.create = prepareResourceWrite(teamRole.create, e.resolveRoleEntityRef)
	registerChild(prepareResourceExecutor(teamRole, e.preparePortalTeamReference))

	registerChild(crudResourceExecutor(NewBaseExecutor(NewPortalEmailConfigAdapter(client), client, dryRun)))
	registerChild(crudResourceExecutor(NewBaseExecutor(NewPortalAuditLogWebhookAdapter(client), client, dryRun)))
	registerChild(crudResourceExecutor(NewBaseExecutor(NewPortalEmailTemplateAdapter(client), client, dryRun)))
}

// Portal singleton CREATE and UPDATE both use the existing update operation.
// Keep the original payload contract and unsupported DELETE behavior.
func portalSingletonResourceExecutor[T any](e *Executor, base *BaseSingletonExecutor[T]) resourceExecutor {
	update := func(ctx context.Context, change *planner.PlannedChange) (string, error) {
		portalID, err := e.resolvePortalRef(ctx, change.References[planner.FieldPortalID])
		if err != nil {
			return "", err
		}
		return base.Update(ctx, *change, portalID)
	}
	return resourceExecutor{contract: base, create: update, update: update}
}

func (e *Executor) preparePortalReference(ctx context.Context, change *planner.PlannedChange) error {
	if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID == "" {
		portalID, err := e.resolvePortalRef(ctx, portalRef)
		if err != nil {
			return fmt.Errorf("failed to resolve portal reference: %w", err)
		}
		portalRef.ID = portalID
		change.References[planner.FieldPortalID] = portalRef
	}
	return nil
}

func (e *Executor) preparePortalPageParentReference(ctx context.Context, change *planner.PlannedChange) error {
	// Handle parent page reference resolution if needed
	if parentPageRef, ok := change.References[planner.FieldParentPageID]; ok && parentPageRef.ID == "" {
		portalID := change.References[planner.FieldPortalID].ID
		parentPageID, err := e.resolvePortalPageRef(ctx, portalID, parentPageRef.Ref, parentPageRef.LookupFields)
		if err != nil {
			return fmt.Errorf("failed to resolve parent page reference: %w", err)
		}
		// Create a new reference with the resolved ID
		parentPageRef.ID = parentPageID
		change.References[planner.FieldParentPageID] = parentPageRef
	}
	return nil
}

func (e *Executor) preparePortalTeamReference(ctx context.Context, change *planner.PlannedChange) error {
	if teamRef, ok := change.References[planner.FieldTeamID]; ok && teamRef.ID == "" {
		portalID := ""
		if portalInfo, exists := change.References[planner.FieldPortalID]; exists {
			portalID = portalInfo.ID
		}
		if portalID == "" && change.Parent != nil {
			portalID = change.Parent.ID
		}
		teamID, err := e.resolvePortalTeamRef(ctx, portalID, teamRef)
		if err != nil {
			return fmt.Errorf("failed to resolve portal team reference: %w", err)
		}
		teamRef.ID = teamID
		change.References[planner.FieldTeamID] = teamRef
	}
	return nil
}

func (e *Executor) preparePortalTeamGroupMappingReference(ctx context.Context, change *planner.PlannedChange) error {
	if teamRef, ok := change.References[planner.FieldTeamID]; ok && teamRef.ID == "" {
		portalID := ""
		if portalInfo, exists := change.References[planner.FieldPortalID]; exists {
			portalID = portalInfo.ID
		}
		if portalID == "" && change.Parent != nil {
			portalID = change.Parent.ID
		}
		teamID, err := e.resolvePortalTeamRef(ctx, portalID, teamRef)
		if err != nil {
			return fmt.Errorf("failed to resolve portal team reference: %w", err)
		}
		teamRef.ID = teamID
		change.References[planner.FieldTeamID] = teamRef
		change.Fields[planner.FieldTeamID] = teamID
		change.ResourceID = teamID
	}
	return nil
}
