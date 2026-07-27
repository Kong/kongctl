package state

import (
	"context"
	"errors"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/stretchr/testify/require"
)

func TestUpdateAIGatewayConfigStoreErrorIncludesIdentifier(t *testing.T) {
	client := NewClient(ClientConfig{
		AIGatewayConfigStoresAPI: &failingAIGatewayConfigStoreUpdateAPI{},
	})

	_, err := client.UpdateAIGatewayConfigStore(
		t.Context(),
		"gateway-id",
		"support-store-id",
		kkComps.UpdateAIGatewayConfigStoreRequest{},
		"default",
	)
	require.Error(t, err)
	require.ErrorContains(t, err, "support-store-id")
}

type failingAIGatewayConfigStoreUpdateAPI struct{}

func (failingAIGatewayConfigStoreUpdateAPI) ListAiGatewayConfigStores(
	context.Context,
	kkOps.ListAiGatewayConfigStoresRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayConfigStoresResponse, error) {
	return nil, nil
}

func (failingAIGatewayConfigStoreUpdateAPI) CreateAiGatewayConfigStore(
	context.Context,
	string,
	kkComps.CreateAIGatewayConfigStoreRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayConfigStoreResponse, error) {
	return nil, nil
}

func (failingAIGatewayConfigStoreUpdateAPI) GetAiGatewayConfigStore(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetAiGatewayConfigStoreResponse, error) {
	return nil, nil
}

func (failingAIGatewayConfigStoreUpdateAPI) UpdateAiGatewayConfigStore(
	context.Context,
	kkOps.UpdateAiGatewayConfigStoreRequest,
	...kkOps.Option,
) (*kkOps.UpdateAiGatewayConfigStoreResponse, error) {
	return nil, errors.New("update failed")
}

func (failingAIGatewayConfigStoreUpdateAPI) DeleteAiGatewayConfigStore(
	context.Context,
	kkOps.DeleteAiGatewayConfigStoreRequest,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayConfigStoreResponse, error) {
	return nil, nil
}
