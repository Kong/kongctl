package resources

// NamespaceParticipant is a namespace-bearing declarative parent resource. It is
// the single source of truth for which resources carry kongctl namespace
// metadata, shared by loader defaulting, namespace validation, and planner
// namespace discovery. Callers keep their own handling of external resources,
// which is intentionally not uniform across those paths.
type NamespaceParticipant struct {
	Type ResourceType
	Ref  string
	// External reports whether the resource is declared as an external reference.
	// Callers decide what that means: the loader rejects kongctl metadata on
	// external resources, the validator skips them, and the planner maps some of
	// them to the external namespace while external control planes still
	// contribute their own.
	External bool
	// SupportsProtected reports whether kongctl.protected defaulting applies.
	// Organization users and system accounts only carry a namespace.
	SupportsProtected bool
	// Label is the human-facing name used in loader defaulting error messages.
	Label string
	// Meta addresses the resource's Kongctl field so callers can read the current
	// metadata and assign defaults in place.
	Meta **KongctlMeta
}

// ForEachNamespaceParticipant visits every namespace-bearing resource in rs and
// returns early if fn returns an error. Both the nested (Analytics.Dashboards,
// Organization.Teams) and the flattened (Dashboards, OrganizationTeams)
// locations are visited so the iterator is correct both before and after
// extractNestedResources runs; only one side is populated at a given stage.
//
// The visit order matches the namespace validator's violation order so error
// messages are unchanged.
func (rs *ResourceSet) ForEachNamespaceParticipant(fn func(NamespaceParticipant) error) error {
	if rs == nil {
		return nil
	}

	for i := range rs.Portals {
		if err := fn(NamespaceParticipant{
			Type: ResourceTypePortal, Ref: rs.Portals[i].Ref, External: rs.Portals[i].IsExternal(),
			SupportsProtected: true, Label: "portal", Meta: &rs.Portals[i].Kongctl,
		}); err != nil {
			return err
		}
	}
	for i := range rs.APIs {
		if err := fn(NamespaceParticipant{
			Type: ResourceTypeAPI, Ref: rs.APIs[i].Ref,
			SupportsProtected: true, Label: "api", Meta: &rs.APIs[i].Kongctl,
		}); err != nil {
			return err
		}
	}
	for i := range rs.CatalogServices {
		if err := fn(NamespaceParticipant{
			Type: ResourceTypeCatalogService, Ref: rs.CatalogServices[i].Ref,
			SupportsProtected: true, Label: "catalog_service", Meta: &rs.CatalogServices[i].Kongctl,
		}); err != nil {
			return err
		}
	}
	for i := range rs.AIGateways {
		if err := fn(NamespaceParticipant{
			Type: ResourceTypeAIGateway, Ref: rs.AIGateways[i].Ref, External: rs.AIGateways[i].IsExternal(),
			SupportsProtected: true, Label: "ai_gateway", Meta: &rs.AIGateways[i].Kongctl,
		}); err != nil {
			return err
		}
	}
	if err := rs.forEachDashboardParticipant(fn); err != nil {
		return err
	}
	for i := range rs.EventGatewayControlPlanes {
		if err := fn(NamespaceParticipant{
			Type: ResourceTypeEventGatewayControlPlane, Ref: rs.EventGatewayControlPlanes[i].Ref,
			External: rs.EventGatewayControlPlanes[i].IsExternal(), SupportsProtected: true,
			Label: "event_gateway", Meta: &rs.EventGatewayControlPlanes[i].Kongctl,
		}); err != nil {
			return err
		}
	}
	for i := range rs.ApplicationAuthStrategies {
		if err := fn(NamespaceParticipant{
			Type: ResourceTypeApplicationAuthStrategy, Ref: rs.ApplicationAuthStrategies[i].Ref,
			SupportsProtected: true, Label: "application_auth_strategy",
			Meta: &rs.ApplicationAuthStrategies[i].Kongctl,
		}); err != nil {
			return err
		}
	}
	for i := range rs.DCRProviders {
		if err := fn(NamespaceParticipant{
			Type: ResourceTypeDCRProvider, Ref: rs.DCRProviders[i].Ref,
			SupportsProtected: true, Label: "dcr_provider", Meta: &rs.DCRProviders[i].Kongctl,
		}); err != nil {
			return err
		}
	}
	for i := range rs.ControlPlanes {
		if err := fn(NamespaceParticipant{
			Type: ResourceTypeControlPlane, Ref: rs.ControlPlanes[i].Ref, External: rs.ControlPlanes[i].IsExternal(),
			SupportsProtected: true, Label: "control_plane", Meta: &rs.ControlPlanes[i].Kongctl,
		}); err != nil {
			return err
		}
	}
	if err := rs.forEachOrganizationTeamParticipant(fn); err != nil {
		return err
	}
	if rs.Organization != nil {
		for i := range rs.Organization.Users {
			if err := fn(NamespaceParticipant{
				Type: ResourceTypeOrganizationUser, Ref: rs.Organization.Users[i].Ref,
				Label: "organization user", Meta: &rs.Organization.Users[i].Kongctl,
			}); err != nil {
				return err
			}
		}
		for i := range rs.Organization.SystemAccounts {
			if err := fn(NamespaceParticipant{
				Type: ResourceTypeOrganizationSystemAccount, Ref: rs.Organization.SystemAccounts[i].Ref,
				Label: "organization system account", Meta: &rs.Organization.SystemAccounts[i].Kongctl,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (rs *ResourceSet) forEachDashboardParticipant(fn func(NamespaceParticipant) error) error {
	dashboard := func(d *DashboardResource) error {
		return fn(NamespaceParticipant{
			Type: ResourceTypeDashboard, Ref: d.Ref,
			SupportsProtected: true, Label: "dashboard", Meta: &d.Kongctl,
		})
	}
	for i := range rs.Dashboards {
		if err := dashboard(&rs.Dashboards[i]); err != nil {
			return err
		}
	}
	if rs.Analytics != nil {
		for i := range rs.Analytics.Dashboards {
			if err := dashboard(&rs.Analytics.Dashboards[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (rs *ResourceSet) forEachOrganizationTeamParticipant(fn func(NamespaceParticipant) error) error {
	team := func(t *OrganizationTeamResource) error {
		return fn(NamespaceParticipant{
			Type: ResourceTypeOrganizationTeam, Ref: t.Ref, External: t.IsExternal(),
			SupportsProtected: true, Label: "team", Meta: &t.Kongctl,
		})
	}
	for i := range rs.OrganizationTeams {
		if err := team(&rs.OrganizationTeams[i]); err != nil {
			return err
		}
	}
	if rs.Organization != nil {
		for i := range rs.Organization.Teams {
			if err := team(&rs.Organization.Teams[i]); err != nil {
				return err
			}
		}
	}
	return nil
}
