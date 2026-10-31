package helpers

import (
	"context"
	"fmt"
	"maps"
	"slices"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/secretoutput"
	"github.com/kong/kongctl/internal/util"
)

// CreateAndCaptureIdentityAuthServerClient confines the initial generated
// credential to a preflight-reserved file and returns a public projection.
func CreateAndCaptureIdentityAuthServerClient(ctx context.Context, api IdentityAuthServerClientsAPI,
	serverID string, request kkComps.CreateClient, output *secretoutput.File, metadata secretoutput.Metadata,
) (*kkComps.AuthServerClient, error) {
	if api == nil || output == nil {
		return nil, fmt.Errorf("auth-server client API and reserved initial-secret output are required")
	}
	if !util.IsValidUUID(serverID) {
		return nil, fmt.Errorf("auth-server client creation requires a parent UUID")
	}
	metadata.AuthServerID = serverID
	if err := output.RecordPending(metadata); err != nil {
		return nil, fmt.Errorf("prepare client creation recovery metadata: %w", err)
	}
	response, err := api.CreateAuthServerClient(httpclient.WithoutRetries(ctx), serverID, request,
		kkOps.WithRetries(retry.Config{Strategy: "none"}))
	if err != nil {
		return nil, fmt.Errorf(
			"%w; client creation outcome is unconfirmed; inspect client metadata and capture files before retrying",
			identitySecretAPIError("create auth-server client", err),
		)
	}
	if response == nil || response.CreatedClient == nil {
		return nil, fmt.Errorf("missing client creation response; inspect metadata and capture files before retrying")
	}
	value := response.CreatedClient
	metadata.AuthServerID, metadata.ClientID = serverID, value.ID
	knownID := "unavailable"
	if identityClientIDPattern.MatchString(value.ID) {
		knownID = value.ID
	}
	if err := output.Capture(metadata, value.ClientSecret); err != nil {
		return nil, fmt.Errorf(
			"client was created but initial-secret capture is unconfirmed: %w; inspect output before retrying",
			err,
		)
	}
	if knownID == "unavailable" || value.Name != request.Name {
		return nil, fmt.Errorf(
			"client creation response has inconsistent identity; initial secret was captured; inspect metadata",
		)
	}
	return &kkComps.AuthServerClient{
		ID: value.ID, Name: value.Name, GrantTypes: slices.Clone(value.GrantTypes),
		ResponseTypes: slices.Clone(value.ResponseTypes), RedirectUris: slices.Clone(value.RedirectUris),
		LoginURI: value.LoginURI, AccessTokenDuration: value.AccessTokenDuration, IDTokenDuration: value.IDTokenDuration,
		AllowAllScopes: value.AllowAllScopes, AllowScopes: slices.Clone(value.AllowScopes), Labels: maps.Clone(value.Labels),
		TokenEndpointAuthMethod: value.TokenEndpointAuthMethod, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}, nil
}
