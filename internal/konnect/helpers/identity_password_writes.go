package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"time"
	"unicode/utf8"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/optionalnullable"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/secretoutput"
	"github.com/kong/kongctl/internal/util"
)

type IdentityPasswordWritesAPI interface {
	IdentityPasswordsReader
	CreatePassword(context.Context, kkOps.CreatePasswordRequest, ...kkOps.Option) (*kkOps.CreatePasswordResponse, error)
	UpdatePassword(context.Context, kkOps.UpdatePasswordRequest, ...kkOps.Option) (*kkOps.UpdatePasswordResponse, error)
	ReplacePassword(
		context.Context,
		kkOps.ReplacePasswordRequest,
		...kkOps.Option,
	) (*kkOps.ReplacePasswordResponse, error)
	DeletePassword(context.Context, kkOps.DeletePasswordRequest, ...kkOps.Option) (*kkOps.DeletePasswordResponse, error)
	DeleteAllPasswords(context.Context, kkOps.DeleteAllPasswordsRequest, ...kkOps.Option) (
		*kkOps.DeleteAllPasswordsResponse, error,
	)
}

func validateIdentityPasswordMaterial(value string) error {
	if !utf8.ValidString(value) || value == "" || utf8.RuneCountInString(value) > 255 {
		return fmt.Errorf("password material must contain 1 to 255 UTF-8 characters")
	}
	return nil
}

func samePasswordExpiration(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}

// Read the fields needed by a preserving PUT without turning missing response
// properties into null expiration or an empty management map.
func passwordWriteObservation(response *http.Response, expectedID string) (*IdentityPassword, error) {
	var raw struct {
		ID        string          `json:"id"`
		ExpiresAt json.RawMessage `json:"expires_at"`
		ManagedBy json.RawMessage `json:"managed_by"`
		CreatedAt time.Time       `json:"created_at"`
		UpdatedAt time.Time       `json:"updated_at"`
	}
	if err := decodeIdentityCredentialResponse(response, &raw); err != nil {
		return nil, err
	}
	if !util.IsValidUUID(raw.ID) || expectedID != "" && raw.ID != expectedID ||
		len(raw.ExpiresAt) == 0 || len(raw.ManagedBy) == 0 {
		return nil, fmt.Errorf("missing or inconsistent password identity, expiration or management metadata")
	}
	value := IdentityPassword{ID: raw.ID, CreatedAt: raw.CreatedAt, UpdatedAt: raw.UpdatedAt}
	if err := json.Unmarshal(raw.ExpiresAt, &value.ExpiresAt); err != nil {
		return nil, fmt.Errorf("invalid password expiration response")
	}
	if err := json.Unmarshal(raw.ManagedBy, &value.ManagedBy); err != nil || value.ManagedBy == nil {
		return nil, fmt.Errorf("invalid password management metadata response")
	}
	return &value, nil
}

func getPasswordForWrite(ctx context.Context, api IdentityPasswordWritesAPI,
	directoryID, principalID, basicAuthID, id string,
) (*IdentityPassword, error) {
	response, err := api.GetPassword(ctx, kkOps.GetPasswordRequest{
		DirectoryID: directoryID, PrincipalID: principalID, BasicAuthID: basicAuthID, PasswordID: id,
	})
	if err != nil {
		return nil, identitySecretAPIError("read basic-auth password before mutation", err)
	}
	if response == nil {
		return nil, fmt.Errorf("missing password observation before mutation")
	}
	return passwordWriteObservation(response.RawResponse, id)
}

// CreateAndCaptureIdentityPassword returns only public metadata. Generated or
// imported material is persisted exclusively to the reserved private file.
func CreateAndCaptureIdentityPassword(ctx context.Context, api IdentityPasswordWritesAPI,
	directoryID, principalID, basicAuthID string, body kkComps.CreatePasswordBody, outputPath string,
) (*IdentityPassword, error) {
	if api == nil {
		return nil, fmt.Errorf("identity password writes API is not configured")
	}
	if err := validateIdentityCredentialPath(directoryID, principalID, basicAuthID); err != nil {
		return nil, err
	}
	if body.Secret != nil {
		if err := validateIdentityPasswordMaterial(*body.Secret); err != nil {
			return nil, err
		}
	}
	metadata := secretoutput.Metadata{DirectoryID: directoryID, PrincipalID: principalID, BasicAuthID: basicAuthID}
	output, err := secretoutput.Reserve(outputPath, metadata)
	if err != nil {
		return nil, err
	}
	defer func() { _ = output.Close() }()
	response, err := api.CreatePassword(httpclient.WithoutRetries(ctx), kkOps.CreatePasswordRequest{
		DirectoryID: directoryID, PrincipalID: principalID, BasicAuthID: basicAuthID, CreatePasswordBody: body,
	}, kkOps.WithRetries(retry.Config{Strategy: "none"}))
	if err != nil {
		return nil, fmt.Errorf("%w; creation outcome is unconfirmed; inspect password metadata before retrying; "+
			"reserved output file: %s", identitySecretAPIError("create basic-auth password", err), outputPath)
	}
	if response == nil || response.StatusCode != http.StatusCreated || response.KongPasswordWithSecret == nil {
		return nil, fmt.Errorf(
			"missing password creation response; inspect metadata before retrying; reserved output: %s",
			outputPath,
		)
	}
	item := response.KongPasswordWithSecret
	knownID := "unavailable"
	if util.IsValidUUID(item.ID) {
		metadata.PasswordID, knownID = item.ID, item.ID
	}
	if err := output.Capture(metadata, item.Secret); err != nil {
		return nil, fmt.Errorf("password %q was created but capture is unconfirmed: %w; inspect %s before retrying",
			knownID, err, outputPath)
	}
	if body.Secret != nil && item.Secret != *body.Secret {
		return nil, fmt.Errorf(
			"password %q was captured but differs from supplied material; inspect the issued record",
			knownID,
		)
	}
	if !util.IsValidUUID(item.ID) {
		return nil, fmt.Errorf(
			"password response has no valid ID; material was captured in %s; inspect parent metadata",
			outputPath,
		)
	}
	created, err := passwordWriteObservation(response.RawResponse, item.ID)
	if err != nil {
		return nil, fmt.Errorf(
			"password %q was captured but its public creation fields cannot be verified: %w",
			item.ID,
			err,
		)
	}
	expiration, supplied := body.ExpiresAt.Get()
	if supplied && !samePasswordExpiration(created.ExpiresAt, expiration) ||
		!maps.Equal(created.ManagedBy, body.ManagedBy) {
		return nil, fmt.Errorf(
			"password %q was captured but requested expiration or metadata was not retained",
			item.ID,
		)
	}
	observed, err := getPasswordForWrite(ctx, api, directoryID, principalID, basicAuthID, item.ID)
	if err != nil {
		return nil, fmt.Errorf(
			"password %q was captured but read-back failed; inspect metadata before retrying: %w",
			item.ID,
			err,
		)
	}
	if !samePasswordExpiration(observed.ExpiresAt, created.ExpiresAt) ||
		!maps.Equal(observed.ManagedBy, created.ManagedBy) {
		return nil, fmt.Errorf(
			"password %q was captured but public read-back differs; inspect the issued record",
			item.ID,
		)
	}
	return observed, nil
}

// An explicit expiration is always sent: the deployed PATCH clears omission.
// Management metadata is observed and verified, never changed through PATCH.
func UpdateIdentityPasswordExpiration(ctx context.Context, api IdentityPasswordWritesAPI,
	directoryID, principalID, basicAuthID, id string, expiresAt *time.Time,
) (*IdentityPassword, error) {
	if api == nil {
		return nil, fmt.Errorf("identity password writes API is not configured")
	}
	if err := validateIdentityCredentialPath(directoryID, principalID, basicAuthID, id); err != nil {
		return nil, err
	}
	current, err := getPasswordForWrite(ctx, api, directoryID, principalID, basicAuthID, id)
	if err != nil {
		return nil, err
	}
	response, err := api.UpdatePassword(ctx, kkOps.UpdatePasswordRequest{
		DirectoryID: directoryID, PrincipalID: principalID, BasicAuthID: basicAuthID, PasswordID: id,
		UpdatePasswordBody: kkComps.UpdatePasswordBody{ExpiresAt: optionalnullable.From(expiresAt)},
	})
	if err != nil {
		return nil, identitySecretAPIError("update basic-auth password expiration", err)
	}
	if response == nil || response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("missing password expiration response; inspect metadata before retrying")
	}
	if _, err := passwordWriteObservation(response.RawResponse, id); err != nil {
		return nil, err
	}
	observed, err := getPasswordForWrite(ctx, api, directoryID, principalID, basicAuthID, id)
	if err != nil {
		return nil, err
	}
	if !samePasswordExpiration(observed.ExpiresAt, expiresAt) || !maps.Equal(observed.ManagedBy, current.ManagedBy) {
		return nil, fmt.Errorf("password expiration update did not retain requested expiration or existing metadata")
	}
	return observed, nil
}

// ReplaceIdentityPassword sends caller-supplied material once. Public read-back
// verifies expiration/metadata, not that the replacement can authenticate.
func ReplaceIdentityPassword(ctx context.Context, api IdentityPasswordWritesAPI,
	directoryID, principalID, basicAuthID, id, material string, expiresAt *time.Time, expirationSupplied bool,
) (*IdentityPassword, error) {
	if api == nil {
		return nil, fmt.Errorf("identity password writes API is not configured")
	}
	if err := validateIdentityCredentialPath(directoryID, principalID, basicAuthID, id); err != nil {
		return nil, err
	}
	if err := validateIdentityPasswordMaterial(material); err != nil {
		return nil, err
	}
	current, err := getPasswordForWrite(ctx, api, directoryID, principalID, basicAuthID, id)
	if err != nil {
		return nil, err
	}
	if !expirationSupplied {
		expiresAt = current.ExpiresAt
	}
	response, err := api.ReplacePassword(httpclient.WithoutRetries(ctx), kkOps.ReplacePasswordRequest{
		DirectoryID: directoryID, PrincipalID: principalID, BasicAuthID: basicAuthID, PasswordID: id,
		ReplacePasswordBody: kkComps.ReplacePasswordBody{
			Secret: material, ExpiresAt: expiresAt, ManagedBy: maps.Clone(current.ManagedBy),
		},
	}, kkOps.WithRetries(retry.Config{Strategy: "none"}))
	if err != nil {
		return nil, fmt.Errorf("%w; replacement outcome is unconfirmed; verify the selected record and authentication "+
			"before retrying", identitySecretAPIError("replace basic-auth password", err))
	}
	if response == nil || response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("missing password replacement response; inspect the selected record before retrying")
	}
	if _, err := passwordWriteObservation(response.RawResponse, id); err != nil {
		return nil, err
	}
	observed, err := getPasswordForWrite(ctx, api, directoryID, principalID, basicAuthID, id)
	if err != nil {
		return nil, err
	}
	if !samePasswordExpiration(observed.ExpiresAt, expiresAt) || !maps.Equal(observed.ManagedBy, current.ManagedBy) {
		return nil, fmt.Errorf("password replacement did not retain requested expiration or existing metadata")
	}
	return observed, nil
}

func DeleteIdentityPassword(ctx context.Context, api IdentityPasswordWritesAPI,
	directoryID, principalID, basicAuthID, id string,
) error {
	if api == nil {
		return fmt.Errorf("identity password writes API is not configured")
	}
	if err := validateIdentityCredentialPath(directoryID, principalID, basicAuthID, id); err != nil {
		return err
	}
	response, err := api.DeletePassword(ctx, kkOps.DeletePasswordRequest{
		DirectoryID: directoryID, PrincipalID: principalID, BasicAuthID: basicAuthID, PasswordID: id,
	})
	if err != nil {
		return identitySecretAPIError("delete basic-auth password", err)
	}
	if response == nil || response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("missing password deletion response; inspect metadata before retrying")
	}
	return nil
}

func DeleteAllIdentityPasswords(ctx context.Context, api IdentityPasswordWritesAPI,
	directoryID, principalID, basicAuthID string,
) error {
	if api == nil {
		return fmt.Errorf("identity password writes API is not configured")
	}
	if err := validateIdentityCredentialPath(directoryID, principalID, basicAuthID); err != nil {
		return err
	}
	response, err := api.DeleteAllPasswords(ctx, kkOps.DeleteAllPasswordsRequest{
		DirectoryID: directoryID, PrincipalID: principalID, BasicAuthID: basicAuthID,
	})
	if err != nil {
		return identitySecretAPIError("delete all basic-auth passwords", err)
	}
	if response == nil || response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("missing password collection deletion response; inspect metadata before retrying")
	}
	remaining, err := ListIdentityPasswords(ctx, api, directoryID, principalID, basicAuthID, 100)
	if err != nil {
		return fmt.Errorf("cannot verify password collection deletion: %w", err)
	}
	if len(remaining) != 0 {
		return fmt.Errorf("passwords are still present after collection deletion; inspect metadata before retrying")
	}
	return nil
}
