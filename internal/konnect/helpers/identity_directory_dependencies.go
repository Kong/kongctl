package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/util"
)

// These collections are deleted by directory DELETE even with force=false.
// A first unfiltered page proves emptiness only if it is also terminal.
type IdentityDirectoryDependenciesAPI interface {
	ListVaultKeys(context.Context, kkOps.ListVaultKeysRequest, ...kkOps.Option) (*kkOps.ListVaultKeysResponse, error)
	ListVaultProviders(context.Context, kkOps.ListVaultProvidersRequest, ...kkOps.Option) (
		*kkOps.ListVaultProvidersResponse, error,
	)
	ListTrustedIdps(context.Context, string, *kkComps.CursorPageParameters, ...kkOps.Option) (
		*kkOps.ListTrustedIdpsResponse, error,
	)
}

type identityDirectoryDependenciesAPI struct{ sdk *kk.SDK }

func (a *identityDirectoryDependenciesAPI) ListVaultKeys(ctx context.Context, request kkOps.ListVaultKeysRequest,
	opts ...kkOps.Option,
) (*kkOps.ListVaultKeysResponse, error) {
	return a.sdk.TokenVaultKeys.ListVaultKeys(ctx, request, opts...)
}

func (a *identityDirectoryDependenciesAPI) ListVaultProviders(ctx context.Context,
	request kkOps.ListVaultProvidersRequest, opts ...kkOps.Option,
) (*kkOps.ListVaultProvidersResponse, error) {
	return a.sdk.TokenVaultProviders.ListVaultProviders(ctx, request, opts...)
}

func (a *identityDirectoryDependenciesAPI) ListTrustedIdps(ctx context.Context, directory string,
	page *kkComps.CursorPageParameters, opts ...kkOps.Option,
) (*kkOps.ListTrustedIdpsResponse, error) {
	return a.sdk.TrustedIDPs.ListTrustedIdps(ctx, directory, page, opts...)
}

func requireEmptyIdentityCollection(response *http.Response, owner, resource string) error {
	var page identityCredentialPage[json.RawMessage]
	if err := decodeIdentityCredentialResponse(response, &page); err != nil {
		return fmt.Errorf("cannot observe %s %s: %w", owner, resource, err)
	}
	if len(page.Data) != 0 {
		return fmt.Errorf("%s has attached %s; remove them explicitly before deleting the %s", owner, resource, owner)
	}
	if page.Meta == nil || page.Meta.Page == nil || page.Meta.Page.Size == nil ||
		math.Trunc(*page.Meta.Page.Size) != *page.Meta.Page.Size {
		return fmt.Errorf("cannot prove an empty %s %s collection: invalid pagination", owner, resource)
	}
	next, err := page.next()
	if err != nil {
		return fmt.Errorf("cannot prove an empty %s %s collection: %w", owner, resource, err)
	}
	if next != nil && *next != "" {
		return fmt.Errorf("cannot prove an empty %s %s collection: observation is not terminal", owner, resource)
	}
	return nil
}

func RequireEmptyIdentityDirectoryVault(ctx context.Context, api IdentityDirectoryDependenciesAPI,
	directory string, vaultEnabled *bool,
) error {
	return RequireIdentityDirectoryVaultRetirements(ctx, api, directory, vaultEnabled, nil)
}

// RequireIdentityDirectoryVaultRetirements permits only explicitly planned key
// removals during planning. Execution still requires empty collections through
// RequireEmptyIdentityDirectoryVault after the ordered key deletes.
func RequireIdentityDirectoryVaultRetirements(ctx context.Context, api IdentityDirectoryDependenciesAPI,
	directory string, vaultEnabled *bool, keysToRemove map[string]string,
) error {
	if !util.IsValidUUID(directory) {
		return fmt.Errorf("directory ID must be a UUID")
	}
	if vaultEnabled == nil || !*vaultEnabled {
		return fmt.Errorf("cannot verify directory Vault dependencies while vault_enabled is false or unknown; " +
			"explicitly enable Vault and inspect/remove its dependencies before deleting the directory")
	}
	if api == nil {
		return fmt.Errorf("directory Vault dependency APIs are not configured; deletion is blocked")
	}
	if len(keysToRemove) > 0 {
		keys, err := ListIdentityVaultKeys(ctx, api, directory, 100)
		if err != nil {
			return err
		}
		for _, key := range keys {
			if keysToRemove[key.ID] != key.Kid {
				return fmt.Errorf("directory has attached Vault keys without explicit planned deletion")
			}
		}
	} else {
		keys, err := api.ListVaultKeys(ctx, kkOps.ListVaultKeysRequest{DirectoryID: directory})
		if err != nil {
			return identitySecretAPIError("observe directory Vault keys before deletion", err)
		}
		if keys == nil {
			return fmt.Errorf("missing directory Vault key collection response; deletion is blocked")
		}
		if err := requireEmptyIdentityCollection(keys.RawResponse, "directory", "Vault keys"); err != nil {
			return err
		}
	}
	providers, err := api.ListVaultProviders(ctx, kkOps.ListVaultProvidersRequest{DirectoryID: directory})
	if err != nil {
		return identitySecretAPIError("observe directory Vault providers before deletion", err)
	}
	if providers == nil {
		return fmt.Errorf("missing directory Vault provider collection response; deletion is blocked")
	}
	if err := requireEmptyIdentityCollection(providers.RawResponse, "directory", "Vault providers"); err != nil {
		return err
	}
	idps, err := api.ListTrustedIdps(ctx, directory, nil)
	if err != nil {
		return identitySecretAPIError("observe directory Trusted IdPs before deletion", err)
	}
	if idps == nil {
		return fmt.Errorf("missing directory Trusted IdP collection response; deletion is blocked")
	}
	return requireEmptyIdentityCollection(idps.RawResponse, "directory", "Trusted IdPs")
}
