package helpers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/secretoutput"
	"github.com/kong/kongctl/internal/util"
)

// CreateAndCaptureIdentityClientSecret returns only metadata. Secret material
// remains confined to the create response and the reserved private output file.
func CreateAndCaptureIdentityClientSecret(ctx context.Context, api IdentityAuthServerClientsAPI,
	serverID, clientID string, request kkComps.CreateSecretPayload, outputPath string,
) (*IdentityClientSecret, error) {
	if api == nil {
		return nil, fmt.Errorf("identity auth-server clients API is not configured")
	}
	if err := validateIdentitySecretPath(serverID, clientID, "", false); err != nil {
		return nil, err
	}
	metadata := secretoutput.Metadata{AuthServerID: serverID, ClientID: clientID}
	output, err := secretoutput.Reserve(outputPath, metadata)
	if err != nil {
		return nil, err
	}
	defer func() { _ = output.Close() }()
	response, err := api.CreateClientSecret(httpclient.WithoutRetries(ctx), kkOps.CreateClientSecretRequest{
		AuthServerID: serverID, ClientID: clientID, CreateSecretPayload: request,
	}, kkOps.WithRetries(retry.Config{Strategy: "none"}))
	if err != nil {
		return nil, fmt.Errorf("%w; creation outcome is unconfirmed; inspect client secret metadata before retrying; "+
			"reserved output file: %s", identitySecretAPIError("create client secret", err), outputPath)
	}
	if response == nil || response.ClientSecretItem == nil {
		return nil, fmt.Errorf("missing client secret creation response; inspect metadata before retrying; "+
			"reserved output: %s",
			outputPath)
	}
	item := response.ClientSecretItem
	metadata.SecretID = item.ID
	knownID := "unavailable"
	if util.IsValidUUID(item.ID) {
		knownID = item.ID
	}
	value := ""
	if item.Secret != nil {
		value = *item.Secret
	}
	if err := output.Capture(metadata, value); err != nil {
		return nil, fmt.Errorf("client secret %q was created but capture is unconfirmed: %w; inspect %s before retrying",
			knownID, err, outputPath)
	}
	if request.Secret != nil && value != *request.Secret {
		return nil, fmt.Errorf("client secret %q was captured but differs from supplied material; inspect the issued record",
			knownID)
	}
	if !util.IsValidUUID(item.ID) {
		return nil, fmt.Errorf("client secret response has no valid ID; material was captured in %s; inspect client metadata",
			outputPath)
	}
	if item.Enabled == nil || (request.Enabled != nil && *item.Enabled != *request.Enabled) {
		return nil, fmt.Errorf("client secret %q was captured but enabled state is inconsistent; inspect metadata", item.ID)
	}
	result := identityClientSecret(*item)
	return &result, nil
}

func UpdateIdentityClientSecret(ctx context.Context, api IdentityAuthServerClientsAPI,
	serverID, clientID, secretID string, enabled bool,
) (*IdentityClientSecret, error) {
	if api == nil {
		return nil, fmt.Errorf("identity auth-server clients API is not configured")
	}
	if err := validateIdentitySecretPath(serverID, clientID, secretID, true); err != nil {
		return nil, err
	}
	response, err := api.UpdateClientSecret(ctx, kkOps.UpdateClientSecretRequest{
		AuthServerID: serverID, ClientID: clientID, ClientSecretID: secretID,
		UpdateSecretPayload: kkComps.UpdateSecretPayload{Enabled: &enabled},
	})
	if err != nil {
		return nil, identitySecretAPIError("update client secret", err)
	}
	if response == nil || response.ClientSecretItem == nil || response.ClientSecretItem.ID != secretID ||
		response.ClientSecretItem.Enabled == nil || *response.ClientSecretItem.Enabled != enabled {
		return nil, fmt.Errorf("missing or inconsistent client secret update response; inspect metadata before retrying")
	}
	value := identityClientSecret(*response.ClientSecretItem)
	return &value, nil
}

func DeleteIdentityClientSecret(ctx context.Context, api IdentityAuthServerClientsAPI,
	serverID, clientID, secretID string,
) error {
	if api == nil {
		return fmt.Errorf("identity auth-server clients API is not configured")
	}
	if err := validateIdentitySecretPath(serverID, clientID, secretID, true); err != nil {
		return err
	}
	response, err := api.DeleteClientSecret(ctx, kkOps.DeleteClientSecretRequest{
		AuthServerID: serverID, ClientID: clientID, ClientSecretID: secretID,
	})
	if err != nil {
		return identitySecretAPIError("delete client secret", err)
	}
	if response == nil || response.StatusCode != 204 {
		return fmt.Errorf("missing or inconsistent client secret deletion response; inspect metadata before retrying")
	}
	return nil
}

func validateIdentitySecretPath(serverID, clientID, secretID string, requireSecret bool) error {
	if !util.IsValidUUID(serverID) || !identityClientIDPattern.MatchString(clientID) ||
		(requireSecret && !util.IsValidUUID(secretID)) {
		return fmt.Errorf("invalid auth-server, client or secret ID")
	}
	return nil
}

// Error payloads can echo supplied material. Report status without retaining a
// raw SDK error as an unwrap target or formatting its body in normal output.
func identitySecretAPIError(operation string, err error) error {
	var generic *sdkerrors.SDKError
	if errors.As(err, &generic) && generic.StatusCode >= 400 && generic.StatusCode <= 599 {
		return fmt.Errorf("failed to %s: HTTP %d", operation, generic.StatusCode)
	}
	for cause := err; cause != nil; cause = errors.Unwrap(cause) {
		var body struct {
			Status int `json:"status"`
		}
		if json.Unmarshal([]byte(cause.Error()), &body) == nil && body.Status >= 400 && body.Status <= 599 {
			return fmt.Errorf("failed to %s: HTTP %d", operation, body.Status)
		}
	}
	return fmt.Errorf("failed to %s: request did not complete successfully", operation)
}
