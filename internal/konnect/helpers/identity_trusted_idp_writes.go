package helpers

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"reflect"
	"unicode/utf8"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/util"
)

// NormalizeIdentityTrustedIDPJIT follows the API's empty-mapping semantics.
// It returns a copy and preserves explicitly configured claim names.
func NormalizeIdentityTrustedIDPJIT(value *kkComps.TrustedIdpJit) *kkComps.TrustedIdpJit {
	if value == nil || value.Mapping == nil {
		return nil
	}
	mapping := *value.Mapping
	if len(mapping.Metadata) == 0 {
		mapping.Metadata = nil
	} else {
		mapping.Metadata = maps.Clone(mapping.Metadata)
	}
	if mapping.Metadata == nil && mapping.Email == nil && mapping.Groups == nil && mapping.DisplayName == nil {
		return nil
	}
	return &kkComps.TrustedIdpJit{Mapping: &mapping}
}

func validateIdentityTrustedIDPPublic(issuer string, display, jwks *string, jit *kkComps.TrustedIdpJit) error {
	parsed, err := url.Parse(issuer)
	if err != nil || !parsed.IsAbs() {
		return fmt.Errorf("trusted IdP issuer_url must be an absolute URI")
	}
	if display != nil && utf8.RuneCountInString(*display) > 255 {
		return fmt.Errorf("trusted IdP display_name must be at most 255 characters")
	}
	if jwks != nil {
		parsed, err := url.Parse(*jwks)
		if err != nil || !parsed.IsAbs() {
			return fmt.Errorf("trusted IdP jwks_uri must be an absolute URI")
		}
	}
	if jit != nil && jit.Mapping != nil && len(jit.Mapping.Metadata) > 64 {
		return fmt.Errorf("trusted IdP JIT metadata mapping allows at most 64 entries")
	}
	return nil
}

func validateIdentityTrustedIDPWriteScope(api IdentityTrustedIDPsReader, directory string) error {
	if api == nil {
		return fmt.Errorf("identity trusted IdPs API is not configured")
	}
	if !util.IsValidUUID(directory) {
		return fmt.Errorf("directory ID must be a UUID")
	}
	return nil
}

// CreateIdentityTrustedIDP makes one create attempt and verifies the public
// configuration. Reachable signing keys/discovery are validated by the API.
func CreateIdentityTrustedIDP(ctx context.Context, api IdentityTrustedIDPsAPI, directory string,
	request kkComps.CreateTrustedIdpPayload,
) (*IdentityTrustedIDP, error) {
	if err := validateIdentityTrustedIDPWriteScope(api, directory); err != nil {
		return nil, err
	}
	if err := validateIdentityTrustedIDPPublic(
		request.IssuerURL, request.DisplayName, request.JwksURI, request.Jit,
	); err != nil {
		return nil, err
	}
	response, err := api.CreateTrustedIdp(httpclient.WithoutRetries(ctx), directory, request,
		kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}))
	if err != nil {
		return nil, identitySecretAPIError("create trusted IdP", err)
	}
	if response == nil || response.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("missing or unsuccessful created trusted IdP response")
	}
	var value IdentityTrustedIDP
	if err := decodeIdentityTrustedIDPResponse(response.RawResponse, &value); err != nil {
		return nil, err
	}
	if value.IssuerURL != request.IssuerURL || value.CreatedAt.IsZero() || value.UpdatedAt.IsZero() ||
		!providerOptionalEqual(
			request.DisplayName,
			value.DisplayName,
		) || !providerOptionalEqual(request.JwksURI, value.JwksURI) ||
		!providerOptionalEqual(request.JitProvisioningEnabled, value.JitProvisioningEnabled) ||
		!providerOptionalEqual(request.VaultAccessEnabled, value.VaultAccessEnabled) ||
		!reflect.DeepEqual(NormalizeIdentityTrustedIDPJIT(request.Jit), NormalizeIdentityTrustedIDPJIT(value.Jit)) {
		return nil, fmt.Errorf(
			"created trusted IdP public configuration differs from the request; inspect before retrying",
		)
	}
	return &value, nil
}

// ObserveIdentityTrustedIDPScope requires full directory membership proof; the
// response has no directory_id with which to validate a detail read alone.
func ObserveIdentityTrustedIDPScope(
	ctx context.Context,
	api IdentityTrustedIDPsReader,
	directory, id, issuer string,
) (*IdentityTrustedIDP, error) {
	if err := validateIdentityTrustedIDPWriteScope(api, directory); err != nil {
		return nil, err
	}
	if !util.IsValidUUID(id) || issuer == "" {
		return nil, fmt.Errorf("trusted IdP scope requires a UUID and issuer")
	}
	values, err := ListIdentityTrustedIDPs(ctx, api, directory, 100)
	if err != nil {
		return nil, err
	}
	seenIssuer := make(map[string]bool)
	found := false
	for _, value := range values {
		if seenIssuer[value.IssuerURL] {
			return nil, fmt.Errorf("trusted IdP collection contains a duplicate issuer")
		}
		seenIssuer[value.IssuerURL] = true
		if value.ID == id {
			if value.IssuerURL != issuer {
				return nil, fmt.Errorf("trusted IdP issuer changed; regenerate the plan")
			}
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("trusted IdP is not in the selected directory; regenerate the plan")
	}
	current, err := GetIdentityTrustedIDP(ctx, api, directory, id)
	if err != nil {
		return nil, err
	}
	if current.IssuerURL != issuer {
		return nil, fmt.Errorf("trusted IdP issuer changed; regenerate the plan")
	}
	return current, nil
}

// ReplaceIdentityTrustedIDP requires a full hydrated representation. Callers
// preserve omitted declarations from fresh state and explicitly select clears.
// This helper does not authorize ownership or change the selected issuer.
func ReplaceIdentityTrustedIDP(ctx context.Context, api IdentityTrustedIDPsAPI, directory, id, issuer string,
	request kkComps.ReplaceTrustedIdpBody,
) (*IdentityTrustedIDP, error) {
	if err := validateIdentityTrustedIDPPublic(
		request.IssuerURL, request.DisplayName, request.JwksURI, request.Jit,
	); err != nil {
		return nil, err
	}
	if request.IssuerURL != issuer {
		return nil, fmt.Errorf("trusted IdP issuer changes require a separate resource")
	}
	if request.JitProvisioningEnabled == nil || request.VaultAccessEnabled == nil {
		return nil, fmt.Errorf("trusted IdP replacement requires hydrated JIT and Vault access booleans")
	}
	if _, err := ObserveIdentityTrustedIDPScope(ctx, api, directory, id, issuer); err != nil {
		return nil, err
	}
	response, err := api.UpdateTrustedIdp(httpclient.WithoutRetries(ctx), kkOps.UpdateTrustedIdpRequest{
		DirectoryID: directory, TrustedIdpID: id, ReplaceTrustedIdpBody: request,
	}, kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}))
	if err != nil {
		return nil, identitySecretAPIError("replace trusted IdP", err)
	}
	if response == nil || response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("missing or unsuccessful replaced trusted IdP response")
	}
	var returned IdentityTrustedIDP
	if err := decodeIdentityTrustedIDPResponse(response.RawResponse, &returned); err != nil {
		return nil, err
	}
	if err := verifyIdentityTrustedIDPReplacement(returned, id, request); err != nil {
		return nil, err
	}
	current, err := GetIdentityTrustedIDP(ctx, api, directory, id)
	if err != nil {
		return nil, err
	}
	if err := verifyIdentityTrustedIDPReplacement(*current, id, request); err != nil {
		return nil, err
	}
	return current, nil
}

func verifyIdentityTrustedIDPReplacement(
	value IdentityTrustedIDP,
	id string,
	request kkComps.ReplaceTrustedIdpBody,
) error {
	if value.ID != id || value.IssuerURL != request.IssuerURL || value.CreatedAt.IsZero() || value.UpdatedAt.IsZero() ||
		!providerNullableEqual(
			value.DisplayName,
			request.DisplayName,
		) || !providerNullableEqual(value.JwksURI, request.JwksURI) ||
		!providerNullableEqual(value.JitProvisioningEnabled, request.JitProvisioningEnabled) ||
		!providerNullableEqual(value.VaultAccessEnabled, request.VaultAccessEnabled) ||
		!reflect.DeepEqual(NormalizeIdentityTrustedIDPJIT(value.Jit), NormalizeIdentityTrustedIDPJIT(request.Jit)) {
		return fmt.Errorf(
			"replaced trusted IdP public configuration was not preserved as requested; inspect before retrying",
		)
	}
	return nil
}

// RequireEmptyIdentityTrustedIDPPrincipals never assumes ownership of records
// associated through OIDC or provisioning. Empty must be explicitly terminal.
func RequireEmptyIdentityTrustedIDPPrincipals(
	ctx context.Context,
	api IdentityTrustedIDPsReader,
	directory, id string,
) error {
	if err := validateIdentityTrustedIDPWriteScope(api, directory); err != nil {
		return err
	}
	if !util.IsValidUUID(id) {
		return fmt.Errorf("trusted IdP ID must be a UUID")
	}
	response, err := api.ListTrustedIdpPrincipals(
		ctx,
		kkOps.ListTrustedIdpPrincipalsRequest{DirectoryID: directory, TrustedIdpID: id},
	)
	if err != nil {
		return identitySecretAPIError("observe trusted IdP principals before deletion", err)
	}
	if response == nil || response.StatusCode != http.StatusOK {
		return fmt.Errorf("missing trusted IdP principal observation; deletion is blocked")
	}
	return requireEmptyIdentityCollection(response.RawResponse, "trusted IdP", "associated principals")
}

// DeleteIdentityTrustedIDP requires caller-established directory authority and
// explicit detachment. Observations cannot exclude a concurrent association.
func DeleteIdentityTrustedIDP(ctx context.Context, api IdentityTrustedIDPsAPI, directory, id, issuer string) error {
	if _, err := ObserveIdentityTrustedIDPScope(ctx, api, directory, id, issuer); err != nil {
		return err
	}
	if err := RequireEmptyIdentityTrustedIDPPrincipals(ctx, api, directory, id); err != nil {
		return err
	}
	response, err := api.DeleteTrustedIdp(httpclient.WithoutRetries(ctx), directory, id,
		kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}))
	if err != nil {
		return identitySecretAPIError("delete trusted IdP", err)
	}
	if response == nil || response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("missing or unsuccessful deleted trusted IdP response")
	}
	return nil
}
