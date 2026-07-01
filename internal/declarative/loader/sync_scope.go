package loader

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
	"sigs.k8s.io/yaml"
)

type rootCollectionScope struct {
	key          string
	resourceType resources.ResourceType
}

type childCollectionScope struct {
	key          string
	resourceType resources.ResourceType
	parentKey    string
	parentType   resources.ResourceType
}

var rootCollectionScopes = []rootCollectionScope{
	{key: "portals", resourceType: resources.ResourceTypePortal},
	{key: "application_auth_strategies", resourceType: resources.ResourceTypeApplicationAuthStrategy},
	{key: "dcr_providers", resourceType: resources.ResourceTypeDCRProvider},
	{key: "control_planes", resourceType: resources.ResourceTypeControlPlane},
	{key: "catalog_services", resourceType: resources.ResourceTypeCatalogService},
	{key: "apis", resourceType: resources.ResourceTypeAPI},
	{key: "event_gateways", resourceType: resources.ResourceTypeEventGatewayControlPlane},
}

var rootChildCollectionScopes = []childCollectionScope{
	{
		key:          "control_plane_data_plane_certificates",
		resourceType: resources.ResourceTypeControlPlaneDataPlaneCertificate,
		parentKey:    "control_plane",
		parentType:   resources.ResourceTypeControlPlane,
	},
	{
		key:          "api_versions",
		resourceType: resources.ResourceTypeAPIVersion,
		parentKey:    "api",
		parentType:   resources.ResourceTypeAPI,
	},
	{
		key:          "api_publications",
		resourceType: resources.ResourceTypeAPIPublication,
		parentKey:    "api",
		parentType:   resources.ResourceTypeAPI,
	},
	{
		key:          "api_implementations",
		resourceType: resources.ResourceTypeAPIImplementation,
		parentKey:    "api",
		parentType:   resources.ResourceTypeAPI,
	},
	{
		key:          "api_documents",
		resourceType: resources.ResourceTypeAPIDocument,
		parentKey:    "api",
		parentType:   resources.ResourceTypeAPI,
	},
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
		key:          "event_gateway_backend_clusters",
		resourceType: resources.ResourceTypeEventGatewayBackendCluster,
		parentKey:    "event_gateway",
		parentType:   resources.ResourceTypeEventGatewayControlPlane,
	},
	{
		key:          "event_gateway_virtual_clusters",
		resourceType: resources.ResourceTypeEventGatewayVirtualCluster,
		parentKey:    "event_gateway",
		parentType:   resources.ResourceTypeEventGatewayControlPlane,
	},
	{
		key:          "event_gateway_listeners",
		resourceType: resources.ResourceTypeEventGatewayListener,
		parentKey:    "event_gateway",
		parentType:   resources.ResourceTypeEventGatewayControlPlane,
	},
	{
		key:          "event_gateway_data_plane_certificates",
		resourceType: resources.ResourceTypeEventGatewayDataPlaneCertificate,
		parentKey:    "event_gateway",
		parentType:   resources.ResourceTypeEventGatewayControlPlane,
	},
	{
		key:          "event_gateway_schema_registries",
		resourceType: resources.ResourceTypeEventGatewaySchemaRegistry,
		parentKey:    "event_gateway",
		parentType:   resources.ResourceTypeEventGatewayControlPlane,
	},
	{
		key:          "event_gateway_static_keys",
		resourceType: resources.ResourceTypeEventGatewayStaticKey,
		parentKey:    "event_gateway",
		parentType:   resources.ResourceTypeEventGatewayControlPlane,
	},
	{
		key:          "event_gateway_tls_trust_bundles",
		resourceType: resources.ResourceTypeEventGatewayTLSTrustBundle,
		parentKey:    "event_gateway",
		parentType:   resources.ResourceTypeEventGatewayControlPlane,
	},
	{
		key:          "event_gateway_listener_policies",
		resourceType: resources.ResourceTypeEventGatewayListenerPolicy,
		parentKey:    "listener",
		parentType:   resources.ResourceTypeEventGatewayListener,
	},
	{
		key:          "event_gateway_virtual_cluster_cluster_policies",
		resourceType: resources.ResourceTypeEventGatewayClusterPolicy,
		parentKey:    "virtual_cluster",
		parentType:   resources.ResourceTypeEventGatewayVirtualCluster,
	},
	{
		key:          "event_gateway_virtual_cluster_produce_policies",
		resourceType: resources.ResourceTypeEventGatewayProducePolicy,
		parentKey:    "virtual_cluster",
		parentType:   resources.ResourceTypeEventGatewayVirtualCluster,
	},
	{
		key:          "event_gateway_virtual_cluster_consume_policies",
		resourceType: resources.ResourceTypeEventGatewayConsumePolicy,
		parentKey:    "virtual_cluster",
		parentType:   resources.ResourceTypeEventGatewayVirtualCluster,
	},
	{
		key:          "organization_team_roles",
		resourceType: resources.ResourceTypeOrganizationTeamRole,
		parentKey:    "team",
		parentType:   resources.ResourceTypeOrganizationTeam,
	},
}

var apiChildCollectionScopes = []childCollectionScope{
	{key: "versions", resourceType: resources.ResourceTypeAPIVersion, parentType: resources.ResourceTypeAPI},
	{key: "publications", resourceType: resources.ResourceTypeAPIPublication, parentType: resources.ResourceTypeAPI},
	{
		key:          "implementations",
		resourceType: resources.ResourceTypeAPIImplementation,
		parentType:   resources.ResourceTypeAPI,
	},
	{key: "documents", resourceType: resources.ResourceTypeAPIDocument, parentType: resources.ResourceTypeAPI},
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

var eventGatewayChildCollectionScopes = []childCollectionScope{
	{
		key:          "backend_clusters",
		resourceType: resources.ResourceTypeEventGatewayBackendCluster,
		parentType:   resources.ResourceTypeEventGatewayControlPlane,
	},
	{
		key:          "virtual_clusters",
		resourceType: resources.ResourceTypeEventGatewayVirtualCluster,
		parentType:   resources.ResourceTypeEventGatewayControlPlane,
	},
	{
		key:          "listeners",
		resourceType: resources.ResourceTypeEventGatewayListener,
		parentType:   resources.ResourceTypeEventGatewayControlPlane,
	},
	{
		key:          "data_plane_certificates",
		resourceType: resources.ResourceTypeEventGatewayDataPlaneCertificate,
		parentType:   resources.ResourceTypeEventGatewayControlPlane,
	},
	{
		key:          "schema_registries",
		resourceType: resources.ResourceTypeEventGatewaySchemaRegistry,
		parentType:   resources.ResourceTypeEventGatewayControlPlane,
	},
	{
		key:          "static_keys",
		resourceType: resources.ResourceTypeEventGatewayStaticKey,
		parentType:   resources.ResourceTypeEventGatewayControlPlane,
	},
	{
		key:          "tls_trust_bundles",
		resourceType: resources.ResourceTypeEventGatewayTLSTrustBundle,
		parentType:   resources.ResourceTypeEventGatewayControlPlane,
	},
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
	for _, entry := range rootCollectionScopes {
		if _, ok := raw[entry.key]; ok {
			scope.AddRoot(entry.resourceType)
		}
	}

	for _, entry := range rootChildCollectionScopes {
		captureRootChildScope(scope, raw, entry)
	}

	captureNestedCollectionScopes(scope, raw, "apis", resources.ResourceTypeAPI, apiChildCollectionScopes)
	captureNestedCollectionScopes(
		scope,
		raw,
		"control_planes",
		resources.ResourceTypeControlPlane,
		[]childCollectionScope{
			{
				key:          "data_plane_certificates",
				resourceType: resources.ResourceTypeControlPlaneDataPlaneCertificate,
				parentType:   resources.ResourceTypeControlPlane,
			},
		},
	)
	captureNestedCollectionScopes(
		scope,
		raw,
		"event_gateways",
		resources.ResourceTypeEventGatewayControlPlane,
		eventGatewayChildCollectionScopes,
	)
	captureNestedEventGatewayScopes(scope, raw)
	if err := captureNestedPortalScopes(scope, raw); err != nil {
		return err
	}
	captureOrganizationScope(scope, raw)
	captureIdentityScope(scope, raw)
	captureAnalyticsScope(scope, raw)

	return nil
}

func captureRootChildScope(scope *resources.SyncScope, raw map[string]any, entry childCollectionScope) {
	value, ok := raw[entry.key]
	if !ok {
		return
	}
	items, ok := asSlice(value)
	if !ok || len(items) == 0 {
		scope.AddRootChildCollection(entry.resourceType)
		return
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
}

func captureNestedCollectionScopes(
	scope *resources.SyncScope,
	raw map[string]any,
	rootKey string,
	parentType resources.ResourceType,
	childScopes []childCollectionScope,
) {
	items, ok := asSlice(raw[rootKey])
	if !ok {
		return
	}
	for _, item := range items {
		parent, ok := asMap(item)
		if !ok {
			continue
		}
		parentRef := stringValue(parent[resources.SchemaFieldRef])
		if parentRef == "" {
			continue
		}
		for _, child := range childScopes {
			if _, ok := parent[child.key]; ok {
				scope.AddChild(parentType, parentRef, child.resourceType)
			}
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

func captureNestedEventGatewayScopes(scope *resources.SyncScope, raw map[string]any) {
	gateways, ok := asSlice(raw["event_gateways"])
	if !ok {
		return
	}
	for _, item := range gateways {
		gateway, ok := asMap(item)
		if !ok {
			continue
		}
		captureVirtualClusterPolicyScopes(scope, gateway["virtual_clusters"])
		captureListenerPolicyScopes(scope, gateway["listeners"])
	}
}

func captureVirtualClusterPolicyScopes(scope *resources.SyncScope, value any) {
	virtualClusters, ok := asSlice(value)
	if !ok {
		return
	}
	for _, item := range virtualClusters {
		vc, ok := asMap(item)
		if !ok {
			continue
		}
		ref := stringValue(vc[resources.SchemaFieldRef])
		if ref == "" {
			continue
		}
		if _, ok := vc["cluster_policies"]; ok {
			scope.AddChild(
				resources.ResourceTypeEventGatewayVirtualCluster,
				ref,
				resources.ResourceTypeEventGatewayClusterPolicy,
			)
		}
		if _, ok := vc["produce_policies"]; ok {
			scope.AddChild(
				resources.ResourceTypeEventGatewayVirtualCluster,
				ref,
				resources.ResourceTypeEventGatewayProducePolicy,
			)
		}
		if _, ok := vc["consume_policies"]; ok {
			scope.AddChild(
				resources.ResourceTypeEventGatewayVirtualCluster,
				ref,
				resources.ResourceTypeEventGatewayConsumePolicy,
			)
		}
	}
}

func captureListenerPolicyScopes(scope *resources.SyncScope, value any) {
	listeners, ok := asSlice(value)
	if !ok {
		return
	}
	for _, item := range listeners {
		listener, ok := asMap(item)
		if !ok {
			continue
		}
		ref := stringValue(listener[resources.SchemaFieldRef])
		if ref == "" {
			continue
		}
		if _, ok := listener["policies"]; ok {
			scope.AddChild(
				resources.ResourceTypeEventGatewayListener,
				ref,
				resources.ResourceTypeEventGatewayListenerPolicy,
			)
		}
	}
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

func captureIdentityScope(scope *resources.SyncScope, raw map[string]any) {
	identity, ok := asMap(raw["identity"])
	if !ok {
		return
	}
	if _, ok := identity["directories"]; ok {
		scope.AddRoot(resources.ResourceTypeIdentityDirectory)
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
