package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// Resolve existing Identity IDs before comparing protected resources. Payload
// references in labels and association fields must be concrete even when their
// targets belong to a later namespace or need no operation. This identifies
// resources; their planners still validate ownership and mutation authority.
func (p *Planner) resolveIdentityPayloadIdentities(ctx context.Context, rs *resources.ResourceSet) error {
	if rs.Identity == nil {
		return nil
	}
	if len(rs.Identity.AuthServers) != 0 {
		all, err := p.listIdentityAuthServers(ctx)
		if err != nil {
			return err
		}
		for idx := range rs.Identity.AuthServers {
			server := &rs.Identity.AuthServers[idx]
			if !server.IsExternal() {
				server.SetKonnectID("")
				for _, current := range all {
					if server.TryMatchKonnectResource(current) {
						break
					}
				}
			}
		}
		for idx := range rs.Identity.Clients {
			client := &rs.Identity.Clients[idx]
			parent, found := rs.GetResourceByRef(resources.NormalizeResourceRef(client.AuthServer))
			if !found || parent.GetKonnectID() == "" {
				continue
			}
			all, err := p.listIdentityAuthServerClients(ctx, parent.GetKonnectID())
			if err != nil {
				return err
			}
			client.SetKonnectID("")
			for _, current := range all {
				if client.TryMatchKonnectResource(current) {
					break
				}
			}
		}
	}
	if len(rs.Identity.Directories) != 0 {
		all, err := p.listIdentityDirectories(ctx)
		if err != nil {
			return err
		}
		for idx := range rs.Identity.Directories {
			directory := &rs.Identity.Directories[idx]
			if directory.IsExternal() {
				continue
			}
			directory.SetKonnectID("")
			for _, current := range all {
				if directory.TryMatchKonnectResource(current) {
					break
				}
			}
		}
	}
	for idx := range rs.Identity.Principals {
		principal := &rs.Identity.Principals[idx]
		principal.SetKonnectID("")
		parent, found := rs.GetResourceByRef(resources.NormalizeResourceRef(principal.Directory))
		if !found || parent.GetKonnectID() == "" {
			if principal.IsExternal() {
				return fmt.Errorf("external principal %q requires an existing directory", principal.Ref)
			}
			continue
		}
		all, err := p.listIdentityPrincipals(ctx, parent.GetKonnectID())
		if err != nil {
			return err
		}
		matches := 0
		for _, current := range all {
			if principal.IsExternal() {
				if current.ID == principal.External.ID {
					principal.SetKonnectID(current.ID)
					matches++
				}
			} else if principal.TryMatchKonnectResource(current) {
				matches++
			}
		}
		if principal.IsExternal() && matches != 1 {
			return fmt.Errorf("external principal %q does not belong to the selected directory", principal.Ref)
		}
		if matches > 1 {
			return fmt.Errorf("multiple principals match the persisted identity of %q", principal.Ref)
		}
	}
	for idx := range rs.Identity.Identities {
		identity := &rs.Identity.Identities[idx]
		identity.SetKonnectID("")
		parent, found := rs.GetResourceByRef(resources.NormalizeResourceRef(identity.Principal))
		if !found || parent.GetKonnectID() == "" {
			if identity.IsExternal() {
				return fmt.Errorf("external identity %q requires an existing principal", identity.Ref)
			}
			continue
		}
		principal := parent.(*resources.IdentityPrincipalResource)
		directory, found := rs.GetResourceByRef(resources.NormalizeResourceRef(principal.Directory))
		if !found || directory.GetKonnectID() == "" {
			continue
		}
		all, err := p.listPrincipalIdentities(ctx, directory.GetKonnectID(), parent.GetKonnectID())
		if err != nil {
			return err
		}
		matches := 0
		for _, current := range all {
			if identity.IsExternal() {
				if current.ID == identity.External.ID {
					identity.SetKonnectID(current.ID)
					matches++
				}
			} else if identity.TryMatchKonnectResource(state.NormalizeIdentityPrincipalIdentity(current)) {
				matches++
			}
		}
		if identity.IsExternal() && matches != 1 {
			return fmt.Errorf("external identity %q does not belong to the selected principal", identity.Ref)
		}
		if matches > 1 {
			return fmt.Errorf("multiple identities match the persisted identity of %q", identity.Ref)
		}
	}
	return nil
}
