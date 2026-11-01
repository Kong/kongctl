package planner

import (
	"context"
	"fmt"
)

// Explicit independent detachments may precede trust retirement. A trust
// declaration never grants authority over an associated principal or identity.
func (p *Planner) resolveIdentityTrustedIDPDeletes(ctx context.Context, plan *Plan) error {
	for i := range plan.Changes {
		trust := &plan.Changes[i]
		if trust.ResourceType != ResourceTypeIdentityTrustedIDP || trust.Action != ActionDelete {
			continue
		}
		if trust.Parent == nil || trust.Parent.ID == "" {
			return fmt.Errorf("trusted IdP deletion requires its directory")
		}
		issuer, ok := trust.Fields[FieldIssuerURL].(string)
		if !ok || issuer == "" {
			return fmt.Errorf("trusted IdP deletion requires its exact issuer")
		}
		associated, err := p.client.ListIdentityTrustedIDPPrincipals(ctx, trust.Parent.ID, trust.ResourceID)
		if err != nil {
			return fmt.Errorf("cannot verify associated principals before deleting trusted IdP: %w", err)
		}
		for _, principal := range associated {
			principalDeleted := false
			for _, change := range plan.Changes {
				if change.ResourceType == ResourceTypeIdentityPrincipal && change.Action == ActionDelete &&
					change.ResourceID == principal.ID && change.Parent != nil && change.Parent.ID == trust.Parent.ID {
					trust.DependsOn = appendDependsOn(trust.DependsOn, change.ID)
					principalDeleted = true
				}
			}
			if principalDeleted {
				continue
			}
			identities, err := p.listPrincipalIdentities(ctx, trust.Parent.ID, principal.ID)
			if err != nil {
				return err
			}
			found := false
			for _, identity := range identities {
				if identity.Type != "oidc" || identity.Issuer == nil || *identity.Issuer != issuer {
					continue
				}
				found = true
				detached := false
				for _, change := range plan.Changes {
					if change.ResourceType != ResourceTypeIdentityPrincipalIdentity || change.ResourceID != identity.ID ||
						change.Parent == nil ||
						change.Parent.ID != principal.ID ||
						change.References[FieldDirectoryID].ID != trust.Parent.ID {
						continue
					}
					if change.Action == ActionDelete {
						detached = true
					} else if change.Action == ActionUpdate && change.Fields[FieldType] == "oidc" {
						next, ok := change.Fields[FieldIssuer].(string)
						detached = ok && next != "" && next != issuer
					}
					if detached {
						trust.DependsOn = appendDependsOn(trust.DependsOn, change.ID)
						break
					}
				}
				if !detached {
					return fmt.Errorf(
						"trusted IdP has associated principals; explicitly detach every matching OIDC identity before deletion",
					)
				}
			}
			if !found {
				return fmt.Errorf(
					"trusted IdP has an associated principal without observable OIDC detachment; remove the association explicitly",
				)
			}
		}
		for _, change := range plan.Changes {
			if change.ResourceType == ResourceTypeIdentityPrincipalIdentity && change.Action != ActionDelete &&
				change.References[FieldDirectoryID].ID == trust.Parent.ID &&
				change.Fields[FieldType] == "oidc" && change.Fields[FieldIssuer] == issuer {
				return fmt.Errorf("cannot attach an OIDC identity while deleting its Trusted IdP")
			}
		}
	}
	return nil
}
