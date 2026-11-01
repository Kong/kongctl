package helpers

import (
	"context"
	"fmt"
	"net/http"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/util"
)

type IdentityGroupKeysLister interface {
	ListGroupKeys(context.Context, kkOps.ListGroupKeysRequest, ...kkOps.Option) (*kkOps.ListGroupKeysResponse, error)
}
type IdentityGroupKeysReader interface {
	IdentityGroupKeysLister
	GetGroupKey(context.Context, kkOps.GetGroupKeyRequest, ...kkOps.Option) (*kkOps.GetGroupKeyResponse, error)
}

func ListIdentityGroupAPIKeys(ctx context.Context, api IdentityGroupKeysLister,
	directoryID, groupID string, pageSize int64,
) ([]IdentityAPIKey, error) {
	if api == nil {
		return nil, fmt.Errorf("group API-key reader is not configured")
	}
	if !util.IsValidUUID(directoryID) || !util.IsValidUUID(groupID) {
		return nil, fmt.Errorf("directory and group IDs must be UUIDs")
	}
	return listIdentityAPIKeys(pageSize, "group", func(page *kkComps.CursorPageParameters) (
		*http.Response, error,
	) {
		response, err := api.ListGroupKeys(ctx, kkOps.ListGroupKeysRequest{
			DirectoryID: directoryID, GroupID: groupID, Page: page,
		})
		if err != nil {
			return nil, identitySecretAPIError("list group API keys", err)
		}
		if response == nil {
			return nil, fmt.Errorf("missing group API-key collection response")
		}
		return response.RawResponse, nil
	})
}

func GetIdentityGroupAPIKey(ctx context.Context, api IdentityGroupKeysReader,
	directoryID, groupID, id string,
) (*IdentityAPIKey, error) {
	if api == nil {
		return nil, fmt.Errorf("group API-key reader is not configured")
	}
	if !util.IsValidUUID(directoryID) || !util.IsValidUUID(groupID) || !util.IsValidUUID(id) {
		return nil, fmt.Errorf("directory, group and API-key IDs must be UUIDs")
	}
	response, err := api.GetGroupKey(ctx, kkOps.GetGroupKeyRequest{
		DirectoryID: directoryID, GroupID: groupID, APIKeyID: id,
	})
	if err != nil {
		return nil, identitySecretAPIError("get group API key", err)
	}
	if response == nil || response.KongPrincipalKey == nil || response.KongPrincipalKey.ID != id {
		return nil, fmt.Errorf("missing or inconsistent group API-key response")
	}
	value, err := principalAPIKey(*response.KongPrincipalKey)
	if err != nil {
		return nil, err
	}
	return &value, nil
}
