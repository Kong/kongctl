package konnect

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

// PrincipalAPI keeps parent-scoped fixtures and captures full replacement writes.
type PrincipalAPI struct {
	Directories map[string][]kkComps.KongPrincipal
	Writes      []kkComps.ReplacePrincipalBody
	Creates     int
	Deletes     int
}

func (a *PrincipalAPI) ListPrincipals(
	_ context.Context,
	request kkOps.ListPrincipalsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListPrincipalsResponse, error) {
	return &kkOps.ListPrincipalsResponse{
		ListKongPrincipals: &kkComps.ListKongPrincipals{Data: a.Directories[request.DirectoryID]},
	}, nil
}

func (a *PrincipalAPI) GetPrincipal(
	_ context.Context,
	directoryID, id string,
	_ ...kkOps.Option,
) (*kkOps.GetPrincipalResponse, error) {
	for _, value := range a.Directories[directoryID] {
		if value.ID == id {
			return &kkOps.GetPrincipalResponse{KongPrincipal: &value}, nil
		}
	}
	return nil, fmt.Errorf("principal fixture %q not found", id)
}

func (a *PrincipalAPI) CreatePrincipal(
	_ context.Context,
	directoryID string,
	request kkComps.CreatePrincipalBody,
	_ ...kkOps.Option,
) (*kkOps.CreatePrincipalResponse, error) {
	a.Creates++
	var value kkComps.KongPrincipal
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	value.ID = fmt.Sprintf("00000000-0000-4000-8000-%012d", a.Creates)
	a.Directories[directoryID] = append(a.Directories[directoryID], value)
	return &kkOps.CreatePrincipalResponse{KongPrincipal: &value}, nil
}

func (a *PrincipalAPI) ReplacePrincipal(
	_ context.Context,
	request kkOps.ReplacePrincipalRequest,
	_ ...kkOps.Option,
) (*kkOps.ReplacePrincipalResponse, error) {
	a.Writes = append(a.Writes, request.ReplacePrincipalBody)
	for i := range a.Directories[request.DirectoryID] {
		value := &a.Directories[request.DirectoryID][i]
		if value.ID != request.PrincipalID {
			continue
		}
		id := value.ID
		data, err := json.Marshal(request.ReplacePrincipalBody)
		if err != nil {
			return nil, err
		}
		*value = kkComps.KongPrincipal{}
		if err := json.Unmarshal(data, value); err != nil {
			return nil, err
		}
		value.ID = id
		return &kkOps.ReplacePrincipalResponse{KongPrincipal: value}, nil
	}
	return nil, fmt.Errorf("principal fixture not found")
}

func (a *PrincipalAPI) DeletePrincipal(
	_ context.Context,
	directoryID, id string,
	_ ...kkOps.Option,
) (*kkOps.DeletePrincipalResponse, error) {
	a.Deletes++
	for i, value := range a.Directories[directoryID] {
		if value.ID == id {
			a.Directories[directoryID] = append(a.Directories[directoryID][:i], a.Directories[directoryID][i+1:]...)
			break
		}
	}
	return &kkOps.DeletePrincipalResponse{}, nil
}

var _ helpers.IdentityPrincipalsAPI = (*PrincipalAPI)(nil)
