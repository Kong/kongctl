package helpers_test

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestAuthServerCompletePaginationRejectsPartialResults(t *testing.T) {
	for _, fault := range []string{"", "repeat", "empty", "bad-meta", "missing"} {
		t.Run(fault, func(t *testing.T) {
			api := &fixture.AuthServerAPI{List: func(
				request kkOps.ListAuthServersRequest,
			) (*kkOps.ListAuthServersResponse, error) {
				page := *request.PageNumber
				value := fixture.AuthServer("server", "ns", false)
				if page == 2 && fault != "repeat" {
					value.ID = "second-id"
				}
				data := []kkComps.AuthServer{value}
				if page == 2 && fault == "empty" {
					data = []kkComps.AuthServer{}
				}
				if fault == "missing" {
					return nil, nil
				}
				number := float64(page)
				if fault == "bad-meta" {
					number = 0
				}
				return &kkOps.ListAuthServersResponse{AuthServerList: &kkComps.AuthServerList{
					Data: data,
					Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Number: number, Size: 1, Total: 2}},
				}}, nil
			}}
			values, err := helpers.ListIdentityAuthServers(t.Context(), api, 1)
			if fault == "" {
				require.NoError(t, err)
				require.Len(t, values, 2)
			} else {
				require.Error(t, err)
				require.Nil(t, values)
			}
		})
	}
}
