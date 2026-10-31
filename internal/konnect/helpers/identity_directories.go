package helpers

import (
	"context"
	"fmt"
	"maps"
	"slices"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/pagination"
)

// IdentityDirectoriesAPI is the management API used by directory declarations.
type IdentityDirectoriesAPI interface {
	CreateDirectory(context.Context, kkComps.CreateDirectoryBody, ...kkOps.Option) (*kkOps.CreateDirectoryResponse, error)
	ListKongDirectories(context.Context, *kkComps.CursorPageParameters, *string,
		...kkOps.Option) (*kkOps.ListKongDirectoriesResponse, error)
	GetDirectory(context.Context, string, ...kkOps.Option) (*kkOps.GetDirectoryResponse, error)
	ReplaceDirectory(context.Context, string, kkComps.ReplaceDirectoryBody,
		...kkOps.Option) (*kkOps.ReplaceDirectoryResponse, error)
	DeleteDirectory(context.Context, string, *kkOps.DeleteDirectoryForce,
		...kkOps.Option) (*kkOps.DeleteDirectoryResponse, error)
}

// ResolveIdentityDirectory always reads a fresh resource, including after a name lookup.
func ResolveIdentityDirectory(ctx context.Context, api IdentityDirectoriesAPI, identifier string,
	pageSize int64,
) (*kkComps.KongDirectory, error) {
	if api == nil {
		return nil, fmt.Errorf("identity directory API is not configured")
	}
	id := identifier
	if !util.IsValidUUID(identifier) {
		values, err := ListIdentityDirectories(ctx, api, pageSize)
		if err != nil {
			return nil, err
		}
		found, err := FindIdentityDirectory(values, identifier)
		if err != nil {
			return nil, err
		}
		if found == nil {
			return nil, fmt.Errorf("identity directory %q not found", identifier)
		}
		id = found.ID
	}
	response, err := api.GetDirectory(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get Kong Identity directory: %w", err)
	}
	if response == nil || response.KongDirectory == nil || response.KongDirectory.ID == "" {
		return nil, fmt.Errorf("missing Kong Identity directory response")
	}
	return response.KongDirectory, nil
}

// IdentityDirectoryReplacement retains every mutable setting for the PUT API.
// PATCH does not support description or managed_by.
func IdentityDirectoryReplacement(current kkComps.KongDirectory) (kkComps.ReplaceDirectoryBody, error) {
	if current.AllowAllControlPlanes == nil || current.TTLSecs == nil || current.NegativeTTLSecs == nil ||
		current.VaultEnabled == nil {
		return kkComps.ReplaceDirectoryBody{},
			fmt.Errorf("cannot safely replace directory: current settings are not fully observable")
	}
	return kkComps.ReplaceDirectoryBody{
		Name: current.Name, Description: &current.Description,
		AllowedControlPlanes:  slices.Clone(current.AllowedControlPlanes),
		AllowAllControlPlanes: current.AllowAllControlPlanes,
		TTLSecs:               current.TTLSecs, NegativeTTLSecs: current.NegativeTTLSecs,
		VaultEnabled: current.VaultEnabled, Labels: maps.Clone(current.Labels), ManagedBy: maps.Clone(current.ManagedBy),
	}, nil
}

// ListIdentityDirectories observes the complete collection before callers filter it.
func ListIdentityDirectories(ctx context.Context, api IdentityDirectoriesAPI,
	pageSize int64,
) ([]kkComps.KongDirectory, error) {
	if api == nil {
		return nil, fmt.Errorf("identity directory API is not configured")
	}
	if pageSize < 1 {
		pageSize = 100
	}
	result := make([]kkComps.KongDirectory, 0)
	var tracker pagination.CursorTracker
	page := &kkComps.CursorPageParameters{Size: &pageSize}
	for {
		response, err := api.ListKongDirectories(ctx, page, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to list Kong Identity directories: %w", err)
		}
		if response == nil || response.ListKongDirectories == nil {
			return nil, fmt.Errorf("missing Kong Identity directory collection response")
		}
		result = append(result, response.ListKongDirectories.Data...)
		cursor, err := tracker.Next(response.ListKongDirectories.Meta.Page.Next)
		if err != nil {
			return nil, fmt.Errorf("failed to paginate Kong Identity directories: %w", err)
		}
		if cursor == "" {
			return result, nil
		}
		page.After = &cursor
	}
}

// FindIdentityDirectory matches the documented unique API name, without case folding.
func FindIdentityDirectory(directories []kkComps.KongDirectory, name string) (*kkComps.KongDirectory, error) {
	var found *kkComps.KongDirectory
	for i := range directories {
		if directories[i].Name != name {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("multiple Kong Identity directories have name %q", name)
		}
		found = &directories[i]
	}
	return found, nil
}
