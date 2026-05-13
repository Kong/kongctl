package state

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dashboardAPIStub struct {
	dashboards []kkComps.DashboardResponse
}

func (s *dashboardAPIStub) DashboardsList(
	_ context.Context,
	_ kkOps.DashboardsListRequest,
	_ ...kkOps.Option,
) (*kkOps.DashboardsListResponse, error) {
	return &kkOps.DashboardsListResponse{
		Object: &kkOps.DashboardsListResponseBody{
			Meta: &kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: float64(len(s.dashboards))},
			},
			Data: s.dashboards,
		},
	}, nil
}

func (s *dashboardAPIStub) DashboardsCreate(
	_ context.Context,
	_ kkComps.DashboardUpdateRequest,
	_ ...kkOps.Option,
) (*kkOps.DashboardsCreateResponse, error) {
	return nil, nil
}

func (s *dashboardAPIStub) DashboardsGet(
	_ context.Context,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DashboardsGetResponse, error) {
	return nil, nil
}

func (s *dashboardAPIStub) DashboardsUpdate(
	_ context.Context,
	_ string,
	_ kkComps.DashboardUpdateRequest,
	_ ...kkOps.Option,
) (*kkOps.DashboardsUpdateResponse, error) {
	return nil, nil
}

func (s *dashboardAPIStub) DashboardsDelete(
	_ context.Context,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DashboardsDeleteResponse, error) {
	return nil, nil
}

func TestClientListManagedDashboardsFiltersByNamespace(t *testing.T) {
	managedID := "managed-id"
	secondManagedID := "second-managed-id"
	otherNamespaceID := "other-namespace-id"
	client := NewClient(ClientConfig{
		DashboardsAPI: &dashboardAPIStub{
			dashboards: []kkComps.DashboardResponse{
				{
					ID:   &managedID,
					Name: "Managed",
					Labels: map[string]string{
						labels.NamespaceKey: "analytics",
					},
				},
				{
					ID:   &secondManagedID,
					Name: "Second Managed",
					Labels: map[string]string{
						labels.NamespaceKey: "analytics",
					},
				},
				{
					ID:   &otherNamespaceID,
					Name: "Other Namespace",
					Labels: map[string]string{
						labels.NamespaceKey: "other",
					},
				},
			},
		},
	})

	dashboards, err := client.ListManagedDashboards(t.Context(), []string{"analytics"})
	require.NoError(t, err)
	require.Len(t, dashboards, 2)
	assert.Equal(t, "managed-id", *dashboards[0].ID)
	assert.Equal(t, "second-managed-id", *dashboards[1].ID)
}
