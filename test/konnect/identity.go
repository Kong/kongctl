// Package konnect supplies API fixtures for KongCTL ownership and payload tests.
package konnect

import (
	"context"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type DirectoryAPI struct {
	helpers.IdentityDirectoriesAPI
	Directories                             []kkComps.KongDirectory
	Current                                 kkComps.KongDirectory
	Created                                 kkComps.CreateDirectoryBody
	Replaced                                kkComps.ReplaceDirectoryBody
	Creates, Replaces, Deletes, Gets, Lists int
	Force                                   *kkOps.DeleteDirectoryForce
	List                                    func(*kkComps.CursorPageParameters) (*kkOps.ListKongDirectoriesResponse, error)
}

func Directory(name, namespace string, protected bool) kkComps.KongDirectory {
	metadata := map[string]string{}
	if namespace != "" {
		metadata = map[string]string{
			"tool": "kongctl", "version": "dev", "kongctl-namespace": namespace,
			"kongctl-protected": "false",
		}
		if protected {
			metadata["kongctl-protected"] = "true"
		}
	}
	return kkComps.KongDirectory{
		ID: "7a5e1b58-48a4-4586-b02d-0d91eccca152", Name: name, Description: "retained description",
		AllowAllControlPlanes: new(true), TTLSecs: new(int64(900)), NegativeTTLSecs: new(int64(450)),
		VaultEnabled: new(false), Labels: map[string]string{"owner": "identity-team"}, ManagedBy: metadata,
	}
}

func (a *DirectoryAPI) ListKongDirectories(_ context.Context, page *kkComps.CursorPageParameters, _ *string,
	_ ...kkOps.Option,
) (*kkOps.ListKongDirectoriesResponse, error) {
	a.Lists++
	if a.List != nil {
		return a.List(page)
	}
	return &kkOps.ListKongDirectoriesResponse{ListKongDirectories: &kkComps.ListKongDirectories{
		Data: a.Directories,
	}}, nil
}

func (a *DirectoryAPI) GetDirectory(context.Context, string, ...kkOps.Option) (*kkOps.GetDirectoryResponse, error) {
	a.Gets++
	return &kkOps.GetDirectoryResponse{KongDirectory: &a.Current}, nil
}

func (a *DirectoryAPI) CreateDirectory(_ context.Context, request kkComps.CreateDirectoryBody,
	_ ...kkOps.Option,
) (*kkOps.CreateDirectoryResponse, error) {
	a.Creates++
	a.Created = request
	return &kkOps.CreateDirectoryResponse{KongDirectory: &a.Current}, nil
}

func (a *DirectoryAPI) ReplaceDirectory(_ context.Context, _ string, request kkComps.ReplaceDirectoryBody,
	_ ...kkOps.Option,
) (*kkOps.ReplaceDirectoryResponse, error) {
	a.Replaces++
	a.Replaced = request
	return &kkOps.ReplaceDirectoryResponse{KongDirectory: &a.Current}, nil
}

func (a *DirectoryAPI) DeleteDirectory(_ context.Context, _ string, force *kkOps.DeleteDirectoryForce,
	_ ...kkOps.Option,
) (*kkOps.DeleteDirectoryResponse, error) {
	a.Deletes++
	a.Force = force
	return &kkOps.DeleteDirectoryResponse{}, nil
}
