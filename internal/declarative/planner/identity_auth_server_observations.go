package planner

import (
	"context"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) listIdentityAuthServerScopes(
	ctx context.Context, parent string,
) ([]state.IdentityAuthServerScope, error) {
	if p.resourceCache != nil {
		if values, ok := p.resourceCache.identityAuthScopes[parent]; ok {
			return values, nil
		}
	}
	values, err := p.client.ListIdentityAuthServerScopes(ctx, parent)
	if err == nil && p.resourceCache != nil {
		p.resourceCache.identityAuthScopes[parent] = values
	}
	return values, err
}

func (p *Planner) listIdentityAuthServerClaims(
	ctx context.Context, parent string,
) ([]state.IdentityAuthServerClaim, error) {
	if p.resourceCache != nil {
		if values, ok := p.resourceCache.identityAuthClaims[parent]; ok {
			return values, nil
		}
	}
	values, err := p.client.ListIdentityAuthServerClaims(ctx, parent)
	if err == nil && p.resourceCache != nil {
		p.resourceCache.identityAuthClaims[parent] = values
	}
	return values, err
}

func (p *Planner) listIdentityAuthServerClients(
	ctx context.Context, parent string,
) ([]kkComps.AuthServerClient, error) {
	if p.resourceCache != nil {
		if values, ok := p.resourceCache.identityAuthClients[parent]; ok {
			return values, nil
		}
	}
	values, err := p.client.ListIdentityAuthServerClients(ctx, parent)
	if err == nil && p.resourceCache != nil {
		p.resourceCache.identityAuthClients[parent] = values
	}
	return values, err
}
