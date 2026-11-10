package konnect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/identitykeys"
)

type VaultKeysAPI struct {
	DirectoryDependenciesAPI
	Keys                                     []helpers.IdentityVaultKey
	Writes, Creates, Updates, Deletes, Lists int
	Counter                                  int
}

func vaultKeyResponse(value any) *http.Response {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data))}
}

func (a *VaultKeysAPI) ListVaultKeys(_ context.Context, request kkOps.ListVaultKeysRequest,
	_ ...kkOps.Option,
) (*kkOps.ListVaultKeysResponse, error) {
	a.Lists++
	keys := make([]helpers.IdentityVaultKey, 0)
	for _, key := range a.Keys {
		if key.DirectoryID == request.DirectoryID {
			keys = append(keys, key)
		}
	}
	return &kkOps.ListVaultKeysResponse{RawResponse: vaultKeyResponse(map[string]any{
		"data": keys, "meta": map[string]any{"page": map[string]any{"size": 100, "next": nil}},
	})}, nil
}

func (a *VaultKeysAPI) GetVaultKey(_ context.Context, directory, id string, _ ...kkOps.Option,
) (*kkOps.GetVaultKeyResponse, error) {
	for _, key := range a.Keys {
		if key.DirectoryID == directory && key.ID == id {
			return &kkOps.GetVaultKeyResponse{RawResponse: vaultKeyResponse(key)}, nil
		}
	}
	return nil, &sdkerrors.SDKError{StatusCode: 404}
}

func (a *VaultKeysAPI) CreateVaultKey(_ context.Context, directory string, request kkComps.CreateVaultKeyBody,
	_ ...kkOps.Option,
) (*kkOps.CreateVaultKeyResponse, error) {
	material, err := identitykeys.ParsePublicKey(request.PublicKey)
	if err != nil {
		return nil, err
	}
	a.Counter++
	value := helpers.IdentityVaultKey{
		ID: fmt.Sprintf("731559d3-39bd-48c7-b33c-%012d", a.Counter), DirectoryID: directory,
		Name: request.Name, PublicKey: material.PEM, Kid: material.Kid, Algorithm: material.Algorithm, KeySize: material.Bits,
		CreatedAt: time.Date(
			2026,
			10,
			31,
			12,
			0,
			0,
			0,
			time.UTC,
		), UpdatedAt: time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC),
	}
	a.Keys = append(a.Keys, value)
	a.Writes++
	a.Creates++
	return &kkOps.CreateVaultKeyResponse{RawResponse: vaultKeyResponse(value)}, nil
}

func (a *VaultKeysAPI) UpdateVaultKey(_ context.Context, request kkOps.UpdateVaultKeyRequest, _ ...kkOps.Option,
) (*kkOps.UpdateVaultKeyResponse, error) {
	for i := range a.Keys {
		if a.Keys[i].DirectoryID == request.DirectoryID && a.Keys[i].ID == request.KeyID {
			a.Keys[i].Name = *request.PatchVaultKeyBody.Name
			a.Writes++
			a.Updates++
			return &kkOps.UpdateVaultKeyResponse{RawResponse: vaultKeyResponse(a.Keys[i])}, nil
		}
	}
	return nil, &sdkerrors.SDKError{StatusCode: 404}
}

func (a *VaultKeysAPI) DeleteVaultKey(_ context.Context, directory, id string, _ ...kkOps.Option,
) (*kkOps.DeleteVaultKeyResponse, error) {
	for i, key := range a.Keys {
		if key.DirectoryID == directory && key.ID == id {
			a.Keys = append(a.Keys[:i], a.Keys[i+1:]...)
			a.Writes++
			a.Deletes++
			return &kkOps.DeleteVaultKeyResponse{}, nil
		}
	}
	return nil, &sdkerrors.SDKError{StatusCode: 404}
}
