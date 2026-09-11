package loader

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/declarative/validator"
	"github.com/kong/kongctl/internal/util"
)

// validateResourceSet validates all resources and checks for ref uniqueness
func (l *Loader) validateResourceSet(rs *resources.ResourceSet) error {
	normalizeOrganizationTeamSelectors(rs)

	// Validate portals
	if err := l.validatePortals(rs.Portals, rs); err != nil {
		return err
	}

	// Validate auth strategies
	if err := l.validateAuthStrategies(rs.ApplicationAuthStrategies, rs); err != nil {
		return err
	}

	// Validate DCR providers
	if err := l.validateDCRProviders(rs.DCRProviders, rs); err != nil {
		return err
	}

	// Validate control planes
	if err := l.validateControlPlanes(rs.ControlPlanes, rs); err != nil {
		return err
	}

	// Validate catalog services
	if err := l.validateCatalogServices(rs.CatalogServices, rs); err != nil {
		return err
	}

	// Validate AI Gateways
	if err := l.validateAIGateways(rs.AIGateways, rs); err != nil {
		return err
	}

	if err := rs.ValidateRegisteredChildren(resources.ResourceTypeAIGateway); err != nil {
		return err
	}

	// Validate dashboards
	if err := l.validateDashboards(rs.Dashboards, rs); err != nil {
		return err
	}

	// Validate gateway services
	if err := l.validateGatewayServices(rs.GatewayServices, rs); err != nil {
		return err
	}

	// Validate audit-log webhook destinations
	if rs.AuditLogs != nil {
		if err := l.validateAuditLogWebhookDestinations(rs.AuditLogs.Destinations, rs); err != nil {
			return err
		}
	}

	// Validate control plane data plane certificates
	if err := l.validateControlPlaneDataPlaneCertificates(rs.ControlPlaneDataPlaneCertificates, rs); err != nil {
		return err
	}

	// Validate APIs and their children
	if err := l.validateAPIs(rs.APIs, rs); err != nil {
		return err
	}

	// Validate API child resources in root storage.
	if err := rs.ValidateRegisteredChildren(resources.ResourceTypeAPI); err != nil {
		return err
	}

	if err := rs.ValidateRegisteredChildren(resources.ResourceTypePortal); err != nil {
		return err
	}

	// Validate organization teams
	if err := l.validateOrganizationTeams(rs.OrganizationTeams, rs); err != nil {
		return err
	}

	if err := l.validateOrganizationTeamRoles(rs.OrganizationTeamRoles, rs); err != nil {
		return err
	}

	if err := l.validateOrganizationUsers(rs); err != nil {
		return err
	}

	if err := l.validateOrganizationSystemAccounts(rs); err != nil {
		return err
	}

	// Validate cross-resource references
	if err := l.validateCrossReferences(rs); err != nil {
		return err
	}

	// Validate namespaces
	if err := l.validateNamespaces(rs); err != nil {
		return err
	}

	return nil
}

func normalizeOrganizationTeamSelectors(rs *resources.ResourceSet) {
	if rs == nil {
		return
	}
	for i := range rs.OrganizationUserTeamMemberships {
		rs.OrganizationUserTeamMemberships[i].Team = resources.NormalizeResourceRef(
			rs.OrganizationUserTeamMemberships[i].Team,
		)
	}
	for i := range rs.OrganizationSystemAccountTeamMemberships {
		rs.OrganizationSystemAccountTeamMemberships[i].Team = resources.NormalizeResourceRef(
			rs.OrganizationSystemAccountTeamMemberships[i].Team,
		)
	}
	for i := range rs.OrganizationTeamRoles {
		role := &rs.OrganizationTeamRoles[i]
		original := role.Team
		role.Team = resources.NormalizeResourceRef(role.Team)
		if rs.SyncScope != nil {
			rs.SyncScope.RebindChildParent(resources.ResourceTypeOrganizationTeam, original, role.Team)
		}
	}
}

func (l *Loader) validateOrganizationUsers(rs *resources.ResourceSet) error {
	if rs.Organization == nil {
		if len(rs.OrganizationUserTeamMemberships) > 0 || len(rs.OrganizationUserRoles) > 0 {
			return fmt.Errorf("organization.users must define selectors for root-level user assignments")
		}
		return nil
	}

	users := rs.Organization.Users
	userRefs := make(map[string]bool)
	assignmentRefs := make(map[string]bool)
	for i := range users {
		user := &users[i]
		if err := user.Validate(); err != nil {
			return fmt.Errorf("invalid organization user %q: %w", user.Ref, err)
		}
		if existing, found := rs.GetResourceByRef(user.Ref); found {
			return fmt.Errorf("duplicate ref '%s' (already defined as %s)", user.Ref, existing.GetType())
		}
		if userRefs[user.Ref] {
			return fmt.Errorf("duplicate organization user ref: %s", user.Ref)
		}
		userRefs[user.Ref] = true
	}

	for i := range rs.OrganizationUserTeamMemberships {
		membership := &rs.OrganizationUserTeamMemberships[i]
		if err := membership.Validate(); err != nil {
			return fmt.Errorf("invalid organization_user_team_membership %q: %w", membership.GetRef(), err)
		}
		if assignmentRefs[membership.GetRef()] {
			return fmt.Errorf("duplicate organization user team membership: %s", membership.GetRef())
		}
		assignmentRefs[membership.GetRef()] = true
		if !userRefs[membership.User] {
			return fmt.Errorf(
				"organization_user_team_membership %q references unknown organization user: %s",
				membership.GetRef(),
				membership.User,
			)
		}
		if tags.IsExternalPlaceholder(membership.Team) {
			continue
		}
		if resource, found := rs.GetResourceByRef(membership.Team); !found {
			return fmt.Errorf("organization_user_team_membership %q references unknown organization_team: %s",
				membership.GetRef(), membership.Team)
		} else if resource.GetType() != resources.ResourceTypeOrganizationTeam {
			return fmt.Errorf("organization_user_team_membership %q references %s but expected organization_team: %s",
				membership.GetRef(), resource.GetType(), membership.Team)
		}
	}

	roleRefs := make(map[string]bool)
	for i := range rs.OrganizationUserRoles {
		role := &rs.OrganizationUserRoles[i]
		if err := role.Validate(); err != nil {
			return fmt.Errorf("invalid organization_user_role %q: %w", role.GetRef(), err)
		}
		if roleRefs[role.GetRef()] {
			return fmt.Errorf("duplicate organization_user_role ref: %s", role.GetRef())
		}
		roleRefs[role.GetRef()] = true
		if !userRefs[role.User] {
			return fmt.Errorf(
				"organization_user_role %q references unknown organization user: %s",
				role.GetRef(),
				role.User,
			)
		}
		if err := l.validateUserRoleEntityReference(role, rs); err != nil {
			return err
		}
	}

	return nil
}

func (l *Loader) validateUserRoleEntityReference(
	role *resources.OrganizationUserRoleResource,
	rs *resources.ResourceSet,
) error {
	return validateRoleEntityReference(
		resources.ResourceTypeOrganizationUserRole,
		role.GetRef(),
		role.EntityID,
		role.EntityTypeName,
		rs,
	)
}

func (l *Loader) validateOrganizationSystemAccounts(rs *resources.ResourceSet) error {
	if rs.Organization == nil {
		if len(rs.OrganizationSystemAccountTeamMemberships) > 0 || len(rs.OrganizationSystemAccountRoles) > 0 {
			return fmt.Errorf(
				"organization.system-accounts must define selectors for root-level system account assignments",
			)
		}
		return nil
	}

	systemAccounts := rs.Organization.SystemAccounts
	userRefs := make(map[string]bool, len(rs.Organization.Users))
	for _, user := range rs.Organization.Users {
		userRefs[user.Ref] = true
	}

	systemAccountRefs := make(map[string]bool)
	assignmentRefs := make(map[string]bool)
	for i := range systemAccounts {
		systemAccount := &systemAccounts[i]
		if err := systemAccount.Validate(); err != nil {
			return fmt.Errorf("invalid organization system account %q: %w", systemAccount.Ref, err)
		}
		if existing, found := rs.GetResourceByRef(systemAccount.Ref); found {
			return fmt.Errorf("duplicate ref '%s' (already defined as %s)", systemAccount.Ref, existing.GetType())
		}
		if userRefs[systemAccount.Ref] {
			return fmt.Errorf(
				"duplicate ref '%s' (already defined as %s)",
				systemAccount.Ref,
				resources.ResourceTypeOrganizationUser,
			)
		}
		if systemAccountRefs[systemAccount.Ref] {
			return fmt.Errorf("duplicate organization system account ref: %s", systemAccount.Ref)
		}
		systemAccountRefs[systemAccount.Ref] = true
	}

	for i := range rs.OrganizationSystemAccountTeamMemberships {
		membership := &rs.OrganizationSystemAccountTeamMemberships[i]
		if err := membership.Validate(); err != nil {
			return fmt.Errorf("invalid organization_system_account_team_membership %q: %w", membership.GetRef(), err)
		}
		if assignmentRefs[membership.GetRef()] {
			return fmt.Errorf("duplicate organization system account team membership: %s", membership.GetRef())
		}
		assignmentRefs[membership.GetRef()] = true
		if !systemAccountRefs[membership.SystemAccount] {
			return fmt.Errorf(
				"organization_system_account_team_membership %q references unknown organization system account: %s",
				membership.GetRef(),
				membership.SystemAccount,
			)
		}
		if tags.IsExternalPlaceholder(membership.Team) {
			continue
		}
		if resource, found := rs.GetResourceByRef(membership.Team); !found {
			return fmt.Errorf("organization_system_account_team_membership %q references unknown organization_team: %s",
				membership.GetRef(), membership.Team)
		} else if resource.GetType() != resources.ResourceTypeOrganizationTeam {
			return fmt.Errorf(
				"organization_system_account_team_membership %q references %s but expected organization_team: %s",
				membership.GetRef(),
				resource.GetType(),
				membership.Team,
			)
		}
	}

	roleRefs := make(map[string]bool)
	for i := range rs.OrganizationSystemAccountRoles {
		role := &rs.OrganizationSystemAccountRoles[i]
		if err := role.Validate(); err != nil {
			return fmt.Errorf("invalid organization_system_account_role %q: %w", role.GetRef(), err)
		}
		if roleRefs[role.GetRef()] {
			return fmt.Errorf("duplicate organization_system_account_role ref: %s", role.GetRef())
		}
		roleRefs[role.GetRef()] = true
		if !systemAccountRefs[role.SystemAccount] {
			return fmt.Errorf(
				"organization_system_account_role %q references unknown organization system account: %s",
				role.GetRef(),
				role.SystemAccount,
			)
		}
		if err := l.validateSystemAccountRoleEntityReference(role, rs); err != nil {
			return err
		}
	}

	return nil
}

func (l *Loader) validateSystemAccountRoleEntityReference(
	role *resources.OrganizationSystemAccountRoleResource,
	rs *resources.ResourceSet,
) error {
	return validateRoleEntityReference(
		resources.ResourceTypeOrganizationSystemAccountRole,
		role.GetRef(),
		role.EntityID,
		role.EntityTypeName,
		rs,
	)
}

func (l *Loader) validateOrganizationTeamRoles(roles []resources.OrganizationTeamRoleResource,
	rs *resources.ResourceSet,
) error {
	roleRefs := make(map[string]bool)

	for i := range roles {
		role := &roles[i]
		if err := role.Validate(); err != nil {
			return fmt.Errorf("invalid organization_team_role %q: %w", role.GetRef(), err)
		}

		if existing, found := rs.GetResourceByRef(role.GetRef()); found {
			if existing.GetType() != resources.ResourceTypeOrganizationTeamRole {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					role.GetRef(), existing.GetType())
			}
		}

		if roleRefs[role.GetRef()] {
			return fmt.Errorf("duplicate organization_team_role ref: %s", role.GetRef())
		}
		roleRefs[role.GetRef()] = true

		if err := l.validateOrganizationTeamRoleReferences(role, rs); err != nil {
			return err
		}
	}

	return nil
}

func (l *Loader) validateOrganizationTeamRoleReferences(
	role *resources.OrganizationTeamRoleResource,
	rs *resources.ResourceSet,
) error {
	if !tags.IsExternalPlaceholder(role.Team) {
		if resource, found := rs.GetResourceByRef(role.Team); !found {
			return fmt.Errorf("organization_team_role %q references unknown organization_team: %s",
				role.GetRef(), role.Team)
		} else if resource.GetType() != resources.ResourceTypeOrganizationTeam {
			return fmt.Errorf("organization_team_role %q references %s but expected organization_team: %s",
				role.GetRef(), resource.GetType(), role.Team)
		}
	}

	if !tags.IsRefPlaceholder(role.EntityID) {
		return nil
	}

	return validateRoleEntityReference(
		resources.ResourceTypeOrganizationTeamRole,
		role.GetRef(),
		role.EntityID,
		role.EntityTypeName,
		rs,
	)
}

func validateRoleEntityReference(
	roleType resources.ResourceType,
	roleRef string,
	entityID string,
	entityTypeName string,
	rs *resources.ResourceSet,
) error {
	if !tags.IsRefPlaceholder(entityID) {
		return nil
	}

	entityRef, _, ok := tags.ParseRefPlaceholder(entityID)
	if !ok || entityRef == "" {
		return fmt.Errorf("%s %q has invalid entity_id reference: %s", roleType, roleRef, entityID)
	}

	expectedType, ok := resources.RoleEntityResourceType(entityTypeName)
	if !ok {
		return fmt.Errorf(
			"%s %q has unsupported entity_type_name for entity_id reference: %s",
			roleType,
			roleRef,
			entityTypeName,
		)
	}

	if resource, found := rs.GetResourceByRef(entityRef); !found {
		return fmt.Errorf("%s %q references unknown %s: %s (field: entity_id)",
			roleType, roleRef, expectedType, entityRef)
	} else if resource.GetType() != expectedType {
		return fmt.Errorf("%s %q references %s but expected %s: %s (field: entity_id)",
			roleType, roleRef, resource.GetType(), expectedType, entityRef)
	}

	return nil
}

// validateOrganizationTeams validates organization team resources
func (l *Loader) validateOrganizationTeams(teams []resources.OrganizationTeamResource,
	rs *resources.ResourceSet,
) error {
	names := make(map[string]string) // name -> ref mapping (names unique per type)

	for i := range teams {
		team := &teams[i]

		// Validate resource
		if err := team.Validate(); err != nil {
			return fmt.Errorf("invalid organization_team %q: %w", team.GetRef(), err)
		}

		// Check global ref uniqueness across different resource types
		if existing, found := rs.GetResourceByRef(team.GetRef()); found {
			if existing.GetType() != resources.ResourceTypeOrganizationTeam {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					team.GetRef(), existing.GetType())
			}
		}

		// Check name uniqueness (within team type only)
		if existingRef, exists := names[team.Name]; exists {
			return fmt.Errorf("duplicate organization_team name '%s' (ref: %s conflicts with ref: %s)",
				team.Name, team.GetRef(), existingRef)
		}

		names[team.Name] = team.GetRef()
	}

	return nil
}

// validateGatewayServices validates gateway service resources
func (l *Loader) validateGatewayServices(
	services []resources.GatewayServiceResource,
	rs *resources.ResourceSet,
) error {
	for i := range services {
		service := &services[i]

		if err := service.Validate(); err != nil {
			return fmt.Errorf("invalid gateway_service %q: %w", service.GetRef(), err)
		}

		if existing, found := rs.GetResourceByRef(service.GetRef()); found {
			if existing.GetType() != resources.ResourceTypeGatewayService {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					service.GetRef(), existing.GetType())
			}
		}
	}

	return nil
}

func (l *Loader) validateAuditLogWebhookDestinations(
	destinations []resources.AuditLogWebhookDestinationResource,
	rs *resources.ResourceSet,
) error {
	refs := make(map[string]bool)

	for i := range destinations {
		destination := &destinations[i]

		if err := destination.Validate(); err != nil {
			return fmt.Errorf("invalid audit_log_webhook_destination %q: %w", destination.GetRef(), err)
		}

		if refs[destination.GetRef()] {
			return fmt.Errorf(
				"duplicate ref '%s' (already defined as audit_log_webhook_destination)",
				destination.GetRef(),
			)
		}
		refs[destination.GetRef()] = true

		if existing, found := rs.GetResourceByRef(destination.GetRef()); found {
			if existing.GetType() != resources.ResourceTypeAuditLogWebhookDestination {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					destination.GetRef(), existing.GetType())
			}
		}
	}

	return nil
}

func (l *Loader) validateControlPlaneDataPlaneCertificates(
	certs []resources.ControlPlaneDataPlaneCertificateResource,
	rs *resources.ResourceSet,
) error {
	identitiesByControlPlane := make(map[string]map[string]string)

	for i := range certs {
		cert := &certs[i]

		if err := cert.Validate(); err != nil {
			return fmt.Errorf("invalid control_plane_data_plane_certificate %q: %w", cert.GetRef(), err)
		}

		if existing, found := rs.GetResourceByRef(cert.GetRef()); found {
			if existing.GetType() != resources.ResourceTypeControlPlaneDataPlaneCertificate {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					cert.GetRef(), existing.GetType())
			}
		}

		if tags.IsExternalPlaceholder(cert.ControlPlane) {
			continue
		}

		if !rs.HasRef(cert.ControlPlane) {
			return fmt.Errorf(
				"control_plane_data_plane_certificate %q references unknown control_plane: %s",
				cert.GetRef(),
				cert.ControlPlane,
			)
		}

		if actualType, _ := rs.GetResourceTypeByRef(cert.ControlPlane); actualType != resources.ResourceTypeControlPlane {
			return fmt.Errorf(
				"control_plane_data_plane_certificate %q references %s but expected control_plane: %s",
				cert.GetRef(),
				actualType,
				cert.ControlPlane,
			)
		}

		identity := resources.ControlPlaneDataPlaneCertificateIdentity(cert.Cert)
		if identitiesByControlPlane[cert.ControlPlane] == nil {
			identitiesByControlPlane[cert.ControlPlane] = make(map[string]string)
		}
		if existingRef, exists := identitiesByControlPlane[cert.ControlPlane][identity]; exists {
			return fmt.Errorf(
				"duplicate data plane certificate for control_plane %q (ref: %s conflicts with ref: %s)",
				cert.ControlPlane,
				cert.GetRef(),
				existingRef,
			)
		}
		identitiesByControlPlane[cert.ControlPlane][identity] = cert.GetRef()
	}

	return nil
}

// validatePortals validates portal resources
func (l *Loader) validatePortals(portals []resources.PortalResource, rs *resources.ResourceSet) error {
	names := make(map[string]string) // name -> ref mapping (names unique per type)

	for i := range portals {
		portal := &portals[i]

		// Validate resource
		if err := portal.Validate(); err != nil {
			return fmt.Errorf("invalid portal %q: %w", portal.GetRef(), err)
		}

		// Check global ref uniqueness across different resource types
		// Don't check within same type - that's handled by the loader during append
		if existing, found := rs.GetResourceByRef(portal.GetRef()); found {
			if existing.GetType() != resources.ResourceTypePortal {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					portal.GetRef(), existing.GetType())
			}
		}

		// Check name uniqueness (within portal type only)
		if existingRef, exists := names[portal.Name]; exists {
			return fmt.Errorf("duplicate portal name '%s' (ref: %s conflicts with ref: %s)",
				portal.Name, portal.GetRef(), existingRef)
		}

		names[portal.Name] = portal.GetRef()
	}

	return nil
}

// validateAuthStrategies validates auth strategy resources
func (l *Loader) validateAuthStrategies(
	strategies []resources.ApplicationAuthStrategyResource,
	rs *resources.ResourceSet,
) error {
	names := make(map[string]string) // name -> ref mapping (names unique per type)

	for i := range strategies {
		strategy := &strategies[i]

		// Validate resource
		if err := strategy.Validate(); err != nil {
			return fmt.Errorf("invalid application_auth_strategy %q: %w", strategy.GetRef(), err)
		}

		// Check global ref uniqueness across different resource types
		// Don't check within same type - that's handled by the loader during append
		if existing, found := rs.GetResourceByRef(strategy.GetRef()); found {
			if existing.GetType() != resources.ResourceTypeApplicationAuthStrategy {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					strategy.GetRef(), existing.GetType())
			}
		}

		// Check name uniqueness (within auth strategy type only)
		stratName := strategy.GetMoniker()
		if existingRef, exists := names[stratName]; exists {
			return fmt.Errorf("duplicate application_auth_strategy name '%s' (ref: %s conflicts with ref: %s)",
				stratName, strategy.GetRef(), existingRef)
		}

		names[stratName] = strategy.GetRef()
	}

	return nil
}

// validateDCRProviders validates DCR provider resources
func (l *Loader) validateDCRProviders(
	providers []resources.DCRProviderResource,
	rs *resources.ResourceSet,
) error {
	names := make(map[string]string) // name -> ref mapping (names unique per type)

	for i := range providers {
		provider := &providers[i]

		// Validate resource
		if err := provider.Validate(); err != nil {
			return fmt.Errorf("invalid dcr_provider %q: %w", provider.GetRef(), err)
		}

		// Check global ref uniqueness across different resource types
		// Don't check within same type - that's handled by the loader during append
		if existing, found := rs.GetResourceByRef(provider.GetRef()); found {
			if existing.GetType() != resources.ResourceTypeDCRProvider {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					provider.GetRef(), existing.GetType())
			}
		}

		providerName := provider.GetMoniker()
		if existingRef, exists := names[providerName]; exists {
			return fmt.Errorf("duplicate dcr_provider name '%s' (ref: %s conflicts with ref: %s)",
				providerName, provider.GetRef(), existingRef)
		}

		names[providerName] = provider.GetRef()
	}

	return nil
}

// validateControlPlanes validates control plane resources
func (l *Loader) validateControlPlanes(
	cps []resources.ControlPlaneResource,
	rs *resources.ResourceSet,
) error {
	names := make(map[string]string) // name -> ref mapping (names unique per type)

	for i := range cps {
		cp := &cps[i]

		// Validate resource
		if err := cp.Validate(); err != nil {
			return fmt.Errorf("invalid control_plane %q: %w", cp.GetRef(), err)
		}

		// Check global ref uniqueness across different resource types
		// Don't check within same type - that's handled by the loader during append
		if existing, found := rs.GetResourceByRef(cp.GetRef()); found {
			if existing.GetType() != resources.ResourceTypeControlPlane {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					cp.GetRef(), existing.GetType())
			}
		}

		// Check name uniqueness (within control plane type only)
		if existingRef, exists := names[cp.Name]; exists {
			return fmt.Errorf("duplicate control_plane name '%s' (ref: %s conflicts with ref: %s)",
				cp.Name, cp.GetRef(), existingRef)
		}

		names[cp.Name] = cp.GetRef()
	}

	return nil
}

// validateCatalogServices validates catalog service resources
func (l *Loader) validateCatalogServices(
	services []resources.CatalogServiceResource,
	rs *resources.ResourceSet,
) error {
	names := make(map[string]string)

	for i := range services {
		service := &services[i]

		if err := service.Validate(); err != nil {
			return fmt.Errorf("invalid catalog_service %q: %w", service.GetRef(), err)
		}

		if existing, found := rs.GetResourceByRef(service.GetRef()); found {
			if existing.GetType() != resources.ResourceTypeCatalogService {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					service.GetRef(), existing.GetType())
			}
		}

		if existingRef, exists := names[service.GetMoniker()]; exists {
			return fmt.Errorf("duplicate catalog_service name '%s' (ref: %s conflicts with ref: %s)",
				service.GetMoniker(), service.GetRef(), existingRef)
		}

		names[service.GetMoniker()] = service.GetRef()
	}

	return nil
}

// validateAIGateways validates AI Gateway resources.
func (l *Loader) validateAIGateways(
	gateways []resources.AIGatewayResource,
	rs *resources.ResourceSet,
) error {
	namesByNamespace := make(map[string]string)

	for i := range gateways {
		gateway := &gateways[i]

		if err := gateway.Validate(); err != nil {
			return fmt.Errorf("invalid ai_gateway %q: %w", gateway.GetRef(), err)
		}

		if existing, found := rs.GetResourceByRef(gateway.GetRef()); found {
			if existing.GetType() != resources.ResourceTypeAIGateway {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					gateway.GetRef(), existing.GetType())
			}
		}

		if gateway.IsExternal() {
			continue
		}
		namespace := resources.GetNamespace(gateway.Kongctl)
		nameKey := namespace + "\x00" + gateway.Name
		if existingRef, exists := namesByNamespace[nameKey]; exists {
			return fmt.Errorf(
				"duplicate ai_gateway name '%s' in namespace '%s' (ref: %s conflicts with ref: %s)",
				gateway.Name,
				namespace,
				gateway.GetRef(),
				existingRef,
			)
		}
		namesByNamespace[nameKey] = gateway.GetRef()
	}

	return nil
}

// Retain the existing test-facing entry point while dispatch uses registration.
func (l *Loader) validateAIGatewayConfigStoreSecrets(rs *resources.ResourceSet) error {
	return rs.ValidateRegisteredResource(resources.ResourceTypeAIGatewayConfigStoreSecret)
}

// validateDashboards validates dashboard resources.
func (l *Loader) validateDashboards(
	dashboards []resources.DashboardResource,
	rs *resources.ResourceSet,
) error {
	refs := make(map[string]struct{})
	namesByNamespace := make(map[string]string)

	for i := range dashboards {
		dashboard := &dashboards[i]

		if err := dashboard.Validate(); err != nil {
			return fmt.Errorf("invalid dashboard %q: %w", dashboard.GetRef(), err)
		}

		if existing, found := rs.GetResourceByRef(dashboard.GetRef()); found {
			if existing.GetType() != resources.ResourceTypeDashboard {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					dashboard.GetRef(), existing.GetType())
			}
		}

		if _, exists := refs[dashboard.GetRef()]; exists {
			return fmt.Errorf("duplicate dashboard ref '%s'", dashboard.GetRef())
		}
		refs[dashboard.GetRef()] = struct{}{}

		namespace := resources.GetNamespace(dashboard.Kongctl)
		nameKey := namespace + "\x00" + dashboard.Name
		if existingRef, exists := namesByNamespace[nameKey]; exists {
			return fmt.Errorf(
				"duplicate dashboard name '%s' in namespace '%s' (ref: %s conflicts with ref: %s)",
				dashboard.Name,
				namespace,
				dashboard.GetRef(),
				existingRef,
			)
		}
		namesByNamespace[nameKey] = dashboard.GetRef()
	}

	return nil
}

// validateAPIs validates API resources and their children
func (l *Loader) validateAPIs(apis []resources.APIResource, rs *resources.ResourceSet) error {
	apiNames := make(map[string]string) // name -> ref mapping (names unique per type)

	for i := range apis {
		api := &apis[i]

		// Validate API resource
		if err := api.Validate(); err != nil {
			return fmt.Errorf("invalid api %q: %w", api.GetRef(), err)
		}

		// Check global ref uniqueness across different resource types
		// Don't check within same type - that's handled by the loader during append
		if existing, found := rs.GetResourceByRef(api.GetRef()); found {
			if existing.GetType() != resources.ResourceTypeAPI {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					api.GetRef(), existing.GetType())
			}
		}

		// Check API name uniqueness (within API type only)
		if existingRef, exists := apiNames[api.Name]; exists {
			return fmt.Errorf("duplicate api name '%s' (ref: %s conflicts with ref: %s)",
				api.Name, api.GetRef(), existingRef)
		}

		apiNames[api.Name] = api.GetRef()

		if err := rs.ValidateRegisteredNestedChildren(api); err != nil {
			return err
		}
	}

	return nil
}

// validateCrossReferences validates that all cross-resource references are valid
func (l *Loader) validateCrossReferences(rs *resources.ResourceSet) error {
	// Validate portal references
	for i := range rs.Portals {
		if err := l.validateResourceReferences(&rs.Portals[i], rs); err != nil {
			return err
		}
	}

	// Validate API child resource references
	for i := range rs.APIs {
		api := &rs.APIs[i]
		// Validate publication references
		for j := range api.Publications {
			if err := l.validateResourceReferences(&api.Publications[j], rs); err != nil {
				return err
			}
		}

		// Validate implementation references
		for j := range api.Implementations {
			if err := l.validateResourceReferences(&api.Implementations[j], rs); err != nil {
				return err
			}
		}
	}

	// Validate separate API child resources (extracted from nested resources)
	for i := range rs.APIPublications {
		if err := l.validateResourceReferences(&rs.APIPublications[i], rs); err != nil {
			return err
		}
	}

	for i := range rs.APIImplementations {
		if err := l.validateResourceReferences(&rs.APIImplementations[i], rs); err != nil {
			return err
		}
	}

	for i := range rs.APIDocuments {
		if err := l.validateResourceReferences(&rs.APIDocuments[i], rs); err != nil {
			return err
		}
	}

	for i := range rs.PortalIPAllowLists {
		if err := l.validateResourceReferences(&rs.PortalIPAllowLists[i], rs); err != nil {
			return err
		}
	}

	for i := range rs.PortalAuditLogWebhooks {
		if err := l.validateResourceReferences(&rs.PortalAuditLogWebhooks[i], rs); err != nil {
			return err
		}
	}

	// Note: API versions don't have outbound references, so no validation needed

	return nil
}

// validateResourceReferences validates references for a single resource using its mapping
func (l *Loader) validateResourceReferences(resource any, rs *resources.ResourceSet) error {
	// fmt.Printf("DEBUG: validateResourceReferences called with resource type: %T\n", resource)

	// Check if resource implements ReferenceMapping
	refMapper, ok := resource.(resources.ReferenceMapping)
	if !ok {
		return nil // Resource doesn't have reference fields
	}

	// Get the resource's ref for error messages
	refResource, ok := resource.(resources.ReferencedResource)
	if !ok {
		return nil // Shouldn't happen, but be safe
	}

	mappings := refMapper.GetReferenceFieldMappings()
	// fmt.Printf("DEBUG: Reference mappings: %v\n", mappings)

	for fieldPath, expectedType := range mappings {
		fieldValue := l.getFieldValue(resource, fieldPath)
		// fmt.Printf("DEBUG: Field %s = '%s' (expected type: %s)\n", fieldPath, fieldValue, expectedType)

		if fieldValue == "" {
			continue // Empty references are allowed (optional fields)
		}

		// Special handling for array fields (e.g., auth_strategy_ids)
		if strings.HasSuffix(fieldPath, "_ids") {
			// For now, skip array validation - would need reflection to handle properly
			continue
		}

		// Skip validation for unresolved reference placeholders
		if strings.HasPrefix(fieldValue, tags.RefPlaceholderPrefix) {
			// This will be resolved during planning/execution phase
			continue
		}
		if tags.IsExternalPlaceholder(fieldValue) {
			if _, ok := tags.ParseExternalPlaceholder(fieldValue); !ok {
				return fmt.Errorf(
					"resource %q has invalid external lookup placeholder (field: %s)",
					refResource.GetRef(),
					fieldPath,
				)
			}
			// External lookups are validated and resolved by the planner once the
			// target type and any parent scope are available.
			continue
		}

		// Skip validation for raw UUIDs — these are already-resolved Konnect
		// resource IDs that cannot be matched against local refs.
		if util.IsValidUUID(fieldValue) {
			continue
		}

		// Check if the referenced resource exists using RefReader
		if !rs.HasRef(fieldValue) {
			return fmt.Errorf("resource %q references unknown %s: %s (field: %s)",
				refResource.GetRef(), expectedType, fieldValue, fieldPath)
		}

		// Verify the referenced resource is of the expected type
		if actualType, _ := rs.GetResourceTypeByRef(fieldValue); string(actualType) != expectedType {
			return fmt.Errorf("resource %q references %s but expected %s: %s (field: %s)",
				refResource.GetRef(), actualType, expectedType, fieldValue, fieldPath)
		}
	}

	return nil
}

// getFieldValue extracts field value using reflection, supporting qualified field names
func (l *Loader) getFieldValue(resource any, fieldPath string) string {
	v := reflect.ValueOf(resource)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	// fmt.Printf("DEBUG: getFieldValue - type: %v, fieldPath: %s\n", v.Type(), fieldPath)

	// Split field path for nested fields (e.g., "service.control_plane_id")
	parts := strings.Split(fieldPath, ".")

	for i, part := range parts {
		// Handle both struct field names and YAML tags
		field := l.findField(v, part)
		if !field.IsValid() {
			return ""
		}

		// For the last part, get the string value
		if i == len(parts)-1 {
			if field.Kind() == reflect.String {
				return field.String()
			} else if field.Kind() == reflect.Pointer && !field.IsNil() {
				elem := field.Elem()
				if elem.Kind() == reflect.String {
					return elem.String()
				}
			}
			return ""
		}

		// For intermediate parts, navigate deeper
		if field.Kind() == reflect.Pointer {
			if field.IsNil() {
				return ""
			}
			v = field.Elem()
		} else {
			v = field
		}
	}

	return ""
}

// findField finds a field by name or YAML tag
func (l *Loader) findField(v reflect.Value, name string) reflect.Value {
	if v.Kind() != reflect.Struct {
		return reflect.Value{}
	}

	t := v.Type()

	// First try direct field name
	if field, ok := t.FieldByName(name); ok {
		return v.FieldByIndex(field.Index)
	}

	// Then try by YAML tag
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" {
			continue
		}

		// Handle yaml tags like "field_name,omitempty"
		tagParts := strings.Split(yamlTag, ",")
		if tagParts[0] == name {
			return v.Field(i)
		}
	}

	// Special case for embedded structs (like SDK types)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Anonymous {
			if embeddedField := l.findField(v.Field(i), name); embeddedField.IsValid() {
				return embeddedField
			}
		}
	}

	// Try converting snake_case to PascalCase for SDK field names
	pascalCase := l.toPascalCase(name)
	if pascalCase != name {
		return l.findField(v, pascalCase)
	}

	return reflect.Value{}
}

// toPascalCase converts snake_case to PascalCase
func (l *Loader) toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		if len(parts[i]) > 0 {
			// Special handling for common abbreviations
			if parts[i] == "id" || parts[i] == "api" || parts[i] == "url" {
				parts[i] = strings.ToUpper(parts[i])
			} else {
				parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
			}
		}
	}
	return strings.Join(parts, "")
}

// validateNamespaces validates all namespace values in the resource set
func (l *Loader) validateNamespaces(rs *resources.ResourceSet) error {
	nsValidator := validator.NewNamespaceValidator()

	// Validate all namespaces
	if err := nsValidator.ValidateNamespaces(rs.NamespaceValues()); err != nil {
		return fmt.Errorf("namespace validation failed: %w", err)
	}

	return nil
}
