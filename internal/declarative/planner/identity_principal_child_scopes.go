package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

type identityPrincipalChildScope struct {
	ref, id, directoryRef, directoryID string
	refs                               map[string]bool
	scoped                             bool
}

func (p *identityDirectoryPlanner) principalChildScopes(ctx context.Context, config *Config,
	directories map[string]state.IdentityDirectory, kind resources.ResourceType,
) ([]*identityPrincipalChildScope, error) {
	rs := p.planner.resources
	var parents []*identityPrincipalChildScope
	byID := make(map[string]*identityPrincipalChildScope)
	for _, principal := range rs.Identity.Principals {
		d, found := rs.GetResourceByRef(resources.NormalizeResourceRef(principal.Directory))
		if !found {
			return nil, fmt.Errorf("identity principal %q has no declared directory", principal.Ref)
		}
		directory := d.(*resources.IdentityDirectoryResource)
		directoryID := directory.GetKonnectID()
		if current, found := directories[directory.Name]; found && !directory.IsExternal() {
			directoryID = current.ID
		}
		id := principal.GetKonnectID()
		if !principal.IsExternal() {
			id = ""
		}
		if !principal.IsExternal() && directoryID != "" {
			all, err := p.planner.listIdentityPrincipals(ctx, directoryID)
			if err != nil {
				return nil, err
			}
			matches := 0
			for _, current := range all {
				if current.ManagedBy[resources.ManagedByToolKey] == resources.ManagedByTool &&
					current.ManagedBy[resources.ChildNamespaceKey] == resources.GetNamespace(principal.Kongctl) &&
					current.GetName() == principal.Ref {
					id = current.ID
					matches++
				}
			}
			if matches > 1 {
				return nil, fmt.Errorf("multiple principals match identity parent %q", principal.Ref)
			}
		}
		key := directoryID + "/" + id
		if id == "" {
			key = principal.Ref
		}
		parent := byID[key]
		if parent == nil {
			parent = &identityPrincipalChildScope{
				ref: principal.Ref, id: id, directoryRef: directory.Ref, directoryID: directoryID,
				refs: make(map[string]bool),
			}
			parents = append(parents, parent)
			byID[key] = parent
		}
		parent.refs[principal.Ref] = true
		if rs.SyncScope != nil {
			byPrincipal := rs.SyncScope.ChildNamespaces[kind]
			_, scoped := byPrincipal[principal.Ref][config.Namespace]
			parent.scoped = parent.scoped || scoped
		}
	}

	return parents, nil
}
