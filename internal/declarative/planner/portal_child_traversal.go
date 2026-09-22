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
	pages := make([]resources.PortalPageResource, 0)
	// Note: Pages have already been extracted to root level by loader
	// We need to find pages that belong to this portal
	for _, page := range planner.desiredPortalPages {
		if page.Portal == desired.Ref {
			pages = append(pages, page)
		}
	}
	if childInScope(resources.ResourceTypePortalPage, len(pages) > 0) {
		if err := handleChildError(planner.planPortalPagesChanges(
			ctx, parentNamespace, portalID, desired.Ref, pages, plan,
		), "page", "pages"); err != nil {
			return err
		}
	}

	// Plan snippets - pass empty array if no snippets defined
	snippets := make([]resources.PortalSnippetResource, 0)
	// Note: Snippets have already been extracted to root level by loader
	// We need to find snippets that belong to this portal
	for _, snippet := range planner.desiredPortalSnippets {
		if snippet.Portal == desired.Ref {
			snippets = append(snippets, snippet)
		}
	}
	if childInScope(resources.ResourceTypePortalSnippet, len(snippets) > 0) {
		if err := handleChildError(planner.planPortalSnippetsChanges(
			ctx, parentNamespace, portalID, desired.Ref, snippets, plan,
		), "snippet", "snippets"); err != nil {
			return err
		}
	}

	// Plan IP allow list (singleton)
	allowLists := make([]resources.PortalIPAllowListResource, 0)
	for _, allowList := range planner.desiredPortalIPAllowLists {
		if allowList.Portal == desired.Ref {
			allowLists = append(allowLists, allowList)
		}
	}
	if childInScope(resources.ResourceTypePortalIPAllowList, len(allowLists) > 0) {
		if err := handleChildError(planner.planPortalIPAllowListsChanges(
			ctx, parentNamespace, portalID, desired.Ref, allowLists, plan,
		), "IP allow list", "IP allow list"); err != nil {
			return err
		}
	}

	// Plan integrations (singleton)
	integrations := make([]resources.PortalIntegrationResource, 0)
	for _, integration := range planner.desiredPortalIntegrations {
		if integration.Portal == desired.Ref {
			integrations = append(integrations, integration)
		}
	}
	if childInScope(resources.ResourceTypePortalIntegration, len(integrations) > 0) {
		if err := handleChildError(planner.planPortalIntegrationsChanges(
			ctx, parentNamespace, portalID, desired.Ref, integrations, plan,
		), "integration", "integrations"); err != nil {
			return err
		}
	}

	// Plan identity providers
	identityProviders := make([]resources.PortalIdentityProviderResource, 0)
	for _, provider := range planner.desiredPortalIdentityProviders {
		if provider.Portal == desired.Ref {
			identityProviders = append(identityProviders, provider)
		}
	}
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
	authSettings := make([]resources.PortalAuthSettingsResource, 0)
	for _, auth := range planner.desiredPortalAuthSettings {
		if auth.Portal == desired.Ref {
			authSettings = append(authSettings, auth)
		}
	}
	if childInScope(resources.ResourceTypePortalAuthSettings, len(authSettings) > 0) {
		if err := handleChildError(planner.planPortalAuthSettingsChanges(
			ctx, plannerCtx, parentNamespace, authSettings, plan,
		), "auth settings", "auth settings"); err != nil {
			return err
		}
	}

	// Plan email config (singleton resource)
	emailConfigs := make([]resources.PortalEmailConfigResource, 0)
	for _, cfg := range planner.desiredPortalEmailConfigs {
		if cfg.Portal == desired.Ref {
			emailConfigs = append(emailConfigs, cfg)
		}
	}
	if childInScope(resources.ResourceTypePortalEmailConfig, len(emailConfigs) > 0) {
		if err := handleChildError(planner.planPortalEmailConfigsChanges(
			ctx, parentNamespace, portalID, desired.Ref, emailConfigs, plan,
		), "email config", "email config"); err != nil {
			return err
		}
	}

	// Plan audit-log webhook (singleton resource)
	auditLogWebhooks := make([]resources.PortalAuditLogWebhookResource, 0)
	for _, webhook := range planner.desiredPortalAuditLogWebhooks {
		if webhook.Portal == desired.Ref {
			auditLogWebhooks = append(auditLogWebhooks, webhook)
		}
	}
	if childInScope(resources.ResourceTypePortalAuditLogWebhook, len(auditLogWebhooks) > 0) {
		if err := handleChildError(planner.planPortalAuditLogWebhooksChanges(
			ctx, parentNamespace, portalID, desired.Ref, auditLogWebhooks, plan,
		), "audit-log webhook", "audit-log webhook"); err != nil {
			return err
		}
	}

	// Plan email templates (set applies create/update only in apply mode)
	templates := make([]resources.PortalEmailTemplateResource, 0)
	for _, tpl := range planner.desiredPortalEmailTemplates {
		if tpl.Portal == desired.Ref {
			templates = append(templates, tpl)
		}
	}
	if childInScope(resources.ResourceTypePortalEmailTemplate, len(templates) > 0) {
		if err := handleChildError(planner.planPortalEmailTemplatesChanges(
			ctx, parentNamespace, portalID, desired.Ref, desired.Name, templates, plan,
		), "email template", "email templates"); err != nil {
			return err
		}
	}

	// Plan customization (singleton resource)
	customizations := make([]resources.PortalCustomizationResource, 0)
	for _, customization := range planner.desiredPortalCustomizations {
		if customization.Portal == desired.Ref {
			customizations = append(customizations, customization)
		}
	}
	if childInScope(resources.ResourceTypePortalCustomization, len(customizations) > 0) {
		if err := handleChildError(planner.planPortalCustomizationsChanges(
			ctx, plannerCtx, parentNamespace, customizations, plan,
		), "customization", "customizations"); err != nil {
			return err
		}
	}

	// Plan teams
	teams := make([]resources.PortalTeamResource, 0)
	for _, team := range planner.desiredPortalTeams {
		if team.Portal == desired.Ref {
			teams = append(teams, team)
		}
	}
	if childInScope(resources.ResourceTypePortalTeam, len(teams) > 0) {
		if err := handleChildError(planner.planPortalTeamsChanges(
			ctx, parentNamespace, portalID, desired.Ref, teams, plan,
		), "team", "teams"); err != nil {
			return err
		}
	}

	mappings := make([]resources.PortalTeamGroupMappingResource, 0)
	for _, mapping := range planner.desiredPortalTeamGroupMappings {
		if mapping.Portal == desired.Ref {
			mappings = append(mappings, mapping)
		}
	}
	if childInScope(resources.ResourceTypePortalTeamGroupMapping, len(mappings) > 0) {
		if err := handleChildError(planner.planPortalTeamGroupMappingsChanges(
			ctx, parentNamespace, portalID, desired.Ref, mappings, plan,
		), "team group mapping", "team group mappings"); err != nil {
			return err
		}
	}

	hasTeamRoles := false
	for _, role := range planner.desiredPortalTeamRoles {
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
	domains := make([]resources.PortalCustomDomainResource, 0)
	for _, domain := range planner.desiredPortalCustomDomains {
		if domain.Portal == desired.Ref {
			domains = append(domains, domain)
		}
	}
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
	logos := make([]resources.PortalAssetLogoResource, 0)
	for _, logo := range planner.desiredPortalAssetLogos {
		if logo.Portal == desired.Ref {
			logos = append(logos, logo)
		}
	}
	if childInScope(resources.ResourceTypePortalAssetLogo, len(logos) > 0) {
		if err := handleChildError(planner.planPortalAssetLogosChanges(
			ctx, plannerCtx, parentNamespace, logos, plan,
		), "asset logo", "asset logos"); err != nil {
			return err
		}
	}

	// Plan asset favicon (singleton resource)
	favicons := make([]resources.PortalAssetFaviconResource, 0)
	for _, favicon := range planner.desiredPortalAssetFavicons {
		if favicon.Portal == desired.Ref {
			favicons = append(favicons, favicon)
		}
	}
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
