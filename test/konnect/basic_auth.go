package konnect

import (
	"context"
	"fmt"
	"maps"
	"slices"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/google/uuid"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type BasicAuthAPI struct {
	helpers.IdentityPrincipalKeysReader
	Parents          map[string][]helpers.IdentityBasicAuth
	Passwords        map[string][]helpers.IdentityPassword
	Keys             map[string][]string
	Writes           []kkComps.ReplaceBasicAuthBody
	Creates, Deletes int
	Failure          string
}

func BasicAuth(username, namespace string, protected bool) helpers.IdentityBasicAuth {
	return helpers.IdentityBasicAuth{
		ID: uuid.NewSHA1(uuid.NameSpaceDNS, []byte(username+namespace)).String(), Username: username,
		ManagedBy: resources.BuildChildManagedBy(namespace, protected, "old"),
	}
}

func (a *BasicAuthAPI) ListPrincipalBasicAuths(_ context.Context, req kkOps.ListPrincipalBasicAuthsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListPrincipalBasicAuthsResponse, error) {
	if a.Failure == "list" {
		return nil, fmt.Errorf("incomplete basic-auth observation")
	}
	values := a.Parents[req.DirectoryID+"/"+req.PrincipalID]
	if values == nil {
		values = []helpers.IdentityBasicAuth{}
	}
	raw, err := groupPage(values)
	return &kkOps.ListPrincipalBasicAuthsResponse{RawResponse: raw}, err
}

func (a *BasicAuthAPI) GetPrincipalBasicAuth(_ context.Context, req kkOps.GetPrincipalBasicAuthRequest,
	_ ...kkOps.Option,
) (*kkOps.GetPrincipalBasicAuthResponse, error) {
	for _, value := range a.Parents[req.DirectoryID+"/"+req.PrincipalID] {
		if value.ID == req.BasicAuthID {
			raw, err := groupRaw(value)
			return &kkOps.GetPrincipalBasicAuthResponse{RawResponse: raw}, err
		}
	}
	return nil, fmt.Errorf("basic-auth fixture not found")
}

func (a *BasicAuthAPI) CreatePrincipalBasicAuth(_ context.Context, req kkOps.CreatePrincipalBasicAuthRequest,
	_ ...kkOps.Option,
) (*kkOps.CreatePrincipalBasicAuthResponse, error) {
	a.Creates++
	if a.Parents == nil {
		a.Parents = make(map[string][]helpers.IdentityBasicAuth)
	}
	value := helpers.IdentityBasicAuth{
		ID: uuid.NewString(), Username: req.CreateBasicAuthBody.Username,
		ManagedBy: maps.Clone(req.CreateBasicAuthBody.ManagedBy),
	}
	key := req.DirectoryID + "/" + req.PrincipalID
	a.Parents[key] = append(a.Parents[key], value)
	raw, err := groupRaw(value)
	return &kkOps.CreatePrincipalBasicAuthResponse{StatusCode: 201, RawResponse: raw}, err
}

func (a *BasicAuthAPI) ReplacePrincipalBasicAuth(_ context.Context, req kkOps.ReplacePrincipalBasicAuthRequest,
	_ ...kkOps.Option,
) (*kkOps.ReplacePrincipalBasicAuthResponse, error) {
	key := req.DirectoryID + "/" + req.PrincipalID
	for i := range a.Parents[key] {
		if a.Parents[key][i].ID != req.BasicAuthID {
			continue
		}
		a.Writes = append(a.Writes, req.ReplaceBasicAuthBody)
		value := &a.Parents[key][i]
		value.Username, value.ManagedBy = req.ReplaceBasicAuthBody.Username, maps.Clone(
			req.ReplaceBasicAuthBody.ManagedBy,
		)
		if a.Failure == "put-drift" {
			value.ManagedBy = nil
		}
		raw, err := groupRaw(value)
		return &kkOps.ReplacePrincipalBasicAuthResponse{StatusCode: 200, RawResponse: raw}, err
	}
	return nil, fmt.Errorf("basic-auth fixture not found")
}

func (a *BasicAuthAPI) DeletePrincipalBasicAuth(_ context.Context, req kkOps.DeletePrincipalBasicAuthRequest,
	_ ...kkOps.Option,
) (*kkOps.DeletePrincipalBasicAuthResponse, error) {
	a.Deletes++
	key := req.DirectoryID + "/" + req.PrincipalID
	a.Parents[key] = slices.DeleteFunc(
		a.Parents[key],
		func(v helpers.IdentityBasicAuth) bool { return v.ID == req.BasicAuthID },
	)
	return &kkOps.DeletePrincipalBasicAuthResponse{StatusCode: 204}, nil
}

func (a *BasicAuthAPI) ListPasswords(_ context.Context, req kkOps.ListPasswordsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListPasswordsResponse, error) {
	if a.Failure == "password-list" {
		return nil, fmt.Errorf("incomplete password observation")
	}
	values := a.Passwords[req.DirectoryID+"/"+req.PrincipalID+"/"+req.BasicAuthID]
	if values == nil {
		values = []helpers.IdentityPassword{}
	}
	raw, err := groupPage(values)
	return &kkOps.ListPasswordsResponse{RawResponse: raw}, err
}

func (*BasicAuthAPI) GetPassword(
	context.Context,
	kkOps.GetPasswordRequest,
	...kkOps.Option,
) (*kkOps.GetPasswordResponse, error) {
	return nil, fmt.Errorf("password detail not configured in fixture")
}

func (a *BasicAuthAPI) ListPrincipalKeys(_ context.Context, req kkOps.ListPrincipalKeysRequest,
	_ ...kkOps.Option,
) (*kkOps.ListPrincipalKeysResponse, error) {
	if a.Failure == "keys" {
		return nil, fmt.Errorf("incomplete API-key observation")
	}
	values := a.Keys[req.DirectoryID+"/"+req.PrincipalID]
	if values == nil {
		values = []string{}
	}
	raw, err := groupPage(values)
	return &kkOps.ListPrincipalKeysResponse{RawResponse: raw}, err
}
