package planner

import (
	"context"
	"fmt"
	"reflect"
	"slices"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
)

func (p *identityAuthServerPlanner) planChildren(ctx context.Context, config *Config, plan *Plan,
	byName map[string]state.IdentityAuthServer,
) error {
	rs := p.planner.resources
	if rs.Identity == nil {
		return nil
	}
	for _, server := range resources.GetResourcesByNamespace[resources.IdentityAuthServerResource](rs, config.Namespace) {
		if server.IsExternal() {
			continue
		}
		id := ""
		var protected bool
		if current, ok := byName[server.Name]; ok {
			id, protected = current.ID, labels.IsProtectedResource(current.Labels)
		}
		var scopes []resources.IdentityAuthServerScopeResource
		var claims []resources.IdentityAuthServerClaimResource
		for _, scope := range rs.Identity.Scopes {
			if resources.NormalizeResourceRef(scope.AuthServer) == server.Ref {
				scopes = append(scopes, scope)
			}
		}
		for _, claim := range rs.Identity.Claims {
			if resources.NormalizeResourceRef(claim.AuthServer) == server.Ref {
				claims = append(claims, claim)
			}
		}
		scopeScoped := rs.SyncScope != nil && rs.SyncScope.ChildInScope(
			resources.ResourceTypeIdentityAuthServer, server.Ref, resources.ResourceTypeIdentityAuthServerScope,
		)
		claimScoped := rs.SyncScope != nil && rs.SyncScope.ChildInScope(
			resources.ResourceTypeIdentityAuthServer, server.Ref, resources.ResourceTypeIdentityAuthServerClaim,
		)
		start := len(plan.Changes)
		if len(scopes) > 0 || (plan.Metadata.Mode == PlanModeSync && scopeScoped) {
			var observed []state.IdentityAuthServerScope
			if id != "" {
				var err error
				observed, err = p.planner.listIdentityAuthServerScopes(ctx, id)
				if err != nil {
					return err
				}
			}
			err := planAuthServerChildCollection(p, config, plan, server, id, protected, scopeScoped,
				ResourceTypeIdentityAuthServerScope, scopes, observed,
				func(s resources.IdentityAuthServerScopeResource) string { return s.Ref },
				func(s resources.IdentityAuthServerScopeResource) map[string]any { return identityScopeFields(s) },
				func(s state.IdentityAuthServerScope) (string, string, map[string]any) {
					return s.ID, s.Name, observedScopeFields(s)
				})
			if err != nil {
				return err
			}
		}
		if len(claims) > 0 || (plan.Metadata.Mode == PlanModeSync && claimScoped) {
			var observed []state.IdentityAuthServerClaim
			if id != "" {
				var err error
				observed, err = p.planner.listIdentityAuthServerClaims(ctx, id)
				if err != nil {
					return err
				}
			}
			var scopeIDs map[string]bool
			for i := range claims {
				claims[i].IncludeInScopes = slices.Clone(claims[i].IncludeInScopes)
				for j, value := range claims[i].IncludeInScopes {
					if tags.IsRefPlaceholder(value) {
						resolved, err := rs.ResolvePayloadReference(value)
						if err != nil {
							return err
						}
						resolvedID, ok := resolved.(string)
						if !ok {
							return fmt.Errorf("scope reference must resolve to an ID string")
						}
						value = resolvedID
					}
					claims[i].IncludeInScopes[j] = value
					if plan.Metadata.Mode == PlanModeDelete || tags.IsRefPlaceholder(value) {
						continue
					}
					if scopeIDs == nil {
						scopeIDs = make(map[string]bool)
						if id != "" {
							scopes, err := p.planner.listIdentityAuthServerScopes(ctx, id)
							if err != nil {
								return err
							}
							for _, scope := range scopes {
								scopeIDs[scope.ID] = true
							}
						}
					}
					if !scopeIDs[value] {
						return fmt.Errorf("claim %q scope ID does not belong to auth server %q",
							claims[i].Ref, server.Name)
					}
				}
			}
			err := planAuthServerChildCollection(p, config, plan, server, id, protected, claimScoped,
				ResourceTypeIdentityAuthServerClaim, claims, observed,
				func(c resources.IdentityAuthServerClaimResource) string { return c.Ref },
				func(c resources.IdentityAuthServerClaimResource) map[string]any { return identityClaimFields(c) },
				func(c state.IdentityAuthServerClaim) (string, string, map[string]any) {
					return c.ID, c.Name, observedClaimFields(c)
				})
			if err != nil {
				return err
			}
		}
		for i := start; i < len(plan.Changes); i++ {
			change := &plan.Changes[i]
			for j := range plan.Changes[:start] {
				root := &plan.Changes[j]
				if root.ResourceType != ResourceTypeIdentityAuthServer {
					continue
				}
				if root.Action == ActionCreate && root.ResourceRef == server.Ref {
					change.DependsOn = appendDependsOn(change.DependsOn, root.ID)
				}
				if root.Action == ActionDelete && root.ResourceID == id && change.Action == ActionDelete {
					root.DependsOn = appendDependsOn(root.DependsOn, change.ID)
				}
			}
		}
	}
	return nil
}

func planAuthServerChildCollection[D, C any](p *identityAuthServerPlanner, config *Config, plan *Plan,
	server resources.IdentityAuthServerResource, serverID string, protected, scoped bool,
	kind string, desired []D, observed []C, ref func(D) string, fields func(D) map[string]any,
	current func(C) (string, string, map[string]any),
) error {
	type record struct {
		id, name string
		fields   map[string]any
	}
	byName := make(map[string]record)
	for _, value := range observed {
		id, name, data := current(value)
		if _, duplicate := byName[name]; duplicate {
			return fmt.Errorf("multiple %s records have name %q", kind, name)
		}
		byName[name] = record{id, name, data}
	}
	desiredNames := make(map[string]bool)
	add := func(action ActionType, resourceRef, id string, data map[string]any, changed map[string]FieldChange) error {
		if protected && action != ActionCreate {
			return fmt.Errorf("auth server %q is protected", server.Name)
		}
		change := PlannedChange{
			ID: p.NextChangeID(action, kind, resourceRef), ResourceType: kind,
			ResourceRef: resourceRef, ResourceID: id, Namespace: config.Namespace,
			Fields: data, ChangedFields: changed, Protection: false, Action: action,
		}
		change.Parent = &ParentInfo{Ref: server.Ref, ID: serverID}
		change.ProtectingParent = &ProtectingParentInfo{
			ResourceType: ResourceTypeIdentityAuthServer,
			ResourceRef:  server.Ref, ResourceID: serverID, ResourceName: server.Name,
		}
		plan.AddChange(change)
		return nil
	}
	for _, d := range desired {
		data := fields(d)
		name := data[FieldName].(string)
		desiredNames[name] = true
		c, found := byName[name]
		if found {
			if target, ok := p.planner.resources.GetResourceByRef(ref(d)); ok {
				if setter, ok := target.(interface{ SetKonnectID(string) }); ok {
					setter.SetKonnectID(c.id)
				}
			}
		}
		if plan.Metadata.Mode == PlanModeDelete {
			if found {
				if err := add(ActionDelete, ref(d), c.id, map[string]any{FieldName: name}, nil); err != nil {
					return err
				}
			}
			continue
		}
		if !found {
			if err := add(ActionCreate, ref(d), "", data, nil); err != nil {
				return err
			}
			continue
		}
		update := map[string]any{FieldName: name}
		changed := make(map[string]FieldChange)
		for field, next := range data {
			old := c.fields[field]
			if field == FieldIncludeInScopes {
				oldValues, _ := old.([]any)
				nextValues, _ := next.([]any)
				if authScopeSetsEqual(oldValues, nextValues) {
					continue
				}
			} else if reflect.DeepEqual(old, next) {
				continue
			}
			update[field] = next
			changed[field] = FieldChange{Old: old, New: next}
		}
		if len(changed) > 0 {
			if err := add(ActionUpdate, ref(d), c.id, update, changed); err != nil {
				return err
			}
		}
	}
	if plan.Metadata.Mode == PlanModeSync && scoped {
		for _, value := range observed {
			id, name, _ := current(value)
			if desiredNames[name] {
				continue
			}
			if err := add(ActionDelete, name, id, map[string]any{FieldName: name}, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func authScopeSetsEqual(old, next []any) bool {
	if len(old) != len(next) {
		return false
	}
	a, b := make([]string, len(old)), make([]string, len(next))
	for i := range old {
		a[i], _ = old[i].(string)
		b[i], _ = next[i].(string)
	}
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

func authScopeIDs(values []string) []any {
	result := make([]any, len(values))
	for i, value := range values {
		result[i] = value
	}
	return result
}

func identityScopeFields(s resources.IdentityAuthServerScopeResource) map[string]any {
	fields := map[string]any{FieldName: s.Name}
	if s.Description != nil {
		fields[FieldDescription] = *s.Description
	}
	if s.Default != nil {
		fields[FieldDefault] = *s.Default
	}
	if s.Enabled != nil {
		fields[FieldEnabled] = *s.Enabled
	}
	if s.IncludeInMetadata != nil {
		fields[FieldIncludeInMetadata] = *s.IncludeInMetadata
	}
	return fields
}

func observedScopeFields(s state.IdentityAuthServerScope) map[string]any {
	f := map[string]any{FieldName: s.Name, FieldDescription: s.Description}
	if s.Default != nil {
		f[FieldDefault] = *s.Default
	}
	if s.Enabled != nil {
		f[FieldEnabled] = *s.Enabled
	}
	if s.IncludeInMetadata != nil {
		f[FieldIncludeInMetadata] = *s.IncludeInMetadata
	}
	return f
}

func identityClaimFields(c resources.IdentityAuthServerClaimResource) map[string]any {
	f := map[string]any{FieldName: c.Name, FieldValue: c.Value}
	if c.IncludeInToken != nil {
		f[FieldIncludeInToken] = *c.IncludeInToken
	}
	if c.IncludeInAllScopes != nil {
		f[FieldIncludeInAllScopes] = *c.IncludeInAllScopes
	}
	if c.Enabled != nil {
		f[FieldEnabled] = *c.Enabled
	}
	if c.IncludeInScopes != nil {
		f[FieldIncludeInScopes] = authScopeIDs(c.IncludeInScopes)
	}
	return f
}

func observedClaimFields(c state.IdentityAuthServerClaim) map[string]any {
	f := map[string]any{FieldName: c.Name, FieldValue: c.Value, FieldIncludeInScopes: authScopeIDs(c.IncludeInScopes)}
	if c.IncludeInToken != nil {
		f[FieldIncludeInToken] = *c.IncludeInToken
	}
	if c.IncludeInAllScopes != nil {
		f[FieldIncludeInAllScopes] = *c.IncludeInAllScopes
	}
	if c.Enabled != nil {
		f[FieldEnabled] = *c.Enabled
	}
	return f
}
