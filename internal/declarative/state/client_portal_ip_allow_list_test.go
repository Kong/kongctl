package state

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	kkErrors "github.com/Kong/sdk-konnect-go/models/sdkerrors"
)

type mockPortalIPAllowListAPI struct {
	createFunc func(
		context.Context,
		string,
		*kkComps.CreatePortalSourceIPRestriction,
		...kkOps.Option,
	) (*kkOps.CreatePortalIPAllowListResponse, error)
	updateFunc func(
		context.Context,
		kkOps.UpdatePortalIPAllowListRequest,
		...kkOps.Option,
	) (*kkOps.UpdatePortalIPAllowListResponse, error)
}

func (m *mockPortalIPAllowListAPI) CreatePortalIPAllowList(
	ctx context.Context,
	portalID string,
	request *kkComps.CreatePortalSourceIPRestriction,
	opts ...kkOps.Option,
) (*kkOps.CreatePortalIPAllowListResponse, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, portalID, request, opts...)
	}
	return nil, fmt.Errorf("CreatePortalIPAllowList not implemented")
}

func (m *mockPortalIPAllowListAPI) ListPortalIPAllowList(
	_ context.Context,
	_ kkOps.ListPortalIPAllowListRequest,
	_ ...kkOps.Option,
) (*kkOps.ListPortalIPAllowListResponse, error) {
	return nil, fmt.Errorf("ListPortalIPAllowList not implemented")
}

func (m *mockPortalIPAllowListAPI) PutPortalIPAllowList(
	_ context.Context,
	_ kkOps.PutPortalIPAllowListRequest,
	_ ...kkOps.Option,
) (*kkOps.PutPortalIPAllowListResponse, error) {
	return nil, fmt.Errorf("PutPortalIPAllowList not implemented")
}

func (m *mockPortalIPAllowListAPI) UpdatePortalIPAllowList(
	ctx context.Context,
	request kkOps.UpdatePortalIPAllowListRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdatePortalIPAllowListResponse, error) {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, request, opts...)
	}
	return nil, fmt.Errorf("UpdatePortalIPAllowList not implemented")
}

func (m *mockPortalIPAllowListAPI) DeletePortalIPAllowList(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DeletePortalIPAllowListResponse, error) {
	return nil, fmt.Errorf("DeletePortalIPAllowList not implemented")
}

func TestCreatePortalIPAllowListRecoversStatusOKSDKError(t *testing.T) {
	client := NewClient(ClientConfig{
		PortalIPAllowListAPI: &mockPortalIPAllowListAPI{
			createFunc: func(
				_ context.Context,
				_ string,
				_ *kkComps.CreatePortalSourceIPRestriction,
				_ ...kkOps.Option,
			) (*kkOps.CreatePortalIPAllowListResponse, error) {
				return nil, kkErrors.NewSDKError(
					"unknown status code returned",
					http.StatusOK,
					`{"allowed_ips":["198.51.100.10"],"id":"entry-1"}`,
					nil,
				)
			},
		},
	})

	id, err := client.CreatePortalIPAllowList(
		testContextWithLogger(),
		"portal-1",
		kkComps.CreatePortalSourceIPRestriction{AllowedIps: []string{"198.51.100.10"}},
		"default",
	)
	if err != nil {
		t.Fatalf("CreatePortalIPAllowList returned error: %v", err)
	}
	if id != "entry-1" {
		t.Fatalf("CreatePortalIPAllowList returned id %q, want %q", id, "entry-1")
	}
}

func TestUpdatePortalIPAllowListRecoversStatusOKSDKError(t *testing.T) {
	client := NewClient(ClientConfig{
		PortalIPAllowListAPI: &mockPortalIPAllowListAPI{
			updateFunc: func(
				_ context.Context,
				_ kkOps.UpdatePortalIPAllowListRequest,
				_ ...kkOps.Option,
			) (*kkOps.UpdatePortalIPAllowListResponse, error) {
				return nil, kkErrors.NewSDKError(
					"unknown status code returned",
					http.StatusOK,
					`{"allowed_ips":["198.51.100.20"],"id":"entry-2"}`,
					nil,
				)
			},
		},
	})

	id, err := client.UpdatePortalIPAllowList(
		testContextWithLogger(),
		"portal-1",
		"entry-2",
		kkComps.UpdatePortalSourceIPRestriction{AllowedIps: []string{"198.51.100.20"}},
		"default",
	)
	if err != nil {
		t.Fatalf("UpdatePortalIPAllowList returned error: %v", err)
	}
	if id != "entry-2" {
		t.Fatalf("UpdatePortalIPAllowList returned id %q, want %q", id, "entry-2")
	}
}
