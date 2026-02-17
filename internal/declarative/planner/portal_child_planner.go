package planner

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkErrors "github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
)

// Portal Customization planning

func (p *Planner) planPortalCustomizationsChanges(
	ctx context.Context, plannerCtx *Config, parentNamespace string,
	desired []resources.PortalCustomizationResource, plan *Plan,
) error { //nolint:unparam // Will return errors in future enhancements
	// Get existing portals to check current customization
	// Use planner context to get namespace filter for API calls
	namespace := plannerCtx.Namespace
	namespaceFilter := []string{namespace}
	existingPortals, _ := p.client.ListManagedPortals(ctx, namespaceFilter)
	portalNameToID := make(map[string]string)
	for _, portal := range existingPortals {
		portalNameToID[portal.Name] = portal.ID
	}

	// For each desired customization
	for _, desiredCustomization := range desired {
		if plan.HasChange("portal_customization", desiredCustomization.GetRef()) {
			continue
		}
		// Find the portal ID
		var portalID string
		var portalName string
		for _, portal := range p.desiredPortals {
			if portal.Ref == desiredCustomization.Portal {
				portalName = portal.Name
				portalID = portalNameToID[portalName]
				break
			}
		}

		// If portal exists, fetch current customization and compare
		if portalID != "" {
			current, err := p.client.GetPortalCustomization(ctx, portalID)
			if err != nil {
				// If portal customization API is not configured, skip processing
				if strings.Contains(err.Error(), "portal customization API not configured") {
					continue
				}
				// If we can't fetch current state, plan the update anyway
				p.planPortalCustomizationUpdate(parentNamespace, desiredCustomization, portalName, portalID, plan)
				continue
			}

			// Compare and only update if needed
			needsUpdate, updateFields := p.shouldUpdatePortalCustomization(current, desiredCustomization)
			if needsUpdate {
				p.planPortalCustomizationUpdateWithFields(
					parentNamespace, desiredCustomization, portalName, portalID, updateFields, plan,
				)
			}
		} else {
			// Portal doesn't exist yet, plan the update for after portal creation
			p.planPortalCustomizationUpdate(parentNamespace, desiredCustomization, portalName, "", plan)
		}
	}

	return nil
}

// Portal Auth Settings planning (singleton)

func (p *Planner) planPortalAuthSettingsChanges(
	ctx context.Context, plannerCtx *Config, parentNamespace string,
	desired []resources.PortalAuthSettingsResource, plan *Plan,
) error {
	namespace := plannerCtx.Namespace
	existingPortals, _ := p.client.ListManagedPortals(ctx, []string{namespace})
	portalNameToID := make(map[string]string)
	for _, portal := range existingPortals {
		portalNameToID[portal.Name] = portal.ID
	}

	for _, desiredSettings := range desired {
		if plan.HasChange(ResourceTypePortalAuthSettings, desiredSettings.GetRef()) {
			continue
		}

		var portalName, portalID string
		for _, portal := range p.desiredPortals {
			if portal.Ref == desiredSettings.Portal {
				portalName = portal.Name
				portalID = portalNameToID[portalName]
				break
			}
		}

		if portalID == "" {
			p.planPortalAuthSettingsUpdate(parentNamespace, desiredSettings, portalName, "", plan)
			continue
		}

		current, err := p.client.GetPortalAuthSettings(ctx, portalID)
		if err != nil {
			return fmt.Errorf("failed to fetch portal auth settings for portal_ref %s: %w",
				desiredSettings.Portal, err)
		}

		needsUpdate, updateFields := p.shouldUpdatePortalAuthSettings(current, desiredSettings)
		if needsUpdate {
			p.planPortalAuthSettingsUpdateWithFields(
				parentNamespace, desiredSettings, portalName, portalID, updateFields, plan,
			)
		}
	}

	return nil
}

func (p *Planner) planPortalAuthSettingsUpdate(
	parentNamespace string, settings resources.PortalAuthSettingsResource, portalName string, portalID string,
	plan *Plan,
) {
	fields := p.buildAllPortalAuthSettingsFields(settings)
	p.planPortalAuthSettingsUpdateWithFields(parentNamespace, settings, portalName, portalID, fields, plan)
}

func (p *Planner) planPortalAuthSettingsUpdateWithFields(
	parentNamespace string, settings resources.PortalAuthSettingsResource, portalName string, portalID string,
	fields map[string]any, plan *Plan,
) {
	if len(fields) == 0 {
		return
	}

	var dependencies []string
	if settings.Portal != "" {
		for _, change := range plan.Changes {
			if change.ResourceType == ResourceTypePortal && change.ResourceRef == settings.Portal {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypePortalAuthSettings, settings.Ref),
		ResourceType: ResourceTypePortalAuthSettings,
		ResourceRef:  settings.Ref,
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    dependencies,
		Namespace:    parentNamespace,
	}

	if settings.Portal != "" {
		lookupName := portalName
		if lookupName == "" {
			lookupName = settings.Portal
		}
		change.Parent = &ParentInfo{
			Ref: settings.Portal,
			ID:  portalID,
		}
		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: settings.Portal,
				LookupFields: map[string]string{
					"name": lookupName,
				},
			},
		}
	}

	p.logger.Debug("Enqueuing portal auth settings update",
		"portal_ref", settings.Portal,
		"settings_ref", settings.Ref,
		"fields", fields,
	)
	plan.AddChange(change)
}

func (p *Planner) shouldUpdatePortalAuthSettings(
	current *kkComps.PortalAuthenticationSettingsResponse,
	desired resources.PortalAuthSettingsResource,
) (bool, map[string]any) {
	updates := make(map[string]any)

	if desired.BasicAuthEnabled != nil && !p.compareBoolToPtr(current.BasicAuthEnabled, desired.BasicAuthEnabled) {
		updates["basic_auth_enabled"] = *desired.BasicAuthEnabled
	}

	if desired.OidcAuthEnabled != nil && !p.compareBoolToPtr(current.OidcAuthEnabled, desired.OidcAuthEnabled) {
		updates["oidc_auth_enabled"] = *desired.OidcAuthEnabled
	}

	if desired.SamlAuthEnabled != nil && !p.comparePtrBools(current.SamlAuthEnabled, desired.SamlAuthEnabled) {
		updates["saml_auth_enabled"] = *desired.SamlAuthEnabled
	}

	if desired.OidcTeamMappingEnabled != nil &&
		!p.compareBoolToPtr(current.OidcTeamMappingEnabled, desired.OidcTeamMappingEnabled) {
		updates["oidc_team_mapping_enabled"] = *desired.OidcTeamMappingEnabled
	}

	if desired.KonnectMappingEnabled != nil &&
		!p.compareBoolToPtr(current.KonnectMappingEnabled, desired.KonnectMappingEnabled) {
		updates["konnect_mapping_enabled"] = *desired.KonnectMappingEnabled
	}

	if desired.IdpMappingEnabled != nil && !p.comparePtrBools(current.IdpMappingEnabled, desired.IdpMappingEnabled) {
		updates["idp_mapping_enabled"] = *desired.IdpMappingEnabled
	}

	if desired.OidcIssuer != nil {
		if current.OidcConfig == nil || current.OidcConfig.Issuer != *desired.OidcIssuer {
			updates["oidc_issuer"] = *desired.OidcIssuer
		}
	}

	if desired.OidcClientID != nil {
		if current.OidcConfig == nil || current.OidcConfig.ClientID != *desired.OidcClientID {
			updates["oidc_client_id"] = *desired.OidcClientID
		}
	}

	if desired.OidcClientSecret != nil {
		updates["oidc_client_secret"] = *desired.OidcClientSecret
	}

	if desired.OidcScopes != nil {
		if current.OidcConfig == nil || !p.compareStringSlices(current.OidcConfig.Scopes, desired.OidcScopes) {
			updates["oidc_scopes"] = desired.OidcScopes
		}
	}

	if desired.OidcClaimMappings != nil {
		if current.OidcConfig == nil ||
			!reflect.DeepEqual(current.OidcConfig.ClaimMappings, desired.OidcClaimMappings) {
			updates["oidc_claim_mappings"] = desired.OidcClaimMappings
		}
	}

	return len(updates) > 0, updates
}

func (p *Planner) buildAllPortalAuthSettingsFields(settings resources.PortalAuthSettingsResource) map[string]any {
	fields := make(map[string]any)

	if settings.BasicAuthEnabled != nil {
		fields["basic_auth_enabled"] = *settings.BasicAuthEnabled
	}
	if settings.OidcAuthEnabled != nil {
		fields["oidc_auth_enabled"] = *settings.OidcAuthEnabled
	}
	if settings.SamlAuthEnabled != nil {
		fields["saml_auth_enabled"] = *settings.SamlAuthEnabled
	}
	if settings.OidcTeamMappingEnabled != nil {
		fields["oidc_team_mapping_enabled"] = *settings.OidcTeamMappingEnabled
	}
	if settings.KonnectMappingEnabled != nil {
		fields["konnect_mapping_enabled"] = *settings.KonnectMappingEnabled
	}
	if settings.IdpMappingEnabled != nil {
		fields["idp_mapping_enabled"] = *settings.IdpMappingEnabled
	}
	if settings.OidcIssuer != nil {
		fields["oidc_issuer"] = *settings.OidcIssuer
	}
	if settings.OidcClientID != nil {
		fields["oidc_client_id"] = *settings.OidcClientID
	}
	if settings.OidcClientSecret != nil {
		fields["oidc_client_secret"] = *settings.OidcClientSecret
	}
	if settings.OidcScopes != nil {
		fields["oidc_scopes"] = settings.OidcScopes
	}
	if settings.OidcClaimMappings != nil {
		fields["oidc_claim_mappings"] = map[string]any{
			"name":   safeString(settings.OidcClaimMappings.Name),
			"email":  safeString(settings.OidcClaimMappings.Email),
			"groups": safeString(settings.OidcClaimMappings.Groups),
		}
	}

	return fields
}

func safeString(val *string) string {
	if val != nil {
		return *val
	}
	return ""
}

func (p *Planner) planPortalCustomizationUpdate(
	parentNamespace string, customization resources.PortalCustomizationResource,
	portalName string, portalID string, plan *Plan,
) {
	// Build all fields from the resource
	fields := p.buildAllCustomizationFields(customization)
	p.planPortalCustomizationUpdateWithFields(parentNamespace, customization, portalName, portalID, fields, plan)
}

func (p *Planner) planPortalCustomizationUpdateWithFields(
	parentNamespace string, customization resources.PortalCustomizationResource, portalName string, portalID string,
	fields map[string]any, plan *Plan,
) {
	// Only proceed if there are fields to update
	if len(fields) == 0 {
		return
	}

	// Determine dependencies - depends on parent portal
	var dependencies []string
	if customization.Portal != "" {
		// Find the change ID for the parent portal
		for _, change := range plan.Changes {
			if change.ResourceType == ResourceTypePortal && change.ResourceRef == customization.Portal {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	// Portal customization is a singleton resource - always use UPDATE action
	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypePortalCustomization, customization.Ref),
		ResourceType: ResourceTypePortalCustomization,
		ResourceRef:  customization.Ref,
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    dependencies,
		Namespace:    parentNamespace,
	}

	// Store parent portal reference
	if customization.Portal != "" {
		// Set Parent field for proper display and serialization
		change.Parent = &ParentInfo{
			Ref: customization.Portal,
			ID:  portalID, // May be empty if portal doesn't exist yet
		}

		// Also store in References for executor to use
		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: customization.Portal,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
	}

	plan.AddChange(change)
}

// shouldUpdatePortalCustomization compares current and desired customization
func (p *Planner) shouldUpdatePortalCustomization(
	current *kkComps.PortalCustomization,
	desired resources.PortalCustomizationResource,
) (bool, map[string]any) {
	updates := make(map[string]any)

	// Compare theme
	if !p.compareTheme(current.Theme, desired.Theme) {
		if desired.Theme != nil {
			updates["theme"] = p.buildThemeFields(desired.Theme)
		}
	}

	// Compare layout
	if !p.compareStringPtr(current.Layout, desired.Layout) {
		if desired.Layout != nil {
			updates["layout"] = *desired.Layout
		}
	}

	// Compare CSS
	if !p.compareStringPtr(current.CSS, desired.CSS) {
		if desired.CSS != nil {
			updates["css"] = *desired.CSS
		}
	}

	// Compare menu
	if !p.compareMenu(current.Menu, desired.Menu) {
		if desired.Menu != nil {
			updates["menu"] = p.buildMenuFields(desired.Menu)
		}
	}

	return len(updates) > 0, updates
}

// buildAllCustomizationFields builds all fields from the customization resource
func (p *Planner) buildAllCustomizationFields(
	customization resources.PortalCustomizationResource,
) map[string]any {
	fields := make(map[string]any)

	// Add theme settings if present
	if customization.Theme != nil {
		fields["theme"] = p.buildThemeFields(customization.Theme)
	}

	// Add layout if present
	if customization.Layout != nil {
		fields["layout"] = *customization.Layout
	}

	// Add CSS if present
	if customization.CSS != nil {
		fields["css"] = *customization.CSS
	}

	// Add menu settings if present
	if customization.Menu != nil {
		fields["menu"] = p.buildMenuFields(customization.Menu)
	}

	return fields
}

// buildThemeFields constructs theme fields map from theme object
func (p *Planner) buildThemeFields(theme *kkComps.Theme) map[string]any {
	themeFields := make(map[string]any)

	// Add mode if present
	if theme.Mode != nil {
		themeFields["mode"] = string(*theme.Mode)
	}

	// Add name if present
	if theme.Name != nil {
		themeFields["name"] = *theme.Name
	}

	// Add colors if present
	if theme.Colors != nil {
		colorsFields := make(map[string]any)
		if theme.Colors.Primary != nil {
			colorsFields["primary"] = *theme.Colors.Primary
		}
		themeFields["colors"] = colorsFields
	}

	return themeFields
}

// buildMenuFields constructs menu fields map from menu object
func (p *Planner) buildMenuFields(menu *kkComps.Menu) map[string]any {
	menuFields := make(map[string]any)

	// Add main menu items
	if menu.Main != nil {
		var mainMenuItems []map[string]any
		for _, item := range menu.Main {
			menuItem := map[string]any{
				"path":       item.Path,
				"title":      item.Title,
				"external":   item.External,
				"visibility": string(item.Visibility),
			}
			mainMenuItems = append(mainMenuItems, menuItem)
		}
		menuFields["main"] = mainMenuItems
	}

	// Add footer sections
	if menu.FooterSections != nil {
		var footerSections []map[string]any
		for _, section := range menu.FooterSections {
			var items []map[string]any
			for _, item := range section.Items {
				menuItem := map[string]any{
					"path":       item.Path,
					"title":      item.Title,
					"external":   item.External,
					"visibility": string(item.Visibility),
				}
				items = append(items, menuItem)
			}
			sectionMap := map[string]any{
				"title": section.Title,
				"items": items,
			}
			footerSections = append(footerSections, sectionMap)
		}
		menuFields["footer_sections"] = footerSections
	}

	return menuFields
}

// compareTheme does deep comparison of theme objects
func (p *Planner) compareTheme(current, desired *kkComps.Theme) bool {
	if current == nil && desired == nil {
		return true
	}
	if current == nil || desired == nil {
		return false
	}

	// Compare mode
	if !p.compareModePtr(current.Mode, desired.Mode) {
		return false
	}

	// Compare name
	if !p.compareStringPtr(current.Name, desired.Name) {
		return false
	}

	// Compare colors
	if current.Colors == nil && desired.Colors == nil {
		return true
	}
	if current.Colors == nil || desired.Colors == nil {
		return false
	}

	return p.compareStringPtr(current.Colors.Primary, desired.Colors.Primary)
}

// compareMenu does deep comparison of menu objects
func (p *Planner) compareMenu(current, desired *kkComps.Menu) bool {
	if current == nil && desired == nil {
		return true
	}
	if current == nil || desired == nil {
		return false
	}

	// Compare main menu items
	if len(current.Main) != len(desired.Main) {
		return false
	}
	for i, currentItem := range current.Main {
		desiredItem := desired.Main[i]
		if currentItem.Path != desiredItem.Path ||
			currentItem.Title != desiredItem.Title ||
			currentItem.External != desiredItem.External ||
			currentItem.Visibility != desiredItem.Visibility {
			return false
		}
	}

	// Compare footer sections
	if len(current.FooterSections) != len(desired.FooterSections) {
		return false
	}
	for i, currentSection := range current.FooterSections {
		desiredSection := desired.FooterSections[i]
		if currentSection.Title != desiredSection.Title ||
			len(currentSection.Items) != len(desiredSection.Items) {
			return false
		}

		// Compare items in section
		for j, currentItem := range currentSection.Items {
			desiredItem := desiredSection.Items[j]
			if currentItem.Path != desiredItem.Path ||
				currentItem.Title != desiredItem.Title ||
				currentItem.External != desiredItem.External ||
				currentItem.Visibility != desiredItem.Visibility {
				return false
			}
		}
	}

	return true
}

// compareStringPtr compares two string pointers
func (p *Planner) compareStringPtr(current, desired *string) bool {
	if current == nil && desired == nil {
		return true
	}
	if current == nil || desired == nil {
		return false
	}
	return *current == *desired
}

// compareModePtr compares two PortalCustomizationMode pointers
func (p *Planner) compareModePtr(current, desired *kkComps.PortalCustomizationMode) bool {
	if current == nil && desired == nil {
		return true
	}
	if current == nil || desired == nil {
		return false
	}
	return *current == *desired
}

func (p *Planner) comparePtrBools(current, desired *bool) bool {
	if current == nil && desired == nil {
		return true
	}
	if current == nil || desired == nil {
		return false
	}
	return *current == *desired
}

func (p *Planner) compareBoolToPtr(current bool, desired *bool) bool {
	if desired == nil {
		return true
	}
	return current == *desired
}

// Portal Custom Domain planning

func (p *Planner) planPortalCustomDomainsChanges(
	ctx context.Context,
	parentNamespace string,
	portalID string,
	portalRef string,
	desired []resources.PortalCustomDomainResource,
	plan *Plan,
) error {
	var desiredDomain *resources.PortalCustomDomainResource
	for i := range desired {
		if plan.HasChange(ResourceTypePortalCustomDomain, desired[i].GetRef()) {
			continue
		}
		desiredDomain = &desired[i]
		break
	}

	portalName := p.findPortalName(portalRef)

	// If the portal does not yet exist, schedule creation based on desired state only.
	if portalID == "" {
		if desiredDomain != nil {
			p.planPortalCustomDomainCreate(parentNamespace, *desiredDomain, portalID, portalRef, portalName, plan)
		}
		return nil
	}

	currentDomain, err := p.client.GetPortalCustomDomain(ctx, portalID)
	if err != nil {
		var apiErr *state.APIClientError
		if errors.As(err, &apiErr) && apiErr.ClientType == "portal custom domain API" {
			if desiredDomain != nil {
				changeID := p.planPortalCustomDomainCreate(
					parentNamespace,
					*desiredDomain,
					portalID,
					portalRef,
					portalName,
					plan,
				)
				plan.AddWarning(
					changeID,
					"unable to inspect existing portal custom domain – assuming create is required",
				)
			}
			return nil
		}

		identifier := portalRef
		if identifier == "" {
			identifier = portalID
		}

		return fmt.Errorf("failed to get portal custom domain for portal %q: %w", identifier, err)
	}

	if desiredDomain == nil {
		if currentDomain != nil && plan.Metadata.Mode == PlanModeSync {
			p.planPortalCustomDomainDelete(parentNamespace, portalRef, portalID, portalName, currentDomain, "", plan)
		}
		return nil
	}

	if currentDomain == nil {
		p.planPortalCustomDomainCreate(parentNamespace, *desiredDomain, portalID, portalRef, portalName, plan)
		return nil
	}

	if p.portalCustomDomainNeedsReplacement(currentDomain, *desiredDomain) {
		deleteID := p.planPortalCustomDomainDelete(
			parentNamespace,
			portalRef,
			portalID,
			portalName,
			currentDomain,
			desiredDomain.Ref,
			plan,
		)
		p.planPortalCustomDomainCreate(parentNamespace, *desiredDomain, portalID, portalRef, portalName, plan, deleteID)
		return nil
	}

	if currentDomain.Enabled != desiredDomain.Enabled {
		p.planPortalCustomDomainUpdate(parentNamespace, *desiredDomain, portalID, portalRef, portalName, plan)
	}

	return nil
}

func (p *Planner) planPortalCustomDomainCreate(
	parentNamespace string,
	domain resources.PortalCustomDomainResource,
	portalID string,
	portalRef string,
	portalName string,
	plan *Plan,
	extraDeps ...string,
) string {
	fields := map[string]any{
		"hostname": domain.Hostname,
		"enabled":  domain.Enabled,
	}

	switch {
	case domain.Ssl.CustomCertificate != nil:
		sslFields := map[string]any{
			"domain_verification_method": domain.Ssl.CustomCertificate.GetDomainVerificationMethod(),
			"custom_certificate":         domain.Ssl.CustomCertificate.GetCustomCertificate(),
			"custom_private_key":         domain.Ssl.CustomCertificate.GetCustomPrivateKey(),
		}
		if skip := domain.Ssl.CustomCertificate.GetSkipCaCheck(); skip != nil {
			sslFields["skip_ca_check"] = *skip
		}
		fields["ssl"] = sslFields
	case domain.Ssl.HTTP != nil:
		fields["ssl"] = map[string]any{
			"domain_verification_method": domain.Ssl.HTTP.GetDomainVerificationMethod(),
		}
	}

	deps := append(p.portalChildDependencies(plan, domain.Portal), extraDeps...)
	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypePortalCustomDomain, domain.Ref),
		ResourceType: ResourceTypePortalCustomDomain,
		ResourceRef:  domain.Ref,
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    uniqueStrings(deps),
		Namespace:    parentNamespace,
		ResourceMonikers: map[string]string{
			"hostname": domain.Hostname,
		},
	}

	ref := domain.Portal
	if ref == "" {
		ref = portalRef
	}
	if ref != "" || portalID != "" {
		change.Parent = &ParentInfo{
			Ref: ref,
			ID:  portalID,
		}

		refInfo := ReferenceInfo{
			Ref: ref,
			ID:  portalID,
		}
		if portalName != "" {
			refInfo.LookupFields = map[string]string{
				"name": portalName,
			}
		}
		change.References = map[string]ReferenceInfo{
			"portal_id": refInfo,
		}
	}

	plan.AddChange(change)
	return change.ID
}

func (p *Planner) planPortalCustomDomainUpdate(
	parentNamespace string,
	domain resources.PortalCustomDomainResource,
	portalID string,
	portalRef string,
	portalName string,
	plan *Plan,
) string {
	deps := p.portalChildDependencies(plan, domain.Portal)
	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypePortalCustomDomain, domain.Ref),
		ResourceType: ResourceTypePortalCustomDomain,
		ResourceRef:  domain.Ref,
		ResourceID:   portalID,
		Action:       ActionUpdate,
		Fields: map[string]any{
			"enabled": domain.Enabled,
		},
		DependsOn: uniqueStrings(deps),
		Namespace: parentNamespace,
		ResourceMonikers: map[string]string{
			"hostname": domain.Hostname,
		},
	}

	ref := domain.Portal
	if ref == "" {
		ref = portalRef
	}
	if ref != "" || portalID != "" {
		change.Parent = &ParentInfo{
			Ref: ref,
			ID:  portalID,
		}
		refInfo := ReferenceInfo{
			Ref: ref,
			ID:  portalID,
		}
		if portalName != "" {
			refInfo.LookupFields = map[string]string{
				"name": portalName,
			}
		}
		change.References = map[string]ReferenceInfo{
			"portal_id": refInfo,
		}
	}

	plan.AddChange(change)
	return change.ID
}

func (p *Planner) planPortalCustomDomainDelete(
	parentNamespace string,
	portalRef string,
	portalID string,
	portalName string,
	current *state.PortalCustomDomain,
	domainRef string,
	plan *Plan,
) string {
	ref := domainRef
	if ref == "" {
		if portalRef != "" {
			ref = fmt.Sprintf("%s__custom_domain", portalRef)
		} else {
			ref = fmt.Sprintf("%s__custom_domain", portalID)
		}
	}

	deps := p.portalChildDependencies(plan, portalRef)
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypePortalCustomDomain, ref),
		ResourceType: ResourceTypePortalCustomDomain,
		ResourceRef:  ref,
		ResourceID:   portalID,
		Action:       ActionDelete,
		DependsOn:    uniqueStrings(deps),
		Namespace:    parentNamespace,
	}

	if current != nil {
		change.Fields = map[string]any{
			"hostname":                   current.Hostname,
			"domain_verification_method": current.DomainVerificationMethod,
			"enabled":                    current.Enabled,
		}
		change.ResourceMonikers = map[string]string{
			"hostname": current.Hostname,
		}
	}

	if portalRef != "" || portalID != "" {
		change.Parent = &ParentInfo{
			Ref: portalRef,
			ID:  portalID,
		}

		refInfo := ReferenceInfo{
			Ref: portalRef,
			ID:  portalID,
		}
		if portalName != "" {
			refInfo.LookupFields = map[string]string{
				"name": portalName,
			}
		}
		change.References = map[string]ReferenceInfo{
			"portal_id": refInfo,
		}
	}

	plan.AddChange(change)
	return change.ID
}

func (p *Planner) portalAssetNeedsUpdate(
	ctx context.Context,
	portalID string,
	desiredDataURL string,
	fetchCurrent func(context.Context, string) (string, error),
) (bool, error) {
	if portalID == "" {
		return true, nil
	}

	currentDataURL, err := fetchCurrent(ctx, portalID)
	if err != nil {
		var sdkErr *kkErrors.SDKError
		if errors.As(err, &sdkErr) && sdkErr.StatusCode == http.StatusNotFound {
			return true, nil
		}
		return false, err
	}

	if currentDataURL == "" {
		return true, nil
	}

	equal, err := dataURLsEqual(desiredDataURL, currentDataURL)
	if err != nil {
		return false, err
	}

	return !equal, nil
}

func dataURLsEqual(desired string, current string) (bool, error) {
	desiredBytes, err := decodeDataURL(desired)
	if err != nil {
		return false, fmt.Errorf("decode desired data URL: %w", err)
	}

	currentBytes, err := decodeDataURL(current)
	if err != nil {
		return false, fmt.Errorf("decode current data URL: %w", err)
	}

	return bytes.Equal(desiredBytes, currentBytes), nil
}

func decodeDataURL(dataURL string) ([]byte, error) {
	if !strings.HasPrefix(dataURL, "data:") {
		return nil, fmt.Errorf("invalid data URL: missing data prefix")
	}

	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid data URL: missing comma separator")
	}

	meta := parts[0]
	payload := parts[1]

	if strings.Contains(meta, ";base64") {
		decoded, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, fmt.Errorf("decode base64 payload: %w", err)
		}
		return decoded, nil
	}

	decoded, err := url.PathUnescape(payload)
	if err != nil {
		return nil, fmt.Errorf("decode url-escaped payload: %w", err)
	}

	return []byte(decoded), nil
}

// Portal Asset Logo planning (singleton)

func (p *Planner) planPortalAssetLogosChanges(
	ctx context.Context, plannerCtx *Config, parentNamespace string,
	desired []resources.PortalAssetLogoResource, plan *Plan,
) error {
	namespace := plannerCtx.Namespace
	existingPortals, _ := p.client.ListManagedPortals(ctx, []string{namespace})
	portalNameToID := make(map[string]string)
	for _, portal := range existingPortals {
		portalNameToID[portal.Name] = portal.ID
	}

	for _, desiredLogo := range desired {
		if plan.HasChange(ResourceTypePortalAssetLogo, desiredLogo.GetRef()) {
			continue
		}

		if p.isPortalExternal(desiredLogo.Portal) {
			continue
		}

		var portalName, portalID string
		for _, portal := range p.desiredPortals {
			if portal.Ref == desiredLogo.Portal {
				portalName = portal.Name
				portalID = portalNameToID[portalName]
				break
			}
		}

		// If portal doesn't exist, plan with empty ID for runtime resolution
		needsUpdate, err := p.portalAssetNeedsUpdate(
			ctx,
			portalID,
			*desiredLogo.File,
			p.client.GetPortalAssetLogo,
		)
		if err != nil {
			return fmt.Errorf("failed to compare portal asset logo for portal %q: %w", desiredLogo.Portal, err)
		}
		if !needsUpdate {
			p.logger.Debug("Skipping portal asset logo update; no changes detected",
				slog.String("portal", desiredLogo.Portal),
			)
			continue
		}

		p.planPortalAssetLogoUpdate(parentNamespace, desiredLogo.Portal, portalName, portalID, *desiredLogo.File, plan)
	}

	return nil
}

func (p *Planner) planPortalAssetLogoUpdate(
	parentNamespace, portalRef, portalName, portalID, dataURL string, plan *Plan,
) {
	ref := fmt.Sprintf("%s-logo", portalRef)

	fields := map[string]any{
		"data_url": dataURL,
	}

	// Find portal creation dependency if portal doesn't exist yet
	deps := []string{}
	if portalID == "" {
		for _, change := range plan.Changes {
			if change.ResourceType == ResourceTypePortal && change.ResourceRef == portalRef {
				deps = append(deps, change.ID)
				break
			}
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypePortalAssetLogo, ref),
		ResourceType: ResourceTypePortalAssetLogo,
		ResourceRef:  ref,
		Action:       ActionUpdate, // Always UPDATE for singletons
		Fields:       fields,
		Namespace:    parentNamespace,
		DependsOn:    uniqueStrings(deps),
	}

	// Set parent info for runtime resolution (follows pattern from customization/auth_settings)
	change.Parent = &ParentInfo{
		Ref: portalRef,
		ID:  portalID, // May be empty if portal doesn't exist yet
	}

	// Also store in References for executor to use
	change.References = map[string]ReferenceInfo{
		"portal_id": {
			Ref: portalRef,
			LookupFields: map[string]string{
				"name": portalName,
			},
		},
	}

	plan.AddChange(change)
}

// Portal Asset Favicon planning (singleton)

func (p *Planner) planPortalAssetFaviconsChanges(
	ctx context.Context, plannerCtx *Config, parentNamespace string,
	desired []resources.PortalAssetFaviconResource, plan *Plan,
) error {
	namespace := plannerCtx.Namespace
	existingPortals, _ := p.client.ListManagedPortals(ctx, []string{namespace})
	portalNameToID := make(map[string]string)
	for _, portal := range existingPortals {
		portalNameToID[portal.Name] = portal.ID
	}

	for _, desiredFavicon := range desired {
		if plan.HasChange(ResourceTypePortalAssetFavicon, desiredFavicon.GetRef()) {
			continue
		}

		if p.isPortalExternal(desiredFavicon.Portal) {
			continue
		}

		var portalName, portalID string
		for _, portal := range p.desiredPortals {
			if portal.Ref == desiredFavicon.Portal {
				portalName = portal.Name
				portalID = portalNameToID[portalName]
				break
			}
		}

		// If portal doesn't exist, plan with empty ID for runtime resolution
		needsUpdate, err := p.portalAssetNeedsUpdate(
			ctx,
			portalID,
			*desiredFavicon.File,
			p.client.GetPortalAssetFavicon,
		)
		if err != nil {
			return fmt.Errorf("failed to compare portal asset favicon for portal %q: %w", desiredFavicon.Portal, err)
		}
		if !needsUpdate {
			p.logger.Debug("Skipping portal asset favicon update; no changes detected",
				slog.String("portal", desiredFavicon.Portal),
			)
			continue
		}

		p.planPortalAssetFaviconUpdate(
			parentNamespace, desiredFavicon.Portal, portalName, portalID, *desiredFavicon.File, plan,
		)
	}

	return nil
}

func (p *Planner) planPortalAssetFaviconUpdate(
	parentNamespace, portalRef, portalName, portalID, dataURL string, plan *Plan,
) {
	ref := fmt.Sprintf("%s-favicon", portalRef)

	fields := map[string]any{
		"data_url": dataURL,
	}

	// Find portal creation dependency if portal doesn't exist yet
	deps := []string{}
	if portalID == "" {
		for _, change := range plan.Changes {
			if change.ResourceType == ResourceTypePortal && change.ResourceRef == portalRef {
				deps = append(deps, change.ID)
				break
			}
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypePortalAssetFavicon, ref),
		ResourceType: ResourceTypePortalAssetFavicon,
		ResourceRef:  ref,
		Action:       ActionUpdate, // Always UPDATE for singletons
		Fields:       fields,
		Namespace:    parentNamespace,
		DependsOn:    uniqueStrings(deps),
	}

	// Set parent info for runtime resolution (follows pattern from customization/auth_settings)
	change.Parent = &ParentInfo{
		Ref: portalRef,
		ID:  portalID, // May be empty if portal doesn't exist yet
	}

	// Also store in References for executor to use
	change.References = map[string]ReferenceInfo{
		"portal_id": {
			Ref: portalRef,
			LookupFields: map[string]string{
				"name": portalName,
			},
		},
	}

	plan.AddChange(change)
}

func (p *Planner) portalChildDependencies(plan *Plan, portalRef string) []string {
	if portalRef == "" {
		return nil
	}

	for _, change := range plan.Changes {
		if change.ResourceType == ResourceTypePortal && change.ResourceRef == portalRef {
			return []string{change.ID}
		}
	}

	return nil
}

func (p *Planner) findPortalName(portalRef string) string {
	if portalRef == "" {
		return ""
	}
	for _, portal := range p.desiredPortals {
		if portal.Ref == portalRef {
			return portal.Name
		}
	}
	return ""
}

func (p *Planner) isPortalExternal(portalRef string) bool {
	if portalRef == "" {
		return false
	}
	for _, portal := range p.desiredPortals {
		if portal.Ref == portalRef {
			return portal.IsExternal()
		}
	}
	return false
}

func (p *Planner) portalCustomDomainNeedsReplacement(
	current *state.PortalCustomDomain,
	desired resources.PortalCustomDomainResource,
) bool {
	if current == nil {
		return true
	}

	if desired.Hostname != "" && !strings.EqualFold(current.Hostname, desired.Hostname) {
		return true
	}

	desiredMethod, desiredSkip := p.desiredPortalCustomDomainSSLConfig(desired)
	currentMethod := strings.ToLower(current.DomainVerificationMethod)

	if desiredMethod != "" && !strings.EqualFold(currentMethod, desiredMethod) {
		return true
	}

	if desiredSkip != boolValue(current.SkipCACheck) {
		return true
	}

	return false
}

// Portal Email Config planning

func (p *Planner) planPortalEmailConfigsChanges(
	ctx context.Context,
	parentNamespace string,
	portalID string,
	portalRef string,
	desired []resources.PortalEmailConfigResource,
	plan *Plan,
) error {
	var desiredCfg *resources.PortalEmailConfigResource
	for i := range desired {
		if plan.HasChange(ResourceTypePortalEmailConfig, desired[i].GetRef()) {
			continue
		}
		desiredCfg = &desired[i]
		break
	}

	portalName := p.findPortalName(portalRef)

	if portalID == "" {
		if desiredCfg != nil {
			p.planPortalEmailConfigCreate(parentNamespace, *desiredCfg, portalID, portalRef, portalName, plan)
		}
		return nil
	}

	currentCfg, err := p.client.GetPortalEmailConfig(ctx, portalID)
	if err != nil {
		var apiErr *state.APIClientError
		if errors.As(err, &apiErr) && apiErr.ClientType == "portal emails API" {
			if desiredCfg != nil {
				changeID := p.planPortalEmailConfigCreate(
					parentNamespace,
					*desiredCfg,
					portalID,
					portalRef,
					portalName,
					plan,
				)
				plan.AddWarning(
					changeID,
					"unable to inspect existing portal email config – assuming create is required",
				)
			}
			return nil
		}

		identifier := portalRef
		if identifier == "" {
			identifier = portalID
		}

		return fmt.Errorf("failed to get portal email config for portal %q: %w", identifier, err)
	}

	if desiredCfg == nil {
		if currentCfg != nil && plan.Metadata.Mode == PlanModeSync {
			p.planPortalEmailConfigDelete(parentNamespace, portalRef, portalID, portalName, plan)
		}
		return nil
	}

	if currentCfg == nil {
		p.planPortalEmailConfigCreate(parentNamespace, *desiredCfg, portalID, portalRef, portalName, plan)
		return nil
	}

	if p.shouldUpdatePortalEmailConfig(currentCfg, *desiredCfg) {
		p.planPortalEmailConfigUpdate(
			parentNamespace,
			*desiredCfg,
			portalID,
			portalRef,
			portalName,
			currentCfg.ID,
			plan,
		)
	}

	return nil
}

func (p *Planner) planPortalEmailConfigCreate(
	parentNamespace string,
	cfg resources.PortalEmailConfigResource,
	portalID string,
	portalRef string,
	portalName string,
	plan *Plan,
) string {
	fields := p.buildPortalEmailConfigFields(cfg)

	deps := p.portalChildDependencies(plan, cfg.Portal)
	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypePortalEmailConfig, cfg.Ref),
		ResourceType: ResourceTypePortalEmailConfig,
		ResourceRef:  cfg.Ref,
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    uniqueStrings(deps),
		Namespace:    parentNamespace,
	}

	ref := cfg.Portal
	if ref == "" {
		ref = portalRef
	}
	if ref != "" || portalID != "" {
		change.Parent = &ParentInfo{
			Ref: ref,
			ID:  portalID,
		}

		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: ref,
				ID:  portalID,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
	}

	plan.AddChange(change)
	return change.ID
}

func (p *Planner) planPortalEmailConfigUpdate(
	parentNamespace string,
	cfg resources.PortalEmailConfigResource,
	portalID string,
	portalRef string,
	portalName string,
	resourceID string,
	plan *Plan,
) {
	fields := p.buildPortalEmailConfigFields(cfg)

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypePortalEmailConfig, cfg.Ref),
		ResourceType: ResourceTypePortalEmailConfig,
		ResourceRef:  cfg.Ref,
		ResourceID:   resourceID,
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    p.portalChildDependencies(plan, cfg.Portal),
		Namespace:    parentNamespace,
	}

	ref := cfg.Portal
	if ref == "" {
		ref = portalRef
	}
	if ref != "" || portalID != "" {
		change.Parent = &ParentInfo{
			Ref: ref,
			ID:  portalID,
		}
		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: ref,
				ID:  portalID,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
	}

	plan.AddChange(change)
}

func (p *Planner) planPortalEmailConfigDelete(
	parentNamespace string,
	portalRef string,
	portalID string,
	portalName string,
	plan *Plan,
) {
	ref := portalRef
	if ref == "" {
		ref = fmt.Sprintf("%s__email_config", portalID)
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypePortalEmailConfig, ref),
		ResourceType: ResourceTypePortalEmailConfig,
		ResourceRef:  ref,
		ResourceID:   portalID,
		Action:       ActionDelete,
		DependsOn:    p.portalChildDependencies(plan, portalRef),
		Namespace:    parentNamespace,
	}

	if portalRef != "" || portalID != "" {
		change.Parent = &ParentInfo{
			Ref: portalRef,
			ID:  portalID,
		}
		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: portalRef,
				ID:  portalID,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
	}

	plan.AddChange(change)
}

func (p *Planner) shouldUpdatePortalEmailConfig(
	current *kkComps.PortalEmailConfig,
	desired resources.PortalEmailConfigResource,
) bool {
	if current == nil {
		return true
	}

	if desired.DomainNameSet {
		currentDomain := getString(current.DomainName)
		if desired.DomainName == nil {
			if currentDomain != "" {
				return true
			}
		} else if currentDomain != *desired.DomainName {
			return true
		}
	}

	if desired.FromNameSet {
		if desired.FromName == nil {
			if current.FromName != "" {
				return true
			}
		} else if *desired.FromName != current.FromName {
			return true
		}
	}

	if desired.FromEmailSet {
		if desired.FromEmail == nil {
			if current.FromEmail != "" {
				return true
			}
		} else if *desired.FromEmail != current.FromEmail {
			return true
		}
	}

	if desired.ReplyToEmailSet {
		if desired.ReplyToEmail == nil {
			if current.ReplyToEmail != "" {
				return true
			}
		} else if *desired.ReplyToEmail != current.ReplyToEmail {
			return true
		}
	}

	return false
}

// Portal Email Templates planning

func (p *Planner) planPortalEmailTemplatesChanges(
	ctx context.Context,
	parentNamespace string,
	portalID string,
	portalRef string,
	portalName string,
	desired []resources.PortalEmailTemplateResource,
	plan *Plan,
) error {
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	existing := make(map[string]state.PortalEmailTemplate)
	if portalID != "" {
		templates, err := p.client.ListPortalCustomEmailTemplates(ctx, portalID)
		if err != nil {
			if strings.Contains(err.Error(), "portal emails API") && strings.Contains(err.Error(), "not configured") {
				return nil
			}
			if !strings.Contains(err.Error(), "not found") {
				return fmt.Errorf("failed to list portal email templates: %w", err)
			}
		} else {
			for _, tpl := range templates {
				existing[tpl.Name] = tpl
			}
		}
	}

	desiredByName := make(map[string]resources.PortalEmailTemplateResource)

	for _, tpl := range desired {
		nameKey := string(tpl.Name)
		desiredByName[nameKey] = tpl

		if plan.HasChange(ResourceTypePortalEmailTemplate, tpl.GetRef()) {
			continue
		}

		if current, ok := existing[nameKey]; ok {
			var currentDetails *state.PortalEmailTemplate
			if portalID != "" {
				full, err := p.client.GetPortalCustomEmailTemplate(
					ctx,
					portalID,
					kkComps.EmailTemplateName(current.Name),
				)
				if err != nil {
					return fmt.Errorf("failed to fetch portal email template %s for comparison: %w", current.Name, err)
				}
				currentDetails = full
			}
			if currentDetails == nil {
				currentDetails = &current
			}

			needsUpdate, fields := p.shouldUpdatePortalEmailTemplate(currentDetails, tpl)
			if needsUpdate {
				p.planPortalEmailTemplateUpdate(
					parentNamespace,
					tpl,
					portalID,
					portalRef,
					portalName,
					fields,
					plan,
				)
			}
		} else {
			p.planPortalEmailTemplateCreate(parentNamespace, tpl, portalID, portalRef, portalName, plan)
		}
	}

	if plan.Metadata.Mode == PlanModeSync && portalID != "" {
		for name, tpl := range existing {
			if _, ok := desiredByName[name]; ok {
				continue
			}
			p.planPortalEmailTemplateDelete(parentNamespace, portalRef, portalID, portalName, tpl, plan)
		}
	}

	return nil
}

func (p *Planner) planPortalEmailTemplateCreate(
	parentNamespace string,
	tpl resources.PortalEmailTemplateResource,
	portalID string,
	portalRef string,
	portalName string,
	plan *Plan,
) {
	fields := p.buildPortalEmailTemplateFields(tpl)

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypePortalEmailTemplate, tpl.Ref),
		ResourceType: ResourceTypePortalEmailTemplate,
		ResourceRef:  tpl.Ref,
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    p.portalChildDependencies(plan, tpl.Portal),
		Namespace:    parentNamespace,
	}

	ref := tpl.Portal
	if ref == "" {
		ref = portalRef
	}
	if ref != "" || portalID != "" {
		change.Parent = &ParentInfo{
			Ref: ref,
			ID:  portalID,
		}
		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: ref,
				ID:  portalID,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
	}

	plan.AddChange(change)
}

func (p *Planner) planPortalEmailTemplateUpdate(
	parentNamespace string,
	tpl resources.PortalEmailTemplateResource,
	portalID string,
	portalRef string,
	portalName string,
	updateFields map[string]any,
	plan *Plan,
) {
	fields := map[string]any{
		"name": tpl.Name,
	}
	for k, v := range updateFields {
		fields[k] = v
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypePortalEmailTemplate, tpl.Ref),
		ResourceType: ResourceTypePortalEmailTemplate,
		ResourceRef:  tpl.Ref,
		ResourceID:   tpl.GetKonnectID(),
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    p.portalChildDependencies(plan, tpl.Portal),
		Namespace:    parentNamespace,
	}

	ref := tpl.Portal
	if ref == "" {
		ref = portalRef
	}
	if ref != "" || portalID != "" {
		change.Parent = &ParentInfo{
			Ref: ref,
			ID:  portalID,
		}
		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: ref,
				ID:  portalID,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
	}

	plan.AddChange(change)
}

func (p *Planner) planPortalEmailTemplateDelete(
	parentNamespace string,
	portalRef string,
	portalID string,
	portalName string,
	current state.PortalEmailTemplate,
	plan *Plan,
) {
	ref := portalRef
	if ref == "" {
		ref = fmt.Sprintf("%s__email_template_%s", portalID, current.Name)
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypePortalEmailTemplate, ref),
		ResourceType: ResourceTypePortalEmailTemplate,
		ResourceRef:  ref,
		ResourceID:   current.Name,
		Action:       ActionDelete,
		DependsOn:    p.portalChildDependencies(plan, portalRef),
		Namespace:    parentNamespace,
		Fields: map[string]any{
			"name": current.Name,
		},
	}

	if portalRef != "" || portalID != "" {
		change.Parent = &ParentInfo{
			Ref: portalRef,
			ID:  portalID,
		}
		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: portalRef,
				ID:  portalID,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
	}

	plan.AddChange(change)
}

func (p *Planner) shouldUpdatePortalEmailTemplate(
	current *state.PortalEmailTemplate,
	desired resources.PortalEmailTemplateResource,
) (bool, map[string]any) {
	updates := make(map[string]any)

	if current == nil {
		return true, updates
	}

	if desired.EnabledSet && desired.Enabled != nil && current.Enabled != *desired.Enabled {
		updates["enabled"] = *desired.Enabled
	}

	if desired.ContentSet {
		if desired.Content == nil {
			if current.Content != nil {
				updates["content"] = nil
			}
		} else {
			contentUpdates := make(map[string]any)
			if desired.Content.SubjectSet {
				currentVal := ""
				if current.Content != nil && current.Content.Subject != nil {
					currentVal = *current.Content.Subject
				}
				if desired.Content.Subject == nil {
					if currentVal != "" {
						contentUpdates["subject"] = nil
					}
				} else if currentVal != *desired.Content.Subject {
					contentUpdates["subject"] = *desired.Content.Subject
				}
			}
			if desired.Content.TitleSet {
				currentVal := ""
				if current.Content != nil && current.Content.Title != nil {
					currentVal = *current.Content.Title
				}
				if desired.Content.Title == nil {
					if currentVal != "" {
						contentUpdates["title"] = nil
					}
				} else if currentVal != *desired.Content.Title {
					contentUpdates["title"] = *desired.Content.Title
				}
			}
			if desired.Content.BodySet {
				currentVal := ""
				if current.Content != nil && current.Content.Body != nil {
					currentVal = *current.Content.Body
				}
				if desired.Content.Body == nil {
					if currentVal != "" {
						contentUpdates["body"] = nil
					}
				} else if currentVal != *desired.Content.Body {
					contentUpdates["body"] = *desired.Content.Body
				}
			}
			if desired.Content.ButtonLabelSet {
				currentVal := ""
				if current.Content != nil && current.Content.ButtonLabel != nil {
					currentVal = *current.Content.ButtonLabel
				}
				if desired.Content.ButtonLabel == nil {
					if currentVal != "" {
						contentUpdates["button_label"] = nil
					}
				} else if currentVal != *desired.Content.ButtonLabel {
					contentUpdates["button_label"] = *desired.Content.ButtonLabel
				}
			}

			if len(contentUpdates) > 0 {
				updates["content"] = contentUpdates
			}
		}
	}

	return len(updates) > 0, updates
}

func (p *Planner) buildPortalEmailTemplateFields(tpl resources.PortalEmailTemplateResource) map[string]any {
	fields := map[string]any{
		"name": tpl.Name,
	}

	if tpl.EnabledSet && tpl.Enabled != nil {
		fields["enabled"] = *tpl.Enabled
	}

	contentSet := tpl.ContentSet || tpl.Content != nil
	if contentSet {
		if tpl.Content == nil {
			fields["content"] = nil
		} else {
			contentFields := make(map[string]any)
			if tpl.Content.SubjectSet || tpl.Content.Subject != nil {
				contentFields["subject"] = tpl.Content.Subject
			}
			if tpl.Content.TitleSet || tpl.Content.Title != nil {
				contentFields["title"] = tpl.Content.Title
			}
			if tpl.Content.BodySet || tpl.Content.Body != nil {
				contentFields["body"] = tpl.Content.Body
			}
			if tpl.Content.ButtonLabelSet || tpl.Content.ButtonLabel != nil {
				contentFields["button_label"] = tpl.Content.ButtonLabel
			}
			if len(contentFields) > 0 {
				fields["content"] = contentFields
			}
		}
	}

	return fields
}

func (p *Planner) buildPortalEmailConfigFields(cfg resources.PortalEmailConfigResource) map[string]any {
	fields := map[string]any{}

	setField := func(set bool, key string, value *string) {
		if !set {
			return
		}
		if value == nil {
			fields[key] = nil
			return
		}
		fields[key] = *value
	}

	setField(cfg.DomainNameSet, "domain_name", cfg.DomainName)
	setField(cfg.FromNameSet, "from_name", cfg.FromName)
	setField(cfg.FromEmailSet, "from_email", cfg.FromEmail)
	setField(cfg.ReplyToEmailSet, "reply_to_email", cfg.ReplyToEmail)

	return fields
}

func (p *Planner) desiredPortalCustomDomainSSLConfig(
	domain resources.PortalCustomDomainResource,
) (string, bool) {
	switch {
	case domain.Ssl.CustomCertificate != nil:
		skip := false
		if value := domain.Ssl.CustomCertificate.GetSkipCaCheck(); value != nil {
			skip = *value
		}
		return strings.ToLower(domain.Ssl.CustomCertificate.GetDomainVerificationMethod()), skip
	case domain.Ssl.HTTP != nil:
		return strings.ToLower(domain.Ssl.HTTP.GetDomainVerificationMethod()), false
	default:
		return "", false
	}
}

func boolValue(ptr *bool) bool {
	if ptr == nil {
		return false
	}
	return *ptr
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))

	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// Portal Page planning

func (p *Planner) planPortalPagesChanges(
	ctx context.Context, parentNamespace string, portalID string, portalRef string,
	desired []resources.PortalPageResource, plan *Plan,
) error {
	// Fetch existing pages for this portal
	existingPages := make([]state.PortalPage, 0)
	if portalID != "" {
		pages, err := p.client.ListManagedPortalPages(ctx, portalID)
		if err != nil {
			// If portal page API is not configured, skip processing
			// This happens in tests or when portal pages feature is not available
			if strings.Contains(err.Error(), "portal page API not configured") {
				// In sync mode with no desired pages, this is OK - nothing to delete
				if plan.Metadata.Mode == PlanModeSync && len(desired) == 0 {
					return nil
				}
				// But if there are desired pages, we need the API
				if len(desired) > 0 {
					return fmt.Errorf("failed to list portal pages: %w", err)
				}
				return nil
			}
			// If portal doesn't exist yet, that's ok - we'll create pages after portal is created
			if !strings.Contains(err.Error(), "not found") {
				return fmt.Errorf("failed to list portal pages: %w", err)
			}
		} else {
			existingPages = pages
		}
	}

	// Build maps for matching
	// Build map from full slug path to page
	existingByPath := make(map[string]state.PortalPage)
	existingByID := make(map[string]state.PortalPage)

	// First, index all pages by ID for easy lookup
	for _, page := range existingPages {
		existingByID[page.ID] = page
	}

	// Helper to build full path for a page
	var getPagePath func(pageID string) string
	pageIDToPath := make(map[string]string) // cache to avoid recalculation

	getPagePath = func(pageID string) string {
		// Check cache first
		if path, cached := pageIDToPath[pageID]; cached {
			return path
		}

		page, exists := existingByID[pageID]
		if !exists {
			return ""
		}

		// Special handling for root page with slug "/"
		normalizedSlug := page.Slug
		if page.Slug != "/" {
			normalizedSlug = strings.TrimPrefix(page.Slug, "/")
		}

		// Root page - path is just the slug
		if page.ParentPageID == "" {
			pageIDToPath[pageID] = normalizedSlug
			return normalizedSlug
		}

		// Child page - build full path recursively
		parentPath := getPagePath(page.ParentPageID)
		if parentPath == "" {
			// Parent not found, use slug only
			pageIDToPath[pageID] = normalizedSlug
			return normalizedSlug
		}

		fullPath := parentPath + "/" + normalizedSlug
		pageIDToPath[pageID] = fullPath
		return fullPath
	}

	// Build the path map for all existing pages
	for _, page := range existingPages {
		path := getPagePath(page.ID)
		if path != "" {
			existingByPath[path] = page
		}
	}

	// Note: We don't have refs for existing pages, so we match by full slug paths

	// Process desired pages
	for _, desiredPage := range desired {
		if plan.HasChange("portal_page", desiredPage.GetRef()) {
			continue
		}
		// Build the full path for this desired page to check if it exists
		var fullPath string
		// Special handling for root page with slug "/"
		normalizedDesiredSlug := desiredPage.Slug
		if desiredPage.Slug != "/" {
			normalizedDesiredSlug = strings.TrimPrefix(desiredPage.Slug, "/")
		}

		if desiredPage.ParentPageRef == "" {
			// Root page
			fullPath = normalizedDesiredSlug
		} else {
			// Child page - build parent path first
			parentPath := p.buildParentPath(desiredPage.ParentPageRef, desired)
			if parentPath != "" {
				fullPath = parentPath + "/" + normalizedDesiredSlug
			} else {
				// Parent path couldn't be built, use slug only
				fullPath = normalizedDesiredSlug
			}
		}

		// Check if page exists by full path
		existingPage, exists := existingByPath[fullPath]

		if !exists {
			// CREATE new page
			p.planPortalPageCreate(parentNamespace, desiredPage, portalRef, portalID, plan)
		} else {
			// Check if UPDATE is needed - must fetch full content first
			if portalID != "" && existingPage.ID != "" {
				fullPage, err := p.client.GetPortalPage(ctx, portalID, existingPage.ID)
				if err != nil {
					return fmt.Errorf("failed to fetch portal page %s for comparison: %w", existingPage.ID, err)
				}

				needsUpdate, updateFields := p.shouldUpdatePortalPage(fullPage, desiredPage)
				if needsUpdate {
					p.planPortalPageUpdate(parentNamespace, existingPage, desiredPage, portalRef, updateFields, plan)
				}
			}
		}
	}

	// In sync mode, delete pages that exist but are not in desired state
	if plan.Metadata.Mode == PlanModeSync {
		// Build set of desired page paths
		desiredPaths := make(map[string]bool)
		for _, desiredPage := range desired {
			// Build the full path for this desired page
			var fullPath string
			// Special handling for root page with slug "/"
			normalizedDesiredSlug := desiredPage.Slug
			if desiredPage.Slug != "/" {
				normalizedDesiredSlug = strings.TrimPrefix(desiredPage.Slug, "/")
			}

			if desiredPage.ParentPageRef == "" {
				// Root page
				fullPath = normalizedDesiredSlug
			} else {
				// Child page - build parent path first
				parentPath := p.buildParentPath(desiredPage.ParentPageRef, desired)
				if parentPath != "" {
					fullPath = parentPath + "/" + normalizedDesiredSlug
				} else {
					// Parent path couldn't be built, use slug only
					fullPath = normalizedDesiredSlug
				}
			}

			desiredPaths[fullPath] = true
		}

		// Find pages to delete
		for path, existingPage := range existingByPath {
			if !desiredPaths[path] {
				p.planPortalPageDelete(portalRef, portalID, existingPage.ID, existingPage.Slug, plan)
			}
		}
	}

	return nil
}

func (p *Planner) planPortalPageCreate(
	parentNamespace string, page resources.PortalPageResource, _ string, portalID string, plan *Plan,
) {
	fields := make(map[string]any)
	fields["slug"] = page.Slug
	fields["content"] = page.Content

	if page.Title != nil {
		fields["title"] = *page.Title
	}

	if page.Visibility != nil {
		fields["visibility"] = string(*page.Visibility)
	}

	if page.Status != nil {
		fields["status"] = string(*page.Status)
	}

	if page.Description != nil {
		fields["description"] = *page.Description
	}

	if page.ParentPageID != nil {
		fields["parent_page_id"] = *page.ParentPageID
	}

	// Determine dependencies - depends on parent portal
	var dependencies []string
	if page.Portal != "" {
		// Find the change ID for the parent portal
		for _, change := range plan.Changes {
			if change.ResourceType == ResourceTypePortal && change.ResourceRef == page.Portal {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypePortalPage, page.GetRef()),
		ResourceType: ResourceTypePortalPage,
		ResourceRef:  page.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    dependencies,
		Namespace:    parentNamespace,
	}

	// Store parent portal reference
	if page.Portal != "" {
		// Find the portal in desiredPortals to get its name
		var portalName string
		for _, portal := range p.desiredPortals {
			if portal.Ref == page.Portal {
				portalName = portal.Name
				break
			}
		}

		// Set Parent field for proper display and serialization
		change.Parent = &ParentInfo{
			Ref: page.Portal,
			ID:  portalID, // May be empty if portal doesn't exist yet
		}

		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: page.Portal,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
		// If we already know the Konnect portal ID, include it to avoid name lookup during execution
		if portalID != "" {
			ref := change.References["portal_id"]
			ref.ID = portalID
			change.References["portal_id"] = ref
		}
	}

	// Handle parent page reference
	if page.ParentPageRef != "" {
		// Add dependency on parent page
		for _, depChange := range plan.Changes {
			if depChange.ResourceType == ResourceTypePortalPage && depChange.ResourceRef == page.ParentPageRef {
				change.DependsOn = append(change.DependsOn, depChange.ID)
				break
			}
		}

		// Build parent path to help with resolution
		// Get all desired pages from the planner
		allPages := make([]resources.PortalPageResource, 0)
		for _, portal := range p.desiredPortals {
			if portal.Ref == page.Portal {
				allPages = append(allPages, portal.Pages...)
				break
			}
		}
		// Also include pages at root level
		allPages = append(allPages, p.desiredPortalPages...)

		parentPath := p.buildParentPath(page.ParentPageRef, allPages)

		// Store parent page reference for resolution
		if change.References == nil {
			change.References = make(map[string]ReferenceInfo)
		}
		change.References["parent_page_id"] = ReferenceInfo{
			Ref: page.ParentPageRef,
			LookupFields: map[string]string{
				"parent_path": parentPath,
			},
		}
	}

	plan.AddChange(change)
}

// shouldUpdatePortalPage checks if a portal page needs updating
func (p *Planner) shouldUpdatePortalPage(
	current *state.PortalPage,
	desired resources.PortalPageResource,
) (bool, map[string]any) {
	updates := make(map[string]any)

	// Compare content (always present)
	if current.Content != desired.Content {
		updates["content"] = desired.Content
	}

	// Compare title if set
	if desired.Title != nil && current.Title != *desired.Title {
		updates["title"] = *desired.Title
	}

	// Compare description if set
	if desired.Description != nil && current.Description != *desired.Description {
		updates["description"] = *desired.Description
	}

	// Compare visibility if set
	if desired.Visibility != nil {
		desiredVis := string(*desired.Visibility)
		if current.Visibility != desiredVis {
			updates["visibility"] = desiredVis
		}
	}

	// Compare status if set
	if desired.Status != nil {
		desiredStatus := string(*desired.Status)
		if current.Status != desiredStatus {
			updates["status"] = desiredStatus
		}
	}

	// Note: We don't update slug or parent_page_id as these would effectively be a different page

	return len(updates) > 0, updates
}

// planPortalPageUpdate creates an UPDATE change for a portal page
func (p *Planner) planPortalPageUpdate(
	parentNamespace string,
	current state.PortalPage,
	desired resources.PortalPageResource,
	portalRef string,
	updateFields map[string]any,
	plan *Plan,
) {
	fields := make(map[string]any)

	// Always include slug for identification
	fields["slug"] = current.Slug

	// Add fields that need updating
	for field, value := range updateFields {
		fields[field] = value
	}

	// Determine dependencies - depends on parent portal
	var dependencies []string
	if portalRef != "" {
		// Find the change ID for the parent portal
		for _, change := range plan.Changes {
			if change.ResourceType == ResourceTypePortal && change.ResourceRef == portalRef {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypePortalPage, desired.GetRef()),
		ResourceType: ResourceTypePortalPage,
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    dependencies,
		Namespace:    parentNamespace,
	}

	// Store parent portal reference
	if portalRef != "" {
		// Find the portal in desiredPortals to get its name
		var portalName string
		for _, portal := range p.desiredPortals {
			if portal.Ref == portalRef {
				portalName = portal.Name
				break
			}
		}

		// Set Parent field for proper display and serialization
		change.Parent = &ParentInfo{
			Ref: portalRef,
			ID:  "", // Already known via ResourceID but not needed for display
		}

		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: portalRef,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
	}

	plan.AddChange(change)
}

// planPortalPageDelete creates a DELETE change for a portal page
func (p *Planner) planPortalPageDelete(
	portalRef string, portalID string, pageID string, slug string, plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypePortalPage, pageID),
		ResourceType: ResourceTypePortalPage,
		ResourceRef:  "[unknown]",
		ResourceID:   pageID,
		ResourceMonikers: map[string]string{
			"slug":          slug,
			"parent_portal": portalRef,
		},
		Parent:    &ParentInfo{Ref: portalRef, ID: portalID},
		Action:    ActionDelete,
		Fields:    map[string]any{"slug": slug},
		DependsOn: []string{},
	}

	// Store parent portal reference
	if portalRef != "" {
		// Find the portal in desiredPortals to get its name
		var portalName string
		for _, portal := range p.desiredPortals {
			if portal.Ref == portalRef {
				portalName = portal.Name
				break
			}
		}

		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: portalRef,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
	}

	plan.AddChange(change)
}

// buildParentPath constructs the full slug path for a page ref
func (p *Planner) buildParentPath(pageRef string, allPages []resources.PortalPageResource) string {
	pathSegments := []string{}
	current := pageRef

	// Build path from bottom up
	for current != "" {
		found := false
		for _, page := range allPages {
			if page.GetRef() == current {
				pathSegments = append([]string{page.Slug}, pathSegments...)
				current = page.ParentPageRef
				found = true
				break
			}
		}
		if !found {
			break // Avoid infinite loop
		}
	}

	return strings.Join(pathSegments, "/")
}

// Portal Snippet planning

func (p *Planner) planPortalSnippetsChanges(
	ctx context.Context, parentNamespace string, portalID string, portalRef string,
	desired []resources.PortalSnippetResource, plan *Plan,
) error {
	// Fetch existing snippets for this portal
	existingSnippets := make(map[string]state.PortalSnippet)
	if portalID != "" {
		snippets, err := p.client.ListPortalSnippets(ctx, portalID)
		if err != nil {
			// If portal snippet API is not configured, skip processing
			if strings.Contains(err.Error(), "portal snippet API not configured") {
				// In sync mode with no desired snippets, this is OK - nothing to delete
				if plan.Metadata.Mode == PlanModeSync && len(desired) == 0 {
					return nil
				}
				// But if there are desired snippets, we need the API
				if len(desired) > 0 {
					return fmt.Errorf("failed to list portal snippets: %w", err)
				}
				return nil
			}
			// If portal doesn't exist yet, that's ok - we'll create snippets after portal is created
			if !strings.Contains(err.Error(), "not found") {
				return fmt.Errorf("failed to list portal snippets: %w", err)
			}
		} else {
			// Build map by name for matching
			for _, snippet := range snippets {
				existingSnippets[snippet.Name] = snippet
			}
		}
	}

	// Process desired snippets
	for _, desiredSnippet := range desired {
		if plan.HasChange("portal_snippet", desiredSnippet.GetRef()) {
			continue
		}
		// Check if snippet exists by name
		if existingSnippet, exists := existingSnippets[desiredSnippet.Name]; exists {
			// Check if UPDATE is needed - must fetch full content first
			if portalID != "" && existingSnippet.ID != "" {
				fullSnippet, err := p.client.GetPortalSnippet(ctx, portalID, existingSnippet.ID)
				if err != nil {
					return fmt.Errorf("failed to fetch portal snippet %s for comparison: %w", existingSnippet.ID, err)
				}

				needsUpdate, updateFields := p.shouldUpdatePortalSnippet(fullSnippet, desiredSnippet)
				if needsUpdate {
					p.planPortalSnippetUpdate(
						parentNamespace,
						existingSnippet,
						desiredSnippet,
						portalRef,
						updateFields,
						plan,
					)
				}
			}
		} else {
			// CREATE new snippet
			p.planPortalSnippetCreate(parentNamespace, desiredSnippet, portalRef, portalID, plan)
		}
	}

	return nil
}

func (p *Planner) planPortalSnippetCreate(
	parentNamespace string, snippet resources.PortalSnippetResource, _ string, portalID string, plan *Plan,
) {
	fields := make(map[string]any)
	fields["name"] = snippet.Name
	fields["content"] = snippet.Content

	// Include optional fields if present
	if snippet.Title != nil {
		fields["title"] = *snippet.Title
	}
	if snippet.Visibility != nil {
		fields["visibility"] = string(*snippet.Visibility)
	}
	if snippet.Status != nil {
		fields["status"] = string(*snippet.Status)
	}
	if snippet.Description != nil {
		fields["description"] = *snippet.Description
	}

	// Determine dependencies - depends on parent portal
	var dependencies []string
	if snippet.Portal != "" {
		// Find the change ID for the parent portal
		for _, change := range plan.Changes {
			if change.ResourceType == ResourceTypePortal && change.ResourceRef == snippet.Portal {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypePortalSnippet, snippet.GetRef()),
		ResourceType: ResourceTypePortalSnippet,
		ResourceRef:  snippet.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    dependencies,
		Namespace:    parentNamespace,
	}

	// Store parent portal reference
	if snippet.Portal != "" {
		// Find the portal in desiredPortals to get its name
		var portalName string
		for _, portal := range p.desiredPortals {
			if portal.Ref == snippet.Portal {
				portalName = portal.Name
				break
			}
		}

		// Set Parent field for proper display and serialization
		change.Parent = &ParentInfo{
			Ref: snippet.Portal,
			ID:  portalID, // May be empty if portal ID is not known yet
		}

		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: snippet.Portal,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
		// If we already know the Konnect portal ID, include it to avoid name lookup during execution
		if portalID != "" {
			ref := change.References["portal_id"]
			ref.ID = portalID
			change.References["portal_id"] = ref
		}
	}

	plan.AddChange(change)
} // shouldUpdatePortalSnippet checks if a portal snippet needs updating
func (p *Planner) shouldUpdatePortalSnippet(
	current *state.PortalSnippet,
	desired resources.PortalSnippetResource,
) (bool, map[string]any) {
	updates := make(map[string]any)

	// Compare content (always present)
	if current.Content != desired.Content {
		updates["content"] = desired.Content
	}

	// Compare title if set
	if desired.Title != nil && current.Title != *desired.Title {
		updates["title"] = *desired.Title
	}

	// Compare description if set
	if desired.Description != nil && current.Description != *desired.Description {
		updates["description"] = *desired.Description
	}

	// Compare visibility if set
	if desired.Visibility != nil {
		desiredVis := string(*desired.Visibility)
		if current.Visibility != desiredVis {
			updates["visibility"] = desiredVis
		}
	}

	// Compare status if set
	if desired.Status != nil {
		desiredStatus := string(*desired.Status)
		if current.Status != desiredStatus {
			updates["status"] = desiredStatus
		}
	}

	// Note: We don't update name as that's the identifier

	return len(updates) > 0, updates
}

// planPortalSnippetUpdate creates an UPDATE change for a portal snippet
func (p *Planner) planPortalSnippetUpdate(
	parentNamespace string,
	current state.PortalSnippet,
	desired resources.PortalSnippetResource,
	portalRef string,
	updateFields map[string]any,
	plan *Plan,
) {
	fields := make(map[string]any)

	// Always include name for identification
	fields["name"] = current.Name

	// Add fields that need updating
	for field, value := range updateFields {
		fields[field] = value
	}

	// Determine dependencies - depends on parent portal
	var dependencies []string
	if portalRef != "" {
		// Find the change ID for the parent portal
		for _, change := range plan.Changes {
			if change.ResourceType == ResourceTypePortal && change.ResourceRef == portalRef {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypePortalSnippet, desired.GetRef()),
		ResourceType: ResourceTypePortalSnippet,
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    dependencies,
		Namespace:    parentNamespace,
	}

	// Store parent portal reference
	if portalRef != "" {
		// Find the portal in desiredPortals to get its name
		var portalName string
		for _, portal := range p.desiredPortals {
			if portal.Ref == portalRef {
				portalName = portal.Name
				break
			}
		}

		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: portalRef,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
	}

	plan.AddChange(change)
}

// Portal Team planning

func (p *Planner) planPortalTeamsChanges(
	ctx context.Context, parentNamespace string, portalID string, portalRef string,
	desired []resources.PortalTeamResource, plan *Plan,
) error {
	if p.logger != nil {
		p.logger.Debug("Planning portal team changes",
			slog.String("portal_ref", portalRef),
			slog.String("namespace", parentNamespace),
			slog.Int("desired_count", len(desired)))
	}

	// Fetch existing teams for this portal
	existingTeams := make(map[string]state.PortalTeam)
	if portalID != "" {
		teams, err := p.client.ListPortalTeams(ctx, portalID)
		if err != nil {
			// If portal team API is not configured, skip processing
			if strings.Contains(err.Error(), "portal team API not configured") {
				return nil
			}
			return fmt.Errorf("failed to list existing portal teams for portal %s: %w", portalID, err)
		}
		for _, team := range teams {
			existingTeams[team.Name] = team
		}
	}

	if p.logger != nil {
		p.logger.Debug("Fetched existing portal teams",
			slog.String("portal_ref", portalRef),
			slog.Int("existing_count", len(existingTeams)))
	}

	// Check for duplicate team names in desired state
	nameCount := make(map[string]int)
	for _, team := range desired {
		nameCount[team.Name]++
	}
	for name, count := range nameCount {
		if count > 1 {
			return fmt.Errorf(
				"duplicate team name %q found in portal %q: team names must be unique within a portal",
				name, portalRef)
		}
	}

	// Check for duplicate team names in existing teams
	if len(existingTeams) > 0 {
		existingNameCount := make(map[string]int)
		for _, team := range existingTeams {
			existingNameCount[team.Name]++
		}
		for name, count := range existingNameCount {
			if count > 1 {
				return fmt.Errorf(
					"multiple existing teams found with name %q in portal %q: cannot manage teams with duplicate names",
					name, portalRef)
			}
		}
	}

	desiredNames := make(map[string]bool)
	for _, desiredTeam := range desired {
		if plan.HasChange(ResourceTypePortalTeam, desiredTeam.GetRef()) {
			continue
		}
		desiredNames[desiredTeam.Name] = true

		if existingTeam, exists := existingTeams[desiredTeam.Name]; exists {
			// Team exists: check for updates
			// Since name is the identifier, if name changes, it's a different resource
			// Only description can be updated
			if shouldUpdate, updateFields := p.shouldUpdatePortalTeam(existingTeam, desiredTeam); shouldUpdate {
				p.planPortalTeamUpdate(
					parentNamespace,
					existingTeam,
					desiredTeam,
					portalRef,
					updateFields,
					plan,
				)
			}
		} else {
			// Team doesn't exist: create
			p.planPortalTeamCreate(parentNamespace, desiredTeam, portalID, plan)
		}
	}

	// In SYNC mode: Delete teams not in desired state
	if plan.Metadata.Mode == PlanModeSync {
		for _, existingTeam := range existingTeams {
			if !desiredNames[existingTeam.Name] {
				p.planPortalTeamDelete(parentNamespace, portalRef, portalID, existingTeam, plan)
			}
		}
	}

	return nil
}

func (p *Planner) planPortalTeamCreate(
	parentNamespace string, team resources.PortalTeamResource, portalID string, plan *Plan,
) {
	if p.logger != nil {
		p.logger.Debug("Plan portal team create",
			slog.String("team_ref", team.GetRef()),
			slog.String("portal_ref", team.Portal),
			slog.String("namespace", parentNamespace),
			slog.String("portal_id", portalID))
	}

	fields := make(map[string]any)
	fields["name"] = team.Name
	if team.Description != nil {
		fields["description"] = *team.Description
	}

	// Determine dependencies - depends on parent portal
	var dependencies []string
	if team.Portal != "" {
		// Find the change ID for the parent portal if it's being created
		for _, change := range plan.Changes {
			if change.ResourceType == ResourceTypePortal && change.ResourceRef == team.Portal {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypePortalTeam, team.GetRef()),
		ResourceType: ResourceTypePortalTeam,
		ResourceRef:  team.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    dependencies,
		Namespace:    parentNamespace,
	}

	// Store parent portal reference
	if team.Portal != "" {
		// Find the portal in desiredPortals to get its name
		var portalName string
		for _, portal := range p.desiredPortals {
			if portal.Ref == team.Portal {
				portalName = portal.Name
				break
			}
		}

		if portalID != "" {
			change.Parent = &ParentInfo{
				Ref: team.Portal,
				ID:  portalID,
			}
		} else {
			change.References = map[string]ReferenceInfo{
				"portal_id": {
					Ref: team.Portal,
					LookupFields: map[string]string{
						"name": portalName,
					},
				},
			}
		}
	}

	plan.AddChange(change)

	if p.logger != nil {
		p.logger.Debug("Queued portal team create change",
			slog.String("change_id", change.ID),
			slog.String("team_ref", team.GetRef()),
			slog.String("portal_ref", team.Portal))
	}
}

func (p *Planner) shouldUpdatePortalTeam(
	current state.PortalTeam, desired resources.PortalTeamResource,
) (bool, map[string]any) {
	updateFields := make(map[string]any)

	// Only description can be updated (name is the identifier)
	desiredDesc := ""
	if desired.Description != nil {
		desiredDesc = *desired.Description
	}

	if current.Description != desiredDesc {
		updateFields["description"] = desiredDesc
	}

	return len(updateFields) > 0, updateFields
}

// planPortalTeamUpdate creates an UPDATE change for a portal team
func (p *Planner) planPortalTeamUpdate(
	parentNamespace string,
	current state.PortalTeam,
	desired resources.PortalTeamResource,
	portalRef string,
	updateFields map[string]any,
	plan *Plan,
) {
	fields := make(map[string]any)
	for field, value := range updateFields {
		fields[field] = value
	}

	// Determine dependencies - depends on parent portal
	var dependencies []string
	if portalRef != "" {
		// Find the change ID for the parent portal
		for _, change := range plan.Changes {
			if change.ResourceType == ResourceTypePortal && change.ResourceRef == portalRef {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypePortalTeam, desired.GetRef()),
		ResourceType: ResourceTypePortalTeam,
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    dependencies,
		Namespace:    parentNamespace,
	}

	// Store parent portal reference
	if portalRef != "" {
		// Find the portal in desiredPortals to get its name
		var portalName string
		for _, portal := range p.desiredPortals {
			if portal.Ref == portalRef {
				portalName = portal.Name
				break
			}
		}

		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: portalRef,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
	}

	plan.AddChange(change)

	if p.logger != nil {
		p.logger.Debug("Queued portal team update change",
			slog.String("change_id", change.ID),
			slog.String("team_ref", desired.GetRef()),
			slog.String("portal_ref", portalRef),
			slog.Any("fields", fields))
	}
}

func (p *Planner) planPortalTeamDelete(
	parentNamespace string, portalRef string, portalID string, team state.PortalTeam, plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypePortalTeam, team.Name),
		ResourceType: ResourceTypePortalTeam,
		ResourceRef:  team.Name,
		ResourceID:   team.ID,
		Action:       ActionDelete,
		Fields:       map[string]any{"name": team.Name},
		Namespace:    parentNamespace,
	}

	// Store parent portal reference
	if portalRef != "" {
		// Find the portal in desiredPortals to get its name
		var portalName string
		for _, portal := range p.desiredPortals {
			if portal.Ref == portalRef {
				portalName = portal.Name
				break
			}
		}

		if portalID != "" {
			change.Parent = &ParentInfo{
				Ref: portalRef,
				ID:  portalID,
			}
		} else {
			change.References = map[string]ReferenceInfo{
				"portal_id": {
					Ref: portalRef,
					LookupFields: map[string]string{
						"name": portalName,
					},
				},
			}
		}
	}

	plan.AddChange(change)

	if p.logger != nil {
		p.logger.Debug("Queued portal team delete change",
			slog.String("change_id", change.ID),
			slog.String("team_name", team.Name),
			slog.String("portal_ref", portalRef),
			slog.String("resource_id", team.ID))
	}
}

// Portal Team Role planning

func (p *Planner) planPortalTeamRolesChanges(
	ctx context.Context,
	parentNamespace string,
	portalID string,
	portalRef string,
	portalName string,
	plan *Plan,
) error {
	if p.logger != nil {
		p.logger.Debug("Planning portal team role changes",
			slog.String("portal_ref", portalRef),
			slog.String("namespace", parentNamespace))
	}

	rolesByTeam := make(map[string][]resources.PortalTeamRoleResource)
	for _, role := range p.desiredPortalTeamRoles {
		if role.Portal == portalRef {
			rolesByTeam[role.Team] = append(rolesByTeam[role.Team], role)
		}
	}

	if len(rolesByTeam) == 0 {
		return nil
	}

	teamByRef := make(map[string]*resources.PortalTeamResource)
	for i := range p.desiredPortalTeams {
		team := p.desiredPortalTeams[i]
		if team.Portal == portalRef {
			teamByRef[team.Ref] = &team
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		for teamRef := range teamByRef {
			if _, ok := rolesByTeam[teamRef]; !ok {
				rolesByTeam[teamRef] = []resources.PortalTeamRoleResource{}
			}
		}
	}

	existingTeamsByName := make(map[string]state.PortalTeam)
	if portalID != "" {
		teams, err := p.client.ListPortalTeams(ctx, portalID)
		if err != nil {
			// If portal team API is not configured, skip processing
			if strings.Contains(err.Error(), "portal team API not configured") {
				return nil
			}
			return fmt.Errorf("failed to list existing portal teams for portal %s: %w", portalID, err)
		}
		for _, team := range teams {
			existingTeamsByName[team.Name] = team
		}
	}

	existingRolesCache := make(map[string]map[string]state.PortalTeamRole)
	teamsDeleted := make(map[string]bool)
	for _, change := range plan.Changes {
		if change.ResourceType == ResourceTypePortalTeam && change.ResourceRef != "" && change.Action == ActionDelete {
			teamsDeleted[change.ResourceRef] = true
		}
	}

	for teamRef, desiredRoles := range rolesByTeam {
		if teamsDeleted[teamRef] {
			continue
		}

		// Build an entity ID for comparison that resolves any ref placeholders
		resolveEntityID := func(entityID string) string {
			if p.resources == nil || !tags.IsRefPlaceholder(entityID) {
				return entityID
			}

			ref, field, ok := tags.ParseRefPlaceholder(entityID)
			if !ok || (field != "" && field != "id" && field != "ID") {
				return entityID
			}

			if api := p.resources.GetAPIByRef(ref); api != nil && api.GetKonnectID() != "" {
				return api.GetKonnectID()
			}

			return entityID
		}

		teamName := ""
		if team, ok := teamByRef[teamRef]; ok {
			teamName = team.Name
		}

		teamID := ""
		if teamName != "" {
			if existingTeam, ok := existingTeamsByName[teamName]; ok {
				teamID = existingTeam.ID
			}
		}

		existingRoles := make(map[string]state.PortalTeamRole)
		if teamID != "" {
			if cached, ok := existingRolesCache[teamID]; ok {
				existingRoles = cached
			} else {
				roles, err := p.client.ListPortalTeamRoles(ctx, portalID, teamID)
				if err != nil {
					return fmt.Errorf("failed to list portal team roles for team %s: %w", teamID, err)
				}
				for _, role := range roles {
					key := buildPortalTeamRoleKey(role.RoleName, role.EntityID, role.EntityTypeName, role.EntityRegion)
					existingRoles[key] = role
				}
				existingRolesCache[teamID] = existingRoles
			}
		}

		desiredKeys := make(map[string]bool)
		for _, role := range desiredRoles {
			key := buildPortalTeamRoleKey(
				role.RoleName,
				resolveEntityID(role.EntityID),
				role.EntityTypeName,
				role.EntityRegion,
			)
			if desiredKeys[key] {
				return fmt.Errorf("duplicate portal team role assignment %q for team %q", key, teamRef)
			}
			desiredKeys[key] = true

			if _, exists := existingRoles[key]; exists {
				continue
			}

			p.planPortalTeamRoleCreate(
				parentNamespace,
				portalRef,
				portalName,
				portalID,
				teamRef,
				teamName,
				teamID,
				findChangeID(plan, ResourceTypePortalTeam, teamRef),
				role,
				plan,
			)
		}

		if plan.Metadata.Mode == PlanModeSync && teamID != "" {
			for key, existingRole := range existingRoles {
				if !desiredKeys[key] {
					p.planPortalTeamRoleDelete(
						parentNamespace,
						portalRef,
						portalName,
						portalID,
						teamRef,
						teamName,
						teamID,
						existingRole,
						plan,
					)
				}
			}
		}
	}

	return nil
}

func (p *Planner) planPortalTeamRoleCreate(
	parentNamespace string,
	portalRef string,
	portalName string,
	portalID string,
	teamRef string,
	teamName string,
	teamID string,
	teamChangeID string,
	role resources.PortalTeamRoleResource,
	plan *Plan,
) {
	fields := map[string]any{
		"role_name":        role.RoleName,
		"entity_id":        role.EntityID,
		"entity_type_name": role.EntityTypeName,
		"entity_region":    role.EntityRegion,
	}

	var dependencies []string
	if teamChangeID != "" {
		dependencies = append(dependencies, teamChangeID)
	}

	portalChangeID := findChangeID(plan, ResourceTypePortal, portalRef)
	if portalChangeID != "" {
		dependencies = append(dependencies, portalChangeID)
	}

	if tags.IsRefPlaceholder(role.EntityID) {
		if apiRef, _, ok := tags.ParseRefPlaceholder(role.EntityID); ok {
			if apiChangeID := findChangeID(plan, string(resources.ResourceTypeAPI), apiRef); apiChangeID != "" {
				dependencies = append(dependencies, apiChangeID)
			}
		}
	}

	refs := map[string]ReferenceInfo{
		"portal_id": {
			Ref: portalRef,
			LookupFields: map[string]string{
				"name": portalName,
			},
		},
		"team_id": {
			Ref: teamRef,
			LookupFields: map[string]string{
				"name": teamName,
			},
		},
	}

	if portalID != "" {
		refs["portal_id"] = ReferenceInfo{
			Ref: portalRef,
			ID:  portalID,
		}
	}

	if teamID != "" {
		refs["team_id"] = ReferenceInfo{
			Ref: teamRef,
			ID:  teamID,
			LookupFields: map[string]string{
				"name": teamName,
			},
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypePortalTeamRole, role.GetRef()),
		ResourceType: ResourceTypePortalTeamRole,
		ResourceRef:  role.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    dependencies,
		Namespace:    parentNamespace,
		References:   refs,
	}

	plan.AddChange(change)

	if p.logger != nil {
		p.logger.Debug("Queued portal team role create change",
			slog.String("change_id", change.ID),
			slog.String("team_ref", teamRef),
			slog.String("portal_ref", portalRef),
			slog.Any("fields", fields))
	}
}

func (p *Planner) planPortalTeamRoleDelete(
	parentNamespace string,
	portalRef string,
	portalName string,
	portalID string,
	teamRef string,
	teamName string,
	teamID string,
	role state.PortalTeamRole,
	plan *Plan,
) {
	refs := map[string]ReferenceInfo{
		"portal_id": {
			Ref: portalRef,
			ID:  portalID,
			LookupFields: map[string]string{
				"name": portalName,
			},
		},
		"team_id": {
			Ref: teamRef,
			ID:  teamID,
			LookupFields: map[string]string{
				"name": teamName,
			},
		},
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypePortalTeamRole, role.RoleName),
		ResourceType: ResourceTypePortalTeamRole,
		ResourceRef:  role.RoleName,
		ResourceID:   role.ID,
		Action:       ActionDelete,
		Fields: map[string]any{
			"role_name":        role.RoleName,
			"entity_id":        role.EntityID,
			"entity_type_name": role.EntityTypeName,
			"entity_region":    role.EntityRegion,
		},
		DependsOn:  []string{},
		Namespace:  parentNamespace,
		References: refs,
		Parent: &ParentInfo{
			Ref: teamRef,
			ID:  teamID,
		},
	}

	plan.AddChange(change)

	if p.logger != nil {
		p.logger.Debug("Queued portal team role delete change",
			slog.String("change_id", change.ID),
			slog.String("team_ref", teamRef),
			slog.String("portal_ref", portalRef),
			slog.Any("fields", change.Fields))
	}
}

func buildPortalTeamRoleKey(roleName, entityID, entityTypeName, entityRegion string) string {
	return fmt.Sprintf("%s|%s|%s|%s", roleName, entityID, entityTypeName, strings.ToLower(entityRegion))
}

func findChangeID(plan *Plan, resourceType string, resourceRef string) string {
	for _, change := range plan.Changes {
		if change.ResourceType == resourceType && change.ResourceRef == resourceRef {
			return change.ID
		}
	}
	return ""
}
