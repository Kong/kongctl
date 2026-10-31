package helpers

import (
	"context"
	"fmt"
	"math"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/util"
)

type IdentityAuthServersAPI interface {
	ListAuthServers(context.Context, kkOps.ListAuthServersRequest, ...kkOps.Option) (*kkOps.ListAuthServersResponse, error)
	CreateAuthServer(context.Context, kkComps.CreateAuthServer, ...kkOps.Option) (*kkOps.CreateAuthServerResponse, error)
	GetAuthServer(context.Context, string, ...kkOps.Option) (*kkOps.GetAuthServerResponse, error)
	UpdateAuthServer(context.Context, string, kkComps.UpdateAuthServer, ...kkOps.Option) (
		*kkOps.UpdateAuthServerResponse, error,
	)
	DeleteAuthServer(context.Context, string, *kkOps.DeleteAuthServerForce, ...kkOps.Option) (
		*kkOps.DeleteAuthServerResponse, error,
	)
}

// ListIdentityAuthServers observes the complete collection before matching or pruning.
func ListIdentityAuthServers(
	ctx context.Context, api IdentityAuthServersAPI, pageSize int64,
) ([]kkComps.AuthServer, error) {
	if api == nil {
		return nil, fmt.Errorf("identity auth-server API is not configured")
	}
	if pageSize < 1 {
		pageSize = 100
	}
	request := kkOps.ListAuthServersRequest{PageSize: &pageSize, PageNumber: new(int64(1))}
	result := make([]kkComps.AuthServer, 0)
	seen := make(map[string]bool)
	for {
		response, err := api.ListAuthServers(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to list auth servers: %w", err)
		}
		if response == nil || response.AuthServerList == nil || response.AuthServerList.Data == nil {
			return nil, fmt.Errorf("missing auth-server collection response")
		}
		page := response.AuthServerList.Meta.Page
		if page.Number != float64(*request.PageNumber) || page.Size < 1 || page.Size != math.Trunc(page.Size) ||
			page.Total < 0 || page.Total != math.Trunc(page.Total) || math.IsInf(page.Total, 0) || math.IsNaN(page.Total) {
			return nil, fmt.Errorf("invalid auth-server pagination metadata")
		}
		for _, value := range response.AuthServerList.Data {
			if value.ID == "" || seen[value.ID] {
				return nil, fmt.Errorf("auth-server collection has a missing or repeated ID")
			}
			seen[value.ID] = true
		}
		result = append(result, response.AuthServerList.Data...)
		if float64(len(result)) == page.Total {
			return result, nil
		}
		if len(response.AuthServerList.Data) == 0 || float64(len(result)) > page.Total {
			return nil, fmt.Errorf("incomplete or inconsistent auth-server pagination")
		}
		*request.PageNumber++
	}
}

func FindIdentityAuthServer(values []kkComps.AuthServer, name string) (*kkComps.AuthServer, error) {
	var result *kkComps.AuthServer
	for i := range values {
		if values[i].Name != name {
			continue
		}
		if result != nil {
			return nil, fmt.Errorf("multiple auth servers have name %q", name)
		}
		result = &values[i]
	}
	return result, nil
}

func ResolveIdentityAuthServer(ctx context.Context, api IdentityAuthServersAPI, identifier string,
	pageSize int64,
) (*kkComps.AuthServer, error) {
	if api == nil {
		return nil, fmt.Errorf("identity auth-server API is not configured")
	}
	id := identifier
	if !util.IsValidUUID(identifier) {
		values, err := ListIdentityAuthServers(ctx, api, pageSize)
		if err != nil {
			return nil, err
		}
		value, err := FindIdentityAuthServer(values, identifier)
		if err != nil {
			return nil, err
		}
		if value == nil {
			return nil, fmt.Errorf("auth server %q not found", identifier)
		}
		id = value.ID
	}
	response, err := api.GetAuthServer(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth server: %w", err)
	}
	if response == nil || response.AuthServer == nil || response.AuthServer.ID != id {
		return nil, fmt.Errorf("missing or inconsistent auth-server response")
	}
	if !util.IsValidUUID(identifier) && response.AuthServer.Name != identifier {
		return nil, fmt.Errorf("auth-server name changed during lookup; retry with its ID")
	}
	return response.AuthServer, nil
}
