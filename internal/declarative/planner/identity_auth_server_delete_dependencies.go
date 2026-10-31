package planner

import (
	"context"
	"fmt"
	"slices"
)

func (p *Planner) resolveIdentityAuthServerDeletes(ctx context.Context, plan *Plan) error {
	for i := range plan.Changes {
		deletion := &plan.Changes[i]
		if deletion.ResourceType != ResourceTypeIdentityAuthServerScope || deletion.Action != ActionDelete {
			continue
		}
		if deletion.Parent == nil || deletion.Parent.ID == "" {
			return fmt.Errorf("scope delete requires a known auth server")
		}
		parentID := deletion.Parent.ID
		clients, err := p.listIdentityAuthServerClients(ctx, parentID)
		if err != nil {
			return err
		}
		for _, client := range clients {
			if slices.Contains(client.AllowScopes, deletion.ResourceID) {
				return fmt.Errorf("cannot delete scope %q: still referenced by client %q", deletion.ResourceRef, client.Name)
			}
		}
		claims, err := p.listIdentityAuthServerClaims(ctx, parentID)
		if err != nil {
			return err
		}
		for _, claim := range claims {
			if !slices.Contains(claim.IncludeInScopes, deletion.ResourceID) {
				continue
			}
			detached := false
			for _, change := range plan.Changes {
				if change.ResourceType != ResourceTypeIdentityAuthServerClaim || change.ResourceID != claim.ID ||
					change.Parent == nil || change.Parent.ID != parentID {
					continue
				}
				if change.Action == ActionDelete {
					deletion.DependsOn = appendDependsOn(deletion.DependsOn, change.ID)
					detached = true
					break
				}
				if values, explicit := change.Fields[FieldIncludeInScopes]; change.Action == ActionUpdate && explicit {
					if !authClaimChangeUsesScope(change, deletion.ResourceID, values) {
						deletion.DependsOn = appendDependsOn(deletion.DependsOn, change.ID)
						detached = true
						break
					}
				}
			}
			if !detached {
				return fmt.Errorf("cannot delete scope %q: still referenced by claim %q", deletion.ResourceRef, claim.Name)
			}
		}
		for _, change := range plan.Changes {
			if change.ResourceType != ResourceTypeIdentityAuthServerClaim || change.Action == ActionDelete ||
				change.Parent == nil || change.Parent.ID != parentID {
				continue
			}
			if values, explicit := change.Fields[FieldIncludeInScopes]; explicit &&
				authClaimChangeUsesScope(change, deletion.ResourceID, values) {
				return fmt.Errorf("cannot delete scope %q: referenced by planned claim %q",
					deletion.ResourceRef, change.ResourceRef)
			}
		}
	}
	return nil
}

func authClaimChangeUsesScope(change PlannedChange, id string, raw any) bool {
	var values []string
	switch list := raw.(type) {
	case []string:
		values = list
	case []any:
		for _, value := range list {
			value, ok := value.(string)
			if !ok {
				return true
			}
			values = append(values, value)
		}
	default:
		return true
	}
	for i, value := range values {
		reference, ok := change.References[fmt.Sprintf("%s.%d", FieldIncludeInScopes, i)]
		if ok && reference.HasResolvedID() {
			value = reference.ID
		}
		if value == id {
			return true
		}
	}
	return false
}
