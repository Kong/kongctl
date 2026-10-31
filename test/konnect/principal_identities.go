package konnect

import (
	"context"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

// PrincipalIdentitiesAPI supplies complete child observations for parent tests.
type PrincipalIdentitiesAPI struct {
	helpers.IdentityPrincipalIdentitiesAPI
	Data     []kkComps.KongPrincipalIdentity
	Error    error
	Requests []kkOps.ListIdentitiesRequest
}

func (a *PrincipalIdentitiesAPI) ListIdentities(_ context.Context, request kkOps.ListIdentitiesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListIdentitiesResponse, error) {
	a.Requests = append(a.Requests, request)
	if a.Error != nil {
		return nil, a.Error
	}
	data := a.Data
	if data == nil {
		data = []kkComps.KongPrincipalIdentity{}
	}
	return &kkOps.ListIdentitiesResponse{
		ListKongPrincipalIdentities: &kkComps.ListKongPrincipalIdentities{Data: data},
	}, nil
}
