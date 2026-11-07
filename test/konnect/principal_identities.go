package konnect

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	kkErrors "github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

// PrincipalIdentitiesAPI supplies complete child observations for parent tests.
type PrincipalIdentitiesAPI struct {
	helpers.IdentityPrincipalIdentitiesAPI
	Data     []kkComps.KongPrincipalIdentity
	Parents  map[string][]kkComps.KongPrincipalIdentity
	Creates  []kkOps.CreateIdentityRequest
	Replaces []kkOps.ReplaceIdentityRequest
	Gets     []kkOps.GetIdentityRequest
	Deletes  []kkOps.DeleteIdentityRequest
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
	data := *a.collection(request.DirectoryID, request.PrincipalID)
	if data == nil {
		data = []kkComps.KongPrincipalIdentity{}
	}
	return &kkOps.ListIdentitiesResponse{
		ListKongPrincipalIdentities: &kkComps.ListKongPrincipalIdentities{Data: data},
	}, nil
}

func (a *PrincipalIdentitiesAPI) collection(directory, principal string) *[]kkComps.KongPrincipalIdentity {
	if a.Parents == nil {
		return &a.Data
	}
	// Map values are not addressable; collection callers use read/write helpers.
	value := a.Parents[directory+"/"+principal]
	return &value
}

func (a *PrincipalIdentitiesAPI) store(directory, principal string, data []kkComps.KongPrincipalIdentity) {
	if a.Parents == nil {
		a.Data = data
	} else {
		a.Parents[directory+"/"+principal] = data
	}
}

func principalIdentityResponse(
	body kkComps.KongPrincipalIdentityBody,
	id string,
) (*kkComps.KongPrincipalIdentity, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	fields["id"], fields["created_at"], fields["updated_at"] = id, "2026-10-31T00:00:00Z", "2026-10-31T00:00:00Z"
	for _, field := range []string{"labels", "managed_by"} {
		if fields[field] == nil {
			fields[field] = map[string]string{}
		}
	}
	if body.Type == kkComps.KongPrincipalIdentityBodyTypeControlPlaneConsumer {
		fields["control_plane_type"] = "api_gateway"
	}
	data, err = json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	var result kkComps.KongPrincipalIdentity
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func identityFixtureID(value kkComps.KongPrincipalIdentity) string {
	data, _ := json.Marshal(value)
	var v struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(data, &v)
	return v.ID
}

func (a *PrincipalIdentitiesAPI) GetIdentity(_ context.Context, request kkOps.GetIdentityRequest,
	_ ...kkOps.Option,
) (*kkOps.GetIdentityResponse, error) {
	a.Gets = append(a.Gets, request)
	for _, v := range *a.collection(request.DirectoryID, request.PrincipalID) {
		if identityFixtureID(v) == request.IdentityID {
			return &kkOps.GetIdentityResponse{KongPrincipalIdentity: &v}, nil
		}
	}
	return nil, &kkErrors.NotFoundError{Status: 404}
}

func (a *PrincipalIdentitiesAPI) CreateIdentity(_ context.Context, request kkOps.CreateIdentityRequest,
	_ ...kkOps.Option,
) (*kkOps.CreateIdentityResponse, error) {
	a.Creates = append(a.Creates, request)
	v, err := principalIdentityResponse(
		request.KongPrincipalIdentityBody,
		fmt.Sprintf("00000000-0000-4000-8100-%012d", len(a.Creates)),
	)
	if err != nil {
		return nil, err
	}
	a.store(
		request.DirectoryID,
		request.PrincipalID,
		append(*a.collection(request.DirectoryID, request.PrincipalID), *v),
	)
	return &kkOps.CreateIdentityResponse{KongPrincipalIdentity: v}, nil
}

func (a *PrincipalIdentitiesAPI) ReplaceIdentity(_ context.Context, request kkOps.ReplaceIdentityRequest,
	_ ...kkOps.Option,
) (*kkOps.ReplaceIdentityResponse, error) {
	a.Replaces = append(a.Replaces, request)
	data := *a.collection(request.DirectoryID, request.PrincipalID)
	for i, v := range data {
		if identityFixtureID(v) != request.IdentityID {
			continue
		}
		value, err := principalIdentityResponse(request.KongPrincipalIdentityBody, request.IdentityID)
		if err != nil {
			return nil, err
		}
		data[i] = *value
		a.store(request.DirectoryID, request.PrincipalID, data)
		return &kkOps.ReplaceIdentityResponse{KongPrincipalIdentity: value}, nil
	}
	return nil, &kkErrors.NotFoundError{Status: 404}
}

func (a *PrincipalIdentitiesAPI) DeleteIdentity(_ context.Context, request kkOps.DeleteIdentityRequest,
	_ ...kkOps.Option,
) (*kkOps.DeleteIdentityResponse, error) {
	a.Deletes = append(a.Deletes, request)
	data := *a.collection(request.DirectoryID, request.PrincipalID)
	for i, v := range data {
		if identityFixtureID(v) != request.IdentityID {
			continue
		}
		a.store(request.DirectoryID, request.PrincipalID, append(data[:i], data[i+1:]...))
		return &kkOps.DeleteIdentityResponse{}, nil
	}
	return nil, &kkErrors.NotFoundError{Status: 404}
}
