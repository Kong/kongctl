package helpers

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/pagination"
)

type IdentityPrincipalKeysReader interface {
	ListPrincipalKeys(context.Context, kkOps.ListPrincipalKeysRequest, ...kkOps.Option) (
		*kkOps.ListPrincipalKeysResponse, error,
	)
	GetIdentityKey(context.Context, kkOps.GetIdentityKeyRequest, ...kkOps.Option) (*kkOps.GetIdentityKeyResponse, error)
}

// PrincipalAPIKey contains public credential metadata only. Older deployments
// omit name; no local name or creation method is inferred from that absence.
type PrincipalAPIKey struct {
	ID        string            `json:"id"                   yaml:"id"`
	Name      *string           `json:"name,omitempty"       yaml:"name,omitempty"`
	ExpiresAt *time.Time        `json:"expires_at"           yaml:"expires_at"`
	ManagedBy map[string]string `json:"managed_by"           yaml:"managed_by"`
	CreatedAt time.Time         `json:"created_at"           yaml:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"           yaml:"updated_at"`
}

type IdentityAPIKey = PrincipalAPIKey

func principalAPIKey(value kkComps.KongPrincipalKey) (PrincipalAPIKey, error) {
	if !util.IsValidUUID(value.ID) {
		return PrincipalAPIKey{}, fmt.Errorf("principal API-key response has no valid ID")
	}
	return PrincipalAPIKey{
		ID: value.ID, Name: value.Name, ExpiresAt: value.ExpiresAt, ManagedBy: maps.Clone(value.ManagedBy),
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}, nil
}

func validatePrincipalKeyReader(api IdentityPrincipalKeysReader, directoryID, principalID string) error {
	if api == nil {
		return fmt.Errorf("principal API-key reader is not configured")
	}
	if !util.IsValidUUID(directoryID) || !util.IsValidUUID(principalID) {
		return fmt.Errorf("directory and principal IDs must be UUIDs")
	}
	return nil
}

func ListPrincipalAPIKeys(ctx context.Context, api IdentityPrincipalKeysReader,
	directoryID, principalID string, pageSize int64,
) ([]PrincipalAPIKey, error) {
	if err := validatePrincipalKeyReader(api, directoryID, principalID); err != nil {
		return nil, err
	}
	return listIdentityAPIKeys(pageSize, "principal", func(page *kkComps.CursorPageParameters) (
		*http.Response, error,
	) {
		response, err := api.ListPrincipalKeys(ctx, kkOps.ListPrincipalKeysRequest{
			DirectoryID: directoryID, PrincipalID: principalID, Page: page,
		})
		if err != nil {
			return nil, identitySecretAPIError("list principal API keys", err)
		}
		if response == nil {
			return nil, fmt.Errorf("missing principal API-key collection response")
		}
		return response.RawResponse, nil
	})
}

func listIdentityAPIKeys(pageSize int64, kind string,
	read func(*kkComps.CursorPageParameters) (*http.Response, error),
) ([]IdentityAPIKey, error) {
	if pageSize < 1 {
		pageSize = 100
	}
	page := &kkComps.CursorPageParameters{Size: &pageSize}
	result := make([]PrincipalAPIKey, 0)
	seen := make(map[string]bool)
	var tracker pagination.CursorTracker
	for {
		response, err := read(page)
		if err != nil {
			return nil, err
		}
		var observed identityCredentialPage[IdentityAPIKey]
		if err := decodeIdentityCredentialResponse(response, &observed); err != nil {
			return nil, err
		}
		next, err := observed.next()
		if err != nil {
			return nil, err
		}
		for _, value := range observed.Data {
			if !util.IsValidUUID(value.ID) {
				return nil, fmt.Errorf("%s API-key collection contains an invalid ID", kind)
			}
			if seen[value.ID] {
				return nil, fmt.Errorf("%s API-key collection repeats an ID", kind)
			}
			seen[value.ID] = true
			result = append(result, value)
		}
		cursor, err := tracker.Next(next)
		if err != nil {
			return nil, fmt.Errorf("cannot paginate %s API keys: %w", kind, err)
		}
		if cursor == "" {
			return result, nil
		}
		page.After = &cursor
	}
}

func GetPrincipalAPIKey(ctx context.Context, api IdentityPrincipalKeysReader,
	directoryID, principalID, id string,
) (*PrincipalAPIKey, error) {
	if err := validatePrincipalKeyReader(api, directoryID, principalID); err != nil {
		return nil, err
	}
	if !util.IsValidUUID(id) {
		return nil, fmt.Errorf("principal API-key ID must be a UUID")
	}
	response, err := api.GetIdentityKey(ctx, kkOps.GetIdentityKeyRequest{
		DirectoryID: directoryID, PrincipalID: principalID, APIKeyID: id,
	})
	if err != nil {
		return nil, identitySecretAPIError("get principal API key", err)
	}
	if response == nil || response.KongPrincipalKey == nil || response.KongPrincipalKey.ID != id {
		return nil, fmt.Errorf("missing or inconsistent principal API-key response")
	}
	value, err := principalAPIKey(*response.KongPrincipalKey)
	if err != nil {
		return nil, err
	}
	return &value, nil
}
