package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

// The final merged plan contains all target deletions, including those produced
// by scoped sync. Registry inventory is global, independent of those scopes.
func (p *Planner) validateIdentityRegistryReferences(ctx context.Context, plan *Plan) error {
	var links []helpers.RegistryAgentPrincipalReference
	observed := false
	for _, change := range plan.Changes {
		if change.Action != ActionDelete ||
			change.ResourceType != ResourceTypeIdentityPrincipal && change.ResourceType != ResourceTypeIdentityDirectory {
			continue
		}
		directoryID, principalID := change.ResourceID, ""
		if change.ResourceType == ResourceTypeIdentityPrincipal {
			if change.Parent == nil || change.Parent.ID == "" {
				return fmt.Errorf("principal deletion requires its directory")
			}
			directoryID, principalID = change.Parent.ID, change.ResourceID
		}
		if !observed {
			var err error
			links, err = p.client.ListRegistryAgentPrincipalReferences(ctx)
			if err != nil {
				return err
			}
			observed = true
		}
		if err := state.RequireEmptyIdentityRegistryReferences(links, directoryID, principalID); err != nil {
			return fmt.Errorf("cannot delete %s %q: %w", change.ResourceType, change.ResourceRef, err)
		}
	}
	return nil
}
