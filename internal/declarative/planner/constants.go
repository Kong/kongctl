package planner

import "github.com/kong/kongctl/internal/declarative/resources"

// Field names used for communication between planner and executor. Internal
// fields are prefixed with underscore to avoid confusion with resource fields.
const (
	// FieldID contains a resource ID
	FieldID = "id"

	// FieldName contains a resource name
	FieldName = "name"

	// FieldDescription contains a resource description
	FieldDescription = "description"

	// FieldCurrentLabels contains the current labels of a resource
	// Used during updates to determine which labels should be removed
	FieldCurrentLabels = "_current_labels"

	// FieldStrategyType contains the current strategy type for auth strategies
	// Used during updates since strategy type cannot be changed
	FieldStrategyType = "_strategy_type"

	// FieldDCRProviderUpdateType contains the current provider type for DCR providers
	// Used during updates since provider type cannot be changed and update config is a union type
	FieldDCRProviderUpdateType = "_provider_type"

	// FieldDCRProviderID contains the DCR provider reference or resolved ID for OIDC auth strategies
	FieldDCRProviderID = "dcr_provider_id"

	// FieldDCRProviderProviderType contains the DCR provider type
	FieldDCRProviderProviderType = "provider_type"

	// FieldDCRProviderIssuer contains the DCR provider issuer URL
	FieldDCRProviderIssuer = "issuer"

	// FieldDCRProviderConfig contains provider-specific DCR configuration
	FieldDCRProviderConfig = "dcr_config"

	// FieldDisplayName contains a resource display name
	FieldDisplayName = "display_name"

	// FieldLabels contains user-managed labels
	FieldLabels = "labels"

	// FieldPreservedLabels contains labels preserved during resource recreation
	FieldPreservedLabels = "preserved_labels"

	// FieldError contains validation errors that should be reported
	// Used when the planner detects an invalid operation
	FieldError = "_error"
)

// Common plan field identifiers.
const (
	FieldNamespace   = "namespace"
	FieldContent     = "content"
	FieldTitle       = "title"
	FieldStatus      = "status"
	FieldAttributes  = "attributes"
	FieldType        = "type"
	FieldConfig      = "config"
	FieldConfigs     = "configs"
	FieldEnabled     = "enabled"
	FieldVersion     = "version"
	FieldSpec        = "spec"
	FieldSlug        = "slug"
	FieldValue       = "value"
	FieldMetadata    = "metadata"
	FieldDataURL     = "data_url"
	FieldDeckBaseDir = "deck_base_dir"
	FieldFlags       = "flags"
	FieldFiles       = "files"
)

// Common relationship and reference field identifiers.
const (
	FieldAPI                          = "api"
	FieldAPIID                        = "api_id"
	FieldAuthStrategyIDs              = "auth_strategy_ids"
	FieldAuthStrategyType             = "strategy_type"
	FieldControlPlaneID               = "control_plane_id"
	FieldControlPlaneName             = "control_plane_name"
	FieldControlPlaneRef              = "control_plane_ref"
	FieldDefaultApplicationStrategyID = "default_application_auth_strategy_id"
	FieldDCRProvider                  = "dcr_provider"
	FieldEntityID                     = "entity_id"
	FieldEntityRegion                 = "entity_region"
	FieldEntityTypeName               = "entity_type_name"
	FieldEventGatewayID               = "event_gateway_id"
	FieldEventGatewayBackendClusterID = "event_gateway_backend_cluster_id"
	FieldEventGatewayListenerID       = "event_gateway_listener_id"
	FieldEventGatewayVirtualClusterID = "event_gateway_virtual_cluster_id"
	FieldGatewayServices              = "gateway_services"
	FieldParentDocumentID             = "parent_document_id"
	FieldParentPageID                 = "parent_page_id"
	FieldParentPath                   = "parent_path"
	FieldPortalID                     = "portal_id"
	FieldRoleName                     = "role_name"
	FieldService                      = "service"
	FieldSlugPath                     = "slug_path"
	FieldTeamID                       = "team_id"
)

// Common portal plan field identifiers.
const (
	FieldAuthenticationEnabled        = "authentication_enabled"
	FieldAutoApproveApplications      = "auto_approve_applications"
	FieldAutoApproveDevelopers        = "auto_approve_developers"
	FieldAutoApproveRegistrations     = "auto_approve_registrations"
	FieldBasicAuthEnabled             = "basic_auth_enabled"
	FieldCanOwnApplications           = "can_own_applications"
	FieldCSS                          = "css"
	FieldDefaultAPIVisibility         = "default_api_visibility"
	FieldDefaultPageVisibility        = "default_page_visibility"
	FieldDomainName                   = "domain_name"
	FieldFromEmail                    = "from_email"
	FieldFromName                     = "from_name"
	FieldHostname                     = "hostname"
	FieldIDPMappingEnabled            = "idp_mapping_enabled"
	FieldKonnectMappingEnabled        = "konnect_mapping_enabled"
	FieldLayout                       = "layout"
	FieldLoginPath                    = "login_path"
	FieldMenu                         = "menu"
	FieldReplyToEmail                 = "reply_to_email"
	FieldRBACEnabled                  = "rbac_enabled"
	FieldSSL                          = "ssl"
	FieldTheme                        = "theme"
	FieldVisibility                   = "visibility"
	FieldCustomFields                 = "custom_fields"
	FieldAuthenticationStrategyUpdate = "authentication_strategy_update"
)

// Common gateway and event-gateway plan field identifiers.
const (
	FieldACLMode                                    = "acl_mode"
	FieldAddresses                                  = "addresses"
	FieldAuthentication                             = "authentication"
	FieldAuthType                                   = "auth_type"
	FieldBootstrapServers                           = "bootstrap_servers"
	FieldCertificate                                = "certificate"
	FieldCert                                       = "cert"
	FieldCloudGateway                               = "cloud_gateway"
	FieldClusterType                                = "cluster_type"
	FieldDestination                                = "destination"
	FieldDNSLabel                                   = "dns_label"
	FieldInsecureAllowAnonymousVirtualClusterAuth   = "insecure_allow_anonymous_virtual_cluster_auth"
	FieldMembers                                    = "members"
	FieldMetadataUpdateIntervalSeconds              = "metadata_update_interval_seconds"
	FieldPorts                                      = "ports"
	FieldProxyURLs                                  = "proxy_urls"
	FieldTLS                                        = "tls"
	FieldDefaultVirtualClusterTarget                = "default_virtual_cluster_target"
	FieldGatewayControlPlaneMembershipControlPlanes = "control_planes"
)

// Resource type constants are aliases for the canonical resource identifiers in
// the resources package. Keep these only for planner/executor compatibility.
const (
	ResourceTypePortal                           = string(resources.ResourceTypePortal)
	ResourceTypeApplicationAuthStrategy          = string(resources.ResourceTypeApplicationAuthStrategy)
	ResourceTypeDCRProvider                      = string(resources.ResourceTypeDCRProvider)
	ResourceTypeControlPlane                     = string(resources.ResourceTypeControlPlane)
	ResourceTypeAPI                              = string(resources.ResourceTypeAPI)
	ResourceTypeAPIVersion                       = string(resources.ResourceTypeAPIVersion)
	ResourceTypeAPIPublication                   = string(resources.ResourceTypeAPIPublication)
	ResourceTypeAPIImplementation                = string(resources.ResourceTypeAPIImplementation)
	ResourceTypeAPIDocument                      = string(resources.ResourceTypeAPIDocument)
	ResourceTypeGatewayService                   = string(resources.ResourceTypeGatewayService)
	ResourceTypeControlPlaneDataPlaneCertificate = string(resources.ResourceTypeControlPlaneDataPlaneCertificate)
	ResourceTypeCatalogService                   = string(resources.ResourceTypeCatalogService)

	ResourceTypePortalCustomization    = string(resources.ResourceTypePortalCustomization)
	ResourceTypePortalCustomDomain     = string(resources.ResourceTypePortalCustomDomain)
	ResourceTypePortalAuthSettings     = string(resources.ResourceTypePortalAuthSettings)
	ResourceTypePortalIdentityProvider = string(resources.ResourceTypePortalIdentityProvider)
	ResourceTypePortalPage             = string(resources.ResourceTypePortalPage)
	ResourceTypePortalSnippet          = string(resources.ResourceTypePortalSnippet)
	ResourceTypePortalTeam             = string(resources.ResourceTypePortalTeam)
	ResourceTypePortalTeamRole         = string(resources.ResourceTypePortalTeamRole)
	ResourceTypePortalAssetLogo        = string(resources.ResourceTypePortalAssetLogo)
	ResourceTypePortalAssetFavicon     = string(resources.ResourceTypePortalAssetFavicon)
	ResourceTypePortalEmailConfig      = string(resources.ResourceTypePortalEmailConfig)
	ResourceTypePortalEmailTemplate    = string(resources.ResourceTypePortalEmailTemplate)

	ResourceTypeEventGatewayControlPlane         = string(resources.ResourceTypeEventGatewayControlPlane)
	ResourceTypeEventGatewayBackendCluster       = string(resources.ResourceTypeEventGatewayBackendCluster)
	ResourceTypeEventGatewayVirtualCluster       = string(resources.ResourceTypeEventGatewayVirtualCluster)
	ResourceTypeEventGatewayListener             = string(resources.ResourceTypeEventGatewayListener)
	ResourceTypeEventGatewayListenerPolicy       = string(resources.ResourceTypeEventGatewayListenerPolicy)
	ResourceTypeEventGatewayDataPlaneCertificate = string(resources.ResourceTypeEventGatewayDataPlaneCertificate)
	ResourceTypeEventGatewayClusterPolicy        = string(resources.ResourceTypeEventGatewayClusterPolicy)
	ResourceTypeEventGatewayProducePolicy        = string(resources.ResourceTypeEventGatewayProducePolicy)
	ResourceTypeEventGatewayConsumePolicy        = string(resources.ResourceTypeEventGatewayConsumePolicy)
	ResourceTypeEventGatewaySchemaRegistry       = string(resources.ResourceTypeEventGatewaySchemaRegistry)
	ResourceTypeEventGatewayStaticKey            = string(resources.ResourceTypeEventGatewayStaticKey)
	ResourceTypeEventGatewayTLSTrustBundle       = string(resources.ResourceTypeEventGatewayTLSTrustBundle)
	ResourceTypeOrganizationTeam                 = string(resources.ResourceTypeOrganizationTeam)

	// ResourceTypeDeck represents an internal deck execution step.
	ResourceTypeDeck = "_deck"
)

// Default values
const (
	// DefaultNamespace is the default namespace when none is specified
	DefaultNamespace = "default"
)
