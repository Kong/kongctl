package planner

// Internal field names used for communication between planner and executor
// These fields are prefixed with underscore to indicate they are internal
// and should not be confused with actual resource fields
const (
	// FieldCurrentLabels contains the current labels of a resource
	// Used during updates to determine which labels should be removed
	FieldCurrentLabels = "_current_labels"

	// FieldStrategyType contains the current strategy type for auth strategies
	// Used during updates since strategy type cannot be changed
	FieldStrategyType = "_strategy_type"

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

	// ResourceTypeDeck represents an internal deck execution step.
	ResourceTypeDeck = "_deck"
)

// Default values
const (
	// DefaultNamespace is the default namespace when none is specified
	DefaultNamespace = "default"
)
