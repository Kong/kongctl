package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/konnect/identitykeys"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/pagination"
)

type IdentityVaultKeysLister interface {
	ListVaultKeys(context.Context, kkOps.ListVaultKeysRequest, ...kkOps.Option) (*kkOps.ListVaultKeysResponse, error)
}

type IdentityVaultKeysReader interface {
	IdentityVaultKeysLister
	GetVaultKey(context.Context, string, string, ...kkOps.Option) (*kkOps.GetVaultKeyResponse, error)
}

type IdentityVaultKeysAPI interface {
	IdentityVaultKeysReader
	CreateVaultKey(
		context.Context,
		string,
		kkComps.CreateVaultKeyBody,
		...kkOps.Option,
	) (*kkOps.CreateVaultKeyResponse, error)
	UpdateVaultKey(context.Context, kkOps.UpdateVaultKeyRequest, ...kkOps.Option) (*kkOps.UpdateVaultKeyResponse, error)
	DeleteVaultKey(context.Context, string, string, ...kkOps.Option) (*kkOps.DeleteVaultKeyResponse, error)
}

// IdentityVaultKey contains encryption public material only. Its name is not a
// unique identifier; Kid is verified against the supplied public material.
type IdentityVaultKey struct {
	ID          string    `json:"id"           yaml:"id"`
	DirectoryID string    `json:"directory_id" yaml:"directory_id"`
	Name        string    `json:"name"         yaml:"name"`
	PublicKey   string    `json:"public_key"   yaml:"public_key"`
	Kid         string    `json:"kid"          yaml:"kid"`
	Algorithm   string    `json:"algorithm"    yaml:"algorithm"`
	KeySize     int       `json:"key_size"     yaml:"key_size"`
	CreatedAt   time.Time `json:"created_at"   yaml:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"   yaml:"updated_at"`
}

func (v *IdentityVaultKey) UnmarshalJSON(data []byte) error {
	type alias IdentityVaultKey
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("invalid Vault public key response")
	}
	key, err := identitykeys.ParsePublicKey(value.PublicKey)
	if err != nil || !util.IsValidUUID(value.ID) || !util.IsValidUUID(value.DirectoryID) ||
		identitykeys.ValidateName(value.Name) != nil || value.Kid != key.Kid || value.Algorithm != key.Algorithm ||
		value.KeySize != key.Bits || value.CreatedAt.IsZero() || value.UpdatedAt.IsZero() {
		return fmt.Errorf("missing or inconsistent Vault public key identity or configuration")
	}
	*v = IdentityVaultKey(value)
	return nil
}

func decodeIdentityVaultKeyResponse(response *http.Response, destination any) error {
	if response == nil || response.Body == nil {
		return fmt.Errorf("missing Vault public key HTTP response")
	}
	data, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	response.Body = io.NopCloser(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("cannot read Vault public key response")
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("cannot decode Vault public key response")
	}
	return nil
}

func validateIdentityVaultKeyScope(api IdentityVaultKeysLister, directory string) error {
	if api == nil {
		return fmt.Errorf("identity Vault public keys API is not configured")
	}
	if !util.IsValidUUID(directory) {
		return fmt.Errorf("directory ID must be a UUID")
	}
	return nil
}

func ListIdentityVaultKeys(ctx context.Context, api IdentityVaultKeysLister, directory string,
	pageSize int64,
) ([]IdentityVaultKey, error) {
	if err := validateIdentityVaultKeyScope(api, directory); err != nil {
		return nil, err
	}
	if pageSize < 1 {
		pageSize = 100
	}
	request := kkOps.ListVaultKeysRequest{
		DirectoryID: directory, Page: &kkComps.CursorPageParameters{Size: &pageSize},
	}
	result := make([]IdentityVaultKey, 0)
	ids, kids := make(map[string]bool), make(map[string]bool)
	var tracker pagination.CursorTracker
	for {
		response, err := api.ListVaultKeys(ctx, request)
		if err != nil {
			return nil, identitySecretAPIError("list Vault public keys", err)
		}
		if response == nil {
			return nil, fmt.Errorf("missing Vault public key collection response")
		}
		var page identityCredentialPage[IdentityVaultKey]
		if err := decodeIdentityVaultKeyResponse(response.RawResponse, &page); err != nil {
			return nil, err
		}
		if page.Meta == nil || page.Meta.Page == nil || page.Meta.Page.Size == nil ||
			math.Trunc(*page.Meta.Page.Size) != *page.Meta.Page.Size {
			return nil, fmt.Errorf("invalid Vault public key pagination")
		}
		next, err := page.next()
		if err != nil {
			return nil, fmt.Errorf("invalid Vault public key collection: %w", err)
		}
		for _, value := range page.Data {
			if value.DirectoryID != directory || ids[value.ID] || kids[value.Kid] {
				return nil, fmt.Errorf("vault public key collection contains an inconsistent or repeated identity")
			}
			ids[value.ID], kids[value.Kid] = true, true
			result = append(result, value)
		}
		cursor, err := tracker.Next(next)
		if err != nil {
			return nil, fmt.Errorf("cannot paginate Vault public keys: %w", err)
		}
		if cursor == "" {
			return result, nil
		}
		request.Page.After = &cursor
	}
}

func GetIdentityVaultKey(ctx context.Context, api IdentityVaultKeysReader, directory, id string,
) (*IdentityVaultKey, error) {
	if err := validateIdentityVaultKeyScope(api, directory); err != nil {
		return nil, err
	}
	if !util.IsValidUUID(id) {
		return nil, fmt.Errorf("vault public key ID must be a UUID")
	}
	response, err := api.GetVaultKey(ctx, directory, id)
	if err != nil {
		return nil, identitySecretAPIError("get Vault public key", err)
	}
	if response == nil {
		return nil, fmt.Errorf("missing Vault public key detail response")
	}
	var value IdentityVaultKey
	if err := decodeIdentityVaultKeyResponse(response.RawResponse, &value); err != nil {
		return nil, err
	}
	if value.ID != id || value.DirectoryID != directory {
		return nil, fmt.Errorf("vault public key response identity differs from the selected directory/key")
	}
	return &value, nil
}

func ResolveIdentityVaultKeyKid(ctx context.Context, api IdentityVaultKeysReader, directory, kid string,
	pageSize int64,
) (*IdentityVaultKey, error) {
	if kid == "" {
		return nil, fmt.Errorf("kid must not be empty")
	}
	values, err := ListIdentityVaultKeys(ctx, api, directory, pageSize)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		if value.Kid != kid {
			continue
		}
		current, err := GetIdentityVaultKey(ctx, api, directory, value.ID)
		if err != nil {
			return nil, err
		}
		if current.Kid != kid {
			return nil, fmt.Errorf("vault public key identity changed during lookup")
		}
		return current, nil
	}
	return nil, fmt.Errorf("vault public key kid %q not found in the selected directory", kid)
}

func CreateIdentityVaultKey(ctx context.Context, api IdentityVaultKeysAPI, directory string,
	request kkComps.CreateVaultKeyBody,
) (*IdentityVaultKey, error) {
	if err := validateIdentityVaultKeyScope(api, directory); err != nil {
		return nil, err
	}
	if err := identitykeys.ValidateName(request.Name); err != nil {
		return nil, err
	}
	key, err := identitykeys.ParsePublicKey(request.PublicKey)
	if err != nil {
		return nil, err
	}
	request.PublicKey = key.PEM
	response, err := api.CreateVaultKey(httpclient.WithoutRetries(ctx), directory, request,
		kkOps.WithRetries(retry.Config{Strategy: "none"}))
	if err != nil {
		return nil, identitySecretAPIError("create Vault public key", err)
	}
	if response == nil {
		return nil, fmt.Errorf("missing created Vault public key response")
	}
	var value IdentityVaultKey
	if err := decodeIdentityVaultKeyResponse(response.RawResponse, &value); err != nil {
		return nil, err
	}
	if value.DirectoryID != directory || value.Kid != key.Kid || value.Name != request.Name {
		return nil, fmt.Errorf("created Vault public key response differs from the requested configuration")
	}
	return &value, nil
}

func UpdateIdentityVaultKey(ctx context.Context, api IdentityVaultKeysAPI, directory, id, kid, name string) error {
	if err := validateIdentityVaultKeyScope(api, directory); err != nil {
		return err
	}
	if !util.IsValidUUID(id) || kid == "" {
		return fmt.Errorf("vault key update requires a UUID and fingerprint")
	}
	if err := identitykeys.ValidateName(name); err != nil {
		return err
	}
	response, err := api.UpdateVaultKey(ctx, kkOps.UpdateVaultKeyRequest{
		DirectoryID: directory, KeyID: id, PatchVaultKeyBody: kkComps.PatchVaultKeyBody{Name: &name},
	})
	if err != nil {
		return identitySecretAPIError("rename Vault public key", err)
	}
	if response == nil {
		return fmt.Errorf("missing updated Vault public key response")
	}
	var value IdentityVaultKey
	if err := decodeIdentityVaultKeyResponse(response.RawResponse, &value); err != nil {
		return err
	}
	if value.ID != id || value.DirectoryID != directory || value.Kid != kid || value.Name != name {
		return fmt.Errorf("updated Vault public key response differs from the requested configuration")
	}
	return nil
}

func DeleteIdentityVaultKey(ctx context.Context, api IdentityVaultKeysAPI, directory, id string) error {
	if err := validateIdentityVaultKeyScope(api, directory); err != nil {
		return err
	}
	if !util.IsValidUUID(id) {
		return fmt.Errorf("vault public key ID must be a UUID")
	}
	_, err := api.DeleteVaultKey(ctx, directory, id)
	if err != nil {
		return identitySecretAPIError("delete Vault public key", err)
	}
	return nil
}
