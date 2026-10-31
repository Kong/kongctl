package planner

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

type identityAuthServerPlanner struct{ *BasePlanner }

func (p *identityAuthServerPlanner) PlannerComponent() string { return ResourceTypeIdentityAuthServer }

func (p *identityAuthServerPlanner) PlanChanges(ctx context.Context, config *Config, plan *Plan) error {
	desired := resources.GetResourcesByNamespace[resources.IdentityAuthServerResource](
		p.planner.resources, config.Namespace,
	)
	hasClients := p.planner.resources.Identity != nil && len(p.planner.resources.Identity.Clients) > 0
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync && !hasClients {
		return nil
	}
	all, err := p.planner.listIdentityAuthServers(ctx)
	if err != nil {
		if state.IsAPIClientError(err) && len(desired) == 0 && !hasClients {
			return nil
		}
		return err
	}
	byName := make(map[string]state.IdentityAuthServer)
	for _, current := range all {
		if _, exists := byName[current.Name]; exists {
			return fmt.Errorf("multiple auth servers have name %q", current.Name)
		}
		byName[current.Name] = current
	}
	externalIDs := make(map[string]bool)
	externalOnly := false
	if p.planner.resources.Identity != nil {
		externalOnly = len(p.planner.resources.Identity.AuthServers) > 0
		for _, a := range p.planner.resources.Identity.AuthServers {
			if a.IsExternal() {
				externalIDs[a.GetKonnectID()] = true
			} else {
				externalOnly = false
			}
		}
	}
	var currentRoots []managedRoot[state.IdentityAuthServer]
	for _, current := range all {
		if externalOnly || externalIDs[current.ID] || current.Labels[labels.NamespaceKey] != config.Namespace {
			continue
		}
		if scope := p.planner.resources.SyncScope; plan.Metadata.Mode == PlanModeSync && scope != nil &&
			scope.IdentityAuthServerNamespaces != nil {
			if _, scoped := scope.IdentityAuthServerNamespaces[config.Namespace]; !scoped {
				continue
			}
		}
		currentRoots = append(currentRoots, managedRoot[state.IdentityAuthServer]{
			resource: current, name: current.Name, protected: labels.IsProtectedResource(current.NormalizedLabels),
		})
	}
	var desiredRoots []managedRoot[resources.IdentityAuthServerResource]
	for _, a := range desired {
		if a.IsExternal() {
			continue
		}
		if current, exists := byName[a.Name]; exists {
			if current.Labels[labels.NamespaceKey] == "" {
				return fmt.Errorf("auth server %q is unowned; explicitly adopt it before managing it", a.Name)
			}
			if current.Labels[labels.NamespaceKey] != config.Namespace {
				return fmt.Errorf("auth server %q belongs to namespace %q", a.Name, current.Labels[labels.NamespaceKey])
			}
		}
		desiredRoots = append(desiredRoots, managedRoot[resources.IdentityAuthServerResource]{
			resource: a, name: a.Name, protected: a.Protected(),
		})
	}
	if err := reconcileManagedRoots(p.BasePlanner, ResourceTypeIdentityAuthServer, desiredRoots, currentRoots,
		managedRootOperations[resources.IdentityAuthServerResource, state.IdentityAuthServer]{
			diff: diffIdentityAuthServer,
			create: func(a resources.IdentityAuthServerResource, plan *Plan) string {
				change := p.change(ActionCreate, a.Ref, "", resources.GetNamespace(a.Kongctl),
					identityAuthServerFields(a), nil, a.Protected())
				plan.AddChange(change)
				return change.ID
			},
			update: func(c state.IdentityAuthServer, a resources.IdentityAuthServerResource,
				fields map[string]any, changed map[string]FieldChange, plan *Plan,
			) {
				fields[FieldName] = c.Name
				plan.AddChange(p.change(ActionUpdate, a.Ref, c.ID,
					resources.GetNamespace(a.Kongctl), fields, changed, a.Protected()))
			},
			changeProtection: func(c state.IdentityAuthServer, a resources.IdentityAuthServerResource,
				old, next bool, fields map[string]any, changed map[string]FieldChange, plan *Plan,
			) {
				fields = maps.Clone(fields)
				changed = maps.Clone(changed)
				fields[FieldName] = c.Name
				changed[FieldProtected] = FieldChange{Old: old, New: next}
				plan.AddChange(p.change(ActionUpdate, a.Ref, c.ID, resources.GetNamespace(a.Kongctl),
					fields, changed, ProtectionChange{Old: old, New: next}))
			},
			remove: func(c state.IdentityAuthServer, plan *Plan) {
				plan.AddChange(p.change(ActionDelete, c.Name, c.ID, c.Labels[labels.NamespaceKey],
					map[string]any{FieldName: c.Name}, nil, false))
			},
		}, plan); err != nil {
		return err
	}
	if err := p.planChildren(ctx, config, plan, byName); err != nil {
		return err
	}
	serverIDs := make(map[string]string)
	for name, server := range byName {
		serverIDs[name] = server.ID
	}
	return p.planClients(ctx, config, plan, serverIDs)
}

func (p *identityAuthServerPlanner) change(action ActionType, ref, id, namespace string,
	fields map[string]any, changed map[string]FieldChange, protection any,
) PlannedChange {
	return PlannedChange{
		ID: p.NextChangeID(action, ResourceTypeIdentityAuthServer, ref), Action: action,
		ResourceType: ResourceTypeIdentityAuthServer, ResourceRef: ref, ResourceID: id, Namespace: namespace,
		Fields: fields, ChangedFields: changed, Protection: protection,
	}
}

func identityAuthServerFields(a resources.IdentityAuthServerResource) map[string]any {
	fields := map[string]any{FieldName: a.Name, FieldAudience: a.Audience}
	if a.Description != nil {
		fields[FieldDescription] = *a.Description
	}
	if a.SigningAlgorithm != nil {
		fields[FieldSigningAlgorithm] = string(*a.SigningAlgorithm)
	}
	if a.TrustedOrigins != nil {
		fields[FieldTrustedOrigins] = slices.Clone(a.TrustedOrigins)
	}
	if a.DCR != nil && a.DCR.DefaultAccessTokenDuration != nil {
		fields[FieldDCR] = map[string]any{FieldDefaultAccessTokenDuration: *a.DCR.DefaultAccessTokenDuration}
	}
	if a.Labels != nil {
		fields[FieldLabels] = a.Labels
	}
	return fields
}

func diffIdentityAuthServer(
	c state.IdentityAuthServer, a resources.IdentityAuthServerResource,
) (bool, map[string]any, map[string]FieldChange) {
	fields := make(map[string]any)
	changed := make(map[string]FieldChange)
	compare := func(field string, old, next any) {
		if old != next {
			fields[field] = next
			changed[field] = FieldChange{Old: old, New: next}
		}
	}
	compare(FieldAudience, c.Audience, a.Audience)
	if a.Description != nil {
		compare(FieldDescription, c.Description, *a.Description)
	}
	if a.SigningAlgorithm != nil {
		var old string
		if c.SigningAlgorithm != nil {
			old = string(*c.SigningAlgorithm)
		}
		compare(FieldSigningAlgorithm, old, string(*a.SigningAlgorithm))
	}
	if a.TrustedOrigins != nil {
		old, next := slices.Clone(c.TrustedOrigins), slices.Clone(a.TrustedOrigins)
		slices.Sort(old)
		slices.Sort(next)
		if !slices.Equal(old, next) {
			fields[FieldTrustedOrigins] = slices.Clone(a.TrustedOrigins)
			changed[FieldTrustedOrigins] = FieldChange{Old: c.TrustedOrigins, New: a.TrustedOrigins}
		}
	}
	if a.DCR != nil && a.DCR.DefaultAccessTokenDuration != nil {
		var old any
		if c.Dcr != nil && c.Dcr.DefaultAccessTokenDuration != nil {
			old = *c.Dcr.DefaultAccessTokenDuration
		}
		if old != *a.DCR.DefaultAccessTokenDuration {
			fields[FieldDCR] = identityAuthServerFields(a)[FieldDCR]
			changed[FieldDCR] = FieldChange{Old: c.Dcr, New: fields[FieldDCR]}
		}
	}
	if a.Labels != nil && !maps.Equal(labels.GetUserLabels(c.Labels), a.Labels) {
		fields[FieldLabels] = a.Labels
		changed[FieldLabels] = FieldChange{Old: labels.GetUserLabels(c.Labels), New: a.Labels}
	}
	return len(fields) > 0, fields, changed
}
