package planner

// Field names used for communication between planner and executor. Internal
// fields are prefixed with underscore to avoid confusion with resource fields.
const (
	// FieldName contains a resource name
	FieldName = "name"

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

	// FieldError contains validation errors that should be reported
	// Used when the planner detects an invalid operation
	FieldError = "_error"
)

// Resource type constants
const (
	// ResourceTypePortal is the resource type for portals
	ResourceTypePortal = "portal"

	// ResourceTypePortalPage is the resource type for portal pages
	ResourceTypePortalPage = "portal_page"

	// ResourceTypePortalSnippet is the resource type for portal snippets
	ResourceTypePortalSnippet = "portal_snippet"

	// ResourceTypePortalTeam is the resource type for portal teams
	ResourceTypePortalTeam = "portal_team"

	// ResourceTypePortalTeamRole is the resource type for portal team roles
	ResourceTypePortalTeamRole = "portal_team_role"

	// ResourceTypePortalCustomization is the resource type for portal customizations
	ResourceTypePortalCustomization = "portal_customization"

	// ResourceTypePortalAuthSettings is the resource type for portal auth settings
	ResourceTypePortalAuthSettings = "portal_auth_settings"

	// ResourceTypePortalIdentityProvider is the resource type for portal identity providers
	ResourceTypePortalIdentityProvider = "portal_identity_provider"

	// ResourceTypePortalCustomDomain is the resource type for portal custom domains
	ResourceTypePortalCustomDomain = "portal_custom_domain"

	// ResourceTypePortalAssetLogo is the resource type for portal logo assets
	ResourceTypePortalAssetLogo = "portal_asset_logo"

	// ResourceTypePortalAssetFavicon is the resource type for portal favicon assets
	ResourceTypePortalAssetFavicon = "portal_asset_favicon"

	// ResourceTypePortalEmailConfig is the resource type for portal email configs
	ResourceTypePortalEmailConfig = "portal_email_config"

	// ResourceTypePortalEmailTemplate is the resource type for portal email templates
	ResourceTypePortalEmailTemplate = "portal_email_template"

	// ResourceTypeEventGatewayControlPlane is the resource type for event gateway control planes
	ResourceTypeEventGatewayControlPlane = "event_gateway"

	// ResourceTypeEventGatewayBackendCluster is the resource type for event gateway backend clusters
	ResourceTypeEventGatewayBackendCluster = "event_gateway_backend_cluster"

	// ResourceTypeEventGatewayVirtualCluster is the resource type for event gateway virtual clusters
	ResourceTypeEventGatewayVirtualCluster = "event_gateway_virtual_cluster"

	// ResourceTypeEventGatewayListener is the resource type for event gateway listeners
	ResourceTypeEventGatewayListener = "event_gateway_listener"

	// ResourceTypeEventGatewayListenerPolicy is the resource type for event gateway listener policies
	ResourceTypeEventGatewayListenerPolicy = "event_gateway_listener_policy"

	// ResourceTypeEventGatewayDataPlaneCertificate is the resource type for event gateway data plane certificates
	ResourceTypeEventGatewayDataPlaneCertificate = "event_gateway_data_plane_certificate"

	// ResourceTypeEventGatewayClusterPolicy is the resource type for event gateway cluster-level policies
	ResourceTypeEventGatewayClusterPolicy = "event_gateway_virtual_cluster_cluster_policy"

	// ResourceTypeEventGatewayProducePolicy is the resource type for event gateway produce policies
	ResourceTypeEventGatewayProducePolicy = "event_gateway_virtual_cluster_produce_policy"
	// ResourceTypeEventGatewayConsumePolicy is the resource type for event gateway virtual cluster consume policies
	ResourceTypeEventGatewayConsumePolicy = "event_gateway_virtual_cluster_consume_policy"
	// ResourceTypeEventGatewaySchemaRegistry is the resource type for event gateway schema registries
	ResourceTypeEventGatewaySchemaRegistry = "event_gateway_schema_registry"

	// ResourceTypeEventGatewayStaticKey is the resource type for event gateway static keys.
	// Static keys do not support update – changes are applied as delete + create.
	ResourceTypeEventGatewayStaticKey = "event_gateway_static_key"

	// ResourceTypeEventGatewayTLSTrustBundle is the resource type for event gateway TLS trust bundles.
	ResourceTypeEventGatewayTLSTrustBundle = "event_gateway_tls_trust_bundle"

	// ResourceTypeDeck represents an internal deck execution step.
	ResourceTypeDeck = "_deck"
)

// Default values
const (
	// DefaultNamespace is the default namespace when none is specified
	DefaultNamespace = "default"
)
