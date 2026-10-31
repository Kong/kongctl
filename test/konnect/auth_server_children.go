package konnect

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	kkErrors "github.com/Kong/sdk-konnect-go/models/sdkerrors"
)

type AuthServerChildrenAPI struct {
	Scopes          map[string][]kkComps.AuthServerScope
	Claims          map[string][]kkComps.Claim
	Clients         map[string][]kkComps.AuthServerClient
	ScopeUpdates    []kkOps.UpdateAuthServerScopeRequest
	ClaimUpdates    []kkOps.UpdateAuthServerClaimRequest
	Deleted         []string
	Writes, created int
}

func fixturePage[T any](all []T, number, size *int64) ([]T, kkComps.PaginatedMeta) {
	page, count := int64(1), int64(100)
	if number != nil {
		page = *number
	}
	if size != nil {
		count = *size
	}
	start := min(int64(len(all)), (page-1)*count)
	end := min(int64(len(all)), start+count)
	data := append(make([]T, 0, end-start), all[start:end]...)
	return data, kkComps.PaginatedMeta{Page: kkComps.PageMeta{
		Number: float64(page), Size: float64(count), Total: float64(len(all)),
	}}
}

func patchAuthChild[T any](current T, request any) T {
	before, _ := json.Marshal(current)
	after, _ := json.Marshal(request)
	var fields, update map[string]json.RawMessage
	_ = json.Unmarshal(before, &fields)
	_ = json.Unmarshal(after, &update)
	maps.Copy(fields, update)
	data, _ := json.Marshal(fields)
	var result T
	_ = json.Unmarshal(data, &result)
	return result
}

func (a *AuthServerChildrenAPI) ListAuthServerScopes(_ context.Context,
	request kkOps.ListAuthServerScopesRequest, _ ...kkOps.Option,
) (*kkOps.ListAuthServerScopesResponse, error) {
	data, meta := fixturePage(a.Scopes[request.AuthServerID], request.PageNumber, request.PageSize)
	return &kkOps.ListAuthServerScopesResponse{ScopeList: &kkComps.ScopeList{Data: data, Meta: meta}}, nil
}

func (a *AuthServerChildrenAPI) GetAuthServerScope(_ context.Context, parent, id string, _ ...kkOps.Option) (
	*kkOps.GetAuthServerScopeResponse, error,
) {
	for _, value := range a.Scopes[parent] {
		if value.ID == id {
			return &kkOps.GetAuthServerScopeResponse{AuthServerScope: &value}, nil
		}
	}
	return nil, &kkErrors.NotFoundError{Status: 404}
}

func (a *AuthServerChildrenAPI) CreateAuthServerScope(_ context.Context, parent string,
	request kkComps.CreateScope, _ ...kkOps.Option,
) (*kkOps.CreateAuthServerScopeResponse, error) {
	a.created++
	a.Writes++
	if a.Scopes == nil {
		a.Scopes = make(map[string][]kkComps.AuthServerScope)
	}
	data, _ := json.Marshal(request)
	var value kkComps.AuthServerScope
	_ = json.Unmarshal(data, &value)
	value.ID = fmt.Sprintf("10000000-0000-4000-8000-%012d", a.created)
	a.Scopes[parent] = append(a.Scopes[parent], value)
	return &kkOps.CreateAuthServerScopeResponse{AuthServerScope: &value}, nil
}

func (a *AuthServerChildrenAPI) UpdateAuthServerScope(_ context.Context,
	request kkOps.UpdateAuthServerScopeRequest, _ ...kkOps.Option,
) (*kkOps.UpdateAuthServerScopeResponse, error) {
	a.ScopeUpdates = append(a.ScopeUpdates, request)
	for i, value := range a.Scopes[request.AuthServerID] {
		if value.ID == request.ScopeID {
			next := patchAuthChild(value, request.UpdateScope)
			a.Scopes[request.AuthServerID][i] = next
			a.Writes++
			return &kkOps.UpdateAuthServerScopeResponse{AuthServerScope: &next}, nil
		}
	}
	return nil, &kkErrors.NotFoundError{Status: 404}
}

func (a *AuthServerChildrenAPI) DeleteAuthServerScope(_ context.Context, parent, id string, _ ...kkOps.Option) (
	*kkOps.DeleteAuthServerScopeResponse, error,
) {
	a.Deleted = append(a.Deleted, id)
	a.Writes++
	a.Scopes[parent] = slices.DeleteFunc(a.Scopes[parent], func(v kkComps.AuthServerScope) bool { return v.ID == id })
	return &kkOps.DeleteAuthServerScopeResponse{}, nil
}

func (a *AuthServerChildrenAPI) ListAuthServerClaims(_ context.Context,
	request kkOps.ListAuthServerClaimsRequest, _ ...kkOps.Option,
) (*kkOps.ListAuthServerClaimsResponse, error) {
	data, meta := fixturePage(a.Claims[request.AuthServerID], request.PageNumber, request.PageSize)
	total := 0
	for _, value := range a.Claims[request.AuthServerID] {
		total += max(1, len(value.IncludeInScopes))
	}
	meta.Page.Total = float64(total)
	return &kkOps.ListAuthServerClaimsResponse{ClaimList: &kkComps.ClaimList{Data: data, Meta: meta}}, nil
}

func (a *AuthServerChildrenAPI) GetAuthServerClaim(_ context.Context, parent, id string, _ ...kkOps.Option) (
	*kkOps.GetAuthServerClaimResponse, error,
) {
	for _, value := range a.Claims[parent] {
		if value.ID == id {
			return &kkOps.GetAuthServerClaimResponse{Claim: &value}, nil
		}
	}
	return nil, &kkErrors.NotFoundError{Status: 404}
}

func (a *AuthServerChildrenAPI) CreateAuthServerClaim(_ context.Context, parent string,
	request kkComps.CreateClaim, _ ...kkOps.Option,
) (*kkOps.CreateAuthServerClaimResponse, error) {
	a.created++
	a.Writes++
	if a.Claims == nil {
		a.Claims = make(map[string][]kkComps.Claim)
	}
	data, _ := json.Marshal(request)
	var value kkComps.Claim
	_ = json.Unmarshal(data, &value)
	value.ID = fmt.Sprintf("10000000-0000-4000-8000-%012d", a.created)
	a.Claims[parent] = append(a.Claims[parent], value)
	return &kkOps.CreateAuthServerClaimResponse{Claim: &value}, nil
}

func (a *AuthServerChildrenAPI) UpdateAuthServerClaim(_ context.Context,
	request kkOps.UpdateAuthServerClaimRequest, _ ...kkOps.Option,
) (*kkOps.UpdateAuthServerClaimResponse, error) {
	a.ClaimUpdates = append(a.ClaimUpdates, request)
	for i, value := range a.Claims[request.AuthServerID] {
		if value.ID == request.ClaimID {
			next := patchAuthChild(value, request.UpdateClaim)
			a.Claims[request.AuthServerID][i] = next
			a.Writes++
			return &kkOps.UpdateAuthServerClaimResponse{Claim: &next}, nil
		}
	}
	return nil, &kkErrors.NotFoundError{Status: 404}
}

func (a *AuthServerChildrenAPI) DeleteAuthServerClaim(_ context.Context, parent, id string, _ ...kkOps.Option) (
	*kkOps.DeleteAuthServerClaimResponse, error,
) {
	a.Deleted = append(a.Deleted, id)
	a.Writes++
	a.Claims[parent] = slices.DeleteFunc(a.Claims[parent], func(v kkComps.Claim) bool { return v.ID == id })
	return &kkOps.DeleteAuthServerClaimResponse{}, nil
}

func (a *AuthServerChildrenAPI) ListAuthServerClients(_ context.Context,
	request kkOps.ListAuthServerClientsRequest, _ ...kkOps.Option,
) (*kkOps.ListAuthServerClientsResponse, error) {
	data, meta := fixturePage(a.Clients[request.AuthServerID], request.PageNumber, request.PageSize)
	return &kkOps.ListAuthServerClientsResponse{ClientList: &kkComps.ClientList{Data: data, Meta: meta}}, nil
}
