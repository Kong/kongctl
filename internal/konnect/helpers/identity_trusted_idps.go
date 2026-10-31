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
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/pagination"
)

type IdentityTrustedIDPsReader interface {
	ListTrustedIdps(context.Context, string, *kkComps.CursorPageParameters, ...kkOps.Option) (
		*kkOps.ListTrustedIdpsResponse, error,
	)
	GetTrustedIdp(context.Context, string, string, ...kkOps.Option) (*kkOps.GetTrustedIdpResponse, error)
	ListTrustedIdpPrincipals(context.Context, kkOps.ListTrustedIdpPrincipalsRequest, ...kkOps.Option) (
		*kkOps.ListTrustedIdpPrincipalsResponse, error,
	)
}

type IdentityTrustedIDPsAPI interface {
	IdentityTrustedIDPsReader
	CreateTrustedIdp(context.Context, string, kkComps.CreateTrustedIdpPayload, ...kkOps.Option) (
		*kkOps.CreateTrustedIdpResponse, error,
	)
	UpdateTrustedIdp(context.Context, kkOps.UpdateTrustedIdpRequest, ...kkOps.Option) (
		*kkOps.UpdateTrustedIdpResponse, error,
	)
	DeleteTrustedIdp(context.Context, string, string, ...kkOps.Option) (*kkOps.DeleteTrustedIdpResponse, error)
}

// IdentityTrustedIDP contains public trust configuration, without SDK defaults
// that could synthesize JIT or Vault access on an incomplete response.
type IdentityTrustedIDP struct {
	ID                     string                 `json:"id"                       yaml:"id"`
	IssuerURL              string                 `json:"issuer_url"               yaml:"issuer_url"`
	DisplayName            *string                `json:"display_name"             yaml:"display_name"`
	JwksURI                *string                `json:"jwks_uri"                 yaml:"jwks_uri"`
	JitProvisioningEnabled *bool                  `json:"jit_provisioning_enabled" yaml:"jit_provisioning_enabled"`
	VaultAccessEnabled     *bool                  `json:"vault_access_enabled"     yaml:"vault_access_enabled"`
	Jit                    *kkComps.TrustedIdpJit `json:"jit,omitempty"            yaml:"jit,omitempty"`
	CreatedAt              time.Time              `json:"created_at"               yaml:"created_at"`
	UpdatedAt              time.Time              `json:"updated_at"               yaml:"updated_at"`
}

func (v *IdentityTrustedIDP) UnmarshalJSON(data []byte) error {
	type alias IdentityTrustedIDP
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("invalid trusted IdP public configuration")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("invalid trusted IdP response")
	}
	if !util.IsValidUUID(value.ID) || value.IssuerURL == "" || len(fields["jwks_uri"]) == 0 ||
		value.JitProvisioningEnabled == nil || value.VaultAccessEnabled == nil {
		return fmt.Errorf("missing or invalid trusted IdP identity or required configuration")
	}
	*v = IdentityTrustedIDP(value)
	return nil
}

func decodeIdentityTrustedIDPResponse(response *http.Response, destination any) error {
	if response == nil || response.Body == nil {
		return fmt.Errorf("missing trusted IdP HTTP response")
	}
	data, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	response.Body = io.NopCloser(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("cannot read trusted IdP response")
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("cannot decode trusted IdP public response")
	}
	return nil
}

type identityTrustedIDPPage[T any] struct {
	Data []T `json:"data"`
	Meta *struct {
		Page *struct {
			Next json.RawMessage `json:"next"`
			Size *float64        `json:"size"`
		} `json:"page"`
	} `json:"meta"`
}

func listIdentityTrustedIDPCollection[T any](pageSize int64,
	fetch func(*kkComps.CursorPageParameters) (*http.Response, error), identify func(T) string,
) ([]T, error) {
	if pageSize < 1 {
		pageSize = 100
	}
	request := &kkComps.CursorPageParameters{Size: &pageSize}
	result := make([]T, 0)
	seen := make(map[string]bool)
	var tracker pagination.CursorTracker
	for {
		response, err := fetch(request)
		if err != nil {
			return nil, err
		}
		var page identityTrustedIDPPage[T]
		if err := decodeIdentityTrustedIDPResponse(response, &page); err != nil {
			return nil, err
		}
		if page.Data == nil || page.Meta == nil || page.Meta.Page == nil || page.Meta.Page.Size == nil ||
			*page.Meta.Page.Size < 1 || math.Trunc(*page.Meta.Page.Size) != *page.Meta.Page.Size ||
			len(page.Meta.Page.Next) == 0 {
			return nil, fmt.Errorf("missing or invalid trusted IdP collection data or pagination")
		}
		var next *string
		if err := json.Unmarshal(page.Meta.Page.Next, &next); err != nil {
			return nil, fmt.Errorf("invalid trusted IdP collection cursor")
		}
		for _, value := range page.Data {
			id := identify(value)
			if !util.IsValidUUID(id) || seen[id] {
				return nil, fmt.Errorf("trusted IdP collection contains an invalid or repeated resource")
			}
			seen[id] = true
			result = append(result, value)
		}
		cursor, err := tracker.Next(next)
		if err != nil {
			return nil, fmt.Errorf("cannot paginate trusted IdP collection: %w", err)
		}
		if cursor == "" {
			return result, nil
		}
		request.After = &cursor
	}
}

func ListIdentityTrustedIDPs(ctx context.Context, api IdentityTrustedIDPsReader, directoryID string,
	pageSize int64,
) ([]IdentityTrustedIDP, error) {
	if api == nil {
		return nil, fmt.Errorf("identity trusted IdPs API is not configured")
	}
	if !util.IsValidUUID(directoryID) {
		return nil, fmt.Errorf("directory ID must be a UUID")
	}
	return listIdentityTrustedIDPCollection(pageSize, func(page *kkComps.CursorPageParameters) (*http.Response, error) {
		response, err := api.ListTrustedIdps(ctx, directoryID, page)
		if err != nil {
			return nil, identitySecretAPIError("list trusted IdPs", err)
		}
		if response == nil {
			return nil, fmt.Errorf("missing trusted IdP collection response")
		}
		return response.RawResponse, nil
	}, func(value IdentityTrustedIDP) string { return value.ID })
}

func GetIdentityTrustedIDP(ctx context.Context, api IdentityTrustedIDPsReader, directoryID, id string,
) (*IdentityTrustedIDP, error) {
	if api == nil {
		return nil, fmt.Errorf("identity trusted IdPs API is not configured")
	}
	if !util.IsValidUUID(directoryID) || !util.IsValidUUID(id) {
		return nil, fmt.Errorf("directory and trusted IdP IDs must be UUIDs")
	}
	response, err := api.GetTrustedIdp(ctx, directoryID, id)
	if err != nil {
		return nil, identitySecretAPIError("get trusted IdP", err)
	}
	if response == nil {
		return nil, fmt.Errorf("missing trusted IdP detail response")
	}
	var value IdentityTrustedIDP
	if err := decodeIdentityTrustedIDPResponse(response.RawResponse, &value); err != nil {
		return nil, err
	}
	if value.ID != id {
		return nil, fmt.Errorf("trusted IdP response identity differs from the selected ID")
	}
	return &value, nil
}

func ResolveIdentityTrustedIDPIssuer(ctx context.Context, api IdentityTrustedIDPsReader, directoryID, issuer string,
	pageSize int64,
) (*IdentityTrustedIDP, error) {
	if issuer == "" {
		return nil, fmt.Errorf("issuer URL must not be empty")
	}
	values, err := ListIdentityTrustedIDPs(ctx, api, directoryID, pageSize)
	if err != nil {
		return nil, err
	}
	var match *IdentityTrustedIDP
	for i := range values {
		if values[i].IssuerURL == issuer {
			if match != nil {
				return nil, fmt.Errorf("multiple trusted IdPs have issuer URL %q in the selected directory", issuer)
			}
			match = &values[i]
		}
	}
	if match == nil {
		return nil, fmt.Errorf("trusted IdP issuer URL %q not found in the selected directory", issuer)
	}
	current, err := GetIdentityTrustedIDP(ctx, api, directoryID, match.ID)
	if err != nil {
		return nil, err
	}
	if current.IssuerURL != issuer {
		return nil, fmt.Errorf("trusted IdP issuer changed during lookup")
	}
	return current, nil
}

func ListIdentityTrustedIDPPrincipals(ctx context.Context, api IdentityTrustedIDPsReader, directoryID, id string,
	pageSize int64,
) ([]IdentityPrincipal, error) {
	if api == nil {
		return nil, fmt.Errorf("identity trusted IdPs API is not configured")
	}
	if !util.IsValidUUID(directoryID) || !util.IsValidUUID(id) {
		return nil, fmt.Errorf("directory and trusted IdP IDs must be UUIDs")
	}
	return listIdentityTrustedIDPCollection(pageSize, func(page *kkComps.CursorPageParameters) (*http.Response, error) {
		response, err := api.ListTrustedIdpPrincipals(ctx, kkOps.ListTrustedIdpPrincipalsRequest{
			DirectoryID: directoryID, TrustedIdpID: id, Page: page,
		})
		if err != nil {
			return nil, identitySecretAPIError("list trusted IdP principals", err)
		}
		if response == nil {
			return nil, fmt.Errorf("missing trusted IdP principal collection response")
		}
		return response.RawResponse, nil
	}, func(value IdentityPrincipal) string { return value.ID })
}
