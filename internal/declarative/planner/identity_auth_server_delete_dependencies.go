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
				if !authScopeDetachment(
					plan,
					deletion,
					ResourceTypeIdentityAuthServerClient,
					client.ID,
					FieldAllowScopes,
				) {
					return fmt.Errorf(
						"cannot delete scope %q: still referenced by client %q",
						deletion.ResourceRef,
						client.Name,
					)
				}
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
			if !authScopeDetachment(
				plan,
				deletion,
				ResourceTypeIdentityAuthServerClaim,
				claim.ID,
				FieldIncludeInScopes,
			) {
				return fmt.Errorf(
					"cannot delete scope %q: still referenced by claim %q",
					deletion.ResourceRef,
					claim.Name,
				)
			}
		}
		for _, change := range plan.Changes {
			var field string
			switch change.ResourceType {
			case ResourceTypeIdentityAuthServerClaim:
				field = FieldIncludeInScopes
			case ResourceTypeIdentityAuthServerClient:
				field = FieldAllowScopes
			default:
				continue
			}
			if change.Action == ActionDelete || change.Parent == nil || change.Parent.ID != parentID {
				continue
			}
			if values, explicit := change.Fields[field]; explicit &&
				authChangeUsesScope(change, field, deletion.ResourceID, values) {
				return fmt.Errorf(
					"cannot delete scope %q: referenced by planned %s %q",
					deletion.ResourceRef,
					change.ResourceType,
					change.ResourceRef,
				)
			}
		}
	}
	return nil
}

func authChangeUsesScope(change PlannedChange, field, id string, raw any) bool {
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
		reference, ok := change.References[fmt.Sprintf("%s.%d", field, i)]
		if ok && reference.HasResolvedID() {
			value = reference.ID
		}
		if value == id {
			return true
		}
	}
	return false
}

// Only an explicit, resolved PATCH list or deletion proves detachment.
func authScopeDetachment(plan *Plan, deletion *PlannedChange, kind, id, field string) bool {
	for _, change := range plan.Changes {
		if change.ResourceType != kind || change.ResourceID != id || change.Parent == nil ||
			change.Parent.ID != deletion.Parent.ID {
			continue
		}
		if change.Action == ActionDelete {
			deletion.DependsOn = appendDependsOn(deletion.DependsOn, change.ID)
			return true
		}
		if raw, explicit := change.Fields[field]; change.Action == ActionUpdate && explicit &&
			!authChangeUsesScope(change, field, deletion.ResourceID, raw) {
			deletion.DependsOn = appendDependsOn(deletion.DependsOn, change.ID)
			return true
		}
	}
	return false
}

func adjustIdentityAuthServerDependencies(changes []PlannedChange) {
	for i := range changes {
		root := &changes[i]
		if root.ResourceType != ResourceTypeIdentityAuthServer {
			continue
		}
		for j := range changes {
			child := &changes[j]
			if child.Parent == nil {
				continue
			}
			switch child.ResourceType {
			case ResourceTypeIdentityAuthServerClient,
				ResourceTypeIdentityAuthServerScope,
				ResourceTypeIdentityAuthServerClaim:
			default:
				continue
			}
			if root.Action == ActionCreate && child.Parent.Ref == root.ResourceRef {
				child.DependsOn = appendDependsOn(child.DependsOn, root.ID)
			}
			if root.Action == ActionDelete && child.Action == ActionDelete && child.Parent.ID == root.ResourceID {
				root.DependsOn = appendDependsOn(root.DependsOn, child.ID)
			}
		}
	}
}
