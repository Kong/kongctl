package helpers

import (
	"log/slog"

	kkSDK "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect

	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/apiutil"
)

const (
	// EnvTrue is the string value "true" used for environment variable checks
	EnvTrue = "true"
)

// Provides an interface for the Konnect Go SDK
// "github.com/Kong/sdk-konnect-go" SDK struct
// allowing for easier testing and mocking
type SDKAPI interface {
	GetControlPlaneAPI() ControlPlaneAPI
	GetControlPlaneGroupsAPI() ControlPlaneGroupsAPI
	GetPortalAPI() PortalAPI
	GetAPIAPI() APIFullAPI // TODO: Change to APIAPI once refactoring is complete
	GetAIGatewayAPI() AIGatewayAPI
	GetAIGatewayProvidersAPI() AIGatewayProvidersAPI
	GetAIGatewayPoliciesAPI() AIGatewayPoliciesAPI
	GetAIGatewayAgentsAPI() AIGatewayAgentsAPI
	GetAIGatewayConsumersAPI() AIGatewayConsumersAPI
	GetAIGatewayConsumerGroupsAPI() AIGatewayConsumerGroupsAPI
	GetAIGatewayModelAPI() AIGatewayModelAPI
	GetAIGatewayMCPServersAPI() AIGatewayMCPServersAPI
	GetAIGatewayVaultsAPI() AIGatewayVaultsAPI
	GetAIGatewayNodesAPI() AIGatewayNodesAPI
	GetCatalogServicesAPI() CatalogServicesAPI
	GetDashboardsAPI() DashboardsAPI
	GetAPIDocumentAPI() APIDocumentAPI
	GetAPIVersionAPI() APIVersionAPI
	GetAPIPublicationAPI() APIPublicationAPI
	GetAPIImplementationAPI() APIImplementationAPI
	GetAppAuthStrategiesAPI() AppAuthStrategiesAPI
	GetDCRProvidersAPI() DCRProvidersAPI
	GetMeAPI() MeAPI
	GetPersonalAccessTokenAPI() PersonalAccessTokenAPI
	GetSystemAccountAccessTokenAPI() SystemAccountAccessTokenAPI
	GetGatewayServiceAPI() GatewayServiceAPI
	GetDataPlaneCertificateAPI() DataPlaneCertificateAPI
	GetSystemAccountAPI() SystemAccountAPI
	GetSystemAccountRolesAPI() SystemAccountRolesAPI
	GetSystemAccountTeamMembershipAPI() SystemAccountTeamMembershipAPI
	GetOrganizationTeamAPI() OrganizationTeamAPI
	GetOrganizationTeamRolesAPI() OrganizationTeamRolesAPI
	GetOrganizationUsersAPI() OrganizationUsersAPI
	GetOrganizationTeamMembershipAPI() OrganizationTeamMembershipAPI
	// Portal child resource APIs
	GetPortalPageAPI() PortalPageAPI
	GetPortalAuthSettingsAPI() PortalAuthSettingsAPI
	GetPortalIPAllowListAPI() PortalIPAllowListAPI
	GetPortalIntegrationsAPI() PortalIntegrationsAPI
	GetPortalIdentityProviderAPI() PortalIdentityProviderAPI
	GetPortalCustomizationAPI() PortalCustomizationAPI
	GetPortalCustomDomainAPI() PortalCustomDomainAPI
	GetPortalSnippetAPI() PortalSnippetAPI
	GetPortalApplicationAPI() PortalApplicationAPI
	GetPortalApplicationRegistrationAPI() PortalApplicationRegistrationAPI
	GetPortalDeveloperAPI() PortalDeveloperAPI
	GetPortalTeamAPI() PortalTeamAPI
	GetPortalTeamRolesAPI() PortalTeamRolesAPI
	GetPortalTeamMembershipAPI() PortalTeamMembershipAPI
	GetAssetsAPI() AssetsAPI
	GetPortalEmailsAPI() PortalEmailsAPI
	GetPortalAuditLogsAPI() PortalAuditLogsAPI
	GetAuditLogDestinationsAPI() AuditLogDestinationsAPI
	GetEventGatewayControlPlaneAPI() EGWControlPlaneAPI
	GetEventGatewayBackendClusterAPI() EventGatewayBackendClusterAPI
	GetEventGatewayVirtualClusterAPI() EventGatewayVirtualClusterAPI
	GetEventGatewayListenerAPI() EventGatewayListenerAPI
	GetEventGatewayListenerPolicyAPI() EventGatewayListenerPolicyAPI
	GetEventGatewayClusterPolicyAPI() EventGatewayClusterPolicyAPI
	GetEventGatewayProducePolicyAPI() EventGatewayProducePolicyAPI
	GetEventGatewayConsumePolicyAPI() EventGatewayConsumePolicyAPI
	GetEventGatewayDataPlaneCertificateAPI() EventGatewayDataPlaneCertificateAPI
	GetEventGatewaySchemaRegistryAPI() EventGatewaySchemaRegistryAPI
	GetEventGatewayStaticKeyAPI() EventGatewayStaticKeyAPI
	GetEventGatewayTLSTrustBundleAPI() EventGatewayTLSTrustBundleAPI
}

// This is the real implementation of the SDKAPI
// which wraps the actual SDK implmentation
type KonnectSDK struct {
	SDK         *kkSDK.SDK
	BaseURL     string
	Token       string
	TokenSource apiutil.TokenSource
	HTTPClient  kkSDK.HTTPClient
	portalImpl  *PortalAPIImpl
}

// Returns the real implementation of the GetControlPlaneAPI
// from the Konnect SDK
func (k *KonnectSDK) GetControlPlaneAPI() ControlPlaneAPI {
	return k.SDK.ControlPlanes
}

// Returns the implementation of the ControlPlaneGroupsAPI interface
func (k *KonnectSDK) GetControlPlaneGroupsAPI() ControlPlaneGroupsAPI {
	if k.SDK == nil {
		return nil
	}

	return k.SDK.ControlPlaneGroups
}

// Returns the implementation of the PortalAPI interface
func (k *KonnectSDK) GetPortalAPI() PortalAPI {
	if k.portalImpl == nil && k.SDK != nil {
		k.portalImpl = &PortalAPIImpl{
			SDK: k.SDK,
		}
	}
	return k.portalImpl
}

// Returns the implementation of the APIAPI interface
func (k *KonnectSDK) GetAPIAPI() APIFullAPI {
	if k.SDK == nil {
		return nil
	}

	return &APIAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the AIGatewayAPI interface.
func (k *KonnectSDK) GetAIGatewayAPI() AIGatewayAPI {
	if k.SDK == nil {
		return nil
	}

	return &AIGatewayAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the AIGatewayProvidersAPI interface.
func (k *KonnectSDK) GetAIGatewayProvidersAPI() AIGatewayProvidersAPI {
	if k.SDK == nil || k.SDK.AIGatewayProviders == nil {
		return nil
	}

	return &AIGatewayProvidersAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the AIGatewayPoliciesAPI interface.
func (k *KonnectSDK) GetAIGatewayPoliciesAPI() AIGatewayPoliciesAPI {
	if k.SDK == nil || k.SDK.AIGatewayPolicies == nil {
		return nil
	}

	return &AIGatewayPoliciesAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the AIGatewayConsumersAPI interface.
func (k *KonnectSDK) GetAIGatewayConsumersAPI() AIGatewayConsumersAPI {
	if k.SDK == nil || k.SDK.AIGatewayConsumers == nil {
		return nil
	}

	return &AIGatewayConsumersAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the AIGatewayAgentsAPI interface.
func (k *KonnectSDK) GetAIGatewayAgentsAPI() AIGatewayAgentsAPI {
	if k.SDK == nil || k.SDK.AIGatewayAgents == nil {
		return nil
	}

	return &AIGatewayAgentsAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the AIGatewayConsumerGroupsAPI interface.
func (k *KonnectSDK) GetAIGatewayConsumerGroupsAPI() AIGatewayConsumerGroupsAPI {
	if k.SDK == nil || k.SDK.AIGatewayConsumerGroups == nil {
		return nil
	}

	return &AIGatewayConsumerGroupsAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the AIGatewayModelAPI interface.
func (k *KonnectSDK) GetAIGatewayModelAPI() AIGatewayModelAPI {
	if k.SDK == nil || k.SDK.AIGatewayModels == nil {
		return nil
	}

	return &AIGatewayModelAPIImpl{
		SDK:         k.SDK,
		BaseURL:     k.BaseURL,
		Token:       k.Token,
		TokenSource: k.TokenSource,
		HTTPClient:  k.HTTPClient,
	}
}

// Returns the implementation of the AIGatewayMCPServersAPI interface.
func (k *KonnectSDK) GetAIGatewayMCPServersAPI() AIGatewayMCPServersAPI {
	if k.SDK == nil || k.SDK.AIGatewayMCPServers == nil {
		return nil
	}

	return &AIGatewayMCPServersAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the AIGatewayVaultsAPI interface.
func (k *KonnectSDK) GetAIGatewayVaultsAPI() AIGatewayVaultsAPI {
	if k.SDK == nil || k.SDK.AIGatewayVaults == nil {
		return nil
	}

	return &AIGatewayVaultsAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the AIGatewayNodesAPI interface.
func (k *KonnectSDK) GetAIGatewayNodesAPI() AIGatewayNodesAPI {
	if k.SDK == nil || k.SDK.AIGatewayNodes == nil {
		return nil
	}

	return &AIGatewayNodesAPIImpl{
		SDK:         k.SDK,
		BaseURL:     k.BaseURL,
		Token:       k.Token,
		TokenSource: k.TokenSource,
		HTTPClient:  k.HTTPClient,
	}
}

// Returns the implementation of the CatalogServicesAPI interface
func (k *KonnectSDK) GetCatalogServicesAPI() CatalogServicesAPI {
	if k.SDK == nil {
		return nil
	}

	return &CatalogServicesAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the DashboardsAPI interface.
func (k *KonnectSDK) GetDashboardsAPI() DashboardsAPI {
	if k.SDK == nil {
		return nil
	}

	return &DashboardsAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the APIDocumentAPI interface
func (k *KonnectSDK) GetAPIDocumentAPI() APIDocumentAPI {
	if k.SDK == nil {
		return nil
	}

	return &APIDocumentAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the APIVersionAPI interface
func (k *KonnectSDK) GetAPIVersionAPI() APIVersionAPI {
	if k.SDK == nil {
		return nil
	}

	return &APIVersionAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the APIPublicationAPI interface
func (k *KonnectSDK) GetAPIPublicationAPI() APIPublicationAPI {
	if k.SDK == nil {
		return nil
	}

	return &APIPublicationAPIImpl{
		SDK:         k.SDK,
		BaseURL:     k.BaseURL,
		Token:       k.Token,
		TokenSource: k.TokenSource,
		HTTPClient:  k.HTTPClient,
	}
}

// Returns the implementation of the APIImplementationAPI interface
func (k *KonnectSDK) GetAPIImplementationAPI() APIImplementationAPI {
	if k.SDK == nil {
		return nil
	}

	return &APIImplementationAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the AppAuthStrategiesAPI interface
func (k *KonnectSDK) GetAppAuthStrategiesAPI() AppAuthStrategiesAPI {
	if k.SDK == nil {
		return nil
	}

	return &AppAuthStrategiesAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the DCRProvidersAPI interface
func (k *KonnectSDK) GetDCRProvidersAPI() DCRProvidersAPI {
	if k.SDK == nil {
		return nil
	}

	return &DCRProvidersAPIImpl{
		SDK:         k.SDK,
		BaseURL:     k.BaseURL,
		Token:       k.Token,
		TokenSource: k.TokenSource,
		HTTPClient:  k.HTTPClient,
	}
}

// Returns the implementation of the GatewayServiceAPI interface
func (k *KonnectSDK) GetGatewayServiceAPI() GatewayServiceAPI {
	if k.SDK == nil {
		return nil
	}

	return k.SDK.Services
}

func (k *KonnectSDK) GetDataPlaneCertificateAPI() DataPlaneCertificateAPI {
	if k.SDK == nil {
		return nil
	}

	return &DataPlaneCertificateAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the PortalPageAPI interface
func (k *KonnectSDK) GetPortalPageAPI() PortalPageAPI {
	if k.SDK == nil {
		return nil
	}

	return &PortalPageAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the PortalAuthSettingsAPI interface
func (k *KonnectSDK) GetPortalAuthSettingsAPI() PortalAuthSettingsAPI {
	if k.SDK == nil {
		return nil
	}

	return &PortalAuthSettingsAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the PortalIPAllowListAPI interface
func (k *KonnectSDK) GetPortalIPAllowListAPI() PortalIPAllowListAPI {
	if k.SDK == nil || k.SDK.PortalsIPAllowList == nil {
		return nil
	}

	return &PortalIPAllowListAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the PortalIntegrationsAPI interface
func (k *KonnectSDK) GetPortalIntegrationsAPI() PortalIntegrationsAPI {
	if k.SDK == nil || k.SDK.PortalIntegrations == nil {
		return nil
	}

	return &PortalIntegrationsAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the PortalIdentityProviderAPI interface
func (k *KonnectSDK) GetPortalIdentityProviderAPI() PortalIdentityProviderAPI {
	if k.SDK == nil {
		return nil
	}

	return &PortalIdentityProviderAPIImpl{
		SDK:         k.SDK,
		BaseURL:     k.BaseURL,
		Token:       k.Token,
		TokenSource: k.TokenSource,
		HTTPClient:  k.HTTPClient,
	}
}

// Returns the implementation of the PortalCustomizationAPI interface
func (k *KonnectSDK) GetPortalCustomizationAPI() PortalCustomizationAPI {
	if k.SDK == nil {
		return nil
	}

	return &PortalCustomizationAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the PortalCustomDomainAPI interface
func (k *KonnectSDK) GetPortalCustomDomainAPI() PortalCustomDomainAPI {
	if k.SDK == nil {
		return nil
	}

	return &PortalCustomDomainAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the PortalSnippetAPI interface
func (k *KonnectSDK) GetPortalSnippetAPI() PortalSnippetAPI {
	if k.SDK == nil {
		return nil
	}

	return &PortalSnippetAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the PortalApplicationAPI interface
func (k *KonnectSDK) GetPortalApplicationAPI() PortalApplicationAPI {
	if k.SDK == nil {
		return nil
	}

	return &PortalApplicationAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the PortalApplicationRegistrationAPI interface
func (k *KonnectSDK) GetPortalApplicationRegistrationAPI() PortalApplicationRegistrationAPI {
	if k.SDK == nil {
		return nil
	}

	return &PortalApplicationRegistrationAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the PortalDeveloperAPI interface
func (k *KonnectSDK) GetPortalDeveloperAPI() PortalDeveloperAPI {
	if k.SDK == nil {
		return nil
	}

	return &PortalDeveloperAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the PortalTeamAPI interface
func (k *KonnectSDK) GetPortalTeamAPI() PortalTeamAPI {
	if k.SDK == nil {
		return nil
	}

	return &PortalTeamAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the PortalTeamRolesAPI interface
func (k *KonnectSDK) GetPortalTeamRolesAPI() PortalTeamRolesAPI {
	if k.SDK == nil {
		return nil
	}

	return &PortalTeamRolesAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the OrganizationTeamRolesAPI interface.
func (k *KonnectSDK) GetOrganizationTeamRolesAPI() OrganizationTeamRolesAPI {
	if k.SDK == nil {
		return nil
	}

	return &OrganizationTeamRolesAPIImpl{SDK: k.SDK}
}

// GetOrganizationUsersAPI returns the implementation of the OrganizationUsersAPI interface.
func (k *KonnectSDK) GetOrganizationUsersAPI() OrganizationUsersAPI {
	if k.SDK == nil || k.SDK.Users == nil {
		return nil
	}

	return &OrganizationUsersAPIImpl{SDK: k.SDK}
}

// GetOrganizationTeamMembershipAPI returns the implementation of the OrganizationTeamMembershipAPI interface.
func (k *KonnectSDK) GetOrganizationTeamMembershipAPI() OrganizationTeamMembershipAPI {
	if k.SDK == nil || k.SDK.TeamMembership == nil {
		return nil
	}

	return &OrganizationTeamMembershipAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the PortalTeamMembershipAPI interface
func (k *KonnectSDK) GetPortalTeamMembershipAPI() PortalTeamMembershipAPI {
	if k.SDK == nil {
		return nil
	}

	return &PortalTeamMembershipAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the MeAPI interface
func (k *KonnectSDK) GetMeAPI() MeAPI {
	if k.SDK == nil {
		return nil
	}

	return k.SDK.Me
}

func (k *KonnectSDK) GetPersonalAccessTokenAPI() PersonalAccessTokenAPI {
	if k.SDK == nil || k.SDK.PersonalAccessTokens == nil {
		return nil
	}

	return &PersonalAccessTokenAPIImpl{SDK: k.SDK}
}

func (k *KonnectSDK) GetSystemAccountAccessTokenAPI() SystemAccountAccessTokenAPI {
	if k.SDK == nil || k.SDK.SystemAccountsAccessTokens == nil {
		return nil
	}

	return &SystemAccountAccessTokenAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the AssetsAPI interface
func (k *KonnectSDK) GetAssetsAPI() AssetsAPI {
	if k.SDK == nil {
		return nil
	}

	return &AssetsAPIImpl{SDK: k.SDK}
}

// GetPortalEmailsAPI returns the implementation of the PortalEmailsAPI interface.
func (k *KonnectSDK) GetPortalEmailsAPI() PortalEmailsAPI {
	if k.SDK == nil || k.SDK.PortalEmails == nil {
		return nil
	}

	return &PortalEmailsAPIImpl{SDK: k.SDK}
}

// GetPortalAuditLogsAPI returns the implementation of the PortalAuditLogsAPI interface.
func (k *KonnectSDK) GetPortalAuditLogsAPI() PortalAuditLogsAPI {
	if k.SDK == nil || k.SDK.PortalAuditLogs == nil {
		return nil
	}

	return &PortalAuditLogsAPIImpl{SDK: k.SDK}
}

// GetAuditLogDestinationsAPI returns the implementation of the AuditLogDestinationsAPI interface.
func (k *KonnectSDK) GetAuditLogDestinationsAPI() AuditLogDestinationsAPI {
	return &AuditLogDestinationsAPIImpl{
		Token:      k.Token,
		HTTPClient: k.HTTPClient,
	}
}

// Returns the implementation of the EGWControlPlaneAPI interface
func (k *KonnectSDK) GetEventGatewayControlPlaneAPI() EGWControlPlaneAPI {
	if k.SDK == nil {
		return nil
	}

	return &EGWControlPlaneAPIImpl{SDK: k.SDK}
}

func (k *KonnectSDK) GetSystemAccountAPI() SystemAccountAPI {
	if k.SDK == nil || k.SDK.SystemAccounts == nil {
		return nil
	}

	return &SystemAccountAPIImpl{SDK: k.SDK}
}

func (k *KonnectSDK) GetSystemAccountRolesAPI() SystemAccountRolesAPI {
	if k.SDK == nil || k.SDK.SystemAccountsRoles == nil {
		return nil
	}

	return &SystemAccountRolesAPIImpl{SDK: k.SDK}
}

func (k *KonnectSDK) GetSystemAccountTeamMembershipAPI() SystemAccountTeamMembershipAPI {
	if k.SDK == nil || k.SDK.SystemAccountsTeamMembership == nil {
		return nil
	}

	return &SystemAccountTeamMembershipAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the EventGatewayBackendCluster interface
func (k *KonnectSDK) GetEventGatewayBackendClusterAPI() EventGatewayBackendClusterAPI {
	if k.SDK == nil {
		return nil
	}

	return &EventGatewayBackendClusterAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the EventGatewayVirtualCluster interface
func (k *KonnectSDK) GetEventGatewayVirtualClusterAPI() EventGatewayVirtualClusterAPI {
	if k.SDK == nil {
		return nil
	}

	return &EventGatewayVirtualClusterAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the EventGatewayListener interface
func (k *KonnectSDK) GetEventGatewayListenerAPI() EventGatewayListenerAPI {
	if k.SDK == nil {
		return nil
	}

	return &EventGatewayListenerAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the EventGatewayListenerPolicyAPI interface
func (k *KonnectSDK) GetEventGatewayListenerPolicyAPI() EventGatewayListenerPolicyAPI {
	if k.SDK == nil {
		return nil
	}

	return &EventGatewayListenerPolicyAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the EventGatewayDataPlaneCertificateAPI interface
func (k *KonnectSDK) GetEventGatewayDataPlaneCertificateAPI() EventGatewayDataPlaneCertificateAPI {
	if k.SDK == nil {
		return nil
	}

	return &EventGatewayDataPlaneCertificateAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the EventGatewayClusterPolicyAPI interface
func (k *KonnectSDK) GetEventGatewayClusterPolicyAPI() EventGatewayClusterPolicyAPI {
	if k.SDK == nil {
		return nil
	}

	return &EventGatewayClusterPolicyAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the EventGatewayProducePolicyAPI interface
func (k *KonnectSDK) GetEventGatewayProducePolicyAPI() EventGatewayProducePolicyAPI {
	if k.SDK == nil {
		return nil
	}

	return &EventGatewayProducePolicyAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the EventGatewayConsumePolicyAPI interface
func (k *KonnectSDK) GetEventGatewayConsumePolicyAPI() EventGatewayConsumePolicyAPI {
	if k.SDK == nil {
		return nil
	}

	return &EventGatewayConsumePolicyAPIImpl{SDK: k.SDK}
}

// GetEventGatewaySchemaRegistryAPI returns the implementation of the EventGatewaySchemaRegistryAPI interface.
func (k *KonnectSDK) GetEventGatewaySchemaRegistryAPI() EventGatewaySchemaRegistryAPI {
	if k.SDK == nil {
		return nil
	}

	return &EventGatewaySchemaRegistryAPIImpl{SDK: k.SDK}
}

// GetEventGatewayStaticKeyAPI returns the implementation of the EventGatewayStaticKeyAPI interface.
func (k *KonnectSDK) GetEventGatewayStaticKeyAPI() EventGatewayStaticKeyAPI {
	if k.SDK == nil {
		return nil
	}

	return &EventGatewayStaticKeyAPIImpl{SDK: k.SDK}
}

// GetEventGatewayTLSTrustBundleAPI returns the implementation of the EventGatewayTLSTrustBundleAPI interface.
func (k *KonnectSDK) GetEventGatewayTLSTrustBundleAPI() EventGatewayTLSTrustBundleAPI {
	if k.SDK == nil {
		return nil
	}

	return &EventGatewayTLSTrustBundleAPIImpl{SDK: k.SDK}
}

func (k *KonnectSDK) GetOrganizationTeamAPI() OrganizationTeamAPI {
	if k.SDK == nil || k.SDK.Teams == nil {
		return nil
	}

	return &OrganizationTeamAPIImpl{SDK: k.SDK}
}

// A function that can build an SDKAPI with a given configuration
type SDKAPIFactory func(cfg config.Hook, logger *slog.Logger) (SDKAPI, error)

// DefaultSDKFactory can be overridden for testing purposes
var DefaultSDKFactory SDKAPIFactory

type Key struct{}

// A Key used to store the SDKFactory in a Context
var SDKAPIFactoryKey = Key{}
