package helpers_test

import (
	"context"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

type claimsReadAPI struct {
	helpers.IdentityAuthServerClaimsAPI
	list func(kkOps.ListAuthServerClaimsRequest) (*kkOps.ListAuthServerClaimsResponse, error)
	get  func(string, string) (*kkOps.GetAuthServerClaimResponse, error)
}

func (a *claimsReadAPI) ListAuthServerClaims(_ context.Context,
	request kkOps.ListAuthServerClaimsRequest, _ ...kkOps.Option,
) (*kkOps.ListAuthServerClaimsResponse, error) {
	return a.list(request)
}

func (a *claimsReadAPI) GetAuthServerClaim(_ context.Context, serverID, id string, _ ...kkOps.Option) (
	*kkOps.GetAuthServerClaimResponse, error,
) {
	return a.get(serverID, id)
}

func TestClaimPaginationExhaustsActualPagesDespiteInflatedTotal(t *testing.T) {
	for _, fault := range []string{"", "late-error", "repeat", "missing", "bad-page", "under-count"} {
		t.Run(fault, func(t *testing.T) {
			var pages []int64
			api := &claimsReadAPI{list: func(request kkOps.ListAuthServerClaimsRequest) (
				*kkOps.ListAuthServerClaimsResponse, error,
			) {
				require.Equal(t, "server", request.AuthServerID)
				require.Nil(t, request.Filter)
				require.Equal(t, int64(1), *request.PageSize)
				number := *request.PageNumber
				pages = append(pages, number)
				if number == 3 && fault == "late-error" {
					return nil, fmt.Errorf("failed final page")
				}
				if fault == "missing" {
					return nil, nil
				}
				data := []kkComps.Claim{}
				if number <= 2 {
					id := fmt.Sprint(number)
					if fault == "repeat" {
						id = "same-id"
					}
					data = append(data, kkComps.Claim{ID: id, Name: "department"})
				}
				meta := kkComps.PageMeta{Number: float64(number), Size: 1, Total: 6}
				if fault == "bad-page" {
					meta.Number = 0
				}
				if fault == "under-count" {
					meta.Total = 0
				}
				return &kkOps.ListAuthServerClaimsResponse{ClaimList: &kkComps.ClaimList{
					Data: data, Meta: kkComps.PaginatedMeta{Page: meta},
				}}, nil
			}}
			values, err := helpers.ListIdentityAuthServerClaims(t.Context(), api, "server", 1)
			if fault != "" {
				require.Error(t, err)
				require.Nil(t, values, "never expose a partial result")
				return
			}
			require.NoError(t, err)
			require.Len(t, values, 2)
			require.Equal(t, []int64{1, 2, 3}, pages)
		})
	}
}

func TestClaimNameLookupRejectsDuplicatesAndUUIDReadsUseTheSelectedParent(t *testing.T) {
	id := "731559d3-39bd-48c7-b33c-eb192847eb20"
	getCalls := 0
	api := &claimsReadAPI{
		list: func(request kkOps.ListAuthServerClaimsRequest) (*kkOps.ListAuthServerClaimsResponse, error) {
			data := []kkComps.Claim{}
			if *request.PageNumber == 1 {
				data = []kkComps.Claim{{ID: id, Name: "department"}, {ID: "other-id", Name: "department"}}
			}
			return &kkOps.ListAuthServerClaimsResponse{ClaimList: &kkComps.ClaimList{
				Data: data,
				Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{
					Number: float64(*request.PageNumber), Size: 100, Total: 2,
				}},
			}}, nil
		},
		get: func(server, claimID string) (*kkOps.GetAuthServerClaimResponse, error) {
			getCalls++
			require.Equal(t, "selected-parent", server)
			require.Equal(t, id, claimID)
			return &kkOps.GetAuthServerClaimResponse{Claim: &kkComps.Claim{ID: id, Name: "department"}}, nil
		},
	}
	_, err := helpers.ResolveIdentityAuthServerClaim(t.Context(), api, "selected-parent", "department", 100)
	require.ErrorContains(t, err, "multiple")
	require.Zero(t, getCalls)
	value, err := helpers.ResolveIdentityAuthServerClaim(t.Context(), api, "selected-parent", id, 100)
	require.NoError(t, err)
	require.Equal(t, id, value.ID)
	require.Equal(t, 1, getCalls)
}
