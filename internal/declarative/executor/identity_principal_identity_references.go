package executor

import (
	"context"
	"fmt"
	"net/http"

	declerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
)

func (e *Executor) validateIdentityReferenceScopes(ctx context.Context, target *planner.PlannedChange) error {
	if e.dryRun {
		return nil
	}
	if len(target.IdentityReferenceScopes) != 0 && unresolvedReferenceID(target.ResourceID) {
		return fmt.Errorf("identity reference checks require a resolved target ID; regenerate the plan")
	}
	parentID := ""
	if target.Parent != nil {
		parentID = target.Parent.ID
	}
	for _, scope := range target.IdentityReferenceScopes {
		all, err := e.client.ListIdentityPrincipalIdentities(ctx, scope.DirectoryID, scope.PrincipalID)
		if err != nil {
			// A preceding planned principal/directory deletion removes this
			// scoped collection. Other observation failures still block deletion.
			if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
				continue
			}
			return fmt.Errorf("cannot verify principal identity references before deletion: %w", err)
		}
		for _, observed := range all {
			identity := resources.IdentityPrincipalIdentityResource{
				Type:         observed.Type,
				AuthServerID: observed.AuthServerID, ClientID: observed.ClientID,
				ControlPlaneID: observed.ControlPlaneID, ConsumerID: observed.ConsumerID,
			}
			if identity.ReferencesTarget(resources.ResourceType(target.ResourceType), target.ResourceID, parentID) {
				return fmt.Errorf(
					"%s is still referenced by principal identity %s; regenerate the plan",
					target.ResourceType,
					observed.ID,
				)
			}
		}
	}
	return nil
}
