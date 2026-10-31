package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"unicode/utf8"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/pagination"
)

type IdentityBasicAuthReader interface {
	ListPrincipalBasicAuths(context.Context, kkOps.ListPrincipalBasicAuthsRequest, ...kkOps.Option) (
		*kkOps.ListPrincipalBasicAuthsResponse, error,
	)
	GetPrincipalBasicAuth(context.Context, kkOps.GetPrincipalBasicAuthRequest, ...kkOps.Option) (
		*kkOps.GetPrincipalBasicAuthResponse, error,
	)
}

type IdentityPasswordsReader interface {
	ListPasswords(context.Context, kkOps.ListPasswordsRequest, ...kkOps.Option) (*kkOps.ListPasswordsResponse, error)
	GetPassword(context.Context, kkOps.GetPasswordRequest, ...kkOps.Option) (*kkOps.GetPasswordResponse, error)
}

// IdentityBasicAuth is the username record, separate from its secret-bearing
// password children. Public projections never serialize an SDK credential item.
type IdentityBasicAuth struct {
	ID        string            `json:"id"         yaml:"id"`
	Username  string            `json:"username"   yaml:"username"`
	ManagedBy map[string]string `json:"managed_by" yaml:"managed_by"`
	CreatedAt time.Time         `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time         `json:"updated_at" yaml:"updated_at"`
}

type IdentityPassword struct {
	ID        string            `json:"id"         yaml:"id"`
	ExpiresAt *time.Time        `json:"expires_at" yaml:"expires_at"`
	ManagedBy map[string]string `json:"managed_by" yaml:"managed_by"`
	CreatedAt time.Time         `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time         `json:"updated_at" yaml:"updated_at"`
}

func validIdentityBasicAuthUsername(username string) bool {
	return utf8.ValidString(username) && utf8.RuneCountInString(username) >= 1 &&
		utf8.RuneCountInString(username) <= 255
}

func validateIdentityCredentialPath(directoryID, principalID string, childIDs ...string) error {
	if !util.IsValidUUID(directoryID) || !util.IsValidUUID(principalID) {
		return fmt.Errorf("directory and principal IDs must be UUIDs")
	}
	for _, id := range childIDs {
		if !util.IsValidUUID(id) {
			return fmt.Errorf("basic-auth and password IDs must be UUIDs")
		}
	}
	return nil
}

// The SDK may return defaults for missing response fields, and decode errors
// can contain credential material. Parse only public fields without retaining
// response bodies or raw decode errors in the returned error chain.
func decodeIdentityCredentialResponse(response *http.Response, destination any) error {
	if response == nil || response.Body == nil {
		return fmt.Errorf("missing credential HTTP response")
	}
	data, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	response.Body = io.NopCloser(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("cannot read credential response")
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("cannot decode public credential metadata")
	}
	return nil
}

type identityCredentialPage[T any] struct {
	Data []T `json:"data"`
	Meta *struct {
		Page *struct {
			Next json.RawMessage `json:"next"`
			Size *float64        `json:"size"`
		} `json:"page"`
	} `json:"meta"`
}

func (p identityCredentialPage[T]) next() (*string, error) {
	if p.Data == nil || p.Meta == nil || p.Meta.Page == nil || p.Meta.Page.Size == nil ||
		*p.Meta.Page.Size < 1 || len(p.Meta.Page.Next) == 0 {
		return nil, fmt.Errorf("missing or invalid credential collection data or pagination")
	}
	var next *string
	if err := json.Unmarshal(p.Meta.Page.Next, &next); err != nil {
		return nil, fmt.Errorf("invalid credential collection cursor")
	}
	return next, nil
}

func ListIdentityBasicAuths(ctx context.Context, api IdentityBasicAuthReader,
	directoryID, principalID string, pageSize int64,
) ([]IdentityBasicAuth, error) {
	if api == nil {
		return nil, fmt.Errorf("identity basic-auth reader is not configured")
	}
	if err := validateIdentityCredentialPath(directoryID, principalID); err != nil {
		return nil, err
	}
	if pageSize < 1 {
		pageSize = 100
	}
	request := kkOps.ListPrincipalBasicAuthsRequest{
		DirectoryID: directoryID, PrincipalID: principalID,
		Page: &kkComps.CursorPageParameters{Size: &pageSize},
	}
	result := make([]IdentityBasicAuth, 0)
	seen := make(map[string]bool)
	var tracker pagination.CursorTracker
	for {
		response, err := api.ListPrincipalBasicAuths(ctx, request)
		if err != nil {
			return nil, identitySecretAPIError("list basic-auth credentials", err)
		}
		if response == nil {
			return nil, fmt.Errorf("missing basic-auth collection response")
		}
		var page identityCredentialPage[IdentityBasicAuth]
		if err := decodeIdentityCredentialResponse(response.RawResponse, &page); err != nil {
			return nil, err
		}
		next, err := page.next()
		if err != nil {
			return nil, err
		}
		for _, value := range page.Data {
			if !util.IsValidUUID(value.ID) || !validIdentityBasicAuthUsername(value.Username) || seen[value.ID] {
				return nil, fmt.Errorf("basic-auth collection contains an invalid or repeated resource")
			}
			seen[value.ID] = true
			result = append(result, value)
		}
		cursor, err := tracker.Next(next)
		if err != nil {
			return nil, fmt.Errorf("cannot paginate basic-auth credentials: %w", err)
		}
		if cursor == "" {
			return result, nil
		}
		request.Page.After = &cursor
	}
}

func GetIdentityBasicAuth(ctx context.Context, api IdentityBasicAuthReader,
	directoryID, principalID, id string,
) (*IdentityBasicAuth, error) {
	if api == nil {
		return nil, fmt.Errorf("identity basic-auth reader is not configured")
	}
	if err := validateIdentityCredentialPath(directoryID, principalID, id); err != nil {
		return nil, err
	}
	response, err := api.GetPrincipalBasicAuth(ctx, kkOps.GetPrincipalBasicAuthRequest{
		DirectoryID: directoryID, PrincipalID: principalID, BasicAuthID: id,
	})
	if err != nil {
		return nil, identitySecretAPIError("get basic-auth credential", err)
	}
	if response == nil {
		return nil, fmt.Errorf("missing basic-auth response")
	}
	var value IdentityBasicAuth
	if err := decodeIdentityCredentialResponse(response.RawResponse, &value); err != nil {
		return nil, err
	}
	if value.ID != id || !validIdentityBasicAuthUsername(value.Username) {
		return nil, fmt.Errorf("missing or inconsistent basic-auth response")
	}
	return &value, nil
}

// Username is an explicit selector: UUID-shaped usernames must not be mistaken
// for resource IDs. A complete scan rejects duplicate matches before detail.
func ResolveIdentityBasicAuthUsername(ctx context.Context, api IdentityBasicAuthReader,
	directoryID, principalID, username string, pageSize int64,
) (*IdentityBasicAuth, error) {
	if !validIdentityBasicAuthUsername(username) {
		return nil, fmt.Errorf("username must contain 1 to 255 characters")
	}
	values, err := ListIdentityBasicAuths(ctx, api, directoryID, principalID, pageSize)
	if err != nil {
		return nil, err
	}
	var match *IdentityBasicAuth
	for _, value := range values {
		if value.Username == username {
			if match != nil {
				return nil, fmt.Errorf("multiple basic-auth credentials match the username in the selected principal")
			}
			match = &value
		}
	}
	if match == nil {
		return nil, fmt.Errorf("basic-auth username not found in the selected principal")
	}
	value, err := GetIdentityBasicAuth(ctx, api, directoryID, principalID, match.ID)
	if err != nil {
		return nil, err
	}
	if value.Username != username {
		return nil, fmt.Errorf("basic-auth username changed during lookup")
	}
	return value, nil
}

func ListIdentityPasswords(ctx context.Context, api IdentityPasswordsReader,
	directoryID, principalID, basicAuthID string, pageSize int64,
) ([]IdentityPassword, error) {
	if api == nil {
		return nil, fmt.Errorf("identity password reader is not configured")
	}
	if err := validateIdentityCredentialPath(directoryID, principalID, basicAuthID); err != nil {
		return nil, err
	}
	if pageSize < 1 {
		pageSize = 100
	}
	request := kkOps.ListPasswordsRequest{
		DirectoryID: directoryID, PrincipalID: principalID, BasicAuthID: basicAuthID,
		Page: &kkComps.CursorPageParameters{Size: &pageSize},
	}
	result := make([]IdentityPassword, 0)
	seen := make(map[string]bool)
	var tracker pagination.CursorTracker
	for {
		response, err := api.ListPasswords(ctx, request)
		if err != nil {
			return nil, identitySecretAPIError("list basic-auth passwords", err)
		}
		if response == nil {
			return nil, fmt.Errorf("missing password collection response")
		}
		var page identityCredentialPage[IdentityPassword]
		if err := decodeIdentityCredentialResponse(response.RawResponse, &page); err != nil {
			return nil, err
		}
		next, err := page.next()
		if err != nil {
			return nil, err
		}
		for _, value := range page.Data {
			if !util.IsValidUUID(value.ID) || seen[value.ID] {
				return nil, fmt.Errorf("password collection contains an invalid or repeated resource")
			}
			seen[value.ID] = true
			result = append(result, value)
		}
		cursor, err := tracker.Next(next)
		if err != nil {
			return nil, fmt.Errorf("cannot paginate basic-auth passwords: %w", err)
		}
		if cursor == "" {
			return result, nil
		}
		request.Page.After = &cursor
	}
}

func GetIdentityPassword(ctx context.Context, api IdentityPasswordsReader,
	directoryID, principalID, basicAuthID, id string,
) (*IdentityPassword, error) {
	if api == nil {
		return nil, fmt.Errorf("identity password reader is not configured")
	}
	if err := validateIdentityCredentialPath(directoryID, principalID, basicAuthID, id); err != nil {
		return nil, err
	}
	response, err := api.GetPassword(ctx, kkOps.GetPasswordRequest{
		DirectoryID: directoryID, PrincipalID: principalID, BasicAuthID: basicAuthID, PasswordID: id,
	})
	if err != nil {
		return nil, identitySecretAPIError("get basic-auth password", err)
	}
	if response == nil {
		return nil, fmt.Errorf("missing password response")
	}
	var value IdentityPassword
	if err := decodeIdentityCredentialResponse(response.RawResponse, &value); err != nil {
		return nil, err
	}
	if value.ID != id {
		return nil, fmt.Errorf("missing or inconsistent password response")
	}
	return &value, nil
}
