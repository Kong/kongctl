package helpers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/konnect/identityproviders"
	"github.com/kong/kongctl/internal/secretoutput"
	"github.com/kong/kongctl/internal/util"
)

// Ordinary metadata readers intentionally lack detail GET. This separate API
// exposes it only for deliberate enrollment-URL issuance into private capture.
type IdentityVaultEnrollmentAPI interface {
	IdentityVaultProvidersReader
	IdentityVaultProviderCredentialsReader
	CreateVaultProviderCredential(
		context.Context,
		kkOps.CreateVaultProviderCredentialRequest,
		...kkOps.Option,
	) (*kkOps.CreateVaultProviderCredentialResponse, error)
	GetVaultProviderCredential(
		context.Context,
		kkOps.GetVaultProviderCredentialRequest,
		...kkOps.Option,
	) (*kkOps.GetVaultProviderCredentialResponse, error)
	DeleteVaultProviderCredential(
		context.Context,
		kkOps.DeleteVaultProviderCredentialRequest,
		...kkOps.Option,
	) (*kkOps.DeleteVaultProviderCredentialResponse, error)
}

func observeIdentitySharedVaultCredentials(ctx context.Context, api IdentityVaultEnrollmentAPI,
	scope IdentityVaultCredentialScope,
) ([]IdentityVaultProviderCredential, string, error) {
	if api == nil {
		return nil, "", fmt.Errorf("vault enrollment API is not configured")
	}
	if !util.IsValidUUID(scope.DirectoryID) || !util.IsValidUUID(scope.ProviderID) {
		return nil, "", fmt.Errorf("directory and provider IDs must be UUIDs")
	}
	if err := identityproviders.ValidateName(scope.ProviderName); err != nil {
		return nil, "", err
	}
	provider, err := GetIdentityVaultProvider(ctx, api, scope.DirectoryID, scope.ProviderID)
	if err != nil {
		return nil, "", err
	}
	if provider.Name != scope.ProviderName || provider.CredentialType != "shared" {
		return nil, "", fmt.Errorf("selected provider identity or shared credential mode changed")
	}
	template, err := GetIdentityCredentialProviderTemplate(ctx, api, provider.TemplateName)
	if err != nil {
		return nil, "", err
	}
	if template.SecretType != "authorization_code" && template.SecretType != "static_secret" {
		return nil, "", fmt.Errorf("unsupported shared provider credential template")
	}
	values, err := ListIdentityVaultProviderCredentials(ctx, api, scope.DirectoryID, scope.ProviderID, 100)
	if err != nil {
		return nil, "", err
	}
	if err := validateIdentitySharedVaultCredentialSlots(values); err != nil {
		return nil, "", err
	}
	return values, template.SecretType, nil
}

func validateIdentitySharedVaultCredentialSlots(values []IdentityVaultProviderCredential) error {
	priorities, hasDefault := make(map[int64]bool), false
	for _, value := range values {
		if value.CredentialType != "shared" || (value.Status != "pending" && value.Status != "active") {
			return fmt.Errorf("provider contains an unexpected credential type or status")
		}
		if value.Priority == nil {
			if hasDefault {
				return fmt.Errorf("multiple default credentials; inspect before writing")
			}
			hasDefault = true
		} else {
			if priorities[*value.Priority] {
				return fmt.Errorf("multiple credentials at one priority; inspect before writing")
			}
			priorities[*value.Priority] = true
		}
	}
	return nil
}

func findIdentitySharedVaultCredential(
	values []IdentityVaultProviderCredential,
	id string,
) (*IdentityVaultProviderCredential, error) {
	for _, value := range values {
		if value.ID == id {
			return &value, nil
		}
	}
	return nil, fmt.Errorf("credential is absent from the selected provider collection")
}

// CreateAndCaptureIdentityVaultEnrollment creates a pending shared OAuth record.
// It neither completes consent nor verifies runtime authorization.
func CreateAndCaptureIdentityVaultEnrollment(ctx context.Context, api IdentityVaultEnrollmentAPI,
	scope IdentityVaultCredentialScope, routing IdentityVaultCredentialRouting, outputPath string,
) (*IdentityVaultProviderCredential, error) {
	if err := routing.Validate(); err != nil {
		return nil, err
	}
	values, kind, err := observeIdentitySharedVaultCredentials(ctx, api, scope)
	if err != nil {
		return nil, err
	}
	if kind != "authorization_code" {
		return nil, fmt.Errorf(
			"enrollment creation requires a shared OAuth provider; use declarations for static credentials",
		)
	}
	if err := requireIdentityVaultSlotAvailable(values, routing, ""); err != nil {
		return nil, err
	}
	metadata := secretoutput.Metadata{DirectoryID: scope.DirectoryID, ProviderID: scope.ProviderID}
	output, err := secretoutput.Reserve(outputPath, metadata)
	if err != nil {
		return nil, err
	}
	defer func() { _ = output.Close() }()
	response, err := api.CreateVaultProviderCredential(
		httpclient.WithoutRetries(ctx),
		kkOps.CreateVaultProviderCredentialRequest{
			DirectoryID: scope.DirectoryID, ProviderID: scope.ProviderID,
			SetVaultProviderCredentialBody: kkComps.SetVaultProviderCredentialBody{
				Selector: routing.Selector, Priority: routing.Priority,
			},
		},
		kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"%w; creation is unconfirmed; inspect metadata before retrying; reserved output: %s",
			identitySecretAPIError("create OAuth credential", err),
			outputPath,
		)
	}
	if response == nil || response.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf(
			"missing created OAuth credential response; inspect metadata before retrying; reserved output: %s",
			outputPath,
		)
	}
	return captureIdentityVaultEnrollment(ctx, api, scope, response.RawResponse, output, metadata, routing, nil)
}

// CaptureIdentityVaultEnrollment explicitly issues a URL through detail GET.
// Even this GET is single-attempt: retrying could issue another capability.
func CaptureIdentityVaultEnrollment(ctx context.Context, api IdentityVaultEnrollmentAPI,
	scope IdentityVaultCredentialScope, id, outputPath string,
) (*IdentityVaultProviderCredential, error) {
	if !util.IsValidUUID(id) {
		return nil, fmt.Errorf("credential ID must be a UUID")
	}
	values, kind, err := observeIdentitySharedVaultCredentials(ctx, api, scope)
	if err != nil {
		return nil, err
	}
	if kind != "authorization_code" {
		return nil, fmt.Errorf("enrollment issuance requires a shared OAuth provider")
	}
	before, err := findIdentitySharedVaultCredential(values, id)
	if err != nil {
		return nil, err
	}
	metadata := secretoutput.Metadata{DirectoryID: scope.DirectoryID, ProviderID: scope.ProviderID, CredentialID: id}
	output, err := secretoutput.Reserve(outputPath, metadata)
	if err != nil {
		return nil, err
	}
	defer func() { _ = output.Close() }()
	response, err := api.GetVaultProviderCredential(
		httpclient.WithoutRetries(ctx),
		kkOps.GetVaultProviderCredentialRequest{
			DirectoryID: scope.DirectoryID, ProviderID: scope.ProviderID, CredentialID: id,
		},
		kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"%w; URL issuance is unconfirmed; reserved output: %s",
			identitySecretAPIError("issue OAuth enrollment URL", err),
			outputPath,
		)
	}
	if response == nil || response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("missing enrollment response; issuance is unconfirmed; reserved output: %s", outputPath)
	}
	return captureIdentityVaultEnrollment(
		ctx,
		api,
		scope,
		response.RawResponse,
		output,
		metadata,
		credentialRouting(*before),
		before,
	)
}

func captureIdentityVaultEnrollment(
	ctx context.Context,
	api IdentityVaultProviderCredentialsReader,
	scope IdentityVaultCredentialScope,
	response *http.Response,
	output *secretoutput.File,
	metadata secretoutput.Metadata,
	routing IdentityVaultCredentialRouting,
	before *IdentityVaultProviderCredential,
) (*IdentityVaultProviderCredential, error) {
	var envelope struct {
		IdentityVaultProviderCredential
		EnrollmentURL string `json:"enrollment_url"`
	}
	if err := decodeIdentityVaultProviderResponse(response, &envelope); err != nil {
		return nil, fmt.Errorf(
			"enrollment response could not be decoded; capture is unconfirmed; inspect metadata before retrying",
		)
	}
	if util.IsValidUUID(envelope.ID) {
		metadata.CredentialID = envelope.ID
	}
	if err := output.CaptureEnrollmentURL(metadata, envelope.EnrollmentURL); err != nil {
		return nil, fmt.Errorf(
			"credential %q enrollment capture is unconfirmed: %w; inspect the reserved file before retrying",
			metadata.CredentialID,
			err,
		)
	}
	u, err := url.Parse(envelope.EnrollmentURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return nil, fmt.Errorf("enrollment URL was captured but its format is invalid; inspect the private file")
	}
	receipt := envelope.IdentityVaultProviderCredential
	if !util.IsValidUUID(receipt.ID) || receipt.CredentialType != "shared" ||
		(receipt.Status != "pending" && receipt.Status != "active") ||
		receipt.CreatedAt.IsZero() || receipt.UpdatedAt.IsZero() || !credentialRouting(receipt).equals(routing) ||
		before == nil && receipt.Status != "pending" ||
		before != nil && (receipt.ID != before.ID || !receipt.CreatedAt.Equal(before.CreatedAt)) {
		return nil, fmt.Errorf("enrollment URL was captured but credential metadata differs; inspect before retrying")
	}
	values, err := ListIdentityVaultProviderCredentials(ctx, api, scope.DirectoryID, scope.ProviderID, 100)
	if err != nil {
		return nil, fmt.Errorf("enrollment URL was captured but metadata readback failed: %w", err)
	}
	if err := validateIdentitySharedVaultCredentialSlots(values); err != nil {
		return nil, err
	}
	observed, err := findIdentitySharedVaultCredential(values, receipt.ID)
	if err != nil {
		return nil, err
	}
	if !credentialRouting(*observed).equals(routing) || !observed.CreatedAt.Equal(receipt.CreatedAt) {
		return nil, fmt.Errorf("enrollment URL was captured but public readback differs; inspect before retrying")
	}
	return observed, nil
}

// DeleteIdentitySharedVaultCredential deliberately excludes user authorizations.
// It rechecks scope through public observations and never issues an enrollment URL.
func DeleteIdentitySharedVaultCredential(ctx context.Context, api IdentityVaultEnrollmentAPI,
	scope IdentityVaultCredentialScope, id string,
) error {
	if !util.IsValidUUID(id) {
		return fmt.Errorf("credential ID must be a UUID")
	}
	values, _, err := observeIdentitySharedVaultCredentials(ctx, api, scope)
	if err != nil {
		return err
	}
	if _, err := findIdentitySharedVaultCredential(values, id); err != nil {
		return err
	}
	response, err := api.DeleteVaultProviderCredential(
		httpclient.WithoutRetries(ctx),
		kkOps.DeleteVaultProviderCredentialRequest{
			DirectoryID: scope.DirectoryID, ProviderID: scope.ProviderID, CredentialID: id,
		},
		kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}),
	)
	if err != nil {
		return identitySecretAPIError("delete shared Vault credential", err)
	}
	if response == nil || response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("missing deleted shared credential response; inspect before retrying")
	}
	values, err = ListIdentityVaultProviderCredentials(ctx, api, scope.DirectoryID, scope.ProviderID, 100)
	if err != nil {
		return err
	}
	for _, value := range values {
		if value.ID == id {
			return fmt.Errorf("shared credential remains after deletion; inspect before retrying")
		}
	}
	return nil
}
