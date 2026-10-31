package helpers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

type clientsReadAPI struct {
	helpers.IdentityAuthServerClientsAPI
	list    func(kkOps.ListAuthServerClientsRequest) (*kkOps.ListAuthServerClientsResponse, error)
	get     func(string, string) (*kkOps.GetAuthServerClientResponse, error)
	secrets func(kkOps.ListClientSecretsRequest) (*kkOps.ListClientSecretsResponse, error)
	secret  func(kkOps.GetClientSecretRequest) (*kkOps.GetClientSecretResponse, error)
}

func (a *clientsReadAPI) ListAuthServerClients(_ context.Context,
	request kkOps.ListAuthServerClientsRequest, _ ...kkOps.Option,
) (*kkOps.ListAuthServerClientsResponse, error) {
	return a.list(request)
}

func (a *clientsReadAPI) GetAuthServerClient(_ context.Context,
	server, id string, _ ...kkOps.Option,
) (*kkOps.GetAuthServerClientResponse, error) {
	return a.get(server, id)
}

func (a *clientsReadAPI) ListClientSecrets(_ context.Context,
	request kkOps.ListClientSecretsRequest, _ ...kkOps.Option,
) (*kkOps.ListClientSecretsResponse, error) {
	return a.secrets(request)
}

func (a *clientsReadAPI) GetClientSecret(_ context.Context,
	request kkOps.GetClientSecretRequest, _ ...kkOps.Option,
) (*kkOps.GetClientSecretResponse, error) {
	return a.secret(request)
}

func TestClientLookupKeepsOpaqueIDsDistinctFromNames(t *testing.T) {
	for _, fault := range []string{"", "duplicate-name", "changed-name", "wrong-id"} {
		t.Run(fault, func(t *testing.T) {
			listCalls, getCalls := 0, 0
			api := &clientsReadAPI{
				list: func(request kkOps.ListAuthServerClientsRequest) (*kkOps.ListAuthServerClientsResponse, error) {
					listCalls++
					require.Equal(t, "server", request.AuthServerID)
					require.Nil(t, request.Filter)
					data := []kkComps.AuthServerClient{{ID: "opaque-id", Name: "machine"}}
					if fault == "duplicate-name" {
						data = append(data, kkComps.AuthServerClient{ID: "other-id", Name: "machine"})
					}
					if *request.PageNumber > 1 {
						data = []kkComps.AuthServerClient{}
					}
					return &kkOps.ListAuthServerClientsResponse{ClientList: &kkComps.ClientList{
						Data: data, Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{
							Number: float64(*request.PageNumber), Size: 100, Total: float64(len(data)),
						}},
					}}, nil
				},
				get: func(server, id string) (*kkOps.GetAuthServerClientResponse, error) {
					getCalls++
					require.Equal(t, "server", server)
					value := &kkComps.AuthServerClient{ID: id, Name: "machine"}
					if fault == "changed-name" {
						value.Name = "renamed"
					}
					if fault == "wrong-id" {
						value.ID = "wrong"
					}
					return &kkOps.GetAuthServerClientResponse{AuthServerClient: value}, nil
				},
			}
			value, err := helpers.ResolveIdentityAuthServerClient(t.Context(), api, "server", "machine", true, 100)
			if fault == "" {
				require.NoError(t, err)
				require.Equal(t, "opaque-id", value.ID)
			} else {
				require.Error(t, err)
				require.Nil(t, value)
			}
			require.Equal(t, 2, listCalls)
			if fault == "duplicate-name" {
				require.Zero(t, getCalls)
			}
			listCalls = 0
			_, err = helpers.ResolveIdentityAuthServerClient(t.Context(), api, "server", "machine", false, 100)
			if fault != "wrong-id" {
				require.NoError(t, err)
			}
			require.Zero(t, listCalls, "positional opaque IDs never fall back to a name lookup")
		})
	}
}

func TestClientSecretReadsNeverExposeValuesAndFailClosedOnIncompletePages(t *testing.T) {
	for _, fault := range []string{"", "last-page-error", "missing-page", "repeat-id"} {
		t.Run(fault, func(t *testing.T) {
			api := &clientsReadAPI{
				secrets: func(request kkOps.ListClientSecretsRequest) (*kkOps.ListClientSecretsResponse, error) {
					require.Equal(t, "server", request.AuthServerID)
					require.Equal(t, "opaque-id", request.ClientID)
					require.Nil(t, request.Filter)
					number := *request.PageNumber
					if number == 2 && fault == "last-page-error" {
						return nil, fmt.Errorf("late read failure")
					}
					if fault == "missing-page" {
						return nil, nil
					}
					id := fmt.Sprint(number)
					if fault == "repeat-id" {
						id = "repeated"
					}
					return &kkOps.ListClientSecretsResponse{ClientSecretList: &kkComps.ClientSecretList{
						Data: []kkComps.ClientSecretItem{{ID: id, Enabled: new(false), Secret: new("private-secret")}},
						Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Number: float64(number), Size: 1, Total: 2}},
					}}, nil
				},
				secret: func(request kkOps.GetClientSecretRequest) (*kkOps.GetClientSecretResponse, error) {
					require.Equal(t, "server", request.AuthServerID)
					require.Equal(t, "opaque-id", request.ClientID)
					return &kkOps.GetClientSecretResponse{ClientSecretItem: &kkComps.ClientSecretItem{
						ID: request.ClientSecretID, Enabled: new(false), Secret: new("private-secret"),
					}}, nil
				},
			}
			values, err := helpers.ListIdentityClientSecrets(t.Context(), api, "server", "opaque-id", 1)
			if fault != "" {
				require.Error(t, err)
				require.Nil(t, values)
				return
			}
			require.NoError(t, err)
			require.Len(t, values, 2)
			value, err := helpers.GetIdentityClientSecret(t.Context(), api, "server", "opaque-id", "secret-id")
			require.NoError(t, err)
			require.NotNil(t, value.Enabled)
			require.False(t, *value.Enabled)
			for _, object := range []any{values, value} {
				for _, marshal := range []func(any) ([]byte, error){json.Marshal, yaml.Marshal} {
					encoded, err := marshal(object)
					require.NoError(t, err)
					require.NotContains(t, string(encoded), "private-secret")
					require.NotContains(t, string(encoded), "\"secret\":")
				}
			}
		})
	}
}

func TestClientPaginationIgnoresAssociationTotalsAndRejectsPartialResults(t *testing.T) {
	for _, fault := range []string{"", "under-count", "late-error", "repeat", "missing", "bad-page"} {
		t.Run(fault, func(t *testing.T) {
			var pages []int64
			api := &clientsReadAPI{list: func(request kkOps.ListAuthServerClientsRequest) (
				*kkOps.ListAuthServerClientsResponse, error,
			) {
				number := *request.PageNumber
				pages = append(pages, number)
				require.Equal(t, "server", request.AuthServerID)
				require.Nil(t, request.Filter)
				if number == 3 && fault == "late-error" {
					return nil, fmt.Errorf("failed terminal read")
				}
				if fault == "missing" {
					return nil, nil
				}
				data := []kkComps.AuthServerClient{}
				if number <= 2 {
					id := fmt.Sprint(number)
					if fault == "repeat" {
						id = "same"
					}
					data = append(data, kkComps.AuthServerClient{ID: id, AllowScopes: []string{"a", "b"}})
				}
				meta := kkComps.PageMeta{Number: float64(number), Size: 1, Total: 4}
				if fault == "under-count" {
					meta.Total = 0
				}
				if fault == "bad-page" {
					meta.Number = 0
				}
				return &kkOps.ListAuthServerClientsResponse{ClientList: &kkComps.ClientList{
					Data: data, Meta: kkComps.PaginatedMeta{Page: meta},
				}}, nil
			}}
			values, err := helpers.ListIdentityAuthServerClients(t.Context(), api, "server", 1)
			if fault != "" && fault != "under-count" {
				require.Error(t, err)
				require.Nil(t, values)
				return
			}
			require.NoError(t, err)
			require.Len(t, values, 2)
			require.Equal(t, []int64{1, 2, 3}, pages)
		})
	}
}
