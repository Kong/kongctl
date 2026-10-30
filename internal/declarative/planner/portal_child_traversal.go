package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
)

// planPortalChildren shares traversal while keeping parent-specific scope and
// error policies. createOnly also applies to unresolved external parents; an
// existing parent uses reconciliation policy even when its ID is empty.
func (p *portalPlannerImpl) planPortalChildren(
	ctx context.Context, plannerCtx *Config, desired resources.PortalResource,
	portalID string, createOnly bool, plan *Plan,
) error {
	planner := p.planner
	childInScope := func(rt resources.ResourceType, hasDesired bool) bool {
		if createOnly {
			return planner.shouldPlanChild(plan, resources.ResourceTypePortal, desired.Ref, rt)
		}
		return p.shouldPlanPortalChild(plan, desired, rt, hasDesired)
	}

	handleChildError := func(err error, label, createLabel string) error {
		if err == nil {
			return nil
		}
		if createOnly {
			planner.logger.Debug("Failed to plan portal "+createLabel+" for new portal",
				"portal", desired.Ref, "error", err.Error())
			return nil
		}
		return fmt.Errorf("failed to plan portal %s changes: %w", label, err)
	}

	// Extract parent namespace for child resources
	parentNamespace := DefaultNamespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		parentNamespace = *desired.Kongctl.Namespace
	}

	// Plan pages - pass empty array if no pages defined
	pages := portalChildrenForRef(planner.desiredPortalPages, desired.Ref,
		func(item *resources.PortalPageResource) *string { return &item.Portal })
	if childInScope(resources.ResourceTypePortalPage, len(pages) > 0) {
		if err := handleChildError(planner.planPortalPagesChanges(
			ctx, parentNamespace, portalID, desired.Ref, pages, plan,
		), "page", "pages"); err != nil {
			return err
		}
	}

	// Plan snippets - pass empty array if no snippets defined
	snippets := portalChildrenForRef(planner.desiredPortalSnippets, desired.Ref,
		func(item *resources.PortalSnippetResource) *string { return &item.Portal })
	if childInScope(resources.ResourceTypePortalSnippet, len(snippets) > 0) {
		if err := handleChildError(planner.planPortalSnippetsChanges(
			ctx, parentNamespace, portalID, desired.Ref, snippets, plan,
		), "snippet", "snippets"); err != nil {
			return err
		}
	}

	// Plan IP allow list (singleton)
	allowLists := portalChildrenForRef(planner.desiredPortalIPAllowLists, desired.Ref,
		func(item *resources.PortalIPAllowListResource) *string { return &item.Portal })
	if childInScope(resources.ResourceTypePortalIPAllowList, len(allowLists) > 0) {
		if err := handleChildError(planner.planPortalIPAllowListsChanges(
			ctx, parentNamespace, portalID, desired.Ref, allowLists, plan,
		), "IP allow list", "IP allow list"); err != nil {
			return err
		}
	}

	// Plan integrations (singleton)
	integrations := portalChildrenForRef(planner.desiredPortalIntegrations, desired.Ref,
		func(item *resources.PortalIntegrationResource) *string { return &item.Portal })
	if childInScope(resources.ResourceTypePortalIntegration, len(integrations) > 0) {
		if err := handleChildError(planner.planPortalIntegrationsChanges(
			ctx, parentNamespace, portalID, desired.Ref, integrations, plan,
		), "integration", "integrations"); err != nil {
			return err
		}
	}

	// Plan identity providers
	identityProviders := portalChildrenForRef(planner.desiredPortalIdentityProviders, desired.Ref,
		func(item *resources.PortalIdentityProviderResource) *string { return &item.Portal })
	if childInScope(resources.ResourceTypePortalIdentityProvider, len(identityProviders) > 0) {
		if err := handleChildError(planner.planPortalIdentityProvidersChanges(
			ctx,
			parentNamespace,
			portalID,
			desired.Ref,
			identityProviders,
			plan,
		), "identity provider", "identity providers"); err != nil {
			return err
		}
	}

	// Plan auth settings after identity providers so idp_mapping_enabled can depend on provider enablement.
	authSettings := portalChildrenForRef(planner.desiredPortalAuthSettings, desired.Ref,
		func(item *resources.PortalAuthSettingsResource) *string { return &item.Portal })
	if childInScope(resources.ResourceTypePortalAuthSettings, len(authSettings) > 0) {
		if err := handleChildError(planner.planPortalAuthSettingsChanges(
			ctx, plannerCtx, parentNamespace, authSettings, plan,
		), "auth settings", "auth settings"); err != nil {
			return err
		}
	}

	// Plan email config (singleton resource)
	emailConfigs := portalChildrenForRef(planner.desiredPortalEmailConfigs, desired.Ref,
		func(item *resources.PortalEmailConfigResource) *string { return &item.Portal })
	if childInScope(resources.ResourceTypePortalEmailConfig, len(emailConfigs) > 0) {
		if err := handleChildError(planner.planPortalEmailConfigsChanges(
			ctx, parentNamespace, portalID, desired.Ref, emailConfigs, plan,
		), "email config", "email config"); err != nil {
			return err
		}
	}

	// Plan audit-log webhook (singleton resource)
	auditLogWebhooks := portalChildrenForRef(planner.desiredPortalAuditLogWebhooks, desired.Ref,
		func(item *resources.PortalAuditLogWebhookResource) *string { return &item.Portal })
	if childInScope(resources.ResourceTypePortalAuditLogWebhook, len(auditLogWebhooks) > 0) {
		if err := handleChildError(planner.planPortalAuditLogWebhooksChanges(
			ctx, parentNamespace, portalID, desired.Ref, auditLogWebhooks, plan,
		), "audit-log webhook", "audit-log webhook"); err != nil {
			return err
		}
	}

	// Plan email templates (set applies create/update only in apply mode)
	templates := portalChildrenForRef(planner.desiredPortalEmailTemplates, desired.Ref,
		func(item *resources.PortalEmailTemplateResource) *string { return &item.Portal })
	if childInScope(resources.ResourceTypePortalEmailTemplate, len(templates) > 0) {
		if err := handleChildError(planner.planPortalEmailTemplatesChanges(
			ctx, parentNamespace, portalID, desired.Ref, desired.Name, templates, plan,
		), "email template", "email templates"); err != nil {
			return err
		}
	}

	// Plan customization (singleton resource)
	customizations := portalChildrenForRef(planner.desiredPortalCustomizations, desired.Ref,
		func(item *resources.PortalCustomizationResource) *string { return &item.Portal })
	if childInScope(resources.ResourceTypePortalCustomization, len(customizations) > 0) {
		if err := handleChildError(planner.planPortalCustomizationsChanges(
			ctx, plannerCtx, parentNamespace, customizations, plan,
		), "customization", "customizations"); err != nil {
			return err
		}
	}

	// Plan teams
	teams := portalChildrenForRef(planner.desiredPortalTeams, desired.Ref,
		func(item *resources.PortalTeamResource) *string { return &item.Portal })
	if childInScope(resources.ResourceTypePortalTeam, len(teams) > 0) {
		if err := handleChildError(planner.planPortalTeamsChanges(
			ctx, parentNamespace, portalID, desired.Ref, teams, plan,
		), "team", "teams"); err != nil {
			return err
		}
	}

	mappings := portalChildrenForRef(planner.desiredPortalTeamGroupMappings, desired.Ref,
		func(item *resources.PortalTeamGroupMappingResource) *string { return &item.Portal })
	if childInScope(resources.ResourceTypePortalTeamGroupMapping, len(mappings) > 0) {
		if err := handleChildError(planner.planPortalTeamGroupMappingsChanges(
			ctx, parentNamespace, portalID, desired.Ref, mappings, plan,
		), "team group mapping", "team group mappings"); err != nil {
			return err
		}
	}

	hasTeamRoles := false
	for _, role := range planner.desiredPortalTeamRoles {
		role.Portal = resources.NormalizeResourceRef(role.Portal)
		if role.Portal == desired.Ref {
			hasTeamRoles = true
			break
		}
	}
	if childInScope(resources.ResourceTypePortalTeamRole, hasTeamRoles) {
		if err := handleChildError(planner.planPortalTeamRolesChanges(
			ctx, parentNamespace, portalID, desired.Ref, desired.Name, plan,
		), "team role", "team roles"); err != nil {
			return err
		}
	}

	// Plan custom domain (singleton resource)
	domains := portalChildrenForRef(planner.desiredPortalCustomDomains, desired.Ref,
		func(item *resources.PortalCustomDomainResource) *string { return &item.Portal })
	if childInScope(resources.ResourceTypePortalCustomDomain, len(domains) > 0) {
		if err := handleChildError(planner.planPortalCustomDomainsChanges(
			ctx,
			parentNamespace,
			portalID,
			desired.Ref,
			domains,
			plan,
		), "custom domain", "custom domains"); err != nil {
			return err
		}
	}

	// Plan asset logo (singleton resource)
	logos := portalChildrenForRef(planner.desiredPortalAssetLogos, desired.Ref,
		func(item *resources.PortalAssetLogoResource) *string { return &item.Portal })
	if childInScope(resources.ResourceTypePortalAssetLogo, len(logos) > 0) {
		if err := handleChildError(planner.planPortalAssetLogosChanges(
			ctx, plannerCtx, parentNamespace, logos, plan,
		), "asset logo", "asset logos"); err != nil {
			return err
		}
	}

	// Plan asset favicon (singleton resource)
	favicons := portalChildrenForRef(planner.desiredPortalAssetFavicons, desired.Ref,
		func(item *resources.PortalAssetFaviconResource) *string { return &item.Portal })
	if childInScope(resources.ResourceTypePortalAssetFavicon, len(favicons) > 0) {
		if err := handleChildError(planner.planPortalAssetFaviconsChanges(
			ctx, plannerCtx, parentNamespace, favicons, plan,
		), "asset favicon", "asset favicons"); err != nil {
			return err
		}
	}

	// Plan nested assets (from portal.Assets.Logo and portal.Assets.Favicon fields)
	if !desired.IsExternal() && desired.Assets != nil {
		if desired.Assets.Logo != nil && *desired.Assets.Logo != "" {
			if childInScope(resources.ResourceTypePortalAssetLogo, true) &&
				!plan.HasChange(ResourceTypePortalAssetLogo, fmt.Sprintf("%s-logo", desired.Ref)) {
				needsUpdate, currentDataURL := true, ""
				if !createOnly {
					var err error
					needsUpdate, currentDataURL, err = planner.portalAssetNeedsUpdate(
						ctx,
						portalID,
						*desired.Assets.Logo,
						planner.client.GetPortalAssetLogo,
					)
					if err != nil {
						return fmt.Errorf("failed to compare portal asset logo for portal %q: %w", desired.Ref, err)
					}
				}
				if needsUpdate {
					planner.planPortalAssetLogoUpdate(
						parentNamespace,
						desired.Ref,
						desired.Name,
						portalID,
						*desired.Assets.Logo,
						currentDataURL,
						plan,
					)
				} else {
					planner.logger.Debug(
						"Skipping portal asset logo update; no changes detected",
						"portal", desired.Ref,
					)
				}
			}
		}
		if desired.Assets.Favicon != nil && *desired.Assets.Favicon != "" {
			if childInScope(resources.ResourceTypePortalAssetFavicon, true) &&
				!plan.HasChange(ResourceTypePortalAssetFavicon, fmt.Sprintf("%s-favicon", desired.Ref)) {
				needsUpdate, currentDataURL := true, ""
				if !createOnly {
					var err error
					needsUpdate, currentDataURL, err = planner.portalAssetNeedsUpdate(
						ctx,
						portalID,
						*desired.Assets.Favicon,
						planner.client.GetPortalAssetFavicon,
					)
					if err != nil {
						return fmt.Errorf("failed to compare portal asset favicon for portal %q: %w", desired.Ref, err)
					}
				}
				if needsUpdate {
					planner.planPortalAssetFaviconUpdate(
						parentNamespace,
						desired.Ref,
						desired.Name,
						portalID,
						*desired.Assets.Favicon,
						currentDataURL,
						plan,
					)
				} else {
					planner.logger.Debug(
						"Skipping portal asset favicon update; no changes detected",
						"portal", desired.Ref,
					)
				}
			}
		}
	}

	return nil
}

func (p *portalPlannerImpl) shouldPlanPortalChild(
	plan *Plan,
	desired resources.PortalResource,
	rt resources.ResourceType,
	hasDesired bool,
) bool {
	if desired.IsExternal() && plan != nil && plan.Metadata.Mode != PlanModeSync {
		return hasDesired
	}
	return p.planner.shouldPlanChild(plan, resources.ResourceTypePortal, desired.Ref, rt)
}

func portalChildrenForRef[T any](items []T, portalRef string, parentRef func(*T) *string) []T {
	children := make([]T, 0)
	for _, item := range items {
		parent := parentRef(&item)
		*parent = resources.NormalizeResourceRef(*parent)
		if *parent == portalRef {
			children = append(children, item)
		}
	}
	return children
}
