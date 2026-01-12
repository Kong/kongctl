package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

type SystemAccountsAPI interface {
	// SystemAccount operations
	ListSystemAccounts(ctx context.Context,
		request kkOps.GetSystemAccountsRequest) (*kkOps.GetSystemAccountsResponse, error)
	GetSystemAccount(ctx context.Context,
		id string) (*kkOps.GetSystemAccountsIDResponse, error)
}

// SystemAccountsAPIImpl provides a concrete implementation backed by the SDK.
type SystemAccountsAPIImpl struct {
	SDK *kkSDK.SDK
}

// ListSystemAccounts implements the SystemAccountsAPI interface
func (p *SystemAccountsAPIImpl) ListSystemAccounts(ctx context.Context,
	request kkOps.GetSystemAccountsRequest) (*kkOps.GetSystemAccountsResponse, error) {
	return p.SDK.SystemAccounts.GetSystemAccounts(ctx, request)
}

// GetSystemAccount implements the SystemAccountsAPI interface
func (p *SystemAccountsAPIImpl) GetSystemAccount(ctx context.Context,
	id string) (*kkOps.GetSystemAccountsIDResponse, error) {
	return p.SDK.SystemAccounts.GetSystemAccountsID(ctx, id)
}
