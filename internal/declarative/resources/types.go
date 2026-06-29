package resources

import (
	"slices"

	"github.com/kong/kongctl/internal/declarative/tags"
)

// ResourceType represents the type of a declarative resource
type ResourceType string

// Resource type constants
const (
	ResourceTypePortal                                  ResourceType = "portal"
	ResourceTypeApplicationAuthStrategy                 ResourceType = "application_auth_strategy"
	ResourceTypeDCRProvider                             ResourceType = "dcr_provider"
	ResourceTypeControlPlane                            ResourceType = "control_plane"
	ResourceTypeControlPlaneGroup                       ResourceType = "control_plane_group"
	ResourceTypeAPI                                     ResourceType = "api"
	ResourceTypeAIGateway                               ResourceType = "ai_gateway"
	ResourceTypeAIGatewayProvider                       ResourceType = "ai_gateway_provider"
	ResourceTypeAIGatewayPolicy                         ResourceType = "ai_gateway_policy"
	ResourceTypeAIGatewayAgent                          ResourceType = "ai_gateway_agent"
	ResourceTypeAIGatewayConsumer                       ResourceType = "ai_gateway_consumer"
	ResourceTypeAIGatewayConsumerGroup                  ResourceType = "ai_gateway_consumer_group"
	ResourceTypeAIGatewayModel                          ResourceType = "ai_gateway_model"
	ResourceTypeAIGatewayMCPServer                      ResourceType = "ai_gateway_mcp_server"
	ResourceTypeAIGatewayVault                          ResourceType = "ai_gateway_vault"
	ResourceTypeAIGatewayNode                           ResourceType = "ai_gateway_node"
	ResourceTypeDashboard                               ResourceType = "dashboard"
	ResourceTypeAPIVersion                              ResourceType = "api_version"
	ResourceTypeAPIPublication                          ResourceType = "api_publication"
	ResourceTypeAPIImplementation                       ResourceType = "api_implementation"
	ResourceTypeAPIDocument                             ResourceType = "api_document"
	ResourceTypeGatewayService                          ResourceType = "gateway_service"
	ResourceTypeControlPlaneDataPlaneCertificate        ResourceType = "control_plane_data_plane_certificate"
	ResourceTypePortalCustomization                     ResourceType = "portal_customization"
	ResourceTypePortalCustomDomain                      ResourceType = "portal_custom_domain"
	ResourceTypePortalAuthSettings                      ResourceType = "portal_auth_settings"
	ResourceTypePortalIPAllowList                       ResourceType = "portal_ip_allow_list"
	ResourceTypePortalIntegration                       ResourceType = "portal_integration"
	ResourceTypePortalIdentityProvider                  ResourceType = "portal_identity_provider"
	ResourceTypePortalTeamGroupMapping                  ResourceType = "portal_team_group_mapping"
	ResourceTypePortalPage                              ResourceType = "portal_page"
	ResourceTypePortalSnippet                           ResourceType = "portal_snippet"
	ResourceTypePortalTeam                              ResourceType = "portal_team"
	ResourceTypePortalTeamRole                          ResourceType = "portal_team_role"
	ResourceTypePortalAssetLogo                         ResourceType = "portal_asset_logo"
	ResourceTypePortalAssetFavicon                      ResourceType = "portal_asset_favicon"
	ResourceTypePortalEmailConfig                       ResourceType = "portal_email_config"
	ResourceTypePortalEmailTemplate                     ResourceType = "portal_email_template"
	ResourceTypePortalAuditLogWebhook                   ResourceType = "portal_audit_log_webhook"
	ResourceTypeAuditLogWebhookDestination              ResourceType = "audit_log_webhook_destination"
	ResourceTypeCatalogService                          ResourceType = "catalog_service"
	ResourceTypeEventGatewayControlPlane                ResourceType = "event_gateway"
	ResourceTypeEventGatewayBackendCluster              ResourceType = "event_gateway_backend_cluster"
	ResourceTypeEventGatewayVirtualCluster              ResourceType = "event_gateway_virtual_cluster"
	ResourceTypeOrganizationTeam                        ResourceType = "organization_team"
	ResourceTypeOrganizationTeamRole                    ResourceType = "organization_team_role"
	ResourceTypeOrganizationUser                        ResourceType = "organization_user"
	ResourceTypeOrganizationUserTeamMembership          ResourceType = "organization_user_team_membership"
	ResourceTypeOrganizationUserRole                    ResourceType = "organization_user_role"
	ResourceTypeOrganizationSystemAccount               ResourceType = "organization_system_account"
	ResourceTypeOrganizationSystemAccountTeamMembership ResourceType = "organization_system_account_team_membership"
	ResourceTypeOrganizationSystemAccountRole           ResourceType = "organization_system_account_role"
	ResourceTypeTeam                                    ResourceType = "team"
	ResourceTypeEventGatewayListener                    ResourceType = "event_gateway_listener"
	ResourceTypeEventGatewayListenerPolicy              ResourceType = "event_gateway_listener_policy"
	ResourceTypeEventGatewayDataPlaneCertificate        ResourceType = "event_gateway_data_plane_certificate"
	ResourceTypeEventGatewayClusterPolicy               ResourceType = "event_gateway_virtual_cluster_cluster_policy"
	ResourceTypeEventGatewayProducePolicy               ResourceType = "event_gateway_virtual_cluster_produce_policy"
	ResourceTypeEventGatewayConsumePolicy               ResourceType = "event_gateway_virtual_cluster_consume_policy"
	ResourceTypeEventGatewaySchemaRegistry              ResourceType = "event_gateway_schema_registry"
	ResourceTypeEventGatewayStaticKey                   ResourceType = "event_gateway_static_key"
	ResourceTypeEventGatewayTLSTrustBundle              ResourceType = "event_gateway_tls_trust_bundle"
)

const (
	// NamespaceExternal is a sentinel (empty string) used internally when handling external resources.
	NamespaceExternal = ""
)

const (
	SchemaFieldRef       = "ref"
	SchemaFieldPortal    = "portal"
	SchemaFieldTeams     = "teams"
	SchemaFieldAIGateway = "ai_gateway"
	jsonNullLiteral      = "null"
)

// ResourceRef represents a reference to another resource
type ResourceRef struct {
	Kind ResourceType `json:"kind" yaml:"kind"`
	Ref  string       `json:"ref"  yaml:"ref"`
}

// NormalizeResourceRef returns the declarative ref for either a plain ref or a
// deferred !ref placeholder.
func NormalizeResourceRef(value string) string {
	ref, _, ok := tags.ParseRefPlaceholder(value)
	if ok {
		return ref
	}
	return value
}

// ResourceSet contains all declarative resources from configuration files
type ResourceSet struct {
	Portals []PortalResource `yaml:"portals,omitempty"                                        json:"portals,omitempty"`
	// AuditLogs contains organization-scoped audit-log resources.
	AuditLogs *AuditLogsResource `yaml:"audit-logs,omitempty"                                     json:"audit-logs,omitempty"` //nolint:lll
	// ApplicationAuthStrategies contains auth strategy configurations
	ApplicationAuthStrategies []ApplicationAuthStrategyResource `yaml:"application_auth_strategies,omitempty"                    json:"application_auth_strategies,omitempty"` //nolint:lll
	DCRProviders              []DCRProviderResource             `yaml:"dcr_providers,omitempty"                                  json:"dcr_providers,omitempty"`               //nolint:lll
	// ControlPlanes contains control plane configurations
	ControlPlanes                     []ControlPlaneResource                     `yaml:"control_planes,omitempty"                                 json:"control_planes,omitempty"`                        //nolint:lll
	CatalogServices                   []CatalogServiceResource                   `yaml:"catalog_services,omitempty"                               json:"catalog_services,omitempty"`                      //nolint:lll
	AIGateways                        []AIGatewayResource                        `yaml:"ai_gateways,omitempty"                                    json:"ai_gateways,omitempty"`                           //nolint:lll
	AIGatewayProviders                []AIGatewayProviderResource                `yaml:"ai_gateway_providers,omitempty"                          json:"ai_gateway_providers,omitempty"`                   //nolint:lll
	AIGatewayPolicies                 []AIGatewayPolicyResource                  `yaml:"ai_gateway_policies,omitempty"                           json:"ai_gateway_policies,omitempty"`                    //nolint:lll
	AIGatewayAgents                   []AIGatewayAgentResource                   `yaml:"ai_gateway_agents,omitempty"                             json:"ai_gateway_agents,omitempty"`                      //nolint:lll
	AIGatewayConsumers                []AIGatewayConsumerResource                `yaml:"ai_gateway_consumers,omitempty"                          json:"ai_gateway_consumers,omitempty"`                   //nolint:lll
	AIGatewayConsumerGroups           []AIGatewayConsumerGroupResource           `yaml:"ai_gateway_consumer_groups,omitempty"                    json:"ai_gateway_consumer_groups,omitempty"`             //nolint:lll
	AIGatewayModels                   []AIGatewayModelResource                   `yaml:"ai_gateway_models,omitempty"                              json:"ai_gateway_models,omitempty"`                     //nolint:lll
	AIGatewayMCPServers               []AIGatewayMCPServerResource               `yaml:"ai_gateway_mcp_servers,omitempty"                         json:"ai_gateway_mcp_servers,omitempty"`                //nolint:lll
	AIGatewayVaults                   []AIGatewayVaultResource                   `yaml:"ai_gateway_vaults,omitempty"                              json:"ai_gateway_vaults,omitempty"`                     //nolint:lll
	AIGatewayNodes                    []AIGatewayNodeResource                    `yaml:"ai_gateway_nodes,omitempty"                               json:"ai_gateway_nodes,omitempty"`                      //nolint:lll
	APIs                              []APIResource                              `yaml:"apis,omitempty"                                           json:"apis,omitempty"`                                  //nolint:lll
	GatewayServices                   []GatewayServiceResource                   `yaml:"gateway_services,omitempty"                               json:"gateway_services,omitempty"`                      //nolint:lll
	ControlPlaneDataPlaneCertificates []ControlPlaneDataPlaneCertificateResource `yaml:"control_plane_data_plane_certificates,omitempty"          json:"control_plane_data_plane_certificates,omitempty"` //nolint:lll
	// API child resources can be defined at root level (with parent reference) or nested under APIs
	APIVersions        []APIVersionResource        `yaml:"api_versions,omitempty"                                   json:"api_versions,omitempty"`        //nolint:lll
	APIPublications    []APIPublicationResource    `yaml:"api_publications,omitempty"                               json:"api_publications,omitempty"`    //nolint:lll
	APIImplementations []APIImplementationResource `yaml:"api_implementations,omitempty"                            json:"api_implementations,omitempty"` //nolint:lll
	APIDocuments       []APIDocumentResource       `yaml:"api_documents,omitempty"                                  json:"api_documents,omitempty"`       //nolint:lll
	// Portal child resources can be defined at root level (with parent reference) or nested under Portals
	PortalCustomizations        []PortalCustomizationResource        `yaml:"portal_customizations,omitempty"                          json:"portal_customizations,omitempty"`          //nolint:lll
	PortalAuthSettings          []PortalAuthSettingsResource         `yaml:"portal_auth_settings,omitempty"                           json:"portal_auth_settings,omitempty"`           //nolint:lll
	PortalIPAllowLists          []PortalIPAllowListResource          `yaml:"portal_ip_allow_lists,omitempty"                          json:"portal_ip_allow_lists,omitempty"`          //nolint:lll
	PortalIntegrations          []PortalIntegrationResource          `yaml:"portal_integrations,omitempty"                            json:"portal_integrations,omitempty"`            //nolint:lll
	PortalIdentityProviders     []PortalIdentityProviderResource     `yaml:"portal_identity_providers,omitempty"                      json:"portal_identity_providers,omitempty"`      //nolint:lll
	PortalTeamGroupMappings     []PortalTeamGroupMappingResource     `yaml:"portal_team_group_mappings,omitempty"                     json:"portal_team_group_mappings,omitempty"`     //nolint:lll
	PortalCustomDomains         []PortalCustomDomainResource         `yaml:"portal_custom_domains,omitempty"                          json:"portal_custom_domains,omitempty"`          //nolint:lll
	PortalPages                 []PortalPageResource                 `yaml:"portal_pages,omitempty"                                   json:"portal_pages,omitempty"`                   //nolint:lll
	PortalSnippets              []PortalSnippetResource              `yaml:"portal_snippets,omitempty"                                json:"portal_snippets,omitempty"`                //nolint:lll
	PortalTeams                 []PortalTeamResource                 `yaml:"portal_teams,omitempty"                                   json:"portal_teams,omitempty"`                   //nolint:lll
	PortalTeamRoles             []PortalTeamRoleResource             `yaml:"portal_team_roles,omitempty"                              json:"portal_team_roles,omitempty"`              //nolint:lll
	PortalAssetLogos            []PortalAssetLogoResource            `yaml:"portal_asset_logos,omitempty"                             json:"portal_asset_logos,omitempty"`             //nolint:lll
	PortalAssetFavicons         []PortalAssetFaviconResource         `yaml:"portal_asset_favicons,omitempty"                          json:"portal_asset_favicons,omitempty"`          //nolint:lll
	PortalEmailConfigs          []PortalEmailConfigResource          `yaml:"portal_email_configs,omitempty"                           json:"portal_email_configs,omitempty"`           //nolint:lll
	PortalEmailTemplates        []PortalEmailTemplateResource        `yaml:"portal_email_templates,omitempty"                         json:"portal_email_templates,omitempty"`         //nolint:lll
	PortalAuditLogWebhooks      []PortalAuditLogWebhookResource      `yaml:"portal_audit_log_webhooks,omitempty"                      json:"portal_audit_log_webhooks,omitempty"`      //nolint:lll
	EventGatewayControlPlanes   []EventGatewayControlPlaneResource   `yaml:"event_gateways,omitempty"                                 json:"event_gateways,omitempty"`                 //nolint:lll
	EventGatewayBackendClusters []EventGatewayBackendClusterResource `yaml:"event_gateway_backend_clusters,omitempty"                 json:"event_gateway_backend_clusters,omitempty"` //nolint:lll
	EventGatewayVirtualClusters []EventGatewayVirtualClusterResource `yaml:"event_gateway_virtual_clusters,omitempty"                 json:"event_gateway_virtual_clusters,omitempty"` //nolint:lll
	// Organization grouping - contains nested resources like teams
	Organization *OrganizationResource `yaml:"organization,omitempty"                                   json:"organization,omitempty"` //nolint:lll
	// Analytics grouping - contains nested resources like dashboards
	Analytics *AnalyticsResource `yaml:"analytics,omitempty" json:"analytics,omitempty"`
	// Teams is populated internally from OrganizationTeams during loading
	// It is not exposed in YAML/JSON to enforce the organization grouping format
	OrganizationTeams                        []OrganizationTeamResource                        `yaml:"-" json:"-"`
	OrganizationTeamRoles                    []OrganizationTeamRoleResource                    `yaml:"organization_team_roles,omitempty"                        json:"organization_team_roles,omitempty"` //nolint:lll
	OrganizationUserTeamMemberships          []OrganizationUserTeamMembershipResource          `yaml:"-" json:"-"`
	OrganizationUserRoles                    []OrganizationUserRoleResource                    `yaml:"-" json:"-"`
	OrganizationSystemAccountTeamMemberships []OrganizationSystemAccountTeamMembershipResource `yaml:"-" json:"-"`
	OrganizationSystemAccountRoles           []OrganizationSystemAccountRoleResource           `yaml:"-" json:"-"`
	EventGatewayListeners                    []EventGatewayListenerResource                    `yaml:"event_gateway_listeners,omitempty"                        json:"event_gateway_listeners,omitempty"`                        //nolint:lll
	EventGatewayListenerPolicies             []EventGatewayListenerPolicyResource              `yaml:"event_gateway_listener_policies,omitempty"                json:"event_gateway_listener_policies,omitempty"`                //nolint:lll
	EventGatewayClusterPolicies              []EventGatewayClusterPolicyResource               `yaml:"event_gateway_virtual_cluster_cluster_policies,omitempty" json:"event_gateway_virtual_cluster_cluster_policies,omitempty"` //nolint:lll
	EventGatewayProducePolicies              []EventGatewayProducePolicyResource               `yaml:"event_gateway_virtual_cluster_produce_policies,omitempty" json:"event_gateway_virtual_cluster_produce_policies,omitempty"` //nolint:lll
	EventGatewayConsumePolicies              []EventGatewayConsumePolicyResource               `yaml:"event_gateway_virtual_cluster_consume_policies,omitempty" json:"event_gateway_virtual_cluster_consume_policies,omitempty"` //nolint:lll
	EventGatewayDataPlaneCertificates        []EventGatewayDataPlaneCertificateResource        `yaml:"event_gateway_data_plane_certificates,omitempty"          json:"event_gateway_data_plane_certificates,omitempty"`          //nolint:lll
	EventGatewaySchemaRegistries             []EventGatewaySchemaRegistryResource              `yaml:"event_gateway_schema_registries,omitempty"                json:"event_gateway_schema_registries,omitempty"`                //nolint:lll
	EventGatewayStaticKeys                   []EventGatewayStaticKeyResource                   `yaml:"event_gateway_static_keys,omitempty"                      json:"event_gateway_static_keys,omitempty"`                      //nolint:lll
	EventGatewayTLSTrustBundles              []EventGatewayTLSTrustBundleResource              `yaml:"event_gateway_tls_trust_bundles,omitempty"                json:"event_gateway_tls_trust_bundles,omitempty"`                //nolint:lll
	// Dashboards is populated internally from Analytics.Dashboards during loading.
	// It is not exposed in YAML/JSON to enforce the analytics grouping format.
	Dashboards []DashboardResource `yaml:"-"                                                        json:"-"`
	// DefaultNamespace tracks namespace from _defaults when no resources are present
	// This is used by the planner to determine which namespace to check for deletions
	DefaultNamespace  string   `yaml:"-"                                                        json:"-"`
	DefaultNamespaces []string `yaml:"-"                                                        json:"-"`
	// EnvSources tracks deferred !env placeholders by resource ref and field path.
	EnvSources map[string]map[string]string `yaml:"-"                                                        json:"-"`
	// SyncScope tracks resource collection keys explicitly present in the input.
	SyncScope *SyncScope `yaml:"-"                                                        json:"-"`
}

// NamespaceOrigin describes how a namespace value was supplied for a resource
type NamespaceOrigin int

const (
	// NamespaceOriginUnset indicates no namespace was resolved for the resource
	NamespaceOriginUnset NamespaceOrigin = iota
	// NamespaceOriginExplicit indicates the namespace was explicitly set on the resource
	NamespaceOriginExplicit
	// NamespaceOriginFileDefault indicates the namespace was inherited from _defaults.kongctl.namespace
	NamespaceOriginFileDefault
	// NamespaceOriginImplicitDefault indicates the namespace fell back to the implicit "default" value
	NamespaceOriginImplicitDefault
)

// KongctlMeta contains tool-specific metadata for resources
type KongctlMeta struct {
	// Protected prevents accidental deletion of critical resources
	Protected *bool `yaml:"protected,omitempty" json:"protected,omitempty"`
	// Namespace for resource isolation and multi-team management
	Namespace *string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	// NamespaceOrigin tracks how the namespace value was derived (not serialized)
	NamespaceOrigin NamespaceOrigin `yaml:"-"                   json:"-"`
}

// FileDefaults holds file-level defaults that apply to all resources in the file
type FileDefaults struct {
	Kongctl *KongctlMetaDefaults `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
}

// KongctlMetaDefaults holds default values for kongctl metadata fields
type KongctlMetaDefaults struct {
	Namespace *string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Protected *bool   `yaml:"protected,omitempty" json:"protected,omitempty"`
}

// HasRef checks if a ref exists globally across all resource types
func (rs *ResourceSet) HasRef(ref string) bool {
	_, found := rs.GetResourceByRef(ref)
	return found
}

// GetResourceByRef returns the resource for a given ref
func (rs *ResourceSet) GetResourceByRef(ref string) (Resource, bool) {
	var found Resource
	rs.ForEachResource(func(r Resource) bool {
		if r.GetRef() == ref {
			found = r
			return false // stop iteration
		}
		return true
	})
	return found, found != nil
}

// AddEnvSource records a deferred !env placeholder for a resource field path.
func (rs *ResourceSet) AddEnvSource(resourceRef, fieldPath, placeholder string) {
	if resourceRef == "" || fieldPath == "" || placeholder == "" {
		return
	}
	if rs.EnvSources == nil {
		rs.EnvSources = make(map[string]map[string]string)
	}
	if rs.EnvSources[resourceRef] == nil {
		rs.EnvSources[resourceRef] = make(map[string]string)
	}
	rs.EnvSources[resourceRef][fieldPath] = placeholder
}

// GetEnvSources returns deferred !env placeholders for a resource ref.
func (rs *ResourceSet) GetEnvSources(resourceRef string) map[string]string {
	if rs == nil || rs.EnvSources == nil {
		return nil
	}
	return rs.EnvSources[resourceRef]
}

// MergeEnvSources copies deferred !env placeholders from another ResourceSet.
func (rs *ResourceSet) MergeEnvSources(other *ResourceSet) {
	if rs == nil || other == nil || len(other.EnvSources) == 0 {
		return
	}
	for resourceRef, paths := range other.EnvSources {
		for path, placeholder := range paths {
			rs.AddEnvSource(resourceRef, path, placeholder)
		}
	}
}

// HasEnvSources returns true when any deferred !env placeholders were recorded.
func (rs *ResourceSet) HasEnvSources() bool {
	return rs != nil && len(rs.EnvSources) > 0
}

// GetResourceTypeByRef returns the resource type for a given ref
func (rs *ResourceSet) GetResourceTypeByRef(ref string) (ResourceType, bool) {
	res, ok := rs.GetResourceByRef(ref)
	if !ok || res == nil {
		return "", false
	}
	return res.GetType(), true
}

// Global lookup methods - search across all namespaces

// GetPortalByRef returns a portal resource by its ref from any namespace
func (rs *ResourceSet) GetPortalByRef(ref string) *PortalResource {
	for i := range rs.Portals {
		if rs.Portals[i].GetRef() == ref {
			return &rs.Portals[i]
		}
	}
	return nil
}

// GetAPIByRef returns an API resource by its ref from any namespace
func (rs *ResourceSet) GetAPIByRef(ref string) *APIResource {
	for i := range rs.APIs {
		if rs.APIs[i].GetRef() == ref {
			return &rs.APIs[i]
		}
	}
	return nil
}

// GetDashboardByRef returns a dashboard resource by its ref from any namespace.
func (rs *ResourceSet) GetDashboardByRef(ref string) *DashboardResource {
	for i := range rs.Dashboards {
		if rs.Dashboards[i].GetRef() == ref {
			return &rs.Dashboards[i]
		}
	}
	return nil
}

// GetControlPlaneByRef returns a control plane resource by its ref from any namespace
func (rs *ResourceSet) GetControlPlaneByRef(ref string) *ControlPlaneResource {
	for i := range rs.ControlPlanes {
		if rs.ControlPlanes[i].GetRef() == ref {
			return &rs.ControlPlanes[i]
		}
	}
	return nil
}

// GetCatalogServiceByRef returns a catalog service resource by its ref from any namespace
func (rs *ResourceSet) GetCatalogServiceByRef(ref string) *CatalogServiceResource {
	for i := range rs.CatalogServices {
		if rs.CatalogServices[i].GetRef() == ref {
			return &rs.CatalogServices[i]
		}
	}
	return nil
}

// GetAIGatewayByRef returns an AI Gateway resource by its ref from any namespace.
func (rs *ResourceSet) GetAIGatewayByRef(ref string) *AIGatewayResource {
	ref = NormalizeResourceRef(ref)
	for i := range rs.AIGateways {
		if rs.AIGateways[i].GetRef() == ref {
			return &rs.AIGateways[i]
		}
	}
	return nil
}

// GetAIGatewayProviderByRef returns an AI Gateway Provider resource by its ref from any namespace.
func (rs *ResourceSet) GetAIGatewayProviderByRef(ref string) *AIGatewayProviderResource {
	for i := range rs.AIGatewayProviders {
		if rs.AIGatewayProviders[i].GetRef() == ref {
			return &rs.AIGatewayProviders[i]
		}
	}
	return nil
}

// GetAIGatewayPolicyByRef returns an AI Gateway Policy resource by its ref from any namespace.
func (rs *ResourceSet) GetAIGatewayPolicyByRef(ref string) *AIGatewayPolicyResource {
	for i := range rs.AIGatewayPolicies {
		if rs.AIGatewayPolicies[i].GetRef() == ref {
			return &rs.AIGatewayPolicies[i]
		}
	}
	return nil
}

// GetAIGatewayAgentByRef returns an AI Gateway Agent resource by its ref from any namespace.
func (rs *ResourceSet) GetAIGatewayAgentByRef(ref string) *AIGatewayAgentResource {
	for i := range rs.AIGatewayAgents {
		if rs.AIGatewayAgents[i].GetRef() == ref {
			return &rs.AIGatewayAgents[i]
		}
	}
	return nil
}

// GetAIGatewayConsumerByRef returns an AI Gateway Consumer resource by its ref from any namespace.
func (rs *ResourceSet) GetAIGatewayConsumerByRef(ref string) *AIGatewayConsumerResource {
	for i := range rs.AIGatewayConsumers {
		if rs.AIGatewayConsumers[i].GetRef() == ref {
			return &rs.AIGatewayConsumers[i]
		}
	}
	return nil
}

// GetAIGatewayConsumerGroupByRef returns an AI Gateway Consumer Group resource by its ref from any namespace.
func (rs *ResourceSet) GetAIGatewayConsumerGroupByRef(ref string) *AIGatewayConsumerGroupResource {
	for i := range rs.AIGatewayConsumerGroups {
		if rs.AIGatewayConsumerGroups[i].GetRef() == ref {
			return &rs.AIGatewayConsumerGroups[i]
		}
	}
	return nil
}

// GetAIGatewayModelByRef returns an AI Gateway Model resource by its ref from any namespace.
func (rs *ResourceSet) GetAIGatewayModelByRef(ref string) *AIGatewayModelResource {
	for i := range rs.AIGatewayModels {
		if rs.AIGatewayModels[i].GetRef() == ref {
			return &rs.AIGatewayModels[i]
		}
	}
	return nil
}

// GetAIGatewayMCPServerByRef returns an AI Gateway MCP Server resource by its ref from any namespace.
func (rs *ResourceSet) GetAIGatewayMCPServerByRef(ref string) *AIGatewayMCPServerResource {
	for i := range rs.AIGatewayMCPServers {
		if rs.AIGatewayMCPServers[i].GetRef() == ref {
			return &rs.AIGatewayMCPServers[i]
		}
	}
	return nil
}

// GetAIGatewayVaultByRef returns an AI Gateway Vault resource by its ref from any namespace.
func (rs *ResourceSet) GetAIGatewayVaultByRef(ref string) *AIGatewayVaultResource {
	for i := range rs.AIGatewayVaults {
		if rs.AIGatewayVaults[i].GetRef() == ref {
			return &rs.AIGatewayVaults[i]
		}
	}
	return nil
}

// GetAIGatewayNodeByRef returns an AI Gateway Node resource by its ref from any namespace.
func (rs *ResourceSet) GetAIGatewayNodeByRef(ref string) *AIGatewayNodeResource {
	for i := range rs.AIGatewayNodes {
		if rs.AIGatewayNodes[i].GetRef() == ref {
			return &rs.AIGatewayNodes[i]
		}
	}
	return nil
}

// GetAuthStrategyByRef returns an auth strategy resource by its ref from any namespace
func (rs *ResourceSet) GetAuthStrategyByRef(ref string) *ApplicationAuthStrategyResource {
	for i := range rs.ApplicationAuthStrategies {
		if rs.ApplicationAuthStrategies[i].GetRef() == ref {
			return &rs.ApplicationAuthStrategies[i]
		}
	}
	return nil
}

// GetDCRProviderByRef returns a DCR provider resource by its ref from any namespace
func (rs *ResourceSet) GetDCRProviderByRef(ref string) *DCRProviderResource {
	for i := range rs.DCRProviders {
		if rs.DCRProviders[i].GetRef() == ref {
			return &rs.DCRProviders[i]
		}
	}
	return nil
}

// Namespace-filtered access methods

// GetPortalsByNamespace returns all portal resources from the specified namespace
func (rs *ResourceSet) GetPortalsByNamespace(namespace string) []PortalResource {
	var filtered []PortalResource
	for _, portal := range rs.Portals {
		if portal.IsExternal() {
			if namespace == NamespaceExternal {
				filtered = append(filtered, portal)
			}
			continue
		}
		if GetNamespace(portal.Kongctl) == namespace {
			filtered = append(filtered, portal)
		}
	}
	return filtered
}

// GetControlPlanesByNamespace returns all control plane resources from the specified namespace
func (rs *ResourceSet) GetControlPlanesByNamespace(namespace string) []ControlPlaneResource {
	var filtered []ControlPlaneResource
	for _, cp := range rs.ControlPlanes {
		if GetNamespace(cp.Kongctl) == namespace {
			filtered = append(filtered, cp)
		}
	}
	return filtered
}

// GetCatalogServicesByNamespace returns all catalog service resources from the specified namespace
func (rs *ResourceSet) GetCatalogServicesByNamespace(namespace string) []CatalogServiceResource {
	var filtered []CatalogServiceResource
	for _, svc := range rs.CatalogServices {
		if GetNamespace(svc.Kongctl) == namespace {
			filtered = append(filtered, svc)
		}
	}
	return filtered
}

// GetAIGatewaysByNamespace returns all AI Gateway resources from the specified namespace.
func (rs *ResourceSet) GetAIGatewaysByNamespace(namespace string) []AIGatewayResource {
	var filtered []AIGatewayResource
	for _, gateway := range rs.AIGateways {
		if gateway.IsExternal() {
			if namespace == NamespaceExternal {
				filtered = append(filtered, gateway)
			}
			continue
		}
		if GetNamespace(gateway.Kongctl) == namespace {
			filtered = append(filtered, gateway)
		}
	}
	return filtered
}

// GetAIGatewayProvidersByNamespace returns all AI Gateway Provider resources from the specified namespace.
func (rs *ResourceSet) GetAIGatewayProvidersByNamespace(namespace string) []AIGatewayProviderResource {
	gatewayByRef := make(map[string]AIGatewayResource)
	for _, gateway := range rs.GetAIGatewaysByNamespace(namespace) {
		gatewayByRef[gateway.Ref] = gateway
	}

	var filtered []AIGatewayProviderResource
	for _, provider := range rs.AIGatewayProviders {
		if _, ok := gatewayByRef[NormalizeResourceRef(provider.AIGateway)]; ok {
			filtered = append(filtered, provider)
		}
	}
	return filtered
}

// GetAIGatewayPoliciesByNamespace returns AI Gateway policy resources from the specified namespace.
func (rs *ResourceSet) GetAIGatewayPoliciesByNamespace(namespace string) []AIGatewayPolicyResource {
	var filtered []AIGatewayPolicyResource
	for _, policy := range rs.AIGatewayPolicies {
		if gateway := rs.GetAIGatewayByRef(policy.AIGateway); gateway != nil {
			if gateway.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, policy)
				}
				continue
			}
			if GetNamespace(gateway.Kongctl) == namespace {
				filtered = append(filtered, policy)
			}
		}
	}
	return filtered
}

// GetAIGatewayAgentsByNamespace returns AI Gateway Agent resources from the specified namespace.
func (rs *ResourceSet) GetAIGatewayAgentsByNamespace(namespace string) []AIGatewayAgentResource {
	var filtered []AIGatewayAgentResource
	for _, agent := range rs.AIGatewayAgents {
		if gateway := rs.GetAIGatewayByRef(agent.AIGateway); gateway != nil {
			if gateway.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, agent)
				}
				continue
			}
			if GetNamespace(gateway.Kongctl) == namespace {
				filtered = append(filtered, agent)
			}
		}
	}
	return filtered
}

// GetAIGatewayConsumersByNamespace returns AI Gateway Consumer resources from the specified namespace.
func (rs *ResourceSet) GetAIGatewayConsumersByNamespace(namespace string) []AIGatewayConsumerResource {
	var filtered []AIGatewayConsumerResource
	for _, consumer := range rs.AIGatewayConsumers {
		if gateway := rs.GetAIGatewayByRef(consumer.AIGateway); gateway != nil {
			if gateway.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, consumer)
				}
				continue
			}
			if GetNamespace(gateway.Kongctl) == namespace {
				filtered = append(filtered, consumer)
			}
		}
	}
	return filtered
}

// GetAIGatewayConsumerGroupsByNamespace returns AI Gateway Consumer Group resources from the specified namespace.
func (rs *ResourceSet) GetAIGatewayConsumerGroupsByNamespace(namespace string) []AIGatewayConsumerGroupResource {
	var filtered []AIGatewayConsumerGroupResource
	for _, group := range rs.AIGatewayConsumerGroups {
		if gateway := rs.GetAIGatewayByRef(group.AIGateway); gateway != nil {
			if gateway.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, group)
				}
				continue
			}
			if GetNamespace(gateway.Kongctl) == namespace {
				filtered = append(filtered, group)
			}
		}
	}
	return filtered
}

// GetAIGatewayModelsByNamespace returns AI Gateway model resources from the specified namespace.
func (rs *ResourceSet) GetAIGatewayModelsByNamespace(namespace string) []AIGatewayModelResource {
	var filtered []AIGatewayModelResource
	for _, model := range rs.AIGatewayModels {
		if gateway := rs.GetAIGatewayByRef(model.AIGateway); gateway != nil {
			if gateway.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, model)
				}
				continue
			}
			if GetNamespace(gateway.Kongctl) == namespace {
				filtered = append(filtered, model)
			}
		}
	}
	return filtered
}

// GetAIGatewayMCPServersByNamespace returns AI Gateway MCP Server resources from the specified namespace.
func (rs *ResourceSet) GetAIGatewayMCPServersByNamespace(namespace string) []AIGatewayMCPServerResource {
	var filtered []AIGatewayMCPServerResource
	for _, server := range rs.AIGatewayMCPServers {
		if gateway := rs.GetAIGatewayByRef(server.AIGateway); gateway != nil {
			if gateway.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, server)
				}
				continue
			}
			if GetNamespace(gateway.Kongctl) == namespace {
				filtered = append(filtered, server)
			}
		}
	}
	return filtered
}

// GetAIGatewayVaultsByNamespace returns AI Gateway Vault resources from the specified namespace.
func (rs *ResourceSet) GetAIGatewayVaultsByNamespace(namespace string) []AIGatewayVaultResource {
	var filtered []AIGatewayVaultResource
	for _, vault := range rs.AIGatewayVaults {
		if gateway := rs.GetAIGatewayByRef(vault.AIGateway); gateway != nil {
			if gateway.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, vault)
				}
				continue
			}
			if GetNamespace(gateway.Kongctl) == namespace {
				filtered = append(filtered, vault)
			}
		}
	}
	return filtered
}

// GetAIGatewayNodesByNamespace returns AI Gateway Node resources from the specified namespace.
func (rs *ResourceSet) GetAIGatewayNodesByNamespace(namespace string) []AIGatewayNodeResource {
	var filtered []AIGatewayNodeResource
	for _, node := range rs.AIGatewayNodes {
		if gateway := rs.GetAIGatewayByRef(node.AIGateway); gateway != nil {
			if gateway.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, node)
				}
				continue
			}
			if GetNamespace(gateway.Kongctl) == namespace {
				filtered = append(filtered, node)
			}
		}
	}
	return filtered
}

// GetAPIsByNamespace returns all API resources from the specified namespace
func (rs *ResourceSet) GetAPIsByNamespace(namespace string) []APIResource {
	var filtered []APIResource
	for _, api := range rs.APIs {
		if GetNamespace(api.Kongctl) == namespace {
			filtered = append(filtered, api)
		}
	}
	return filtered
}

// GetDashboardsByNamespace returns all dashboard resources from the specified namespace.
func (rs *ResourceSet) GetDashboardsByNamespace(namespace string) []DashboardResource {
	var filtered []DashboardResource
	for _, dashboard := range rs.Dashboards {
		if GetNamespace(dashboard.Kongctl) == namespace {
			filtered = append(filtered, dashboard)
		}
	}
	return filtered
}

// GetAuthStrategiesByNamespace returns all auth strategy resources from the specified namespace
func (rs *ResourceSet) GetAuthStrategiesByNamespace(namespace string) []ApplicationAuthStrategyResource {
	var filtered []ApplicationAuthStrategyResource
	for _, strategy := range rs.ApplicationAuthStrategies {
		if GetNamespace(strategy.Kongctl) == namespace {
			filtered = append(filtered, strategy)
		}
	}
	return filtered
}

// GetDCRProvidersByNamespace returns all DCR provider resources from the specified namespace
func (rs *ResourceSet) GetDCRProvidersByNamespace(namespace string) []DCRProviderResource {
	var filtered []DCRProviderResource
	for _, provider := range rs.DCRProviders {
		if GetNamespace(provider.Kongctl) == namespace {
			filtered = append(filtered, provider)
		}
	}
	return filtered
}

// GetAPIVersionsByNamespace returns all API version resources from the specified namespace
func (rs *ResourceSet) GetAPIVersionsByNamespace(namespace string) []APIVersionResource {
	var filtered []APIVersionResource
	for _, version := range rs.APIVersions {
		// Check if parent API is in the namespace
		if api := rs.GetAPIByRef(version.API); api != nil && GetNamespace(api.Kongctl) == namespace {
			filtered = append(filtered, version)
		}
	}
	return filtered
}

// GetAPIPublicationsByNamespace returns all API publication resources from the specified namespace
func (rs *ResourceSet) GetAPIPublicationsByNamespace(namespace string) []APIPublicationResource {
	var filtered []APIPublicationResource
	for _, pub := range rs.APIPublications {
		// Check if parent API is in the namespace
		if api := rs.GetAPIByRef(pub.API); api != nil && GetNamespace(api.Kongctl) == namespace {
			filtered = append(filtered, pub)
		}
	}
	return filtered
}

// GetAPIImplementationsByNamespace returns all API implementation resources from the specified namespace
func (rs *ResourceSet) GetAPIImplementationsByNamespace(namespace string) []APIImplementationResource {
	var filtered []APIImplementationResource
	for _, impl := range rs.APIImplementations {
		// Check if parent API is in the namespace
		if api := rs.GetAPIByRef(impl.API); api != nil && GetNamespace(api.Kongctl) == namespace {
			filtered = append(filtered, impl)
		}
	}
	return filtered
}

// GetAPIDocumentsByNamespace returns all API document resources from the specified namespace
func (rs *ResourceSet) GetAPIDocumentsByNamespace(namespace string) []APIDocumentResource {
	var filtered []APIDocumentResource
	for _, doc := range rs.APIDocuments {
		// Check if parent API is in the namespace
		if api := rs.GetAPIByRef(doc.API); api != nil && GetNamespace(api.Kongctl) == namespace {
			filtered = append(filtered, doc)
		}
	}
	return filtered
}

// GetPortalCustomizationsByNamespace returns all portal customization resources from the specified namespace
func (rs *ResourceSet) GetPortalCustomizationsByNamespace(namespace string) []PortalCustomizationResource {
	var filtered []PortalCustomizationResource
	for _, custom := range rs.PortalCustomizations {
		// Check if parent portal is in the namespace
		if portal := rs.GetPortalByRef(custom.Portal); portal != nil {
			if portal.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, custom)
				}
				continue
			}
			if GetNamespace(portal.Kongctl) == namespace {
				filtered = append(filtered, custom)
			}
		}
	}
	return filtered
}

// GetPortalAuthSettingsByNamespace returns all portal auth settings resources from the specified namespace
func (rs *ResourceSet) GetPortalAuthSettingsByNamespace(namespace string) []PortalAuthSettingsResource {
	var filtered []PortalAuthSettingsResource
	for _, settings := range rs.PortalAuthSettings {
		if portal := rs.GetPortalByRef(settings.Portal); portal != nil {
			if portal.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, settings)
				}
				continue
			}
			if GetNamespace(portal.Kongctl) == namespace {
				filtered = append(filtered, settings)
			}
		}
	}
	return filtered
}

// GetPortalIPAllowListsByNamespace returns all portal IP allow-list resources from the specified namespace.
func (rs *ResourceSet) GetPortalIPAllowListsByNamespace(namespace string) []PortalIPAllowListResource {
	var filtered []PortalIPAllowListResource
	for _, allowList := range rs.PortalIPAllowLists {
		if portal := rs.GetPortalByRef(allowList.Portal); portal != nil {
			if portal.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, allowList)
				}
				continue
			}
			if GetNamespace(portal.Kongctl) == namespace {
				filtered = append(filtered, allowList)
			}
		}
	}
	return filtered
}

// GetPortalIntegrationsByNamespace returns all portal integration resources from the specified namespace
func (rs *ResourceSet) GetPortalIntegrationsByNamespace(namespace string) []PortalIntegrationResource {
	var filtered []PortalIntegrationResource
	for _, integration := range rs.PortalIntegrations {
		if portal := rs.GetPortalByRef(integration.Portal); portal != nil {
			if portal.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, integration)
				}
				continue
			}
			if GetNamespace(portal.Kongctl) == namespace {
				filtered = append(filtered, integration)
			}
		}
	}
	return filtered
}

// GetPortalIdentityProvidersByNamespace returns all portal identity provider resources from the specified namespace
func (rs *ResourceSet) GetPortalIdentityProvidersByNamespace(namespace string) []PortalIdentityProviderResource {
	var filtered []PortalIdentityProviderResource
	for _, provider := range rs.PortalIdentityProviders {
		if portal := rs.GetPortalByRef(provider.Portal); portal != nil {
			if portal.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, provider)
				}
				continue
			}
			if GetNamespace(portal.Kongctl) == namespace {
				filtered = append(filtered, provider)
			}
		}
	}
	return filtered
}

// GetPortalTeamGroupMappingsByNamespace returns portal team group mappings from the specified namespace.
func (rs *ResourceSet) GetPortalTeamGroupMappingsByNamespace(namespace string) []PortalTeamGroupMappingResource {
	var filtered []PortalTeamGroupMappingResource
	for _, mapping := range rs.PortalTeamGroupMappings {
		if portal := rs.GetPortalByRef(mapping.Portal); portal != nil {
			if portal.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, mapping)
				}
				continue
			}
			if GetNamespace(portal.Kongctl) == namespace {
				filtered = append(filtered, mapping)
			}
		}
	}
	return filtered
}

// GetPortalCustomDomainsByNamespace returns all portal custom domain resources from the specified namespace
func (rs *ResourceSet) GetPortalCustomDomainsByNamespace(namespace string) []PortalCustomDomainResource {
	var filtered []PortalCustomDomainResource
	for _, domain := range rs.PortalCustomDomains {
		// Check if parent portal is in the namespace
		if portal := rs.GetPortalByRef(domain.Portal); portal != nil {
			if portal.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, domain)
				}
				continue
			}
			if GetNamespace(portal.Kongctl) == namespace {
				filtered = append(filtered, domain)
			}
		}
	}
	return filtered
}

// GetPortalPagesByNamespace returns all portal page resources from the specified namespace
func (rs *ResourceSet) GetPortalPagesByNamespace(namespace string) []PortalPageResource {
	var filtered []PortalPageResource
	for _, page := range rs.PortalPages {
		// Check if parent portal is in the namespace
		if portal := rs.GetPortalByRef(page.Portal); portal != nil {
			if portal.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, page)
				}
				continue
			}
			if GetNamespace(portal.Kongctl) == namespace {
				filtered = append(filtered, page)
			}
		}
	}
	return filtered
}

// GetPortalSnippetsByNamespace returns all portal snippet resources from the specified namespace
func (rs *ResourceSet) GetPortalSnippetsByNamespace(namespace string) []PortalSnippetResource {
	var filtered []PortalSnippetResource
	for _, snippet := range rs.PortalSnippets {
		// Check if parent portal is in the namespace
		if portal := rs.GetPortalByRef(snippet.Portal); portal != nil {
			if portal.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, snippet)
				}
				continue
			}
			if GetNamespace(portal.Kongctl) == namespace {
				filtered = append(filtered, snippet)
			}
		}
	}
	return filtered
}

// GetPortalEmailConfigsByNamespace returns all portal email config resources from the specified namespace
func (rs *ResourceSet) GetPortalEmailConfigsByNamespace(namespace string) []PortalEmailConfigResource {
	var filtered []PortalEmailConfigResource
	for _, cfg := range rs.PortalEmailConfigs {
		if portal := rs.GetPortalByRef(cfg.Portal); portal != nil {
			if portal.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, cfg)
				}
				continue
			}
			if GetNamespace(portal.Kongctl) == namespace {
				filtered = append(filtered, cfg)
			}
		}
	}
	return filtered
}

// GetPortalAuditLogWebhooksByNamespace returns all portal audit-log webhook resources from the specified namespace
func (rs *ResourceSet) GetPortalAuditLogWebhooksByNamespace(namespace string) []PortalAuditLogWebhookResource {
	var filtered []PortalAuditLogWebhookResource
	for _, webhook := range rs.PortalAuditLogWebhooks {
		if portal := rs.GetPortalByRef(webhook.Portal); portal != nil {
			if portal.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, webhook)
				}
				continue
			}
			if GetNamespace(portal.Kongctl) == namespace {
				filtered = append(filtered, webhook)
			}
		}
	}
	return filtered
}

// GetPortalEmailTemplatesByNamespace returns all portal email template resources from the specified namespace
func (rs *ResourceSet) GetPortalEmailTemplatesByNamespace(namespace string) []PortalEmailTemplateResource {
	var filtered []PortalEmailTemplateResource
	for _, tpl := range rs.PortalEmailTemplates {
		if portal := rs.GetPortalByRef(tpl.Portal); portal != nil {
			if portal.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, tpl)
				}
				continue
			}
			if GetNamespace(portal.Kongctl) == namespace {
				filtered = append(filtered, tpl)
			}
		}
	}
	return filtered
}

// GetPortalTeamsByNamespace returns all portal team resources from the specified namespace
func (rs *ResourceSet) GetPortalTeamsByNamespace(namespace string) []PortalTeamResource {
	var filtered []PortalTeamResource
	for _, team := range rs.PortalTeams {
		// Check if parent portal is in the namespace
		if portal := rs.GetPortalByRef(team.Portal); portal != nil {
			if portal.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, team)
				}
				continue
			}
			if GetNamespace(portal.Kongctl) == namespace {
				filtered = append(filtered, team)
			}
		}
	}
	return filtered
}

// GetPortalTeamRolesByNamespace returns all portal team role resources from the specified namespace
func (rs *ResourceSet) GetPortalTeamRolesByNamespace(namespace string) []PortalTeamRoleResource {
	var filtered []PortalTeamRoleResource
	for _, role := range rs.PortalTeamRoles {
		if portal := rs.GetPortalByRef(role.Portal); portal != nil {
			if portal.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, role)
				}
				continue
			}
			if GetNamespace(portal.Kongctl) == namespace {
				filtered = append(filtered, role)
			}
		}
	}
	return filtered
}

// GetEventGatewayControlPlanesByNamespace returns all EGW CP resources from the specified namespace
func (rs *ResourceSet) GetEventGatewayControlPlanesByNamespace(namespace string) []EventGatewayControlPlaneResource {
	var filtered []EventGatewayControlPlaneResource
	for _, cp := range rs.EventGatewayControlPlanes {
		if cp.IsExternal() {
			if namespace == NamespaceExternal {
				filtered = append(filtered, cp)
			}
			continue
		}
		if GetNamespace(cp.Kongctl) == namespace {
			filtered = append(filtered, cp)
		}
	}
	return filtered
}

// GetBackendClusterByRef returns a backend cluster resource by its ref from any namespace
func (rs *ResourceSet) GetBackendClusterByRef(ref string) *EventGatewayBackendClusterResource {
	for i := range rs.EventGatewayBackendClusters {
		if rs.EventGatewayBackendClusters[i].GetRef() == ref {
			return &rs.EventGatewayBackendClusters[i]
		}
	}
	return nil
}

// GetEventGatewayControlPlaneByRef returns an Event Gateway resource by its ref from any namespace.
func (rs *ResourceSet) GetEventGatewayControlPlaneByRef(ref string) *EventGatewayControlPlaneResource {
	for i := range rs.EventGatewayControlPlanes {
		if rs.EventGatewayControlPlanes[i].GetRef() == ref {
			return &rs.EventGatewayControlPlanes[i]
		}
	}
	return nil
}

// GetVirtualClusterByRef returns a virtual cluster resource by its ref from any namespace
func (rs *ResourceSet) GetVirtualClusterByRef(ref string) *EventGatewayVirtualClusterResource {
	for gatewayIdx := range rs.EventGatewayControlPlanes {
		for clusterIdx := range rs.EventGatewayControlPlanes[gatewayIdx].VirtualClusters {
			if rs.EventGatewayControlPlanes[gatewayIdx].VirtualClusters[clusterIdx].GetRef() == ref {
				return &rs.EventGatewayControlPlanes[gatewayIdx].VirtualClusters[clusterIdx]
			}
		}
	}
	for i := range rs.EventGatewayVirtualClusters {
		if rs.EventGatewayVirtualClusters[i].GetRef() == ref {
			return &rs.EventGatewayVirtualClusters[i]
		}
	}
	return nil
}

// GetOrganizationTeamsByNamespace returns all organization_team resources from the specified namespace
func (rs *ResourceSet) GetOrganizationTeamsByNamespace(namespace string) []OrganizationTeamResource {
	var filtered []OrganizationTeamResource
	for _, team := range rs.OrganizationTeams {
		if team.IsExternal() {
			if namespace == NamespaceExternal {
				filtered = append(filtered, team)
			}
			continue
		}
		if GetNamespace(team.Kongctl) == namespace {
			filtered = append(filtered, team)
		}
	}
	return filtered
}

// GetOrganizationTeamRolesByNamespace returns all organization_team_role resources from the specified namespace.
func (rs *ResourceSet) GetOrganizationTeamRolesByNamespace(namespace string) []OrganizationTeamRoleResource {
	teamByRef := make(map[string]OrganizationTeamResource)
	for _, team := range rs.OrganizationTeams {
		teamByRef[team.Ref] = team
	}

	var filtered []OrganizationTeamRoleResource
	for _, role := range rs.OrganizationTeamRoles {
		if team, ok := teamByRef[role.Team]; ok {
			if team.IsExternal() {
				if namespace == NamespaceExternal {
					filtered = append(filtered, role)
				}
				continue
			}
			if GetNamespace(team.Kongctl) == namespace {
				filtered = append(filtered, role)
			}
		}
	}
	return filtered
}

// GetOrganizationUserTeamMembershipsByNamespace returns all user team membership resources in a namespace.
func (rs *ResourceSet) GetOrganizationUserTeamMembershipsByNamespace(
	namespace string,
) []OrganizationUserTeamMembershipResource {
	teamByRef := make(map[string]OrganizationTeamResource)
	for _, team := range rs.GetOrganizationTeamsByNamespace(namespace) {
		teamByRef[team.Ref] = team
	}

	var filtered []OrganizationUserTeamMembershipResource
	for _, membership := range rs.OrganizationUserTeamMemberships {
		if _, ok := teamByRef[membership.Team]; ok {
			filtered = append(filtered, membership)
		}
	}
	return filtered
}

// GetOrganizationUserRolesByNamespace returns all user role resources in a namespace.
func (rs *ResourceSet) GetOrganizationUserRolesByNamespace(namespace string) []OrganizationUserRoleResource {
	userByRef := make(map[string]OrganizationUserResource)
	for _, user := range rs.organizationUsers() {
		userByRef[user.Ref] = user
	}

	var filtered []OrganizationUserRoleResource
	for _, role := range rs.OrganizationUserRoles {
		if user, ok := userByRef[role.User]; ok && GetNamespace(user.Kongctl) == namespace {
			filtered = append(filtered, role)
		}
	}
	return filtered
}

// GetOrganizationSystemAccountTeamMembershipsByNamespace returns all system account team memberships in a namespace.
func (rs *ResourceSet) GetOrganizationSystemAccountTeamMembershipsByNamespace(
	namespace string,
) []OrganizationSystemAccountTeamMembershipResource {
	teamByRef := make(map[string]OrganizationTeamResource)
	for _, team := range rs.GetOrganizationTeamsByNamespace(namespace) {
		teamByRef[team.Ref] = team
	}

	var filtered []OrganizationSystemAccountTeamMembershipResource
	for _, membership := range rs.OrganizationSystemAccountTeamMemberships {
		if _, ok := teamByRef[membership.Team]; ok {
			filtered = append(filtered, membership)
		}
	}
	return filtered
}

// GetOrganizationSystemAccountRolesByNamespace returns all system account role resources in a namespace.
func (rs *ResourceSet) GetOrganizationSystemAccountRolesByNamespace(
	namespace string,
) []OrganizationSystemAccountRoleResource {
	systemAccountByRef := make(map[string]OrganizationSystemAccountResource)
	for _, systemAccount := range rs.organizationSystemAccounts() {
		systemAccountByRef[systemAccount.Ref] = systemAccount
	}

	var filtered []OrganizationSystemAccountRoleResource
	for _, role := range rs.OrganizationSystemAccountRoles {
		if systemAccount, ok := systemAccountByRef[role.SystemAccount]; ok &&
			GetNamespace(systemAccount.Kongctl) == namespace {
			filtered = append(filtered, role)
		}
	}
	return filtered
}

func (rs *ResourceSet) organizationUsers() []OrganizationUserResource {
	if rs == nil || rs.Organization == nil {
		return nil
	}
	return rs.Organization.Users
}

func (rs *ResourceSet) organizationSystemAccounts() []OrganizationSystemAccountResource {
	if rs == nil || rs.Organization == nil {
		return nil
	}
	return rs.Organization.SystemAccounts
}

// GetNamespace safely extracts namespace from kongctl metadata
func GetNamespace(kongctl *KongctlMeta) string {
	if kongctl == nil || kongctl.Namespace == nil {
		return "default"
	}
	return *kongctl.Namespace
}

// AddDefaultNamespace records a default namespace if not already present. The first
// value encountered is also stored in DefaultNamespace for backward compatibility.
func (rs *ResourceSet) AddDefaultNamespace(namespace string) {
	if namespace == "" {
		return
	}
	if rs.DefaultNamespace == "" {
		rs.DefaultNamespace = namespace
	}
	if slices.Contains(rs.DefaultNamespaces, namespace) {
		return
	}
	rs.DefaultNamespaces = append(rs.DefaultNamespaces, namespace)
}

// GetBackendClustersForGateway returns all backend clusters (nested + root-level) for a given gateway ref
func (rs *ResourceSet) GetBackendClustersForGateway(gatewayRef string) []EventGatewayBackendClusterResource {
	var clusters []EventGatewayBackendClusterResource

	// Find the gateway to get nested clusters
	for _, gateway := range rs.EventGatewayControlPlanes {
		if gateway.Ref == gatewayRef {
			// Add nested backend clusters
			for _, cluster := range gateway.BackendClusters {
				clusterCopy := cluster
				clusterCopy.EventGateway = gatewayRef
				clusters = append(clusters, clusterCopy)
			}
			break
		}
	}

	// Add root-level backend clusters for this gateway
	for _, cluster := range rs.EventGatewayBackendClusters {
		if cluster.EventGateway == gatewayRef {
			clusters = append(clusters, cluster)
		}
	}

	return clusters
}

// GetAIGatewayProvidersForGateway returns all providers for a given AI Gateway ref.
func (rs *ResourceSet) GetAIGatewayProvidersForGateway(gatewayRef string) []AIGatewayProviderResource {
	var providers []AIGatewayProviderResource
	for _, provider := range rs.AIGatewayProviders {
		if NormalizeResourceRef(provider.AIGateway) == gatewayRef {
			providers = append(providers, provider)
		}
	}
	return providers
}

// GetAIGatewayPoliciesForGateway returns all AI Gateway Policies for a given gateway ref.
func (rs *ResourceSet) GetAIGatewayPoliciesForGateway(gatewayRef string) []AIGatewayPolicyResource {
	var policies []AIGatewayPolicyResource
	for _, policy := range rs.AIGatewayPolicies {
		if NormalizeResourceRef(policy.AIGateway) == gatewayRef {
			policies = append(policies, policy)
		}
	}
	return policies
}

// GetAIGatewayAgentsForGateway returns all AI Gateway Agents for a given gateway ref.
func (rs *ResourceSet) GetAIGatewayAgentsForGateway(gatewayRef string) []AIGatewayAgentResource {
	var agents []AIGatewayAgentResource
	for _, agent := range rs.AIGatewayAgents {
		if NormalizeResourceRef(agent.AIGateway) == gatewayRef {
			agents = append(agents, agent)
		}
	}
	return agents
}

// GetAIGatewayConsumersForGateway returns all AI Gateway Consumers for a given gateway ref.
func (rs *ResourceSet) GetAIGatewayConsumersForGateway(gatewayRef string) []AIGatewayConsumerResource {
	var consumers []AIGatewayConsumerResource
	for _, consumer := range rs.AIGatewayConsumers {
		if NormalizeResourceRef(consumer.AIGateway) == gatewayRef {
			consumers = append(consumers, consumer)
		}
	}
	return consumers
}

// GetAIGatewayConsumerGroupsForGateway returns all AI Gateway Consumer Groups for a given gateway ref.
func (rs *ResourceSet) GetAIGatewayConsumerGroupsForGateway(gatewayRef string) []AIGatewayConsumerGroupResource {
	var groups []AIGatewayConsumerGroupResource
	for _, group := range rs.AIGatewayConsumerGroups {
		if NormalizeResourceRef(group.AIGateway) == gatewayRef {
			groups = append(groups, group)
		}
	}
	return groups
}

// GetVirtualClustersForGateway returns all virtual clusters (nested + root-level) for a given gateway ref
func (rs *ResourceSet) GetVirtualClustersForGateway(gatewayRef string) []EventGatewayVirtualClusterResource {
	var clusters []EventGatewayVirtualClusterResource

	// Find the gateway to get nested clusters
	for _, gateway := range rs.EventGatewayControlPlanes {
		if gateway.Ref == gatewayRef {
			// Add nested virtual clusters
			for _, cluster := range gateway.VirtualClusters {
				clusterCopy := cluster
				clusterCopy.EventGateway = gatewayRef
				clusters = append(clusters, clusterCopy)
			}
			break
		}
	}

	// Add root-level virtual clusters for this gateway
	for _, cluster := range rs.EventGatewayVirtualClusters {
		if cluster.EventGateway == gatewayRef {
			clusters = append(clusters, cluster)
		}
	}

	return clusters
}

// GetListenersForEventGateway returns all listeners (nested + root-level) for a specific event gateway
func (rs *ResourceSet) GetListenersForEventGateway(gatewayRef string) []EventGatewayListenerResource {
	var listeners []EventGatewayListenerResource

	// Add nested listeners from the event gateway
	for _, gateway := range rs.EventGatewayControlPlanes {
		if gateway.Ref == gatewayRef {
			// Add nested listeners
			for _, listener := range gateway.Listeners {
				listenerCopy := listener
				listenerCopy.EventGateway = gatewayRef
				listeners = append(listeners, listenerCopy)
			}
			break
		}
	}

	// Add root-level listeners for this gateway
	for _, listener := range rs.EventGatewayListeners {
		if listener.EventGateway == gatewayRef {
			listeners = append(listeners, listener)
		}
	}

	return listeners
}

// GetPoliciesForListener returns all listener policies (nested + root-level) for a specific listener
func (rs *ResourceSet) GetPoliciesForListener(listenerRef string) []EventGatewayListenerPolicyResource {
	var policies []EventGatewayListenerPolicyResource

	// Add nested policies from the listener
	// Listeners can be nested inside event gateways or at root level
	for _, gateway := range rs.EventGatewayControlPlanes {
		for _, listener := range gateway.Listeners {
			if listener.Ref == listenerRef {
				for _, policy := range listener.Policies {
					policyCopy := policy
					policyCopy.EventGatewayListener = listenerRef
					policies = append(policies, policyCopy)
				}
			}
		}
	}

	// Check root-level listeners
	for _, listener := range rs.EventGatewayListeners {
		if listener.Ref == listenerRef {
			for _, policy := range listener.Policies {
				policyCopy := policy
				policyCopy.EventGatewayListener = listenerRef
				policies = append(policies, policyCopy)
			}
		}
	}

	// Add root-level policies for this listener
	for _, policy := range rs.EventGatewayListenerPolicies {
		if policy.EventGatewayListener == listenerRef {
			policies = append(policies, policy)
		}
	}

	return policies
}

// GetDataPlaneCertificatesForGateway returns all data plane certificates (nested + root-level)
// for a specific event gateway
func (rs *ResourceSet) GetDataPlaneCertificatesForGateway(
	gatewayRef string,
) []EventGatewayDataPlaneCertificateResource {
	var certs []EventGatewayDataPlaneCertificateResource

	// Add nested data plane certificates from the event gateway
	for _, gateway := range rs.EventGatewayControlPlanes {
		if gateway.Ref == gatewayRef {
			// Add nested certificates
			for _, cert := range gateway.DataPlaneCertificates {
				certCopy := cert
				certCopy.EventGateway = gatewayRef
				certs = append(certs, certCopy)
			}
			break
		}
	}

	// Add root-level data plane certificates for this gateway
	for _, cert := range rs.EventGatewayDataPlaneCertificates {
		if cert.EventGateway == gatewayRef {
			certs = append(certs, cert)
		}
	}

	return certs
}

// GetDataPlaneCertificatesForControlPlane returns all data plane certificates
// (nested + root-level) for a specific control plane.
func (rs *ResourceSet) GetDataPlaneCertificatesForControlPlane(
	controlPlaneRef string,
) []ControlPlaneDataPlaneCertificateResource {
	var certs []ControlPlaneDataPlaneCertificateResource

	for _, controlPlane := range rs.ControlPlanes {
		if controlPlane.Ref == controlPlaneRef {
			for _, cert := range controlPlane.DataPlaneCertificates {
				certCopy := cert
				certCopy.ControlPlane = controlPlaneRef
				certs = append(certs, certCopy)
			}
			break
		}
	}

	for _, cert := range rs.ControlPlaneDataPlaneCertificates {
		if cert.ControlPlane == controlPlaneRef {
			certs = append(certs, cert)
		}
	}

	return certs
}

// GetSchemaRegistriesForGateway returns all schema registries (nested + root-level)
// for a specific event gateway
func (rs *ResourceSet) GetSchemaRegistriesForGateway(
	gatewayRef string,
) []EventGatewaySchemaRegistryResource {
	var regs []EventGatewaySchemaRegistryResource

	// Add nested schema registries from the event gateway
	for _, gateway := range rs.EventGatewayControlPlanes {
		if gateway.Ref == gatewayRef {
			for _, reg := range gateway.SchemaRegistries {
				regCopy := reg
				regCopy.EventGateway = gatewayRef
				regs = append(regs, regCopy)
			}
			break
		}
	}

	// Add root-level schema registries for this gateway
	for _, reg := range rs.EventGatewaySchemaRegistries {
		if reg.EventGateway == gatewayRef {
			regs = append(regs, reg)
		}
	}

	return regs
}

// GetClusterPoliciesForVirtualCluster returns all cluster policies (nested + root-level)
// for a specific virtual cluster
func (rs *ResourceSet) GetClusterPoliciesForVirtualCluster(
	virtualClusterRef string,
) []EventGatewayClusterPolicyResource {
	var policies []EventGatewayClusterPolicyResource

	// Add nested policies from the virtual cluster
	// Virtual clusters can be nested inside event gateways or at root level
	for _, gateway := range rs.EventGatewayControlPlanes {
		for _, vc := range gateway.VirtualClusters {
			if vc.Ref == virtualClusterRef {
				for _, policy := range vc.ClusterPolicies {
					policyCopy := policy
					policyCopy.VirtualCluster = virtualClusterRef
					policies = append(policies, policyCopy)
				}
			}
		}
	}

	// Check root-level virtual clusters
	for _, vc := range rs.EventGatewayVirtualClusters {
		if vc.Ref == virtualClusterRef {
			for _, policy := range vc.ClusterPolicies {
				policyCopy := policy
				policyCopy.VirtualCluster = virtualClusterRef
				policies = append(policies, policyCopy)
			}
		}
	}

	// Add root-level cluster policies for this virtual cluster
	for _, policy := range rs.EventGatewayClusterPolicies {
		if policy.VirtualCluster == virtualClusterRef {
			policies = append(policies, policy)
		}
	}

	return policies
}

// GetProducePoliciesForVirtualCluster returns all produce policies (nested + root-level)
// for a specific virtual cluster
func (rs *ResourceSet) GetProducePoliciesForVirtualCluster(
	virtualClusterRef string,
) []EventGatewayProducePolicyResource {
	var policies []EventGatewayProducePolicyResource

	// Add nested policies from the virtual cluster
	for _, gateway := range rs.EventGatewayControlPlanes {
		for _, vc := range gateway.VirtualClusters {
			if vc.Ref == virtualClusterRef {
				for _, policy := range vc.ProducePolicies {
					policyCopy := policy
					policyCopy.VirtualCluster = virtualClusterRef
					policies = append(policies, policyCopy)
				}
			}
		}
	}

	// Check root-level virtual clusters
	for _, vc := range rs.EventGatewayVirtualClusters {
		if vc.Ref == virtualClusterRef {
			for _, policy := range vc.ProducePolicies {
				policyCopy := policy
				policyCopy.VirtualCluster = virtualClusterRef
				policies = append(policies, policyCopy)
			}
		}
	}

	// Add root-level produce policies for this virtual cluster
	for _, policy := range rs.EventGatewayProducePolicies {
		if policy.VirtualCluster == virtualClusterRef {
			policies = append(policies, policy)
		}
	}

	return policies
}

// GetConsumePoliciesForVirtualCluster returns all consume policies (nested + root-level)
// for a specific virtual cluster.
func (rs *ResourceSet) GetConsumePoliciesForVirtualCluster(
	virtualClusterRef string,
) []EventGatewayConsumePolicyResource {
	var policies []EventGatewayConsumePolicyResource

	// Add nested consume policies from virtual clusters inside event gateways
	for _, gateway := range rs.EventGatewayControlPlanes {
		for _, vc := range gateway.VirtualClusters {
			if vc.Ref == virtualClusterRef {
				for _, policy := range vc.ConsumePolicies {
					policyCopy := policy
					policyCopy.VirtualCluster = virtualClusterRef
					policies = append(policies, policyCopy)
				}
			}
		}
	}

	// Check root-level virtual clusters
	for _, vc := range rs.EventGatewayVirtualClusters {
		if vc.Ref == virtualClusterRef {
			for _, policy := range vc.ConsumePolicies {
				policyCopy := policy
				policyCopy.VirtualCluster = virtualClusterRef
				policies = append(policies, policyCopy)
			}
		}
	}

	// Add root-level consume policies for this virtual cluster
	for _, policy := range rs.EventGatewayConsumePolicies {
		if policy.VirtualCluster == virtualClusterRef {
			policies = append(policies, policy)
		}
	}

	return policies
}

// GetStaticKeysForGateway returns all static keys (nested + root-level)
// for a specific event gateway
func (rs *ResourceSet) GetStaticKeysForGateway(
	gatewayRef string,
) []EventGatewayStaticKeyResource {
	var keys []EventGatewayStaticKeyResource

	// Add nested static keys from the event gateway
	for _, gateway := range rs.EventGatewayControlPlanes {
		if gateway.Ref == gatewayRef {
			for _, sk := range gateway.StaticKeys {
				skCopy := sk
				skCopy.EventGateway = gatewayRef
				keys = append(keys, skCopy)
			}
			break
		}
	}

	// Add root-level static keys for this gateway
	for _, sk := range rs.EventGatewayStaticKeys {
		if sk.EventGateway == gatewayRef {
			keys = append(keys, sk)
		}
	}

	return keys
}

// GetTrustBundlesForGateway returns all TLS trust bundles (nested + root-level)
// for a specific event gateway
func (rs *ResourceSet) GetTrustBundlesForGateway(
	gatewayRef string,
) []EventGatewayTLSTrustBundleResource {
	var bundles []EventGatewayTLSTrustBundleResource

	// Add nested trust bundles from the event gateway
	for _, gateway := range rs.EventGatewayControlPlanes {
		if gateway.Ref == gatewayRef {
			for _, tb := range gateway.TrustBundles {
				tbCopy := tb
				tbCopy.EventGateway = gatewayRef
				bundles = append(bundles, tbCopy)
			}
			break
		}
	}

	// Add root-level trust bundles for this gateway
	for _, tb := range rs.EventGatewayTLSTrustBundles {
		if tb.EventGateway == gatewayRef {
			bundles = append(bundles, tb)
		}
	}

	return bundles
}

// GetAIGatewayModelsForGateway returns all AI Gateway Models for a given gateway ref.
func (rs *ResourceSet) GetAIGatewayModelsForGateway(gatewayRef string) []AIGatewayModelResource {
	var models []AIGatewayModelResource
	for _, model := range rs.AIGatewayModels {
		if NormalizeResourceRef(model.AIGateway) == gatewayRef {
			models = append(models, model)
		}
	}
	return models
}

// GetAIGatewayMCPServersForGateway returns all AI Gateway MCP Servers for a given gateway ref.
func (rs *ResourceSet) GetAIGatewayMCPServersForGateway(gatewayRef string) []AIGatewayMCPServerResource {
	var servers []AIGatewayMCPServerResource
	for _, server := range rs.AIGatewayMCPServers {
		if NormalizeResourceRef(server.AIGateway) == gatewayRef {
			servers = append(servers, server)
		}
	}
	return servers
}

// GetAIGatewayVaultsForGateway returns all AI Gateway Vaults for a given gateway ref.
func (rs *ResourceSet) GetAIGatewayVaultsForGateway(gatewayRef string) []AIGatewayVaultResource {
	var vaults []AIGatewayVaultResource
	for _, vault := range rs.AIGatewayVaults {
		if NormalizeResourceRef(vault.AIGateway) == gatewayRef {
			vaults = append(vaults, vault)
		}
	}
	return vaults
}

// GetAIGatewayNodesForGateway returns all AI Gateway Nodes for a given gateway ref.
func (rs *ResourceSet) GetAIGatewayNodesForGateway(gatewayRef string) []AIGatewayNodeResource {
	var nodes []AIGatewayNodeResource
	for _, node := range rs.AIGatewayNodes {
		if NormalizeResourceRef(node.AIGateway) == gatewayRef {
			nodes = append(nodes, node)
		}
	}
	return nodes
}
