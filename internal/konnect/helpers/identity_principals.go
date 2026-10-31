package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/util/pagination"
)

type IdentityPrincipalsAPI interface {
	CreatePrincipal(
		context.Context,
		string,
		kkComps.CreatePrincipalBody,
		...kkOps.Option,
	) (*kkOps.CreatePrincipalResponse, error)
	ReplacePrincipal(
		context.Context,
		kkOps.ReplacePrincipalRequest,
		...kkOps.Option,
	) (*kkOps.ReplacePrincipalResponse, error)
	DeletePrincipal(context.Context, string, string, ...kkOps.Option) (*kkOps.DeletePrincipalResponse, error)
	ListPrincipals(context.Context, kkOps.ListPrincipalsRequest, ...kkOps.Option) (*kkOps.ListPrincipalsResponse, error)
	GetPrincipal(context.Context, string, string, ...kkOps.Option) (*kkOps.GetPrincipalResponse, error)
}

// IdentityPrincipal preserves presence of active, which older deployments omit.
// The generated SDK's plain bool would incorrectly turn that omission into false.
type IdentityPrincipal struct {
	ID            string                                  `json:"id"                       yaml:"id"`
	DisplayName   *string                                 `json:"display_name,omitempty"   yaml:"display_name,omitempty"`
	Description   string                                  `json:"description"              yaml:"description"`
	Type          *kkComps.KongPrincipalType              `json:"type"                     yaml:"type"`
	Metadata      map[string]kkComps.KongIdentityMetadata `json:"metadata"                 yaml:"metadata"`
	Labels        map[string]string                       `json:"labels"                   yaml:"labels"`
	ManagedBy     map[string]string                       `json:"managed_by"               yaml:"managed_by"`
	ProvisionedBy *kkComps.ProvisionedBy                  `json:"provisioned_by,omitempty" yaml:"provisioned_by,omitempty"`
	Active        *bool                                   `json:"active,omitempty"         yaml:"active,omitempty"`
	Groups        []string                                `json:"groups,omitempty"         yaml:"groups,omitempty"`
	CreatedAt     time.Time                               `json:"created_at"               yaml:"created_at"`
	UpdatedAt     time.Time                               `json:"updated_at"               yaml:"updated_at"`
}

func decodeIdentityPrincipalResponse(response *http.Response, destination any) (bool, error) {
	if response == nil || response.Body == nil {
		return false, nil
	}
	data, err := io.ReadAll(response.Body)
	response.Body.Close()
	response.Body = io.NopCloser(bytes.NewReader(data))
	if err != nil {
		return true, fmt.Errorf("cannot read principal response")
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return true, fmt.Errorf("cannot decode principal response")
	}
	return true, nil
}

func principalFromSDK(value kkComps.KongPrincipal) IdentityPrincipal {
	return IdentityPrincipal{
		ID: value.ID, DisplayName: value.DisplayName, Description: value.Description,
		Type: value.Type, Metadata: value.Metadata, Labels: value.Labels, ManagedBy: value.ManagedBy,
		ProvisionedBy: value.ProvisionedBy, Groups: value.Groups, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func ListIdentityPrincipals(ctx context.Context, api IdentityPrincipalsAPI, directoryID string,
	pageSize int64,
) ([]IdentityPrincipal, error) {
	if api == nil {
		return nil, fmt.Errorf("identity principals API is not configured")
	}
	if pageSize < 1 {
		pageSize = 100
	}
	request := kkOps.ListPrincipalsRequest{
		DirectoryID: directoryID,
		Page:        &kkComps.CursorPageParameters{Size: &pageSize},
	}
	result := make([]IdentityPrincipal, 0)
	var tracker pagination.CursorTracker
	for {
		response, err := api.ListPrincipals(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to list principals: %w", err)
		}
		if response == nil || response.ListKongPrincipals == nil {
			return nil, fmt.Errorf("missing principal collection response")
		}
		var raw struct {
			Data []IdentityPrincipal `json:"data"`
		}
		decoded, err := decodeIdentityPrincipalResponse(response.RawResponse, &raw)
		if err != nil {
			return nil, err
		}
		if decoded && raw.Data == nil {
			return nil, fmt.Errorf("missing principal collection data")
		}
		if !decoded {
			for _, value := range response.ListKongPrincipals.Data {
				raw.Data = append(raw.Data, principalFromSDK(value))
			}
		}
		for _, value := range raw.Data {
			if value.ID == "" {
				return nil, fmt.Errorf("principal collection contains a resource without an ID")
			}
		}
		result = append(result, raw.Data...)
		cursor, err := tracker.Next(response.ListKongPrincipals.Meta.Page.Next)
		if err != nil {
			return nil, fmt.Errorf("failed to paginate principals: %w", err)
		}
		if cursor == "" {
			return result, nil
		}
		request.Page.After = &cursor
	}
}

func GetIdentityPrincipal(ctx context.Context, api IdentityPrincipalsAPI, directoryID, principalID string,
) (*IdentityPrincipal, error) {
	if api == nil {
		return nil, fmt.Errorf("identity principals API is not configured")
	}
	response, err := api.GetPrincipal(ctx, directoryID, principalID)
	if err != nil {
		return nil, fmt.Errorf("failed to get principal: %w", err)
	}
	if response == nil || response.KongPrincipal == nil {
		return nil, fmt.Errorf("missing principal response")
	}
	var value IdentityPrincipal
	decoded, err := decodeIdentityPrincipalResponse(response.RawResponse, &value)
	if err != nil {
		return nil, err
	}
	if !decoded {
		value = principalFromSDK(*response.KongPrincipal)
	}
	if value.ID == "" {
		return nil, fmt.Errorf("principal response has no ID")
	}
	return &value, nil
}

// IdentityPrincipalReplacement hydrates full PUT fields from a fresh response.
// Q10: Active is deliberately omitted; absence cannot be inferred as false.
func IdentityPrincipalReplacement(value IdentityPrincipal) kkComps.ReplacePrincipalBody {
	return kkComps.ReplacePrincipalBody{
		DisplayName: value.DisplayName, Description: &value.Description,
		Type: value.Type, Metadata: value.Metadata, Labels: value.Labels, ManagedBy: value.ManagedBy,
	}
}
