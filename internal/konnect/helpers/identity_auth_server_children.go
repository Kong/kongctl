package helpers

import (
	"context"
	"fmt"
	"math"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/util"
)

type IdentityAuthServerScopesAPI interface {
	ListAuthServerScopes(context.Context, kkOps.ListAuthServerScopesRequest, ...kkOps.Option) (
		*kkOps.ListAuthServerScopesResponse, error,
	)
	GetAuthServerScope(context.Context, string, string, ...kkOps.Option) (*kkOps.GetAuthServerScopeResponse, error)
	CreateAuthServerScope(context.Context, string, kkComps.CreateScope, ...kkOps.Option) (
		*kkOps.CreateAuthServerScopeResponse, error,
	)
	UpdateAuthServerScope(context.Context, kkOps.UpdateAuthServerScopeRequest, ...kkOps.Option) (
		*kkOps.UpdateAuthServerScopeResponse, error,
	)
	DeleteAuthServerScope(context.Context, string, string, ...kkOps.Option) (*kkOps.DeleteAuthServerScopeResponse, error)
}

type IdentityAuthServerClientsReader interface {
	ListAuthServerClients(context.Context, kkOps.ListAuthServerClientsRequest, ...kkOps.Option) (
		*kkOps.ListAuthServerClientsResponse, error,
	)
}

func ListIdentityAuthServerClients(ctx context.Context, api IdentityAuthServerClientsReader,
	serverID string, pageSize int64,
) ([]kkComps.AuthServerClient, error) {
	if api == nil {
		return nil, fmt.Errorf("identity auth-server clients API is not configured")
	}
	return listAuthServerChildren(ctx, "clients", pageSize, true,
		func(ctx context.Context, number, size int64) ([]kkComps.AuthServerClient, *kkComps.PageMeta, error) {
			response, err := api.ListAuthServerClients(ctx, kkOps.ListAuthServerClientsRequest{
				AuthServerID: serverID, PageNumber: &number, PageSize: &size,
			})
			if err != nil || response == nil || response.ClientList == nil {
				return nil, nil, err
			}
			return response.ClientList.Data, &response.ClientList.Meta.Page, nil
		}, func(value kkComps.AuthServerClient) string { return value.ID })
}

type IdentityAuthServerClaimsAPI interface {
	ListAuthServerClaims(context.Context, kkOps.ListAuthServerClaimsRequest, ...kkOps.Option) (
		*kkOps.ListAuthServerClaimsResponse, error,
	)
	GetAuthServerClaim(context.Context, string, string, ...kkOps.Option) (*kkOps.GetAuthServerClaimResponse, error)
	CreateAuthServerClaim(context.Context, string, kkComps.CreateClaim, ...kkOps.Option) (
		*kkOps.CreateAuthServerClaimResponse, error,
	)
	UpdateAuthServerClaim(context.Context, kkOps.UpdateAuthServerClaimRequest, ...kkOps.Option) (
		*kkOps.UpdateAuthServerClaimResponse, error,
	)
	DeleteAuthServerClaim(context.Context, string, string, ...kkOps.Option) (*kkOps.DeleteAuthServerClaimResponse, error)
}

// List operations must return no usable partial inventory on any failed page.
func listAuthServerChildren[T any](ctx context.Context, kind string, pageSize int64, reliableTotal bool,
	page func(context.Context, int64, int64) ([]T, *kkComps.PageMeta, error), id func(T) string,
) ([]T, error) {
	if pageSize < 1 {
		pageSize = 100
	}
	result := make([]T, 0)
	seen := make(map[string]bool)
	for number := int64(1); ; number++ {
		values, meta, err := page(ctx, number, pageSize)
		if err != nil {
			return nil, fmt.Errorf("failed to list auth-server %s: %w", kind, err)
		}
		if values == nil || meta == nil {
			return nil, fmt.Errorf("missing auth-server %s collection response", kind)
		}
		if meta.Number != float64(number) || meta.Size < 1 || math.IsInf(meta.Size, 0) ||
			meta.Size != math.Trunc(meta.Size) || meta.Total < 0 || math.IsInf(meta.Total, 0) ||
			meta.Total != math.Trunc(meta.Total) || float64(len(values)) > meta.Size {
			return nil, fmt.Errorf("invalid auth-server %s pagination metadata", kind)
		}
		for _, value := range values {
			key := id(value)
			if key == "" || seen[key] {
				return nil, fmt.Errorf("auth-server %s collection has a missing or repeated ID", kind)
			}
			seen[key] = true
		}
		result = append(result, values...)
		if reliableTotal && float64(len(result)) > meta.Total {
			return nil, fmt.Errorf("inconsistent auth-server %s pagination total", kind)
		}
		if reliableTotal && float64(len(result)) == meta.Total {
			return result, nil
		}
		if len(values) == 0 {
			if !reliableTotal {
				return result, nil
			}
			return nil, fmt.Errorf("incomplete auth-server %s pagination", kind)
		}
	}
}

func ListIdentityAuthServerScopes(ctx context.Context, api IdentityAuthServerScopesAPI,
	serverID string, pageSize int64,
) ([]kkComps.AuthServerScope, error) {
	if api == nil {
		return nil, fmt.Errorf("identity auth-server scopes API is not configured")
	}
	return listAuthServerChildren(ctx, "scopes", pageSize, true,
		func(ctx context.Context, number, size int64) ([]kkComps.AuthServerScope, *kkComps.PageMeta, error) {
			response, err := api.ListAuthServerScopes(ctx, kkOps.ListAuthServerScopesRequest{
				AuthServerID: serverID, PageNumber: &number, PageSize: &size,
			})
			if err != nil || response == nil || response.ScopeList == nil {
				return nil, nil, err
			}
			return response.ScopeList.Data, &response.ScopeList.Meta.Page, nil
		}, func(value kkComps.AuthServerScope) string { return value.ID })
}

func ListIdentityAuthServerClaims(ctx context.Context, api IdentityAuthServerClaimsAPI,
	serverID string, pageSize int64,
) ([]kkComps.Claim, error) {
	if api == nil {
		return nil, fmt.Errorf("identity auth-server claims API is not configured")
	}
	// Q13: deployed totals count scope associations. Exhaust actual pages through
	// the terminal empty page rather than treating that total as a claim count.
	return listAuthServerChildren(ctx, "claims", pageSize, false,
		func(ctx context.Context, number, size int64) ([]kkComps.Claim, *kkComps.PageMeta, error) {
			response, err := api.ListAuthServerClaims(ctx, kkOps.ListAuthServerClaimsRequest{
				AuthServerID: serverID, PageNumber: &number, PageSize: &size,
			})
			if err != nil || response == nil || response.ClaimList == nil {
				return nil, nil, err
			}
			return response.ClaimList.Data, &response.ClaimList.Meta.Page, nil
		}, func(value kkComps.Claim) string { return value.ID })
}

func authServerChildID[T any](identifier, kind string, values []T, id, name func(T) string) (string, error) {
	var result string
	for _, value := range values {
		if name(value) != identifier {
			continue
		}
		if result != "" {
			return "", fmt.Errorf("multiple auth-server %s have name %q; use an ID", kind, identifier)
		}
		result = id(value)
	}
	if result == "" {
		return "", fmt.Errorf("auth-server %s %q not found", kind, identifier)
	}
	return result, nil
}

func ResolveIdentityAuthServerScope(ctx context.Context, api IdentityAuthServerScopesAPI,
	serverID, identifier string, pageSize int64,
) (*kkComps.AuthServerScope, error) {
	if api == nil {
		return nil, fmt.Errorf("identity auth-server scopes API is not configured")
	}
	id := identifier
	if !util.IsValidUUID(identifier) {
		values, err := ListIdentityAuthServerScopes(ctx, api, serverID, pageSize)
		if err != nil {
			return nil, err
		}
		id, err = authServerChildID(identifier, "scopes", values,
			func(v kkComps.AuthServerScope) string { return v.ID },
			func(v kkComps.AuthServerScope) string { return v.Name })
		if err != nil {
			return nil, err
		}
	}
	response, err := api.GetAuthServerScope(ctx, serverID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth-server scope: %w", err)
	}
	if response == nil || response.AuthServerScope == nil || response.AuthServerScope.ID != id ||
		(!util.IsValidUUID(identifier) && response.AuthServerScope.Name != identifier) {
		return nil, fmt.Errorf("missing or inconsistent auth-server scope response")
	}
	return response.AuthServerScope, nil
}

func ResolveIdentityAuthServerClaim(ctx context.Context, api IdentityAuthServerClaimsAPI,
	serverID, identifier string, pageSize int64,
) (*kkComps.Claim, error) {
	if api == nil {
		return nil, fmt.Errorf("identity auth-server claims API is not configured")
	}
	id := identifier
	if !util.IsValidUUID(identifier) {
		values, err := ListIdentityAuthServerClaims(ctx, api, serverID, pageSize)
		if err != nil {
			return nil, err
		}
		id, err = authServerChildID(identifier, "claims", values,
			func(v kkComps.Claim) string { return v.ID }, func(v kkComps.Claim) string { return v.Name })
		if err != nil {
			return nil, err
		}
	}
	response, err := api.GetAuthServerClaim(ctx, serverID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth-server claim: %w", err)
	}
	if response == nil || response.Claim == nil || response.Claim.ID != id ||
		(!util.IsValidUUID(identifier) && response.Claim.Name != identifier) {
		return nil, fmt.Errorf("missing or inconsistent auth-server claim response")
	}
	return response.Claim, nil
}
