package konnect

import (
	"context"
	"io"
	"net/http"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// DirectoryDependenciesAPI supplies explicitly empty Vault observations by
// default. Individual tests can attach one collection without a parent cascade.
type DirectoryDependenciesAPI struct {
	Attached string
	Calls    []string
}

func (a *DirectoryDependenciesAPI) response(resource string) *http.Response {
	a.Calls = append(a.Calls, resource)
	data := "[]"
	if a.Attached == resource {
		data = `[{"id":"731559d3-39bd-48c7-b33c-eb192847eb20"}]`
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(
		`{"data":` + data + `,"meta":{"page":{"size":20,"next":null}}}`,
	))}
}

func (a *DirectoryDependenciesAPI) ListVaultKeys(context.Context, kkOps.ListVaultKeysRequest, ...kkOps.Option) (
	*kkOps.ListVaultKeysResponse, error,
) {
	return &kkOps.ListVaultKeysResponse{RawResponse: a.response("Vault keys")}, nil
}

func (a *DirectoryDependenciesAPI) ListVaultProviders(
	context.Context, kkOps.ListVaultProvidersRequest, ...kkOps.Option,
) (
	*kkOps.ListVaultProvidersResponse, error,
) {
	return &kkOps.ListVaultProvidersResponse{RawResponse: a.response("Vault providers")}, nil
}

func (a *DirectoryDependenciesAPI) ListTrustedIdps(
	context.Context, string, *kkComps.CursorPageParameters, ...kkOps.Option,
) (
	*kkOps.ListTrustedIdpsResponse, error,
) {
	return &kkOps.ListTrustedIdpsResponse{RawResponse: a.response("Trusted IdPs")}, nil
}
