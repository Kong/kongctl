package helpers

import "testing"

// This is a mock implementation of the SDKAPI interface
type MockKonnectSDK struct {
	IdentityDirectoryDependenciesAPIFactory func() IdentityDirectoryDependenciesAPI
	IdentityPasswordWritesAPIFactory        func() IdentityPasswordWritesAPI
	IdentityTrustedIDPsAPIFactory           func() IdentityTrustedIDPsAPI
	IdentityBasicAuthFactory                func() IdentityBasicAuthAPI
	IdentityGroupsAPIFactory                func() IdentityGroupsAPI
	IdentityGroupsFactory                   func() IdentityGroupsReader
	IdentityGroupMembershipsFactory         func() IdentityGroupMembershipsReader
	IdentityPrincipalKeysFactory            func() IdentityPrincipalKeysReader
	IdentityPrincipalIdentitiesFactory      func() IdentityPrincipalIdentitiesAPI
	IdentityAuthServerClientsAPIFactory     func() IdentityAuthServerClientsAPI
	IdentityAuthServerClientsFactory        func() IdentityAuthServerClientsReader
	IdentityAuthServerScopesFactory         func() IdentityAuthServerScopesAPI
	IdentityAuthServerClaimsFactory         func() IdentityAuthServerClaimsAPI
	IdentityAuthServersFactory              func() IdentityAuthServersAPI
	IdentityPrincipalsFactory               func() IdentityPrincipalsAPI
	IdentityDirectoriesFactory              func() IdentityDirectoriesAPI
	Token                                   string
	T                                       *testing.T
	CPAPIFactory                            func() ControlPlaneAPI
	ControlPlaneGroupsFactory               func() ControlPlaneGroupsAPI
	PortalFactory                           func() PortalAPI
	APIFactory                              func() APIFullAPI
	AIGatewayFactory                        func() AIGatewayAPI
	AIGatewayProvidersFactory               func() AIGatewayProvidersAPI
	AIGatewayAuthStrategiesFactory          func() AIGatewayAuthStrategiesAPI
	AIGatewayCustomPoliciesFactory          func() AIGatewayCustomPoliciesAPI
	AIGatewayPoliciesFactory                func() AIGatewayPoliciesAPI
	AIGatewayAgentsFactory                  func() AIGatewayAgentsAPI
	AIGatewayConsumersFactory               func() AIGatewayConsumersAPI
	AIGatewayConsumerGroupsFactory          func() AIGatewayConsumerGroupsAPI
	AIGatewayModelFactory                   func() AIGatewayModelAPI
	AIGatewayMCPServersFactory              func() AIGatewayMCPServersAPI
	AIGatewayConfigStoresFactory            func() AIGatewayConfigStoresAPI
	AIGatewayVaultsFactory                  func() AIGatewayVaultsAPI
	AIGatewayDatastoresFactory              func() AIGatewayDatastoresAPI
	AIGatewayNodesFactory                   func() AIGatewayNodesAPI
	AIGatewayDataPlaneCertificatesFactory   func() AIGatewayDataPlaneCertificatesAPI
	AIGatewayCertificatesFactory            func() AIGatewayCertificatesAPI
	AIGatewayCACertificatesFactory          func() AIGatewayCACertificatesAPI
	AIGatewaySNIsFactory                    func() AIGatewaySNIsAPI
	CatalogServicesFactory                  func() CatalogServicesAPI
	DashboardsFactory                       func() DashboardsAPI
	APIDocumentFactory                      func() APIDocumentAPI
	APIVersionFactory                       func() APIVersionAPI
	APIPublicationFactory                   func() APIPublicationAPI
	APIImplementationFactory                func() APIImplementationAPI
	AppAuthStrategiesFactory                func() AppAuthStrategiesAPI
	DCRProvidersFactory                     func() DCRProvidersAPI
	MeFactory                               func() MeAPI
	PersonalAccessTokenFactory              func() PersonalAccessTokenAPI
	SystemAccountAccessTokenFactory         func() SystemAccountAccessTokenAPI
	GatewayServiceFactory                   func() GatewayServiceAPI
	DataPlaneCertificateFactory             func() DataPlaneCertificateAPI
	SystemAccountFactory                    func() SystemAccountAPI
	SystemAccountRolesFactory               func() SystemAccountRolesAPI
	SystemAccountTeamMembershipFactory      func() SystemAccountTeamMembershipAPI
	OrganizationTeamFactory                 func() OrganizationTeamAPI
	OrganizationTeamRolesFactory            func() OrganizationTeamRolesAPI
	OrganizationUsersFactory                func() OrganizationUsersAPI
	OrganizationMembershipFactory           func() OrganizationTeamMembershipAPI
