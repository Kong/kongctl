package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/util"
)

// PrincipalIdentityReplacement projects a fresh response into its full PUT
// body. Generated IDs, timestamps and control_plane_type are read-only.
// Ownership decisions and field overlays belong to the caller.
func PrincipalIdentityReplacement(value PrincipalIdentity) (kkComps.KongPrincipalIdentityBody, error) {
	labels, managedBy := maps.Clone(value.Labels), maps.Clone(value.ManagedBy)
	switch value.Type {
	case "oidc":
		if value.Issuer != nil && value.Claim != nil {
			return kkComps.CreateKongPrincipalIdentityBodyOidc(kkComps.KongPrincipalIdentityOIDC{
				Issuer: *value.Issuer,
				Claim: kkComps.KongPrincipalIdentityOIDCClaim{
					Name: value.Claim.Name, Value: value.Claim.Value,
				},
				Labels: labels, ManagedBy: managedBy,
			}), nil
		}
	case "auth_server_client":
		if value.AuthServerID != nil && value.ClientID != nil {
			return kkComps.CreateKongPrincipalIdentityBodyAuthServerClient(
				kkComps.KongPrincipalIdentityAuthServerClient{
					AuthServerID: *value.AuthServerID, ClientID: *value.ClientID, Labels: labels, ManagedBy: managedBy,
				},
			), nil
		}
	case "control_plane_consumer":
		if value.ControlPlaneID != nil && value.ConsumerID != nil {
			var workspace *string
			if value.WorkspaceID != nil {
				workspace = new(*value.WorkspaceID)
			}
			return kkComps.CreateKongPrincipalIdentityBodyControlPlaneConsumer(kkComps.KongPrincipalIdentityCPConsumer{
				ControlPlaneID: *value.ControlPlaneID, ConsumerID: *value.ConsumerID, WorkspaceID: workspace,
				Labels: labels, ManagedBy: managedBy,
			}), nil
		}
	case "custom":
		if value.Key != nil && value.Value != nil {
			return kkComps.CreateKongPrincipalIdentityBodyCustom(kkComps.KongPrincipalIdentityCustom{
				Key: *value.Key, Value: *value.Value, Labels: labels, ManagedBy: managedBy,
			}), nil
		}
	default:
		return kkComps.KongPrincipalIdentityBody{}, fmt.Errorf("unsupported principal identity type %q", value.Type)
	}
	return kkComps.KongPrincipalIdentityBody{}, fmt.Errorf("incomplete %s principal identity response", value.Type)
}

func validatePrincipalIdentityWrite(api IdentityPrincipalIdentitiesAPI, directoryID, principalID string,
	body kkComps.KongPrincipalIdentityBody,
) (*PrincipalIdentity, error) {
	if api == nil {
		return nil, fmt.Errorf("principal identities API is not configured")
	}
	if !util.IsValidUUID(directoryID) || !util.IsValidUUID(principalID) {
		return nil, fmt.Errorf("directory and principal IDs must be UUIDs")
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("invalid principal identity request: %w", err)
	}
	// Validate the selected union before any request, including all required
	// variant fields. The SDK's union discriminator alone is insufficient.
	var verified kkComps.KongPrincipalIdentityBody
	if err := json.Unmarshal(data, &verified); err != nil {
		return nil, fmt.Errorf("invalid principal identity request: %w", err)
	}
	var requested PrincipalIdentity
	if err := json.Unmarshal(data, &requested); err != nil {
		return nil, fmt.Errorf("invalid principal identity request: %w", err)
	}
	return &requested, nil
}

func verifyPrincipalIdentityWrite(value *kkComps.KongPrincipalIdentity, requested *PrincipalIdentity,
	id string,
) (*PrincipalIdentity, error) {
	if value == nil {
		return nil, fmt.Errorf("missing principal identity write response; inspect the collection before retrying")
	}
	observed, err := principalIdentityFromSDK(*value)
	if err != nil {
		return nil, err
	}
	if id != "" && observed.ID != id {
		return nil, fmt.Errorf("principal identity write response does not match the selected ID")
	}
	if observed.Type != requested.Type || !maps.Equal(observed.ManagedBy, requested.ManagedBy) {
		return nil, fmt.Errorf("principal identity %s was written but type or managed_by was not retained; "+
			"inspect the collection before retrying", observed.ID)
	}
	return &observed, nil
}

// CreatePrincipalIdentity does not replay an uncertain POST. managed_by has no
// server-enforced uniqueness; declarative collision/ownership checks belong to
// its caller. A successful create must be visible under the selected parent.
func CreatePrincipalIdentity(ctx context.Context, api IdentityPrincipalIdentitiesAPI, directoryID, principalID string,
	body kkComps.KongPrincipalIdentityBody,
) (*PrincipalIdentity, error) {
	requested, err := validatePrincipalIdentityWrite(api, directoryID, principalID, body)
	if err != nil {
		return nil, err
	}
	response, err := api.CreateIdentity(httpclient.WithoutRetries(ctx), kkOps.CreateIdentityRequest{
		DirectoryID: directoryID, PrincipalID: principalID, KongPrincipalIdentityBody: body,
	}, kkOps.WithRetries(retry.Config{Strategy: "none"}))
	if err != nil {
		return nil, fmt.Errorf("failed to create principal identity; inspect the collection before retrying: %w", err)
	}
	if response == nil {
		return nil, fmt.Errorf("missing principal identity create response; inspect the collection before retrying")
	}
	created, err := verifyPrincipalIdentityWrite(response.KongPrincipalIdentity, requested, "")
	if err != nil {
		return nil, err
	}
	current, err := GetPrincipalIdentity(ctx, api, directoryID, principalID, created.ID, 100)
	if err != nil {
		return nil, fmt.Errorf("principal identity %s was created but read-back failed; inspect before retrying: %w",
			created.ID, err)
	}
	if current.Type != requested.Type || !maps.Equal(current.ManagedBy, requested.ManagedBy) {
		return nil, fmt.Errorf("principal identity %s did not retain its type or managed_by on read-back", created.ID)
	}
	return current, nil
}

// ReplacePrincipalIdentity accepts a complete payload, including metadata.
// Hydrate omitted fields with PrincipalIdentityReplacement before overlays.
// Q21 requires fresh membership proof even though PUT currently scopes correctly.
func ReplacePrincipalIdentity(ctx context.Context, api IdentityPrincipalIdentitiesAPI,
	directoryID, principalID, id string, body kkComps.KongPrincipalIdentityBody,
) (*PrincipalIdentity, error) {
	requested, err := validatePrincipalIdentityWrite(api, directoryID, principalID, body)
	if err != nil {
		return nil, err
	}
	current, err := GetPrincipalIdentity(ctx, api, directoryID, principalID, id, 100)
	if err != nil {
		return nil, err
	}
	if current.Type != requested.Type {
		return nil, fmt.Errorf("principal identity type cannot be changed; create a separate identity")
	}
	response, err := api.ReplaceIdentity(ctx, kkOps.ReplaceIdentityRequest{
		DirectoryID: directoryID, PrincipalID: principalID, IdentityID: id, KongPrincipalIdentityBody: body,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to replace principal identity: %w", err)
	}
	if response == nil {
		return nil, fmt.Errorf("missing principal identity replacement response")
	}
	return verifyPrincipalIdentityWrite(response.KongPrincipalIdentity, requested, id)
}

// DeletePrincipalIdentity must prove collection membership before DELETE: the
// deployed DELETE route accepts an identity through another existing principal.
func DeletePrincipalIdentity(ctx context.Context, api IdentityPrincipalIdentitiesAPI,
	directoryID, principalID, id string,
) error {
	if _, err := GetPrincipalIdentity(ctx, api, directoryID, principalID, id, 100); err != nil {
		return err
	}
	_, err := api.DeleteIdentity(ctx, kkOps.DeleteIdentityRequest{
		DirectoryID: directoryID, PrincipalID: principalID, IdentityID: id,
	})
	if err != nil {
		return fmt.Errorf("failed to delete principal identity: %w", err)
	}
	return nil
}
