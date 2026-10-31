package planner

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
)

func (p *identityAuthServerPlanner) planClients(ctx context.Context, config *Config, plan *Plan,
	servers map[string]string,
) error {
	rs := p.planner.resources
	if rs.Identity == nil {
		return nil
	}
	type parent struct {
		ref, id, name string
		aliases       map[string]bool
		scoped        bool
		desired       []resources.IdentityAuthServerClientResource
	}
	var parents []*parent
	byID := make(map[string]*parent)
	for _, server := range rs.Identity.AuthServers {
		id := server.GetKonnectID()
		if !server.IsExternal() {
			id = servers[server.Name]
		}
		key := id
		if key == "" {
			key = server.Ref
		}
		entry := byID[key]
		if entry == nil {
			name := server.Name
			if id != "" {
				current, err := p.planner.client.GetIdentityAuthServerByID(ctx, id)
				if err != nil {
					return err
				}
				if current == nil {
					return fmt.Errorf("client parent auth server %q no longer exists", server.Ref)
				}
				name = current.Name
			}
			entry = &parent{ref: server.Ref, id: id, name: name, aliases: make(map[string]bool)}
			parents, byID[key] = append(parents, entry), entry
		}
		entry.aliases[server.Ref] = true
		if rs.SyncScope != nil {
			namespaces := rs.SyncScope.ChildNamespaces[resources.ResourceTypeIdentityAuthServerClient][server.Ref]
			_, scoped := namespaces[config.Namespace]
			entry.scoped = entry.scoped || scoped
		}
	}
	for _, entry := range parents {
		for _, client := range rs.Identity.Clients {
			if entry.aliases[resources.NormalizeResourceRef(client.AuthServer)] &&
				resources.GetNamespace(client.Kongctl) == config.Namespace {
				entry.desired = append(entry.desired, client)
			}
		}
		if len(entry.desired) == 0 && (plan.Metadata.Mode != PlanModeSync || !entry.scoped) {
			continue
		}
		var observed []kkComps.AuthServerClient
		if entry.id != "" {
			var err error
			observed, err = p.planner.listIdentityAuthServerClients(ctx, entry.id)
			if err != nil {
				return err
			}
		}
		byName := make(map[string]kkComps.AuthServerClient)
		for _, current := range observed {
			if _, exists := byName[current.Name]; exists {
				return fmt.Errorf("multiple clients have name %q under auth server %q", current.Name, entry.name)
			}
			byName[current.Name] = current
		}
		seenNames := make(map[string]bool)
		var desired []managedRoot[resources.IdentityAuthServerClientResource]
		for _, client := range entry.desired {
			if seenNames[client.Name] {
				return fmt.Errorf("duplicate client name %q through auth-server aliases", client.Name)
			}
			seenNames[client.Name] = true
			if current, found := byName[client.Name]; found {
				if err := validateClientLabelOwner(current, config.Namespace); err != nil {
					return err
				}
				if resource, ok := rs.GetResourceByRef(client.Ref); ok {
					if setter, ok := resource.(interface{ SetKonnectID(string) }); ok {
						setter.SetKonnectID(current.ID)
					}
				}
			}
			client.AllowScopes = slices.Clone(client.AllowScopes)
			if plan.Metadata.Mode != PlanModeDelete {
				var scopeIDs map[string]bool
				scopeNames := make(map[string]string)
				if entry.id != "" && len(client.AllowScopes) > 0 {
					values, err := p.planner.listIdentityAuthServerScopes(ctx, entry.id)
					if err != nil {
						return err
					}
					scopeIDs = make(map[string]bool)
					for _, scope := range values {
						if scopeNames[scope.Name] != "" {
							return fmt.Errorf("multiple scopes have name %q", scope.Name)
						}
						scopeIDs[scope.ID], scopeNames[scope.Name] = true, scope.ID
					}
				}
				for i, value := range client.AllowScopes {
					if tags.IsRefPlaceholder(value) {
						ref, _, _ := tags.ParseRefPlaceholder(value)
						resolved, err := rs.ResolvePayloadReference(value)
						if err != nil {
							return err
						}
						var ok bool
						value, ok = resolved.(string)
						if !ok {
							return fmt.Errorf("client scope reference must resolve to an ID string")
						}
						// The client's namespace may be planned before its parent's.
						// Match existing scopes now so a stable reference is a no-op.
						if tags.IsRefPlaceholder(value) {
							if target, found := rs.GetResourceByRef(ref); found {
								if scope, ok := target.(*resources.IdentityAuthServerScopeResource); ok &&
									scopeNames[scope.Name] != "" {
									value = scopeNames[scope.Name]
									scope.SetKonnectID(value)
								}
							}
						}
					}
					client.AllowScopes[i] = value
					if tags.IsRefPlaceholder(value) {
						continue
					}
					if scopeIDs == nil {
						scopeIDs = make(map[string]bool)
					}
					if !scopeIDs[value] {
						return fmt.Errorf(
							"client %q scope ID does not belong to auth server %q",
							client.Ref,
							entry.name,
						)
					}
				}
			}
			desired = append(desired, managedRoot[resources.IdentityAuthServerClientResource]{
				resource: client,
				name:     client.Name, protected: client.Protected(),
			})
		}
		var current []managedRoot[kkComps.AuthServerClient]
		for _, client := range observed {
			if client.Labels[labels.NamespaceKey] == config.Namespace {
				if err := validateClientLabelOwner(client, config.Namespace); err != nil {
					return err
				}
				current = append(current, managedRoot[kkComps.AuthServerClient]{
					resource: client, name: client.Name,
					protected: labels.IsProtectedResource(client.Labels),
				})
			}
		}
		start := len(plan.Changes)
		childPlan := plan
		if plan.Metadata.Mode == PlanModeSync && !entry.scoped {
			copyPlan := *plan
			copyPlan.Metadata.Mode = PlanModeApply
			childPlan = &copyPlan
		}
		add := func(action ActionType, ref, id string, fields map[string]any,
			changed map[string]FieldChange, protection any,
		) string {
			change := PlannedChange{
				ID: p.NextChangeID(action, ResourceTypeIdentityAuthServerClient, ref), Action: action,
				ResourceType: ResourceTypeIdentityAuthServerClient, ResourceRef: ref, ResourceID: id, Fields: fields,
				ChangedFields: changed, Namespace: config.Namespace, Protection: protection,
				Parent: &ParentInfo{Ref: entry.ref, ID: entry.id}, References: map[string]ReferenceInfo{
					FieldAuthServerID: {
						Ref:          entry.ref,
						ID:           entry.id,
						LookupFields: map[string]string{FieldName: entry.name},
					},
				},
			}
			childPlan.AddChange(change)
			return change.ID
		}
		// Collection authority is scoped to the selected child namespace, not all
		// namespaces visible through the auth-server parent.
		if err := reconcileManagedRoots(p.BasePlanner, ResourceTypeIdentityAuthServerClient, desired, current,
			managedRootOperations[resources.IdentityAuthServerClientResource, kkComps.AuthServerClient]{
				diff: diffIdentityClient,
				create: func(d resources.IdentityAuthServerClientResource, _ *Plan) string {
					return add(ActionCreate, d.Ref, "", identityClientFields(d), nil, d.Protected())
				},
				update: func(c kkComps.AuthServerClient, d resources.IdentityAuthServerClientResource,
					fields map[string]any, changed map[string]FieldChange, _ *Plan,
				) {
					fields[FieldName] = c.Name
					add(ActionUpdate, d.Ref, c.ID, fields, changed, d.Protected())
				},
				changeProtection: func(c kkComps.AuthServerClient, d resources.IdentityAuthServerClientResource,
					old, next bool, fields map[string]any, changed map[string]FieldChange, _ *Plan,
				) {
					fields, changed = maps.Clone(fields), maps.Clone(changed)
					fields[FieldName] = c.Name
					changed[FieldProtected] = FieldChange{Old: old, New: next}
					add(ActionUpdate, d.Ref, c.ID, fields, changed, ProtectionChange{Old: old, New: next})
				},
				remove: func(c kkComps.AuthServerClient, _ *Plan) {
					add(ActionDelete, c.Name, c.ID,
						map[string]any{FieldName: c.Name}, nil, false)
				},
			}, childPlan); err != nil {
			return err
		}
		if childPlan != plan {
			plan.Changes = childPlan.Changes
		}
		for i := start; i < len(plan.Changes); i++ {
			for _, root := range plan.Changes[:start] {
				if root.ResourceType == ResourceTypeIdentityAuthServer && root.Action == ActionCreate &&
					entry.aliases[root.ResourceRef] {
					plan.Changes[i].DependsOn = appendDependsOn(plan.Changes[i].DependsOn, root.ID)
				}
			}
		}
	}
	return nil
}

func validateClientLabelOwner(client kkComps.AuthServerClient, namespace string) error {
	owner := client.Labels[labels.NamespaceKey]
	if owner == "" {
		return fmt.Errorf("client %q is unowned; explicitly adopt it before managing it", client.Name)
	}
	if owner != namespace {
		return fmt.Errorf("client %q belongs to namespace %q", client.Name, owner)
	}
	if value := client.Labels[labels.ProtectedKey]; value != "" && value != "true" && value != "false" {
		return fmt.Errorf("client %q has invalid protection labels", client.Name)
	}
	return nil
}

func identityClientEnumValues[T ~string](values []T) []any {
	result := make([]any, len(values))
	for i, value := range values {
		result[i] = string(value)
	}
	return result
}

func identityClientFields(c resources.IdentityAuthServerClientResource) map[string]any {
	fields := map[string]any{
		FieldName: c.Name, FieldGrantTypes: identityClientEnumValues(c.GrantTypes),
		FieldResponseTypes: identityClientEnumValues(c.ResponseTypes),
	}
	if c.RedirectURIs != nil {
		fields[FieldRedirectURIs] = authScopeIDs(c.RedirectURIs)
	}
	if c.LoginURI != nil {
		fields[FieldLoginURI] = *c.LoginURI
	}
	if c.AccessTokenDuration != nil {
		fields[FieldAccessTokenDuration] = *c.AccessTokenDuration
	}
	if c.IDTokenDuration != nil {
		fields[FieldIDTokenDuration] = *c.IDTokenDuration
	}
	if c.AllowAllScopes != nil {
		fields[FieldAllowAllScopes] = *c.AllowAllScopes
	}
	if c.AllowScopes != nil {
		fields[FieldAllowScopes] = authScopeIDs(c.AllowScopes)
	}
	if c.TokenEndpointAuthMethod != nil {
		fields[FieldTokenEndpointAuthMethod] = string(*c.TokenEndpointAuthMethod)
	}
	if c.Labels != nil {
		fields[FieldLabels] = maps.Clone(c.Labels)
	}
	return fields
}

func observedIdentityClientFields(c kkComps.AuthServerClient) map[string]any {
	fields := map[string]any{
		FieldName: c.Name, FieldGrantTypes: identityClientEnumValues(c.GrantTypes),
		FieldResponseTypes: identityClientEnumValues(c.ResponseTypes), FieldRedirectURIs: authScopeIDs(c.RedirectUris),
		FieldAllowScopes: authScopeIDs(c.AllowScopes), FieldLabels: labels.GetUserLabels(c.Labels),
	}
	if c.LoginURI != nil {
		fields[FieldLoginURI] = *c.LoginURI
	}
	if c.AccessTokenDuration != nil {
		fields[FieldAccessTokenDuration] = *c.AccessTokenDuration
	}
	if c.IDTokenDuration != nil {
		fields[FieldIDTokenDuration] = *c.IDTokenDuration
	}
	if c.AllowAllScopes != nil {
		fields[FieldAllowAllScopes] = *c.AllowAllScopes
	}
	if c.TokenEndpointAuthMethod != nil {
		fields[FieldTokenEndpointAuthMethod] = string(*c.TokenEndpointAuthMethod)
	}
	return fields
}

func diffIdentityClient(current kkComps.AuthServerClient, desired resources.IdentityAuthServerClientResource) (
	bool, map[string]any, map[string]FieldChange,
) {
	fields, changes := make(map[string]any), make(map[string]FieldChange)
	observed := observedIdentityClientFields(current)
	for field, next := range identityClientFields(desired) {
		old := observed[field]
		if field == FieldName {
			continue
		}
		if field == FieldLabels && maps.Equal(labels.ExtractLabelsFromField(old), labels.ExtractLabelsFromField(next)) {
			continue
		}
		if field == FieldAllowScopes || field == FieldGrantTypes {
			before, _ := old.([]any)
			after, _ := next.([]any)
			if authScopeSetsEqual(before, after) {
				continue
			}
		} else if reflect.DeepEqual(old, next) {
			continue
		}
		fields[field], changes[field] = next, FieldChange{Old: old, New: next}
	}
	return len(fields) > 0, fields, changes
}
