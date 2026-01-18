package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

type SystemAccountAPI interface {
	// SystemAccount operations
	ListSystemAccounts(ctx context.Context,
		request kkOps.GetSystemAccountsRequest) (*kkOps.GetSystemAccountsResponse, error)
	GetSystemAccount(ctx context.Context,
		id string) (*kkOps.GetSystemAccountsIDResponse, error)
}

// SystemAccountAPIImpl provides a concrete implementation backed by the SDK.
type SystemAccountAPIImpl struct {
	SDK *kkSDK.SDK
}

// ListSystemAccounts implements the SystemAccountAPI interface
func (p *SystemAccountAPIImpl) ListSystemAccounts(ctx context.Context,
	request kkOps.GetSystemAccountsRequest) (*kkOps.GetSystemAccountsResponse, error) {
	return p.SDK.SystemAccounts.GetSystemAccounts(ctx, request)
}

// GetSystemAccount implements the SystemAccountAPI interface
func (p *SystemAccountAPIImpl) GetSystemAccount(ctx context.Context,
	id string) (*kkOps.GetSystemAccountsIDResponse, error) {
	return p.SDK.SystemAccounts.GetSystemAccountsID(ctx, id)
}
