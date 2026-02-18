package resources

// ResourceType represents the type of a declarative resource
type ResourceType string

// Resource type constants
const (
	ResourceTypePortal                     ResourceType = "portal"
	ResourceTypeApplicationAuthStrategy    ResourceType = "application_auth_strategy"
	ResourceTypeControlPlane               ResourceType = "control_plane"
	ResourceTypeAPI                        ResourceType = "api"
	ResourceTypeAPIVersion                 ResourceType = "api_version"
	ResourceTypeAPIPublication             ResourceType = "api_publication"
	ResourceTypeAPIImplementation          ResourceType = "api_implementation"
	ResourceTypeAPIDocument                ResourceType = "api_document"
	ResourceTypeGatewayService             ResourceType = "gateway_service"
	ResourceTypePortalCustomization        ResourceType = "portal_customization"
	ResourceTypePortalCustomDomain         ResourceType = "portal_custom_domain"
	ResourceTypePortalAuthSettings         ResourceType = "portal_auth_settings"
	ResourceTypePortalPage                 ResourceType = "portal_page"
	ResourceTypePortalSnippet              ResourceType = "portal_snippet"
	ResourceTypePortalTeam                 ResourceType = "portal_team"
	ResourceTypePortalTeamRole             ResourceType = "portal_team_role"
	ResourceTypePortalAssetLogo            ResourceType = "portal_asset_logo"
	ResourceTypePortalAssetFavicon         ResourceType = "portal_asset_favicon"
	ResourceTypePortalEmailConfig          ResourceType = "portal_email_config"
	ResourceTypePortalEmailTemplate        ResourceType = "portal_email_template"
	ResourceTypeCatalogService             ResourceType = "catalog_service"
	ResourceTypeEventGatewayControlPlane   ResourceType = "event_gateway"
	ResourceTypeEventGatewayBackendCluster ResourceType = "event_gateway_backend_cluster"
	ResourceTypeEventGatewayVirtualCluster ResourceType = "event_gateway_virtual_cluster"
	ResourceTypeOrganizationTeam           ResourceType = "organization_team"
)

const (
	// NamespaceExternal is a sentinel (empty string) used internally when handling external resources.
	NamespaceExternal = ""
)

// ResourceRef represents a reference to another resource
type ResourceRef struct {
	Kind string `json:"kind" yaml:"kind"`
	Ref  string `json:"ref"  yaml:"ref"`
}

// ResourceSet contains all declarative resources from configuration files
type ResourceSet struct {
	Portals []PortalResource `yaml:"portals,omitempty"                        json:"portals,omitempty"`
	// ApplicationAuthStrategies contains auth strategy configurations
	ApplicationAuthStrategies []ApplicationAuthStrategyResource `yaml:"application_auth_strategies,omitempty"    json:"application_auth_strategies,omitempty"` //nolint:lll
	// ControlPlanes contains control plane configurations
	ControlPlanes   []ControlPlaneResource   `yaml:"control_planes,omitempty"                                 json:"control_planes,omitempty"` //nolint:lll //nolint:lll
	CatalogServices []CatalogServiceResource `yaml:"catalog_services,omitempty"                             json:"catalog_services,omitempty"` //nolint:lll //nolint:lll
	APIs            []APIResource            `yaml:"apis,omitempty"                           json:"apis,omitempty"`
	GatewayServices []GatewayServiceResource `yaml:"gateway_services,omitempty"               json:"gateway_services,omitempty"` //nolint:lll
	// API child resources can be defined at root level (with parent reference) or nested under APIs
	APIVersions        []APIVersionResource        `yaml:"api_versions,omitempty"                   json:"api_versions,omitempty"`        //nolint:lll
	APIPublications    []APIPublicationResource    `yaml:"api_publications,omitempty"               json:"api_publications,omitempty"`    //nolint:lll
	APIImplementations []APIImplementationResource `yaml:"api_implementations,omitempty"            json:"api_implementations,omitempty"` //nolint:lll
	APIDocuments       []APIDocumentResource       `yaml:"api_documents,omitempty"                  json:"api_documents,omitempty"`       //nolint:lll
	// Portal child resources can be defined at root level (with parent reference) or nested under Portals
	PortalCustomizations        []PortalCustomizationResource        `yaml:"portal_customizations,omitempty"          json:"portal_customizations,omitempty"`          //nolint:lll
	PortalAuthSettings          []PortalAuthSettingsResource         `yaml:"portal_auth_settings,omitempty"           json:"portal_auth_settings,omitempty"`           //nolint:lll
	PortalCustomDomains         []PortalCustomDomainResource         `yaml:"portal_custom_domains,omitempty"          json:"portal_custom_domains,omitempty"`          //nolint:lll
	PortalPages                 []PortalPageResource                 `yaml:"portal_pages,omitempty"                   json:"portal_pages,omitempty"`                   //nolint:lll
	PortalSnippets              []PortalSnippetResource              `yaml:"portal_snippets,omitempty"                json:"portal_snippets,omitempty"`                //nolint:lll
	PortalTeams                 []PortalTeamResource                 `yaml:"portal_teams,omitempty"                   json:"portal_teams,omitempty"`                   //nolint:lll
	PortalTeamRoles             []PortalTeamRoleResource             `yaml:"portal_team_roles,omitempty"              json:"portal_team_roles,omitempty"`              //nolint:lll
	PortalAssetLogos            []PortalAssetLogoResource            `yaml:"portal_asset_logos,omitempty"             json:"portal_asset_logos,omitempty"`             //nolint:lll
	PortalAssetFavicons         []PortalAssetFaviconResource         `yaml:"portal_asset_favicons,omitempty"          json:"portal_asset_favicons,omitempty"`          //nolint:lll
	PortalEmailConfigs          []PortalEmailConfigResource          `yaml:"portal_email_configs,omitempty"           json:"portal_email_configs,omitempty"`           //nolint:lll
	PortalEmailTemplates        []PortalEmailTemplateResource        `yaml:"portal_email_templates,omitempty"         json:"portal_email_templates,omitempty"`         //nolint:lll
	EventGatewayControlPlanes   []EventGatewayControlPlaneResource   `yaml:"event_gateways,omitempty"                 json:"event_gateways,omitempty"`                 //nolint:lll
	EventGatewayBackendClusters []EventGatewayBackendClusterResource `yaml:"event_gateway_backend_clusters,omitempty" json:"event_gateway_backend_clusters,omitempty"` //nolint:lll
	EventGatewayVirtualClusters []EventGatewayVirtualClusterResource `yaml:"event_gateway_virtual_clusters,omitempty" json:"event_gateway_virtual_clusters,omitempty"` //nolint:lll
	// Organization grouping - contains nested resources like teams
	Organization *OrganizationResource `yaml:"organization,omitempty"                   json:"organization,omitempty"`
	// Teams is populated internally from OrganizationTeams during loading
	// It is not exposed in YAML/JSON to enforce the organization grouping format
	OrganizationTeams []OrganizationTeamResource `yaml:"-"                                        json:"-"`
	// DefaultNamespace tracks namespace from _defaults when no resources are present
	// This is used by the planner to determine which namespace to check for deletions
	DefaultNamespace  string   `yaml:"-"                                        json:"-"`
	DefaultNamespaces []string `yaml:"-"                                        json:"-"`
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
	// Check Portals
	for i := range rs.Portals {
		if rs.Portals[i].GetRef() == ref {
			return &rs.Portals[i], true
		}
	}

	// Check ApplicationAuthStrategies
	for i := range rs.ApplicationAuthStrategies {
		if rs.ApplicationAuthStrategies[i].GetRef() == ref {
			return &rs.ApplicationAuthStrategies[i], true
		}
	}

	// Check ControlPlanes
	for i := range rs.ControlPlanes {
		if rs.ControlPlanes[i].GetRef() == ref {
			return &rs.ControlPlanes[i], true
		}
	}

	// Check Catalog Services
	for i := range rs.CatalogServices {
		if rs.CatalogServices[i].GetRef() == ref {
			return &rs.CatalogServices[i], true
		}
	}

	// Check APIs
	for i := range rs.APIs {
		if rs.APIs[i].GetRef() == ref {
			return &rs.APIs[i], true
		}
	}

	// Check Gateway services
	for i := range rs.GatewayServices {
		if rs.GatewayServices[i].GetRef() == ref {
			return &rs.GatewayServices[i], true
		}
	}

	// Check API child resources
	for i := range rs.APIVersions {
		if rs.APIVersions[i].GetRef() == ref {
			return &rs.APIVersions[i], true
		}
	}

	for i := range rs.APIPublications {
		if rs.APIPublications[i].GetRef() == ref {
			return &rs.APIPublications[i], true
		}
	}

	for i := range rs.APIImplementations {
		if rs.APIImplementations[i].GetRef() == ref {
			return &rs.APIImplementations[i], true
		}
	}

	for i := range rs.APIDocuments {
		if rs.APIDocuments[i].GetRef() == ref {
			return &rs.APIDocuments[i], true
		}
	}

	// Check Portal child resources
	for i := range rs.PortalCustomizations {
		if rs.PortalCustomizations[i].GetRef() == ref {
			return &rs.PortalCustomizations[i], true
		}
	}
	for i := range rs.PortalAuthSettings {
		if rs.PortalAuthSettings[i].GetRef() == ref {
			return &rs.PortalAuthSettings[i], true
		}
	}

	for i := range rs.PortalCustomDomains {
		if rs.PortalCustomDomains[i].GetRef() == ref {
			return &rs.PortalCustomDomains[i], true
		}
	}

	for i := range rs.PortalPages {
		if rs.PortalPages[i].GetRef() == ref {
			return &rs.PortalPages[i], true
		}
	}

	for i := range rs.PortalSnippets {
		if rs.PortalSnippets[i].GetRef() == ref {
			return &rs.PortalSnippets[i], true
		}
	}

	for i := range rs.PortalTeams {
		if rs.PortalTeams[i].GetRef() == ref {
			return &rs.PortalTeams[i], true
		}
	}

	for i := range rs.PortalTeamRoles {
		if rs.PortalTeamRoles[i].GetRef() == ref {
			return &rs.PortalTeamRoles[i], true
		}
	}

	for i := range rs.PortalAssetLogos {
		if rs.PortalAssetLogos[i].GetRef() == ref {
			return &rs.PortalAssetLogos[i], true
		}
	}

	for i := range rs.PortalAssetFavicons {
		if rs.PortalAssetFavicons[i].GetRef() == ref {
			return &rs.PortalAssetFavicons[i], true
		}
	}

	for i := range rs.PortalEmailConfigs {
		if rs.PortalEmailConfigs[i].GetRef() == ref {
			return &rs.PortalEmailConfigs[i], true
		}
	}

	for i := range rs.PortalEmailTemplates {
		if rs.PortalEmailTemplates[i].GetRef() == ref {
			return &rs.PortalEmailTemplates[i], true
		}
	}

	// Check Teams
	for i := range rs.OrganizationTeams {
		if rs.OrganizationTeams[i].GetRef() == ref {
			return &rs.OrganizationTeams[i], true
		}
	}

	return nil, false
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

// GetAuthStrategyByRef returns an auth strategy resource by its ref from any namespace
func (rs *ResourceSet) GetAuthStrategyByRef(ref string) *ApplicationAuthStrategyResource {
	for i := range rs.ApplicationAuthStrategies {
		if rs.ApplicationAuthStrategies[i].GetRef() == ref {
			return &rs.ApplicationAuthStrategies[i]
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
	for _, ns := range rs.DefaultNamespaces {
		if ns == namespace {
			return
		}
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
