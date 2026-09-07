package loader

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
	"sigs.k8s.io/yaml"
)

type childCollectionScope struct {
	key              string
	resourceType     resources.ResourceType
	parentKey        string
	parentType       resources.ResourceType
	emptyRootMessage string
}

var rootChildCollectionScopes = []childCollectionScope{
	{
		key:          "portal_customizations",
		resourceType: resources.ResourceTypePortalCustomization,
		parentKey:    resources.SchemaFieldPortal,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "portal_auth_settings",
		resourceType: resources.ResourceTypePortalAuthSettings,
		parentKey:    resources.SchemaFieldPortal,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "portal_ip_allow_lists",
		resourceType: resources.ResourceTypePortalIPAllowList,
		parentKey:    resources.SchemaFieldPortal,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "portal_integrations",
		resourceType: resources.ResourceTypePortalIntegration,
		parentKey:    resources.SchemaFieldPortal,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "portal_identity_providers",
		resourceType: resources.ResourceTypePortalIdentityProvider,
		parentKey:    resources.SchemaFieldPortal,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "portal_team_group_mappings",
		resourceType: resources.ResourceTypePortalTeamGroupMapping,
		parentKey:    resources.SchemaFieldPortal,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "portal_custom_domains",
		resourceType: resources.ResourceTypePortalCustomDomain,
		parentKey:    resources.SchemaFieldPortal,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "portal_pages",
		resourceType: resources.ResourceTypePortalPage,
		parentKey:    resources.SchemaFieldPortal,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "portal_snippets",
		resourceType: resources.ResourceTypePortalSnippet,
		parentKey:    resources.SchemaFieldPortal,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "portal_teams",
		resourceType: resources.ResourceTypePortalTeam,
		parentKey:    resources.SchemaFieldPortal,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "portal_team_roles",
		resourceType: resources.ResourceTypePortalTeamRole,
		parentKey:    resources.SchemaFieldPortal,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "portal_asset_logos",
		resourceType: resources.ResourceTypePortalAssetLogo,
		parentKey:    resources.SchemaFieldPortal,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "portal_asset_favicons",
		resourceType: resources.ResourceTypePortalAssetFavicon,
		parentKey:    resources.SchemaFieldPortal,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "portal_email_configs",
		resourceType: resources.ResourceTypePortalEmailConfig,
		parentKey:    resources.SchemaFieldPortal,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "portal_email_templates",
		resourceType: resources.ResourceTypePortalEmailTemplate,
		parentKey:    resources.SchemaFieldPortal,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "portal_audit_log_webhooks",
		resourceType: resources.ResourceTypePortalAuditLogWebhook,
		parentKey:    resources.SchemaFieldPortal,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "organization_team_roles",
		resourceType: resources.ResourceTypeOrganizationTeamRole,
		parentKey:    "team",
		parentType:   resources.ResourceTypeOrganizationTeam,
	},
	{
		key:          "organization_user_team_memberships",
		resourceType: resources.ResourceTypeOrganizationUserTeamMembership,
		parentKey:    resources.SchemaFieldUser,
		parentType:   resources.ResourceTypeOrganizationUser,
	},
	{
		key:          "organization_user_roles",
		resourceType: resources.ResourceTypeOrganizationUserRole,
		parentKey:    resources.SchemaFieldUser,
		parentType:   resources.ResourceTypeOrganizationUser,
	},
	{
		key:          "organization_system_account_team_memberships",
		resourceType: resources.ResourceTypeOrganizationSystemAccountTeamMembership,
		parentKey:    resources.SchemaFieldSystemAccount,
		parentType:   resources.ResourceTypeOrganizationSystemAccount,
	},
	{
		key:          "organization_system_account_roles",
		resourceType: resources.ResourceTypeOrganizationSystemAccountRole,
		parentKey:    resources.SchemaFieldSystemAccount,
		parentType:   resources.ResourceTypeOrganizationSystemAccount,
	},
}

var portalChildCollectionScopes = []childCollectionScope{
	{
		key:          "customization",
		resourceType: resources.ResourceTypePortalCustomization,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "auth_settings",
		resourceType: resources.ResourceTypePortalAuthSettings,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "ip_allow_list",
		resourceType: resources.ResourceTypePortalIPAllowList,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "integrations",
		resourceType: resources.ResourceTypePortalIntegration,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "identity_providers",
		resourceType: resources.ResourceTypePortalIdentityProvider,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "custom_domain",
		resourceType: resources.ResourceTypePortalCustomDomain,
		parentType:   resources.ResourceTypePortal,
	},
	{key: "pages", resourceType: resources.ResourceTypePortalPage, parentType: resources.ResourceTypePortal},
	{key: "snippets", resourceType: resources.ResourceTypePortalSnippet, parentType: resources.ResourceTypePortal},
	{
		key:          resources.SchemaFieldTeams,
		resourceType: resources.ResourceTypePortalTeam,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "email_config",
		resourceType: resources.ResourceTypePortalEmailConfig,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "email_templates",
		resourceType: resources.ResourceTypePortalEmailTemplate,
		parentType:   resources.ResourceTypePortal,
	},
	{
		key:          "audit_log_webhook",
		resourceType: resources.ResourceTypePortalAuditLogWebhook,
		parentType:   resources.ResourceTypePortal,
	},
}

var portalSingletonChildKeys = map[string]struct{}{
	"customization":     {},
	"auth_settings":     {},
	"ip_allow_list":     {},
	"integrations":      {},
	"custom_domain":     {},
	"email_config":      {},
	"email_templates":   {},
	"audit_log_webhook": {},
}

func captureSyncScope(content []byte, rs *resources.ResourceSet) error {
	var raw map[string]any
	// Called after strict parsing succeeds; use a relaxed pass only to inspect
	// YAML key presence before nested resources are extracted.
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return fmt.Errorf("failed to inspect sync scope: %w", err)
	}
	if len(raw) == 0 {
		return nil
	}

	scope := rs.EnsureSyncScope()
	collections := resources.SyncCollections()
	for _, collection := range collections {
		if collection.ParentType == "" {
			if _, ok := raw[collection.RootKey]; ok {
				scope.AddRoot(collection.ResourceType)
			}
		}
	}

	if err := captureRegisteredChildScopes(scope, raw, collections); err != nil {
		return err
	}
	for _, entry := range rootChildCollectionScopes {
		if err := captureRootChildScope(scope, raw, entry); err != nil {
			return err
		}
	}

	if err := captureNestedPortalScopes(scope, raw); err != nil {
		return err
	}
	captureOrganizationScope(scope, raw)
	captureAnalyticsScope(scope, raw)

	return nil
}

func captureRegisteredChildScopes(
	scope *resources.SyncScope,
	raw map[string]any,
	collections []resources.SyncCollection,
) error {
	for _, collection := range collections {
		if collection.ParentType == "" {
			continue
		}
		entry := childCollectionScope{
			key:              collection.RootKey,
			resourceType:     collection.ResourceType,
			parentKey:        collection.ParentKey,
			parentType:       collection.ParentType,
			emptyRootMessage: collection.EmptyRootMessage,
		}
		if err := captureRootChildScope(scope, raw, entry); err != nil {
			return err
		}
	}
	for _, collection := range collections {
		for _, path := range collection.NestedPaths {
			captureNestedCollectionPath(scope, raw, path, collection.ParentType, collection.ResourceType)
		}
	}
	return nil
}

func captureRootChildScope(scope *resources.SyncScope, raw map[string]any, entry childCollectionScope) error {
	value, ok := raw[entry.key]
	if !ok {
		return nil
	}
	items, ok := asSlice(value)
	if !ok || len(items) == 0 {
		if entry.emptyRootMessage != "" {
			return fmt.Errorf("%s cannot be empty because %s", entry.key, entry.emptyRootMessage)
		}
		scope.AddRootChildCollection(entry.resourceType)
		return nil
	}
	for _, item := range items {
		m, ok := asMap(item)
		if !ok {
			continue
		}
		if parentRef := stringValue(m[entry.parentKey]); parentRef != "" {
			scope.AddChild(entry.parentType, parentRef, entry.resourceType)
		}
	}
	return nil
}

func captureNestedCollectionPath(
	scope *resources.SyncScope,
	raw map[string]any,
	path []string,
	parentType, resourceType resources.ResourceType,
) {
	if len(path) == 1 {
		if parentRef := stringValue(raw[resources.SchemaFieldRef]); parentRef != "" {
			if _, ok := raw[path[0]]; ok {
				scope.AddChild(parentType, parentRef, resourceType)
			}
		}
		return
	}
	items, ok := asSlice(raw[path[0]])
	if !ok {
		return
	}
	for _, item := range items {
		if parent, ok := asMap(item); ok {
			captureNestedCollectionPath(scope, parent, path[1:], parentType, resourceType)
		}
	}
}

func captureNestedPortalScopes(scope *resources.SyncScope, raw map[string]any) error {
	items, ok := asSlice(raw["portals"])
	if !ok {
		return nil
	}
	for _, item := range items {
		portal, ok := asMap(item)
		if !ok {
			continue
		}
		portalRef := stringValue(portal[resources.SchemaFieldRef])
		if portalRef == "" {
			continue
		}
		for key := range portalSingletonChildKeys {
			if value, ok := portal[key]; ok && value == nil {
				return fmt.Errorf(
					"portal %q child singleton %q cannot be null; omit the key to ignore it or provide an object to manage it",
					portalRef,
					key,
				)
			}
		}
		for _, child := range portalChildCollectionScopes {
			if _, ok := portal[child.key]; ok {
				scope.AddChild(resources.ResourceTypePortal, portalRef, child.resourceType)
				if child.resourceType == resources.ResourceTypePortalTeam {
					// Team role declarations are nested under teams, so a teams
					// key scopes both the teams and their role assignments.
					scope.AddChild(resources.ResourceTypePortal, portalRef, resources.ResourceTypePortalTeamRole)
					if portalTeamsIncludeGroupMappings(portal[child.key]) {
						scope.AddChild(
							resources.ResourceTypePortal,
							portalRef,
							resources.ResourceTypePortalTeamGroupMapping,
						)
					}
				}
			}
		}
		assetsValue, assetsPresent := portal["assets"]
		if assetsPresent && assetsValue == nil {
			return fmt.Errorf(
				"portal %q child singleton %q cannot be null; omit the key to ignore assets or provide an object to manage them",
				portalRef,
				"assets",
			)
		}
		if assets, ok := asMap(assetsValue); ok {
			if err := capturePortalAssetScope(
				scope, assets, "logo", "assets.logo", portalRef, resources.ResourceTypePortalAssetLogo,
			); err != nil {
				return err
			}
			if err := capturePortalAssetScope(
				scope, assets, "favicon", "assets.favicon", portalRef, resources.ResourceTypePortalAssetFavicon,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func capturePortalAssetScope(
	scope *resources.SyncScope,
	assets map[string]any,
	assetKey, qualifiedKey string,
	portalRef string,
	resourceType resources.ResourceType,
) error {
	value, ok := assets[assetKey]
	if !ok {
		return nil
	}
	if value == nil {
		return fmt.Errorf(
			"portal %q child singleton %q cannot be null; omit the key to ignore it or provide a value",
			portalRef,
			qualifiedKey,
		)
	}
	scope.AddChild(resources.ResourceTypePortal, portalRef, resourceType)
	return nil
}

func portalTeamsIncludeGroupMappings(value any) bool {
	teams, ok := asSlice(value)
	if !ok {
		return false
	}
	for _, item := range teams {
		team, ok := asMap(item)
		if !ok {
			continue
		}
		if _, ok := team["group_mappings"]; ok {
			return true
		}
	}
	return false
}

func captureOrganizationScope(scope *resources.SyncScope, raw map[string]any) {
	org, ok := asMap(raw["organization"])
	if !ok {
		return
	}
	if teams, ok := org[resources.SchemaFieldTeams]; ok {
		scope.AddRoot(resources.ResourceTypeOrganizationTeam)
		captureOrganizationTeamRoleScopes(scope, teams)
	}
	if _, ok := org["users"]; ok {
		scope.MarkOrganizationUsersScoped()
	}
	if _, ok := org["system-accounts"]; ok {
		scope.MarkOrganizationSystemAccountsScoped()
	}
}

func captureOrganizationTeamRoleScopes(scope *resources.SyncScope, value any) {
	teams, ok := asSlice(value)
	if !ok {
		return
	}
	for _, item := range teams {
		team, ok := asMap(item)
		if !ok {
			continue
		}
		ref := stringValue(team[resources.SchemaFieldRef])
		if ref == "" {
			continue
		}
		if _, ok := team["roles"]; ok {
			scope.AddChild(resources.ResourceTypeOrganizationTeam, ref, resources.ResourceTypeOrganizationTeamRole)
		}
	}
}

func captureAnalyticsScope(scope *resources.SyncScope, raw map[string]any) {
	analytics, ok := asMap(raw["analytics"])
	if !ok {
		return
	}
	if _, ok := analytics["dashboards"]; ok {
		scope.AddRoot(resources.ResourceTypeDashboard)
	}
}

func asMap(value any) (map[string]any, bool) {
	m, ok := value.(map[string]any)
	return m, ok
}

func asSlice(value any) ([]any, bool) {
	items, ok := value.([]any)
	return items, ok
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}
