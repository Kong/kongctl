package state

import (
	"context"
	"fmt"
	"net/http"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	declerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

// IdentityDirectory keeps public labels separate from projected lifecycle metadata.
type IdentityDirectory struct {
	kkComps.KongDirectory
	NormalizedLabels map[string]string
}

func (d IdentityDirectory) GetNormalizedLabels() map[string]string { return d.NormalizedLabels }

func NormalizeIdentityDirectory(value kkComps.KongDirectory) IdentityDirectory {
	var ownership map[string]string
	if value.ManagedBy[resources.ManagedByToolKey] == resources.ManagedByTool {
		ownership = resources.NormalizedChildOwnership(value.ManagedBy)
	}
	return IdentityDirectory{KongDirectory: value, NormalizedLabels: ownership}
}

func (c *Client) ListIdentityDirectories(ctx context.Context) ([]IdentityDirectory, error) {
	if err := ValidateAPIClient(c.identityDirectoriesAPI, "Kong Identity directories API"); err != nil {
		return nil, err
	}
	values, err := helpers.ListIdentityDirectories(ctx, c.identityDirectoriesAPI, 100)
	if err != nil {
		return nil, err
	}
	result := make([]IdentityDirectory, 0, len(values))
	for _, value := range values {
		result = append(result, NormalizeIdentityDirectory(value))
	}
	return result, nil
}

func (c *Client) GetIdentityDirectoryByName(ctx context.Context, name string) (*IdentityDirectory, error) {
	values, err := c.ListIdentityDirectories(ctx)
	if err != nil {
		return nil, err
	}
	var result *IdentityDirectory
	for i := range values {
		if values[i].Name != name {
			continue
		}
		if result != nil {
			return nil, fmt.Errorf("multiple Kong Identity directories have name %q", name)
		}
		result = &values[i]
	}
	return result, nil
}

func (c *Client) GetIdentityDirectoryByID(ctx context.Context, id string) (*IdentityDirectory, error) {
	if err := ValidateAPIClient(c.identityDirectoriesAPI, "Kong Identity directories API"); err != nil {
		return nil, err
	}
	response, err := c.identityDirectoriesAPI.GetDirectory(ctx, id)
	if err != nil {
		if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get Kong Identity directory: %w", err)
	}
	if response == nil || response.KongDirectory == nil {
		return nil, fmt.Errorf("missing Kong Identity directory response")
	}
	result := NormalizeIdentityDirectory(*response.KongDirectory)
	return &result, nil
}

func (c *Client) CreateIdentityDirectory(ctx context.Context,
	request kkComps.CreateDirectoryBody,
) (*IdentityDirectory, error) {
	if err := ValidateAPIClient(c.identityDirectoriesAPI, "Kong Identity directories API"); err != nil {
		return nil, err
	}
	if err := stampManagedByRequest(ctx, resources.ResourceTypeIdentityDirectory, &request); err != nil {
		return nil, err
	}
	response, err := c.identityDirectoriesAPI.CreateDirectory(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kong Identity directory: %w", err)
	}
	if response == nil || response.KongDirectory == nil || response.KongDirectory.ID == "" {
		return nil, fmt.Errorf("missing created Kong Identity directory response")
	}
	if err := validateDirectoryVaultSetting(request.VaultEnabled, response.KongDirectory); err != nil {
		return nil, fmt.Errorf("directory %s was created, but %w", response.KongDirectory.ID, err)
	}
	result := NormalizeIdentityDirectory(*response.KongDirectory)
	return &result, nil
}

func (c *Client) ReplaceIdentityDirectory(ctx context.Context, id string, request kkComps.ReplaceDirectoryBody) error {
	if err := ValidateAPIClient(c.identityDirectoriesAPI, "Kong Identity directories API"); err != nil {
		return err
	}
	if err := stampManagedByRequest(ctx, resources.ResourceTypeIdentityDirectory, &request); err != nil {
		return err
	}
	response, err := c.identityDirectoriesAPI.ReplaceDirectory(ctx, id, request)
	if err != nil {
		return fmt.Errorf("failed to replace Kong Identity directory: %w", err)
	}
	if response == nil || response.KongDirectory == nil {
		return fmt.Errorf("missing replaced Kong Identity directory response")
	}
	return validateDirectoryVaultSetting(request.VaultEnabled, response.KongDirectory)
}

func validateDirectoryVaultSetting(requested *bool, observed *kkComps.KongDirectory) error {
	if requested != nil && (observed.VaultEnabled == nil || *requested != *observed.VaultEnabled) {
		return fmt.Errorf("directory API did not retain vault_enabled=%t; verify Token Vault availability",
			*requested)
	}
	return nil
}

func (c *Client) DeleteIdentityDirectory(ctx context.Context, id string) error {
	if err := ValidateAPIClient(c.identityDirectoriesAPI, "Kong Identity directories API"); err != nil {
		return err
	}
	// Never request a cascade through principals or other unmanaged descendants.
	_, err := c.identityDirectoriesAPI.DeleteDirectory(ctx, id, nil)
	if err != nil && declerrors.ExtractStatusCodeFromError(err) != http.StatusNotFound {
		return fmt.Errorf("failed to delete Kong Identity directory: %w", err)
	}
	return nil
}
