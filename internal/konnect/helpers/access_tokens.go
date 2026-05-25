package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

type PersonalAccessTokenAPI interface {
	ListUsersPersonalAccessTokens(
		ctx context.Context,
		userID string,
		opts ...kkOps.Option,
	) (*kkOps.ListUsersPersonalAccessTokensResponse, error)
	CreatePersonalAccessToken(
		ctx context.Context,
		userID string,
		request *kkComps.PersonalAccessTokenCreateRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreatePersonalAccessTokenResponse, error)
	GetPersonalAccessTokenDetails(
		ctx context.Context,
		userID string,
		tokenID string,
		opts ...kkOps.Option,
	) (*kkOps.GetPersonalAccessTokenDetailsResponse, error)
	DeletePersonalAccessToken(
		ctx context.Context,
		userID string,
		tokenID string,
		opts ...kkOps.Option,
	) (*kkOps.DeletePersonalAccessTokenResponse, error)
}

type SystemAccountAccessTokenAPI interface {
	GetSystemAccountIDAccessTokens(
		ctx context.Context,
		request kkOps.GetSystemAccountIDAccessTokensRequest,
		opts ...kkOps.Option,
	) (*kkOps.GetSystemAccountIDAccessTokensResponse, error)
	PostSystemAccountsIDAccessTokens(
		ctx context.Context,
		accountID string,
		request *kkComps.CreateSystemAccountAccessToken,
		opts ...kkOps.Option,
	) (*kkOps.PostSystemAccountsIDAccessTokensResponse, error)
	GetSystemAccountsIDAccessTokensID(
		ctx context.Context,
		accountID string,
		tokenID string,
		opts ...kkOps.Option,
	) (*kkOps.GetSystemAccountsIDAccessTokensIDResponse, error)
	DeleteSystemAccountsIDAccessTokensID(
		ctx context.Context,
		accountID string,
		tokenID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteSystemAccountsIDAccessTokensIDResponse, error)
}

type PersonalAccessTokenAPIImpl struct {
	SDK *kkSDK.SDK
}

func (p *PersonalAccessTokenAPIImpl) ListUsersPersonalAccessTokens(
	ctx context.Context,
	userID string,
	opts ...kkOps.Option,
) (*kkOps.ListUsersPersonalAccessTokensResponse, error) {
	return p.SDK.PersonalAccessTokens.ListUsersPersonalAccessTokens(ctx, userID, opts...)
}

func (p *PersonalAccessTokenAPIImpl) CreatePersonalAccessToken(
	ctx context.Context,
	userID string,
	request *kkComps.PersonalAccessTokenCreateRequest,
	opts ...kkOps.Option,
) (*kkOps.CreatePersonalAccessTokenResponse, error) {
	return p.SDK.PersonalAccessTokens.CreatePersonalAccessToken(ctx, userID, request, opts...)
}

func (p *PersonalAccessTokenAPIImpl) GetPersonalAccessTokenDetails(
	ctx context.Context,
	userID string,
	tokenID string,
	opts ...kkOps.Option,
) (*kkOps.GetPersonalAccessTokenDetailsResponse, error) {
	return p.SDK.PersonalAccessTokens.GetPersonalAccessTokenDetails(ctx, userID, tokenID, opts...)
}

func (p *PersonalAccessTokenAPIImpl) DeletePersonalAccessToken(
	ctx context.Context,
	userID string,
	tokenID string,
	opts ...kkOps.Option,
) (*kkOps.DeletePersonalAccessTokenResponse, error) {
	return p.SDK.PersonalAccessTokens.DeletePersonalAccessToken(ctx, userID, tokenID, opts...)
}

type SystemAccountAccessTokenAPIImpl struct {
	SDK *kkSDK.SDK
}

func (s *SystemAccountAccessTokenAPIImpl) GetSystemAccountIDAccessTokens(
	ctx context.Context,
	request kkOps.GetSystemAccountIDAccessTokensRequest,
	opts ...kkOps.Option,
) (*kkOps.GetSystemAccountIDAccessTokensResponse, error) {
	return s.SDK.SystemAccountsAccessTokens.GetSystemAccountIDAccessTokens(ctx, request, opts...)
}

func (s *SystemAccountAccessTokenAPIImpl) PostSystemAccountsIDAccessTokens(
	ctx context.Context,
	accountID string,
	request *kkComps.CreateSystemAccountAccessToken,
	opts ...kkOps.Option,
) (*kkOps.PostSystemAccountsIDAccessTokensResponse, error) {
	return s.SDK.SystemAccountsAccessTokens.PostSystemAccountsIDAccessTokens(ctx, accountID, request, opts...)
}

func (s *SystemAccountAccessTokenAPIImpl) GetSystemAccountsIDAccessTokensID(
	ctx context.Context,
	accountID string,
	tokenID string,
	opts ...kkOps.Option,
) (*kkOps.GetSystemAccountsIDAccessTokensIDResponse, error) {
	return s.SDK.SystemAccountsAccessTokens.GetSystemAccountsIDAccessTokensID(ctx, accountID, tokenID, opts...)
}

func (s *SystemAccountAccessTokenAPIImpl) DeleteSystemAccountsIDAccessTokensID(
	ctx context.Context,
	accountID string,
	tokenID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteSystemAccountsIDAccessTokensIDResponse, error) {
	return s.SDK.SystemAccountsAccessTokens.DeleteSystemAccountsIDAccessTokensID(ctx, accountID, tokenID, opts...)
}
