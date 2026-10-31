package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	kkErrors "github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/pagination"
)

type IdentityPrincipalIdentitiesAPI interface {
	ListIdentities(context.Context, kkOps.ListIdentitiesRequest, ...kkOps.Option) (*kkOps.ListIdentitiesResponse, error)
	GetIdentity(context.Context, kkOps.GetIdentityRequest, ...kkOps.Option) (*kkOps.GetIdentityResponse, error)
	CreateIdentity(context.Context, kkOps.CreateIdentityRequest, ...kkOps.Option) (*kkOps.CreateIdentityResponse, error)
	ReplaceIdentity(
		context.Context,
		kkOps.ReplaceIdentityRequest,
		...kkOps.Option,
	) (*kkOps.ReplaceIdentityResponse, error)
	UpdateIdentity(context.Context, kkOps.UpdateIdentityRequest, ...kkOps.Option) (*kkOps.UpdateIdentityResponse, error)
	DeleteIdentity(context.Context, kkOps.DeleteIdentityRequest, ...kkOps.Option) (*kkOps.DeleteIdentityResponse, error)
}

type PrincipalIdentityClaim struct {
	Name  string `json:"name"  yaml:"name"`
	Value string `json:"value" yaml:"value"`
}

// PrincipalIdentity flattens the SDK's four response variants without inventing
// fields that the API does not return. ControlPlaneType is observed, read-only.
type PrincipalIdentity struct {
	ID               string                  `json:"id"                           yaml:"id"`
	Type             string                  `json:"type"                         yaml:"type"`
	Issuer           *string                 `json:"issuer,omitempty"             yaml:"issuer,omitempty"`
	Claim            *PrincipalIdentityClaim `json:"claim,omitempty"              yaml:"claim,omitempty"`
	AuthServerID     *string                 `json:"auth_server_id,omitempty"     yaml:"auth_server_id,omitempty"`
	ClientID         *string                 `json:"client_id,omitempty"          yaml:"client_id,omitempty"`
	ControlPlaneID   *string                 `json:"control_plane_id,omitempty"   yaml:"control_plane_id,omitempty"`
	ConsumerID       *string                 `json:"consumer_id,omitempty"        yaml:"consumer_id,omitempty"`
	WorkspaceID      *string                 `json:"workspace_id,omitempty"       yaml:"workspace_id,omitempty"`
	ControlPlaneType *string                 `json:"control_plane_type,omitempty" yaml:"control_plane_type,omitempty"`
	Key              *string                 `json:"key,omitempty"                yaml:"key,omitempty"`
	Value            *string                 `json:"value,omitempty"              yaml:"value,omitempty"`
	Labels           map[string]string       `json:"labels"                       yaml:"labels"`
	ManagedBy        map[string]string       `json:"managed_by"                   yaml:"managed_by"`
	CreatedAt        time.Time               `json:"created_at"                   yaml:"created_at"`
	UpdatedAt        time.Time               `json:"updated_at"                   yaml:"updated_at"`
}

func principalIdentityFromSDK(value kkComps.KongPrincipalIdentity) (PrincipalIdentity, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return PrincipalIdentity{}, fmt.Errorf("cannot encode principal identity response")
	}
	var result PrincipalIdentity
	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("cannot decode principal identity response")
	}
	if !util.IsValidUUID(result.ID) || result.Type == "" {
		return result, fmt.Errorf("principal identity response has no valid identity")
	}
	return result, nil
}

func ListPrincipalIdentities(ctx context.Context, api IdentityPrincipalIdentitiesAPI, directoryID, principalID string,
	pageSize int64,
) ([]PrincipalIdentity, error) {
	if api == nil {
		return nil, fmt.Errorf("principal identities API is not configured")
	}
	if !util.IsValidUUID(directoryID) || !util.IsValidUUID(principalID) {
		return nil, fmt.Errorf("directory and principal IDs must be UUIDs")
	}
	if pageSize < 1 {
		pageSize = 100
	}
	request := kkOps.ListIdentitiesRequest{
		DirectoryID: directoryID, PrincipalID: principalID,
		Page: &kkComps.CursorPageParameters{Size: &pageSize},
	}
	result := make([]PrincipalIdentity, 0)
	seen := make(map[string]bool)
	var tracker pagination.CursorTracker
	for {
		response, err := api.ListIdentities(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to list principal identities: %w", err)
		}
		if response == nil || response.ListKongPrincipalIdentities == nil ||
			response.ListKongPrincipalIdentities.Data == nil {
			return nil, fmt.Errorf("missing principal identity collection data")
		}
		for _, value := range response.ListKongPrincipalIdentities.Data {
			identity, err := principalIdentityFromSDK(value)
			if err != nil {
				return nil, err
			}
			if seen[identity.ID] {
				return nil, fmt.Errorf("principal identity collection repeats an ID")
			}
			seen[identity.ID] = true
			result = append(result, identity)
		}
		cursor, err := tracker.Next(response.ListKongPrincipalIdentities.Meta.Page.Next)
		if err != nil {
			return nil, fmt.Errorf("failed to paginate principal identities: %w", err)
		}
		if cursor == "" {
			return result, nil
		}
		request.Page.After = &cursor
	}
}

// GetPrincipalIdentity verifies collection membership because the deployed GET
// route ignores an existing principal's identity association (API question Q21).
// Writers must use the same fresh proof rather than trusting GET alone.
func GetPrincipalIdentity(ctx context.Context, api IdentityPrincipalIdentitiesAPI, directoryID, principalID, id string,
	pageSize int64,
) (*PrincipalIdentity, error) {
	if !util.IsValidUUID(id) {
		return nil, fmt.Errorf("principal identity ID must be a UUID")
	}
	values, err := ListPrincipalIdentities(ctx, api, directoryID, principalID, pageSize)
	if err != nil {
		return nil, err
	}
	found := false
	for _, value := range values {
		if value.ID == id {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf(
			"principal identity does not belong to the selected principal: %w",
			&kkErrors.NotFoundError{
				Status: 404, Title: "Principal identity not found",
			},
		)
	}
	response, err := api.GetIdentity(
		ctx,
		kkOps.GetIdentityRequest{DirectoryID: directoryID, PrincipalID: principalID, IdentityID: id},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get principal identity: %w", err)
	}
	if response == nil || response.KongPrincipalIdentity == nil {
		return nil, fmt.Errorf("missing principal identity response")
	}
	value, err := principalIdentityFromSDK(*response.KongPrincipalIdentity)
	if err != nil {
		return nil, err
	}
	if value.ID != id {
		return nil, fmt.Errorf("principal identity response does not match the selected ID")
	}
	return &value, nil
}
