package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/util"
)

type IdentityBasicAuthAPI interface {
	IdentityBasicAuthReader
	IdentityPasswordsReader
	CreatePrincipalBasicAuth(context.Context, kkOps.CreatePrincipalBasicAuthRequest, ...kkOps.Option) (
		*kkOps.CreatePrincipalBasicAuthResponse, error,
	)
	ReplacePrincipalBasicAuth(context.Context, kkOps.ReplacePrincipalBasicAuthRequest, ...kkOps.Option) (
		*kkOps.ReplacePrincipalBasicAuthResponse, error,
	)
	DeletePrincipalBasicAuth(context.Context, kkOps.DeletePrincipalBasicAuthRequest, ...kkOps.Option) (
		*kkOps.DeletePrincipalBasicAuthResponse, error,
	)
}

// Any API key blocks parent deletion. An unfiltered, empty terminal page is
// sufficient; no key material or remote ownership grants implicit revocation.
func VerifyPrincipalAPIKeysEmpty(ctx context.Context, api IdentityPrincipalKeysReader,
	directoryID, principalID string,
) error {
	if err := validatePrincipalKeyReader(api, directoryID, principalID); err != nil {
		return err
	}
	response, err := api.ListPrincipalKeys(ctx, kkOps.ListPrincipalKeysRequest{
		DirectoryID: directoryID, PrincipalID: principalID,
	})
	if err != nil {
		return identitySecretAPIError("verify principal API keys before deletion", err)
	}
	if response == nil {
		return fmt.Errorf("missing principal API-key collection response")
	}
	var page identityCredentialPage[json.RawMessage]
	if err := decodeIdentityCredentialResponse(response.RawResponse, &page); err != nil {
		return err
	}
	next, err := page.next()
	if err != nil {
		return err
	}
	if len(page.Data) != 0 || next != nil && *next != "" {
		return fmt.Errorf(
			"principal has API keys or an incomplete credential observation; remove keys explicitly before deletion",
		)
	}
	return nil
}

// IdentityBasicAuthReplacement projects a fresh username record into a full
// PUT body. Callers decide ownership and overlay explicit requested fields.
// Password values and password records are not part of this request.
func IdentityBasicAuthReplacement(value IdentityBasicAuth) kkComps.ReplaceBasicAuthBody {
	return kkComps.ReplaceBasicAuthBody{Username: value.Username, ManagedBy: maps.Clone(value.ManagedBy)}
}

func verifyIdentityBasicAuthWrite(response *http.Response, id, username string,
	managedBy map[string]string,
) (*IdentityBasicAuth, error) {
	var value IdentityBasicAuth
	if err := decodeIdentityCredentialResponse(response, &value); err != nil {
		return nil, fmt.Errorf("cannot verify basic-auth write; inspect the collection before retrying: %w", err)
	}
	if !util.IsValidUUID(value.ID) ||
		id != "" && value.ID != id || value.Username != username || !maps.Equal(value.ManagedBy, managedBy) {
		return nil, fmt.Errorf(
			"basic-auth write did not retain requested identity or metadata; inspect before retrying",
		)
	}
	return &value, nil
}

func CreateIdentityBasicAuth(ctx context.Context, api IdentityBasicAuthAPI,
	directoryID, principalID string, body kkComps.CreateBasicAuthBody,
) (*IdentityBasicAuth, error) {
	if api == nil {
		return nil, fmt.Errorf("identity basic-auth API is not configured")
	}
	if err := validateIdentityCredentialPath(directoryID, principalID); err != nil {
		return nil, err
	}
	if !validIdentityBasicAuthUsername(body.Username) {
		return nil, fmt.Errorf("username must contain 1 to 255 characters")
	}
	// An uncertain POST may have succeeded. Neither retry layer may resend it.
	response, err := api.CreatePrincipalBasicAuth(httpclient.WithoutRetries(ctx), kkOps.CreatePrincipalBasicAuthRequest{
		DirectoryID: directoryID, PrincipalID: principalID, CreateBasicAuthBody: body,
	}, kkOps.WithRetries(retry.Config{Strategy: "none"}))
	if err != nil {
		return nil, fmt.Errorf("%w; inspect the collection before retrying",
			identitySecretAPIError("create basic-auth credential", err))
	}
	if response == nil || response.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("missing basic-auth creation response; inspect the collection before retrying")
	}
	value, err := verifyIdentityBasicAuthWrite(response.RawResponse, "", body.Username, body.ManagedBy)
	if err != nil {
		return nil, err
	}
	observed, err := GetIdentityBasicAuth(ctx, api, directoryID, principalID, value.ID)
	if err != nil {
		return nil, fmt.Errorf("cannot verify created basic-auth credential; inspect before retrying: %w", err)
	}
	if observed.Username != body.Username || !maps.Equal(observed.ManagedBy, body.ManagedBy) {
		return nil, fmt.Errorf("created basic-auth credential did not retain requested fields; inspect before retrying")
	}
	return observed, nil
}

func ReplaceIdentityBasicAuth(ctx context.Context, api IdentityBasicAuthAPI,
	directoryID, principalID, id string, body kkComps.ReplaceBasicAuthBody,
) (*IdentityBasicAuth, error) {
	if api == nil {
		return nil, fmt.Errorf("identity basic-auth API is not configured")
	}
	if err := validateIdentityCredentialPath(directoryID, principalID, id); err != nil {
		return nil, err
	}
	if !validIdentityBasicAuthUsername(body.Username) {
		return nil, fmt.Errorf("username must contain 1 to 255 characters")
	}
	response, err := api.ReplacePrincipalBasicAuth(ctx, kkOps.ReplacePrincipalBasicAuthRequest{
		DirectoryID: directoryID, PrincipalID: principalID, BasicAuthID: id, ReplaceBasicAuthBody: body,
	})
	if err != nil {
		return nil, identitySecretAPIError("replace basic-auth credential", err)
	}
	if response == nil || response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("missing basic-auth replacement response; inspect the credential before retrying")
	}
	if _, err := verifyIdentityBasicAuthWrite(response.RawResponse, id, body.Username, body.ManagedBy); err != nil {
		return nil, err
	}
	observed, err := GetIdentityBasicAuth(ctx, api, directoryID, principalID, id)
	if err != nil {
		return nil, fmt.Errorf("cannot verify replaced basic-auth credential; inspect before retrying: %w", err)
	}
	if observed.Username != body.Username || !maps.Equal(observed.ManagedBy, body.ManagedBy) {
		return nil, fmt.Errorf(
			"replaced basic-auth credential did not retain requested fields; inspect before retrying",
		)
	}
	return observed, nil
}

// VerifyIdentityBasicAuthEmpty establishes no authority over passwords. A
// native DELETE can cascade, so callers must remove passwords explicitly.
func VerifyIdentityBasicAuthEmpty(ctx context.Context, api IdentityPasswordsReader,
	directoryID, principalID, id string,
) error {
	values, err := ListIdentityPasswords(ctx, api, directoryID, principalID, id, 100)
	if err != nil {
		return fmt.Errorf("cannot verify basic-auth passwords before deletion: %w", err)
	}
	if len(values) != 0 {
		return fmt.Errorf("basic-auth credential still has passwords; remove them explicitly before deleting it")
	}
	return nil
}

func DeleteIdentityBasicAuth(ctx context.Context, api IdentityBasicAuthAPI,
	directoryID, principalID, id string,
) error {
	if api == nil {
		return fmt.Errorf("identity basic-auth API is not configured")
	}
	if err := validateIdentityCredentialPath(directoryID, principalID, id); err != nil {
		return err
	}
	if err := VerifyIdentityBasicAuthEmpty(ctx, api, directoryID, principalID, id); err != nil {
		return err
	}
	response, err := api.DeletePrincipalBasicAuth(ctx, kkOps.DeletePrincipalBasicAuthRequest{
		DirectoryID: directoryID, PrincipalID: principalID, BasicAuthID: id,
	})
	if err != nil {
		return identitySecretAPIError("delete basic-auth credential", err)
	}
	if response == nil || response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("missing basic-auth deletion response; inspect the credential before retrying")
	}
	return nil
}
