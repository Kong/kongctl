package helpers

import (
	"context"
	"fmt"
	"net/http"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/konnect/identityproviders"
	"github.com/kong/kongctl/internal/util"
)

// Credential detail GET is intentionally absent: it can issue OAuth enrollment
// capabilities. These writes support only static_secret/shared providers.
type IdentityStaticVaultCredentialsAPI interface {
	IdentityVaultProvidersReader
	IdentityVaultProviderCredentialsReader
	CreateVaultProviderCredential(context.Context, kkOps.CreateVaultProviderCredentialRequest, ...kkOps.Option) (
		*kkOps.CreateVaultProviderCredentialResponse, error,
	)
	UpdateVaultProviderCredential(context.Context, kkOps.UpdateVaultProviderCredentialRequest, ...kkOps.Option) (
		*kkOps.UpdateVaultProviderCredentialResponse, error,
	)
	DeleteVaultProviderCredential(context.Context, kkOps.DeleteVaultProviderCredentialRequest, ...kkOps.Option) (
		*kkOps.DeleteVaultProviderCredentialResponse, error,
	)
}

type IdentityStaticVaultCredentialScope = IdentityVaultCredentialScope

// A nil pair denotes the default slot. A selector requires a nonnegative
// priority; priorities, rather than selectors, identify nondefault slots.
type IdentityVaultCredentialRouting struct {
	Selector *string
	Priority *int64
}

func (r IdentityVaultCredentialRouting) Validate() error {
	if (r.Selector == nil) != (r.Priority == nil) || r.Priority != nil && *r.Priority < 0 {
		return fmt.Errorf("credential selector and nonnegative priority must be supplied together")
	}
	return nil
}

func (r IdentityVaultCredentialRouting) equals(other IdentityVaultCredentialRouting) bool {
	return providerNullableEqual(r.Selector, other.Selector) && providerNullableEqual(r.Priority, other.Priority)
}

func credentialRouting(value IdentityVaultProviderCredential) IdentityVaultCredentialRouting {
	return IdentityVaultCredentialRouting{Selector: value.Selector, Priority: value.Priority}
}

// Authority and parent protection must be established by the caller. This
// helper independently verifies the fresh API scope and static provider mode.
func observeIdentityStaticVaultCredentials(ctx context.Context, api IdentityStaticVaultCredentialsAPI,
	scope IdentityStaticVaultCredentialScope,
) ([]IdentityVaultProviderCredential, error) {
	if api == nil {
		return nil, fmt.Errorf("static Vault credential API is not configured")
	}
	if !util.IsValidUUID(scope.DirectoryID) || !util.IsValidUUID(scope.ProviderID) {
		return nil, fmt.Errorf("directory and provider IDs must be UUIDs")
	}
	if err := identityproviders.ValidateName(scope.ProviderName); err != nil {
		return nil, err
	}
	provider, err := GetIdentityVaultProvider(ctx, api, scope.DirectoryID, scope.ProviderID)
	if err != nil {
		return nil, err
	}
	if provider.Name != scope.ProviderName ||
		provider.TemplateName != string(kkComps.ProviderTemplateNameStaticSecret) ||
		provider.CredentialType != "shared" {
		return nil, fmt.Errorf("provider identity or static shared mode changed; regenerate the plan")
	}
	return listIdentityStaticVaultCredentials(ctx, api, scope)
}

func listIdentityStaticVaultCredentials(ctx context.Context, api IdentityVaultProviderCredentialsReader,
	scope IdentityStaticVaultCredentialScope,
) ([]IdentityVaultProviderCredential, error) {
	values, err := ListIdentityVaultProviderCredentials(ctx, api, scope.DirectoryID, scope.ProviderID, 100)
	if err != nil {
		return nil, err
	}
	priorities, hasDefault := make(map[int64]bool), false
	for _, value := range values {
		if value.CredentialType != "shared" || value.Status != "active" {
			return nil, fmt.Errorf(
				"static provider has an unexpected credential type or status; inspect before writing",
			)
		}
		if value.Priority == nil {
			if hasDefault {
				return nil, fmt.Errorf("multiple default credentials; inspect before writing")
			}
			hasDefault = true
		} else {
			if priorities[*value.Priority] {
				return nil, fmt.Errorf("multiple credentials at one routing priority; inspect before writing")
			}
			priorities[*value.Priority] = true
		}
	}
	return values, nil
}

func requireIdentityStaticVaultCredential(values []IdentityVaultProviderCredential, id string,
	expected IdentityVaultCredentialRouting,
) (*IdentityVaultProviderCredential, error) {
	for _, value := range values {
		if value.ID == id {
			if !credentialRouting(value).equals(expected) {
				return nil, fmt.Errorf("credential routing changed; regenerate the plan")
			}
			return &value, nil
		}
	}
	return nil, fmt.Errorf("credential is absent from the selected provider; regenerate the plan")
}

func requireIdentityVaultSlotAvailable(values []IdentityVaultProviderCredential,
	routing IdentityVaultCredentialRouting, exceptID string,
) error {
	for _, value := range values {
		if value.ID != exceptID && providerNullableEqual(value.Priority, routing.Priority) {
			return fmt.Errorf("credential routing slot is occupied; regenerate the plan")
		}
	}
	return nil
}

// CreateIdentityStaticVaultCredential consumes explicitly supplied material in
// one POST attempt. Its public receipt/readback cannot prove stored material.
func CreateIdentityStaticVaultCredential(ctx context.Context, api IdentityStaticVaultCredentialsAPI,
	scope IdentityStaticVaultCredentialScope, routing IdentityVaultCredentialRouting, secret string,
) (*IdentityVaultProviderCredential, error) {
	if err := routing.Validate(); err != nil {
		return nil, err
	}
	values, err := observeIdentityStaticVaultCredentials(ctx, api, scope)
	if err != nil {
		return nil, err
	}
	if err := requireIdentityVaultSlotAvailable(values, routing, ""); err != nil {
		return nil, err
	}
	response, err := api.CreateVaultProviderCredential(httpclient.WithoutRetries(ctx),
		kkOps.CreateVaultProviderCredentialRequest{
			DirectoryID: scope.DirectoryID, ProviderID: scope.ProviderID,
			SetVaultProviderCredentialBody: kkComps.SetVaultProviderCredentialBody{
				Secret: &secret, Selector: routing.Selector, Priority: routing.Priority,
			},
		}, kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}))
	if err != nil {
		return nil, identitySecretAPIError("create static Vault credential", err)
	}
	if response == nil || response.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf(
			"missing or unsuccessful created static Vault credential response; inspect before retrying",
		)
	}
	return verifyIdentityStaticVaultCredentialWrite(ctx, api, scope, response.RawResponse, routing, nil)
}

// ReplaceIdentityStaticVaultCredential always requires explicit secret material.
// A nil routing argument preserves freshly observed routing; a nonnil empty
// pair explicitly selects the default slot. expected detects stale saved plans.
func ReplaceIdentityStaticVaultCredential(ctx context.Context, api IdentityStaticVaultCredentialsAPI,
	scope IdentityStaticVaultCredentialScope, id string, expected IdentityVaultCredentialRouting,
	routing *IdentityVaultCredentialRouting, secret string,
) (*IdentityVaultProviderCredential, error) {
	if !util.IsValidUUID(id) {
		return nil, fmt.Errorf("credential ID must be a UUID")
	}
	if err := expected.Validate(); err != nil {
		return nil, err
	}
	if routing != nil {
		if err := routing.Validate(); err != nil {
			return nil, err
		}
	}
	values, err := observeIdentityStaticVaultCredentials(ctx, api, scope)
	if err != nil {
		return nil, err
	}
	before, err := requireIdentityStaticVaultCredential(values, id, expected)
	if err != nil {
		return nil, err
	}
	desired := credentialRouting(*before)
	if routing != nil {
		desired = *routing
	}
	if err := requireIdentityVaultSlotAvailable(values, desired, id); err != nil {
		return nil, err
	}
	response, err := api.UpdateVaultProviderCredential(httpclient.WithoutRetries(ctx),
		kkOps.UpdateVaultProviderCredentialRequest{
			DirectoryID: scope.DirectoryID, ProviderID: scope.ProviderID, CredentialID: id,
			SetVaultProviderCredentialBody: kkComps.SetVaultProviderCredentialBody{
				Secret: &secret, Selector: desired.Selector, Priority: desired.Priority,
			},
		}, kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}))
	if err != nil {
		return nil, identitySecretAPIError("replace static Vault credential", err)
	}
	if response == nil || response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"missing or unsuccessful replaced static Vault credential response; inspect before retrying",
		)
	}
	return verifyIdentityStaticVaultCredentialWrite(ctx, api, scope, response.RawResponse, desired, before)
}

func verifyIdentityStaticVaultCredentialWrite(ctx context.Context, api IdentityVaultProviderCredentialsReader,
	scope IdentityStaticVaultCredentialScope, response *http.Response, routing IdentityVaultCredentialRouting,
	before *IdentityVaultProviderCredential,
) (*IdentityVaultProviderCredential, error) {
	var receipt IdentityVaultProviderCredential
	if err := decodeIdentityVaultProviderResponse(response, &receipt); err != nil {
		return nil, err
	}
	if !util.IsValidUUID(receipt.ID) || receipt.CredentialType != "shared" || receipt.Status != "active" ||
		receipt.CreatedAt.IsZero() || receipt.UpdatedAt.IsZero() || !credentialRouting(receipt).equals(routing) ||
		before != nil && (receipt.ID != before.ID || !receipt.CreatedAt.Equal(before.CreatedAt)) {
		return nil, fmt.Errorf("static credential public receipt differs from the request; inspect before retrying")
	}
	values, err := listIdentityStaticVaultCredentials(ctx, api, scope)
	if err != nil {
		return nil, err
	}
	observed, err := requireIdentityStaticVaultCredential(values, receipt.ID, routing)
	if err != nil {
		return nil, err
	}
	if !observed.CreatedAt.Equal(receipt.CreatedAt) {
		return nil, fmt.Errorf("static credential readback identity changed; inspect before retrying")
	}
	return observed, nil
}

func DeleteIdentityStaticVaultCredential(ctx context.Context, api IdentityStaticVaultCredentialsAPI,
	scope IdentityStaticVaultCredentialScope, id string, expected IdentityVaultCredentialRouting,
) error {
	if !util.IsValidUUID(id) {
		return fmt.Errorf("credential ID must be a UUID")
	}
	if err := expected.Validate(); err != nil {
		return err
	}
	values, err := observeIdentityStaticVaultCredentials(ctx, api, scope)
	if err != nil {
		return err
	}
	if _, err := requireIdentityStaticVaultCredential(values, id, expected); err != nil {
		return err
	}
	response, err := api.DeleteVaultProviderCredential(httpclient.WithoutRetries(ctx),
		kkOps.DeleteVaultProviderCredentialRequest{
			DirectoryID: scope.DirectoryID, ProviderID: scope.ProviderID, CredentialID: id,
		}, kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}))
	if err != nil {
		return identitySecretAPIError("delete static Vault credential", err)
	}
	if response == nil || response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("missing or unsuccessful deleted static Vault credential response; inspect before retrying")
	}
	values, err = listIdentityStaticVaultCredentials(ctx, api, scope)
	if err != nil {
		return err
	}
	for _, value := range values {
		if value.ID == id {
			return fmt.Errorf("static credential remains after deletion; inspect before retrying")
		}
	}
	return nil
}
