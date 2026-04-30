package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// SystemAccountAccessTokenAPI defines operations for managing system account
// access tokens via the Konnect Identity v3 API.
type SystemAccountAccessTokenAPI interface {
	ListSystemAccountAccessTokens(ctx context.Context,
		request kkOps.GetSystemAccountIDAccessTokensRequest,
	) (*kkOps.GetSystemAccountIDAccessTokensResponse, error)

	GetSystemAccountAccessToken(ctx context.Context,
		accountID string, tokenID string,
	) (*kkOps.GetSystemAccountsIDAccessTokensIDResponse, error)

	CreateSystemAccountAccessToken(ctx context.Context,
		accountID string, body *kkComps.CreateSystemAccountAccessToken,
	) (*kkOps.PostSystemAccountsIDAccessTokensResponse, error)

	DeleteSystemAccountAccessToken(ctx context.Context,
		accountID string, tokenID string,
	) (*kkOps.DeleteSystemAccountsIDAccessTokensIDResponse, error)
}

// SystemAccountAccessTokenAPIImpl provides a concrete implementation backed
// by the SDK.
type SystemAccountAccessTokenAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *SystemAccountAccessTokenAPIImpl) ListSystemAccountAccessTokens(
	ctx context.Context,
	request kkOps.GetSystemAccountIDAccessTokensRequest,
) (*kkOps.GetSystemAccountIDAccessTokensResponse, error) {
	return a.SDK.SystemAccountsAccessTokens.GetSystemAccountIDAccessTokens(ctx, request)
}

func (a *SystemAccountAccessTokenAPIImpl) GetSystemAccountAccessToken(
	ctx context.Context,
	accountID string,
	tokenID string,
) (*kkOps.GetSystemAccountsIDAccessTokensIDResponse, error) {
	return a.SDK.SystemAccountsAccessTokens.GetSystemAccountsIDAccessTokensID(ctx, accountID, tokenID)
}

func (a *SystemAccountAccessTokenAPIImpl) CreateSystemAccountAccessToken(
	ctx context.Context,
	accountID string,
	body *kkComps.CreateSystemAccountAccessToken,
) (*kkOps.PostSystemAccountsIDAccessTokensResponse, error) {
	return a.SDK.SystemAccountsAccessTokens.PostSystemAccountsIDAccessTokens(ctx, accountID, body)
}

func (a *SystemAccountAccessTokenAPIImpl) DeleteSystemAccountAccessToken(
	ctx context.Context,
	accountID string,
	tokenID string,
) (*kkOps.DeleteSystemAccountsIDAccessTokensIDResponse, error) {
	return a.SDK.SystemAccountsAccessTokens.DeleteSystemAccountsIDAccessTokensID(ctx, accountID, tokenID)
}
