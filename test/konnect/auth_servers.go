package konnect

import (
	"context"
	"fmt"
	"maps"
	"slices"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type AuthServerAPI struct {
	Servers          []kkComps.AuthServer
	Writes           []kkComps.UpdateAuthServer
	Creates, Deletes int
	Force            *kkOps.DeleteAuthServerForce
	List             func(kkOps.ListAuthServersRequest) (*kkOps.ListAuthServersResponse, error)
}

func AuthServer(name, namespace string, protected bool) kkComps.AuthServer {
	value := kkComps.AuthServer{
		ID: "a362d8c9-2f26-4893-95a9-d8c279df31c2", Name: name, Audience: "https://api.example.test",
		Description: "Retained description", SigningAlgorithm: new(kkComps.AuthServerSigningAlgorithmRs256),
		TrustedOrigins: []string{"https://client.example.test"},
		Dcr:            &kkComps.DcrSettings{DefaultAccessTokenDuration: new(int64(600))},
		Labels:         map[string]string{"owner": "test"},
	}
	if namespace != "" {
		value.Labels["KONGCTL-namespace"] = namespace
		value.Labels["KONGCTL-protected"] = "false"
		if protected {
			value.Labels["KONGCTL-protected"] = "true"
		}
	}
	return value
}

func (a *AuthServerAPI) ListAuthServers(
	_ context.Context, request kkOps.ListAuthServersRequest, _ ...kkOps.Option,
) (*kkOps.ListAuthServersResponse, error) {
	if a.List != nil {
		return a.List(request)
	}
	values := slices.Clone(a.Servers)
	if values == nil {
		values = []kkComps.AuthServer{}
	}
	return &kkOps.ListAuthServersResponse{AuthServerList: &kkComps.AuthServerList{
		Data: values,
		Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Number: 1, Size: 100, Total: float64(len(values))}},
	}}, nil
}

func (a *AuthServerAPI) GetAuthServer(
	_ context.Context, id string, _ ...kkOps.Option,
) (*kkOps.GetAuthServerResponse, error) {
	for _, value := range a.Servers {
		if value.ID == id {
			return &kkOps.GetAuthServerResponse{AuthServer: &value}, nil
		}
	}
	return nil, fmt.Errorf("auth-server fixture not found")
}

func (a *AuthServerAPI) CreateAuthServer(
	_ context.Context, request kkComps.CreateAuthServer, _ ...kkOps.Option,
) (*kkOps.CreateAuthServerResponse, error) {
	a.Creates++
	value := AuthServer(request.Name, "", false)
	value.ID = fmt.Sprintf("00000000-0000-4000-8000-%012d", a.Creates)
	value.Audience = request.Audience
	value.Labels = maps.Clone(request.Labels)
	if request.Description != nil {
		value.Description = *request.Description
	}
	if request.SigningAlgorithm != nil {
		value.SigningAlgorithm = request.SigningAlgorithm
	}
	if request.TrustedOrigins != nil {
		value.TrustedOrigins = slices.Clone(request.TrustedOrigins)
	}
	if request.Dcr != nil && request.Dcr.DefaultAccessTokenDuration != nil {
		value.Dcr = &kkComps.DcrSettings{DefaultAccessTokenDuration: request.Dcr.DefaultAccessTokenDuration}
	}
	a.Servers = append(a.Servers, value)
	return &kkOps.CreateAuthServerResponse{AuthServer: &value}, nil
}

func (a *AuthServerAPI) UpdateAuthServer(
	_ context.Context, id string, request kkComps.UpdateAuthServer, _ ...kkOps.Option,
) (*kkOps.UpdateAuthServerResponse, error) {
	a.Writes = append(a.Writes, request)
	for i := range a.Servers {
		value := &a.Servers[i]
		if value.ID != id {
			continue
		}
		if request.Name != nil {
			value.Name = *request.Name
		}
		if request.Description != nil {
			value.Description = *request.Description
		}
		if request.Audience != nil {
			value.Audience = *request.Audience
		}
		if request.SigningAlgorithm != nil {
			value.SigningAlgorithm = new(kkComps.AuthServerSigningAlgorithm(*request.SigningAlgorithm))
		}
		if request.TrustedOrigins != nil {
			value.TrustedOrigins = slices.Clone(request.TrustedOrigins)
		}
		if request.Dcr != nil && request.Dcr.DefaultAccessTokenDuration != nil {
			value.Dcr = &kkComps.DcrSettings{DefaultAccessTokenDuration: request.Dcr.DefaultAccessTokenDuration}
		}
		for key, entry := range request.Labels {
			if entry == nil {
				delete(value.Labels, key)
			} else {
				value.Labels[key] = *entry
			}
		}
		return &kkOps.UpdateAuthServerResponse{AuthServer: value}, nil
	}
	return nil, fmt.Errorf("auth-server fixture not found")
}

func (a *AuthServerAPI) DeleteAuthServer(
	_ context.Context, id string, force *kkOps.DeleteAuthServerForce, _ ...kkOps.Option,
) (*kkOps.DeleteAuthServerResponse, error) {
	a.Deletes++
	a.Force = force
	for i, value := range a.Servers {
		if value.ID == id {
			a.Servers = append(a.Servers[:i], a.Servers[i+1:]...)
			break
		}
	}
	return &kkOps.DeleteAuthServerResponse{}, nil
}

var _ helpers.IdentityAuthServersAPI = (*AuthServerAPI)(nil)
